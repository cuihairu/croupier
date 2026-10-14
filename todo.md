# TODO

需求清单：一条一句，只列需求。完成项保留一行证据（commit hash / 测试规模），过程叙述见 git log 与 docs/ 对应设计文档。

## 未完成

- [x] 4862 ops 告警列表按 game/env 过滤：Alert 加 game_id/env 列（迁移 0043）+ opsAlerts 按 GameScopeFromContext 过滤（scope 告警 + 平台级告警）+ 打戳三点（evaluator 继承 rule scope / inbound generic payload scope / control_handler capture req scope）+ 面板告警页全局视野确认（/api/v1/alerts 无 scope 不过滤）（4862 迁移 0043 + 三处同步 + 测试，2026-10-13）
- [ ] 4310 MongoDB Connector：CDC 内置源收口项，排期待用户立项（立项简档已备 docs/design/cdc-mongodb-connector.md，三项拍板：立项排期 / beforeImage v1 保守 false / snapshot 不做）；SQL Server/Oracle/TiDB/CockroachDB/国产库明确不做，更多数据源由使用方按 SourceConnector 契约自行实现
- [ ] Agent 面板三页（agent-dashboard-design 简档）：Proposed 待用户过目批准，零代码；sidecar/devops/capture 每 type 一页+探针卡+职责数据+日志下钻，scope 联动
- [ ] Provider 插件 P1（#66 批二）：openapi driver 迁移 internal/platform/openapi → internal/drivers/openapi + externalplatform 迁入；按 §9 分期另行立项
- [ ] 插件机制正式设计落地（plugin-mechanism 简档）：Proposed 待拍板后按 §8 分期；含 LAN 资产接入（cockpit 优先）、配置热更双轨等 12 项决策

- [ ] 游戏客户端模拟探测（玩家侧拨测）：存量延后需求补落档（设计曾存在、用户拍板延后、从未写入文档，2026-10-14 澄清来历；服务端互探已否决「意义就小了」，要做就站玩家客户端视角，不做过渡方案）；本仓无可复用游戏协议栈（sdks/ 是游戏服务器→agent 注册面，e2e-agent-probe 是 croupier wire mock）；协议适配骨架设计已并入 docs/design/agent-health-recovery-review.md §5.5（三层=内核/骨架/适配器；四条接入路径默认声明式；「协议未接入」一等状态绝不判不可用；L1 端口+HTTP=R2 顺路 → L2 声明式私有协议零码 → L3 按游戏重要度逐个接）；P2 按需立项（触发即玩家视角，玩家代表性网络选点随立项给）
- [ ] 探测结果上行统计与在线健康度（设计单 2026-10-14 三条，简档 §13）：不对称上报（正常态聚合走 MetricsReport 采样器折叠、异常态明细即时走 SendAlertEvent probe.detail）+ 在线数据汇总复用 ops 模块 game/env 维度与 AlertRule evaluator（在线健康度=状态矩阵不出合成分）+ 采样频率 server 策略（基础/时段/特殊点三粒度，ConfigVersion 落库 configsync 下发热更，默认 30s 显式，异常态提频兜底）；R2 部分已随 R2 批落码（采样器折叠 + 不对称明细上行）；R3 次子批（AlertRule evaluator 消费 probe.<target>.<successRate|failed|checked|latencyP95Ms>，checked=0 视为缺失、P95 桶粒度近似）已交付（证据见简档 §12）；R3 三子批（probe.policy 采样策略：kernel 三粒度+热更/ConfigVersion 管理面五端点/wire 下行/agent 热更+异常态提频）已交付（证据见简档 §12）；R3 四子批（probe_stats 持久化：迁移 0044 probe_json 列+MetricsStore 写读重建）已交付（证据见简档 §12）；R3 余下（ops 在线汇总页/server 内核装配）待后续批
- [ ] 受管程序生命周期统一模型（agent-supervisor-design §10，三份外部评审反哺 2026-10-14）：ManagedProcess 扩 kind（program/probe-adapter/游戏进程本体，三配置模型归并两层一套）+ 四类状态 + 更新五要求与**指纹硬要求**（sha256+execlog 查询口径+回滚留痕，游戏进程更新需维护窗口联动）+ 远程 Shell 硬边界 + Agent 自升级单列；Reconciler 归属未核实（待核 Supervisor/dispatch 边界）；设计已反哺 data-flow/agents-inventory/monitoring/upgrade-rollback/README，随 R2-R4 后排期

- [ ] Agent 健康监控与恢复：外部方案评审 + server 侧监控下放 agent（来源：用户 2026-10-14）——评审定稿 docs/design/agent-health-recovery-review.md：重合度审计 + 真缺口七项 + 迁移四步 + 三点已拍板（2026-10-14：probe.targets 每 agent 均可配 devops 主用 / 上行走 report.Conn 扩 SendAlertEvent 新 wire / DBSource 归属列 R4 才动表不停 R2/R3）+ 补充裁决并入（第三方监控零新增 agent 部署、单一权威分工表、实现层/实例层分标、dbmon SQL 探测收敛 core/healthprobe 不留第二套、熔断器=调用方策略留 server dispatch、游戏模拟探测=存量延后需求 P2）；R2 装配面代码批已交付（commit ebcde49，87 条新用例 + 全量门禁绿，证据见简档 §12）；R3 首子批（窗口→Alert 生命周期：probe.window 开/合转 firing/resolved + details 落最近 5 次失败明细）已交付（commit 06976e1，11 条新用例，证据见简档 §12）；R3 次子批（evaluator probe 指标路径）已交付（证据见简档 §12）；R3 余下（probe_stats 持久化/在线汇总/probe.policy/server 内核装配）待后续批

## 已完成

### 平台与面板

- [x] 上传即成页契约与绑定正交化 T1-T13（T1 65f88486b 至 T13 545477a5e，2026-09-18 全部完成）
- [x] 菜单管理系统 T-M1 至 T-M10（menu_items 表、CRUD、权限过滤、默认菜单种子，2026-09-18）
- [x] 风控面板：/risk 分类、capture gates 表单/JSON 双模式编辑、策略版本历史与行级回滚、风险事件人工处理四列（迁移 0042）、告警中心迁移 /risk/alerts（2026-10-13）
- [x] 密码箱抽象与可替换提供商：vault.Provider 接口、local/env 双实现、vault:// 引用接入 settings/DSN/JWT 三 seam（internal/platform/vault，2026-10-13）
- [x] 服务器状态源：默认 atlas、通用 http 源可切换（切换六用例 + http 源 12 用例，2026-10-12）
- [x] 设置页外部服务只读清单 S3-web（92b6cfc，2026-10-11）
- [x] 三 agent 体系：sidecar 定名、devops 堡垒机代理、capture CDC；使用场景文档、一键安装脚本、逐字段配置文档（2026-10-12）
- [x] 插件机制 M2-M4：Outlet 告警出口、webhook 告警入口、外部服务接入模板（2026-10-10）
- [x] 系统公告 Markdown 渲染补齐（#50）与运维七页全量核对（#51 至 #56，2026-10-08）
- [x] Provider 插件设计立项 #66、Agent Supervisor 立项 #67（设计简档加 S1-S3 交付，2026-10-10）

### CDC 捕获链

- [x] 统一契约：ChangeEvent 模型、FilterPolicy 策略头、SourceConnector 六方法接口、能力闭集（agents/capture/connector，2026-10-11）
- [x] 三闸与告警链：reconcile/cep/lineage 加 CaptureRuleHitReport 上报（fb189bb、db31e20、4f9584e、01646d4、b9ddc43，2026-10-11）
- [x] 中心策略管理：config_versions 资产化乐观锁、发布依赖与能力感知 lint、appliedRevision 上报、回滚端点、位点-策略联动四拍板（2026-10-13）
- [x] 过滤策略落地：event filter 热换、失败语义与统计计数入 Stats（runner_stats_test.go 三用例，2026-10-13）
- [x] MySQL/MariaDB Connector：flavor 双方言、GTID 变量分叉、CI capture-matrix（2026-10-13）
- [x] PostgreSQL P0 闭环：pgoutput 解码层、流式 Connector、runner 多源装配、活栈全链端到端、六阶段集成测试（2026-10-13）
- [x] 4301 验证清单六项本地活栈跑通：规则热生效、断线恢复、位点恢复、新表启用、过滤错误可观测、跨 game/env 隔离（22ffb40 修复 Poll 启动失败 walsender 泄漏，2026-10-13）
- [x] capture agent 进 docker compose：Dockerfile.capture-agent + 配置 + postgres-init 发布脚本 + compose 默认启用（CAPTURE_AGENT_REPLICAS 开关，默认 1）+ 文档同步（ef3c44f，2026-10-14）
- [x] capture 链部署环境验收：4301 六项在部署机复验通过+截图取证（本地 capture agent×2 + 本地 CDC 源经 SSH 隧道指部署 server；部署栈滚动升级至最新 main 镜像；admin 口令已重置须改回；证据 docs/证据/4301-deploy-acceptance/README.md，2026-10-14）

### 测试与竞态

- [x] `-race` 全量不可绿修复（存量，与 CI/门禁无关，2026-10-14）：pkg/protocol 溢出用例 checkptr fatal（`!race` 标签隔离）、internal/server 测试 atomic 写/裸读、internal/hotreload 插件 ABI 不匹配（构建标签常量对 + 插件同参编译）、cmd/ingest+cmd/server 测试竞态、TCPListener wg.Add/wg.Wait 契约违反（Close 先排空 accept 循环再 Wait，serveStarted/serveExited 两通道定序）；证据 55f63d1 + 555031a，internal/server -race count=50/10 零竞态、cmd/server -race 绿；边界：本机 14 核高负载下 -race 全量仍有 600s 环境性超时（非真竞态，memory 已记录）

### 质量工程

- [x] Go 覆盖率巡检 56 轮：internal 多包收口至 100%（2026-09-27 至 10-08）
- [x] web 覆盖率巡检 40 余轮：全站零测试页归零，全量 jest 360+ suites 绿（2026-09-28 至 10-01）
- [x] Dependabot/audit advisory 收口：axios 12 条、dompurify（2026-09-30）
- [x] PageSpec/渲染守卫复验七批（dashboard_vnext_guard，2026-09-30）
- [x] 增量测试口径：scripts/test-changed.sh + make test-changed（改动分类 + 反向依赖闭包 + 三类验证，84b4f0c，2026-10-14）
- [x] 快照产物测试抖动修复：report mtime 显式阶梯（25/25 通过，7cba119，2026-10-14）
- [x] Docker workflow build-server 超时修复：gha 层缓存（含 go mod cache mount）+ timeout 15→25min，新 run 全绿（f64d569，2026-10-14）
