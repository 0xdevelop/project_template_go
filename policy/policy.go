// Package policy 是维护调度域：对外仅 PolicyServicesStart / PolicyServicesStop 两个方法
// （对齐 api.StartAPIServices / StopApiServices 形态）。具体维护事项封装在本包内部：
// 长驻执行域的拉起，以及周期大循环——每轮把维护方法清单各单次执行一遍（侦测未尽之事拉起来继续、
// 周期清理等），执行完间隔 policy_cfg.policy_duration（每轮现读现解）进入下一轮。
package policy

import (
	"context"
	"sync"
	"time"

	"github.com/0xYeah/project_template_go/ability/ability_task"
	"github.com/0xYeah/project_template_go/api/api_auth/api_auth_verify_code"
	"github.com/0xYeah/project_template_go/policy/policy_config"
	"github.com/george012/gtbox/gtbox_log"
)

var (
	policyCancel    context.CancelFunc
	policyWaitGroup sync.WaitGroup
)

// PolicyServicesStart 启动维护调度：拉起长驻执行域与周期维护大循环。
// 未尽之事（如死进程遗留的 running 任务）由大循环每轮侦测自愈——启动恢复即第一轮循环的自然效果，
// 不设特权启动阶段。main 只需调用本方法与 PolicyServicesStop。
func PolicyServicesStart() {
	ctx, cancel := context.WithCancel(context.Background())
	policyCancel = cancel

	// 长驻执行域：异步任务 Worker 池（并发数由 task_cfg.worker_count 决定，域内自管）
	policyWaitGroup.Add(1)
	go func() {
		defer policyWaitGroup.Done()
		ability_task.RunAsyncTaskWorkers(ctx)
	}()

	// 周期维护大循环：每轮单次执行清单内各维护方法，执行完间隔 policy_duration 进下一轮
	policyWaitGroup.Add(1)
	go func() {
		defer policyWaitGroup.Done()
		gtbox_log.LogInfof("policy maintenance loop started")
		for {
			runMaintenanceOnce(ctx)
			select {
			case <-ctx.Done():
				gtbox_log.LogInfof("policy maintenance loop exited")
				return
			case <-time.After(policy_config.CurrentPolicyDuration()):
			}
		}
	}()
}

// runMaintenanceOnce 单次执行维护方法清单；新增维护事项在此追加一行，main 与循环骨架不动。
func runMaintenanceOnce(ctx context.Context) {
	// 未尽之事侦测：DB running 且不在本进程执行中的孤儿任务 → 重置 queued 拉起来继续
	if err := ability_task.RequeueOrphanedTasks(ctx); err != nil {
		gtbox_log.LogErrorf("policy maintenance [async_task_requeue] failed: %v", err)
	}
	// 周期清理：过期超保留窗口的验证码行
	if err := api_auth_verify_code.PurgeExpired(ctx); err != nil {
		gtbox_log.LogErrorf("policy maintenance [auth_verify_code_purge] failed: %v", err)
	}
}

// PolicyServicesStop 优雅停止维护调度：通知退出并等待长驻执行域与大循环跑完手头事项收尾。
func PolicyServicesStop() {
	if policyCancel == nil {
		return
	}
	policyCancel()
	policyWaitGroup.Wait()
	gtbox_log.LogInfof("policy services stopped")
}
