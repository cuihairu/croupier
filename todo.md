# Croupier TODO（进度与未完成项汇总）

> 说明：此列表基于仓库内的文档“进行中/待实现/路线图”和代码中的 TODO/FIXME 线索整理；按优先级（P0 最高）归类。末尾附“已完成（摘录）”用于整体进度对齐。

<!-- progress-summary:start -->
## 进度概览

| 分类 | 未完成 | 已完成 | 完成率 |
| --- | ---: | ---: | ---: |
| P0 | 0 | 15 | 100.0% |
| P1 | 11 | 26 | 70.3% |
| P2 | 18 | 0 | 0.0% |
| P3 | 12 | 0 | 0.0% |
| P3b | 6 | 0 | 0.0% |
| P4 | 200 | 27 | 11.9% |
| 总计 | 247 | 68 | 21.6% |

| 范围 | 未完成 |
| --- | ---: |
| Docs | 136 |
| Dashboard | 23 |
| SDKs | 27 |
| Internal | 9 |
| SpecWorkflow | 18 |
| Services | 8 |
| Tools | 0 |
| Docker | 6 |
| Other | 14 |
| Cmd | 1 |
| Configs | 2 |
| Scripts | 2 |
| Proto | 1 |
<!-- progress-summary:end -->



## 下一步（建议顺序）

- Proto-First：完善 `protoc-gen-croupier` 对自定义 options 的映射（auth/semantics/ui/labels 等）`tools/protoc-gen-croupier/main.go:72`
- Proto-First：支持 `emit_manifest=true`（生成 `manifest.json`、`schema/*.json`、可选 `.desc`）`docs/providers-manifest.md:99`
- TLS：打通配置→证书加载→拨号/监听（Agent/Dispatcher/Edge/Server 统一路径）`services/agent/etc/agent.yaml:1`
- Server：启动 gRPC ControlService（mTLS）并与 go-zero HTTP 控制面收敛 `services/server/cmd/root.go:96`
- Jobs：job 路由持久化/重启恢复策略（避免 Server/Edge 重启后无法查询）`internal/platform/dispatch/dispatcher.go:18`
- Dashboard：Entities 的 JSON Schema 编辑体验增强（编辑器/预览/校验联动）`dashboard/src/pages/Entities/index.tsx:140`

## 使用方式

- 只在真正完成代码/文档/验证后，把对应条目标记为 `- [x]`
- 修改任务内容时，优先保留路径引用（`path:line`）便于回溯
- 如果某项决定“不做/移除”，建议把理由写到该行末尾（例如 `// wontfix: ...`）而不是直接删除

## 快速索引

- P0：影响可用性/用户体验的缺口
- P1：核心能力（控制面/描述符/低代码）
- P2：SDK（以 C++ 为主）
- P3：Analytics 路线图（Worker/Ingest/ClickHouse）
- P3b：平台长期能力（规划）
- P4：工程化与一致性（持续项）
- 文档 checklist（详细）：把各文档内的 checklist 未完成项逐条展开
- Done：已完成（摘录）

## 本地验证

- `go test ./...`（当前通过；多数包显示 `[no test files]`）
- `go vet ./...`（当前无输出）
- `make lint`（当前 `Nothing to be done for lint`）

## 任务拆分（按领域）

- Dashboard/UX：鉴权跳转、Jobs 统计、导出、Traces、筛选字典、插件加载、mock 回退策略
- 控制面/Jobs：终态事件判定、job 路由持久化/探测策略、StreamJob/StartJob 一致性、状态命名统一
- 安全/TLS：Agent↔Server、Dispatcher↔Agent、Edge、adapters、SDK 的 TLS/mTLS 全链路打通
- 工程化/Docker：`web/` vs `dashboard/` 目录不一致、compose flags/entrypoint 对齐、demo Dockerfile 修复
- 文档：大量 checklist 未闭环（见下方“文档 checklist（详细）”汇总表）

## P0 - 影响可用性/用户体验的缺口

### 执行指引（先做什么）

- `ErrorShowType.REDIRECT`：统一未登录/无权限的跳转目标（login/403），并在 requestErrorConfig 中补齐 redirect 分支
- `runningJobs`：后端已有 `/api/v1/jobs`（dashboard ops.ts），直接按 `status=running` 拉取并写入 stats
- `exportToXLSX`：要么引入 xlsx 依赖并按需 import，要么 UI 只提供 CSV 并移除 XLSX 入口
- `FunctionFormRenderer tabs`：明确是否需要 tabs layout；需要则实现 ui schema 分组渲染，不需要则删掉空分支/改为显式不支持提示
- `Pack plugins`：决定“插件来自 pack assets 还是后端 registry”，给出加载协议与缓存策略；否则移除占位日志
- `Telemetry Traces`：优先做“跳转 Grafana/Jaeger Explore URL”（零后端改动）；再考虑新增后端 traces 查询 API
- Analytics filters（Attribution/Payments）：短期改为 tags 输入/可搜索；长期接入后端字典/枚举接口
- `VirtualObjectManager mock fallback`：接口失败时不要静默回退到 mockEntities，改为 toast+空态+重试

- [x] Dashboard：组件统计里的 `runningJobs` 改为从 Job API 拉取（例如 `/api/v1/jobs?status=running`）`dashboard/src/pages/ComponentManagement/index.tsx:114`
- [x] Dashboard：实现 `ErrorShowType.REDIRECT` 的跳转逻辑（未登录/无权限等场景）`dashboard/src/requestErrorConfig.ts:61`
- [x] Dashboard：表单渲染支持 `ui:layout.type=tabs` 的分组布局 `dashboard/src/components/FunctionFormRenderer/index.tsx:162`
- [x] Dashboard：Pack plugins 支持从 `/api/v1/packs` 读取 `web_plugins` 并动态加载 `dashboard/src/plugin/registry.tsx:94`
- [x] Analytics Retention：留存页面已接入 `/api/v1/analytics/retention`（cohort 表格 + 导出）`dashboard/src/pages/Analytics/Retention/index.tsx:1`
- [x] Analytics：未交付页面不展示在菜单（例如 Segments 占位页已隐藏）`dashboard/config/routes.ts:28`
- [x] Analytics Attribution：后端接口未实现，页面从菜单隐藏并增加明确提示 `dashboard/config/routes.ts:27`
- [x] Analytics Payments：筛选项支持“输入+联想”（AutoComplete），后端无字典时也可直接输入 `dashboard/src/pages/Analytics/Payments/index.tsx:126`
- [x] Ops Notifications：移除/不展示 `SMS(占位)` 等误导选项 `dashboard/src/pages/Ops/Notifications/index.tsx:100`
- [x] 导出：`exportToXLSX` 不再依赖 `window.XLSX`（多 sheet 将下载多个 CSV）`dashboard/src/utils/export.ts:1`
- [x] Agent：FunctionServer 在无实例/拨号失败时返回明确 gRPC 状态码 `internal/app/agent/function_server.go:31`
- [x] Dashboard Telemetry：Traces 页面移除 mock 数据，改为跳转 Grafana/Jaeger `dashboard/src/pages/Telemetry/Traces.tsx:1`
- [x] Dashboard Telemetry：通过 `/api/ops/config` 下发 `grafana_explore_url/jaeger_url` 供跳转使用 `dashboard/src/services/croupier/ops.ts:100`
- [x] Dashboard ComponentManagement：VirtualObjectManager 接口失败不再静默回退 mock，改为提示+空态 `dashboard/src/pages/ComponentManagement/components/VirtualObjectManager.tsx:113`
- [x] Build：修复 `make server` 产物为 `ar archive`（当前在 `services/server` 下构建 `./cmd` 非 `main` 包）；应改为构建 `services/server`（`package main`）并验证生成的 `bin/croupier-server` 可执行 `Makefile:71`

## P1 - 核心能力（控制面/描述符/低代码）

### 执行指引（先做什么）

- Provider Manifest：先在 `RegisterCapabilities` 入口做 schema 校验 + 返回结构化错误；再补齐“merge descriptors”的冲突策略与测试用例
- Proto-First：先把 `protoc-gen-croupier` 的 options 映射打通（auth/ui/labels）；再做 emit_manifest/schema 输出，保证与 Provider Manifest 设计一致
- Jobs/Dispatch：先把 `StreamJob`/终态判定/路由清理闭环跑通；再考虑 jobRouting 持久化/重启恢复策略
- TLS：优先统一“配置→证书加载→拨号/监听”的单一路径（避免各处各自实现）
- 文档对齐：先修最影响运行/入门的（config/README/docker/compose），再处理历史评审类文档

- [x] Provider Manifest：`RegisterCapabilities` 接收到的 manifest 做 JSON Schema 校验（`docs/providers-manifest.schema.json`），失败返回结构化错误 `internal/platform/control/server.go:77`
- [x] Provider Manifest：补齐“合并为统一 descriptors 并暴露”的验收用例/测试（provider/descriptor 聚合、覆盖优先级、冲突处理）`internal/platform/control/server.go:112`
- [x] Proto-First 生成：完善 `protoc-gen-croupier` 对自定义 options 的映射（auth/semantics/ui/labels 等），并补齐文档/示例 proto `tools/protoc-gen-croupier/main.go:72`
- [x] Proto-First 生成：支持 `emit_manifest=true` 开关，并生成 `schema/*.json`（JSON Schema）与可选 `.desc`（FDS），与 Provider Manifest 设计对齐 `docs/providers-manifest.md:99`
- [ ] Entity 管理界面：增强 Schema 编辑体验（完整 JSON Schema 编辑器、UI 配置工具等）`dashboard/src/pages/Entities/index.tsx:140`
- [ ] 函数管理 UX：补齐搜索/分类/批量操作/版本管理等（文档列出的缺口）`docs/FUNCTION_MANAGEMENT_ARCHITECTURE_ANALYSIS.md:57`
- [ ] 函数管理：落地“统一函数管理菜单 + 5 个专注页面 + 重定向兼容”（阶段 1 交付物）`docs/FUNCTION_MANAGEMENT_EXECUTIVE_SUMMARY.md:246`
- [ ] 函数管理：新增/补齐调用历史（数据模型 + API + UI 展示 + rerun）`docs/FUNCTION_MANAGEMENT_EXECUTIVE_SUMMARY.md:246`
- [ ] 函数管理：新增 Agents/覆盖率分析相关 API（agents 列表、agent functions、coverage analysis）`docs/FUNCTION_MANAGEMENT_EXECUTIVE_SUMMARY.md:246`
- [ ] Packs：新增包内容详情/版本历史/灰度发布（canary）API 与 UI `docs/FUNCTION_MANAGEMENT_EXECUTIVE_SUMMARY.md:246`
- [x] 文档自洽：`docs/ARCHITECTURE.md` 引用“TODO List”但正文缺失；补 TODO 列表或删引用 `docs/ARCHITECTURE.md:302`
- [x] 文档自洽：`docs/ARCHITECTURE.md` 中“实体管理(规划中)”与当前 Dashboard 已有 Entities 页面不一致；更新描述/标注现状 `docs/ARCHITECTURE.md:257`
- [x] Approvals 存储：补齐 PG/SQLite 的 Store 实现（或删除/替换 placeholder 接口），避免运行期只能用内存 `internal/platform/approvals/store.go:140`
- [x] Prom adapter：补齐 StartJob（或在 descriptors/UI 中禁用异步能力并清晰返回 NotImplemented）`tools/adapters/prom/main.go:97`
- [x] HTTP adapter：`StartJob` 目前返回空 jobId；实现异步作业或明确返回 NotImplemented（并补齐 Cancel/Stream 行为）`tools/adapters/http/main.go:213`
- [x] HTTP adapter：注册到 Agent 时仍使用 `grpc.WithInsecure()`；按 Agent mTLS 配置打通 TLS/mTLS `tools/adapters/http/main.go:248`
- [x] HTTP adapter：实现了 `grafana.search_dashboards` 但注册到 Agent 的 functions 列表未包含该 id；补齐注册或移除实现避免“存在但不可发现” `tools/adapters/http/main.go:254`
- [x] HTTP adapter：本地 gRPC server 直接 `grpc.NewServer()`（无 TLS）；按本地接入/零信任要求启用 TLS/mTLS（并与 Agent 注册配置对齐）`tools/adapters/http/main.go:239`
- [x] Prom adapter：注册到 Agent 时仍使用 `grpc.WithInsecure()`；按 Agent mTLS 配置打通 TLS/mTLS `tools/adapters/prom/main.go:138`
- [x] Prom adapter：本地 gRPC server 直接 `grpc.NewServer()`（无 TLS）；按本地接入/零信任要求启用 TLS/mTLS（并与 Agent 注册配置对齐）`tools/adapters/prom/main.go:128`
- [x] Agent FunctionService：实现 `StreamJob`（当前 agent 侧未实现，会影响 job 实时流式能力）`internal/app/agent/function_server.go:1`
- [x] Jobs：完善终态事件判定（至少包含 canceled/failed），避免 `StreamJob` 不收敛导致 jobRouting 不清理 `internal/platform/dispatch/dispatcher.go:187`
- [x] Jobs：统一 job event/type 命名与状态映射（例如 canceled vs cancelled、completed vs succeeded），避免 Edge/SDK/UI 展示与终态判断不一致 `sdks/go/pkg/croupier/function_server.go:97`
- [x] Jobs：job 路由当前仅存内存（`jobID -> agent addr`），Server/Edge 重启后无法查询历史 job；需要持久化或实现“全量探测/回退策略” `internal/platform/dispatch/dispatcher.go:18`
- [ ] Agent↔Server：当前使用 insecure gRPC（无 mTLS），与“零信任/mTLS”设计不一致；补齐 TLS 配置与证书加载 `internal/app/agent/upstream.go:73`
- [ ] Dispatcher↔Agent：dispatcher 侧也使用 insecure gRPC 直连 agent；补齐 TLS/mTLS dial（可复用 `internal/platform/tlsutil`）`internal/platform/dispatch/dispatcher.go:259`
- [x] ConnPool：`InsecureSkipVerify` 配置语义不正确（当前会直接使用明文 insecure credentials）；应改为 `credentials.NewTLS(&tls.Config{InsecureSkipVerify:true})` 或更名为 `InsecurePlaintext` `internal/connpool/pool.go:243`
- [ ] TLS 配置落地：虽然 `services/agent/etc/agent.yaml` 与 `services/edge/etc/edge.yaml` 有 TLS/CA/Insecure 配置，但 `internal/app/agent/*`、dispatcher 等路径未实际使用；打通配置→拨号→证书加载链路 `services/agent/etc/agent.yaml:1`
- [x] Agent Upstream：使用了已废弃/不推荐的 `grpc.WithTimeout`；改为 `DialContext` + ctx 超时，并与 TLS 配置统一 `internal/app/agent/upstream.go:80`
- [x] Edge：gRPC server 目前未配置 TLS（直接 `grpc.NewServer()`），需要根据 `services/edge/internal/config` 启用 TLS/mTLS `services/edge/cmd/root.go:99`
- [ ] Edge 配置语义：`Server.InternalAddr` 当前被当作“gRPC 监听地址”使用（net.Listen），但字段命名像“上游地址”；明确 listen/upstream 拆分并更新配置/代码 `services/edge/cmd/root.go:95`
- [x] Server：启动 gRPC ControlService（支持 mTLS：配置 CA 则要求客户端证书；未配证书时 dev 模式仍允许明文）`services/server/cmd/root.go:96`
- [ ] Server/Agent/Edge：目前存在两套“Agent 注册/ControlService”实现（go-zero HTTP 的 server vs internal/app/edge 的 gRPC 控制面），需要明确哪套是主路径并收敛（避免 registry/dispatcher 分叉）`internal/app/edge/app.go:24`
- [x] Edge：Edge gRPC server 未启用 TLS/mTLS（直接 `grpc.NewServer()`），需要支持 mTLS 并与配置/证书对齐 `services/edge/cmd/root.go:99`
- [x] Agentlocal：LocalStore 的 `Prune` 从未调用，实例/函数可能永久残留；增加定时清理与 maxAge 配置 `internal/platform/agentlocal/store.go:130`
- [x] Agent Upstream：store.OnUpdate 回调当前每次变更都触发 sync（且使用 `context.Background()`），需要 debounce/合并并增加超时/重试策略 `internal/app/agent/upstream.go:71`
- [x] Agentlocal：LocalControlService proto 含 `GetJobResult`，但 server 未实现；补齐实现或从 proto/调用链中移除 `internal/platform/agentlocal/local_control.go:1`

## P2 - SDK（以 C++ 为主）

### 执行指引（先做什么）

- C++：先把 invoker 的 gRPC 真实连通（Invoke/StartJob/StreamJob/CancelJob）与错误码对齐，再做 JSON schema 校验与 descriptor 加载
- Go：先补齐 TLS/mTLS（证书加载 + dial/server creds），再清理库内 `fmt.Print*` 输出与 mock build-tag 分叉策略
- 多语言 proto 同步：明确“根 proto 为单一真源”，用脚本/CI 校验各 SDK proto 目录一致

- [ ] C++ SDK：Invoker 实现真实 gRPC 连接与调用（Invoke/StartJob/StreamJob/CancelJob 目前为模拟）`sdks/cpp/src/croupier_client.cpp:678`
- [ ] C++ SDK：补齐 Invoker 的具体 gRPC 调用实现（Invoke/StartJob/CancelJob 等仍是 TODO）`sdks/cpp/src/croupier_client.cpp:705`
- [ ] C++ SDK：补齐其余 gRPC 调用点（多处仍是 TODO）`sdks/cpp/src/croupier_client.cpp:725` `sdks/cpp/src/croupier_client.cpp:787`
- [ ] C++ SDK：补齐 StreamJob 的流式实现（当前为 TODO）`sdks/cpp/src/croupier_client.cpp:749`
- [ ] C++ SDK：补齐 schema 校验实现（当前 ValidateJSON 为 placeholder，且部分参数被忽略）`sdks/cpp/src/croupier_client.cpp:38`
- [ ] C++ SDK：补齐 JSON 文件读取与解析（LoadObjectDescriptor/LoadComponentDescriptor）`sdks/cpp/src/croupier_client.cpp:912`
- [ ] C++ SDK：补齐 LoadComponentDescriptor 解析（当前为 TODO）`sdks/cpp/src/croupier_client.cpp:925`
- [ ] C++ SDK：补齐 JSON 解析（ParseJSON 等仍是 TODO/placeholder）`sdks/cpp/src/croupier_client.cpp:1059` `sdks/cpp/src/croupier_client.cpp:1072`
- [ ] C++ SDK：补齐资源/配置序列化与更健壮的 JSON 解析（当前为 placeholder/手写拼接）`sdks/cpp/src/croupier_client.cpp:1152`
- [ ] C++ SDK：补齐 config 序列化（当前为 TODO/placeholder）`sdks/cpp/src/croupier_client.cpp:1153`
- [ ] C++ SDK：动态加载器可选增强——扫描符号自动发现函数（注释 TODO）`sdks/cpp/src/plugin/dynamic_loader.cpp:644`
- [ ] C++ SDK：补齐 Heartbeat/注册相关的实现与文档对齐（目前标注“待实现/不实现”不一致）`sdks/cpp/src/grpc_service.cpp:180`
- [ ] C++ SDK：构建与 CI 优化（离线 proto、统一 CMakeLists、补 CodeQL/静态分析等）`docs/CPP_SDK_ANALYSIS.md:750`
- [ ] Go SDK：gRPC dial 默认使用 `insecure.NewCredentials()`；补齐 TLS/mTLS 配置（至少支持 CA/客户端证书/ServerName）并与服务端配置对齐 `sdks/go/pkg/croupier/client.go:331`
- [ ] Go SDK：`createTLSCredentials`/`createServerTLSCredentials` 目前是占位实现（忽略 CAFile/CertFile/KeyFile）；补齐证书加载与 mTLS 校验 `sdks/go/pkg/croupier/grpc_manager.go:253`
- [ ] Go SDK：库代码包含大量 `fmt.Printf/Println`（会污染使用方 stdout）；改为可注入 logger / debug 开关（示例程序可保留输出）`sdks/go/pkg/croupier/client.go:132`
- [ ] Go SDK：proto 生成脚本只在 CI 模式运行（本地直接 return 并提示“mock gRPC”）；补齐本地可用的生成方式/文档，避免“本地无法更新 proto” `sdks/go/scripts/generate_proto.go:224`
- [ ] Go SDK：mock gRPC 的 build tag（`croupier_mock_grpc`）与默认实现并存，容易造成构建/行为分叉；明确策略（保留但文档化 or 移除）并在 CI 校验 `sdks/go/internal/grpc_manager.go:1`

## P3 - Analytics 路线图（Worker/Ingest/ClickHouse）

### 执行指引（先做什么）

- M1：先补 ingest 的限流/超时/body size 限制与结构化日志，再补 worker pending/重放/死信，最后做 ClickHouse 分区/物化视图
- M2：先把入口/消费/写入指标打通（QPS/429/积压/延迟），再做 schema 注册与高基数治理
- M3：先明确 Kafka 迁移路径（dual-write/回放/切换窗口），再做多租户限额与预置看板

- [ ] M1 稳定性：Ingest 限流/熔断、超时与并发配置、全链路日志与 SLO `docs/analytics/enhancement-plan.md:1`
- [ ] Ingest：补齐基础治理能力（rate limit/熔断/请求体大小限制、结构化日志、SLO 指标暴露）`services/ingest/cmd/root.go:1`
- [ ] Ingest：目前缺少 HTTP server 超时与 body size 限制（如 ReadHeaderTimeout/IdleTimeout/MaxBytesReader），存在 DoS 风险 `services/ingest/cmd/root.go:69`
- [ ] M1 稳定性：Worker 批处理参数自适应、重试与死信队列、消费延迟告警 `docs/analytics/enhancement-plan.md:1`
- [ ] Worker：Redis Streams consumer group 未处理 pending/重放（缺少 `XAUTOCLAIM`/dead-letter），崩溃/重启后可能丢处理或积压；补齐 pending 恢复与错误策略 `internal/analytics/worker/worker.go:145`
- [ ] M1 稳定性：ClickHouse 分区/排序键检查、物化视图落地 P95/P99 聚合 `docs/analytics/enhancement-plan.md:1`
- [ ] M2 治理：入口/消费/写入指标与告警（QPS/429/积压/延迟）`docs/analytics/enhancement-plan.md:1`
- [ ] M2 治理：事件 schema 注册与校验、维度白名单与高基数采样 `docs/analytics/enhancement-plan.md:1`
- [ ] M2 治理：TTL/冷热分层/自动归档 `docs/analytics/enhancement-plan.md:1`
- [ ] M3 规模化：Kafka 替代 Redis Streams、Worker 水平扩展与分组 `docs/analytics/enhancement-plan.md:1`
- [ ] M3 规模化：多租户与限额管理（按游戏/环境/渠道）`docs/analytics/enhancement-plan.md:1`
- [ ] M3 规模化：预置看板模板与 A/B 评估组件 `docs/analytics/enhancement-plan.md:1`

## P3b - 平台长期能力（规划）

### 执行指引（先做什么）

- 先把多租户的“边界”定义清楚（数据模型、权限、资源隔离层级：game/env/tenant），再做 Composite/Relationship/Workflow 等高级特性
- 插件生态优先确定打包/签名/兼容策略（版本、依赖、权限声明），再做市场/模板库

- [ ] 进阶特性：Composite Entity（组合实体）`docs/VIRTUAL_OBJECT_DESIGN.md:925`
- [ ] 进阶特性：Entity Relationship（实体关系）`docs/VIRTUAL_OBJECT_DESIGN.md:925`
- [ ] 进阶特性：Workflow Orchestration（工作流编排）`docs/VIRTUAL_OBJECT_DESIGN.md:925`
- [ ] 进阶特性：Dynamic Entity 生成 `docs/VIRTUAL_OBJECT_DESIGN.md:925`
- [ ] 多租户：租户级别的组件/数据/权限隔离 `docs/ARCHITECTURE.md:304`
- [ ] 插件生态：第三方组件市场/模板库/社区贡献机制 `docs/ARCHITECTURE.md:304`

## P4 - 工程化与一致性（持续项）

### 执行指引（先做什么）

- Docker/Compose：先修 “web vs dashboard” 目录不一致与 compose command/flags 对齐，让 `docker compose up -d` 可用
- 配置：先把 `services/*/etc/*.yaml` 的 YAML 语法问题修掉（例如 `//` 注释），再统一 config 模型与文档
- 安全：先禁用/改造危险脚本（push/force-push），再补 CodeQL/SAST 与 CI 校验
- 测试：从 RBAC/鉴权、manifest 校验、dispatch/jobs 三块补最小单测，避免后续大改无法回归

- [ ] 补齐关键单测：RBAC/鉴权、Entity/Manifest 校验、ComponentManager、Registry/Dispatch、Jobs（当前 `go test ./...` 绝大多数包无测试）
- [ ] Docs：清理/收敛文档中的残留 TODO 注释（与实际代码/计划对齐，避免误导读者）`docs/CPP_SDK_DEEP_ANALYSIS.md:250`
- [ ] Docs：C++ SDK 文档索引中存在 “TODO: Register with agent via gRPC” 片段；补齐实现链接或移除/替换为当前方案 `docs/CPP_SDK_DIRECTORY_INDEX.md:259`
- [ ] Docs：虚拟对象 Quick Reference 引用 `TODO.md`（大小写/路径）需与仓库实际 `todo.md` 对齐 `docs/VIRTUAL_OBJECT_QUICK_REFERENCE.md:437`
- [ ] 清理/完成仓库内文档 checklist：函数管理系统重构部署清单剩余 40 项（部署与验收项）`docs/函数管理系统重构部署清单.md:1`
- [ ] 清理/完成仓库内文档 checklist：spec workflow 模板任务剩余 17 项（模板与规范项）`.spec-workflow/templates/tasks-template.md:1`
- [ ] 清理/完成仓库内文档 checklist：Dashboard 文档索引剩余 14 项（文档补齐/校对）`dashboard/WEB_DOCUMENTATION_INDEX.md:1`
- [ ] 清理/完成仓库内文档 checklist：函数管理系统重构完成总结剩余 12 项（总结项待补齐/核验）`docs/函数管理系统重构完成总结.md:1`
- [ ] 清理/完成仓库内文档 checklist：函数管理 Quick Reference 剩余 12 项（落地步骤/页面/接口）`docs/FUNCTION_MANAGEMENT_QUICK_REFERENCE.md:1`
- [ ] 清理/完成仓库内文档 checklist：C++ SDK 分析文档剩余 11 项（实现/验证缺口）`docs/CPP_SDK_ANALYSIS.md:1`
- [ ] 清理/完成仓库内文档 checklist：C++ SDK 分析摘要剩余 9 项（工程化/发布缺口）`docs/CPP_SDK_ANALYSIS_SUMMARY.md:1`
- [ ] 清理/完成仓库内文档 checklist：VSCode Setup 剩余 7 项（开发环境配置）`SETUP_VSCODE.md:1`
- [ ] 清理/完成仓库内文档 checklist：SDK 注册流程剩余 7 项（多语言 SDK 对齐）`sdks/SDK_REGISTRATION_FLOW.md:1`
- [ ] 清理/完成仓库内文档 checklist：C++ SDK Docs Index 剩余 7 项（文档索引缺口）`docs/CPP_SDK_DOCS_INDEX.md:1`
- [ ] 清理/完成仓库内文档 checklist：Dashboard 前端分析剩余 6 项（前端缺口跟进）`dashboard/FRONTEND_ANALYSIS.md:1`
- [ ] 清理/完成仓库内文档 checklist：虚拟对象 Quick Reference 剩余 4 项（对齐 TODO 引用/落地）`docs/VIRTUAL_OBJECT_QUICK_REFERENCE.md:1`
- [x] 统一文档元信息：`docs/README.md` 的 License/Go 版本与仓库实际不一致（LICENSE=Apache-2.0；go.mod=go1.25）`docs/README.md:4`
- [x] Node 版本对齐：`docs/README.md` 要求 Node.js 18+，但 `dashboard/package.json` engines 要求 `>=22` `docs/README.md:59`
- [x] Go 版本对齐：README/CI/go.mod 的 Go 版本声明不一致（README=1.25+；go.mod=1.25；CI=1.25）`README.md:1`
- [x] Go 工具链对齐：CI 使用 Go 1.25，但 `go.mod` 为 1.25；确认是否升级 go.mod 到 1.25 或降低 CI `go-version` `go.mod:3`
- [x] 安全：补齐 CodeQL/SAST 工作流（仓库已有文档提及但 CI 未集成）`.github/workflows/codeql.yml:1`
- [x] 文档：C++ SDK README 补齐 Troubleshooting/FAQ（文档评估里标注缺失）`docs/sdks/cpp/README.md:1`
- [x] 文档发布：`docs/README.md` 标注“函数管理系统章节为草稿，稍后发布”，需要补齐/发布到 docs 站点或移除占位描述 `docs/README.md:104`
- [x] 构建跨平台：`make analytics-spec` 依赖 PowerShell 脚本，补齐 macOS/Linux 可运行的替代实现（或改为 Node/Go 工具）`Makefile:130`
- [x] 仓库结构文档对齐：AGENTS/README 中提到 `web/`（Umi Max）但当前仓库实际使用 `dashboard/`（子模块）且无 `web/` 目录 `AGENTS.md:1`
- [x] Docker：`docker/Dockerfile.web` 依赖 `web/` 目录，但仓库实际无 `web/`（当前是 `dashboard/`）；需要更新 Dockerfile/compose 或补齐 web 目录 `docker/Dockerfile.web:1`
- [x] Docker Compose：`docker/docker-compose.yml` 的 `web` service 依赖 Dockerfile.web + `web/` 目录；当前无法构建/启动，需要同步修复 `docker/docker-compose.yml:142`
- [x] Docker Compose：server/edge/agent 的 command 使用 `--addr/--http_addr/--cert/--key/--ca` 等 flags，但 go-zero 版本的 `services/*/cmd` 可能不支持这些 flags（只支持 `--config` 等）；需要对齐镜像 entrypoint 与参数 `docker/docker-compose.yml:87`
- [x] Docker 文档对齐：`docker/README.md` 的 quickstart/端口/服务列表基于旧 CLI flags 与旧 web 目录，需与当前实现（go-zero + dashboard 子模块）一致 `docker/README.md:1`
- [x] Dockerfile：`docker/Dockerfile.demo` 构建路径指向 `./cmd/demo/main.go`，但仓库无该文件（实际 demo 在 `services/demo/main.go`）；修正构建入口 `docker/Dockerfile.demo:1`
- [x] Docker Compose Telemetry：`docker-compose.telemetry.yaml` 依赖 `Dockerfile.demo`，当前无法构建 demo；修复后验证 compose 可拉起 demo `docker/docker-compose.telemetry.yaml:100`
- [ ] 文档/命令对齐：`docs/analytics/README.md` 的 quickstart 仍引用旧 `./bin/croupier server --config ...` 命令，和当前实际二进制/参数体系不一致；更新示例 `docs/analytics/README.md:119`
- [ ] 文档/命令对齐：多处文档引用 `./bin/croupier ...`（assignments/packs 等 CLI），但仓库当前没有该统一 CLI 二进制；需要实现 CLI（或改文档为现有工具 `schema-validator`/`pack-builder`）`docs/assignments.md:22`
- [ ] 配置/文档对齐：`docs/config.md` 描述的 Cobra+Viper 多文件 include/profile 体系与当前 go-zero `conf.MustLoad` 不一致；需要统一实现或更新文档 `docs/config.md:1`
- [ ] CLI：文档里存在 `croupier config test`（合并 include/profile 并校验配置）等命令，但仓库缺少对应二进制/入口；实现统一 CLI（复用 `internal/cli/common`）或删改文档 `docs/config.md:146`
- [ ] DB 迁移：存在 `cmd/server/migrate_wip.go.txt` 草稿但未落地到可用命令；决定实现迁移子命令或移除草稿避免误导 `cmd/server/migrate_wip.go.txt:1`
- [x] 文档对齐：`docs/README.md` 中的组件路径仍指向 `internal/server|agent|edge`，但当前入口在 `services/*` 与 `internal/app/*`，需要更新 `docs/README.md:34`
- [x] 文档对齐：`services/README.md` 的 go-zero 服务规划与仓库当前实际目录不一致（例如 services/api 未落地）`services/README.md:1`
- [x] 文档对齐：`architecture_review.md` 的“无法编译/目录不存在”结论可能已过期；更新为当前状态或标注历史背景 `architecture_review.md:1`
- [ ] 权限策略对齐：`configs/rbac_policy.csv` 中的 HTTP 路径与当前 API 前缀（`/api/v1/...`）可能不一致；需要统一或兼容路由 `configs/rbac_policy.csv:1`
- [ ] RBAC 注释对齐：economy_manager “REST endpoints not implemented” 是否仍成立；若已实现则补授权，若未实现则补实现或移除相关角色/文档 `configs/rbac_policy.csv:44`
- [ ] Demo 服务：metrics endpoint 目前为 placeholder；明确用途并实现或移除 `services/demo/main.go:132`
- [ ] 默认种子数据：`Bootstrap placeholder game` 这类默认数据需明确是否仅用于 dev（生产环境禁用/可配置）`services/server/internal/svc/game_seed.go:157`
- [ ] 脚本安全：`scripts/test-ci.sh` 会直接 push/force-push，风险较高；改为“本地校验/提示用户手动操作”或移除 `scripts/test-ci.sh:1`
- [ ] 脚本安全：`scripts/sync-sdk-generated.sh` 会在脚本内提示后执行 `git push`（对子模块/多仓库场景风险较高）；考虑增加 `--dry-run`、默认不推送或在 CI 里禁止推送 `scripts/sync-sdk-generated.sh:175`
- [x] 配置文件有效性：`services/edge/etc/edge.yaml` 使用 `//` 注释（非 YAML 语法），会导致解析失败；改为 `#` 或移除 `services/edge/etc/edge.yaml:35`
- [ ] 日志与噪音：移除/替换 agentlocal 的 `fmt.Printf("DEBUG: ...")`（改为可控的结构化日志或仅在 debug level 输出）`internal/platform/agentlocal/store.go:37`
- [ ] 日志与噪音：移除/替换 LocalControlService 的 `fmt.Printf("DEBUG: RegisterLocal...")` `internal/platform/agentlocal/local_control.go:25`
- [ ] Telemetry：OTLP exporter 默认 `WithInsecure()`，需要根据配置显式区分 dev/prod，支持 TLS（至少提供开关与 CA 配置）`internal/telemetry/provider.go:96`
- [ ] Telemetry：配置解析目前对 float/int/duration 是硬编码有限集合（parseFloat/parseIntOrDefault/parseDurationOrDefault），需改为 `strconv`/`time.ParseDuration` 并对非法值报错/回退 `internal/telemetry/provider.go:202`
- [ ] 配置体系统一：`internal/config` 与 `services/*/etc/*.yaml`（go-zero）存在两套配置模型，且字段不一致（如 TLS/端口/日志）；需要明确“单一真源”与迁移策略 `internal/config/types.go:8`
- [ ] Proto：Public Management API 目前是 placeholder，明确是否需要对外暴露/实现或删除 `proto/croupier/api/v1/management.proto:1`
- [ ] SDK Proto 同步：各语言 SDK 目录下的 `proto/croupier/api/v1/management.proto` 同样是 placeholder；明确“以根 proto 为准”的同步策略（自动同步脚本/CI 校验）`sdks/go/proto/croupier/api/v1/management.proto:1`
- [ ] JS SDK 示例：`Basic client is a placeholder` 的示例需要补齐可运行实现或删除/标注限制 `sdks/js/examples/main.ts:179`

### 文档 checklist（详细）

<!-- doc-checklist-summary:start -->

| 文档 | 未完成项 |
| --- | ---: |
| `docs/函数管理系统重构部署清单.md` | 40 |
| `.spec-workflow/templates/tasks-template.md` | 17 |
| `dashboard/WEB_DOCUMENTATION_INDEX.md` | 14 |
| `docs/函数管理系统重构完成总结.md` | 12 |
| `docs/FUNCTION_MANAGEMENT_QUICK_REFERENCE.md` | 12 |
| `docs/CPP_SDK_ANALYSIS.md` | 11 |
| `docs/CPP_SDK_ANALYSIS_SUMMARY.md` | 9 |
| `SETUP_VSCODE.md` | 7 |
| `sdks/SDK_REGISTRATION_FLOW.md` | 7 |
| `docs/CPP_SDK_DOCS_INDEX.md` | 7 |
| `dashboard/FRONTEND_ANALYSIS.md` | 6 |
| `docs/VIRTUAL_OBJECT_QUICK_REFERENCE.md` | 4 |

<!-- doc-checklist-summary:end -->



#### docs/函数管理系统重构部署清单.md

- [ ] 开发环境测试通过 `docs/函数管理系统重构部署清单.md:13`
- [ ] 测试环境部署验证 `docs/函数管理系统重构部署清单.md:14`
- [ ] 生产环境配置准备 `docs/函数管理系统重构部署清单.md:15`
- [ ] 数据库备份完成 `docs/函数管理系统重构部署清单.md:16`
- [ ] 回滚方案准备 `docs/函数管理系统重构部署清单.md:17`
- [ ] 函数目录页面正常显示 `docs/函数管理系统重构部署清单.md:118`
- [ ] 函数调用功能正常工作 `docs/函数管理系统重构部署清单.md:119`
- [ ] 实例管理页面数据正确 `docs/函数管理系统重构部署清单.md:120`
- [ ] 调用历史记录完整 `docs/函数管理系统重构部署清单.md:121`
- [ ] 权限控制生效 `docs/函数管理系统重构部署清单.md:122`
- [ ] 旧版本GmFunctions页面仍可访问 `docs/函数管理系统重构部署清单.md:125`
- [ ] URL重定向正常工作 `docs/函数管理系统重构部署清单.md:126`
- [ ] 数据格式兼容 `docs/函数管理系统重构部署清单.md:127`
- [ ] API向后兼容 `docs/函数管理系统重构部署清单.md:128`
- [ ] 页面加载时间 < 3秒 `docs/函数管理系统重构部署清单.md:131`
- [ ] 搜索响应时间 < 500ms `docs/函数管理系统重构部署清单.md:132`
- [ ] 内存使用正常 `docs/函数管理系统重构部署清单.md:133`
- [ ] 无内存泄漏 `docs/函数管理系统重构部署清单.md:134`
- [ ] 函数目录页面显示正常 `docs/函数管理系统重构部署清单.md:264`
- [ ] 搜索功能工作正常 `docs/函数管理系统重构部署清单.md:265`
- [ ] 函数调用执行成功 `docs/函数管理系统重构部署清单.md:266`
- [ ] 实例状态监控正确 `docs/函数管理系统重构部署清单.md:267`
- [ ] 调用历史数据完整 `docs/函数管理系统重构部署清单.md:268`
- [ ] 权限控制生效 `docs/函数管理系统重构部署清单.md:269`
- [ ] 国际化显示正确 `docs/函数管理系统重构部署清单.md:270`
- [ ] 页面加载时间 < 3秒 `docs/函数管理系统重构部署清单.md:273`
- [ ] API响应时间 < 500ms `docs/函数管理系统重构部署清单.md:274`
- [ ] 内存使用稳定 `docs/函数管理系统重构部署清单.md:275`
- [ ] CPU使用率正常 `docs/函数管理系统重构部署清单.md:276`
- [ ] 网络请求优化 `docs/函数管理系统重构部署清单.md:277`
- [ ] Chrome浏览器兼容 `docs/函数管理系统重构部署清单.md:280`
- [ ] Firefox浏览器兼容 `docs/函数管理系统重构部署清单.md:281`
- [ ] Safari浏览器兼容 `docs/函数管理系统重构部署清单.md:282`
- [ ] 移动端适配正常 `docs/函数管理系统重构部署清单.md:283`
- [ ] 旧版本URL重定向 `docs/函数管理系统重构部署清单.md:284`
- [ ] 权限检查正确 `docs/函数管理系统重构部署清单.md:287`
- [ ] 数据验证有效 `docs/函数管理系统重构部署清单.md:288`
- [ ] XSS防护正常 `docs/函数管理系统重构部署清单.md:289`
- [ ] CSRF防护生效 `docs/函数管理系统重构部署清单.md:290`
- [ ] 敏感信息脱敏 `docs/函数管理系统重构部署清单.md:291`

#### .spec-workflow/templates/tasks-template.md

- [ ] 1. Create core interfaces in src/types/feature.ts `.spec-workflow/templates/tasks-template.md:3`
- [ ] 2. Create base model class in src/models/FeatureModel.ts `.spec-workflow/templates/tasks-template.md:12`
- [ ] 3. Add specific model methods to FeatureModel.ts `.spec-workflow/templates/tasks-template.md:21`
- [ ] 4. Create model unit tests in tests/models/FeatureModel.test.ts `.spec-workflow/templates/tasks-template.md:30`
- [ ] 5. Create service interface in src/services/IFeatureService.ts `.spec-workflow/templates/tasks-template.md:39`
- [ ] 6. Implement feature service in src/services/FeatureService.ts `.spec-workflow/templates/tasks-template.md:48`
- [ ] 7. Add service dependency injection in src/utils/di.ts `.spec-workflow/templates/tasks-template.md:57`
- [ ] 8. Create service unit tests in tests/services/FeatureService.test.ts `.spec-workflow/templates/tasks-template.md:66`
- [ ] 4. Create API endpoints `.spec-workflow/templates/tasks-template.md:75`
- [ ] 4.1 Set up routing and middleware `.spec-workflow/templates/tasks-template.md:81`
- [ ] 4.2 Implement CRUD endpoints `.spec-workflow/templates/tasks-template.md:89`
- [ ] 5. Add frontend components `.spec-workflow/templates/tasks-template.md:97`
- [ ] 5.1 Create base UI components `.spec-workflow/templates/tasks-template.md:103`
- [ ] 5.2 Implement feature-specific components `.spec-workflow/templates/tasks-template.md:111`
- [ ] 6. Integration and testing `.spec-workflow/templates/tasks-template.md:119`
- [ ] 6.1 Write end-to-end tests `.spec-workflow/templates/tasks-template.md:125`
- [ ] 6.2 Final integration and cleanup `.spec-workflow/templates/tasks-template.md:133`

#### dashboard/WEB_DOCUMENTATION_INDEX.md

- [ ] 读 QUICK_REFERENCE "文件快速定位" 部分 `dashboard/WEB_DOCUMENTATION_INDEX.md:179`
- [ ] 复制 CONFIGURATION_EXAMPLE 中的代码到 5 个文件 `dashboard/WEB_DOCUMENTATION_INDEX.md:180`
- [ ] 创建 src/pages/YourPage/index.tsx `dashboard/WEB_DOCUMENTATION_INDEX.md:181`
- [ ] 参考 EXAMPLE_USERS_PAGE.tsx 实现页面逻辑 `dashboard/WEB_DOCUMENTATION_INDEX.md:182`
- [ ] 重启开发服务器: pnpm dev `dashboard/WEB_DOCUMENTATION_INDEX.md:183`
- [ ] 浏览器访问 http://localhost:8000 测试 `dashboard/WEB_DOCUMENTATION_INDEX.md:184`
- [ ] 在 src/access.ts 中定义新的权限函数 `dashboard/WEB_DOCUMENTATION_INDEX.md:187`
- [ ] 在路由中使用 access 属性(隐藏菜单) `dashboard/WEB_DOCUMENTATION_INDEX.md:188`
- [ ] 在页面中用权限函数控制按钮(禁用/隐藏) `dashboard/WEB_DOCUMENTATION_INDEX.md:189`
- [ ] 测试没有权限的用户看不到菜单/按钮 `dashboard/WEB_DOCUMENTATION_INDEX.md:190`
- [ ] 在 src/services/croupier/index.ts 中定义 API 函数 `dashboard/WEB_DOCUMENTATION_INDEX.md:193`
- [ ] 在页面中 import 该函数 `dashboard/WEB_DOCUMENTATION_INDEX.md:194`
- [ ] 使用 try-catch 处理错误 `dashboard/WEB_DOCUMENTATION_INDEX.md:195`
- [ ] 用 message.success/error 提示用户 `dashboard/WEB_DOCUMENTATION_INDEX.md:196`

#### docs/函数管理系统重构完成总结.md

- [ ] WebSocket实时更新 `docs/函数管理系统重构完成总结.md:214`
- [ ] 函数性能监控 `docs/函数管理系统重构完成总结.md:215`
- [ ] 批量操作工具 `docs/函数管理系统重构完成总结.md:216`
- [ ] 导入导出功能 `docs/函数管理系统重构完成总结.md:217`
- [ ] 可视化流程设计 `docs/函数管理系统重构完成总结.md:220`
- [ ] AI辅助函数开发 `docs/函数管理系统重构完成总结.md:221`
- [ ] 多租户完整支持 `docs/函数管理系统重构完成总结.md:222`
- [ ] 移动端原生应用 `docs/函数管理系统重构完成总结.md:223`
- [ ] 云原生架构升级 `docs/函数管理系统重构完成总结.md:226`
- [ ] 微前端架构演进 `docs/函数管理系统重构完成总结.md:227`
- [ ] 低代码平台集成 `docs/函数管理系统重构完成总结.md:228`
- [ ] 生态系统扩展 `docs/函数管理系统重构完成总结.md:229`

#### docs/FUNCTION_MANAGEMENT_QUICK_REFERENCE.md

- [ ] 更新菜单配置 `docs/FUNCTION_MANAGEMENT_QUICK_REFERENCE.md:218`
- [ ] 创建 5 个新页面目录 `docs/FUNCTION_MANAGEMENT_QUICK_REFERENCE.md:219`
- [ ] 创建后向兼容重定向 `docs/FUNCTION_MANAGEMENT_QUICK_REFERENCE.md:220`
- [ ] 新增 3 个 API 端点 `docs/FUNCTION_MANAGEMENT_QUICK_REFERENCE.md:221`
- [ ] 重构 Invoke 页面 `docs/FUNCTION_MANAGEMENT_QUICK_REFERENCE.md:226`
- [ ] 集成调用历史 API `docs/FUNCTION_MANAGEMENT_QUICK_REFERENCE.md:227`
- [ ] 增强 Assignments 管理 `docs/FUNCTION_MANAGEMENT_QUICK_REFERENCE.md:228`
- [ ] 增强 Packs 管理 `docs/FUNCTION_MANAGEMENT_QUICK_REFERENCE.md:229`
- [ ] 版本对比工具 `docs/FUNCTION_MANAGEMENT_QUICK_REFERENCE.md:234`
- [ ] 细粒度权限实现 `docs/FUNCTION_MANAGEMENT_QUICK_REFERENCE.md:235`
- [ ] 可视化监控 `docs/FUNCTION_MANAGEMENT_QUICK_REFERENCE.md:236`
- [ ] 性能优化 `docs/FUNCTION_MANAGEMENT_QUICK_REFERENCE.md:237`

#### docs/CPP_SDK_ANALYSIS.md

- [ ] C++17 编译器已安装 (GCC 8+, Clang 10+, MSVC 2019+) `docs/CPP_SDK_ANALYSIS.md:872`
- [ ] CMake 3.20+ 已安装 `docs/CPP_SDK_ANALYSIS.md:873`
- [ ] vcpkg 已配置（可选但推荐） `docs/CPP_SDK_ANALYSIS.md:874`
- [ ] 网络连接正常（Proto 下载需要） `docs/CPP_SDK_ANALYSIS.md:875`
- [ ] 足够的磁盘空间 (~2GB vcpkg, ~800MB 优化后) `docs/CPP_SDK_ANALYSIS.md:876`
- [ ] 交叉编译工具链已安装（如需跨平台构建） `docs/CPP_SDK_ANALYSIS.md:877`
- [ ] 选择使用哪个 GitHub Actions 工作流 (cpp-sdk-build.yml 或 optimized-build.yml) `docs/CPP_SDK_ANALYSIS.md:881`
- [ ] 验证预生成 Proto 文件是否已提交 (sdks/cpp/generated/) `docs/CPP_SDK_ANALYSIS.md:882`
- [ ] 配置 GitHub Actions 缓存（加速构建） `docs/CPP_SDK_ANALYSIS.md:883`
- [ ] 设置发布权限（GITHUB_TOKEN） `docs/CPP_SDK_ANALYSIS.md:884`
- [ ] 测试离线构建模式 `docs/CPP_SDK_ANALYSIS.md:885`

#### docs/CPP_SDK_ANALYSIS_SUMMARY.md

- [ ] C++17 编译器安装 `docs/CPP_SDK_ANALYSIS_SUMMARY.md:210`
- [ ] CMake 3.20+ 安装 `docs/CPP_SDK_ANALYSIS_SUMMARY.md:211`
- [ ] vcpkg 配置 (可选但推荐) `docs/CPP_SDK_ANALYSIS_SUMMARY.md:212`
- [ ] 网络连接 (Proto 下载) `docs/CPP_SDK_ANALYSIS_SUMMARY.md:213`
- [ ] 磁盘空间 (~2GB) `docs/CPP_SDK_ANALYSIS_SUMMARY.md:214`
- [ ] 选择工作流 (cpp-sdk-build vs optimized-build) `docs/CPP_SDK_ANALYSIS_SUMMARY.md:217`
- [ ] 验证预生成文件 (sdks/cpp/generated/) `docs/CPP_SDK_ANALYSIS_SUMMARY.md:218`
- [ ] 配置 GitHub Actions 缓存 `docs/CPP_SDK_ANALYSIS_SUMMARY.md:219`
- [ ] 设置发布权限 `docs/CPP_SDK_ANALYSIS_SUMMARY.md:220`

#### SETUP_VSCODE.md

- [ ] 安装所有推荐插件 `SETUP_VSCODE.md:151`
- [ ] 配置Go语言环境 `SETUP_VSCODE.md:152`
- [ ] 安装goctl CLI工具 `SETUP_VSCODE.md:153`
- [ ] 配置工作区设置 `SETUP_VSCODE.md:154`
- [ ] 创建调试配置 `SETUP_VSCODE.md:155`
- [ ] 测试API生成功能 `SETUP_VSCODE.md:156`
- [ ] 验证Proto文件支持 `SETUP_VSCODE.md:157`

#### sdks/SDK_REGISTRATION_FLOW.md

- [ ] All SDKs use identical data structures aligned with proto `sdks/SDK_REGISTRATION_FLOW.md:286`
- [ ] All SDKs implement two-layer registration (SDK→Agent→Server) `sdks/SDK_REGISTRATION_FLOW.md:287`
- [ ] All SDKs support multi-tenant isolation (game_id/env) `sdks/SDK_REGISTRATION_FLOW.md:288`
- [ ] All SDKs support dual build modes (local/CI) `sdks/SDK_REGISTRATION_FLOW.md:289`
- [ ] All SDKs have consistent configuration options `sdks/SDK_REGISTRATION_FLOW.md:290`
- [ ] All SDKs handle errors in the same manner `sdks/SDK_REGISTRATION_FLOW.md:291`
- [ ] All SDKs provide similar API patterns in their respective languages `sdks/SDK_REGISTRATION_FLOW.md:292`

#### docs/CPP_SDK_DOCS_INDEX.md

- [ ] 您已了解项目的基本目标（虚拟对象注册） `docs/CPP_SDK_DOCS_INDEX.md:305`
- [ ] 您有相关的 C++ 开发基础 `docs/CPP_SDK_DOCS_INDEX.md:306`
- [ ] 您熟悉 CMake 和构建系统概念 `docs/CPP_SDK_DOCS_INDEX.md:307`
- [ ] 您理解 gRPC 和 Protobuf 的基本概念 `docs/CPP_SDK_DOCS_INDEX.md:308`
- [ ] 先查看 CPP_SDK_QUICK_REFERENCE.md（常见问题） `docs/CPP_SDK_DOCS_INDEX.md:311`
- [ ] 再查看 CPP_SDK_ANALYSIS.md（问题点详解） `docs/CPP_SDK_DOCS_INDEX.md:312`
- [ ] 最后查看相关源代码文件 `docs/CPP_SDK_DOCS_INDEX.md:313`

#### dashboard/FRONTEND_ANALYSIS.md

- [ ] 1. 更新 `config/routes.ts` 添加路由 + 权限 `dashboard/FRONTEND_ANALYSIS.md:569`
- [ ] 2. 在 `src/locales/en-US/menu.ts` 和 `zh-CN/menu.ts` 添加菜单标签 `dashboard/FRONTEND_ANALYSIS.md:570`
- [ ] 3. 创建 `src/pages/YourPage/index.tsx` 页面组件 `dashboard/FRONTEND_ANALYSIS.md:571`
- [ ] 4. 在 `src/services/croupier/index.ts` 添加 API 调用函数 `dashboard/FRONTEND_ANALYSIS.md:572`
- [ ] 5. 在 `src/access.ts` 定义权限检查函数 `dashboard/FRONTEND_ANALYSIS.md:573`
- [ ] 6. 测试权限(有权限用户能看到菜单/按钮) `dashboard/FRONTEND_ANALYSIS.md:574`

#### docs/VIRTUAL_OBJECT_QUICK_REFERENCE.md

- [ ] **Entity定义** `docs/VIRTUAL_OBJECT_QUICK_REFERENCE.md:326`
- [ ] **Function定义** `docs/VIRTUAL_OBJECT_QUICK_REFERENCE.md:333`
- [ ] **Resource定义** `docs/VIRTUAL_OBJECT_QUICK_REFERENCE.md:341`
- [ ] **Component清单** `docs/VIRTUAL_OBJECT_QUICK_REFERENCE.md:347`

## Done - 已完成（摘录）

- [x] 虚拟对象四层模型（Entity/Function/Resource/Component）与 Schema 驱动验证机制落地 `docs/VIRTUAL_OBJECT_DESIGN.md:868`
- [x] 组件管理：安装/卸载/启用/禁用 + 依赖检查 + HTTP API 集成 `docs/ARCHITECTURE.md:271`
- [x] 已有组件（player/item/economy/entity-management）基础能力与后端聚合 API（entities/descriptors/providers 等）`docs/VIRTUAL_OBJECT_DESIGN.md:868`
- [x] Dashboard：Entity CRUD + Preview（基础版本）`dashboard/src/pages/Entities/index.tsx:1`
- [x] ControlService：支持 `RegisterCapabilities` 接收 provider manifest（gzip JSON）并入库到 registry `internal/platform/control/server.go:76`
- [x] `protoc-gen-croupier` 生成 `manifest.json`（基础 functions 列表）`tools/protoc-gen-croupier/main.go:205`

### 文档标注已完成（未核验）

- [x] **架构设计文档**: 完整的系统架构说明 `docs/函数管理系统重构完成总结.md:194`
- [x] **API文档**: 所有接口的详细说明 `docs/函数管理系统重构完成总结.md:195`
- [x] **组件文档**: 每个组件的使用指南 `docs/函数管理系统重构完成总结.md:196`
- [x] **部署文档**: 详细的部署和配置指南 `docs/函数管理系统重构完成总结.md:197`
- [x] **用户指南**: 新界面的使用说明 `docs/函数管理系统重构完成总结.md:200`
- [x] **功能对比**: 新旧功能对比 `docs/函数管理系统重构完成总结.md:201`
- [x] **常见问题**: FAQ和故障排除 `docs/函数管理系统重构完成总结.md:202`
- [x] **最佳实践**: 使用建议和技巧 `docs/函数管理系统重构完成总结.md:203`
- [x] **代码规范**: 开发标准和约定 `docs/函数管理系统重构完成总结.md:206`
- [x] **测试指南**: 测试编写和执行 `docs/函数管理系统重构完成总结.md:207`
- [x] **贡献指南**: 如何参与项目开发 `docs/函数管理系统重构完成总结.md:208`
- [x] **版本管理**: 版本发布和更新流程 `docs/函数管理系统重构完成总结.md:209`
- [x] 所有组件开发完成 `docs/函数管理系统重构部署清单.md:6`
- [x] 单元测试编写完成 `docs/函数管理系统重构部署清单.md:7`
- [x] 文档和示例完成 `docs/函数管理系统重构部署清单.md:8`
- [x] 类型定义完整 `docs/函数管理系统重构部署清单.md:9`
- [x] 向后兼容性验证 `docs/函数管理系统重构部署清单.md:10`
- [x] 新界面使用指南 `docs/函数管理系统重构部署清单.md:256`
- [x] 功能对比文档 `docs/函数管理系统重构部署清单.md:257`
- [x] 视频教程链接 `docs/函数管理系统重构部署清单.md:258`
- [x] 常见问题解答 `docs/函数管理系统重构部署清单.md:259`
