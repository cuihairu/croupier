# 当前架构审核与值得做的事项

审核日期：2026-10-28。范围：Server 装配、HTTP/scope、函数执行、审批、审计、集群、数据库路由、Agent 公共运行时与角色、页面运行时、分析链路及中英文文档入口。

本次是源码追踪与针对性测试，不是全仓逐行安全审计，也没有验证线上配置、网络策略、压测或灾备演练。审核期间工作区有其他变更，开始观察到 HEAD `91881988`，后续为 `70533b11`；以下结论以列出的文件及符号为依据。未修改业务实现。

## 判断

**总体方向合理，应保留模块化 Server + 出站 Agent + SDK 的主体结构。当前主要问题是运行保证与装配的一致性，优先补齐审批持久化、集群信任边界和故障就绪语义，收益高于拆微服务或重写传输。**

合理之处：

- 运营 HTTP 与 Agent 长连接分面，适合游戏 VPC 出站接入；连接归属通过 owner 转发处理，不把 net.Conn 当作共享状态。
- `core/transport/tcp` 是实际公共传输实现；`internal/transport/tcp/mux_conn.go` 是别名兼容层，不是第二份独立 MuxConn。
- `GameDBMiddleware` 在单库与分库模式都验证 scope，避免把“分库”误当权限隔离。
- `internal/platform/functioninvoke` 收口函数执行；页面 snapshot 在上层治理，避免每个 UI 自建调用链。
- task_runs/task_events 已持久化，跨实例取消有数据库兜底；页面执行有数据库幂等占位，不能笼统说“所有状态都在内存”。
- 独立 ingest/worker 让分析负载与控制面分离；四类 Agent 复用公共包，方向清楚。

主要代价：自建传输需要持续维护背压/重连/取消/wire 兼容；ServiceContext 及 API 包边界耦合较大；HA 已支持路由，但部分业务状态尚未满足同等级保证。它是带分布式执行面的模块化单体，不是完成拆分的微服务体系。

## 已核实的问题

### A01 · P1 · 审批存储没有接入生产持久化

- **证据**：`internal/svc/service_context.go:342` 创建 `approvals.NewMemStore()`，`:504` 注入 `ApprovalsStore`；生产源码未找到后续替换。`internal/platform/approvals/sql_store.go:17` 已有 `NewSQLStore`。
- **触发/影响**：A 实例创建的审批，B 实例无法从自己的内存读取；重启丢失待审批记录及结果。审批系统不能据此宣称跨实例完整 HA。
- **改法**：生产装配 SQL store，测试明确注入 memory；不能只换构造器。SQL `Approve/Reject` 当前是读后 Save（同文件 `Approve/Reject`），还需原子 pending→决定状态、并发冲突和执行领取语义，防止双批准导致重复副作用。
- **验收**：A 创建/B 审批可见；重启仍可查询；并发批准/拒绝仅一方成功；重复批准不重复执行；覆盖审批续跑和页面审批。
- **投入**：中。建议第一批。

### A02 · P1 · 集群互联的身份信任完全依赖网络隔离

- **证据**：`cmd/server/cluster.go` 的 `startCluster` 固定 `Insecure: true`；`interconnectHandler.Handle` 直接按消息类型进入 forward；`internal/cluster/transport.go` 的 hello 只看 role 字符串；`ServeForwardHandler` 校验 forwarded/epoch，不认证对端。
- **触发/影响**：能连接互联端口的非可信进程可接触内部执行入口；epoch fencing 用于防旧 owner，不是身份认证。`localInvoker.dispatch` 直接投递，不再运行 HTTP policy/approval。
- **现有文档情况**：`docs/zh/architecture/server-ha-multi-instance.md` §5.3 已诚实记录明文与网络隔离约束，但该文档前面的图仍标 mTLS；`internal/cluster/transport.go` 注释也仍声称 mTLS 拒绝 Agent 证书。不能把已披露边界描述为新发现的未知漏洞。
- **改法**：给 cluster 配置独立的 CA/cert/key/server identity，双向验证；hello 绑定已认证连接身份，forward 必须在认证后；升级期间显式 legacy 开关并限制网络访问。
- **验收**：无证书、错误 CA、Agent 身份、未握手 forward 均被拒；合法 Server 能转发 invoke/start/cancel，轮换证书不破坏 owner fencing。
- **投入**：中。第二批；若网络隔离无法保证，提升为部署阻断事项。

### A03 · P1 · 集群失败降级与健康检查不匹配

- **证据**：`cmd/server/cluster.go:startCluster` Redis ping/DDL 失败返回 nil，日志写 running standalone；`cmd/server/root.go` 继续启动控制监听和 HTTP。`internal/api/monitoring/service.go:Healthz` 只聚合 database/registry/ops；handler 对结果调用 `response.Success`。
- **触发/影响**：配置要求集群但实际启动为单实例，仍可进入 HTTP LB；只看 HTTP 状态的健康探针也不能从 200 判断组件 OK 字段。
- **改法**：独立 `/livez` 与 `/readyz`；配置 cluster.enabled 时，成员租约、互联监听和必要依赖未就绪应 ready=503；启动失败默认 fail closed，显式允许降级的开发配置除外。
- **验收**：Redis 不通、成员表不可写、互联端口绑定失败、租约丢失均摘流；恢复后按明确策略重入；进程存活探针仍可用于诊断。
- **投入**：小到中。与 A02 同批。

### A04 · P1（分库场景）· 绑定表和实际数据库名是两个来源

- **证据**：`internal/svc/game_middleware.go:validateGameScope` 查 `LookupDatabaseName`，只检查非空；随后 `Router.GameDB` 使用 `NameForGame` 重新计算名字。`internal/svc/service_context.go:newGameRouter` 没有注入绑定表 resolver。`internal/api/game/service.go` 更新环境时保留已有 `DatabaseName`。
- **触发/影响**：绑定表有保留的旧名称或修改 gameDBPrefix 后，校验的是旧绑定，打开的可能是新命名数据库。默认命名还存在非单射：`(a_b,c)` 与 `(a,b_c)` 都得到 `game_a_b_c`；这是命名函数可直接验证的冲突，不等于已经证实线上发生串库。
- **改法**：以授权后的 binding.database_name 为物理路由真值；新增名称使用稳定 ID/无碰撞编码，并约束非共享绑定的物理库名唯一；为已有绑定提供显式迁移。
- **验收**：自定义/旧库名、前缀变更、下划线边界与并发首次打开均读到预期库；禁止静默新建空库替代旧库。
- **投入**：中。启用 multiGame 前完成。

### A05 · P2 · 审计哈希覆盖与首页承诺不一致

- **证据**：`internal/audit/audit.go:calculateHash` 覆盖 timestamp/event_type/actor_id/resource ID/outcome/details/prev_hash/sequence，未覆盖 Context（game/env）、Changes、Action 等字段；`internal/svc/service_context.go` 使用 `NewAuditService(auditStore, nil)`，未装配 signer。函数审计在 `functioninvoke/invoke.go` 中受 RequireAudit 条件控制；记录失败的部分 helper 仅输出日志。
- **影响**：修改未参与哈希的字段不会改变该哈希；哈希链不能等同外部可信、防管理员整链重写的防篡改存储。“所有操作都审计”超出已核实实现。
- **改法**：定义版本化规范化审计 payload，补齐 scope/变化内容等关键字段；明确审计失败是拒绝执行、重试还是容许丢失；有强合规需求再加外部锚点/签名存储。
- **验收**：逐字段篡改测试、历史版本链兼容、多实例竞争与审计库故障用例；首页按真实保证描述。
- **投入**：中。首页措辞本次已修订，运行保证待落实。

### A06 · P2 · 数据库路由首次打开忽略取消且缓存缺少容量边界

- **证据**：`internal/db/router/router.go:GameDB(_ context.Context, ...)` 丢弃 ctx；Open/EnsureDatabase/MigrateGame 回调没有 context 参数；缓存按物理库长期持有，只有显式 Forget/Close 清理。
- **影响**：慢建库或迁移时调用方取消不能中止首次打开；很多 game/env 会积累连接池。属于规模和故障时的资源风险，不是已测出的性能瓶颈。
- **改法**：context-aware resolver/open/migrate，加开库预算和取消等待；增加池数量、活跃连接和首次打开耗时指标，再按规模决定空闲回收。
- **验收**：取消等待快速返回；相同库 singleflight 不被破坏；不同库互不阻塞；连接池上限可观测。
- **投入**：中。A04 后做。

### A07 · P2 · 分析发布端的 Kafka 选项不等于内置端到端支持

- **证据**：`internal/analytics/mq/factory.go:NewFromEnv` 支持 redis/kafka/noop；`internal/analytics/worker/worker.go:NewWorker/Run` 直接使用 Redis client 与 XReadGroup。
- **影响**：仅切换 ingest 的 ANALYTICS_MQ_TYPE=kafka，内置 worker 不会随之消费 Kafka；需要额外 consumer/桥接。
- **改法**：优先在配置和文档标出内置 worker 只消费 Redis；只有实际部署需求明确时才扩展消费适配接口。
- **验收**：不支持的组合启动前可被识别；默认 Redis→worker→ClickHouse 闭环测试通过；Kafka 若声称支持必须有完整消费验证。
- **投入**：小（约束与说明），中到大（完整 Kafka 消费）。
- **状态**：ARCH-013 已收口（2026-10-30）——`worker.NewWorker` 启动预检在 `ANALYTICS_MQ_TYPE=kafka` 时返回错误拒绝启动，compose 三模板把 `ANALYTICS_MQ_TYPE` 传给 analytics-worker 使预检可见；默认（redis/未设）链不受影响，noop 仍为显式本地调试；不扩建 Kafka consumer，Kafka 完整消费验证仍不在范围。

### A08 · P2 · 模块边界值得渐进收口，不值得立即大拆

- **证据**：`internal/platform/functioninvoke/invoke.go` 反向依赖 `internal/api/assignment`；`ServiceContext` 同时暴露大量 model/服务/后台生命周期。审核时 service_context.go 约 1824 行、routes.go 1599 行、dispatcher.go 1576 行、agent/app.go 1185 行。行数只是导航信号，依赖方向才是问题依据。
- **改法**：先把 assignment gate 下沉到不依赖 HTTP 的领域包；按 auth/scope、execution、pages、ops、analytics 分装配模块；给公共 core/agents 禁止 internal 依赖设置 CI 检查。保留现有部署拓扑。
- **验收**：platform 执行链不 import api；公共包依赖检查可执行；原函数/页面/审批行为测试维持不变。
- **投入**：中，分批；前置修复完成后再做。

## 文档与代码差异清单

| 文档表述/位置                                                   | 源码事实                                                                        | 本次处理或后续任务                           |
| --------------------------------------------------------------- | ------------------------------------------------------------------------------- | -------------------------------------------- |
| 首页/架构总览：Agent↔Server 默认 TLS                            | 模板 control.cert/key 为空，Agent insecure=true；root.go 由证书是否存在决定 TLS | 首页与新总览改为“支持 TLS，依配置启用”       |
| 首页：所有操作审计、哈希链防篡改                                | 审计按路径/政策；哈希覆盖部分字段；默认 signer=nil                              | 首页收窄承诺；A05                            |
| HA 文档图：实例互联 mTLS                                        | 正文已披露明文，源码也明文                                                      | 新首页图按当前状态画；历史设计图需标目标态   |
| 总览仅三组件，所有 Agent 都接 SDK                               | sidecar 接 SDK；capture/synthetics/devops 有独立角色                            | 新总览增加角色及成熟度边界                   |
| agents/README：devops 仅骨架                                    | runner 已装 Broker/Hub/白名单热更新，但面板中继未接，RDP/VNC 拒绝               | 后续维护能力矩阵，不把局部实现说成全链路可用 |
| CLAUDE.md：数据库绑定表驱动物理路由                             | 当前 Router 仍重算物理名                                                        | A04，不仅改文字掩盖实现差距                  |
| “完整统一 session 生命周期”容易理解为所有角色都自动拥有所有机制 | 公共 transport + 各角色显式装配 register/backoff 等                             | 总览区分公共机制与角色业务生命周期           |
| 双语架构页以空“阅读路径”结尾                                    | 缺少可用后续入口                                                                | 本次补实际链接与源文件索引                   |
| VitePress ignoreDeadLinks=true                                  | 构建成功不能证明站内链接正确                                                    | A09 文档专项校验                             |

## 排除的误报与保留边界

- **没有确认 HTTP 可直接注入审批绕过标记**：`internal/api/function/dto.go` Metadata 是 `json:"-"`。内部 trusted metadata 可进一步改为类型化 context，但不列为已证实 HTTP 漏洞。
- **没有确认跨实例任务取消缺失**：`cmd/server/root.go` 注入 TaskAgentLookup；dispatcher 本地未命中后查 task_runs，再 owner forward。
- **没有将文件 chain.Writer 当成生产审计实现**：生产装配为 SQLAuditStore + AuditService，避免把未接线工具的问题升级为生产结论。
- **没有将 gRPC 依赖当作 Agent 主链路仍用 gRPC 的证据**：主链路已追踪到自建 TCP；插件/生态依赖须单独看用途。
- 页面存在受控 draft 执行入口，因此“Console 发布态执行”和“所有入口都仅允许已发布页面”不是同一个断言。
- 本次没有声称 SDK 六语言完全等价、没有测量生产性能，也没有证明现有所有 game-scoped model 都完整按 env 隔离。

## 值得做的执行清单

| 顺序 | 编号 | 工作                               | 完成标准                           | 依赖             |
| ---- | ---- | ---------------------------------- | ---------------------------------- | ---------------- |
| 1    | A01  | 审批 SQL 装配 + 原子状态与执行领取 | 跨实例、重启、并发重复批准不丢不重 | 无               |
| 2    | A02  | 集群 mTLS 与已认证连接身份         | 非 Server 对端不能访问 forward     | 网络/证书配置    |
| 3    | A03  | live/ready 与强制 HA 模式          | 故障节点从 LB 摘除，恢复语义可验   | A02 可并行       |
| 4    | A04  | 分库绑定真值与名称唯一性           | 旧库与新库路由不漂移、不碰撞       | 启用分库前       |
| 5    | A05  | 审计 payload 版本与失败策略        | 关键字段篡改可检出，失败不静默     | 兼容既有链       |
| 6    | A07  | 分析配置支持矩阵                   | Kafka 发布与 Redis 消费边界明确    | 无               |
| 7    | A06  | 连接预算/取消/容量监控             | 超时取消与池数量有证据             | A04              |
| 8    | A08  | 领域边界和装配拆分                 | 无 api 反向依赖，行为不变          | 关键缺陷先完成   |
| 9    | A09  | 双语文档与架构图防漂移             | 检查端口、TLS、角色、链接及图资源  | 本次文档作为基线 |

A09 建议检查新改页面的站内链接与资源，逐步清理全站死链后再撤销 ignoreDeadLinks；不要一次打开全站 strict 检查阻塞无关修复。新增架构图保留生成脚本与桌面/手机两种布局。

**暂不做**：拆成大量微服务、重写成 gRPC、迁移所有存储引擎、给 devops 补全所有远程桌面协议、为技术完整性补 Kafka consumer。只有明确需求与验证数据支持时再推进。

## 验证记录

2026-10-29 完成验证（审核开始于 2026-10-28）：

- `pnpm --dir docs build` 通过；已有 chunk size、Sass legacy API 与部分语法高亮语言提示，不阻断构建。
- `go test ./internal/cluster ./internal/audit ./internal/db/router ./internal/platform/approvals -count=1` 四包通过。首次沙箱运行因禁止监听本机 socket 失败，经授权在沙箱外重跑通过。
- Chromium 检查四张 SVG 的画布与卡片文本边界通过；中英文首页在 1440px 与 390px 下加载正确 SVG，无页面横向溢出，`/croupier/` base 路径正确。
- SVG XML 解析、新总览阅读路径目标存在性与 `git diff --check` 检查通过。全站仍开启 ignoreDeadLinks，本次没有宣称全站无死链。
- 本次未运行全仓 Go/Web 测试、生产部署或故障演练；这是一份文档与审核交付，定向测试通过不代表上述问题已修复。

所有 P1/P2 是审核结论和待办，不表示已修复。
