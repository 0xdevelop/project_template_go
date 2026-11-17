package ability_task

import (
	"context"
	"errors"
	"regexp"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/0xdevelop/project_template_go/ability/ability_task/ability_task_model"
	"github.com/0xdevelop/project_template_go/storage"
	"github.com/goccy/go-json"
	"gorm.io/gorm"
)

// 步进阶段来源（与 model 列 comment、docs/ability_task.md 同步维护）。
const (
	StepKindEngine   = "engine"
	StepKindBusiness = "business"
)

// 引擎阶段名：只由 Go 引擎写入，业务不得注册同名阶段（与 model 列 comment、docs/ability_task.md 同步维护）。
const (
	EngineStepAccepted = "accepted"
	EngineStepClaimed  = "claimed"
	EngineStepRequeued = "requeued"
	EngineStepFinished = "finished"
)

// stepMessageMaxBytes 与 model Message 列 size 一致；超长按 UTF-8 边界截断。
const stepMessageMaxBytes = 512

var (
	ErrStepNoTaskContext            = errors.New("step report requires async task execute context")
	ErrStepNotRegistered            = errors.New("step is not registered for this task method")
	ErrStepBusinessStatusNotAllowed = errors.New("business status is not registered for the current step")
	ErrNoOpenBusinessStep           = errors.New("no open business step in the current attempt")
)

var stepNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// BusinessStepSpec 是业务为某个 Async 方法注册的一个阶段及其允许的业务状态字符串枚举值域。
// BusinessStatuses 可为空：该阶段只表达阶段本身，任何 ReportStepBusinessStatus 都会被拒绝。
type BusinessStepSpec struct {
	Step             string
	BusinessStatuses []string
}

var (
	businessStepRegistryMutex sync.RWMutex
	// method -> step -> allowed business statuses
	businessStepRegistry = map[string]map[string]map[string]struct{}{}
)

// RegisterBusinessSteps 由业务子包在自己的 LoadAPIMethods 中为 Async 方法注册阶段与业务状态值域。
// 非法名字、与引擎阶段重名、重复阶段/状态、同一方法以不同定义重复注册均启动期 panic；
// 同一方法以完全相同的定义重复注册视为幂等（测试与重复装配场景）。
func RegisterBusinessSteps(method string, specs ...BusinessStepSpec) {
	if method == "" {
		panic("ability_task: RegisterBusinessSteps requires a method name")
	}
	if len(specs) == 0 {
		panic("ability_task: RegisterBusinessSteps requires at least one step for " + method)
	}
	steps := make(map[string]map[string]struct{}, len(specs))
	for _, spec := range specs {
		if !stepNamePattern.MatchString(spec.Step) {
			panic("ability_task: invalid business step name for " + method + ": " + spec.Step)
		}
		switch spec.Step {
		case EngineStepAccepted, EngineStepClaimed, EngineStepRequeued, EngineStepFinished:
			panic("ability_task: business step must not reuse engine step name: " + spec.Step)
		}
		if _, dup := steps[spec.Step]; dup {
			panic("ability_task: duplicate business step for " + method + ": " + spec.Step)
		}
		if len(spec.BusinessStatuses) > 32 {
			panic("ability_task: at most 32 business statuses per step")
		}
		statuses := make(map[string]struct{}, len(spec.BusinessStatuses))
		for _, status := range spec.BusinessStatuses {
			if !stepNamePattern.MatchString(status) {
				panic("ability_task: invalid business status for " + method + "/" + spec.Step + ": " + status)
			}
			if _, dup := statuses[status]; dup {
				panic("ability_task: duplicate business status for " + method + "/" + spec.Step + ": " + status)
			}
			statuses[status] = struct{}{}
		}
		steps[spec.Step] = statuses
	}
	businessStepRegistryMutex.Lock()
	defer businessStepRegistryMutex.Unlock()
	if existing, ok := businessStepRegistry[method]; ok {
		if !sameStepDefinition(existing, steps) {
			panic("ability_task: business steps already registered with a different definition for " + method)
		}
		return
	}
	businessStepRegistry[method] = steps
}

func sameStepDefinition(a, b map[string]map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for step, aStatuses := range a {
		bStatuses, ok := b[step]
		if !ok || len(aStatuses) != len(bStatuses) {
			return false
		}
		for status := range aStatuses {
			if _, ok := bStatuses[status]; !ok {
				return false
			}
		}
	}
	return true
}

func registeredBusinessStep(method, step string) (map[string]struct{}, bool) {
	businessStepRegistryMutex.RLock()
	defer businessStepRegistryMutex.RUnlock()
	statuses, ok := businessStepRegistry[method][step]
	return statuses, ok
}

func truncateStepMessage(message string) string {
	if len(message) <= stepMessageMaxBytes {
		return message
	}
	cut := stepMessageMaxBytes
	for cut > 0 && !utf8.RuneStart(message[cut]) {
		cut--
	}
	return message[:cut]
}

// appendTaskStep 关闭该任务所有开放阶段（completed_at=now），再追加一条新阶段。
// 必须在已对任务行写锁定（条件 UPDATE 命中）的事务内调用；step_seq 取 MAX+1，唯一索引兜底。
// runStatus=done 的阶段（finished）开启即结束：started_at=completed_at=now。
func appendTaskStep(tx *gorm.DB, taskID string, attempt int, step, kind, runStatus, message string, now time.Time) error {
	if err := tx.Model(&ability_task_model.AsyncTaskStep{}).
		Where("task_id = ? AND completed_at IS NULL", taskID).
		Updates(map[string]interface{}{"completed_at": &now, "updated_at": now}).Error; err != nil {
		return err
	}
	var maxSeq int
	if err := tx.Model(&ability_task_model.AsyncTaskStep{}).
		Where("task_id = ?", taskID).
		Select("COALESCE(MAX(step_seq), 0)").
		Scan(&maxSeq).Error; err != nil {
		return err
	}
	row := &ability_task_model.AsyncTaskStep{
		Model:            gorm.Model{CreatedAt: now, UpdatedAt: now},
		TaskID:           taskID,
		StepSeq:          maxSeq + 1,
		Attempt:          attempt,
		Step:             step,
		StepKind:         kind,
		RunStatus:        runStatus,
		BusinessStatuses: "[]",
		Message:          truncateStepMessage(message),
		StartedAt:        now,
	}
	if runStatus == RunStatusDone {
		row.CompletedAt = &now
	}
	return tx.Create(row).Error
}

// appendFinishedStep 记录引擎终态阶段：attempt 取任务当前 execution_count，message 等于任务 error_message。
// 必须在把任务置 done 的同一事务内、条件 UPDATE 命中之后调用。
func appendFinishedStep(tx *gorm.DB, taskID string, errorMessage string, now time.Time) error {
	var attempt int
	if err := tx.Model(&ability_task_model.AsyncTask{}).
		Where("task_id = ?", taskID).
		Select("execution_count").
		Scan(&attempt).Error; err != nil {
		return err
	}
	return appendTaskStep(tx, taskID, attempt, EngineStepFinished, StepKindEngine, RunStatusDone, errorMessage, now)
}

// lockRunningAttempt 以条件 UPDATE 写锁定当前执行轮次的 running 任务行并返回其 method；
// 任务不再 running 或已进入新的执行轮次（旧轮次迟到写入）时返回 ErrTaskNotActive。
func lockRunningAttempt(tx *gorm.DB, info *AsyncTaskInfo, now time.Time) (string, error) {
	result := tx.Model(&ability_task_model.AsyncTask{}).
		Where("task_id = ? AND run_status = ? AND execution_count = ?", info.TaskID, RunStatusRunning, info.ExecutionCount).
		Update("updated_at", now)
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected != 1 {
		return "", ErrTaskNotActive
	}
	var method string
	if err := tx.Model(&ability_task_model.AsyncTask{}).
		Where("task_id = ?", info.TaskID).
		Select("method").
		Scan(&method).Error; err != nil {
		return "", err
	}
	return method, nil
}

// BeginStep 供异步业务 Execute 开启一个已注册的业务阶段：关闭当前开放阶段，追加
// step_kind=business、run_status=running（引擎写）、business_statuses=[] 的新阶段。
// 只能在 Worker 注入的任务 context 内、当前执行轮次 running 时调用；未注册阶段返回 ErrStepNotRegistered。
func BeginStep(ctx context.Context, step string, message string) error {
	info, ok := AsyncTaskInfoFromContext(ctx)
	if !ok {
		return ErrStepNoTaskContext
	}
	db, err := storage.GormDB(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return db.Transaction(func(tx *gorm.DB) error {
		method, lockErr := lockRunningAttempt(tx, info, now)
		if lockErr != nil {
			return lockErr
		}
		if _, registered := registeredBusinessStep(method, step); !registered {
			return ErrStepNotRegistered
		}
		return appendTaskStep(tx, info.TaskID, info.ExecutionCount, step, StepKindBusiness, RunStatusRunning, message, now)
	})
}

// ReportStepBusinessStatus 供异步业务 Execute 向当前开放的业务阶段追加一个业务状态值：
// 值必须在该方法该阶段注册的值域内；已存在则不重复追加（阵列按首次上报顺序去重）。
// message 原样覆盖该阶段 message（空串即清空）。
func ReportStepBusinessStatus(ctx context.Context, businessStatus string, message string) error {
	info, ok := AsyncTaskInfoFromContext(ctx)
	if !ok {
		return ErrStepNoTaskContext
	}
	db, err := storage.GormDB(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return db.Transaction(func(tx *gorm.DB) error {
		method, lockErr := lockRunningAttempt(tx, info, now)
		if lockErr != nil {
			return lockErr
		}
		current := &ability_task_model.AsyncTaskStep{}
		findErr := tx.Where("task_id = ?", info.TaskID).Order("step_seq DESC").Take(current).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			return ErrNoOpenBusinessStep
		}
		if findErr != nil {
			return findErr
		}
		if current.StepKind != StepKindBusiness || current.Attempt != info.ExecutionCount || current.CompletedAt != nil {
			return ErrNoOpenBusinessStep
		}
		allowed, registered := registeredBusinessStep(method, current.Step)
		if !registered {
			return ErrStepNotRegistered
		}
		if _, allowedStatus := allowed[businessStatus]; !allowedStatus {
			return ErrStepBusinessStatusNotAllowed
		}
		statuses, parseErr := decodeBusinessStatuses(current.BusinessStatuses)
		if parseErr != nil {
			return parseErr
		}
		present := false
		for _, existing := range statuses {
			if existing == businessStatus {
				present = true
				break
			}
		}
		if !present {
			statuses = append(statuses, businessStatus)
		}
		encoded, encodeErr := json.Marshal(statuses)
		if encodeErr != nil {
			return encodeErr
		}
		return tx.Model(current).Updates(map[string]interface{}{
			"business_statuses": string(encoded),
			"updated_at":        now,
			"message":           truncateStepMessage(message),
		}).Error
	})
}

func decodeBusinessStatuses(raw string) ([]string, error) {
	statuses := make([]string, 0)
	if raw == "" {
		return statuses, nil
	}
	if err := json.Unmarshal([]byte(raw), &statuses); err != nil {
		return nil, err
	}
	if statuses == nil {
		statuses = make([]string, 0)
	}
	return statuses, nil
}

// loadTaskSteps 读取任务全部阶段（step_seq 升序）并序列化为只读结果。
func loadTaskSteps(tx *gorm.DB, taskID string) ([]map[string]interface{}, error) {
	rows := make([]*ability_task_model.AsyncTaskStep, 0)
	if err := tx.Where("task_id = ?", taskID).Order("step_seq ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	steps := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		statuses, err := decodeBusinessStatuses(row.BusinessStatuses)
		if err != nil {
			return nil, err
		}
		item := map[string]interface{}{
			"step_seq":          row.StepSeq,
			"attempt":           row.Attempt,
			"step":              row.Step,
			"step_kind":         row.StepKind,
			"run_status":        row.RunStatus,
			"business_statuses": statuses,
			"message":           row.Message,
			"started_at":        row.StartedAt.UTC().Format(time.RFC3339Nano),
			"updated_at":        row.UpdatedAt.UTC().Format(time.RFC3339Nano),
			"completed_at":      nil,
		}
		if row.CompletedAt != nil {
			item["completed_at"] = row.CompletedAt.UTC().Format(time.RFC3339Nano)
		}
		steps = append(steps, item)
	}
	return steps, nil
}
