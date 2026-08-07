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
	TaskID          string     `gorm:"type:char(36);not null;uniqueIndex" json:"task_id"`
	UserID          *string    `gorm:"type:char(36);index;comment:受理时刻的属主身份快照，无属主任务为 NULL" json:"user_id"`
	Method          string     `gorm:"size:191;not null;index" json:"method"`
	RunStatus       string     `gorm:"size:16;not null;index;comment:程序级运行状态枚举——queued:排队待认领 running:Worker 执行中 done:唯一终态" json:"run_status"`
	ProcessStatus   string     `gorm:"size:16;not null;default:'';comment:业务级处理结局枚举——空:进行中 success:成功 failed:业务失败 cancelled:已取消 system_error:系统故障" json:"process_status"`
	Progress        float64    `gorm:"not null;default:0;comment:量化进度 0-100" json:"progress"`
	ProgressMessage string     `gorm:"size:512" json:"progress_message"`
	ErrorCode       int        `gorm:"not null;default:0" json:"error_code"`
	ErrorMessage    string     `gorm:"size:512" json:"error_message"`
	InputPayload    string     `gorm:"type:mediumtext;comment:受理时的 arguments JSON，Worker 重放输入" json:"-"`
	ResultPayload   string     `gorm:"type:mediumtext;comment:success 时的 Execute 返回值 JSON" json:"-"`
	StartedAt       *time.Time `gorm:"index" json:"started_at"`
	CompletedAt     *time.Time `gorm:"index" json:"completed_at"`
}
