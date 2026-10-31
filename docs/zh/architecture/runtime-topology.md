---
title: 运行架构、连接与生命周期（源码现状）
---

# 运行架构、连接与生命周期

> **2026-10-30 最新复审：CHANGES REQUIRED。** 调度已前置领取、多个入口已接入 Govern、主记录落库失败和 descriptor 读取失败会拒绝派发；但复现了函数 ACL 漏检、风险读取错误降低审批、跨槽 claimed 未恢复及终态事件部分写入。正常 drain/停机路径已有实现，已接收任务与总截止仍有缺口。详见 [最新复审](../../review/reliability-recheck-2026-10-30.md)。真实高危写入不获上线批准。

核查日 **2026-10-30**。这是源码与仓库部署模板的现状，不是线上盘点或生产认证。**已实现**表示可追踪到生产装配；**目标设计**表示尚有运行缺口；**待核实**表示需要真实配置、账号或环境。本轮不修改业务实现。历史审核见 仓库审核记录 `docs/review/architecture-audit-2026-10-28.md`。待办唯一入口为仓库根 `todo.md`。

<ArchitectureDiagram lang="zh" />

[打开完整中文拓扑 SVG](/architecture/system-zh.svg) · [English SVG](/architecture/system-en.svg)

## 可部署单元与代码边界

| 单元 / 部署位置                | 启动入口（已构建产物）                                                                      | 实际职责与限制 / 源码                                                                                                                                                                                |
| ------------------------------ | ------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Server / 控制面网络            | `bin/croupier-server --config configs/server.yaml`                                          | 单进程 Gin HTTP、注册发现、dispatch、审批/审计、页面/契约、Ops、后台调度；`cmd/server/root.go`、`internal/svc/service_context.go`。这些模块不是独立微服务。                                          |
| Web / 浏览器 + 静态 nginx 容器 | `docker compose -f docker/docker-compose.quickstart.yml up -d` 中 dashboard                 | Umi/React 客户端；nginx 提供静态文件并反代 API；`web/config/proxy.ts`、`docker/nginx-main.conf`。                                                                                                    |
| Mobile / 运营终端              | Flutter 应用，`mobile/lib/main.dart`                                                        | HTTP API 客户端，`mobile/lib/app/providers.dart`；没有证据证明移动端覆盖全部 Web 功能或已有生产安装。                                                                                                |
| sidecar / 游戏主机或 VPC       | `bin/croupier-agent --config configs/agent.yaml`                                            | SDK 网关、函数转发、任务、受管程序、探针、扩展；`cmd/sidecar-agent` → `internal/app/agent` → `agents/sidecar`。                                                                                      |
| capture / 能访问数据源的主机   | `bin/capture-agent --config configs/capture-agent.yaml`                                     | 独立 CDC、位点、过滤/规则、告警 spool、可选事件 journal；`agents/capture/runner/runner.go`。                                                                                                         |
| synthetics / 消费方网络观察点  | `bin/synthetics-agent --config configs/synthetics-agent.yaml`                               | 目标探测及外部探测插件；`agents/synthetics/runner/runner.go`。独立角色装配不能等同 sidecar 全部内置 Checker。                                                                                        |
| devops / 运维网络              | `bin/devops-agent --config configs/devops-agent.yaml`                                       | 注册、白名单热更、SSH Broker/Hub/回放基础；`agents/devops/runner/runner.go`。面板下行中继未接通，RDP/VNC 显式拒绝。                                                                                  |
| 游戏服务 / 游戏方运行环境      | 游戏自己的进程嵌入 `sdks/{go,js,python,java,csharp,cpp}`                                    | SDK 是库，不是另一个服务；游戏权威状态由游戏方负责。六语言 API 与恢复能力以 SDK 矩阵和各实现为准，不能由 Go 实现推断全部语言。                                                                       |
| 函数 / 探测插件 / Agent 子进程 | 宿主按配置 `exec.Command` 启动                                                              | `core/pluginhost/engine.go`、`agents/sidecar/funcplugin`、`agents/synthetics/pluginhost`：go-plugin 本地 gRPC；不是 SDK 或 Agent 上行协议。宿主控制退出与熔断；`no_go_plugin` 构建裁剪另见既有文档。 |
| ingest / 分析接入网络          | `bin/ingest --addr :8088`                                                                   | HTTP 签名校验并发布事件/支付 JSON；`cmd/ingest/cmd/root.go`。                                                                                                                                        |
| analytics-worker / 分析网络    | `bin/analytics-worker`（环境变量配置）                                                      | Redis 消费组、批写 ClickHouse、聚合/死信；`cmd/analytics-worker/main.go`、`internal/analytics/worker/worker.go`。                                                                                    |
| 运维 CLI / 一次性作业          | `cmd/analytics-export`、`cmd/check-db`、`cmd/schema-validator`、`tools/protoc-gen-croupier` | 导出、诊断、校验、代码生成工具，不是必须常驻服务。                                                                                                                                                   |

`core/` 是显式组合的公共包（transport/register/configsync/backoff/process/pluginhost 等），不是独立部署 Agent。数据库、Redis、ClickHouse 是外部基础服务。nginx/HAProxy 是可选部署入口。Herald、OIDC、Vault、QuickSDK/扩展平台与观测后端按开关启用，不是 Croupier 内部服务。

## 连接清单：方向是建连或所标数据流

总览图只表示拓扑。双向会话内的请求方向不能由初始 TCP 箭头推断；下面列出实际用途、认证与数据。未列固定端口的外部依赖均取配置，不能据模板推断线上地址。

| 连接 / 触发                                                            | 协议与交互                                                                                                                                | 认证、内容与依据                                                                                                                                                                                                                                                                                      |
| ---------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 浏览器/移动端 → Server；页面加载/操作                                  | HTTP 请求响应；任务结果 SSE 是 Server → 客户端的持续响应；可经 nginx                                                                      | JWT 与 scope 中间件（部分公共 API 例外）；gameId/env、PageSpec binding、参数、审批/任务查询；`internal/handler/routes.go`、`internal/svc/service_context.go`、`internal/svc/game_middleware.go`、`internal/api/console`、`internal/api/task`。                                                        |
| 四类 Agent → Server control `:19090`；启动/重连/热更                   | 自定义 TCP 帧 + protobuf；注册/心跳请求响应；已建 session 双向复用，承载命令/配置与上报                                                   | TLS 按配置；CA 可启 mTLS。没有 HTTP JWT 注册流程；注册 AgentID/gameId/env 后绑定连接身份，后续上行归一化防止改 scope；不等于证书绑定业务身份。`internal/server/tcp_listener.go`、`control_handler.go`、`agents/controlconn/controlconn.go`、`core/transport/tcp`。                                    |
| SDK → sidecar provider `:19091`；游戏进程连接                          | ProviderConnect/ACK、Heartbeat/ACK；Server → sidecar → SDK Invoke，SDK 返回 JSON 结果；SDK 也可经网关调用函数                             | SDK 模板依赖本地/受控网络，TLS 可配置；Connect 校验 serviceId、登记元数据，不存在自动继承 Web JWT 的保证。`internal/agent/tcp_local_listener.go`、`local_handler.go`、`agents/sidecar/provider_session.go`。                                                                                          |
| sidecar → Server HTTP / 扩展同步启用                                   | `CROUPIER_EXTENSION_SYNC_API` 下周期 GET `/api/v1/agents/:id/extensions`，8s 超时与版本 payload；经 `X-Croupier-Agent-Token` 携带机器身份 | `internal/app/agent/extension_sync_puller.go` 发送 ARCH-021 机器 JWT（CROUPIER_EXTENSION_SYNC_TOKEN）；`internal/svc/service_context.go` 将机器 token 限定在端点（其余路径 403），`internal/api/extension/service.go:AgentSyncPayload` 校验 agentID scope。签发：`croupier-server token agent-sync`。 |
| Server A → owner Server B；本实例无目标 session                        | 明文 TCP interconnect、hello、单跳请求响应；invoke/start/cancel 与原调用者上下文/epoch                                                    | 当前 hello 仅校验 role，无对端密码学认证；epoch 拒绝过时 owner，不承担认证。`cmd/server/cluster.go`、`mesh_forwarder.go`、`internal/cluster/transport.go`。                                                                                                                                           |
| Server → 关系库；请求/后台作业                                         | GORM/驱动 SQL，读写元数据、业务、契约、页面、任务、审计                                                                                   | DSN 凭据/TLS 依配置；metadata DB 与 dbctx 选定 game DB 区分；`internal/svc/service_context.go`、`internal/db/router`、`internal/db/dbctx`。                                                                                                                                                           |
| Server → 共享 DB 或 Redis；集群启用                                    | 成员/owner 租约、epoch、发现快照定期刷新                                                                                                  | 同后端配置的凭据；不共享 net.Conn；`cmd/server/cluster.go`、`internal/cluster`。Redis 缓存与分析流用途另行区分。                                                                                                                                                                                      |
| capture → 源数据库；启动/源断线重开                                    | MySQL/MariaDB binlog（含 GTID 路径）、PostgreSQL pgoutput、MongoDB change stream；持续源事件返回 capture                                  | 数据源凭据与复制权限由配置供给；本地位点与源确认；`agents/capture/source`、`agents/capture/connector/{pg,mongo}`、`runner/runner.go`。源数据不自动经 ingest 进入 ClickHouse。                                                                                                                         |
| capture → Server；规则命中                                             | AlertEventRequest/ACK；断线/失败由 spool 重试，Server 幂等告警 ID 入库                                                                    | 绑定 Agent 身份；rule/source position/对象键/时间与 scope；`runner/alert_spool.go`、`control_handler.go:handleAlertEventAsAgent`。可选 journal 留在 capture 本地。                                                                                                                                    |
| 探针宿主 → 目标；轮询周期                                              | 按角色/目标 HTTP、TCP、DB 或自定义协议；返回状态/时延/失败阶段                                                                            | 目标凭据来自配置/受控下发，不等于控制面认证；`core/healthprobe`、`internal/app/agent/probe_manager.go`、`agents/synthetics/probe`。Server 也有本机探测装配。                                                                                                                                          |
| synthetics → Server / 每次结果                                         | session 上 MsgAlertEvent protobuf 通知，无应用 ACK；发送预算 5s                                                                           | `agents/synthetics/runner/runner.go:execute`、`agents/controlconn/controlconn.go:SendProbeEvent`：断线/发送失败只日志，不持久重放，不能套用 capture spool 保证。                                                                                                                                      |
| Agent 宿主 → 插件子进程；第一次调用/重建                               | HashiCorp go-plugin 本地 gRPC Describe 后 Invoke/Check 请求响应                                                                           | 握手契约与本机进程边界，不是远程授权；调用传受控凭据，探测结果清理凭据；`core/pluginhost/engine.go`、`agents/synthetics/pluginhost/result.go`。                                                                                                                                                       |
| devops Hub → SSH 主机；仅本地 Broker 被调用时                          | SSH 流、窗口调整、关闭与 asciicast 记录                                                                                                   | allowlist、用户凭据和 known_hosts 指纹；`agents/devops/session/{broker,hub,sshtarget}.go`。从面板到这里的通路尚缺失，不能画为已通。                                                                                                                                                                   |
| 外部生产者 → ingest `:8088`                                            | HTTP JSON，HMAC/时间窗口；写 MQ 后返回                                                                                                    | secret/per-secret 配置、限流、可选 Redis 去重；无共享或 per-game 密钥时返回 403 ingest_disabled；生产凭据仍待核实。                                                                                                                                                                                   |
| ingest / Server telemetry bridge → Redis Streams → worker → ClickHouse | 发布 XADD；worker XREADGROUP/XAUTOCLAIM/XACK；ClickHouse 原生驱动批写                                                                     | Redis URL/密码、ClickHouse DSN；bridge 按开关启用；`internal/analytics/mq`、`internal/telemetry`、`internal/analytics/worker/worker.go`。Kafka 发布分支存在，内置 worker 无 Kafka 消费分支。                                                                                                          |
| Server → Herald（可选）；告警/报表通知                                 | SDK HTTP trigger 请求响应，外部通知异步触发；站内消息先落库                                                                               | token 环境变量、app/受众与 dedup key；缺配置走 noop，有限重试；`internal/platform/outlet/{outlet,herald}.go`、`internal/svc/service_context.go:registerOutlets`。                                                                                                                                     |
| Server/sidecar 扩展 → 第三方平台（可选）                               | HTTP API；按外部平台函数触发                                                                                                              | QuickSDK OpenID/OpenKey 签名及扩展驱动配置；`internal/extensions/official/externalplatform/client.go`、`internal/app/agent/extension_external_bridge.go`。仓库实现不证明真实账号已验收。                                                                                                              |

OIDC 登录、Vault secret resolver、SMTP/对象存储、OTel/监控链接是可配置的支撑依赖，按路径启用，详见 `internal/svc/service_context.go`、`internal/config/config.go`、`configs/server.yaml`；总览以“可选集成”合并展示，不能推断每个实例必然连接全部外部系统。实际启用矩阵待 DEPLOY-001。

## 谁负责发现、路由、调度和状态

- **发现**：SDK Connect/Heartbeat/断开维护 sidecar LocalStore 与 ProviderSession；sidecar 将全量函数/进程快照注册上行，变更去抖后重注册。Server registry 保存 scope/函数/Provider/到期时间并持久化 `agent_sessions`，启动恢复快照；快照不是活连接。证据：`internal/app/agent/upstream.go:buildRegisterPayload/updateLoop`、`internal/platform/registry/store.go`、`internal/server/control_handler.go`。
- **失效**：Server 注册 TTL 默认 5 分钟，心跳按默认 TTL 续期；live session 断链立即按 `(agentID, sessionID)` 清除，避免旧连接清除新连接。registry 定时清理到期记录及 Provider 投影；DB 锚点可作为最后已知身份，不能作为在线证明。证据：`internal/server/agent_session.go:RemoveSession`、`internal/server/tcp_listener.go`、`internal/platform/registry/agent_session_db.go`。
- **路由**：`internal/platform/dispatch/dispatcher.go` 过滤 scope/函数/可用候选并按策略选择，优先本机 session，否则 owner 转发；sidecar 再选择 Provider、内建函数或插件。不是 Server 直接拨游戏进程。函数调用默认上限 15s，metadata budget 会在 1s–上限间钳制；失败重试取决于路径/错误，不可推导副作用安全重试。
- **调度**：Server `internal/tasks/scheduler/manager.go` 消费任务定义，派发函数或执行 server-local 报表；sidecar `internal/app/agent/ops_schedule.go` 运行受管程序本地 cron。两者复用 `core/process` cron，但不是一个独立调度服务；本地分钟槽去重、不补跑不能外推为分布式 exactly-once。
- **存储**：关系库保存任务/页面/契约/审计等；scope 是授权边界，multiGame 仅部分模型走 dbctx，不代表所有表都物理分库。生产开库已采用 binding.database_name 真值；首次开库 Open 阶段预算仍有缺口（ARCH-009）。审批生产装配仍 `approvals.NewMemStore()`；SQLStore 存在不代表已启用。证据：`internal/svc/service_context.go`、`internal/svc/game_middleware.go`、`internal/db/router/router.go`。

## 独立生命周期图与失败语义

每张时序图从上向下表示时间，竖虚线是生命线；带 ALT 的框是互斥分支，ACK 必须按正文解释，交叉线无圆点不相连。

### 启动注册与重连

![启动注册与恢复](/architecture/registration-zh.svg)

[完整 SVG](/architecture/registration-zh.svg)

Server 先创建 ServiceContext/数据库与注册物化、scheduler、session resolver、可选集群，再启动 control 与 HTTP。DB Ping 失败可能只警告；集群初始化部分失败会返回 standalone，`healthz` 不是完整 readiness。Agent 启动时校验非空 scope、建立 TCP/TLS、Register，成功 ACK 后进入心跳；SDK 可以晚于上行接入，函数变化会重新注册。

`core/register` 的 `SyncAttempts` 每次注册默认 10s、同步退避 200ms→2s；`DialAndRegister/SyncOnce` 不加该超时，继承调用方/传输预算。心跳每 3s/单次 3s、连续失败 2 次重建连接；首拨失败 RunReconnectLoop 退避 5→60s（×1.5），心跳恢复在 tick 内重拨，不共用该退避循环。相关默认值可配置，预算边界见 ARCH-022。连接替换前关闭旧连接，Stop/ctx 取消结束循环。新 Register 从当前业务快照组装，不是重放在途命令。SDK 重连策略依各语言；图不宣称所有语言都有持久恢复。证据：`core/register/{register,registration,loops}.go`、`internal/app/agent/upstream.go`。

### 受控同步调用、审批与异步任务

![执行与任务](/architecture/execution-zh.svg)

[完整 SVG](/architecture/execution-zh.svg)

![Task lifecycle](/architecture/task-zh.svg)

[Task SVG](/architecture/task-zh.svg)

HTTP 身份/scope → 页面快照 binding（或受控草稿）→ functioninvoke 的禁用/分配/输入契约/政策门。拒绝、待审批与执行互斥。任务与调度已接入 Govern，但函数 ACL 和风险来源故障仍有 R30-01/02 的复现，不能声明完全一致治理或撤权后必不派发。Create 只校验可调度性，每次触发再治理。生产 dispatch 注入 TaskRunWriter，task_runs 写失败禁止 StartTask（ARCH-026）；descriptor 查询错误拒绝，确无 schema 才跳过（ARCH-027）。返回 taskId 仅代表接收；取消先记意图再路由，Cancel ACK 不证明外部副作用撤销。

**实现边界**：TaskRunner 对在途同 ID Start 做内存去重、用 generation 清理并有限重试终态；跨进程重启无持久去重。Server 先更新终态再插事件的部分写入缺口已复现，不能保证重试补齐事件。页面幂等不替代游戏持久收据。证据：`agents/sidecar/task_runner.go`、`internal/server/control_handler.go:handleTaskEvent`、`internal/api/approval/service.go`。

### CDC 告警与出口

![CDC 与告警](/architecture/events-zh.svg)

[完整 SVG](/architecture/events-zh.svg)

capture 加载位点→连接源→Poll→可选 journal/consume→AckCheckpoint→周期刷本地 bookmark；源流结束刷位点/Close 后重开，损坏 bookmark 不当作空位点继续。规则命中先入本地 spool 再异步请求 Server ACK，成功才删除 spool；入队满和持久化失败各有日志/计数。Server 幂等写告警，随后站内通知与可选 Herald 出口；`Dispatch` 在入库后、构造 ACK 前启动异步 goroutine；通知送达可先于或晚于 ACK。“告警已入库”的 ACK 不代表外部受众已收到。外部出口默认 3 次尝试、2s 间隔、15s 请求预算，永久错误不重试；维护 gate 可能抑制通知。原始 CDC 数据不是默认分析事件总线。证据：`agents/capture/runner/{runner,alert_spool}.go`、`internal/server/control_handler.go`、`internal/platform/outlet/outlet.go`。

### 插件与停机

![插件生命周期](/architecture/plugin-zh.svg)

[完整 SVG](/architecture/plugin-zh.svg)

宿主先取得并发 gate，按需启动子进程、握手/Dispense、Describe，才调用 Check/Invoke。失败关闭进程并记退避/熔断，成功复用常驻进程；Stop 禁止新调用，最多等待 5s 在途后 Kill。探测凭据通过受控入参传递且结果脱敏，不推导插件是安全沙箱。证据：`core/pluginhost/engine.go`。

![停机与边界](/architecture/shutdown-zh.svg)

[完整 SVG](/architecture/shutdown-zh.svg)

Provider drain 已具会话 Draining、RPC 在途计数、路由拒绝与默认宽限等待，undrain 使用 reason 哨兵；但已接收而未进入 RPC 的任务会因注册先被摘除而失败（R30-09），ACK 不保证任务资源全排空。宽限到期不强制关仍在跑的请求。Server 正常停机顺序为 StopAccepting → HTTP drain → rootCancel → StopScheduler/ControlService.Stop → WaitConnections → ServiceContext.Close 冲刷并关池、清模板重建器。外层等待 30s，但最终 defer StopScheduler 未受其约束（R30-07），不能保证总退出时间。sidecar Stop 依次停插件、本地监听、上行、遥测和 Ops；worker main 取消后只等固定 200ms。证据：`internal/agent/local_handler.go`、`cmd/server/root.go`、`internal/svc/service_context.go`、`internal/app/agent/app.go`、`cmd/analytics-worker/main.go`。故障验收见 ARCH-015–018、DATA-015。

## 部署方式与故障处理

- **本地二进制**：Makefile 定义 server/agent/agent-capture/agent-synthetics/agent-devops/ingest/worker，产物见上表。本轮未执行编译。四类 Agent 每实例一个 gameId/env，端口为进程内默认，可配置。
- **quickstart**：`docker/docker-compose.quickstart.yml` 是单 Server + sidecar + dashboard + PostgreSQL + Redis；宿主 `8443 → 容器 19090` 是端口映射，不意味着 HTTPS。analytics、sdk-examples profile 才启额外组件。capture/synthetics/devops 不在默认栈，不凭图宣称已部署。
- **HA 模板**：`docker/docker-compose.deploy.yml` 的双 Server/sidecar 与 HAProxy，dashboard nginx 走 HTTP，Agent 走 L4；成员/owner 后端共享，session 留各实例内存。模板/healthcheck 的存在不证明线上可故障切换。互联仍需可信网络隔离，mTLS 为目标设计。
- **TLS/状态卷**：开发模板证书留空、insecure 开启；生产需核对终止点、CA/证书、数据库/Redis ACL。保留 Server data、capture bookmark/spool/journal、devops casts 与审计日志、sidecar desired_state/schedule_pause 文件和插件版本。数据库备份不覆盖内存审批与正在执行的任务。
- **故障诊断顺序**：先核对 scope/当前配置与真实进程→注册 ACK→owner/session→函数/Provider 快照→受控执行拒绝原因→task event/凭据。Agent 离线不靠重写 DB 在线标志恢复；SDK 断线应观察 Provider 移除和重注册；执行超时先查询结果和游戏凭据，禁止据未知结果盲目重发高风险操作。
- **分析可靠性**：明细批次已改为 Send 后 ACK，重入队/死信 XADD 错误不确认原消息；聚合并发读写仍复现 race（DATA-014），worker 退出不等待全部后台协程（DATA-015）。真实崩溃/重放验收仍属 ARCH-012。

## 现状与目标分开

| 已实现 / 限制                                     | 目标设计或待核实                        | TODO                   |
| ------------------------------------------------- | --------------------------------------- | ---------------------- |
| owner 单跳转发 / epoch；部分协调失败 standalone   | 认证互联、集群 ready、失租约摘流        | ARCH-003–005           |
| 内存审批；SQLStore 未装配                         | 跨实例持久审批、原子决定与执行领取      | ARCH-001/002           |
| 页面/任务关系库存储；游戏副作用不由 UI 幂等覆盖   | 任务重放协议、真实游戏凭据              | ARCH-014/015、GAME-001 |
| DB scope 校验；binding 驱动物理路由               | Open 预算、连接池容量                   | ARCH-006/009/010       |
| drain/停机正常路径已接入；待发任务/总截止仍有缺口 | 断连/drain/SIGTERM 演练、宽限后强制关闭 | ARCH-018               |
| devops SSH 基础；面板中继缺失                     | 中继/回放界面；不自动扩 RDP/VNC         | AGENT-002/003          |
| SLP/私有协议骨架与插件宿主                        | Login 级真实账号验收                    | PROBE-001–003          |

## 外部资料与版本边界

以下官方资料于 **2026-10-29** 核查，只用于协议语义，不作为本仓实现验收。版本来自仓库，实际部署摘要仍待 DEPLOY-001。

| 产品/库             | 仓库版本/模板                          | 官方资料与核对结论                                                                                                                | 待确认                                     |
| ------------------- | -------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------ |
| Redis               | quickstart `7-alpine`（浮动标签）      | [XACK](https://redis.io/docs/latest/commands/xack/)：从消费组 PEL 移除已确认条目，不是下游数据库事务                              | 实际版本、持久化/裁剪策略、故障窗口        |
| HashiCorp go-plugin | `go.mod v1.8.0`                        | [官方仓库](https://github.com/hashicorp/go-plugin)：子进程 RPC，支持 gRPC 多语言；本仓限定本地 gRPC                               | 平台/语言/真实协议目标验收                 |
| PostgreSQL          | quickstart `15-alpine`                 | [15 逻辑复制](https://www.postgresql.org/docs/15/logical-replication.html)：发布/订阅及复制身份；本仓 pgoutput 路径另需 slot/权限 | 源端 publication/slot、WAL 保留及实际版本  |
| MySQL               | `go-mysql v1.9.1`；源端版本未确定      | [MySQL 8.0 GTID](https://dev.mysql.com/doc/refman/8.0/en/replication-gtids.html)：GTID 是复制定位语义                             | MySQL/MariaDB flavor、权限、保留与恢复样本 |
| ClickHouse          | driver `v2.43.0`，模板 `23-alpine`     | 批写调用以 `worker.go` / driver 接口为证据，本轮未扩展产品选型调查                                                                | 实际服务器兼容性/去重/持久化见 ARCH-012    |
| Herald              | `v0.1.3-0.20261014084113-9875146dc50a` | 本仓 SDK 调用与 outlet 代码为证据，不宣称外部送达                                                                                 | trigger 配置、受众与真实收据               |

MongoDB change stream、OIDC/Vault、第三方平台账号、镜像/许可证全矩阵专题本轮未开展，登记 DEPLOY-001 / PROBE-003；不得因接口存在勾选验证完成。

## 2026-10-30 失败边界复核

上文正常流程不构成完整故障保证。drain 前已接收但未进入 Provider RPC 的任务可能在 ACK 后因 pickInstance 注册已删除而失败；taskId 续接例外无法覆盖此前置查找。Server 的外层 30s 等待不覆盖最终无界 defer StopScheduler。task_runs 终态更新与 task_events 写入不是一个事务，第二步失败后重试会被直接 ACK。审计 Log 的 strict 错误仍被部分调用方吞掉。上述缺口已分别回填 ARCH-015/016/017/008；工作区 watchdog 属在途实现，不能代替 ARCH-018 演练。证据及具体复现见 [最新复审](../../review/reliability-recheck-2026-10-30.md)。
