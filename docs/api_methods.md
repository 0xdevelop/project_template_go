# API 方法清单

> 本文件由 `gen_api_docs.sh` 从方法注册表生成（v0.0.8，共 1 个方法），禁止手改；新增方法后重新执行生成。

## 统一调用方式

所有方法经同一入口调用，JSON-RPC over HTTP、MCP、WebSocket、gRPC 只是外壳不同，
运输同一份请求和同一份结果。业务方法由 `params.name` 选择，业务入参放在
`params.arguments`：

```json
{
  "jsonrpc": "2.0",
  "id": "request-id",
  "method": "tools/call",
  "params": {
    "name": "auth.login.email",
    "arguments": {
      "email": "user@example.com",
      "password": "password"
    }
  }
}
```

统一返回 MCP `CallToolResult`：成功 `isError=false`，`content[0].text`
直接承载业务 JSON；业务失败（含未注册方法、参数不合法、鉴权失败）HTTP 状态仍为
200，`isError=true`，`content[0].text` 为
`{"error_code":...,"error_msg":"..."}`。

### 业务错误码

| error_code | error_msg |
| ---------- | --------- |
| 0 | 成功 |
| 10001 | method not found |
| 10002 | method is not supported |
| 10003 | invalid arguments |
| 10004 | permission denied |

## test

检查统一 API 调用链是否可用

`arguments` JSON Schema：

```json
{
  "additionalProperties": false,
  "type": "object"
}
```
