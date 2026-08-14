# project_template_go 项目契约

以下规则适用于本仓库及由本模版实例化的项目的后续所有任务。

## API 与 Ability 边界

- API 通信与执行的完整目标契约以 `docs/api_description.md` 为准。
- JSON-RPC、MCP、WebSocket、gRPC 都只是通信协议 Adapter，统一运输 MCP Tool Call 数据模型：请求固定为 `tools/call → name → arguments`，业务结果固定为 MCP `CallToolResult`，不得各自复制或分叉业务调用格式。
- 每种通信协议只提供一个固定业务调用入口，所有业务能力统一通过 `params.name` 选择；不得为具体 Ability 增加独立 URL、WebSocket message type 或 gRPC RPC。`test` 是默认注册方法，不是独立入口。
- 各 Adapter 只解析自己的协议外壳，并把 `method`、`params` 和当前可选编解码所需的 `encryptionKey` 交给 `api_executer.APIExecuter`。
- `api_executer.APIExecuter` 是唯一实际业务执行入口；它统一校验 `tools/call`、提取 `name` 和 `arguments`、完成可选编解码，再读取已注册项并调用实际 `Execute`，不得维护另一份业务 `switch case`。
- 包装配采用“父包带子包”：`ability` 父包是 Ability 方法目录的唯一装配入口，由父包主动 import 并加载直属业务子包；子包在自己的 `LoadAPIMethods()` 中注册本域方法名、描述、InputSchema 和实际 `Execute`，不得反向 import、调用或注册到父包。更深层级继续由各父包按同一规则管理自己的直属子包，上层不得越级逐个装配叶子包。
- Go 不会自动发现目录下的新子包；新增 Ability 子包时，必须由其直属父包显式加入加载调用，不得用隐藏 `init()`、反向注册、反射扫描或额外框架伪造自动加载。
- `api/api_supported_methods` 只承载共享方法结构、有序目录和实际 `Execute`，不得声明具体业务域的方法或引用 `ability`。方法描述和执行函数不得维护两份映射。
- `ability` 返回 `value/error`；`APIExecuter` 统一生成 MCP `CallToolResult`。结果沿本次调用栈返回发起调用的 Adapter，再由该 Adapter 编码本协议响应。
- `CallToolResult.isError` 是固定 wire 字段：成功必须明确返回 `false`，失败必须返回 `true`；不得沿用 SDK 的默认值省略行为。
- 禁止引入 `Invocation`、同构请求包装、Method Handler 中间层、第二套执行器、返回分发器或跨协议广播。
- `arguments` 直接承载业务载荷，也是唯一可选编解码边界，不再增加额外载荷包装。具体算法、密钥来源及非标准编码不属于 API 编排契约；如启用编解码，只能转换 `arguments`，不得改变方法名、请求 ID、错误字段或协议外壳。
- `test` 固定由 `ability` 父包最先加载。新增业务方法原则上只增加对应 Ability 子包内的方法描述、实际函数和直属父包加载调用，不得修改既有通信协议实现或 `APIExecuter`。唯一例外是 `SupportedMethod.Async` 受理语义的一次性接入：`Async` 标识只允许由 `APIExecuter` 读取并统一受理（事务写任务记录、返回 `task_id`、交给 Worker 调用同一个 `Execute`）；`Execute` 保持纯业务工作函数，各异步方法不得自写受理。
- `SupportedMethod.Async` 只由后端注册设置，不属于调用参数。默认同步；异步方法必须以 MySQL 持久任务记录为真相源，返回 `task_id` 前完成事务写入，并由 Worker 后续调用同一注册项的 `Execute`。不得使用请求 goroutine、内存 channel 或连接存活状态冒充可恢复任务。

## MCP 协议基线

- MCP 只实现协议版本 `2026-07-28`，使用官方 Go SDK `v1.7.0` 及其 stateless、per-request `_meta`、`server/discover` 和标准 MCP HTTP headers；不为旧协议增加兼容分支。
- 不声明或使用已由 SEP-2577 废弃的 roots、sampling、logging capability；SDK 为兼容窗口保留的旧 API 不构成本项目能力。
- 协议层不得自造 MCP Tasks、进程内结果缓存、第二套执行链或返回分发器。长任务的受理、进度与结果查询只能实现为 Ability 层普通注册方法，走同一条统一调用链，真相源是数据库任务记录。
- JSON-RPC HTTP 只允许 `POST /` 承载真实协议调用；`GET /` 使用 `HomeHandler`，`GET /robots.txt` 使用独立 `RobotsHandler`，其余不匹配路径或方法使用 `HomeHandler`。
- gRPC 只提供一个 `APIService.Call` RPC，使用原生 protobuf `CallRequest` / `CallResponse` 运输统一调用数据和结果；实际业务方法通过 `params.name` 扩展，不新增 RPC。`.proto` 源只放在 `api/api_grpc/proto`，生成的 `.pb.go` 只放在 `api/api_grpc/protobuf`，并且只由根目录 `gen_proto.sh` 生成。

## 文档与临时目录

- 每个功能域使用对应的 `docs/ability_xxx.md` 维护功能概览、业务流程、预期目的和实现进度，README 只保留入口链接，不重复维护细节。
- Ability 文档统一模板（`<!-- TOC -->` 索引 + 编号 H1/H2，无独立大标题）：`1. <域> 域定位与边界 → 2. 业务流转（mermaid 调用/状态图）→ 3. 方法契约（每方法 `## 3.N.`：一句语义 + JSON 传参举例 + 实现要求要点）→ 4. 数据与并发纪律 → 5. 验收边界 → 6. 实现进度`；无流转或无对外方法的域可省对应节并顺延编号。入参 schema 权威只在 `api_methods.md`（编号分组由 `gen_api_docs.go` 生成、正文不带 TOC 块，禁手改；功能域树形分类目录由 `/docs_api` 渲染页左侧承担，实现在 `web/src/docs_api.ts` 的 buildToc：H1 域为可折叠父节点、H2 方法为子节点），ability 文档不重复 schema。
- 实现进度统一使用 Markdown checklist 和状态标识：`[ ] ⬜ TODO` 未开始、`[ ] 🔄 IN_PROGRESS` 推进中、`[ ] 🟡 DONE` 实现完成未验收、`[x] ✅ ACCEPTED` 验收通过终态、`[ ] ❓ DISPUTED` 有争议需对齐。只有 `ACCEPTED` 可以打勾；不得把计划、编译、局部测试或仅完成代码提前标记为验收通过。
- 方法清单文档 `docs/api_methods.md` 由根目录 `gen_api_docs.sh` 从 `api_supported_methods` 注册表生成，禁止手改；新增方法后重新执行生成，发版由 `git_tag.sh` 自动调用。对外只暴露单个渲染页 `GET /docs_api`（`api_methods.md` 内容由服务端注入页面，无任何原始文件路由，页面源码为 `test_ui/web/src/docs_api.*`）；`api_description.md` 等内部契约一律不对外、不 embed。
- `docs/` 文档需要网页呈现时统一走既定模式：文档本体留在 `docs/` 作唯一事实源，对外文档经 `docs` 包 `//go:embed` 随二进制发布；test_ui 现有 mux 挂单页路由，文档内容由服务端注入页面（dev 优先读磁盘、离仓回落 embed），不设任何原始文件路由。失败路径与未匹配路径统一 `HomeHandler` 中性空响应，不输出任何提示。不新增 HTTP 服务、不复制文档、不引入 Python 或额外构建步骤。
- 项目根目录 `tmp/` 只存放可再生成的缓存、中间产物和临时探针，允许随时整体删除。正式源码、配置事实和续做所需状态不得依赖 `tmp/` 中的内容。

## Auth 准入域与统一门禁

- Auth 是 API 准入域，实现整体位于 `api/api_auth`（子包 `api_auth_verify_code` / `api_auth_register` / `api_auth_session` / `api_auth_common` / `api_auth_config` / `api_auth_model`），负责验证码、注册/登录认证编排、session/JWT、刷新和退出；`ability` 树内不得出现鉴权/准入代码。
- APIExecuter 内置统一准入门禁：非 `Public` 注册方法在 Execute 前经 `api_auth_session.AuthenticateRequest` 鉴权——入口按 `auth_cfg.auth_type` 分发鉴权类型（缺省与当前唯一支持值均为 `jwt`；非法值 fail-closed 拒绝全部受保护方法），jwt 类型验证 `arguments.jwt_token`，验证后即将其从 arguments 移除——业务 Execute 与 Async 落库只见业务参数；身份经 context 下传，业务方法用 `api_auth_session.AuthenticatedUser(ctx)` 读取，不得自行鉴权。门禁可经 `auth_cfg.enabled` 开关（缺省启用，段/键缺席均按启用处理）：显式 `enabled: false` 时门禁放行且不提供身份（`jwt_token` 传入仍被移除），依赖 `AuthenticatedUser` 的方法照常返回权限错误——不造匿名/系统假身份。`jwt_token` 入参 schema 由方法注册表按非 Public 恒定自动注入（不随开关变化），业务注册声明即启动 panic。`SupportedMethod.Public` 零值为受保护（fail-closed），仅 `test`、验证码、注册、登录、refresh 显式标记 `Public: true`。
- api 层（协议 Adapter、APIExecuter、准入门禁、`api_auth` 域）定型后不再随业务演进修改；新增功能一律进 `ability` 业务子包。契约明记的既有例外：`SupportedMethod.Async` 受理语义的一次性接入，与 `ability_user_profile` 的顶层装配（profile 依赖父包 `ability_user` 数据方法，父包带子包装配会成 import 环）。
- Auth 需要用户身份或校验密码时必须调用 `ability_user` 的方法，不得直接查询、创建或更新 User model；注册事务把当前 GORM transaction 传给 User 方法，User 方法不得另开事务。
- 邮件发送供应商由 Auth `email` 配置的 `provider` 字段选择，当前仅支持 `resend`（官方 Go SDK）；`api_key`、`from`、`product_name`、`verification_subject` 等均为强配置，缺失或非法在配置加载层显式拒绝并输出字段级 warning 日志（指向 `auth_cfg.xxx.yyy` 键路径，敏感字段只报名不报值），不在业务代码里兜默认值。
- 密码、验证码、session/token 等敏感值不得明文落库或写日志。唯一例外：Debug 运行模式下验证码明文可写 Debug 日志供本地联调，Release/Test 模式禁止。认证失败响应不得泄露邮箱或手机号是否存在。

## MySQL 与 GORM

- `db.GlobalMysqlCtl` 是项目唯一 MySQL 入口。它由 `main.go` 使用 `gtbox_orm_mysql.Instance()` 初始化，并在连接成功后才启动 API 服务。
- 业务代码不得再次调用 `gtbox_orm_mysql.Instance()`、`gorm.Open` 或建立其他数据库连接池。
- 所有查询和写入必须使用现有全局实例；需要 GORM 能力时使用 `db.GlobalMysqlCtl.MysqlDB.WithContext(ctx)`。
- 多步写操作使用同一全局实例的 GORM `Transaction`。不得用多次无事务写入拼接一个原子业务动作。
- 表结构以 Go model 为事实源，通过全局实例执行 GORM `AutoMigrate` 同步（登记于 `db.MysqlAutoMigrate`）。Model 使用 GORM tag 声明唯一索引、普通索引、NULL 和字段约束。
- 库表 model 统一嵌入 `gorm.Model`，其 `ID` 仅作行主键不出业务接口；业务标识用归属前缀明确命名（如 `UserID`、`SessionID`），确需 UUID 时直接调用 `uuid.NewString()`，不得增加只封装一行的 ID 生成函数。
- 禁止通过 `mysql` CLI、手写 DDL、外部 shell 脚本或另一套迁移框架修改表结构；`AutoMigrate` 无法安全完成的破坏性变更必须先确认。
- 数据库未初始化或连接不可用时必须返回明确错误，禁止 nil panic，也不得把未连接环境描述成真实数据库验证通过。

## Task 异步与 Policy 维护调度

- `SupportedMethod.Async` 受理已按预留契约一次性接入 `APIExecuter`（门禁后调 `ability_task.AcceptAsyncTask` 写入持久化排队区并返回 `task_id`）；至此 api 层封版，后续任何业务演进不得再修改 `APIExecuter`。
- 任务状态三字段分层：程序级 `run_status`（queued/running/done，done 唯一终态）、业务级 `process_status`（空/success/failed/cancelled/system_error）、量化 `progress`。两层值域零交集；枚举值在 Go 常量、model 列 comment、`docs/ability_task.md` 三处同步维护，改一处必改三处。
- Worker 认领一律 `FOR UPDATE SKIP LOCKED`（MySQL 8.0+）；异步 `Execute` 必须幂等或自查重（重启恢复重跑的兜底契约）；任务载荷明文 JSON 存储，敏感值不得进入异步方法 arguments 与结果。
- `policy` 是统一调度域：`main.go` 调用 `PolicyServicesStart`，由该方法封装并拉起唯一内部大循环；每轮按 `runtime.NumCPU() × policy_cfg.workers_scaller` 扣除正在执行数，仅以剩余名额从持久化排队区派发单次 Worker。Worker 不常驻、不空转，一次只执行一条任务并在完成后自然释放；长任务跨轮持续占用原名额，不得重复扩容。其他维护事项异步互不阻塞，同类事项必须防重入；单次域函数不得自建周期死循环。重启遗留 `running` 任务由第一轮 `RequeueOrphanedTasks` 自然捞回，远程任务的 `Execute` 必须依靠持久化远程 ID/幂等键自查续跑。

## 工具与测试纪律

- 不得自造独立小工具 main 包、演示/测试程序包，工具类代码不得编进服务二进制，也不得塞进既有功能包（如 `custom_cmd`）。
- 生成类工具统一形态：根目录 `gen_*.sh` 脚本 + `//go:build ignore` 单文件生成器（与 `tools.go` 同为 build-tag 隔离，不属于任何包、不进任何构建），只由脚本 `go run` 调用。
- 不得堆积无验收价值的 `*_test.go`。新增测试必须对应真实验收路径（协议行为、关键失败路径），不为覆盖率或演示而写。

## 验收

- 协议层测试、单元测试、编译和 `AutoMigrate` 成功是不同证据，不得互相替代。
- 业务失败必须通过统一 `CallToolResult` 返回（HTTP 200 + `isError=true`）；请求外壳损坏、协议不匹配或服务内部故障才使用协议错误。
- 所有缓存和中间文件放在项目 `tmp/`，且删除 `tmp/` 不得影响后续重新构建或续做；启动的进程必须关闭并复查无残留。
