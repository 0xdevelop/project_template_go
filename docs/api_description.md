# API 通信与执行架构

本文定义 API 调用编排、Ability 执行和业务返回的稳定边界。

## 统一调用格式

项目统一复用 MCP Tool Call 的数据模型。只有 MCP 端口实现完整 MCP 协议；JSON-RPC、WebSocket 和 gRPC 只运输同一份 Tool Call 请求和 `CallToolResult`，不得各自定义另一套业务调用格式。

每种通信协议只提供一个固定业务调用入口。具体 Ability 统一通过 `params.name` 选择，不得为业务方法增加独立 URL、WebSocket message type 或 gRPC RPC。`test` 是默认注册且固定排在第一项的方法，不是独立入口。

唯一业务调用请求为：

```json
{
  "jsonrpc": "2.0",
  "id": "request-id",
  "method": "tools/call",
  "params": {
    "name": "test",
    "arguments": {}
  }
}
```

固定含义：

- `id` 是 JSON-RPC/MCP 单次请求关联标识，不属于业务参数。
- Adapter 只解析本协议外壳，并把外层 `method`、`params` 和当前可选编解码所需的 `encryptionKey` 交给 `APIExecuter`。
- `APIExecuter` 内统一校验 `tools/call`，并从 `params` 提取 `name` 和 `arguments`；各 Adapter 不得重复实现这段提取。
- `params.name` 是注册的方法名。
- `params.arguments` 直接是方法的业务入参载荷，不再增加额外载荷包装。
- 不再支持把业务方法名直接放在 JSON-RPC `method` 中调用。

`arguments` 是唯一可选编解码边界。未编码时为方法自己的 JSON 对象；启用自定义或非标准编解码时为加密后的 JSON 字符串。编解码不得改变 `id`、`name`、`error_code`、`error_msg` 或协议外壳，也不得被本架构绑定到某种算法、密钥来源或 Session 规则。MCP 标准的 `arguments` 是对象，因此字符串形式属于项目自定义加密扩展，不是标准 MCP 请求。

## 统一返回格式

`APIExecuter` 统一返回 MCP `CallToolResult`：

```json
{
  "_meta": {
    "io.modelcontextprotocol/serverInfo": {
      "name": "project_template_go",
      "version": "v0.0.3"
    }
  },
  "content": [
    {
      "type": "text",
      "text": "{\"order_id\":\"order-123\"}"
    }
  ],
  "isError": false,
  "resultType": "complete"
}
```

业务失败仍返回同一结构：

```json
{
  "_meta": {
    "io.modelcontextprotocol/serverInfo": {
      "name": "project_template_go",
      "version": "v0.0.3"
    }
  },
  "content": [
    {
      "type": "text",
      "text": "{\"error_code\":10002,\"error_msg\":\"method is not supported\"}"
    }
  ],
  "isError": true,
  "resultType": "complete"
}
```

约束：

- 普通 JSON 业务结果使用一个 `TextContent`，其 `type` 为 `text`，`text` 是业务 JSON 字符串；图片、音频和资源类结果应使用 MCP 对应的 content 类型，不得把二进制伪装成 JSON 文本。
- 不返回可选的 `structuredContent`。MCP `2026-07-28` 官方 Go SDK 会在结果中自动补充 `_meta.io.modelcontextprotocol/serverInfo`；统一结果构造器为其他三个 Adapter 设置同一值，避免字段体系分叉。
- `resultType` 是 MCP `2026-07-28` 的 wire-level 完成状态；当前普通调用固定为 `complete`。
- 成功时 `content.text` 直接承载 Ability 返回值，并明确返回 `isError=false`。
- 失败时 `content.text` 承载 `error_code` 和 `error_msg`，并设置 `isError=true`。
- `isError` 是四种 Adapter 的固定返回字段，不得因值为 `false` 而省略。
- 普通业务失败不得伪造空 token、空用户或假业务数据。
- Tool 执行中的 API 失败、输入失败和业务失败返回同一份 `CallToolResult` 并设置 `isError=true`；请求外壳损坏、协议不匹配或无法形成结果时才使用协议错误。

## 各通信协议

- MCP：由官方 Go SDK 原生处理 `server/discover`、`tools/call` 和 `CallToolResult`。项目只启用 `2026-07-28` stateless 请求，不声明已废弃的 roots、sampling、logging capability。
- JSON-RPC over HTTP：只有 `POST /` 承载 JSON-RPC 请求，HTTP `200` 的 `result` 直接放同一份 `CallToolResult`；`GET /` 返回 Home，`GET /robots.txt` 由独立 `RobotsHandler` 返回禁止索引规则，其余不匹配请求返回 Home。
- WebSocket：文本帧运输同一份 JSON-RPC Tool Call 请求和响应。
- gRPC：只提供一个 `APIService.Call` RPC。它使用原生 protobuf `CallRequest` 运输 `request_id`、`method`、`params`，使用 `CallResponse` 返回同一 `request_id` 和统一 `CallToolResult`。实际业务方法通过 `params.name` 扩展，不新增 RPC；协议错误使用 gRPC status。

协议 Adapter 只负责本协议连接、外壳解析和响应编码，不得复制 `tools/call` 内部提取、方法选择或业务执行。哪个 Adapter 调用 `APIExecuter`，统一结果就沿本次调用栈回到该 Adapter；协议层不设置独立结果查询接口、进程内结果缓存或跨协议返回分发。长任务的受理与进度、结果读取实现为 Ability 层普通注册方法，走同一条统一调用链，真相源是数据库任务记录。

gRPC 的 `.proto` 源只放在 `api/api_grpc/proto`，生成的 `.pb.go` 只放在 `api/api_grpc/protobuf`，并且只由根目录 `gen_proto.sh` 生成。新增 Ability 方法不修改 proto、生成脚本或 gRPC Handler。

## 方法注册与执行

`SupportedMethod` 同时保存方法说明和实际执行函数：

```go
type SupportedMethod struct {
    Name        string
    Description string
    InputSchema map[string]interface{}
    Async       bool
    Execute     func(context.Context, interface{}) (interface{}, error)
}
```

`InputSchema` 只用于 MCP `tools/list` 描述 `arguments`，不是 Ability 的业务执行参数。

`Async` 是后端注册方法时设置的执行语义，不属于前端参数，也不进入 `InputSchema`。默认值 `false` 同步调用 `Execute`；长任务设为 `true` 后由统一受理将输入写入 MySQL 持久化排队区并立即返回 `task_id`，Policy 再按空闲执行名额认领任务，并按方法名查回同一个 `Execute`。调用方通过 `task.get` / `task.list` 查询状态与结果，排队中任务可通过 `task.cancel` 取消。

`APIExecuter` 是唯一执行入口，但不维护业务 `switch`。它统一从外层 `method`、`params` 提取业务方法名和 `arguments`，完成可选编解码后，从有序方法目录取得注册项并调用该项的 `Execute`。方法描述和执行函数不得再维护两份映射。

Ability 继续只返回 `value/error`：

- 成功返回 `value, nil`。
- 可预期业务失败返回带业务码的 error。
- 数据库不可用、无法编码或内部状态损坏返回普通 error。

禁止引入 `Invocation`、另一层 Method Handler、`ExecutionResult`、返回路由器、跨协议广播、反射扫描、隐藏 `init()` 注册或代码生成框架。

## 父包带子包

API 启动时只调用 `ability.LoadAbilityAPIMethods()`：

1. `ability` 先注册 `test`，确保它始终是第一项。
2. `ability` 主动加载直属业务子包。
3. 每个业务父包继续主动加载自己的直属子包。
4. 子包在自己的 `LoadAPIMethods()` 中注册本域描述和执行函数，不反向 import 或调用父包。
5. 上层不得越级逐个装配叶子包。

`api_supported_methods` 只提供有序方法目录、按名称查找和共享结构，不声明具体业务方法，也不引用 `ability`。

## 通用错误码

```text
0      SUCCESS
10001  API_METHOD_NOT_FOUND
10002  API_METHOD_NOT_SUPPORTED
10003  API_INVALID_ARGUMENTS
10004  API_PERMISSION_DENIED
```

已注册但暂未实现的方法返回 `API_METHOD_NOT_SUPPORTED`。不存在或拼写错误的方法返回 `API_METHOD_NOT_FOUND`。
