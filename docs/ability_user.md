[← 返回 README](../README.md)

# User Ability 契约

## 当前边界

User 域持有 User model、用户查询和密码处理。Auth 只调用 User 包方法，不直接操作 User model。

当前 `users` 字段：

- `id`：`gorm.Model` 自带行主键，仅数据库内部使用，不进业务代码、JWT 或响应。
- `user_id`：稳定业务用户 ID，UUID `char(36)`，创建用户时用 `uuid.NewString()` 生成；JWT claims、session、组织等跨域引用一律使用它。
- `user_name`：注册必填主标识，全局唯一；统一规范化（trim + 小写），`^[a-z][a-z0-9_]{2,31}$`；**不提供修改接口**。
- `nick_name`：昵称，创建时默认取 `user_name` 的值，经 `user.nickname.change` 可改（trim 后 1–32 字符）。
- `bind_email`：绑定邮箱，可空唯一；注册经邮箱验证码在同一事务内完成绑定，注册后必非空。不变量：`bind_email IS NULL ⟺ email_verified_at IS NULL`。
- `bind_phone`：可选绑定手机号，未绑定为 `NULL`，唯一。
- `password_hash`：密码哈希，JSON 序列化排除（`json:"-"`），不得进入任何响应。
- `email_verified_at` / `phone_verified_at`：各自与 `bind_email` / `bind_phone` 同生共死——绑定事务一起写入，解绑一起清空，换绑重新验证后写新值。登录判断只看 bind 字段非空，验证时间仅作审计。
- 以及 `gorm.Model` 默认的 `created_at`、`updated_at`、`deleted_at`。

不创建 UserProfile model 或表。手机号只有验证完成后才能写入 `bind_phone`。

对外方法：`user.nickname.change { jwt_token, nick_name }`——注册于 `ability_user_profile` 子包（因跨域鉴权依赖由 `ability` 顶层装配，契约明记例外）。

## 方法归属

`PasswordChange`、手机号绑定/修改和其他用户资料操作属于 User 域。具体绑定流程未确认前不提前实现。

组织成员关系操作（加入、退出、被踢、转让）**不属于 User 域**——它们写的是 `organizations` 表，按「方法写谁的表就归谁的域」归 Org 域（判定见 [`ability_org.md`](ability_org.md) 功能域归属节）；用户主动发起只是鉴权上下文。

Auth 注册在同一个 GORM transaction 内调用 User 创建方法；邮箱和已绑定手机号的密码校验也通过 User 包完成。

## 实现进度

- [ ] 🟡 DONE — User model、用户创建和邮箱密码校验已实现。
- [ ] 🟡 DONE — 已绑定手机号密码查询使用 `users.bind_phone`。
- [ ] ⬜ TODO — 实现并验收手机号验证与绑定。
- [ ] ⬜ TODO — 对齐并实现 `PasswordChange`。
