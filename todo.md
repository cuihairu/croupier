# Croupier 当前待办

更新时间：2026-07-11

本清单只记录当前仍需推进的事项。已完成的历史迁移压缩到末尾，避免干扰优先级判断。

## 当前判断

项目主干的 Server/Agent session、分库路由、任务调度已有基础闭环；当前明显滞后的是“发布级闭环”：

1. 跨语言 SDK 一致性没有收尾。
2. CI 缺少可执行的最小 E2E。
3. API 面铺得较宽，但部分包没有测试。
4. 文档和协议里仍有历史 `gRPC` / `rpc_addr` / `Job` 命名残留。
5. `ServiceContext` 仍承担过多组合根职责，需要按领域继续收敛。

本阶段不以向后兼容为约束。历史协议、旧命名、旧文档可以直接删除或重命名；如必须暂留，必须写清删除条件和负责人。

## P0：SDK 与协议一致性收尾

- [x] **补齐 JS/TS L3 Invoker。** `sdks/js/src/invoker.ts` 已落地：独立 HTTP 调用方模块，支持 `invoke` / `startTask` / `streamTask`（SSE 轮询）/ `cancelTask`，与 Provider Client 解耦；`index.ts` 导出 `Invoker`/`createInvoker`/`InvokerError`；`SDK_FEATURE_MATRIX.md` Invoker 表 JS 标 ✅。
  - 位置：`sdks/js/src/invoker.ts`、`sdks/js/src/index.ts`、`sdks/SDK_FEATURE_MATRIX.md`
  - 验证：JS SDK CI `success`；`scripts/check-sdk-matrix.sh` exit 0、无 JS Invoker warning。

- [x] **删除 C++ SDK legacy wire name 残留。** `protocol.h` 已删 `MSG_START_JOB_REQUEST` / `MSG_STREAM_JOB_REQUEST` / `MSG_CANCEL_JOB_REQUEST` / `MSG_JOB_EVENT` / `MSG_GET_JOB_RESULT_REQUEST/RESPONSE(旧 0x0201xx)` 等别名，补 `MSG_GET_TASK_RESULT_REQUEST/RESPONSE (0x050107/8)`；`croupier_client.cpp` 形参 `grpc_status_code`→`status_code`；Lua binding `start_job`→`start_task` 等。
  - 位置：`sdks/cpp/include/croupier/sdk/protocol.h`、`sdks/cpp/src/croupier_client.cpp`、`sdks/cpp/src/bindings/lua_binding_sol2.cpp`
  - 验证：`rg "MSG_START_JOB|MSG_STREAM_JOB|MSG_CANCEL_JOB|MSG_JOB_EVENT|MSG_GET_JOB_RESULT_REQUEST" sdks/cpp/` 零命中；C++ SDK CI `success`。

- [x] **把 SDK matrix warning 升级为失败条件。** `scripts/check-sdk-matrix.sh` 的 warning 分支改为 `exit 1`（含 JS Invoker 缺失、C++ legacy wire name、旧 README 术语）；CI 的 SDK matrix conformance 步骤据此可阻断旧命名新增。
  - 位置：`scripts/check-sdk-matrix.sh`、`.github/workflows/ci.yml`
  - 验证：`bash scripts/check-sdk-matrix.sh; echo $?` → exit 0（当前仓库无 warning）；脚本逻辑为 warning 即 1。

- [~] **清理 `Job` 命名，统一为 `Task`（主链路 + SDK 手写层完成；generated 滞后待收尾）。**
  - ✅ 已完成：Go/Python/JS/C++/C# 的手写层、proto、`internal/`、文档、示例已统一 `Task`；C# 测试 `TaskStatus` 与 `System.Threading.Tasks.TaskStatus` 碰撞已全限定修复。
  - ✅ 已收尾：用 `buf generate` 重新生成入库 stub，使仓库码 == proto 当前态。`sdks/python/generated` 刷新到 Task/ProviderConnect schema；`sdks/csharp/generated` regen（JobEvent→TaskEvent，清 ~2500 行旧码）+ 修 `csharp/buf.gen.yaml` out 路径（`../generated`→`sdks/csharp/generated`）；`sdks/csharp/src/Croupier.Sdk/CroupierInvoker.cs` 的 Job→Task 引用同步更新（StartJobResponse→StartTaskResponse 等）；删除废弃的 `sdks/python/croupier/pb`（手选旧集，含已删 proto，运行时不使用——`__init__.py` 经 `_load_proto_module` 从 `generated/` 加载）+ 清 `pyproject.toml` 引用。CI - Python SDK / C# SDK / Core / Docker / CodeQL 全绿。
  - 验证（主链路）：`rg "StartJob|StreamJob|CancelJob|JobEvent|GetJobResult|job_id" --glob '!**/generated/**' --glob '!**/pb/**' --glob '!*.md' sdks/ proto/ internal/` 零命中。

- [x] **清理 `rpc_addr` / LocalControl / gRPC callback 兼容字段。**
  - ✅ 已完成：主链路路由零依赖 rpc_addr；运行时无 gRPC 反向回拨；LocalControl 概念清理；Python SDK 手写层清理。
  - ✅ schema 级清理（三波，全 CI 绿）：
    - 批1后端：`server.AgentSession` 删 RPCAddr + 加 `Addr()`（conn.RemoteAddr）；`reg.AgentSession` RPCAddr→Addr（runtime RemoteAddr）；`handleRegisterRequest` 加 remoteAddr 参数（不再 req.RpcAddr）；DB 两个 AgentSessionDB 删 RPCAddr not null 列 + `MigrateAgentSessions` DropColumn；Addr 不持久化（重连重新建立）。
    - 批2 proto+SDK：`register.proto` 删 rpc_addr 字段 4 + reserved；buf regen go/cpp/csharp/python；agent upstream 删 UpstreamMetadata.Addr/legacyRPCAddr/RpcAddr 注册。
    - 批3 DTO+前端：ops/registry/logic DTO 删 RPCAddr/RpcAddr compatibility（留 Addr）；前端 rpcAddr→addr（Ops pages + services）；rate_limit 删 raw.rpc_addr（后端从未暴露）。
  - 验证：`rg "rpc_addr|RPCAddr|RpcAddr" internal/ proto/ web/src --glob '!**/.umi/**'` 仅注释/归档（SDK generated 已无 rpc_addr）。

## P0：恢复发布级 E2E

- [x] **恢复 CI 最小 E2E job。** 根因是旧 job 用 `configs/server.yaml`（mysql/postgres）在无 DB 的 CI runner 启动失败；改为 `configs/test-sqlite.yaml`（无外部 DB，admin 从 configs/*.json seed），删除 `if: ${{ false }}`，新增确定性 readiness probe（60s 轮询 /healthz），跑 `scripts/e2e/happy-path.sh`。
  - 位置：`.github/workflows/ci.yml`（e2e job）、`scripts/e2e/happy-path.sh`、`configs/test-sqlite.yaml`
  - 验证：本地 `happy-path.sh` 5/5 通过（healthz + auth + /games + /ops/agents + /tasks）；CI 在 PR 上自动执行、失败阻断合并。

- [x] **新增 Server-Agent-SDK happy path E2E。**
  - ✅ Server 启动 + 健康检查 + auth + REST surface 在 `happy-path.sh` 覆盖。
  - ✅ Agent TCP 注册握手（首帧 Register + Heartbeat）由 `examples/cmd/e2e-agent-probe` 落地：复用生产 `internal/transport/tcp.Client`（而非重写协议），发 RegisterRequest → 校验 RegisterResponse.SessionId → 发 HeartbeatRequest 验证握手后链路。CI e2e job 已加 `go build -o bin/e2e-agent-probe`，`happy-path.sh` 的 agent TCP 步骤从 skip 变 PASS。
  - 验证：本地 `happy-path.sh` 6/6 PASS（含 agent TCP register handshake）；握手状态机单元测试 `TestAgentSessionHandler` 覆盖未注册 Heartbeat 拒绝 / 重复 Register 拒绝 / 跨 agent_id 拒绝。

- [x] **新增 Task lifecycle E2E。**
  - 覆盖路径：`startTask` → 轮询 `/tasks/:id/events`（started/progress/completed）→ `cancelTask`（cancel_requested/cancelled）→ 状态落库。
  - 实现：`scripts/e2e/task-lifecycle.sh` + `examples/cmd/e2e-agent-probe`（serve 模式 mock agent：复用 `internal/transport/tcp.MuxConn` 接 StartTask/Cancel + 流 TaskEvent）+ `examples/cmd/e2e-function-seed`（插 function 元数据行，因无 create API）。CI e2e job 接入，本地 8/8 PASS。
  - 附带修复（server bug）：`internal/server/tcp_listener.go` handleRegister 现在把声明的 functions 写入 dispatcher registry（调 `ControlService.handleRegisterRequest`）。之前 TCP 注册的 functions 完全丢弃，dispatcher 永远报 `no live agent for function`——这是阻断所有 agent function 路由的真实 regression。
  - 注：无真 SSE；`GET /tasks/:id/stream` 不存在，实际端点是 `/events` 轮询（从 DB 读 `task_runs`/`task_events`）。

## P1：API 测试覆盖补齐

- [x] **为无同包测试的 API 包补 smoke/contract tests。** 15 个原无测试的包全部补齐（workspace/schema/storage/functioncall/rate_limit 各 handler+service 测试；entity/faq/feedback/message/meta/node/routes/terms/ticket/user 各 handler 测试），共 161 个测试，含一个 node handler bug 修复（UpdateMeta 缺 JSON 绑定）。
  - 位置：`internal/api/{workspace,schema,storage,functioncall,rate_limit,entity,faq,feedback,message,meta,node,routes,terms,ticket,user}/*_test.go`
  - 验证：`/usr/local/go/bin/go test ./internal/api/...`（零失败）。

- [x] **建立 API contract guard。** 核心 contract bug 修复：`response.Error` 新增 `gorm.ErrRecordNotFound → 404` 映射（原先 model 层 NotFound 泄漏为 500），补契约测试 `TestError_GormRecordNotFound`。workspace/schema/rate_limit 的 not-found 路径、functioncall/task 的 Detail 现在统一返回 404 + `{"error":"not_found"}`。
  - 位置：`internal/common/response/response.go`、`internal/common/response/response_test.go`
  - 验证：`/usr/local/go/bin/go test ./internal/api/... ./internal/common/response/...`（零失败，零回归）。
  - 🟡 待后续：`functioncall.Service.Detail` 返回字段与 `List` 不一致（丢弃 FunctionID/GameID/Env/AgentID），非 contract 阻塞，单独修。
  - ✅ 已完成：ops handlers 不再返回 envelope `{code,message}`。全部 ~25 个 ops 响应 DTO（agent/backup/alert/silence/node/health/metrics/maintenance/functions/mq/notifications）改为直返业务 payload（`{agents}`/`{functions}`/`{nodes}`/`{metrics}`/...），符合 CLAUDE.md 契约。前端早已期望直返，此修复同时修正前端链路。分 3 批提交。

## P1：文档重新收敛

- [x] **统一 SDK 文档的单一事实源。** `SDK_FEATURE_MATRIX.md`（L1–L4 分层 + 实现状态）与 `docs/sdks/sdk-parity-matrix.md`（Required/Optional/Forbidden 基线）定位互补，加交叉引用明确主从；历史模型仅以 `Forbidden`/迁移说明出现，不作默认链路。
  - 位置：`sdks/SDK_FEATURE_MATRIX.md`、`docs/sdks/sdk-parity-matrix.md`
  - 验证：文档不把 `gRPC`/`LocalControl`/`rpc_addr` 作为默认链路。

- [x] **清理 SDK README 的旧构建/旧链路描述。** 6 个 README 删除"gRPC 作为真实/生产传输"的描述（确认所有 SDK 运行时均无 gRPC 传输代码，go.mod 零 grpc 依赖），改写为 TCP session 单一链路；按 L1 Provider / L3 Invoker / 语言扩展分层。
  - 位置：`sdks/{go,cpp,java,js}/README.md`、`sdks/cpp/COMPLETE_SDK_README.md`、`sdks/js/examples/README.md`
  - 验证：`scripts/check-sdk-matrix.sh` exit 0；`rg -i "gRPC|rpc_addr|LocalControl" sdks/*/README.md` 零命中。
  - 🟡 代码脚手架残留（文档已清，代码待协调，非 README）：`sdks/go/scripts/generate_proto.go` + `PROTO_GENERATION.md` 仍描述 `croupier_real_grpc` 构建标签模式（运行时未使用，`pkg/pb` 是 protobuf 消息非 gRPC stub）；`sdks/go/Makefile` 的 `build-with-grpc` 是空目标；`sdks/cpp/src/croupier_client.cpp:1643` 有误导性 `grpc_status_code` 形参名。

- [x] **重新规划 docs 信息架构。** 首页（`docs/index.md`）按用户路径重组：hero actions 去掉「文档治理」主入口，改为「快速开始 / SDK 指南 / API 参考」；「文档导航」表格改为五大路径（快速开始 / SDK / API / 运维 / 开发），历史迁移文档标为架构侧栏折叠项（不作接入主路径）。现有 sidebar 结构已按用户路径组织（guide/api/sdks/development/analytics），无需大改。
  - 验证：本地 `pnpm build`（vitepress）通过（20s，无错）；首页不再以迁移文档为主路径。
  - 🟡 待后续：`guide/quick-start` 可补「Server + Agent + 一个 SDK 的最小闭环」端到端示例（当前聚焦 Server+Agent 启动）。

## P1：重构候选，先讨论边界再动代码

- [ ] **继续拆 `ServiceContext`。**
  - 问题：`internal/svc/service_context.go` 聚合 DB、Router、Dispatcher、Cache、Audit、Policy、ObjectStore、Ops、Model、Extension services，组合根过大。
  - 建议：按领域拆窄接口/模块 provider，而不是一次性“大重构”。
  - 优先拆分：
    - Task runtime：Dispatcher、TaskModel、Agent session resolver。
    - Game scope：GameModel、Router、dbctx。
    - Storage：ObjectStore、Storage API、权限校验。
    - Ops：OpsStateStore、MetricsStore、SystemInfoCache。
  - 判断标准：只有当消费者能从 `*svc.ServiceContext` 缩小到领域接口时才算有效重构。
  - 边界讨论结论（2026-07-12）：探查 ~44 指针字段（~20 Model + Dispatcher/Router/Cache/Audit/Policy/ObjectStore/Ops/Extensions），所有 handler/service 依赖全量。先拆 **TaskRuntime** 试点（task lifecycle E2E + server test 覆盖，有测试保护）：定义 `TaskRuntime` 接口（Dispatcher + TaskModel + AgentSessionResolver），task handler/service 依赖该接口而非 `*ServiceContext`。验证模式后扩展 GameScope/Storage/Ops。收益：测试隔离（mock 领域接口）+ 依赖面缩小；风险：大重构，分步。

- [ ] **收敛 API handler/service 模式。**
  - 问题：API 包数量多，handler/service/model 组合方式不完全统一，新增测试成本偏高。
  - 建议：抽取通用 response、binding、pagination、scope 校验模式；避免每个包重复样板。
  - 约束：只抽真实重复，不为了“框架化”引入额外复杂度。
  - 边界讨论结论（2026-07-12）：探查 32 处 `response.Error(c, err)` + 37 处 `ShouldBindJSON/BindQuery` 重复；已有 `internal/common/response` + `requestbind.BindQueryCompat`，但 pagination/scope 校验仍各包重复。先审计 `PaginationOptions` + `RequireGameEnvScope` 调用，抽 1-2 个 helper（`Paginate(req)` + `ResolveScope(c, svcCtx)`），不引框架。收益：新增 handler 成本降 + 一致性；风险：过度抽象。

- [ ] **收敛 Analytics 链路的生产 readiness。**
  - 现状：已有 `cmd/ingest`、`cmd/analytics-worker`、ClickHouse/Redis/Flink 文档和 compose 配置，但需要专项确认端到端与部署成熟度。
  - 建议：单独审计 ingest → MQ → worker → ClickHouse → API 查询链路。
  - 验证：最小 analytics E2E，覆盖事件写入、消费、落库、查询。
  - 边界讨论结论（2026-07-12）：探查 `cmd/ingest` + `cmd/analytics-worker` + `cmd/analytics-export` + `internal/analytics` 存在；compose 配置未在仓库根（需确认部署配置位置）。独立专项，不与 ServiceContext 重构耦合。先审计链路完整性（代码 + 配置 + compose），确认 ingest/worker/export 可启动 + 连 ClickHouse/Redis；再补最小 E2E（docker-compose 起 ClickHouse/Redis）。风险：外部依赖（ClickHouse/Redis/Flink），E2E 需 compose。

## P2：工程治理

- [x] **把“无兼容遗留”写入仓库治理规则。** `docs/development/repository-guidelines.md` 新增「兼容性与遗留治理」节：默认 canonical 命名、默认删除不留别名、暂留须标删除条件+登记 todo+检测脚本、历史只进 `docs/archive/`；`documentation-governance.md` 加交叉引用。检查工具 `scripts/check-sdk-matrix.sh`（warning 即 exit 1）兜底。
  - 位置：`docs/development/repository-guidelines.md`、`docs/development/documentation-governance.md`
  - 验证：规则文档含「无兼容遗留原则」节；`rg "StartJob|RegisterLocal|rpc_addr"` 仅归档/门控/脚本命中。

- [ ] **建立 TODO 审计节奏。**
  - 每次完成事项必须写清：
    - 影响路径
    - 验证命令
    - 是否有残留 warning
  - 已完成事项应移动到“已完成摘要”，不要长期占据 P0/P1。

## 已完成摘要（保留索引，不再作为待办）

- [x] 限制游戏数据库解析与创建：未知 `(game_id, env)` 不再触发建库。
- [x] 修复 Agent 重连时旧连接删除新会话的问题。
- [x] Agent-Server 链路强制首帧 Register 与握手状态机。
- [x] SDK-Agent 首帧规则收敛。
- [x] 控制面优雅停机。
- [x] shared session runtime 抽取，并支持 reconnect-safe remove。
- [x] Router 首次打开 DB 使用 `singleflight`，避免全局锁内 I/O。
- [x] REST Task Start/Cancel 统一走 Dispatcher。
- [x] Agent TaskRunner 抽象已落地。
- [x] 分库边界审计和 `dbctx` 契约测试已落地。
- [x] `ServiceContext` 拆分已有 `ports.Permissions` 锚点。
- [x] 补齐 JS/TS L3 Invoker（`sdks/js/src/invoker.ts`）。
- [x] 删除 C++ SDK legacy wire name 残留（`protocol.h` 等）。
- [x] SDK matrix warning 升级为失败条件（`check-sdk-matrix.sh` warning 即 exit 1）。
- [x] Job→Task 命名：主链路 + SDK 手写层 + proto + 文档统一（generated 滞后收尾见 P0）。
- [x] Python SDK 手写层清理废弃 `rpc_addr` 引用（CI 绿）。
- [x] 新增 `examples/cmd/e2e-agent-probe`：复用生产 `internal/transport/tcp.Client` 完成 Agent→Server 注册握手（Register+Heartbeat）；CI e2e job 接入后 happy-path 6/6 PASS。
- [x] 新增 Task lifecycle E2E：`task-lifecycle.sh` + probe serve 模式（MuxConn mock agent）+ `e2e-function-seed`，CI 接入本地 8/8 PASS。
- [x] 修复 server regression：`tcp_listener.handleRegister` 把 agent 声明的 functions 写入 dispatcher registry（之前丢失，dispatcher 无法路由 function）。

## 最近验证

- [x] 2026-07-11：`/usr/local/go/bin/go test ./...` 通过；所有 CI workflow `success`（CI - Core / Python / C# / C++ / Java / JavaScript SDK / CodeQL / Docker / Nightly / Release）。
- [x] 2026-07-11：`scripts/check-sdk-matrix.sh` exit 0，无 warning（JS Invoker 已补、C++ legacy wire name 已清、warning 已升级为 exit 1）。
- [x] 2026-07-10：`/usr/local/go/bin/go test ./...` 通过。
- [x] 2026-07-10：`scripts/check-sdk-matrix.sh` 通过但仍有 warning：JS Invoker 缺失、C++ 3 个 legacy wire name 文件。

## 维护规则

- 完成项目必须附带受影响路径和可复现验证命令。
- SDK 完成状态按语言分别记录，不用其他 SDK 的通过结果推断。
- 不新增历史兼容层；旧协议、旧命名、旧文档优先删除。
- 重构必须先明确边界和收益：减少依赖面、消除重复、提升测试隔离，至少满足一项。
