// Package policy 是统一策略调度域：周期从持久化排队区认领异步任务，
// 按 CPU 倍率限制全局执行数，并异步派发各项周期维护任务。
package policy

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xdevelop/project_template_go/ability/ability_task"
	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_verify_code"
	"github.com/0xdevelop/project_template_go/policy/policy_config"
	"github.com/0xdevelop/project_template_go/storage"
	"github.com/george012/gtbox/gtbox_log"
)

var (
	runningWorkers atomic.Int64
	requeueRunning atomic.Bool
	purgeRunning   atomic.Bool
)

// PolicyServicesStart 启动调度循环，返回停止派发并等待在途任务结束的方法。
func PolicyServicesStart() func() {
	ctx, cancel := context.WithCancel(context.Background())
	var pending sync.WaitGroup
	pending.Add(1)
	go func() {
		defer pending.Done()

		gtbox_log.LogInfof("policy maintenance loop started")
		for ctx.Err() == nil {
			if _, err := storage.MysqlDB(ctx); err == nil {
				dispatchQueuedTasks(ctx, &pending)

				runOnceAsync(&requeueRunning, &pending, func() {
					if err := ability_task.RequeueOrphanedTasks(ctx); err != nil {
						gtbox_log.LogErrorf("policy maintenance [async_task_requeue] failed: %v", err)
					}
				})
			}

			if _, err := storage.GormDB(ctx); err == nil {
				runOnceAsync(&purgeRunning, &pending, func() {
					if err := api_auth_verify_code.PurgeExpired(ctx); err != nil {
						gtbox_log.LogErrorf("policy maintenance [auth_verify_code_purge] failed: %v", err)
					}
				})
			}

			timer := time.NewTimer(policy_config.CurrentPolicyDuration())
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
	return func() {
		cancel()
		pending.Wait()
	}
}

// dispatchQueuedTasks 按当前空闲名额派发单次 Worker；Worker 执行完自然释放，不常驻、不空转。
func dispatchQueuedTasks(ctx context.Context, pending *sync.WaitGroup) {
	available := int64(policy_config.CurrentMaxWorkers()) - runningWorkers.Load()
	for i := int64(0); i < available; i++ {
		if ctx.Err() != nil {
			return
		}
		runningWorkers.Add(1)
		pending.Add(1)
		go func() {
			defer pending.Done()
			defer runningWorkers.Add(-1)
			if _, err := ability_task.ExecuteNextQueuedTask(ctx); err != nil {
				gtbox_log.LogErrorf("policy async task dispatch failed: %v", err)
			}
		}()
	}
}

// runOnceAsync 使不同维护任务异步互不阻塞，同一任务上一轮未结束时不重复派发。
func runOnceAsync(running *atomic.Bool, pending *sync.WaitGroup, task func()) {
	if !running.CompareAndSwap(false, true) {
		return
	}
	pending.Add(1)
	go func() {
		defer pending.Done()
		defer running.Store(false)
		task()
	}()
}
