package policy_config

import (
	"runtime"
	"testing"
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
