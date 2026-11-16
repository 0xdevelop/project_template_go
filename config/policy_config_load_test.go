package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xdevelop/project_template_go/policy/policy_config"
)

func TestLoadConfigTaskLifecycleTimeoutExplicitSwitch(t *testing.T) {
	previous := GlobalConfig
	previousPolicy := policy_config.CurrentCfgPolicy
	t.Cleanup(func() {
		GlobalConfig = previous
		policy_config.CurrentCfgPolicy = previousPolicy
	})
	base := "storage_cfg:\n  storage_sqlite:\n    enabled: true\n    path: ./data/t.db\n"
	for _, tc := range []struct {
		name    string
		policy  string
		wantErr bool
		want    bool
	}{
		{"enabled false ignores zero", "policy_cfg:\n  task_lifecycle_timeout_enabled: false\n  task_lifecycle_timeout_seconds: 0\n", false, false},
		{"enabled false ignores negative", "policy_cfg:\n  task_lifecycle_timeout_enabled: false\n  task_lifecycle_timeout_seconds: -7\n", false, false},
		{"enabled true zero rejected", "policy_cfg:\n  task_lifecycle_timeout_enabled: true\n  task_lifecycle_timeout_seconds: 0\n", true, false},
		{"enabled true negative rejected", "policy_cfg:\n  task_lifecycle_timeout_enabled: true\n  task_lifecycle_timeout_seconds: -1\n", true, false},
		{"enabled true positive", "policy_cfg:\n  task_lifecycle_timeout_enabled: true\n  task_lifecycle_timeout_seconds: 60\n", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			GlobalConfig = &FileConfig{}
			policy_config.CurrentCfgPolicy = nil
			file := filepath.Join(t.TempDir(), "c.yaml")
			if err := os.WriteFile(file, []byte(base+tc.policy), 0o600); err != nil {
				t.Fatal(err)
			}
			err := LoadConfig(file)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected validation error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if policy_config.TaskLifecycleTimeoutEnforced() != tc.want {
				t.Fatalf("enforced = %t, want %t", policy_config.TaskLifecycleTimeoutEnforced(), tc.want)
			}
		})
	}
}
