> 最终审查：SHA256可由明确标注的操作者本地内容基准提供，并非必须官方签发。实例生产默认/来源与迁移边界以protenix_build/docs/resource_final_review.md为准。本轮不扩组件功能。

> M2 续做（2026-11-23 13:37 UTC+8）：现已提供 common/resource.Controller + RunCLI、resource.status / resource.prepare（原鉴权与持久任务）、数据快照与集合锁；主程序所有模式的配置/日志及相对存储资源锚到二进制，不再使用 debug cwd/受保护 config_local 的隐式回退。下面 M1 为保留的历史记录，最终实测结果见末尾。模板不含任何模型/URL/角色映射知识。

# 通用资源准备契约（本地实现里程碑）

2026-11-23 12:36 UTC+8。能力位于 common/resource，不属于 task 引擎；此模块不含任何模型知识。实例模型清单/URL/条件资源集由实例自己定义。资源与应用路径在 common/app_path 按可执行文件目录解析，不能依赖 cwd。

File: path、URL、size、可信 sha256。Config 显式 enabled；Download.Options 显式 download_enabled / retry_enabled / retry_count（首次之外次数，0 不重试）、parallelism（1-16）、chunk_size。Confirmed 为调用方确认结果，不从配置反序列化。缺资源在无 TTY 场景返回 ErrConfirmationRequired，不能阻塞 stdin；CLI/前端确认 control path 尚未接入。Progress 是真实下载字节，不得写成推理 progress。

Prepare：校验已有路径（包括目录软链接），成功无网络 -> 缺失检查确认 -> canonical 路径 flock 跨进程所有权 -> HEAD/强 ETag -> Range 并发分片 -> 中断保留分片/实体变化丢弃 -> SHA256/length -> sync + 同文件系统硬链接 no-replace 原子发布（不是覆盖式 rename）。Range 忽略不追加，单流重下；缺强 ETag 单流不续传；HEAD 不支持/无长度安全拒绝。现有不匹配资源拒绝覆盖。没有可信摘要就拒绝，不把非空当 ready。WithReady 只在传入集合全部 ready 后执行消费回调；集合完整性是实例职责。

已通过本地 -race 测试：两请求共享下载、跨进程互斥、有界分片、断线续传、实体变更重下、Range 忽略降级、hash 错不发布、取消/未确认不调用消费者、软链接 ready 零网络、不同 cwd 可执行路径解析。不是正式安装或科学验收。

Release main 已从二进制目录定位 conf/config.yaml，SQLite 相对路径同根；debug go run 仍保留历史模式。确认状态持久化/CLI确认适配器、安装器统一接入、全部上游资源清单/跨平台锁不属于这个已测里程碑的完成声明。原 product_acceptance 闭环仍然有效；没有安装/启动/业务/卸载证据不能称产品完工。

工作流：目标无本地 .auto_coding 状态，宿主未发现 xjb_code connector，本次仅 markdown 续做，不重新初始化或伪造服务端 session/task。实例详细上游证据与后续断点在 protenix_build/docs/resource_preparation_contract.md。

## 实测命令与断点

Go 1.26.10 arm64（本机现有 toolchain），GOTOOLCHAIN=local、GOPROXY=off、GOSUMDB=off；GOCACHE 与 TMPDIR 均在本项目 tmp/resource-tests 下。
- `go test -race ./common/... ./config`：PASS（最终源码，2026-11-23 UTC+8）。
- `go build -o tmp/resource-tests/project_template_go .`：PASS。
- 新增强 ETag 中途变化拒绝发布、retry_enabled=false 只尝试1次 / true+retry_count=2 尝试3次测试：PASS。
- 既有资源校验时的 SHA256 读取与合并阶段尚非逐块 context-aware，取消可能等本地 IO 完成；消费者门禁会复查取消，不将该延迟隐藏。

M1（通用组件与实例假 CLI 门禁）本地测试里程碑已完成；产品 acceptance 仍未完成。下一步核对实例证据，补真实资源清单与可审计确认 control path，不要求用户代做验证。

## M2 最终实测补记（覆盖上文M1历史限制）

2026-11-23 13:37 UTC+8。配置→显式确认→跨进程单准备者→完整校验ready→CLI门禁已形成可测试纵向闭环。原鉴权/持久任务承载resource.status/resource.prepare，无新增服务或task进度协议；原生resource CLI在正常App启动前运行，不打开数据库/服务。所有模式均以二进制目录为配置/日志/自有相对资源根；wrapper相对资源/cache以runtime根解析，仅上游子进程env，不写全局环境。

- 全量最终 `go test -race ./...` 与build均通过：模板 本地测试终端记录，实例 本地测试终端记录。实例的真实原生PTY/跨OS进程CLI smoke也通过，结果 `protenix_build/tmp/resource-tests/native-cli-result.json`。
- HEAD不支持/无长度但manifest有可信长度可安全有界单流GET；Range不可靠时不续拼。取消检查已进入本地hash/合并。已存在但无效文件拒绝覆盖；整集合预检先于首个网络请求。
- 新增普通tar来源能力：可信归档摘要与成员摘要分别验证，共享同族归档payload锁/缓存，普通regular成员按指定路径读取而非通用目录解压，验证后原子no-replace发布；隐式归档缓存不跟随软链接。缓存/分片会增加峰值磁盘，不支持gzip，不编造真实AF2摘要。
- 快照是最后观测而非进程存活；集合确认绑定集合名和完整配置，配置/目标改变使旧确认失效。不同canonical目标/硬链接别名不承诺全局内容去重，已有硬链接/外部任意突变不是安全沙箱保证。
- 实例的有限默认v1/no-MSA与preview/no-MSA资源知识及Python自动下载分支、生产清单信任缺口详见protenix_build/docs/upstream_resource_matrix.md；未核查模式显式阻止，不是opt-in绕过。任意自配外部CLI只有显式manifest门禁，不等于官方Python防重证明。
- 原历史v0.0.2资产不可变，未重新打包/安装，不能宣称旧包已获得新行为。前端展示适配后续可接原Bun+TS，不改视觉；确认准备不会暗中重跑先前失败推理任务。
- 未执行sudo/systemd/安装/卸载/大权重下载/GPU/commit/push/tag，未修改受保护配置/build_srcs/show_yeah。保留原README及原官方资产验收计划。

### 追加代码审查回归

恢复后审查发现：Range恢复时，旧staging中的manifest/part/complete如果是软链接或硬链接，原来的普通open/append/truncate可能跟随并破坏无关文件。现已对内部staging采用O_NOFOLLOW、先fstat确认regular且单链接再truncate/append，O_NONBLOCK避免FIFO阻塞；真实配置资源路径仍允许目录/文件软链接只读复用，不把两者混淆。补6组软/硬链接回归验证无关文件内容不变且不发布，以及资源/内部文件FIFO非阻塞拒绝。不是对同UID恶意并发任意改目录的完整沙箱承诺。

### 最终恢复断点 2026-11-23 13:48 UTC+8

上述staging安全修正后的最终全量race/build也PASS：模板 本地测试终端记录，实例 本地测试终端记录（含最新真实PTY/跨进程CLI smoke）。最终补丁为各项目 `tmp/resource-tests/resource-preparation-final.patch`，审查清单 `tmp/resource-tests/review_manifest.json`；不含原README改动/原assets验收计划。当前模型生产默认清单尚无已确立的真实性来源，不自动补假摘要；已授权的本地权重快照仅用于只读内容相等性实验。后续仍需新assets正式安装/业务/卸载验收及明确清单信任边界，不重打旧tag。
