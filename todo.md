# TODO

需求清单：一条一句，只列需求。完成项保留一行证据（commit hash / 测试规模），过程叙述见 git log 与 docs/ 对应设计文档。

## 未完成

- [x] 4862 ops 告警列表按 game/env 过滤：Alert 加 game_id/env 列（迁移 0043）+ opsAlerts 按 GameScopeFromContext 过滤（scope 告警 + 平台级告警）+ 打戳三点（evaluator 继承 rule scope / inbound generic payload scope / control_handler capture req scope）+ 面板告警页全局视野确认（/api/v1/alerts 无 scope 不过滤）（4862 迁移 0043 + 三处同步 + 测试，2026-10-13）
- [ ] 4310 MongoDB Connector：CDC 内置源收口项，排期待用户立项（立项简档已备 docs/design/cdc-mongodb-connector.md，三项拍板：立项排期 / beforeImage v1 保守 false / snapshot 不做）；SQL Server/Oracle/TiDB/CockroachDB/国产库明确不做，更多数据源由使用方按 SourceConnector 契约自行实现
- [ ] Agent 面板三页（agent-dashboard-design 简档）：Proposed 待用户过目批准，零代码；sidecar/devops/capture 每 type 一页+探针卡+职责数据+日志下钻，scope 联动
- [ ] Provider 插件 P1（#66 批二）：openapi driver 迁移 internal/platform/openapi → internal/drivers/openapi + externalplatform 迁入；按 §9 分期另行立项
- [ ] 插件机制正式设计落地（plugin-mechanism 简档）：Proposed 待拍板后按 §8 分期；含 LAN 资产接入（cockpit 优先）、配置热更双轨等 12 项决策

- [ ] 游戏客户端模拟探测（玩家侧拨测）：存量延后需求补落档（设计曾存在、用户拍板延后、从未写入文档，2026-10-14 澄清来历；服务端互探已否决「意义就小了」，要做就站玩家客户端视角，不做过渡方案）；本仓无可复用游戏协议栈（sdks/ 是游戏服务器→agent 注册面，e2e-agent-probe 是 croupier wire mock）；协议适配骨架设计已并入 docs/design/agent-health-recovery-review.md §5.5（三层=内核/骨架/适配器；四条接入路径默认声明式；「协议未接入」一等状态绝不判不可用；L1 端口+HTTP=R2 顺路 → L2 声明式私有协议零码 → L3 按游戏重要度逐个接）；P2 按需立项（触发即玩家视角，玩家代表性网络选点随立项给）；**2026-10-15 用户裁定数据源走网络收集并代决选型**：选型定稿 docs/design/game-data-sources.md——推荐 PrismarineJS/minecraft-data（唯一 MIT 可走数据消费通道；机器可读 protocol.json 构建期拉取+转换生成 steps，原始 json 不随仓分发）+ 首适配器目标=Minecraft Java offline-mode 自托管（真实游戏非 mock，零账号 E2E；Luanti=UDP 族第二候选，AzerothCore/TrinityCore=L3 商业 MMO 知识原型零拷贝）；架构用户代决=三层切分先骨架+一个真实游戏适配器，「协议未接入」一等状态不伪装；**待账号验证**（不勾完成不写 mock，与未实现区分）：Minecraft online-mode 加密链（需 1 个 MC Java 账号+可出网环境）、用户自家游戏玩家侧拨测（测试账号与环境随立项给）；License 变更需用户确认；本轮只落选型文档未动实现
- [x] 探测结果上行统计与在线健康度（设计单 2026-10-14 三条，简档 §13）：不对称上报（正常态聚合走 MetricsReport 采样器折叠、异常态明细即时走 SendAlertEvent probe.detail）+ 在线数据汇总复用 ops 模块 game/env 维度与 AlertRule evaluator（在线健康度=状态矩阵不出合成分）+ 采样频率 server 策略（基础/时段/特殊点三粒度，ConfigVersion 落库 configsync 下发热更，默认 30s 显式，异常态提频兜底）；R2 部分已随 R2 批落码（采样器折叠 + 不对称明细上行）；R3 次子批（AlertRule evaluator 消费 probe.<target>.<successRate|failed|checked|latencyP95Ms>，checked=0 视为缺失、P95 桶粒度近似）已交付（证据见简档 §12）；R3 三子批（probe.policy 采样策略：kernel 三粒度+热更/ConfigVersion 管理面五端点/wire 下行/agent 热更+异常态提频）已交付（证据见简档 §12）；R3 四子批（probe_stats 持久化：迁移 0044 probe_json 列+MetricsStore 写读重建）已交付（证据见简档 §12）；R3 五子批（server 侧内核装配：probe: 段+本机探针面+HandleServerProbeEvent 同链生命周期视角=server+手动诊断端点）已交付（证据见简档 §12）；R3 六子批（ops 在线汇总：GET /api/v1/ops/probe/summary 状态矩阵 server 面 + 前端卡片挂节点维护页，两视角并存+scope 头过滤+在线趋势「未接入」灰态）已交付（证据见简档 §12）；R3 收口，全条目完成（2026-10-15 翻账）
- [ ] 受管程序生命周期统一模型（agent-supervisor-design §10，三份外部评审反哺 2026-10-14）：ManagedProcess 扩 kind（program/probe-adapter/游戏进程本体，三配置模型归并两层一套）+ 四类状态 + 更新五要求与**指纹硬要求**（sha256+execlog 查询口径+回滚留痕，游戏进程更新需维护窗口联动）+ 远程 Shell 硬边界 + Agent 自升级单列；Reconciler 归属已核验（2026-10-15：既有机制补全不新增组件——supervisor 恢复环=运行状态事件驱动协调器、dispatch=命令路由面，data-flow §6.2 落笔）；**管理状态首块已实现**（2026-10-15）：运维手动 start/stop 期望落盘 `desired_state.json`（supervisor 日志目录），开机自启先查期望——运维主动停的进程跨 agent 重启不再被误启（§10.2「区分崩溃与主动停止」落地），缺记录 fail-open 沿既有 AutoRestart 语义零漂移，损坏文件 fail-open 空映射，原子写 tmp+rename，写盘失败仅告警不回滚；**远程 Shell 边界物化已交付**（2026-10-15，agent-supervisor-design §10.7）：既有 allowExec 远程命令面三件套核对与收口——双闸（ops.enabled+allowExec 缺省 false 同开，既有口径不变）+ 白名单改 fail-closed（`execAllowedCommands` 空列表不再等价「放行一切」，一律拒绝；Validate 拦配置面 + ExecuteCommand 运行期兜底）+ 审计补全（每次尝试写 execution_logs，source=agent_exec，操作人/命令/参数/退出码入审计载荷，审计面缺失 fail-open 不阻断主路径）；**ManagedProcess 扩 kind 已交付**（2026-10-15，§10.1/§10.8）：`kind` 闭集字段（program/probe-adapter，缺省 program 存量零漂移）+ 适配器清单字段（adapterId/adapterVersion/adapterProtocol 缺省 croupier-probe-jsonl-v1/adapterCapabilities，内嵌于 ManagedProcessConfig 而非独立文件，executable 复用既有 command/args）+ Validate 拦未知 kind 与 probe-adapter 缺身份 + kind/adapterId 随 supervisor 快照上行走面板（proto 追加 12/13）；**调度状态已交付**（2026-10-15，§10.6）：`schedule[]`（action 闭集 start/stop/restart + cron 五字段与 server 侧任务调度同一 ParseCron，Validate 配置加载即拦）+ 到点触发走显式动作同一漏斗（desired-state 落盘与 `schedule_fire` 事件随流，scheduled stop 与手动停同信号 stopCh、恢复环不误拉起，用例固化）+ 30s 分钟槽 (进程,动作,槽) 去重幂等 + 无补跑（crond 语义）+ 快照追加 next_schedule_at_unix/next_schedule_action（proto 14/15，调度状态事前可见）+ 边界：schedule 属本地配置意图不受 allowRestart 门禁（文档明示）；**更新状态+指纹已交付**（2026-10-15，§10.4）：`ops.allowUpdate` 独立双闸（缺省 false）+ `POST /ops/agents/:agentId/processes/:name/update`（stagedPath+expectedSha256，64 位小写 hex 内容哈希钉扎 fail-closed）+ wire 0x040123/24 + 先停后换静默核心（不发 manual_* 中间事件）+ 旧二进制 `.croupier-bak` 备份成功后保留为 last-known-good + 稳定性窗口 5s 内退出自动回滚旧版（update_rolled_back 事件带回滚指纹，无旧版首次安装失败无事件）+ 指纹硬要求三处落面（事件 update_applied/update_rolled_back、execution_logs source=process_update 持久查询主入口、快照 fingerprint proto 16 随 metrics 上行）；诚实边界：签名校验未实现（哈希为稳定锚点）、DB/配置兼容性检查未实现（游戏侧知识）、游戏进程维护窗口/在线阈值/流量导入闸未核实随首个真实游戏定；**自升级单列 v0.1 草稿已交付**（2026-10-15，§10.5）：失败不变量=「不替换也能活」退路（旧版 rename 备份不删+监管者重启兜底，最坏=按旧版重启不砖化）+ 四个待拍板问题（升级源建议 server 中继 / 触发建议手动 API 受授权双闸 / 重启机制建议 fork+exec 子进程接力 / 失败姿态=稳定性窗口+带外回滚）+ 指纹可先落项**已交付**（2026-10-15：SystemInfo.agent_binary_sha256 proto 11，agent spawn 语义自报；`GET /ops/agents/:agentId/system-info` 三级数据面（缓存最后已知→隧道 live RPC 回填→registry 标签降级）全携带指纹——原路由仅回标签 stub 到不了 API 面，已升级；SystemInfoCache.Get 改 proto.Clone 根治手写拷贝漂移；边界=缓存内存态，持久最后已知指纹需 agent_sessions 扩列随自升级一并落）；**持久「最后已知指纹」锚点已交付**（2026-10-15，§10.5）：agent 每次注册把运行二进制指纹写进注册标签（`labels.AgentBinarySHA256`，internal/common/labels 单一权威键；配置/动态标签不可覆盖——合并后注入，计算事实非配置；读不到不伪造），随标签走既有 upsert 持久化进 agent_sessions.labels（无需迁移，且规避独立列被 UpdateAll upsert 零值抹写）；system-info 数据面三级扩四级（+ DB 最后已知锚点 `registry.AgentSessionModel.LastKnownIdentity`，Unscoped 读——会话过期软删后行仍在，锚点跨 server 重启与会话 TTL）；实现口径偏离原「表扩列」记录（改用注册标签通道，理由见设计简档 §10.5）；边界：标签受 sanitizeAgentLabels 限额（2026-10-15 补强：平台键 `agentBinarySha256` 豁免限额——原实现满 32 随机 break 会按运气丢指纹，>32 标签的 agent 锚点不再静默丢失）、锚点新鲜度=最后一次注册（真值以 live RPC 与 execlog 为准）；**暂停与恢复调度 v1 已交付**（2026-10-15，§10.2 暂停对象=调度）：`POST /ops/agents/:agentId/processes/:name/schedule/pause`（或 resume，两路由同一处理器 suspend 由路径固定不走请求体）+ wire 0x040125/26 + 暂停声明的 cron 触发（进程运行态不动、不 stop、不影响恢复环、不改进程期望状态）+ 落盘 `schedule_pause_state.json`（与期望状态同目录同原子写，跨 agent 重启保留，缺/损坏 fail-open 空映射）+ scheduleTick 在槽去重前跳过暂停进程（暂停期间不写 schedFired，恢复后下一匹配槽照常触发、暂停期间不补跑，crond 语义）+ 门禁走 allowRestart 双闸（远程运维动作，与「定时触发本身不受门禁」不矛盾——门禁的是远程开关不是本地配置意图，无独立配置键）+ 快照 schedule_suspended proto 17 + 事件 schedule_paused/schedule_resumed（§3.6 闭集追加）；**顺带修复进程动作面绑定缺口**（#59 POST 侧）：start/stop/restart/update 原走 body 绑定，ShouldBindJSON 对面板发的无 body POST 一律报 EOF（400 bad_request），四条路由线上全失效；改为 agentId/name 只取路径参数（uri tag 不被 JSON 绑定读取），update 保留 stagedPath/expectedSha256 的 body 绑定 + 路径兜底；路由级用例钉死空 body POST 不再返回 EOF；边界诚实：暂停**程序**与暂停**探测**未做（程序暂停≈手动停+记住恢复，增量价值弱；探测暂停≈维护窗口抑制告警，随告警策略面一并做，不猜实现）；**§10 快照字段与调度暂停面板接入已交付**（2026-10-15：SupervisorDrawer 补「调度」列（下一次触发时间+动作 Tag，§10.6 事前可见；暂停态覆盖为「调度已暂停」）、`kind=probe-adapter` 的 `adapter:<id>` 标（§10.1）、指纹前 12 位 `fp:<hash>` 标（§10.4）、暂停/恢复调度按钮（暂停 Popconfirm/恢复直点，已暂停行禁用暂停、无调度条目禁用暂停，成功后复拉快照）、事件闭集五型本地化标签与 Tag 色（schedule_fire/schedule_paused/schedule_resumed/update_applied/update_rolled_back）；web 侧 SupervisedProcess 类型与归一化补齐 server DTO 全字段（lastEventUnix/nextRestartAtUnix/kind/adapterId/nextScheduleAtUnix/nextScheduleAction/fingerprint/scheduleSuspended），缺省零值旧版快照零漂移；23 条 drawer 用例 + Ops/Nodes 89 条全绿 + tsc 0 错误 + dashboard_vnext_guard PASSED）；实现待用户拍板四项（拍板前不猜实现）；**§10.2「更新配置文件」行核实为未实现、无设计**（2026-10-15 台账核实）：`core/configsync` 版本化拉取器先例只接 agent 自身配置（probe 策略/targets、插件同步），托管进程配置文件无分发链路，能力表行不因先例存在视为已交付；立项需先拍板五问（路径沙箱、校验口径、原子性与重启耦合回滚、体积带宽、审批闸二人规则），已记入设计简档 §10.2，拍板前不猜实现

- [x] Agent 健康监控与恢复：外部方案评审 + server 侧监控下放 agent（来源：用户 2026-10-14）——评审定稿 docs/design/agent-health-recovery-review.md：重合度审计 + 真缺口七项 + 迁移四步 + 三点已拍板（2026-10-14：probe.targets 每 agent 均可配 devops 主用 / 上行走 report.Conn 扩 SendAlertEvent 新 wire / DBSource 归属列 R4 才动表不停 R2/R3）+ 补充裁决并入（第三方监控零新增 agent 部署、单一权威分工表、实现层/实例层分标、dbmon SQL 探测收敛 core/healthprobe 不留第二套、熔断器=调用方策略留 server dispatch、游戏模拟探测=存量延后需求 P2）；R2 装配面代码批已交付（commit ebcde49，87 条新用例 + 全量门禁绿，证据见简档 §12）；R3 首子批（窗口→Alert 生命周期：probe.window 开/合转 firing/resolved + details 落最近 5 次失败明细）已交付（commit 06976e1，11 条新用例，证据见简档 §12）；R3 次子批（evaluator probe 指标路径）已交付（证据见简档 §12）；R3 三至六子批（probe.policy 管理面与下发 / probe_stats 持久化 / server 内核装配 / ops 在线汇总 server 面与前端卡片）已交付（证据见简档 §12）；R3 收口；R4 收敛批已交付（R4-1 归属列 0045 commit f57ff76 + R4-2 dbmon 判定切 DBChecker commit 79aaf76 + R4-3 周期探针按归属下发——拉取时派生 db_sources.owner_agent_id 目标集、wire 0x040121/0x040122、ApplyTargets diff 装配、derive/hash/拉取降级边界见简档 §12）；R4-4 验证已过（diagnose 端点保留 + DBChecker 判定链，简档 §12）；全条目完成（2026-10-15 翻账）；受管程序生命周期单列上行 15

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
