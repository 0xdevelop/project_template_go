// Package ability_task_config defines async task worker configuration loaded by config.
package ability_task_config

import "runtime"

var CurrentCfgTask *TaskConfig

type TaskConfig struct {
	// WorkerCount 为异步任务 Worker 并发数；0 或缺省表示自动取 CPU 核数。
	WorkerCount int `yaml:"worker_count" json:"worker_count" toml:"worker_count"`
}

// CurrentWorkerCount 返回生效的 Worker 并发数；未配置或非法值回落 CPU 核数（性能参数默认，非业务兜底）。
func CurrentWorkerCount() int {
	if CurrentCfgTask == nil || CurrentCfgTask.WorkerCount < 1 {
		return runtime.NumCPU()
	}
	return CurrentCfgTask.WorkerCount
}
