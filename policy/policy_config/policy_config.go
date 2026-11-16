// Package policy_config defines maintenance scheduling configuration loaded by config.
package policy_config

import (
	"fmt"
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
	// TaskLifecycleTimeoutEnabled 是任务整体生命周期超时开关，必须显式写 true/false，默认 false。
	// false 时不论 task_lifecycle_timeout_seconds 填什么都不生效，不因耗时终止任务，也不暗含三天或其他默认期限。
	TaskLifecycleTimeoutEnabled bool `yaml:"task_lifecycle_timeout_enabled" json:"task_lifecycle_timeout_enabled" toml:"task_lifecycle_timeout_enabled" comment:"是否启用任务整体生命周期超时；必须是 true/false。false 时忽略 task_lifecycle_timeout_seconds"`
	// TaskLifecycleTimeoutSeconds 是任务整体生命周期超时（秒），从 Accept 写入 created_at 起算到当前时刻。
	// 仅当 TaskLifecycleTimeoutEnabled 为 true 时生效，此时必须是正整数；0 或负数在配置校验时直接报错，不解释为关闭或无限。
	// 不为内部步骤另设短超时。
	TaskLifecycleTimeoutSeconds int `yaml:"task_lifecycle_timeout_seconds" json:"task_lifecycle_timeout_seconds" toml:"task_lifecycle_timeout_seconds" comment:"任务整体生命周期超时（秒），从 Accept/created_at 起算；仅在 task_lifecycle_timeout_enabled=true 时生效，且必须为正整数"`
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

// Validate 校验 policy_cfg。task_lifecycle_timeout_enabled=true 时 task_lifecycle_timeout_seconds 必须为正整数；
// 关闭时 seconds 被忽略，不做校验。未配置 policy_cfg 时视为全部关闭。
func (c *PolicyConfig) Validate() error {
	if c == nil {
		return nil
	}
	if c.TaskLifecycleTimeoutEnabled && c.TaskLifecycleTimeoutSeconds <= 0 {
		return fmt.Errorf("policy_cfg.task_lifecycle_timeout_seconds must be a positive integer (seconds) when policy_cfg.task_lifecycle_timeout_enabled=true, got %d", c.TaskLifecycleTimeoutSeconds)
	}
	return nil
}

// TaskLifecycleTimeoutEnforced 表示整体生命周期超时是否启用（只看显式开关）。
func TaskLifecycleTimeoutEnforced() bool {
	return CurrentCfgPolicy != nil && CurrentCfgPolicy.TaskLifecycleTimeoutEnabled
}

// CurrentTaskLifecycleTimeout 返回整体生命周期超时；仅在开关为 true 且 seconds 为正时返回正值，否则返回 0（调用方据此不施加期限）。
// 开关为 true 而 seconds<=0 的配置已在加载期被 Validate 拒绝，这里只做防御，不把它当作无限。
func CurrentTaskLifecycleTimeout() time.Duration {
	if !TaskLifecycleTimeoutEnforced() || CurrentCfgPolicy.TaskLifecycleTimeoutSeconds <= 0 {
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
