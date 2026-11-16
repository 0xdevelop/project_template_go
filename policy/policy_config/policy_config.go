// Package policy_config defines maintenance scheduling configuration loaded by config.
package policy_config

import (
	"runtime"
	"strings"
	"time"

	"github.com/george012/gtbox/gtbox_log"
)

var CurrentCfgPolicy *PolicyConfig

type PolicyConfig struct {
	// PolicyDuration 是定期轮询事项「执行完一次到下一次开始」的间隔，Go duration 字符串（如 "10s"、"1h"）。
	PolicyDuration string `yaml:"policy_duration" json:"policy_duration" toml:"policy_duration"`
	// WorkersScaller 是 Policy 异步任务最大并发倍率，实际上限 = CPU 数量 × 倍率。
	WorkersScaller int `yaml:"workers_scaller" json:"workers_scaller" toml:"workers_scaller"`
	// TaskLifecycleTimeoutSeconds 是任务整体生命周期超时（秒）。从 Accept 写入 created_at 起算到当前时刻。
	// 0（默认）表示关闭，不因耗时终止任务；负数按 0 处理。不为内部步骤另设短超时。未拍板的默认期限不会在此写成三天并启用。
	TaskLifecycleTimeoutSeconds int `yaml:"task_lifecycle_timeout_seconds" json:"task_lifecycle_timeout_seconds" toml:"task_lifecycle_timeout_seconds" comment:"任务整体生命周期超时（秒）。从 Accept/created_at 起算；0 表示关闭"`
	// TaskExecuteRetryLimitEnabled 为 true 时才执行 task_execute_retry_limit_count；false（默认）保持历史孤儿恢复（不按次数截断）。
	TaskExecuteRetryLimitEnabled bool `yaml:"task_execute_retry_limit_enabled" json:"task_execute_retry_limit_enabled" toml:"task_execute_retry_limit_enabled" comment:"是否启用执行重试次数限制；false 时不截断孤儿恢复。必须是 true/false，不用 0/1 代替"`
	// TaskExecuteRetryLimitCount 是首次执行之外允许的重试次数。仅当 TaskExecuteRetryLimitEnabled 为 true 时生效。0 表示不重试，不是无限。
	TaskExecuteRetryLimitCount int `yaml:"task_execute_retry_limit_count" json:"task_execute_retry_limit_count" toml:"task_execute_retry_limit_count" comment:"首次执行之外允许的重试次数；0 表示不重试，不是无限。仅在 task_execute_retry_limit_enabled=true 时生效"`
}

const defaultPolicyDuration = 10 * time.Second

// CurrentPolicyDuration 用时现解 policy_duration 字符串；缺省或非法回落默认 10s 并输出字段级警告。
func CurrentPolicyDuration() time.Duration {
	if CurrentCfgPolicy == nil || strings.TrimSpace(CurrentCfgPolicy.PolicyDuration) == "" {
		return defaultPolicyDuration
	}
	duration, err := time.ParseDuration(strings.TrimSpace(CurrentCfgPolicy.PolicyDuration))
	if err != nil || duration <= 0 {
		gtbox_log.LogWarnf("policy config invalid: policy_cfg.policy_duration is not a valid duration, falling back to %s", defaultPolicyDuration)
		return defaultPolicyDuration
	}
	return duration
}

// CurrentMaxWorkers 返回 Policy 允许的最大异步执行数；倍率缺省或非法时按 1 计算。
func CurrentMaxWorkers() int {
	scaller := 1
	if CurrentCfgPolicy != nil && CurrentCfgPolicy.WorkersScaller > 0 {
		scaller = CurrentCfgPolicy.WorkersScaller
	}
	return runtime.NumCPU() * scaller
}

// CurrentTaskLifecycleTimeout 返回整体生命周期超时；0 或负值或未配置表示关闭。
func CurrentTaskLifecycleTimeout() time.Duration {
	if CurrentCfgPolicy == nil || CurrentCfgPolicy.TaskLifecycleTimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(CurrentCfgPolicy.TaskLifecycleTimeoutSeconds) * time.Second
}

// TaskExecuteRetryLimitEnforced 表示是否按次数截断恢复性重跑。
func TaskExecuteRetryLimitEnforced() bool {
	return CurrentCfgPolicy != nil && CurrentCfgPolicy.TaskExecuteRetryLimitEnabled
}

// CurrentTaskExecuteRetryLimitCount 返回首次之外允许的重试次数；未启用限制时调用方不应使用该值截断。
// 启用时负数按 0（不重试）处理，不得解释为无限。
func CurrentTaskExecuteRetryLimitCount() int {
	if CurrentCfgPolicy == nil || CurrentCfgPolicy.TaskExecuteRetryLimitCount <= 0 {
		return 0
	}
	return CurrentCfgPolicy.TaskExecuteRetryLimitCount
}
