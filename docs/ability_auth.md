[← 返回 README](../README.md)

# Auth Ability 契约

## 文档状态

本文定义 Auth 的方法、身份、验证码、JWT 和验收边界。

Auth 是 **API 准入域**：实现整体位于 `api/api_auth`（不在 `ability` 树内），涵盖通信之后的权限准入——验证码、注册、登录、session/JWT 与统一准入门禁；`ability` 只承载业务功能实现。文件名沿用 `ability_auth.md` 以保持链接稳定。

## 预期目的

提供真实可用、可失效、可审计且不泄露敏感身份信息的验证码、登录和 token 链路。Auth 负责认证编排；User 数据、密码变更和手机号绑定属于 User 域。

## 对外方法

```text
auth.verify_code.send.email
auth.verify_code.check.email
auth.verify_code.send.sms
auth.verify_code.check.sms
auth.register
auth.login.email
auth.login.phone
auth.logout
auth.jwt_token.check
auth.jwt_token.refresh
```

## 验证码

邮箱验证码方法：

- `auth.verify_code.send.email`：接收 `email`，发送验证码。
- `auth.verify_code.check.email`：接收 `email`、`verify_code`，检查验证码。

短信验证码方法：

- `auth.verify_code.send.sms`：当前明确返回 `API_METHOD_NOT_SUPPORTED`。
- `auth.verify_code.check.sms`：当前明确返回 `API_METHOD_NOT_SUPPORTED`。

验证码统一存入 `VerificationCode`，以规范化后的 `recipient` 绑定接收目标。Email 和 SMS 只区分发送通道，不拆分验证码表。验证码域实现收口在 `api_auth_verify_code` 子包：状态机（生成、限流、尝试次数、消费状态）按 recipient 键控、渠道无关；邮件投递收口在单一函数内按 `email.provider` 配置选择供应商，SMS 实施时复用同一状态机、只新增渠道编排与投递函数。

`check` 只检查、不消费，也不产生授权结果。检查失败必须扣减尝试次数；达到上限后锁定。`auth.register`、验证码登录等最终业务动作仍须重新校验并一次性消费验证码。

验证码不得明文落库、写日志或进入响应（唯一例外：Debug 运行模式可写 Debug 日志供本地联调，Release/Test 禁止），必须具备 TTL、尝试次数、发送间隔、小时发送上限和消费状态。

## 注册

`auth.register` 接收：

```json
{
  "user_name": "yeah_dev",
  "email": "user@example.com",
  "password": "...",
  "verify_code": "123456"
}
```

`user_name` 是注册必填主标识（全局唯一，规范化与校验见 `ability_user.md`）；email 为绑定形态，注册事务内经验证码完成绑定。注册必须在同一个 GORM 事务中校验并消费邮箱验证码、创建稳定用户身份。用户 ID 与用户名、邮箱、手机号无关。

## 登录

邮箱登录：

```text
auth.login.email
```

```json
{
  "login_method": "password",
  "email": "user@example.com",
  "password": "..."
}
```

手机号密码登录：

```text
auth.login.phone
```

```json
{
  "login_method": "password",
  "phone": "+8613800000000",
  "password": "..."
}
```

手机号验证码登录仍使用 `auth.login.phone`，但当前明确返回 `API_METHOD_NOT_SUPPORTED`：

```json
{
  "login_method": "verify_code",
  "phone": "+8613800000000",
  "verify_code": "123456"
}
```

邮箱密码与手机号密码认证失败必须使用相同业务错误，不得泄露身份是否存在。手机号密码登录只允许使用 `users.bind_phone` 中已验证的 canonical phone。

## JWT Token

- `auth.jwt_token.check`：接收 `jwt_token`，检查 token、session 和用户状态，返回当前身份数据。
- `auth.jwt_token.refresh`：接收 `refresh_token`，轮换 refresh token 并签发新 JWT。
- `auth.logout`：接收 `jwt_token`，撤销对应 session。

JWT 必须固定签名算法、issuer、audience、有效期和 token 类型。refresh token 只保存哈希，轮换和 logout 必须使用数据库事务或原子更新。

## User 边界

绑定手机号保存在 `users.bind_phone`，未绑定为 `NULL`。Auth 通过 User 包方法取得对应用户，不直接查询 User model。

手机号绑定、解绑和修改属于 User 域。完整边界见 [`ability_user.md`](ability_user.md)。

密码变更同样属于 User 域。Auth 不直接读写 User model，也不保存或暴露 `PasswordHash`；注册和登录只调用 User 包方法。

## 业务错误

Auth 现阶段不增加独立错误码区间或枚举映射，只复用 API 通用错误码。身份认证和 token 校验失败统一使用 `API_PERMISSION_DENIED`，验证码或账户输入不可用统一使用 `API_INVALID_ARGUMENTS`，不得通过错误码或消息暴露邮箱、手机号或账户是否存在。邮件供应商、数据库或内部状态故障属于服务错误，不伪装成业务成功。

已进入 `APIExecuter` 的可预期 Auth 业务失败必须通过统一 MCP `CallToolResult` 返回：`content` 中提供 `error_code` 和 `error_msg`，并设置 `isError=true`；不得使用协议错误冒充普通业务状态。

## 数据库与配置

- `db.GlobalMysqlCtl` 是唯一 MySQL 入口。
- Auth 查询和写入使用 `MysqlDB.WithContext(ctx)`；多步写操作使用 `Transaction`。
- 表结构以 Go model 为事实源并通过 `AutoMigrate` 同步。
- 不得通过另一连接池、原始 DDL、mysql CLI 或外部脚本修改表结构。
- 邮件通道供应商由 auth 配置 `email.provider` 选择，当前仅支持 `resend`（Resend 官方 Go SDK）；配置继续进入 `config.FileConfig` 的 YAML/JSON/TOML 体系。
- `email` 配置块的 `provider`、`api_key`、`from`、`product_name`（邮件文案品牌名）、`verification_subject` 均为强配置：缺失或非法在配置加载层显式拒绝，不在业务代码里兜默认值。
- Auth 配置校验失败输出字段级 warning 日志（指向 `auth_cfg.xxx.yyy` 完整键路径，敏感字段只报名不报值），便于快速定位配置问题。
- API key、密码、验证码和 token 不得写日志、落入错误响应或以明文保存。

## 验收边界

真实验收必须分别证明：

- 邮箱验证码由有效 Resend 配置成功送达。
- 邮箱验证码检查不会消费，最终注册会一次性消费。
- 注册创建 User。
- 邮箱密码登录成功。
- 已验证手机号密码登录成功。
- JWT check、refresh rotation 和 logout 失效真实可用。
- SMS 相关调用和手机号验证码登录返回 `API_METHOD_NOT_SUPPORTED`。
- 四种协议的业务失败均使用正常协议状态和相同业务返回值。

## 实现进度

状态按 `TODO → IN_PROGRESS → DONE → ACCEPTED` 流转；发生争议时进入 `DISPUTED`，对齐后回到 `IN_PROGRESS`。只有 `ACCEPTED` 可以打勾。

本文档随模板携带：实现代码已随模板就位（DONE），真实验收（邮件送达、登录链路等）须在实例化项目内按各自环境完成后方可 ACCEPTED。

- [ ] 🟡 DONE — Auth 配置、验证码哈希、JWT 基础实现已存在；密码哈希由 User 域持有。
- [ ] 🟡 DONE — verify_code 域包收口完成：`api_auth_verify_code` 持有 email/SMS 四方法，email provider 由 `email.provider` 配置选择（`EmailConfig` 含 `product_name` 品牌配置），配置校验失败有字段级 warning 日志。
- [ ] 🟡 DONE — 准入域整体上移：auth 六子包迁至 `api/api_auth`；APIExecuter 统一准入门禁（非 Public 方法 Execute 前验 `arguments.jwt_token`，身份经 context 下传）；业务方法删除自行鉴权样板；wire 与 `api_methods.md` 零变化。
- [ ] 🟡 DONE — Auth method 描述和实际 Execute 函数已统一注册。
- [ ] 🟡 DONE — 邮箱验证码 `send/check` 和注册消费语义已实现。
- [ ] 🟡 DONE — `auth.login.email`、`auth.login.phone` 登录语义已实现（login.phone 待手机号绑定实现后验收）。
- [ ] 🟡 DONE — `auth.jwt_token.check/refresh` 与 `auth.logout` 已接入。
- [ ] 🟡 DONE — Auth 业务失败已复用 API 通用错误码，并完成四协议 `CallToolResult` 验证。
- [ ] 🟡 DONE — 可选绑定手机号字段为 `users.bind_phone`。
- [ ] 🟡 DONE — 使用有效 Resend 配置完成真实邮箱主流程验收。
- [ ] ⬜ TODO — 使用真实已验证手机号完成密码登录验收。
