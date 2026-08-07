# API 方法清单

> 本文件由 `gen_api_docs.sh` 从方法注册表生成（v0.0.11，共 15 个方法），禁止手改；新增方法后重新执行生成。

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

## auth.verify_code.send.email

发送邮箱验证码

`arguments` JSON Schema：

```json
{
  "additionalProperties": false,
  "properties": {
    "email": {
      "format": "email",
      "maxLength": 320,
      "minLength": 3,
      "type": "string"
    }
  },
  "required": [
    "email"
  ],
  "type": "object"
}
```

## auth.verify_code.check.email

检查邮箱验证码

`arguments` JSON Schema：

```json
{
  "additionalProperties": false,
  "properties": {
    "email": {
      "format": "email",
      "maxLength": 320,
      "minLength": 3,
      "type": "string"
    },
    "verify_code": {
      "pattern": "^[0-9]{6}$",
      "type": "string"
    }
  },
  "required": [
    "email",
    "verify_code"
  ],
  "type": "object"
}
```

## auth.verify_code.send.sms

发送短信验证码（暂不支持）

`arguments` JSON Schema：

```json
{
  "additionalProperties": false,
  "properties": {
    "phone": {
      "pattern": "^\\+[1-9][0-9]{7,14}$",
      "type": "string"
    }
  },
  "required": [
    "phone"
  ],
  "type": "object"
}
```

## auth.verify_code.check.sms

检查短信验证码（暂不支持）

`arguments` JSON Schema：

```json
{
  "additionalProperties": false,
  "properties": {
    "phone": {
      "pattern": "^\\+[1-9][0-9]{7,14}$",
      "type": "string"
    },
    "verify_code": {
      "pattern": "^[0-9]{6}$",
      "type": "string"
    }
  },
  "required": [
    "phone",
    "verify_code"
  ],
  "type": "object"
}
```

## auth.register

使用邮箱验证码注册账户；user_name 为注册必填主标识

`arguments` JSON Schema：

```json
{
  "additionalProperties": false,
  "properties": {
    "email": {
      "format": "email",
      "maxLength": 320,
      "minLength": 3,
      "type": "string"
    },
    "password": {
      "maxLength": 128,
      "minLength": 15,
      "type": "string"
    },
    "user_name": {
      "maxLength": 32,
      "minLength": 3,
      "type": "string"
    },
    "verify_code": {
      "pattern": "^[0-9]{6}$",
      "type": "string"
    }
  },
  "required": [
    "user_name",
    "email",
    "password",
    "verify_code"
  ],
  "type": "object"
}
```

## auth.login.email

使用邮箱登录

`arguments` JSON Schema：

```json
{
  "additionalProperties": false,
  "properties": {
    "email": {
      "format": "email",
      "maxLength": 320,
      "minLength": 3,
      "type": "string"
    },
    "login_method": {
      "enum": [
        "password"
      ],
      "type": "string"
    },
    "password": {
      "maxLength": 128,
      "minLength": 1,
      "type": "string"
    }
  },
  "required": [
    "login_method",
    "email",
    "password"
  ],
  "type": "object"
}
```

## auth.login.phone

使用手机号登录

`arguments` JSON Schema：

```json
{
  "additionalProperties": false,
  "properties": {
    "login_method": {
      "enum": [
        "password",
        "verify_code"
      ],
      "type": "string"
    },
    "password": {
      "maxLength": 128,
      "minLength": 1,
      "type": "string"
    },
    "phone": {
      "pattern": "^\\+[1-9][0-9]{7,14}$",
      "type": "string"
    },
    "verify_code": {
      "pattern": "^[0-9]{6}$",
      "type": "string"
    }
  },
  "required": [
    "login_method",
    "phone"
  ],
  "type": "object"
}
```

## auth.logout

撤销当前登录状态

`arguments` JSON Schema：

```json
{
  "additionalProperties": false,
  "properties": {
    "jwt_token": {
      "maxLength": 8192,
      "minLength": 1,
      "type": "string"
    }
  },
  "required": [
    "jwt_token"
  ],
  "type": "object"
}
```

## auth.jwt_token.check

检查 JWT token 并返回当前身份

`arguments` JSON Schema：

```json
{
  "additionalProperties": false,
  "properties": {
    "jwt_token": {
      "maxLength": 8192,
      "minLength": 1,
      "type": "string"
    }
  },
  "required": [
    "jwt_token"
  ],
  "type": "object"
}
```

## auth.jwt_token.refresh

轮换 refresh token 并签发新 JWT token

`arguments` JSON Schema：

```json
{
  "additionalProperties": false,
  "properties": {
    "refresh_token": {
      "maxLength": 8192,
      "minLength": 1,
      "type": "string"
    }
  },
  "required": [
    "refresh_token"
  ],
  "type": "object"
}
```

## user.nickname.change

修改当前用户昵称

`arguments` JSON Schema：

```json
{
  "additionalProperties": false,
  "properties": {
    "jwt_token": {
      "maxLength": 8192,
      "minLength": 1,
      "type": "string"
    },
    "nick_name": {
      "maxLength": 32,
      "minLength": 1,
      "type": "string"
    }
  },
  "required": [
    "jwt_token",
    "nick_name"
  ],
  "type": "object"
}
```

## task.get

查询我的异步任务状态与结果

`arguments` JSON Schema：

```json
{
  "additionalProperties": false,
  "properties": {
    "jwt_token": {
      "maxLength": 8192,
      "minLength": 1,
      "type": "string"
    },
    "task_id": {
      "maxLength": 36,
      "minLength": 36,
      "type": "string"
    }
  },
  "required": [
    "jwt_token",
    "task_id"
  ],
  "type": "object"
}
```

## task.list

列出我的异步任务（新→旧，最多 50 条）

`arguments` JSON Schema：

```json
{
  "additionalProperties": false,
  "properties": {
    "jwt_token": {
      "maxLength": 8192,
      "minLength": 1,
      "type": "string"
    }
  },
  "required": [
    "jwt_token"
  ],
  "type": "object"
}
```

## task.cancel

取消我的排队中任务（执行中任务不可取消）

`arguments` JSON Schema：

```json
{
  "additionalProperties": false,
  "properties": {
    "jwt_token": {
      "maxLength": 8192,
      "minLength": 1,
      "type": "string"
    },
    "task_id": {
      "maxLength": 36,
      "minLength": 36,
      "type": "string"
    }
  },
  "required": [
    "jwt_token",
    "task_id"
  ],
  "type": "object"
}
```
