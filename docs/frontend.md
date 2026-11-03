<!-- TOC -->

- [1. 前端定位与边界](#1-前端定位与边界)
- [2. 配置与启动](#2-配置与启动)
- [3. 构建与发布](#3-构建与发布)
- [4. 验收边界与实现进度](#4-验收边界与实现进度)

<!-- /TOC -->

# 1. 前端定位与边界

`frontend` 提供 TypeScript/Bun 页面壳子，示例文字为 `this is example frontend`。Go 只托管构建后的静态资源，不提供业务代理或另一套 API。

# 2. 配置与启动

配置沿用 `config.FileConfig` 的 YAML/JSON/TOML 装配：

```yaml
frontend_cfg:
  enabled: true
  bind_address: "127.0.0.1"
  port: 13006
```

缺少配置段或 `enabled: false` 时不监听，并记录对应停用原因；启用时地址和端口必填，非法配置在加载时拒绝，端口占用直接返回错误。新生成的默认配置和发布配置示例包含此段；既有本地配置由维护者显式加入，不自动回写。

`main.go` 在数据库初始化和迁移成功后启动前端，退出时关闭前端监听。页面只提供构建产物，未知路径和非 GET/HEAD 请求返回中性空响应。

# 3. 构建与发布

`frontend/package.json` 的 `build` 使用 Bun 原生构建 TypeScript/CSS，无第三方前端依赖。`frontend/dist/` 不入仓，由 `//go:embed` 随 Go 二进制发布，部署时不依赖 Bun 或源码目录。

GoLand 共享 Run/Debug 配置和 `build.sh` 均在 Go 编译前构建 `test_ui/web` 和 `frontend`。命令行运行：

```bash
./gen_test_ui.sh && ./gen_frontend.sh && go run .
```

共享配置中的 `NpmBeforeRunTask` 是 JetBrains 固定任务标识，两个构建任务均通过 `package-manager: bun` 明确使用 Bun。GoLand 的构建输出位于启动前任务控制台；服务启动日志中的 `Frontend Run On` 仅在 `frontend_cfg.enabled` 时出现。

2026-11-03 用户要求参考 Cursor 会话 `cc85014d-c5c3-4ed9-9859-7e9ed65e99eb` 回灌通用能力：沿用二进制嵌入、配置控制和页面地址日志；测试台监听字段直接归现有 `config.TestUIConfig`。该会话原话为「在之前的 config 基础上增加字段不就行了」。

# 4. 验收边界与实现进度

- [x] ✅ ACCEPTED — TS/Bun 页面壳子、前端配置域、静态资源嵌入与既有构建入口已完成本机验收。
- [x] ✅ ACCEPTED — 配置关闭、非法配置、端口冲突、HTTP 资源访问和监听关闭回归通过；ego lite 实测页面渲染、根路径刷新、JS/CSS 加载及源码路径空响应通过。
- [x] ✅ ACCEPTED — 本地 SQLite 环境中，原 GoLand 配置完成主进程启动、前端地址日志和正常退出验收；退出码为 0，全部监听端口释放。MySQL/PostgreSQL/Redis 的真实业务验收不在此结论内。
