// Package policy_config defines maintenance scheduling configuration loaded by config.
package policy_config

import (
	"strings"
	"time"

	"github.com/george012/gtbox/gtbox_log"
)

var CurrentCfgPolicy *PolicyConfig

type PolicyConfig struct {
	// PolicyDuration 是定期轮询事项「执行完一次到下一次开始」的间隔，Go duration 字符串（如 "10s"、"1h"）。
	PolicyDuration string `yaml:"policy_duration" json:"policy_duration" toml:"policy_duration"`
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
