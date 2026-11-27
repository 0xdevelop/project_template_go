# 程序级 runtime 承载说明

这是承载关系说明，不是业务前端。
业务前端是 `frontend` 按 `frontend_cfg.enabled`
提供的标准化页面，和这份说明不是同一个东西。

与 [程序级 runtime 承载说明](template_runtime.html) 同一关系。
点 runtime，或点调用方。
弹框约当前窗口 86vw×84vh，里面的步骤全部展开。
调用方不排进 runtime 四面。

## 程序级 runtime

四面同时属于 main。
它们承载以后挂上的业务，
不排成业务前面的步骤。

### 选配置

`setupApp` 读编译时注入的 `mRunMode`。
值是 debug、test、release。空值按 debug。

main 随后改配置路径。
debug 用 `./example_files/config_local.yaml`。
test 与 release 用 `./conf/config.yaml`。
路径相对进程当前目录。
`setupApp` 里先拼的可执行目录路径会被盖掉。

`SyncConfigFile` 打印即将加载的路径。
读失败或解析失败时记日志并 `os.Exit(1)`。
目标文件不存在，或内容为空，
先写入 `generateDefaultConfig` 再加载。
生成的缺省只监听 `127.0.0.1`。
对外部署改配置里的地址，代码不替它改。
邮件未配置齐全只日志提示，不阻断启动；验证码仍按原流程记录 INFO。

### 监听

配置可用之后，先完成 `storage.OpenStorage` 和 `storage.GormAutoMigrate`。
任一步失败都不启动 API。
随后 `ApiConfig.Validate`
检查每一个已启用的协议。
`bind_address` 为空，或 port 不在 1 到 65535，
main 记日志并返回，四个协议都不启动。
代码不补 `127.0.0.1`。

校验通过后 `StartAPIServices` 按段启动。
段为 nil，或 `Enabled` 不为 true，该协议不听。
JSON-RPC、MCP、WebSocket、gRPC 各用自己的地址。
业务入口都是 `tools/call` 的 name 和 arguments，
结果是 `CallToolResult`。
不为某个 Ability 另开 URL、消息类型或 RPC。

JSON-RPC 只有 `POST /` 进入 `APIExecuter`。
`GET /` 用 HomeHandler，
`GET /robots.txt` 用 RobotsHandler，
其余路径和方法也回 HomeHandler。
MCP 用官方 Go SDK，协议版本 2026-07-28。
WebSocket 把结果写回当前连接。
gRPC 只有一个 `APIService.Call`。

`APIExecuter` 只接受 `tools/call`，
取出 name 和 arguments，
非公开方法先走门禁，
再调用已注册的 Execute。
公开方法不读凭证；受保护方法的 JWT 沿原校验，
API key 还须注册项 `APIKeyAllowed=true`，默认 false 拒绝。
同步方法当场执行。
注册项 `Async` 为 true 时只受理并返回 `task_id`，
执行留给调度。

| 监听 | 生成缺省 | config_example.yaml |
| --- | ---: | --- |
| JSON-RPC | 127.0.0.1:7095 | 0.0.0.0:13001 |
| MCP | 127.0.0.1:7096 | 0.0.0.0:13002 |
| WebSocket | 127.0.0.1:7098 | 0.0.0.0:13004 |
| gRPC | 127.0.0.1:7099 | 0.0.0.0:13005 |

### 调度

API 启动之后，main 调用 `PolicyServicesStart`。
它拉起唯一维护循环，并返回停止函数。
main 用 defer 在退出时取消循环并等待在途任务。

每一轮按 `runtime.NumCPU()`
乘以 `policy_cfg.workers_scaller`，
再减去正在执行的 Worker 数，得到空闲名额。
倍率缺省或不是正数时按 1。
生成的缺省配置写 1。

每个 Worker 只跑一次 `ExecuteNextQueuedTask`，
结束就释放。没有常驻空转的 Worker。
长任务占着原来的名额，直到这次执行结束。
认领成功后按任务上的方法名找回同一个注册项，
把受理时存下的 arguments 交给 Execute。

两轮之间的间隔是 `policy_duration`。
缺省为 10s；解析失败或非正数时回落 10s，并记警告。
主库这一轮不可用，就等到下一轮，
不在内存里假装有队列。

### 容错

重启留下的 `running`、
且不在本进程执行集合里的任务，
由 `RequeueOrphanedTasks` 捞回排队区，
再进入同一个 Execute。

整体超时开关 `task_lifecycle_timeout_enabled` 为 true 时，
`ExpireTimedOutTasks` 处理超时任务。
生成缺省是 false。
同一类维护上一轮没结束，
`runOnceAsync` 不重复派发。
验证码清理走另一条，彼此不阻塞。

`task.cancel` 只允许取消本用户的排队中任务，
原子写入 `done/cancelled`；执行中、已结束或非属主均拒绝。
停机时调度循环的 context 被取消，
在途 Worker 结束后进程才继续往下收。

## 调用方

调用方在 runtime 外面。
它们调用已经挂上的 API。
`enabled` 只决定是否把这个面提供出来。

### test_ui

test_ui 是多协议 API 调试台。

main 只在运行模式不是 release 时调用 `LoadTestWeb`。
release 不进入这个函数，
配置里 enabled 为 true 也不会打开监听。
函数内部再用 `sync.Once` 包住。

`test_ui_cfg` 为空，或 `enabled` 不为 true，
函数直接返回。
启用时 `bind_address` 不能是空白，
port 必须在 1 到 65535。
不合法只记错误，不监听，
也不因此停掉已经启动的 API。

页面里可以选 JSON-RPC、MCP、WebSocket、gRPC，
本模板没有 REST 接口，REST 页签不显示。
选中的协议经 `POST /api/proxy` 转到对应入口，
调的是同一批已注册方法。
调试台自己不实现这些方法。

静态页来自嵌入的 `test_ui/web`。
另有 `GET /api/config` 和 `GET /docs_api`。
生成缺省地址是 `127.0.0.1:7097`。
示例文件是 `127.0.0.1:13003`。

### frontend

frontend 是标准化页面，只走一种协议。
模板页是这个调用方的壳，不在页面里改选协议。
要在页面里改选协议，用 test_ui。

debug、test、release 都会调用 `FrontendServicesStart`。
配置段缺失，或 `enabled` 为 false：不监听，
进程继续等信号。
embed 仍留在二进制里，关的是监听。

启用但地址为空，或 port 不在 1 到 65535：
`Validate` 返回错误，main 因此 return，
已启动的 API 和调度由 defer 停下。
生成缺省地址是 `127.0.0.1:7100`。
示例文件是 `127.0.0.1:13006`。
真正监听以被选中的配置文件为准。

文件来自 `frontend/dist` 的 embed。
只对存在的普通文件响应 GET 和 HEAD。
这里没有多协议切换，也没有业务代理。
退出时若服务已启动，Shutdown 最多等 5 秒。
