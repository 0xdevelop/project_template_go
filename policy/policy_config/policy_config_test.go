package policy_config

import (
	"runtime"
	"testing"
	"time"
)

func TestCurrentMaxWorkers(t *testing.T) {
	original := CurrentCfgPolicy
	t.Cleanup(func() { CurrentCfgPolicy = original })

	tests := []struct {
		name    string
		config  *PolicyConfig
		scaller int
	}{
		{name: "missing config", config: nil, scaller: 1},
		{name: "missing scaller", config: &PolicyConfig{}, scaller: 1},
		{name: "invalid scaller", config: &PolicyConfig{WorkersScaller: -1}, scaller: 1},
		{name: "configured scaller", config: &PolicyConfig{WorkersScaller: 5}, scaller: 5},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			CurrentCfgPolicy = test.config
			want := runtime.NumCPU() * test.scaller
			if got := CurrentMaxWorkers(); got != want {
				t.Fatalf("CurrentMaxWorkers() = %d, want %d", got, want)
			}
		})
	}
}

func TestTaskReliabilityConfigDefaults(t *testing.T) {
	original := CurrentCfgPolicy
	t.Cleanup(func() { CurrentCfgPolicy = original })

	t.Run("missing config means timeout off and retry limit not enforced", func(t *testing.T) {
		CurrentCfgPolicy = nil
		if CurrentTaskLifecycleTimeout() != 0 {
			t.Fatalf("timeout = %s, want 0", CurrentTaskLifecycleTimeout())
		}
		if TaskExecuteRetryLimitEnforced() {
			t.Fatal("retry limit must be off by default")
		}
		if CurrentTaskExecuteRetryLimitCount() != 0 {
			t.Fatalf("retry count = %d, want 0", CurrentTaskExecuteRetryLimitCount())
		}
	})

	t.Run("zero and negative timeout stay off", func(t *testing.T) {
		CurrentCfgPolicy = &PolicyConfig{TaskLifecycleTimeoutSeconds: 0}
		if CurrentTaskLifecycleTimeout() != 0 {
			t.Fatal("0 must mean disabled, not an implicit default deadline")
		}
		CurrentCfgPolicy = &PolicyConfig{TaskLifecycleTimeoutSeconds: -1}
		if CurrentTaskLifecycleTimeout() != 0 {
			t.Fatal("negative timeout must be treated as disabled, not infinite")
		}
	})

	t.Run("positive timeout is created_at lifecycle in seconds", func(t *testing.T) {
		CurrentCfgPolicy = &PolicyConfig{TaskLifecycleTimeoutSeconds: 90}
		if CurrentTaskLifecycleTimeout() != 90*time.Second {
			t.Fatalf("got %s", CurrentTaskLifecycleTimeout())
		}
	})

	t.Run("retry limit uses true/false and 0 means no retry", func(t *testing.T) {
		CurrentCfgPolicy = &PolicyConfig{TaskExecuteRetryLimitEnabled: false, TaskExecuteRetryLimitCount: 3}
		if TaskExecuteRetryLimitEnforced() {
			t.Fatal("false must not enforce the count")
		}
		CurrentCfgPolicy = &PolicyConfig{TaskExecuteRetryLimitEnabled: true, TaskExecuteRetryLimitCount: 0}
		if !TaskExecuteRetryLimitEnforced() {
			t.Fatal("true must enforce the count")
		}
		if CurrentTaskExecuteRetryLimitCount() != 0 {
			t.Fatal("0 means no retry, not unlimited")
		}
		CurrentCfgPolicy = &PolicyConfig{TaskExecuteRetryLimitEnabled: true, TaskExecuteRetryLimitCount: -8}
		if CurrentTaskExecuteRetryLimitCount() != 0 {
			t.Fatal("negative count must be 0, not unlimited")
		}
		CurrentCfgPolicy = &PolicyConfig{TaskExecuteRetryLimitEnabled: true, TaskExecuteRetryLimitCount: 2}
		if CurrentTaskExecuteRetryLimitCount() != 2 {
			t.Fatalf("got %d", CurrentTaskExecuteRetryLimitCount())
		}
	})
}
