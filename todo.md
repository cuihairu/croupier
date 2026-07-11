# Croupier 当前待办

更新时间：2026-07-10

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

- [ ] **补齐 JS/TS L3 Invoker。**
  - 现状：`sdks/SDK_FEATURE_MATRIX.md` 明确记录 JS Invoker 暂未提供；`scripts/check-sdk-matrix.sh` 只把该问题记为 warning，不阻断 CI。
  - 目标：JS/TS 提供独立 Invoker，支持 `invoke` / `startTask` / `streamTask` / `cancelTask`，并与其他 SDK 一致。
  - 位置：`sdks/js/src/`、`sdks/js/examples/`、`sdks/SDK_FEATURE_MATRIX.md`、`scripts/check-sdk-matrix.sh`
  - 验证：JS 单测 + `scripts/check-sdk-matrix.sh` 无 JS Invoker warning。

- [ ] **删除 C++ SDK legacy wire name 残留。**
  - 现状：`scripts/check-sdk-matrix.sh` 仍检出 3 个 C++ 文件引用 `MSG_START_JOB_REQUEST` / `MSG_STREAM_JOB_REQUEST` / `MSG_CANCEL_JOB_REQUEST` / `MSG_JOB_EVENT`。
  - 目标：统一为 `Task` 命名；不保留 `Job` 兼容别名。
  - 位置：`sdks/cpp/src/croupier_client.cpp`、`sdks/cpp/tests/test_invoker.cpp`、`sdks/cpp/tests/test_protocol.cpp`
  - 验证：`scripts/check-sdk-matrix.sh` 无 wire warning；C++ 相关测试通过。

- [ ] **把 SDK matrix warning 升级为失败条件。**
  - 现状：`scripts/check-sdk-matrix.sh` 在有 warning 时仍 `exit 0`，CI 无法阻止旧命名继续滞留。
  - 目标：除显式 allowlist 外，SDK 缺能力、旧 wire name、旧 README 术语全部失败。
  - 位置：`scripts/check-sdk-matrix.sh`、`.github/workflows/ci.yml`
  - 验证：CI 的 “SDK matrix conformance” 能阻断新增旧术语/旧命名。

- [ ] **清理 `Job` 命名，统一为 `Task`。**
  - 目标：源码、proto、生成代码、文档、示例统一使用 `Task`。
  - 范围：`StartJob`、`StreamJob`、`CancelJob`、`JobEvent`、`job_id`、`GetJobResult` 等历史命名。
  - 说明：不考虑兼容旧 API；直接重命名和删除旧入口。
  - 验证：`rg "StartJob|StreamJob|CancelJob|JobEvent|GetJobResult|job_id"` 只允许出现在迁移说明或历史归档中。

- [~] **清理 `rpc_addr` / LocalControl / gRPC callback 兼容字段（主链路已完成，schema 级残留待专项）。**
  - ✅ 已完成：主链路路由零依赖 rpc_addr（`internal/platform/dispatch/` 无引用）；运行时无 gRPC 反向回拨代码（无 `grpc.Dial`）；LocalControl 概念清理（Java Protocol.java 注释改 ProviderService）。
  - 🟡 待专项（schema 级，需协调 migration，不强行删以免破坏 ops）：`rpc_addr` 仍是 ops 展示用的 agent 地址镜像（`Addr`/`RPCAddr` 重复字段，前端 Nodes/Jobs/RateLimits 页面消费 `rpcAddr`）；DB 列 `agent_sessions.rpc_addr`（`internal/model/agent_session_model.go`）；proto 字段 `RegisterRequest.rpc_addr`（删需 SDK 重新生成）。
  - 删除门控：需同步 (1) ops 改用 TCP session `RemoteAddr` 取代 legacy rpc_addr 镜像；(2) Server 删 `RPCAddr` 响应字段 + 前端改读 `addr`；(3) DB migration 删列；(4) proto 删字段 + SDK 重生成。
  - 验证（主链路）：`rg "rpc_addr|RPCAddr" internal/platform/dispatch/ internal/logic/ops/agent_ops_client.go` 无路由用途引用。

## P0：恢复发布级 E2E

- [x] **恢复 CI 最小 E2E job。** 根因是旧 job 用 `configs/server.yaml`（mysql/postgres）在无 DB 的 CI runner 启动失败；改为 `configs/test-sqlite.yaml`（无外部 DB，admin 从 configs/*.json seed），删除 `if: ${{ false }}`，新增确定性 readiness probe（60s 轮询 /healthz），跑 `scripts/e2e/happy-path.sh`。
  - 位置：`.github/workflows/ci.yml`（e2e job）、`scripts/e2e/happy-path.sh`、`configs/test-sqlite.yaml`
  - 验证：本地 `happy-path.sh` 5/5 通过（healthz + auth + /games + /ops/agents + /tasks）；CI 在 PR 上自动执行、失败阻断合并。

- [~] **新增 Server-Agent-SDK happy path E2E。**
  - ✅ Server 启动 + 健康检查 + auth + REST surface 已在 `happy-path.sh` 覆盖。
  - 🟡 Agent TCP 注册握手（首帧 Register、session 路由、dispatcher）脚本已预留 `./bin/e2e-agent-probe` 接入点；握手状态机本身已由 `internal/server` 单元测试覆盖（`TestAgentSessionHandler`：未注册 Heartbeat 拒绝、重复 Register 拒绝、跨 agent_id 拒绝）。Agent probe 二进制待后续补一个 examples/cmd 小程序发送 RegisterRequest 帧。

- [ ] **新增 Task lifecycle E2E。**
  - 覆盖路径：`startTask` → `streamTask` → `cancelTask` → 状态落库/事件返回。
  - 目标：防止 REST task、Dispatcher、Agent TaskRunner、SDK Invoker 之间出现协议漂移。
  - 待补：依赖 Agent probe（无真实 agent 运行时，startTask 无法端到端完成）。
  - 目标：防止 REST task、Dispatcher、Agent TaskRunner、SDK Invoker 之间出现协议漂移。

## P1：API 测试覆盖补齐

- [ ] **为无同包测试的 API 包补 smoke/contract tests。**
  - 当前无测试包：
    - `internal/api/entity`
    - `internal/api/faq`
    - `internal/api/feedback`
    - `internal/api/functioncall`
    - `internal/api/message`
    - `internal/api/meta`
    - `internal/api/node`
    - `internal/api/rate_limit`
    - `internal/api/routes`
    - `internal/api/schema`
    - `internal/api/storage`
    - `internal/api/terms`
    - `internal/api/ticket`
    - `internal/api/user`
    - `internal/api/workspace`
  - 优先级：先补 `workspace`、`schema`、`storage`、`functioncall`、`rate_limit`。
  - 验证：`/usr/local/go/bin/go test ./internal/api/...`

- [ ] **建立 API contract guard。**
  - 目标：核心 API 的错误格式、分页字段、鉴权失败、game/env scope 行为稳定。
  - 范围：workspace、schema、storage、functioncall、rate_limit、task。
  - 验证：新增 contract tests；避免仅靠 handler 单测。

## P1：文档重新收敛

- [ ] **统一 SDK 文档的单一事实源。**
  - 目标：`sdks/SDK_FEATURE_MATRIX.md` 和 `docs/sdks/sdk-parity-matrix.md` 只保留当前基线，不再混入历史模型。
  - 处理：README 只写当前接入方式；历史说明移动到归档或直接删除。
  - 验证：文档中不再把 `gRPC`、`LocalControl`、`rpc_addr` 作为默认链路。

- [ ] **清理 SDK README 的旧构建/旧链路描述。**
  - 重点：`sdks/go/README.md`、`sdks/cpp/README.md`、`sdks/cpp/COMPLETE_SDK_README.md`、`sdks/java/README.md`、`sdks/js/examples/README.md`
  - 目标：所有语言文档按 L1 Provider、L3 Invoker、语言扩展分层描述。
  - 验证：`scripts/check-sdk-matrix.sh` 的 README hygiene 覆盖这些关键文件。

- [ ] **重新规划 docs 信息架构。**
  - 目标：按“用户路径”而不是“历史开发过程”组织文档。
  - 建议结构：
    - 快速开始：Server + Agent + 一个 SDK 的最小闭环。
    - SDK：Provider / Invoker / 配置 / 错误处理 / 示例。
    - API：REST contract、鉴权、game/env scope。
    - 运维：部署、E2E、监控、备份。
    - 开发：架构、代码规范、扩展策略、发布规则。
  - 验证：`docs` 构建通过；首页不再链接过时迁移文档作为主路径。

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

- [ ] **收敛 API handler/service 模式。**
  - 问题：API 包数量多，handler/service/model 组合方式不完全统一，新增测试成本偏高。
  - 建议：抽取通用 response、binding、pagination、scope 校验模式；避免每个包重复样板。
  - 约束：只抽真实重复，不为了“框架化”引入额外复杂度。

- [ ] **收敛 Analytics 链路的生产 readiness。**
  - 现状：已有 `cmd/ingest`、`cmd/analytics-worker`、ClickHouse/Redis/Flink 文档和 compose 配置，但需要专项确认端到端与部署成熟度。
  - 建议：单独审计 ingest → MQ → worker → ClickHouse → API 查询链路。
  - 验证：最小 analytics E2E，覆盖事件写入、消费、落库、查询。

## P2：工程治理

- [ ] **把“无兼容遗留”写入仓库治理规则。**
  - 目标：新协议、新 SDK、新文档不再默认保留旧入口。
  - 规则：如果确实需要暂留兼容字段，必须同时提交删除计划和检查脚本。
  - 位置：`docs/development/repository-guidelines.md`、`docs/development/documentation-governance.md`

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

## 最近验证

- [x] 2026-07-10：`/usr/local/go/bin/go test ./...` 通过。
- [x] 2026-07-10：`scripts/check-sdk-matrix.sh` 通过但仍有 warning：JS Invoker 缺失、C++ 3 个 legacy wire name 文件。

## 维护规则

- 完成项目必须附带受影响路径和可复现验证命令。
- SDK 完成状态按语言分别记录，不用其他 SDK 的通过结果推断。
- 不新增历史兼容层；旧协议、旧命名、旧文档优先删除。
- 重构必须先明确边界和收益：减少依赖面、消除重复、提升测试隔离，至少满足一项。
