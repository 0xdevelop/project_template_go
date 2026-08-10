[← 返回 README](../README.md)

<!-- TOC -->

- [1. User 域定位与边界](#1-user-域定位与边界)
- [2. 方法契约](#2-方法契约)
    - [2.1. user.nickname.change](#21-usernicknamechange)
- [3. 数据与并发纪律](#3-数据与并发纪律)
- [4. 验收边界](#4-验收边界)
- [5. 实现进度](#5-实现进度)

<!-- /TOC -->

# 1. User 域定位与边界

User 域持有 User model、用户查询和密码处理。Auth 只调用 User 包方法，不直接操作 User model；Auth 注册在同一个 GORM transaction 内调用 User 创建方法，邮箱和已绑定手机号的密码校验也通过 User 包完成。

方法归属：`PasswordChange`、手机号绑定/修改和其他用户资料操作属于 User 域，具体绑定流程未确认前不提前实现。组织/团队类成员关系操作**不属于 User 域**——它们写的是对应业务域的表，按「方法写谁的表就归谁的域」归对应业务域；用户主动发起只是鉴权上下文。

本域无独立业务流转，方法为即时读写，不设流转图。

# 2. 方法契约

受保护方法 wire 必传 `arguments.jwt_token`，由 API 门禁统一验证并消费（验证后即移除），业务层零感知；入参 schema 权威见 [`api_methods.md`](api_methods.md)。

## 2.1. user.nickname.change

修改当前用户昵称。

```json
{ "nick_name": "新昵称" }
```

- trim 后 1–32 字符；注册于 `ability_user_profile` 子包（因跨域鉴权依赖由 `ability` 顶层装配，契约明记例外）。

# 3. 数据与并发纪律

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

跨域出口：`FindByUserID` / `FindByUserIDs`（批量，供业务域成员列表类组合查询）——调用方拿 model 只读展示字段，不得回写。

# 4. 验收边界

真实验收必须分别证明：

- 注册创建 User：`bind_email` 与 `email_verified_at` 同事务写入、`nick_name` 默认取 `user_name`、`bind_phone` 为 NULL。
- 邮箱密码、已绑定手机号密码校验经 User 包完成且失败不泄露身份存在性。
- `user.nickname.change` 生效且长度约束成立。

# 5. 实现进度

状态按 `TODO → IN_PROGRESS → DONE → ACCEPTED` 流转；发生争议时进入 `DISPUTED`，对齐后回到 `IN_PROGRESS`。只有 `ACCEPTED` 可以打勾。

本文档随模板携带：实现代码已随模板就位（DONE），真实验收须在实例化项目内按各自环境完成后方可 ACCEPTED。

- [ ] 🟡 DONE — User model、用户创建和邮箱密码校验已实现。
- [ ] 🟡 DONE — 已绑定手机号密码查询使用 `users.bind_phone`。
- [ ] 🟡 DONE — `FindByUserIDs` 批量出口供跨域成员列表类组合查询。
- [ ] ⬜ TODO — 实现并验收手机号验证与绑定。
- [ ] ⬜ TODO — 对齐并实现 `PasswordChange`。
