# 项目协作约定

完整项目契约以 `AGENTS.md` 为准。

## 新会话对齐（本文件是唯一自动加载入口，其余文档必须显式读）

动手前按改动域先读完对应文档，再动代码：

- 任何改动：`AGENTS.md`（全量强制规则）。
- API 编排 / 协议 Adapter / 执行链 / 新增方法：`docs/api_description.md`。
- 架构总览与模版使用方式：`README.md`。

## 会话必守硬边界（摘要，全文以 `AGENTS.md` 为准）

- JSON-RPC、MCP、WebSocket、gRPC 只是通信协议 Adapter，统一运输 `tools/call → name → arguments`，统一返回 MCP `CallToolResult`；不得各自分叉业务调用格式。
- `api_executer.APIExecuter` 是唯一业务执行入口；禁止第二套业务 `switch`、`Invocation`、Method Handler 中间层、返回分发器、跨协议广播。
- 新增业务方法只增加对应 Ability 子包内注册和直属父包（`ability`）加载调用，不修改 Adapter、`APIExecuter` 或 proto；装配按“父包带子包”，禁止隐藏 `init()`、反向注册、反射扫描。
- `.proto` 源只放 `api/api_grpc/proto`，生成的 `.pb.go` 只放 `api/api_grpc/protobuf`，只由根目录 `gen_proto.sh` 生成。
- `db.GlobalMysqlCtl` 是唯一 MySQL 入口；表结构以 Go model + `AutoMigrate` 为事实源，禁止手写 DDL 或第二套迁移。
- **禁止自造流程与工具程序**：不建独立小工具 main 包 / 测试程序包，工具代码不进服务二进制、不塞进既有功能包；生成类工具只有一种形态——根目录 `gen_*.sh` + `//go:build ignore` 单文件（`tools.go` 同款隔离）。不堆无验收价值的 `*_test.go`。动任何既有包前先确认其定位。
- Auth 是 API 准入域，实现位于 `api/api_auth`；APIExecuter 统一准入门禁在 Execute 前验 `arguments.jwt_token` 并即时将其从 arguments 移除，业务方法经 `api_auth_session.AuthenticatedUser(ctx)` 取身份、不自行鉴权、不见 token；`jwt_token` schema 由方法注册表按非 Public 自动注入，业务注册禁止声明。
- API 文档链：`docs/api_methods.md` 由 `gen_api_docs.sh` 从方法注册表生成（功能域编号分组，正文不带 TOC；域树形分类目录在 `/docs_api` 渲染页左侧，实现在 `web/src/docs_api.ts`），禁止手改。对外只有 `GET /docs_api` 单渲染页（内容由 `test_ui` 服务端注入，页面源码 `test_ui/web/src/docs_api.*`，无原始文件路由）；`api_description.md` 等内部契约不对外、不 embed。

## 本地运行事实

- 默认端口：JSON-RPC `13001`、MCP `13002`、test_ui `13003`、WebSocket `13004`、gRPC `13005`。
- 本地起服务用 `go run`（Debug 模式读 `./example_files/config_local.yaml`）；改码重测前按端口精确 kill 旧进程再重启。
- 会话临时文件一律放项目根 `tmp/`（允许整体删除，不得承载续做状态）。

## 代码注释

方法注释只需简要说明方法本身做什么，不重复实现步骤、参数细节或架构背景。
