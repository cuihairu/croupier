# Croupier 深度审核（2026-11-12）

审核基线：`7247a5db4`（完整 commit 与关键文件 SHA-256 见 [源码指纹](evidence/2026-11-12/source-sha256.json)）。本轮为七个分区的风险路径审核，结合源码追踪、隔离复现和原有测试；**不是全仓所有文件逐行审查，也不是生产验收**。

业务边界：一家游戏公司的多个游戏及环境；检查账号权限、`gameId/env` 作用域、全局管理操作及物理数据库路由。此前台账误用了“租户”，本轮已纠正，不引入该概念或新业务模型。

结论：**需要修复**。当前台账中的 AUD-001..018 含 **17 项已确认问题、1 项待 CI 工具链复核的兼容性风险**；另外复核了已有 PWR-001..006。合计 **23 项确认问题（P1 17 / P2 6）及 1 项待验证风险**。其中 15 项问题已动态复现，其余 8 项有静态调用链证据。本轮新增 AUD-007..018 共 12 项，保留原编号，未把已有 PowerOps 问题重复编号。

本轮没有修改业务实现、版本号、用户的移动端锁文件和分析配置；交付为报告、台账和复现证据。已有测试通过不代表下面问题已修复。

## 审核覆盖与边界

| 分区                           | 实际检查路径                                                                                                                                                | 本轮结果与未验证部分                                                                                                                                                  |
| ------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| AUD-COV-001 API / 权限         | 全局与 scoped 路由装配 → JWT/MFA → `GameDBMiddleware` → admin/backup/storage/powerops handler、service、model；`RequireAnyPermission`、函数 Govern/风险策略 | 确认 AUD-007/008/010/016/017。游戏作用域校验不能替代全局操作授权。没有逐个执行全部 REST 端点                                                                          |
| AUD-COV-002 存储 / 任务        | `dbctx`、Router 绑定解析/缓存/首开/关闭、迁移版本与锁、任务事务事件/收割、scheduler 领取与过期处理、缓存、analytics MQ 接口、真实备份执行器                 | 确认 AUD-009/015；复核 PWR-001..006。没有真实 MySQL/Postgres/Redis/ClickHouse 故障恢复测试。仓内不存在原台账写的 `internal/queue`，实际入口为 `internal/analytics/mq` |
| AUD-COV-003 Agent / 协议       | Server/Agent/Provider 握手、token 校验、TLS 配置与装配、帧长度、双车道、RPC 响应、关闭与 Stop；Agent 注册 handler 锁和 mux.Wait                             | 确认 AUD-001/002/004；旧“读循环同步执行 handler”已失效。未做真实网络拥塞和全部 Agent 角色长跑                                                                         |
| AUD-COV-004 六语言 SDK / 插件  | 六语言 Provider 鉴权注入、默认 transport 创建、TLS、双车道与错误回帧；Go SDK、三语言 golden；hostdriver 探针凭据；插件边界守卫                              | 确认 AUD-005/013/014/018。未运行 Java/C#/C++ 完整构建和全套插件故障测试；C++ 单独编译实际响应 writer 做确定性短写复现                                                 |
| AUD-COV-005 Web                | scope store → 请求头注入 → HTTP/SSE；scope 重载 hook；登录与 token 存储；历史 Bugs/Tickets/Alerts/Tools/navigation URL sink                                 | 确认 AUD-012；C-005 仍在。运行 TypeScript 和 3 个 Jest 套件；未执行全部页面、浏览器 E2E 或新的视觉/a11y 验收                                                          |
| AUD-COV-006 Mobile             | session secure storage、Dio token/scope 注入与 401、scope controller、analytics/approvals 请求生命周期、App 登出路径、原生配置抽查                          | 确认 AUD-003。真实 AnalyticsController 的乱序请求已复现；未运行 CI 固定 Flutter、原生构建或实机安全验证                                                               |
| AUD-COV-007 构建 / 发布 / 运维 | release/nightly/sdk matrix/SDK CI/mobile CI、版本守卫、Compose/Server/Agent 配置、备份、文档和治理文件                                                      | 确认 AUD-011、保留 AUD-006 风险；版本/页面守卫、Go 编译、Web tsc、docs build 已运行。未触发远端流水线、发布、签名或部署                                               |
| AUD-COV-008 历史复核与总报告   | 11 月 1 日 C-001..032、11 月 4 日外部 35 条及 WIP 落地记录，结合 11 月 10 日 CR-FIX 台账                                                                    | 逐项口径见后文；明确区分源码已修、部分修复、仍存在和本轮无法验证的外部事实                                                                                            |

上表的完成只表示本轮列出的风险路径已审查、发现已报告。没有把未测试的子系统或生成代码标成“无问题”。

## 当前问题

### AUD-007 · P1 · 全局备份接口仅检查登录身份

证据：`internal/handler/routes.go:114`、`internal/handler/routes_business.go:66-74`；`internal/api/backup/handler.go` 只绑定 DTO；`service.go:37/75/151/177` 没有角色、权限或可见范围校验。全局备份组也不经过游戏作用域中间件。

动态复现：创建真实的零角色账号并签发 JWT，经实际 AuthMiddleware 调用备份列表、下载、删除，均返回 200；下载内容为夹具中的私有数据库字节，删除后记录确实消失。该测试使用本地临时文件，不是生产数据泄漏测试。

影响：普通登录账号可读取和删除已生成的全局备份；Create 的执行器使用 Server 配置的数据源，不能把这个接口当成普通游戏只读入口。修复应在服务边界核验平台备份管理权限，并覆盖列表、创建、下载、删除全部路径；低权限测试应断言 403 及无数据库/对象存储副作用。

### AUD-008 · P1 · 仅设置生产模式无法阻止公开 JWT 开发密钥回退

证据：`internal/security/jwtutil/secret.go:44-80`。`isDevelopmentMode` 只识别配置中的开发模式；`server.mode=prod/production/release` 本身不会返回 false，若环境未另设 `CROUPIER_MODE=prod/production`，最后仍返回 true。空密钥被解析为公开的 `DevSecret()`。

动态复现：清空相关环境覆盖，分别传入上述三个 Server.Mode，全部成功返回开发密钥。原有“生产模式报错”测试同时设置了环境变量，未测出仅 YAML/配置指定生产模式的分支。

影响限于未显式配置密钥的该类部署：可导致可预测的 JWT 签名密钥，从而允许伪造身份。修复应让明确生产配置拒绝空密钥，并覆盖配置与环境组合；不得把未知运行模式默认为允许公开密钥。

### AUD-009 · P1 · SQLite 备份静默丢失 WAL 中的已提交数据

证据：`internal/platform/backupexec/executor.go:227-242` 直接 `os.Open(e.dsn)` 复制主文件；没有 SQLite 快照、backup API、`VACUUM INTO` 或覆盖 WAL 的一致性协议。普通文件读取不会取得 SQLite 数据库快照。

动态复现：真实 SQLite 建表后开启 WAL、关闭自动 checkpoint、提交一行；调用实际 `dump` 成功返回文件和 64 位 SHA-256，但打开备份后行数为 0，源库为 1。

影响：checksum 证明被复制的字节未改变，无法证明备份包含已提交事务。在线文件复制还存在并发写入一致性风险，本轮未进一步复现物理损坏。应使用引擎提供的一致性快照机制，验收必须实际打开备份核对事务内容。

### AUD-010 · P1 · 已删除账号的旧 JWT 仍能通过认证

证据：`internal/svc/service_context_auth.go:304-316` 把任何 `GetTokenVersion` 错误都视为可放行；`internal/model/admin_model.go:220-225` 对软删除账号返回 RecordNotFound。永久不存在与临时存储故障没有区分。

动态复现：签发 JWT 后软删除账号，再创建**全新的**认证中间件（没有 30 秒缓存因素），`authenticate` 仍返回该账号身份且 err=nil。

影响：删除账号没有可靠撤销登录身份；没有二次权限检查的接口仍可使用该身份，不能声称所有业务接口都会继续放行。应对账号不存在/禁用拒绝认证，并单独定义存储故障处理策略；测试需覆盖删除、禁用和版本不匹配。

### AUD-016 · P1 · 对象存储接口允许低权限身份覆盖备份对象

证据：`internal/handler/routes_storage.go:19-30` 挂在 protected 组，仅继承认证；`internal/api/storage/service.go:122-175` 直接用客户端 path 调用全局 ObjectStore.Put，无操作权限和保留前缀隔离。

动态复现：预置临时对象 `backups/private.sqlite`；仅注入登录身份、未提供任何角色权限，实际 UploadObject handler 返回 200；重新读取对象为攻击夹具的 replacement 字节。此探针隔离 handler/service，不是完整登录流测试；全局路由及认证职责另外静态核实。

影响：可破坏备份/部署等共用对象存储内容，存储密钥并未泄露也不构成保护。应核验存储管理权限，并按既有产品定义保护备份和发布保留路径；覆盖覆写、删除、批量删除、目录改名和签名下载。

### AUD-017 · P1 · 合法游戏作用域账号可修改全局节点电源配置

证据：`internal/handler/routes_task.go:218-223` 的 power 端点位于 scoped Ops 组；`internal/api/ops/powerops.go:221-263` 按全局 nodeID 找节点并更新 Meta，没有权限检查，也不验证节点与当前作用域的关系。注释明确电源操作本身为全局操作。

动态复现：真实零角色账号只授予一个合法游戏/dev 作用域，经实际 AuthMiddleware + GameDBMiddleware，PUT 全局 `node-1` 的 power-config 返回 200，并改变其 BMC 地址。没有执行真实硬件动作。

影响：游戏环境可访问性不能充当平台电源管理授权。Power 的列表、读取、提交、续行、拒绝也缺少统一操作权限入口；本轮动态确认的是配置写入，未声称审批机制被绕过。应按既有角色权限控制全局硬件管理，不通过发明新业务分域解决。

### AUD-001 · P1 · Provider token 校验未与传输保密绑定

证据：`cmd/sidecar-agent/root.go:638-647` 对 providerAuth.require 只检查 token 是否非空；样例 `configs/agent.yaml` 的 localAddr 为 `0.0.0.0:19091`、tls.enabled=false。`internal/agent/tcp_local_listener.go:388` 验证握手 metadata.authToken，但该验证不保证链路加密。

静态确认：在这些设置的组合下，开启 token 校验仍可通过明文 listener 传输 token；没有进行真实抓包或重放测试。应限制非 loopback 的受认证链路使用 TLS，或提供明确、显式的受控明文配置；保留本地开发可用性。和 AUD-014 的 SDK TLS 缺失必须一起处理，不能只开启 Agent TLS 后宣布六语言可用。

### AUD-002 · P1 · RPC handler 错误可以成为成功传输与空 protobuf

证据：`internal/transport/tcp/server.go:339-357`、`core/transport/tcp/mux_conn.go:370-383`。Invoke 用字符串拼接 JSON；其他错误回空 body。`internal/platform/dispatch/dispatcher_invoke.go:149-156` 解码成功即记录成功；SDK 的旧 TCP Invoker 也只返回 Payload，不能把它当成当前 HTTP Invoker 全路径的证据。

动态复现：本地真实 TCP Server/Client，handler 返回含引号与换行的错误；Call err=nil，InvokeResponse.Payload 非法 JSON。StartTask handler 报错时 Call 仍 err=nil，空 body 可解码成零值 StartTaskResponse。

另核实 JS/Python/Java/C++/C# 的过载或异常兜底也存在空响应形态，说明三条现有 golden 向量没有覆盖错误语义。应定义传输层结构化失败状态，JSON 使用编码器，逐消息核对客户端可观察结果；不能靠错误日志或解析业务 Payload 文本代替失败状态。

### AUD-014 · P1 · C++ / C# Provider 的 TLS 配置未进入实际连接路径

C++：`sdks/cpp/src/croupier_client.cpp:417/592` 构造 TCPTransport 只传 host/port/timeout；`tcp_transport.cpp:261` Connect 为普通 socket，Call 使用 send/recv。`ClientConfig` 的 cert/key/ca/server_name 未进入该路径；`:776` 却可仅根据 insecure=false 自报 transport_security_mode=tls。

C#：`sdks/csharp/src/Croupier.Sdk/CroupierClient.cs:681/774` 默认 factory 只传地址、两个 timeout 和 logger；`Transport/TCPTransport.cs:202` 直接取得 NetworkStream。该默认实现没有 SslStream 或 TLS 协商。配置类中的 Insecure/CertFile/KeyFile/CaFile/ServerName 未接入 Provider transport。

静态调用链确认；未运行两语言完整 SDK 与真实 TLS Agent 端到端。范围是内置的 Provider 默认连接路径，不扩张到 Server HTTP Invoker，也不否认调用者自行注入 transport 的可能。影响：安全选项无效、token 仍明文发送、TLS listener 不能正常兼容；功能矩阵将两者 TLS 标绿与实现不符。应真正实现 TLS 或在请求安全模式时明确拒绝连接，并补 TLS 服务端端到端测试。

### AUD-013 · P1 · C++ 响应 writer 静默接受短写，且缺少共享帧写锁

证据：`sdks/cpp/src/tcp_transport_inbound.cpp:148-180` 仅一次 send，只判断 sent<0；没有补全剩余字节。多个 worker 和控制车道共用同一 socket，写帧处没有连接级互斥；普通 Call 在 `tcp_transport.cpp:430` 也单独 send。

动态复现：编译实际 writer，使用链接器 `--wrap=send` 注入合法的正数短写：请求写 140 字节，仅接受 5 字节，函数仍静默返回，无重试、错误或失效连接动作。

并发帧交错是静态风险，未做拥塞网络下的动态重现。应统一每条连接的完整帧写锁和 write-all，失败后使挂起调用能尽快感知连接不可用；仅补打印负数错误不足以修复短写。

### AUD-018 · P2 · Python reader 遇非法大帧后保留虚假连接状态

证据：`sdks/python/croupier/transport/tcp.py:298-340`。ConnectionError/OSError 分支会清 connected 并通知 pending；ValueError 等走通用 Exception，只日志后 break，没有统一 finally 收口。

动态复现：给实际 `_reader_loop` 注入超过 32 MiB 的帧头；reader 返回后 connected/running 仍为 True，pending 无错误、event 未置位。无需申请大 payload 内存即可触发。

影响：后续 connect 的已连接短路和等待响应状态与死掉的读线程不一致；在途调用必须耗尽各自超时。应在全部异常/退出路径统一清连接状态、关闭 socket、失败 pending，并保留重连可观察性。

### AUD-003 · P1 · 移动端分析摘要受旧作用域请求覆盖

证据：`mobile/lib/features/analytics/analytics_controller.dart:23-34/54-83`。copyWith 不能清空已有 summary；refresh 没有请求代次/取消保护；失败保留旧 summary。`analytics_dashboard_page.dart:57-82` 仅在 error 非空且 summary 为空时显示错误页。

动态复现：使用实际 Riverpod AnalyticsController、InMemorySessionStore 与 Dio 拦截器；先发 alpha 请求，再切 beta 并发请求；beta 先返回后正确展示，随后 alpha 返回覆盖 state，当前会话仍为 beta。所有响应为本地合成数据。

应在游戏/环境或会话改变时清理旧数据，以请求代次或取消控制拒绝过期响应；失败必须明确标记旧内容与重试。approvals controller 等也有类似请求模式，本轮未逐控制器动态复现，不能把一个控制器测试说成全移动端覆盖。

### AUD-011 · P1 · 移动端 CI 覆盖率步骤必然解析失败

证据：`.github/workflows/ci-mobile.yml:43-61`。Python 使用单引号 heredoc `<<'PYEOF'`，`:58` 的 `float("${COVERAGE_THRESHOLD:-80}")` 不发生 shell 展开。

动态复现：从实际 workflow 提取原始 shell 片段，提供 LF=10、LH=10 的有效 lcov；打印 100.0% 后仍抛 ValueError，无法进入阈值比较。此结论不依赖 Flutter/Dart 版本。

应通过环境读取阈值或直接使用数值默认值；验证 100% 通过、阈值以下拒绝、无有效覆盖数据拒绝，再同步 CR-FIX-26 的完成口径。

### AUD-012 · P2 · Web SSE 的 CRLF 跨分块时改变事件类型

证据：`web/src/services/core/http.ts:174-184` 每读一个 chunk 就将尾部孤立 CR 归一为 LF；下一个 chunk 起始 LF 被误认为新的空行。

动态复现实际 TypeScript 类：相同字节 `event: update\r\ndata: hello\r\n\r\n` 整块输入得到 update/hello；切在第一个 CR 和 LF 之间时得到 message/hello。

影响：具名事件监听器可能收不到事件，事件名及 data 的归属被破坏。应对跨分块的 CR 保留解析状态，并覆盖所有 CRLF、UTF-8、多行 data 分块边界；本轮只复现 CRLF 场景。

### AUD-015 · P2 · 多游戏进程重启后未打开的库不会收割陈旧任务

证据：`internal/tasks/store.go:135-156` 只枚举 ScopeResolver.OpenScopes；`cmd/server/root.go:554/803` 接 Router.OpenScopes；`internal/db/router/router.go:388` 仅返回当前进程缓存中的库。新进程缓存为空不代表磁盘库没有上个进程的任务。

动态复现真实文件库与现有 scope resolver 夹具：创建一小时前的 running 任务；模拟新进程没有 OpenScopes，ReapStale 返回 0，任务保持 running；将 scope 加入已打开集合后同一收割器更新 1 行。Router 冷启动缓存为空由源码另行核实。

应让重启恢复按已登记数据库/持久任务索引有界巡检，或明确首开恢复并提供独立完成保障，不能永久依赖用户主动访问该游戏。

### AUD-004 · P2 · Agent Stop 超时返回成功

证据：`cmd/sidecar-agent/service.go:164-175` 等待 done 或 15 秒后均 return nil。没有将未停止状态上报服务管理器。静态确认，未额外等待 15 秒构造卡死 runner。

应区分正常结束、runner 失败及超时，超时返回稳定错误并保留诊断；重复 Stop 保持幂等。

### AUD-005 · P2 · SDK 鉴权矩阵与代码漂移

证据：`sdks/SDK_FEATURE_MATRIX.md:41-44` 仍写 Go/Python/Java 未注入 Provider 握手 token；当前 Go `tcp_manager.go:869-877`、Python `__init__.py:357-361`、Java `CroupierClientImpl.java:356-361` 均已注入 authToken。

六语言的握手 token 注入已逐一静态找到，不能沿用 11 月 4 日“缺统一 Provider 鉴权”的完整旧结论。矩阵 TLS 行的另一处实质性漂移见 AUD-014。应以实现与端到端证据更新能力矩阵，不能仅靠 API 符号存在守卫。

### AUD-006 · P2 · 待验证：移动端工作区锁文件与 CI 工具链

当前用户未提交的 `mobile/pubspec.lock` 声明 Dart `>=3.11.0-0`，`pubspec.yaml` 下界为 ^3.9.0，CI 固定 Flutter 3.35.1。当前本机实际 Flutter 3.47.9 / Dart 3.13.7。

本轮移动端探针仅在本机已有工具链下用 `--no-pub` 运行；没有安装/切换 CI 工具链，未更改锁文件。因此本项为**兼容性风险，不能宣称 CI 已因依赖解析失败**。需要用固定 CI 工具链解析并检查锁文件零 diff，或在依赖确有需要时同步工具链及 SDK 下界。独立于已经确定复现的 AUD-011。

## 已有 PowerOps 六项复核

| 编号    | 级别 | 当前证据与结果                                                                                                                                                           | 验证边界                                                                                  |
| ------- | ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------- |
| PWR-001 | P1   | `request.go:224-253` Get→pending 检查→审批查询→无条件 Update；`store_gorm.go:28` 为 Save。用真实 GormStore 和审批屏障让两个请求都读到 pending，两次 BeginResume 全部成功 | 已动态复现重复领取；没有执行真实 reboot/force_off                                         |
| PWR-002 | P1   | `request.go:198` WithoutCancel 移除 deadline，`:310` RunAction 直接消费该 context；verify 的时间循环在动作完成后才进入，且 Status 调用前没有独立 deadline                | 静态确认；未挂起真实云命令。仅使用时间循环不能中断卡住的 Provider                         |
| PWR-003 | P1   | `request.go:306-324` 所有动作循环完成后才首次 ReplaceTargets；BeginResume 只接受 pending；没有 executing/verifying 的重启恢复入口                                        | 静态确认；原“每目标变化即刷新”注释与实现不符，未注入进程崩溃                              |
| PWR-004 | P1   | `request.go:397-432` terminalState 按 Verified 聚合，finish 只按 Success 计数；实际复现单目标 `state=failed,succeeded=1,failed=0`                                        | 已动态复现终态与计数矛盾                                                                  |
| PWR-005 | P1   | Submit 对 status 同样建审批；expectedState(status) 为 ""；正常返回 on/off 的 Status 永远不等于它，循环持续到 verifyTimeout。设计文件 status 免审批                       | 静态确认正常状态返回的路径；未运行默认 120 秒等待；Provider 缺失/查询失败等路径不必都等满 |
| PWR-006 | P2   | verifiable 排除 WOL；terminalState 把 Success 且不可验证的目标当 done，未等关联 Agent 上线                                                                               | 静态确认，原测试还固化了此行为；未用真实 WOL 包或主机重连验证                             |

11 月 12 日 `ResumeAsync` 返回快照的竞态修复已存在（`5c107a50a`），它不解决原子领取、超时或持久进度，不能据此关闭上述六项。

## 历史外部 35 项按当前代码复核

“源码已修/已落地”表示本轮核对了现有实现或文件；不表示曾成功在远端运行，更不表示真实发布已完成。外部报告保留其 11 月 4 日快照，本报告负责当前状态，不改写历史原文。

| 原编号 / CR-FIX | 当前判定                         | 当前证据或剩余边界                                                                                                                                                       |
| --------------- | -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| P0-1 / 01       | 源码已修                         | JS/Python manifest 为 Apache-2.0，根 LICENSE 有 2026 cuihairu 版权行                                                                                                     |
| P0-2 / 02       | 已复验                           | `check-version-sync.sh` 全部 10 处声明 0.1.6，通过；比 v0.1.5 tag 更新是正常发布间状态                                                                                   |
| P0-3 / 03       | 源码已修                         | `config_control_server.go:259` ValidateInterconnectTLS，`cmd/server/root.go:198` 启动校验；cluster 运行时共用 TLS 完整性谓词；不等于 Provider listener 的 AUD-001 已解决 |
| P0-4 / 04       | 发布保护已落地，远端未验         | release/nightly 有 quality-gate、依赖门禁、SHA256SUMS 与 cosign；未签名或发布本次产物                                                                                    |
| P0-5 / 05       | 部分修复                         | SDK CI 已含 proto 触发，buf breaking 与三条共享 golden 在案；Go/Python/JS 实际通过。没有 Provider 鉴权/TLS/错误状态等完整行为场景，AUD-002/014 证明其边界                |
| P1-6 / 06       | 源码/文档已纠偏                  | architecture/index 状态表已为 SQLStore；实际装配为 NewSQLStore。根 CLAUDE.md 概述仍残留 MemStore 旧描述，作为文档观察保留                                                |
| P1-7 / 07       | 部分修复                         | 不兼容请求版本返回支持列表并保留连接，替代原全断开；不能据此证明完整能力协商或 N-1 兼容窗口                                                                              |
| P1-8 / 08       | 源码已修                         | Java controlPool、C++ control_pool、C# controlLimiter 均在；不再是仅 3/6 双车道。各实现对控制队列的有界性不同，未压测                                                    |
| P1-9 / 09       | 已复验                           | TCP Server 当前 executionWorker/有界业务队列/独立控制车道，现有慢 handler、inflight、deadline 测试通过                                                                   |
| P1-10 / 10      | 设计与代码已补                   | 协议文档 delivery matrix；Dispatcher 仅 errAgentUnreachable 故障转移，完整响应（含 busy）不自动重放。错误响应的成功误判另见 AUD-002                                      |
| P1-11 / 11      | 部分修复                         | 六语言 authToken 注入、Agent 强制开关已实现；安全传输缺口为 AUD-001/014，矩阵漂移为 AUD-005                                                                              |
| P1-12 / 12      | 源码已修，远端未验               | nightly 使用按日 tag 固定 commit，不再 tag -f；校验/签名步骤存在。未验证线上历史产物                                                                                     |
| P1-13 / 13      | 编排已落地，实际注册中心无法复核 | reusable SDK matrix 六车道与 release draft 门禁存在；没有用工作流文件存在代替真实 npm/PyPI/Maven/NuGet/Go/vcpkg 发布记录                                                 |
| P1-14 / 14      | 源码已修                         | ci.yml 全量包含 pkg/cmd；codecov 不再 informational，SDK target=85。未查询远端覆盖率服务或分支保护设置                                                                   |
| P1-15 / 15      | 源码已修                         | agent_session_db.go 原生 ALTER TABLE，失败日志并下次重试；遗留非映射列保留不阻断现有业务。未复验各数据库升级/回滚                                                        |
| P1-16 / 16      | 文件已落地                       | CONTRIBUTING/CHANGELOG/CoC/NOTICE/CODEOWNERS/模板齐备，SECURITY 为 0.1.x+main，报告渠道已列；PGP 仍明确未配置                                                            |
| P1-17 / 17      | 构建已复验                       | ignoreDeadLinks 改限定豁免，locale parity 接 CI；本机 docs build 通过。未逐页人工审校译文                                                                                |
| P1-18 / 18      | 文档已修                         | 现行 `docs/architecture/adr/0001-transport.md` 已替代归档权威引用；gameops-demo 已存在                                                                                   |
| P1-19 / 19      | 工程设施已落地，本轮视觉未验     | jsx-a11y 与 axe E2E 在案；未重跑四主题浏览器对比度，不宣称本轮 WCAG 验收完成                                                                                             |
| P1-20 / 20      | 源码卫生已修                     | Makefile 为 web 与内置 SDK；测试产物已 untrack。当前用户未提交改动独立保留                                                                                               |
| P2-21 / 21      | 部分修复                         | 四层依赖守卫已接 CI；原 svc/service、api/handler 全量合并未做，不能只因 guard 通过宣布组织结构迁移完成                                                                   |
| P2-22 / 22      | 部分修复                         | Background/panic 约束与豁免守卫在案；没有以注释豁免等同实际有界执行，PWR-002 是反例。所有 TODO 消化未验                                                                  |
| P2-23 / 23      | 源码已修                         | proto/buf.breaking.yaml 与 CI 基线获取/比对步骤存在；本轮未制作破坏性 proto 变异运行                                                                                     |
| P2-24 / 24      | 仍未完成                         | 现有 CR-FIX-24/PERF 项保留：没有已验证的并发容量目标与 P95 门禁                                                                                                          |
| P2-25 / 25      | 流水线已落地，远端未验           | nightly-test 具备 race+shuffle、重复测试、fuzz 与日志产物；本轮未跑整套 nightly                                                                                          |
| P2-26 / 26      | 部分修复，完成口径需修正         | coverage/golden/威胁模型已新增；但 coverage gate 必然 ValueError，见 AUD-011，不能继续称 CI 覆盖率门禁有效                                                               |
| P2-27 / 27      | 初始化治理已落地                 | fresh bootstrap 默认口令轮换；demo/fixture 明确 allowDefaultCredentials。未扫描真实部署凭据                                                                              |
| P2-28 / 28      | 仍未完成                         | 版本统一，能力实际不同；矩阵未能反映 TLS、握手鉴权现状，既有 CR-FIX-28 保留                                                                                              |
| P2-29 / 29      | 文档约定已落地                   | CONTRIBUTING 的文件/语种约定与 glossary 入口存在；不把残存历史中文文件名当运行时缺陷                                                                                     |
| P2-30 / 30      | 文件已修                         | 界面截图为 WebP，guide 引用及 optimize-screenshots.sh 在案；未重抓新截图                                                                                                 |
| P3-31 / 31      | 文件已修                         | 根 ROADMAP.md 存在                                                                                                                                                       |
| P3-32 / 32      | 文件已修                         | EN/zh transport-glossary 包含传输术语                                                                                                                                    |
| P3-33 / 33      | 入口已复验（3 语言）             | `scripts/contract/run-golden-vectors.sh --only go,python,js --strict` 无跳过；另三语言测试源码存在但本轮没运行                                                           |
| P3-34 / 34      | 文件已修                         | `docs/development/frontend-styles.md` 说明 antd/token/antd-style/global.less/Tailwind 的实际适用范围                                                                     |
| P3-35 / 35      | 文件已修                         | EN/zh `docs/development/versioning.md` 说明根 VERSION 与独立 Go module 策略                                                                                              |

## 11 月 1 日 C-001..032 复核

下表“源码修复保留”只覆盖对应旧发现，避免把修复后别处的新缺陷混为一谈。未对全部旧发现做新的动态变异/回归验证。

| 编号  | 当前复核                                 | 源码证据 / 新边界                                                                                  |
| ----- | ---------------------------------------- | -------------------------------------------------------------------------------------------------- |
| C-001 | 源码修复保留                             | Bugs href/addLink 经 safeExternalHref；本轮通用 URL 测试通过                                       |
| C-002 | 源码修复保留                             | provision/template validateCloudInitExtraLine，Validate 与 BuildCloudInit 均检查                   |
| C-003 | 源码修复保留                             | JS transport parseMessage 校验、外层读异常/finally 断开并失败 pending；Python 类似问题另列 AUD-018 |
| C-004 | 源码修复保留                             | Web SSE 走 fetch Authorization；后端仍支持 query token 是兼容入口，本轮不把它说成当前 SSE 必入 URL |
| C-005 | 仍存在                                   | Web 登录/HTTP client 读写 localStorage token；沿用原待办，不擅自迁移鉴权模型                       |
| C-006 | 源码修复保留                             | Tickets/Detail 附件经 safeExternalHref                                                             |
| C-007 | 源码修复保留                             | Alerts 多个外链经 safeExternalHref                                                                 |
| C-008 | 源码修复保留                             | Agent exec 输入边界与既有白名单规则保留；Server 全局操作的权限遗漏属于 AUD-017 等独立问题          |
| C-009 | 源码修复保留                             | svc/db.go 数据库名白名单验证再构建 DDL                                                             |
| C-010 | 源码修复保留                             | OpenCast 的 validCastSessionID 白名单                                                              |
| C-011 | 源码修复保留                             | Java TlsSocketFactory 使用临时 keystore password 参数与 SecureRandom，不再固定口令                 |
| C-012 | 源码修复保留                             | Python varint 解码查截断与 10 字节上限                                                             |
| C-013 | 源码修复保留                             | Go SDK inbound 回帧 writeResponseFrame 的错误被记录；SDK 原有测试本轮通过                          |
| C-014 | 旧负数错误日志修复保留，但相关缺口仍存在 | C++ 负数 send 错误已记日志；正数短写仍静默，实际复现为 AUD-013                                     |
| C-015 | 源码修复保留                             | Java ServerHttpInvoker JSON 错误解析失败会 logger.warn 后回退文本                                  |
| C-016 | 源码修复保留                             | hostdriver resolveProbeSecret 使用环境值或 crypto/rand                                             |
| C-017 | 源码修复保留                             | Tools 外链经 safeExternalHref                                                                      |
| C-018 | 源码修复保留                             | compositeRuntime 导航经 safeExternalHref + noopener/noreferrer                                     |
| C-019 | 与 C-007 合并                            | 同一 Alerts URL sink，不重复计数                                                                   |
| C-020 | 源码修复保留                             | realtime message JSON.parse/pushRealtime 有 catch，finally 清 loading                              |
| C-021 | 本轮未动态复验旧负载 flake               | 原修复与后续 metrics 截止提交在当前历史；不能仅凭一次普通测试宣布压力下不抖动                      |
| C-022 | 源码修复保留                             | OpsAgentsList fallback 遍历持 RegistryStore.RLock                                                  |
| C-023 | 源码修复保留                             | provisionAgentRegistry.WaitRegistered 遍历持 RLock                                                 |
| C-024 | 源码修复保留                             | Agent local listener mux.Run 返回后 mux.Wait                                                       |
| C-025 | 源码修复保留                             | openapi/provider_call.go defer Close，ReadAll 后才判状态并消费 body                                |
| C-026 | 源码修复保留                             | SlidingWindowLimiter.GetStats 对空 timestamps 做 len>0 检查                                        |
| C-027 | 源码修复保留                             | session/hub consumer 超时可取消底层 writer；未做本轮长跑泄漏测试                                   |
| C-028 | 源码修复保留                             | mysql_pump flush 的 out 发送带 ctx.Done                                                            |
| C-029 | 源码修复保留                             | Server/Provider session handler state/markRegistered 使用同一 mutex                                |
| C-030 | 源码修复保留                             | externalplatform cache.Close 使用 sync.Once                                                        |
| C-031 | 源码修复保留                             | TracerProvider exportTokens 有界，ExportSpans 错误落日志；未验外部 exporter 卡住时的停机上限       |
| C-032 | 原“不适用”结论保留                       | 对应依赖 Close 无返回值；不是一个可通过补 `_ =` 修复的问题，本轮未修改依赖                         |

11 月 4 日 WIP 记录中的迁移索引、PushFile 符号链接、部署对象唯一键、analytics worker 双 flush 等，历史已记录对应落地 commit。本轮未为其中每个问题重复故障注入；不能把那张 WIP 提交盘点表当成当前全仓清洁证明。

## 行为观察与排除项

- 组件模板 Create 当前仅凭登录身份可写，模型是全局模板库，副本实例化语义已文档化。探针确认了行为，但未找到本轮足以裁定其必须按作者/角色隔离的明确契约，因此**不计已确认缺陷，不擅自增加权限规则或游戏分库**。
- 公开 registry/OpenAPI 的匿名只读行为在路由和服务注释中明确；没有把“未登录可读”直接当漏洞。
- `permission.PermissionMiddleware` 的 X-Admin-ID fallback 需要可达装配证据；生产入口实际使用 svc.AuthMiddleware，本轮未把未确认可达的辅助中间件报成全站认证绕过。
- 强制改密目前为前端软闸，原 OPEN-ISSUES 已明确该已知边界；不冒充新发现，也不与已存在的 MFA API gate 混淆。
- 本轮未验证外部注册中心、真实部署凭据、完整 CI 状态和生产数据；所有相关结论均保留范围限定。

## 验证与交付

[证据目录与复现入口](evidence/2026-11-12/README.md) 保存所有探针源码、日志、工具链和关键源码指纹。

- Go overlay：10 个包、11 个探针通过。其中模板为行为观察；其余探针对应 10 项问题。再加 Web、Mobile、Python、C++、CI 五个独立探针，共 15 项问题动态复现。
- `go build ./...` 通过；Web `tsc` 通过；`pnpm --dir docs build` 通过。
- 根模块原有测试命令：`go test -count=1 -timeout 8m ./internal/svc ./internal/config ./internal/cache ./internal/policy ./internal/platform/functioninvoke ./internal/platform/dispatch ./internal/api/backup ./internal/api/component ./internal/powerops ./internal/tasks/... ./internal/db/router ./internal/transport/tcp ./core/transport/tcp`。最终结果见 `go-baseline.log`。
- Go SDK 独立 module 内 transport/croupier 两包原有测试通过；首次误将其作为根 module 子目录的命令错误单独留档，已纠正，不计产品缺陷。
- Web 定向 Jest 3 套件、45 测试通过；指定的 scope store 测试路径未额外匹配到套件，不把它计为已执行。
- Go/Python/JS 三语言 golden 向量 strict 运行通过，无跳过；这只验证文件中三种样本及相关断言。
- 页面契约守卫与版本一致性守卫通过。
- 本机 Flutter 3.47.9 / Dart 3.13.7 的 Controller 乱序探针通过；未跑 Flutter 3.35.1 CI 工具链。

修复顺序建议：先封住全局操作权限与生产 JWT 默认值（AUD-007/008/010/016/017），再处理备份一致性与协议真实失败/TLS（AUD-009/002/013/014/001），然后收敛 PowerOps 状态机、移动端陈旧响应和失效 CI，最后处理其余 P2。这里的顺序不替代游戏侧 GAME-009/010 已有高风险操作协议待办。
