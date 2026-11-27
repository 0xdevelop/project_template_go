# 验证码 INFO 日志与可选邮件

适用于现有 `auth.verify_code.send.email`，不新增投递模式、日志开关或新的控制接口。完整账户与消费契约见 [ability_auth.md](ability_auth.md)，方法结构见 [api_methods.md](api_methods.md)。

## 行为与响应

- 每次**真实生成**验证码后，立即调用服务 INFO 日志，不受 Debug/Test/Release、邮件开关、配置是否齐全、供应商是否可用或发送结果影响。日志先于后续存储/邮件操作，不因发送失败漏打。
- 日志只带随机 `delivery_id`、真实码、计划过期时间和 `validity=pending`；不带完整邮箱、用户名、API key 或供应商错误正文。日志是用户授权的服务端运维信息，必须限制权限与保留周期；不得放到 API、聊天、截图或测试输出。
- **生成日志不等于邮件已发，也不等于该码已有效。** 非法请求、频控或生成前依赖错误没有生成码，不制造日志码。
- 成功响应中 `code_logged=true` 表示已记录 INFO；`sent=false` 表示邮件未启用或未配置齐全，`sent=true` 仅表示已启用且完整配置的邮件由供应商调用成功确认受理，不宣称实际收件箱送达。另有 `delivery_id` 和 `expires_in`，没有明文码。
- 邮件配置完整并启用时，最终校验/供应商调用失败明确返回 `API_VERIFY_CODE_DELIVERY_FAILED`，不降级为成功，不因为日志存在返回“邮件已发”。存储故障依现有协议返回服务错误。

## 配置与迁移

```yaml
auth_cfg:
  email:
    enabled: false # 仅关闭邮件，不能关闭验证码 INFO 日志
    provider: "resend" # 供应商枚举值；能力配置键仍是 email
    api_key: ""
    from: ""
    reply_to: ""
    product_name: "your_service"
    verification_subject: "your_service verification code"
  verification:
    send_interval_seconds: 60
    # 其余 TTL、尝试次数、小时上限、hash secret 沿用现有必备配置。
```

以上仅为相关片段，完整示例见 `example_files/config_example.yaml`；不是可直接替代完整配置的文件。

- 2026-11-27 用户明确「邮件未配置就日志中拿验证码」「未配置日志中提示一下即可」：没有 `email` 块，或供应商、凭证、发件人、品牌、主题任一必填项未填写时，不阻断启动，日志明确提示未配置及缺失字段，不调用供应商；原验证码生成、INFO 日志、持久化与激活照常。此规则同样适用于旧块和显式 `enabled: true` 但未配齐的块，不修改现场配置内容。
- 新配置仍显式写 `email.enabled: false`；启用写 `true`。显式 false 一律不发信；完整旧块未写 `enabled` 时保留发送意图。已填写字段的枚举、地址与 Header 语法仍严格校验，非法值不能当作未配置。
- 邮件配置齐全且未关闭时，加载期与最终发送前两层校验继续生效。凭证的实际授权与发件域权限仍由供应商 API 判定，失败如实返回。
- `enabled=false` 可保留空凭证，不调用供应商；已填写字段的类型、地址/凭证/Header 语法和供应商枚举仍须合法，只有依赖字段是否必须填写随开关分层。布尔值、字符串、整数按各格式严格检查；拒绝空/数字布尔值、未知键、非法枚举、错误类型、负数/零/浮点冷却值和 Header 注入，不进行字符串到布尔值等隐式转换。
- 通用冷却键为 `verification.send_interval_seconds`。旧 `resend_interval_seconds` 仅作迁移别名（原意是重新发送间隔，不是供应商选择）；二者同时存在必须相等且为正整数，否则加载失败。
- 不存在 `verification.delivery_mode`，也不存在验证码 INFO 日志 `enabled`。`resend`/`resen` 不是邮件能力配置块名，拼错/未知键明确报错，不静默忽略。
- 不自动修改现场 `example_files/config_local.yaml`。运维需按自身邮件意图自行迁移，禁止把真实凭证贴到报告。

## 存储、失败与安全边界

1. 输入先校验并规范化；事务内检查冷却及小时上限，再使用现有安全随机生成器真实生码并立即 INFO 记录。事务内仅存 HMAC hash、期限、尝试预算与频控信息，初始 `sent_at=NULL`。
2. 邮件未配置齐全或已关闭时，在成功持久化后激活；邮件配置齐全且开启时，最终边界检查 context、开关、供应商、凭证、From/Reply-To、收件人、码格式和 TTL，只有供应商调用成功后才尝试激活。
3. `sent_at` 沿用既有字段作**可消费门控时间**，不作为邮件已发送的凭据；邮件结果看响应 `sent`。检查和注册都要求非空门控时间与有效 hash，不能凭日志消费 pending 码。
4. 邮件失败使用脱离请求取消、带 5 秒上限的清理 context，只清除该行该 hash 且仍未激活的码，不删除频控额度，不影响后续更新的码。若清理存储也失败会显式告警，`sent_at=NULL` 仍阻止消费；不得谎称清理成功。事务提交/后续激活失败也可能已存在 INFO 日志，该日志不保证有效。
5. 原有 TTL、错误尝试扣减、冷却、小时发送上限、hash、检查不消费、注册事务内一次性消费及账户枚举防护保持。短信仍不支持，不制造短信验证码。

## 可重复验证

```bash
go test ./api/api_auth/... ./config
```

表驱动测试捕获 INFO sink，只在内存比对真实码与数据库 hash/模拟邮件内容，断言消息不输出码。覆盖无邮件、显式关闭、邮件成功/失败、残缺依赖、非法枚举/类型、别名迁移、回滚仍记日志、pending/失败码不可用、频控不生码、TTL/错误预算以及注册单次消费。`TestIsolatedHTTPVerificationWorkflows` 使用临时 SQLite 和真实 loopback HTTP，走生产 JSON-RPC Handler → APIExecuter → 注册方法；仅邮件供应商和 INFO sink 是本地替身。它不等于真实供应商发信或收件箱送达验收，也不触发对外邮件。

## 本地实测（2026-11-27，Asia/Shanghai）

- 原 GoLand `project_template_go-debug` 于 12:01:58 启动 PID 52987；12:01:59 日志确认邮件凭证未配置，仅告警，SQLite 迁移与 API、测试台均启动成功。现场配置未改动；实际测试台为 `127.0.0.1:7099`。
- ego-browser 经测试台代理调用真实 JSON-RPC `auth.verify_code.send.email`：`sent=false`、`code_logged=true`，响应无验证码。只在内存读取对应服务 INFO 日志中的码，再调用原 `auth.verify_code.check.email`，返回 `valid=true`；SQLite 只读核对该行已有激活时间与 hash。没有外发邮件或新建账户，活跃用户仍为 0。
- 测试台三类筛选与实际注册表一致：公开 9、受保护 12、允许 API key 7；公开方法携带无效凭证仍成功，四协议受保护调用均返回权限业务错误。`/docs_api` 已真实渲染 21 项准入说明。
- `go test -mod=readonly -race ./...`、Bun 构建及差异格式检查通过。有效 JWT/API key 的允许与拒绝矩阵、异步受理前拒绝及受理记录去凭证，由隔离 SQLite 调用链回归证明；现场没有可登录账户，本次没有新建账户来补此项现场验收。
- 以上是开发环境联调；未执行发版、GitHub Release 资产安装、systemd 或真实邮件供应商验收，不构成产品安装验收。
- 联调后已停止本次从停止态启动的服务，复查 PID 与 7095–7099、13006 端口均释放，浏览器任务空间已关闭。
