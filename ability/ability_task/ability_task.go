// Package ability_task manages durable asynchronous task state:
// 受理（APIExecuter Async 一次性接入的唯一收口）、Worker 执行、重启恢复与任务查询。
// 状态三字段核心 run_status/process_status/progress，枚举与 model 列 comment、契约文档三处同步。
package ability_task

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/0xYeah/project_template_go/ability/ability_task/ability_task_model"
	"github.com/0xYeah/project_template_go/api/api_auth/api_auth_common"
	"github.com/0xYeah/project_template_go/api/api_auth/api_auth_session"
	"github.com/0xYeah/project_template_go/api/api_error_code"
	"github.com/0xYeah/project_template_go/api/api_supported_methods"
	"github.com/0xYeah/project_template_go/common"
	appdb "github.com/0xYeah/project_template_go/db"
	"github.com/george012/gtbox/gtbox_log"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 程序级运行状态（与 model 列 comment、docs/ability_task.md 同步维护）。
const (
	RunStatusQueued  = "queued"
	RunStatusRunning = "running"
	RunStatusDone    = "done"
)

// 业务级处理结局（空字符串 = 进行中；与 model 列 comment、docs/ability_task.md 同步维护）。
const (
	ProcessStatusSuccess     = "success"
	ProcessStatusFailed      = "failed"
	ProcessStatusCancelled   = "cancelled"
	ProcessStatusSystemError = "system_error"
)

const (
	MethodTaskGet    = "task.get"
	MethodTaskList   = "task.list"
	MethodTaskCancel = "task.cancel"

	// task.list 返回条数上限。
	taskListLimit = 50
)

var ErrTaskNotActive = errors.New("task is not active")

// inFlightTasks 记录本进程 Worker 正在执行的 task_id（claim 事务提交前登记、finish 后移除），
// 供孤儿侦测排除——登记先于 DB running 状态出现，无误判窗口。
var (
	inFlightTasksMutex sync.Mutex
	inFlightTasks      = map[string]struct{}{}
)

func markTaskInFlight(taskID string) {
	inFlightTasksMutex.Lock()
	defer inFlightTasksMutex.Unlock()
	inFlightTasks[taskID] = struct{}{}
}

func unmarkTaskInFlight(taskID string) {
	inFlightTasksMutex.Lock()
	defer inFlightTasksMutex.Unlock()
	delete(inFlightTasks, taskID)
}

func inFlightTaskIDs() []string {
	inFlightTasksMutex.Lock()
	defer inFlightTasksMutex.Unlock()
	taskIDs := make([]string, 0, len(inFlightTasks))
	for taskID := range inFlightTasks {
		taskIDs = append(taskIDs, taskID)
	}
	return taskIDs
}

func LoadAPIMethods() {
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        MethodTaskGet,
		Description: "查询我的异步任务状态与结果",
		InputSchema: api_auth_common.InputSchema(
			map[string]interface{}{
				"task_id": api_auth_common.StringSchema("", 36, 36),
			},
			"task_id",
		),
		Execute: TaskGet,
	})
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        MethodTaskList,
		Description: "列出我的异步任务（新→旧，最多 50 条）",
		InputSchema: api_auth_common.InputSchema(
			map[string]interface{}{},
		),
		Execute: TaskList,
	})
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        MethodTaskCancel,
		Description: "取消我的排队中任务（执行中任务不可取消）",
		InputSchema: api_auth_common.InputSchema(
			map[string]interface{}{
				"task_id": api_auth_common.StringSchema("", 36, 36),
			},
			"task_id",
		),
		Execute: TaskCancel,
	})
}

// AcceptAsyncTask 是 Async 方法的统一受理：写入持久化排队区并返回 task_id。
// 只由 APIExecuter 调用（AGENTS 预留的一次性接入）；身份取门禁 context 快照，无身份任务属主为 NULL。
func AcceptAsyncTask(ctx context.Context, methodName string, abilityParams interface{}) (interface{}, error) {
	inputJSON, err := json.Marshal(abilityParams)
	if err != nil {
		return nil, api_error_code.ErrInvalidArguments
	}
	var userID *string
	if user, _, identityErr := api_auth_session.AuthenticatedUser(ctx); identityErr == nil {
		userID = &user.UserID
	}
	db, err := appdb.MysqlDB(ctx)
	if err != nil {
		return nil, err
	}

	task := &ability_task_model.AsyncTask{
		TaskID:       uuid.NewString(),
		UserID:       userID,
		Method:       methodName,
		RunStatus:    RunStatusQueued,
		InputPayload: string(inputJSON),
	}
	if err = db.Create(task).Error; err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"task_id":    task.TaskID,
		"run_status": task.RunStatus,
	}, nil
}

// AsyncTaskInfo 是 Worker 注入业务 Execute context 的任务身份快照；
// 与门禁在线身份（AuthenticatedUser）语义分离：异步执行不要求受理时的 session 仍存活。
type AsyncTaskInfo struct {
	TaskID string
	UserID *string
}

type asyncTaskInfoContextKey struct{}

// AsyncTaskInfoFromContext 供异步业务 Execute 读取当前任务身份快照。
func AsyncTaskInfoFromContext(ctx context.Context) (*AsyncTaskInfo, bool) {
	info, ok := ctx.Value(asyncTaskInfoContextKey{}).(*AsyncTaskInfo)
	return info, ok && info != nil
}

// ExecuteNextQueuedTask 从持久化排队区原子认领一条任务并执行。
// 本方法是单次 Worker，由 Policy 按空闲名额异步调用；无排队任务时返回 false。
func ExecuteNextQueuedTask(ctx context.Context) (bool, error) {
	task, err := claimQueuedTask(ctx)
	if err != nil || task == nil {
		return false, err
	}
	executeClaimedTask(ctx, task)
	return true, nil
}

// claimQueuedTask 用 FOR UPDATE SKIP LOCKED 认领一条排队任务（高并发多 worker 零踩踏），无任务返回 nil。
func claimQueuedTask(ctx context.Context) (*ability_task_model.AsyncTask, error) {
	db, err := appdb.MysqlDB(ctx)
	if err != nil {
		return nil, err
	}
	task := &ability_task_model.AsyncTask{}
	claimed := false
	err = db.Transaction(func(tx *gorm.DB) error {
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("run_status = ?", RunStatusQueued).
			Order("id ASC").
			Take(task).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			return nil
		}
		if findErr != nil {
			return findErr
		}
		now := time.Now().UTC()
		if updateErr := tx.Model(task).Updates(map[string]interface{}{
			"run_status": RunStatusRunning,
			"started_at": &now,
		}).Error; updateErr != nil {
			return updateErr
		}
		// in-flight 登记必须先于事务提交：DB running 可见时本进程执行记录已存在，孤儿侦测无误判窗口
		markTaskInFlight(task.TaskID)
		claimed = true
		return nil
	})
	if err != nil || !claimed {
		if claimed {
			unmarkTaskInFlight(task.TaskID)
		}
		return nil, err
	}
	return task, nil
}

// executeClaimedTask 调用注册表里同一个 Execute 并落终态；panic、方法下线、序列化故障归 system_error。
func executeClaimedTask(ctx context.Context, task *ability_task_model.AsyncTask) {
	defer unmarkTaskInFlight(task.TaskID)
	// 停止信号不中断已认领任务：跑完当前再退
	taskCtx := context.WithValue(
		context.WithoutCancel(ctx),
		asyncTaskInfoContextKey{},
		&AsyncTaskInfo{TaskID: task.TaskID, UserID: task.UserID},
	)

	supportedMethod, ok := api_supported_methods.Method(task.Method)
	if !ok {
		finishTask(taskCtx, task.TaskID, ProcessStatusSystemError, "", 0, "async method is no longer registered: "+task.Method)
		return
	}
	var input interface{}
	if err := json.Unmarshal([]byte(task.InputPayload), &input); err != nil {
		finishTask(taskCtx, task.TaskID, ProcessStatusSystemError, "", 0, "task input payload is not valid JSON")
		return
	}

	value, execErr := callExecuteRecovered(taskCtx, supportedMethod, task, input)

	if execErr != nil {
		if businessErr, isBusiness := api_error_code.As(execErr); isBusiness {
			finishTask(taskCtx, task.TaskID, ProcessStatusFailed, "", businessErr.Code, businessErr.Message)
			return
		}
		finishTask(taskCtx, task.TaskID, ProcessStatusSystemError, "", 0, execErr.Error())
		return
	}
	resultJSON, err := json.Marshal(value)
	if err != nil {
		finishTask(taskCtx, task.TaskID, ProcessStatusSystemError, "", 0, "task result is not serializable")
		return
	}
	finishTask(taskCtx, task.TaskID, ProcessStatusSuccess, string(resultJSON), 0, "")
}

// callExecuteRecovered 执行业务函数；panic 经统一封装 common.PanicHandler
// 输出富日志并转为 err，任务据此落 system_error，Worker 循环不中断。
func callExecuteRecovered(
	ctx context.Context,
	supportedMethod *api_supported_methods.SupportedMethod,
	task *ability_task_model.AsyncTask,
	input interface{},
) (value interface{}, err error) {
	defer common.PanicHandler(&err, "async task execute: method="+task.Method+" task_id="+task.TaskID)
	return supportedMethod.Execute(ctx, input)
}

// finishTask 把任务落到唯一终态 done + 对应业务结局；running 行由当前 worker 独占，直接条件更新。
func finishTask(ctx context.Context, taskID string, processStatus string, resultPayload string, errorCode int, errorMessage string) {
	db, err := appdb.MysqlDB(ctx)
	if err != nil {
		gtbox_log.LogErrorf("async task finish failed: task_id=%s: %v", taskID, err)
		return
	}
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"run_status":     RunStatusDone,
		"process_status": processStatus,
		"completed_at":   &now,
		"error_code":     errorCode,
		"error_message":  errorMessage,
	}
	if processStatus == ProcessStatusSuccess {
		updates["result_payload"] = resultPayload
		updates["progress"] = 100.0
	}
	result := db.Model(&ability_task_model.AsyncTask{}).
		Where("task_id = ? AND run_status = ?", taskID, RunStatusRunning).
		Updates(updates)
	if result.Error != nil {
		gtbox_log.LogErrorf("async task finish failed: task_id=%s: %v", taskID, result.Error)
		return
	}
	if result.RowsAffected != 1 {
		gtbox_log.LogWarnf("async task finish skipped, row not in running state: task_id=%s", taskID)
	}
}

// RequeueOrphanedTasks 侦测未尽之事：把 DB 中 running 但不在本进程 in-flight 执行集合的孤儿任务
// 重置回 queued 拉起来继续。进程重启后第一轮侦测即捞回全部死进程遗留（此时 in-flight 为空）；
// 运行中意外产生的孤儿同样每轮自愈。重跑兜底依赖异步 Execute 幂等（契约明记，责任归业务函数）。
// 由 policy 周期维护大循环调用。
func RequeueOrphanedTasks(ctx context.Context) error {
	db, err := appdb.MysqlDB(ctx)
	if err != nil {
		return err
	}
	query := db.Model(&ability_task_model.AsyncTask{}).
		Where("run_status = ?", RunStatusRunning)
	if inFlight := inFlightTaskIDs(); len(inFlight) > 0 {
		query = query.Where("task_id NOT IN ?", inFlight)
	}
	result := query.Updates(map[string]interface{}{
		"run_status": RunStatusQueued,
		"started_at": nil,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		gtbox_log.LogInfof("async task requeue: %d orphaned task(s) back to queued", result.RowsAffected)
	}
	return nil
}

// UpdateProgress 供异步业务 Execute 上报量化进度；仅活跃（非 done）任务可更新。
func UpdateProgress(ctx context.Context, taskID string, progress float64, message string) error {
	if taskID == "" || progress < 0 || progress > 100 {
		return errors.New("invalid task progress")
	}
	db, err := appdb.MysqlDB(ctx)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		task := &ability_task_model.AsyncTask{}
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("task_id = ?", taskID).Take(task).Error
		if findErr != nil {
			return findErr
		}
		if task.RunStatus == RunStatusDone {
			return ErrTaskNotActive
		}
		return tx.Model(task).Updates(map[string]interface{}{
			"progress":         progress,
			"progress_message": message,
		}).Error
	})
}

func TaskGet(ctx context.Context, input interface{}) (interface{}, error) {
	params, ok := api_auth_common.InputObject(input)
	if !ok || !api_auth_common.HasOnlyKeys(params, "task_id") {
		return nil, api_error_code.ErrInvalidArguments
	}
	taskID, taskIDOK := api_auth_common.RequiredString(params, "task_id")
	if !taskIDOK || uuid.Validate(taskID) != nil {
		return nil, api_error_code.ErrInvalidArguments
	}
	user, _, err := api_auth_session.AuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}
	db, err := appdb.MysqlDB(ctx)
	if err != nil {
		return nil, err
	}

	task := &ability_task_model.AsyncTask{}
	findErr := db.Where("task_id = ? AND user_id = ?", taskID, user.UserID).Take(task).Error
	if errors.Is(findErr, gorm.ErrRecordNotFound) {
		return nil, api_error_code.ErrInvalidArguments
	}
	if findErr != nil {
		return nil, findErr
	}
	return map[string]interface{}{"task": taskResult(task, true)}, nil
}

func TaskList(ctx context.Context, input interface{}) (interface{}, error) {
	params, ok := api_auth_common.InputObject(input)
	if !ok || !api_auth_common.HasOnlyKeys(params) {
		return nil, api_error_code.ErrInvalidArguments
	}
	user, _, err := api_auth_session.AuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}
	db, err := appdb.MysqlDB(ctx)
	if err != nil {
		return nil, err
	}

	tasks := make([]*ability_task_model.AsyncTask, 0)
	if listErr := db.Where("user_id = ?", user.UserID).
		Order("id DESC").
		Limit(taskListLimit).
		Find(&tasks).Error; listErr != nil {
		return nil, listErr
	}
	taskResults := make([]map[string]interface{}, 0, len(tasks))
	for _, task := range tasks {
		taskResults = append(taskResults, taskResult(task, false))
	}
	return map[string]interface{}{"tasks": taskResults}, nil
}

// TaskCancel 取消我的排队中任务：queued 原子置 done+cancelled；running/done 或非属主统一拒绝。
func TaskCancel(ctx context.Context, input interface{}) (interface{}, error) {
	params, ok := api_auth_common.InputObject(input)
	if !ok || !api_auth_common.HasOnlyKeys(params, "task_id") {
		return nil, api_error_code.ErrInvalidArguments
	}
	taskID, taskIDOK := api_auth_common.RequiredString(params, "task_id")
	if !taskIDOK || uuid.Validate(taskID) != nil {
		return nil, api_error_code.ErrInvalidArguments
	}
	user, _, err := api_auth_session.AuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}
	db, err := appdb.MysqlDB(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	result := db.Model(&ability_task_model.AsyncTask{}).
		Where("task_id = ? AND user_id = ? AND run_status = ?", taskID, user.UserID, RunStatusQueued).
		Updates(map[string]interface{}{
			"run_status":     RunStatusDone,
			"process_status": ProcessStatusCancelled,
			"completed_at":   &now,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, api_error_code.ErrInvalidArguments
	}
	return map[string]interface{}{"cancelled": true, "task_id": taskID}, nil
}

func taskResult(task *ability_task_model.AsyncTask, includeResult bool) map[string]interface{} {
	item := map[string]interface{}{
		"task_id":          task.TaskID,
		"method":           task.Method,
		"run_status":       task.RunStatus,
		"process_status":   task.ProcessStatus,
		"progress":         task.Progress,
		"progress_message": task.ProgressMessage,
		"error_code":       task.ErrorCode,
		"error_message":    task.ErrorMessage,
		"created_at":       task.CreatedAt.UTC().Format(time.RFC3339),
	}
	if task.StartedAt != nil {
		item["started_at"] = task.StartedAt.UTC().Format(time.RFC3339)
	}
	if task.CompletedAt != nil {
		item["completed_at"] = task.CompletedAt.UTC().Format(time.RFC3339)
	}
	if includeResult && task.ProcessStatus == ProcessStatusSuccess && task.ResultPayload != "" {
		var resultValue interface{}
		if err := json.Unmarshal([]byte(task.ResultPayload), &resultValue); err == nil {
			item["result"] = resultValue
		}
	}
	return item
}
