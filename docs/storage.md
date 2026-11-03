# 1. 程序固定存储

`storage` 管理程序启动配置里的固定存储，每种引擎一个实例，MySQL、SQLite、PostgreSQL、Redis 可同时启用。连接配置沿用对应 gtbox SDK，配置层负责现有键名映射。

```yaml
storage_cfg:
  primary: sqlite
  storage_sqlite:
    enabled: true
    path: ./data/project_template_go.db
  storage_mysql:
    enabled: false
```

`primary` 只选择一个已启用的关系库，取值为 `mysql`、`sqlite`、`postgresql`。只开一个关系库时可省略；开多个时必须显式指定。指向关闭的库、未知引擎或多库未指定主库，均在打开连接前拒绝。

Auth/User 经 `GormDB(ctx)` 使用主库；指定引擎使用 `MysqlDB(ctx)`、`SqliteDB(ctx)`、`PostgreSQLDB(ctx)`。Task 按既有 MySQL 8.0+ 契约固定使用 `MysqlDB(ctx)`；未启用 MySQL 时异步受理返回明确错误。

Redis 经 `Redis()` 取句柄；`storage_redis.database: true` 时可用 `RedisDatabase()`。Redis 不参与关系库主库选择。

`OpenStorage` 重复打开会拒绝替换在用连接池。`GormAutoMigrate` 同步已打开关系库的 model；任务载荷的 GORM `size` 声明在 MySQL 仍生成 `mediumtext`，在 PostgreSQL/SQLite 生成 `text`。旧根配置 `mysql_cfg` 的数字端口、字符串端口及 `UTC`、`Asia/Shanghai`、`Asia%2FShanghai`、数字时区继续由配置层映射；非法时区明确报错。

退出顺序由 `main` 负责：关闭前端和 API、停止 Policy 派发并等待在途任务、关闭固定存储。存储句柄读取与打开、关闭使用同一读写锁。

# 2. Ability 独立实例

2026-11-03 本会话用户明确：「storage 是程序本身用的，ability 动态处理的那是基于 gtbox_xxx 新建的实体去实例化处理的」「默认有全局，也能支持独立创建」。

Ability 运行期处理外部数据源时，直接调用对应 gtbox SDK 的 `New`，显式传入连接参数，持有独立实例并负责 `Close`。它可以同时持有多个 MySQL、Redis 或其他引擎实例，与程序固定存储的生命周期互不接管。

连接元数据是否入库、存入何种业务 model，按具体 Ability 的实际需求决定；模板当前没有实现动态数据源管理域。

# 3. 验收边界

配置解析、驱动字段类型、SQLite 存量迁移、关闭并发与端口冲突属于不同验证路径。MySQL/PostgreSQL/Redis 的真实连接、多库并存及业务结果需在相应实例环境单独验收。
