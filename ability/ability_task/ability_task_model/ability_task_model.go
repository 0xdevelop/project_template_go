// Package ability_task_model stores durable asynchronous task records.
package ability_task_model

import (
	"time"

	"gorm.io/gorm"
)

// AsyncTask 状态三字段核心：RunStatus（程序级）+ ProcessStatus（业务级）+ Progress（量化进度）。
// 两层状态值域零交集：RunStatus 只表达调度生命周期，ProcessStatus 只表达业务处理结局；
// RunStatus=done 是唯一终态，且 done ⟺ ProcessStatus 非空。
// 枚举值与 ability_task 包内 RunStatus* / ProcessStatus* 常量及列 comment 三处同步维护。
type AsyncTask struct {
	gorm.Model
	TaskID           string     `gorm:"type:char(36);not null;uniqueIndex" json:"task_id"`
	UserID           *string    `gorm:"type:char(36);index;comment:受理时刻的属主身份快照，无属主任务为 NULL" json:"user_id"`
	Method           string     `gorm:"size:191;not null;index" json:"method"`
	RunStatus        string     `gorm:"size:16;not null;index;comment:程序级运行状态枚举——queued:排队待认领 running:Worker 执行中 done:唯一终态" json:"run_status"`
	ProcessStatus    string     `gorm:"size:16;not null;default:'';comment:业务级处理结局枚举——空:进行中 success:成功 failed:业务失败 cancelled:已取消 system_error:系统故障" json:"process_status"`
	Progress         float64    `gorm:"not null;default:0;comment:量化进度 0-100" json:"progress"`
	ProgressMessage  string     `gorm:"size:512" json:"progress_message"`
	ErrorCode        int        `gorm:"not null;default:0" json:"error_code"`
	ErrorMessage     string     `gorm:"size:512" json:"error_message"`
	InputPayload     string     `gorm:"size:16777215;comment:受理时的 arguments JSON，Worker 重放输入" json:"-"`
	ResultPayload    string     `gorm:"size:16777215;comment:success 时的 Execute 返回值 JSON" json:"-"`
	StartedAt        *time.Time `gorm:"index" json:"started_at"`
	CompletedAt      *time.Time `gorm:"index" json:"completed_at"`
	ExecutionCount   int        `gorm:"not null;default:0;comment:已认领执行次数，含首次；用于重试次数限制" json:"execution_count"`
	WritebackPending bool       `gorm:"not null;default:false;comment:Execute 已结束但终态写回未完成，禁止当作孤儿重跑" json:"writeback_pending"`
}

// AsyncTaskStep 是任务的步进记录（只追加、只读回看）：每行一个阶段，按 (task_id, step_seq) 唯一。
// RunStatus（程序级）只由 Go 引擎写；BusinessStatuses（业务级字符串枚举阵列）只由业务 Execute 经引擎 API 追加。
// 枚举值与 ability_task 包内 StepKind* / RunStatus* / EngineStep* 常量及 docs/ability_task.md 三处同步维护。
type AsyncTaskStep struct {
	gorm.Model
	TaskID           string     `gorm:"type:char(36);not null;uniqueIndex:idx_async_task_step_seq,priority:1" json:"task_id"`
	StepSeq          int        `gorm:"not null;uniqueIndex:idx_async_task_step_seq,priority:2;comment:任务内步进序号，从 1 起连续递增，跨执行轮次不重置" json:"step_seq"`
	Attempt          int        `gorm:"not null;default:0;comment:所属执行轮次，等于写入时任务 execution_count；0=首次认领之前（受理排队）" json:"attempt"`
	Step             string     `gorm:"size:64;not null;comment:阶段名——引擎阶段 accepted:受理排队 claimed:认领执行 requeued:孤儿重排队 finished:终态；业务阶段为业务注册的名字" json:"step"`
	StepKind         string     `gorm:"size:16;not null;comment:阶段来源枚举——engine:Go 引擎写 business:业务 Execute 经引擎 API 写" json:"step_kind"`
	RunStatus        string     `gorm:"size:16;not null;comment:本阶段所处的程序级运行状态枚举（与任务 run_status 同值域）——queued running done，只由引擎写" json:"run_status"`
	BusinessStatuses string     `gorm:"size:4096;not null;default:'[]';comment:业务级状态字符串枚举阵列 JSON，按首次上报顺序去重；引擎阶段恒为 []" json:"-"`
	Message          string     `gorm:"size:512;not null;default:''" json:"message"`
	StartedAt        time.Time  `gorm:"not null" json:"started_at"`
	CompletedAt      *time.Time `gorm:"comment:阶段结束时刻，NULL=当前阶段仍开放；下一阶段开启或任务终态时由引擎写" json:"completed_at"`
}
