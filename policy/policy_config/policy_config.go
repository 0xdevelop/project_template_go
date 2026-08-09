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
