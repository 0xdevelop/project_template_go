[← 返回 README](../README.md)

# Policy 统一调度契约

## 预期目的

`policy` 是统一策略调度域：周期从 MySQL 持久化排队区认领异步任务，限制全局同时执行数，并异步派发未尽之事恢复、周期清理等维护事项。Task 域只负责受理、持久状态与单条任务执行，不自建 Worker 池或轮询循环。

## 大循环

`main.go` 在数据库迁移完成后调用 `PolicyServicesStart()`；该方法拉起内部 goroutine 后立即返回，由内部大循环每轮执行：

1. 读取最大执行数 `runtime.NumCPU() × policy_cfg.workers_scaller`。
2. 扣除当前正在运行的长任务，仅按剩余空闲名额异步派发单次 Worker。
3. 异步派发孤儿 `running` 任务恢复与验证码过期清理；不同维护任务互不阻塞，同一维护任务上轮未结束时跳过重复派发。
4. 间隔 `policy_cfg.policy_duration` 后进入下一轮。

Worker 不常驻、不空转：每个 Worker 只原子认领一条 `queued` 任务，执行完后释放 goroutine 与 Policy 计数名额。长任务跨越多轮时仍只占一个名额，不会因周期到达而重复执行或突破并发上限。

## 配置

```yaml
policy_cfg:
  policy_duration: "10s"
  workers_scaller: 5
```

- `policy_duration`：两轮派发的间隔，缺省或非法时回落 `10s`。
- `workers_scaller`：CPU 并发倍率，缺省或非法时回落 `1`。例如 2 CPU 配置 `5` 时最多同时执行 10 条异步任务。

## 重启续跑

进程重启后第一轮时本地 in-flight 为空，`RequeueOrphanedTasks` 将死进程遗留的 `running` 任务恢复为 `queued`，后续由 Policy 再次认领。运行期间正常执行的任务记录在本进程 in-flight 中，不会被误恢复。

进程无法从 Go 函数的中断指令处继续，因此恢复契约是按持久化输入重放。涉及远程服务的 `Execute` 必须持久化远程任务 ID/幂等键并自查续跑，避免重复创建远程任务。当前 in-flight 判定为单实例边界，多实例部署需增加数据库租约与执行实例标识。

## 装配纪律

- 新增周期事项 = 对应域提供一个单次函数 + Policy 大循环追加一次异步派发。
- 单次函数不得自建周期死循环。
- 需防重入的周期任务通过独立 `atomic.Bool` 守卫，不使用全局 `WaitGroup` 阻塞其他任务。
- 普通 API 由协议服务、Go runtime、操作系统与数据库连接池承载，不受 Policy Worker 名额影响。

## 当前实现

- [ ] 🟡 DONE — `PolicyServicesStart` 封装并拉起唯一内部大循环。
- [ ] 🟡 DONE — CPU 倍率动态并发、持久化队列认领与临时 Worker 释放。
- [ ] 🟡 DONE — 孤儿任务恢复、验证码过期清理异步防重入。
- [ ] ⬜ TODO — done 任务归档/清理进大循环清单（保留窗口待拍板）。
