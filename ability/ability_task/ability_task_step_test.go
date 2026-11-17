package ability_task

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/0xdevelop/project_template_go/ability/ability_task/ability_task_model"
	"github.com/0xdevelop/project_template_go/api/api_error_code"
	"github.com/0xdevelop/project_template_go/policy/policy_config"
	"github.com/0xdevelop/project_template_go/storage"
	"gorm.io/gorm"
)

func acceptStepTask(t *testing.T, db *gorm.DB, method string) string {
	t.Helper()
	v, err := AcceptAsyncTask(context.Background(), method, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	id := v.(map[string]interface{})["task_id"].(string)
	if err := db.Model(&ability_task_model.AsyncTask{}).Where("task_id = ?", id).Update("user_id", "step-owner").Error; err != nil {
		t.Fatal(err)
	}
	return id
}
func stepSnapshot(t *testing.T, db *gorm.DB, id string) map[string]interface{} {
	t.Helper()
	item, err := ownedTaskWithSteps(db, id, "step-owner")
	if err != nil {
		t.Fatal(err)
	}
	return item
}
func checkClosedSteps(t *testing.T, db *gorm.DB, id string) {
	t.Helper()
	item := stepSnapshot(t, db, id)
	steps := item["steps"].([]map[string]interface{})
	if len(steps) == 0 || item["run_status"] != RunStatusDone {
		t.Fatalf("not finished: %+v", item)
	}
	for i, step := range steps {
		if step["step_seq"] != i+1 || step["completed_at"] == nil {
			t.Fatalf("open/disordered step: %+v", step)
		}
		if _, ok := step["business_statuses"].([]string); !ok {
			t.Fatalf("not a string array: %+v", step)
		}
		for _, key := range []string{"started_at", "updated_at", "completed_at"} {
			ts, e := time.Parse(time.RFC3339Nano, step[key].(string))
			if e != nil || ts.Location() != time.UTC {
				t.Fatalf("non UTC timestamp: %+v", step)
			}
		}
	}
	if steps[len(steps)-1]["step"] != EngineStepFinished {
		t.Fatal(steps)
	}
}

func TestStepsGenericBusinessAndReadOnlyRunningSnapshot(t *testing.T) {
	db := setupTaskSQLite(t)
	const method = "generic-step-probe"
	RegisterBusinessSteps(method, BusinessStepSpec{Step: "prepare", BusinessStatuses: []string{"loaded", "checked"}}, BusinessStepSpec{Step: "deliver", BusinessStatuses: []string{"written"}})
	entered, release := make(chan struct{}), make(chan struct{})
	registerProbe(t, method, func(ctx context.Context, _ interface{}) (interface{}, error) {
		if e := ReportStepBusinessStatus(ctx, "loaded", ""); !errors.Is(e, ErrNoOpenBusinessStep) {
			return nil, errors.New("engine stage accepted business status")
		}
		if e := BeginStep(ctx, "unknown", ""); !errors.Is(e, ErrStepNotRegistered) {
			return nil, errors.New("unregistered step accepted")
		}
		if e := BeginStep(ctx, "prepare", "preparing"); e != nil {
			return nil, e
		}
		if e := ReportStepBusinessStatus(ctx, "invented", ""); !errors.Is(e, ErrStepBusinessStatusNotAllowed) {
			return nil, errors.New("illegal status accepted")
		}
		for _, status := range []string{"loaded", "checked", "loaded"} {
			if e := ReportStepBusinessStatus(ctx, status, "checked input"); e != nil {
				return nil, e
			}
		}
		close(entered)
		<-release
		if e := BeginStep(ctx, "deliver", "delivering"); e != nil {
			return nil, e
		}
		if e := ReportStepBusinessStatus(ctx, "written", ""); e != nil {
			return nil, e
		}
		return "ok", nil
	})
	id := acceptStepTask(t, db, method)
	initial := stepSnapshot(t, db, id)
	if initial["current_step"].(map[string]interface{})["step"] != EngineStepAccepted {
		t.Fatal(initial)
	}
	done := make(chan error, 1)
	go func() { _, e := ExecuteNextQueuedTask(context.Background()); done <- e }()
	<-entered
	running := stepSnapshot(t, db, id)
	current := running["current_step"].(map[string]interface{})
	if running["run_status"] != RunStatusRunning || current["step"] != "prepare" || current["attempt"] != 1 || current["completed_at"] != nil {
		t.Fatal(running)
	}
	statuses := current["business_statuses"].([]string)
	if len(statuses) != 2 || statuses[0] != "loaded" || statuses[1] != "checked" {
		t.Fatal(statuses)
	}
	before := current["updated_at"]
	again := stepSnapshot(t, db, id)
	if again["current_step"].(map[string]interface{})["updated_at"] != before {
		t.Fatal("read changed step")
	}
	if _, e := ownedTaskWithSteps(db, id, "another-owner"); e == nil {
		t.Fatal("cross-owner read accepted")
	}
	close(release)
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	checkClosedSteps(t, db, id)
	if e := BeginStep(context.Background(), "prepare", ""); !errors.Is(e, ErrStepNoTaskContext) {
		t.Fatal(e)
	}
	var task ability_task_model.AsyncTask
	if e := db.Where("task_id = ?", id).Take(&task).Error; e != nil {
		t.Fatal(e)
	}
	oldCtx := context.WithValue(context.Background(), asyncTaskInfoContextKey{}, &AsyncTaskInfo{TaskID: id, ExecutionCount: 1})
	if e := BeginStep(oldCtx, "prepare", ""); !errors.Is(e, ErrTaskNotActive) {
		t.Fatal(e)
	}
}

func TestStepsPreserveRequeueHistoryAndRejectStaleAttempt(t *testing.T) {
	db := setupTaskSQLite(t)
	const method = "generic-requeue-step-probe"
	RegisterBusinessSteps(method, BusinessStepSpec{Step: "work", BusinessStatuses: []string{"observed"}})
	id := acceptStepTask(t, db, method)
	first, e := claimQueuedTask(context.Background())
	if e != nil || first == nil {
		t.Fatal(e)
	}
	firstCtx := context.WithValue(context.Background(), asyncTaskInfoContextKey{}, &AsyncTaskInfo{TaskID: id, ExecutionCount: first.ExecutionCount})
	if e := BeginStep(firstCtx, "work", "first execution"); e != nil {
		t.Fatal(e)
	}
	if e := ReportStepBusinessStatus(firstCtx, "observed", ""); e != nil {
		t.Fatal(e)
	}
	unmarkTaskInFlight(id)
	if e := RequeueOrphanedTasks(context.Background()); e != nil {
		t.Fatal(e)
	}
	queued := stepSnapshot(t, db, id)
	if queued["current_step"].(map[string]interface{})["step"] != EngineStepRequeued {
		t.Fatal(queued)
	}
	second, e := claimQueuedTask(context.Background())
	if e != nil || second == nil {
		t.Fatal(e)
	}
	defer unmarkTaskInFlight(id)
	if second.ExecutionCount != 2 {
		t.Fatal(second.ExecutionCount)
	}
	if e := BeginStep(firstCtx, "work", ""); !errors.Is(e, ErrTaskNotActive) {
		t.Fatalf("stale attempt accepted: %v", e)
	}
	secondCtx := context.WithValue(context.Background(), asyncTaskInfoContextKey{}, &AsyncTaskInfo{TaskID: id, ExecutionCount: second.ExecutionCount})
	if e := BeginStep(secondCtx, "work", "second execution"); e != nil {
		t.Fatal(e)
	}
	finishTask(context.Background(), id, ProcessStatusSuccess, `"ok"`, 0, "")
	steps := stepSnapshot(t, db, id)["steps"].([]map[string]interface{})
	if len(steps) != 7 || steps[2]["attempt"] != 1 || steps[4]["attempt"] != 2 || steps[2]["business_statuses"].([]string)[0] != "observed" {
		t.Fatal(steps)
	}
	checkClosedSteps(t, db, id)
}

func TestStepsTerminalPaths(t *testing.T) {
	for _, mode := range []string{"cancel", "queued_timeout", "running_timeout", "panic", "business_error", "missing_method", "uncertain_writeback", "retry_limit"} {
		t.Run(mode, func(t *testing.T) {
			db := setupTaskSQLite(t)
			method := "terminal-step-" + mode
			RegisterBusinessSteps(method, BusinessStepSpec{Step: "work"})
			id := acceptStepTask(t, db, method)
			switch mode {
			case "cancel":
				// Same conditional transaction helper as public task.cancel after owner verification.
				now := time.Now().UTC()
				hit, e := markTaskDoneWithStep(db, id, "task_id = ? AND user_id = ? AND run_status = ?", []interface{}{id, "step-owner", RunStatusQueued}, map[string]interface{}{"run_status": RunStatusDone, "process_status": ProcessStatusCancelled, "completed_at": &now}, now)
				if e != nil || !hit {
					t.Fatalf("cancel %t %v", hit, e)
				}
			case "queued_timeout":
				policy_config.CurrentCfgPolicy = &policy_config.PolicyConfig{TaskLifecycleTimeoutEnabled: true, TaskLifecycleTimeoutSeconds: 1}
				db.Model(&ability_task_model.AsyncTask{}).Where("task_id = ?", id).Update("created_at", time.Now().UTC().Add(-time.Hour))
				if e := ExpireTimedOutTasks(context.Background()); e != nil {
					t.Fatal(e)
				}
			case "uncertain_writeback", "retry_limit":
				_, e := claimQueuedTask(context.Background())
				if e != nil {
					t.Fatal(e)
				}
				unmarkTaskInFlight(id)
				if mode == "uncertain_writeback" {
					db.Model(&ability_task_model.AsyncTask{}).Where("task_id = ?", id).Update("writeback_pending", true)
				} else {
					policy_config.CurrentCfgPolicy = &policy_config.PolicyConfig{TaskExecuteRetryLimitEnabled: true, TaskExecuteRetryLimitCount: 0}
				}
				if e := RequeueOrphanedTasks(context.Background()); e != nil {
					t.Fatal(e)
				}
			default:
				if mode != "missing_method" {
					registerProbe(t, method, func(ctx context.Context, _ interface{}) (interface{}, error) {
						if e := BeginStep(ctx, "work", ""); e != nil {
							return nil, e
						}
						switch mode {
						case "panic":
							panic("step probe panic")
						case "business_error":
							return nil, api_error_code.ErrInvalidArguments
						case "running_timeout":
							<-ctx.Done()
							return nil, ctx.Err()
						}
						return nil, nil
					})
				}
				if mode == "running_timeout" {
					policy_config.CurrentCfgPolicy = &policy_config.PolicyConfig{TaskLifecycleTimeoutEnabled: true, TaskLifecycleTimeoutSeconds: 1}
				}
				if _, e := ExecuteNextQueuedTask(context.Background()); e != nil {
					t.Fatal(e)
				}
			}
			checkClosedSteps(t, db, id)
			expected := ProcessStatusSystemError
			if mode == "cancel" {
				expected = ProcessStatusCancelled
			}
			if mode == "business_error" {
				expected = ProcessStatusFailed
			}
			if got := stepSnapshot(t, db, id)["process_status"]; got != expected {
				t.Fatalf("got %v want %v", got, expected)
			}
		})
	}
}

func TestLegacyTaskStepsAreEmptyWithoutInventedHistory(t *testing.T) {
	db := setupTaskSQLite(t)
	if e := db.Migrator().DropTable(&ability_task_model.AsyncTaskStep{}); e != nil {
		t.Fatal(e)
	}
	user := "step-owner"
	task := &ability_task_model.AsyncTask{TaskID: "legacy", UserID: &user, Method: "legacy", RunStatus: RunStatusDone, ProcessStatus: ProcessStatusSuccess}
	if e := db.Create(task).Error; e != nil {
		t.Fatal(e)
	}
	if e := storage.GormAutoMigrate(); e != nil {
		t.Fatal(e)
	}
	item := stepSnapshot(t, db, "legacy")
	if len(item["steps"].([]map[string]interface{})) != 0 || item["current_step"] != nil {
		t.Fatal(item)
	}
}

func TestStepRegistrationRejectsAmbiguousDefinitions(t *testing.T) {
	for _, spec := range []BusinessStepSpec{{Step: "claimed"}, {Step: ""}, {Step: "work", BusinessStatuses: []string{""}}, {Step: "work", BusinessStatuses: []string{"ok", "ok"}}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("registration did not panic: %+v", spec)
				}
			}()
			RegisterBusinessSteps("invalid-register-probe", spec)
		}()
	}
	RegisterBusinessSteps("idempotent-register-probe", BusinessStepSpec{Step: "work", BusinessStatuses: []string{"ok"}})
	RegisterBusinessSteps("idempotent-register-probe", BusinessStepSpec{Step: "work", BusinessStatuses: []string{"ok"}})
	defer func() {
		if recover() == nil {
			t.Fatal("different re-registration accepted")
		}
	}()
	RegisterBusinessSteps("idempotent-register-probe", BusinessStepSpec{Step: "work", BusinessStatuses: []string{"other"}})
}
