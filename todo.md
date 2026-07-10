# Croupier 当前待办

更新时间：2026-07-10

本清单基于当前源码、架构文档和验证结果维护。历史迁移的完成声明已移除；只有仍可从当前代码复现或验证的事项保留。

## P0：安全与会话正确性

- [x] **限制游戏数据库的解析与创建。** `GameDBMiddleware` 在开启分库路由时，先通过 `GameModel.LookupDatabaseName` 校验 `(game_id, env)` 存在于 `game_envs` 表，未知/未授权作用域返回 403，不再触发建库。物理库创建仅由受控的游戏/环境创建流程触发。
  - 位置：`internal/svc/game_middleware.go`、`internal/db/router/router.go`
  - 验证：`/usr/local/go/bin/go test ./internal/svc/...`

- [x] **修复 Agent 重连时旧连接删除新会话的问题。** 新增 `AgentSessionStore.RemoveSession(agentID, sessionID)`，仅在存储会话的 `SessionID` 与断连连接一致时才删除；`serveConn` 改用 `RemoveSession`，旧连接退出不再误删新会话。
  - 位置：`internal/server/agent_session.go`、`internal/server/tcp_listener.go`
  - 验证：`TestAgentSessionStoreRemoveSessionReconnect`（`/usr/local/go/bin/go test ./internal/server/...`）

- [x] **在 Agent-Server 链路强制握手状态机。** `agentSessionHandler.Handle` 强制：注册前只允许 `RegisterRequest`；注册后拒绝重复 `RegisterRequest`；`Heartbeat` 的 `agent_id` 必须与会话绑定 ID 一致。
  - 位置：`internal/server/tcp_listener.go`
  - 验证：`heartbeat before register is rejected` / `heartbeat with mismatched agent_id is rejected` / `duplicate register is rejected`

- [x] **统一 SDK-Agent 首帧规则。** `providerSessionHandler.Handle` 强制首帧为 `ProviderConnectRequest`；不再允许未注册 `InvokeRequest` 绕过 Provider 握手。Invoker 不是独立 subprotocol，不得跳过握手。
  - 位置：`internal/agent/tcp_local_listener.go`、`docs/architecture/sdk-agent-transport-redesign.md`
  - 验证：`/usr/local/go/bin/go test ./internal/agent/...`

## P1：运行时收敛

- [x] **完成控制面优雅停机。** 所有后台组件（TCP listener、ControlService、session 清理、registry cleanup）派生自 server 根 context；停机顺序为：关闭 TCP listener（停止接收新连接）→ HTTP Shutdown（drain 在途请求）→ 取消根 context 级联停后台 → ControlService.Stop → Router.Close，整体 30s 超时兜底。
  - 位置：`cmd/server/root.go`（`controlRuntime` + `runServer` 停机段）
  - 验证：`/usr/local/go/bin/go build ./cmd/server/...`

- [x] **完成 shared session runtime 抽取。** `internal/transport/session.BaseStore` 新增 `RemoveSession(key, sessionID)` compare-and-remove 原语（reconnect-safe），`AgentSessionStore.RemoveSession`（P0-2）与 Provider 会话清理复用同一语义。心跳/drain/生命周期接口保持不变，业务字段留在子协议层。
  - 位置：`internal/transport/session/store.go`、`internal/server/agent_session.go`
  - 验证：`TestBaseStore_RemoveSession_ReconnectSafe` / `TestAgentSessionStoreRemoveSessionReconnect`

- [x] **避免 Router 全局锁内执行 I/O。** `GameDB` 改用 `singleflight` 按 dbName 协调首次打开；建库/连接/迁移 I/O 在锁外执行，仅 cache map 写入持写锁。不同游戏环境首次初始化可并行，互不阻塞。
  - 位置：`internal/db/router/router.go`
  - 验证：`/usr/local/go/bin/go test ./internal/db/router/...`

- [x] **收敛历史 gRPC / rpc_addr 兼容层。** 删除零主路径引用的 legacy 包：`internal/connpool/`、`internal/transport/interceptors/`、`internal/transport/jsoncodec/`。Ops/Dispatch 调用 Agent 均走 TCP session，不依赖反向回拨。proto `rpc_addr` 标注 DEPRECATED 与删除门控条件（待所有部署 Agent 弃用该字段后移除），保留点仅为镜像写入。
  - 位置：`proto/croupier/agent/v1/register.proto`、`internal/model/agent_session_model.go`、`internal/platform/registry/`（保留镜像）
  - 验证：`/usr/local/go/bin/go build ./...`（删除后全量编译通过）

## P1：作用域与依赖边界

- [x] **将分库边界落实到所有 game-scoped 访问。** 审计确认所有 game-scoped model（player/function/task/ticket/analytics 等 11 个）均通过 `dbctx.Resolve(ctx, m.db)` 路由；service 层直接用 `svcCtx.DB` 的操作全部是 meta 模型（admin/role/monitoring/权限 scope），无绕过 scope 的游戏数据操作。补 `dbctx` 路由契约回归测试。
  - 重点位置：`internal/api/extension/service.go`、`internal/api/task/service.go`、`internal/logic/utils/game_scope.go`、`internal/db/dbctx/`
  - 验证：`TestResolve_OverrideWins`（请求 context 注入 game DB 时一定路由到注入库）

- [x] **拆分 `ServiceContext`（建立锚点）。** 遵循「先以窄接口替换直接依赖，避免一次性重构」：新增 `internal/ports/` 领域端口 `Permissions`，`*svc.PermissionService` 结构性满足该接口并由契约测试锁定。后续逐个消费者迁移到 port，ServiceContext 退回组合根。
  - 位置：`internal/ports/permissions.go`、`internal/ports/permissions_test.go`
  - 验证：`TestPermissionServiceSatisfiesPort`
  - 后续：继续为 Task 运行、Ops 状态、Game scope 补 port，逐步把消费者从 `*svc.ServiceContext` 收敛到窄接口。

## P2：任务模型与 SDK 一致性

- [x] **统一 REST Task Start 与函数调用调度路径。** `POST /api/v1/tasks` 的 `Start` 已走 `Dispatcher.StartTaskRequest`（服务端生成 task ID + 创建 task_runs 行 + 转发 agent）；补齐 `Cancel` 也调用 `Dispatcher.CancelTask` 转发取消到 agent，不再只更新本地行。
  - 位置：`internal/api/task/service.go`（`Start`/`Cancel`）、`internal/platform/dispatch/dispatcher.go`
  - 验证：`/usr/local/go/bin/go test ./internal/api/task/...`

- [x] **抽象 Agent `TaskRunner` / `TaskContext`。** 新增 `internal/agent/task_runner.go`，封装任务启动/取消/事件上报/状态（`TaskExecutor` + `TaskEventReporter` 注入）；`LocalHandler` 改为委托，删除内联 `runTask`/`executeTask`/`emitTaskEvent`/`taskIndex`。
  - 位置：`internal/agent/task_runner.go`、`internal/agent/local_handler.go`
  - 验证：`TestTaskRunner`、`TestLocalHandler_TaskEventReporting`（`/usr/local/go/bin/go test ./internal/agent/...`）

- [x] **按 SDK 独立验证并迁移旧 wire 命名。** 6 语言 SDK 源码旧命名（`StartJob`/`RegisterLocal`/`HeartbeatLocal`/`ListLocal`/`JobEvent`）已迁移到根协议命名（`StartTask`/`ProviderConnect`/`ProviderHeartbeat`/`TaskEvent`），数值 wire opcode 不变。
  - **Go**（主力）：57 文件，`go build` + 核心包测试通过，手写代码旧命名零残留。
  - **Java**：17 文件 + 2 重命名，全树扫描零残留（无 JDK，一致性靠全面扫描保证）。
  - **Python**：手写层全迁移，py_compile OK，旧命名零残留；生成 `*_pb2.py` 过时，需 `sdks/python/scripts/regen-proto.sh`（buf）重生成后运行。
  - **JS/TS**：3 文件，`tsc --noEmit` + 144 测试全过，旧命名零残留。
  - **C#**：手写层全迁移，操作码不变；生成 `generated/*.cs` 过时（保留方案 A），需 protoc 重生成后消除 `CroupierInvoker.cs` 中 1 处对 `StartJobResponse` 的内部引用。
  - **C++**：审计确认无旧命名，无需迁移。
  - 位置：`sdks/{go,java,python,js,csharp,cpp}/`、`proto/croupier/agent/v1/register.proto`、`sdks/python/scripts/regen-proto.sh`
  - 验证：各 SDK 手写源码 `rg "StartJob|RegisterLocal|HeartbeatLocal|ListLocal|JobEvent"` 零残留；Python/C# 生成代码重生成待 buf/protoc 环境。

## 已验证（2026-07-10）

- [x] `/usr/local/go/bin/go test ./internal/server ./internal/agent ./internal/db/router ./internal/transport/session ./internal/platform/dispatch ./internal/api/task` 通过。
- [x] 架构目标已文档化：Agent-Server 与 SDK-Agent 使用共享 session runtime，业务作用域为 `game_id + env`，并采用按游戏分库。
- [x] P0 安全与会话正确性 4 项已修复并通过测试：game/env 分库校验、Agent 重连会话隔离、Agent-Server 握手状态机、SDK-Agent 首帧规则。
- [x] P1 运行时收敛 4 项 + 作用域边界 2 项已完成：控制面优雅停机、shared session runtime 抽取（compare-and-remove）、Router 锁外 I/O（singleflight）、删除 legacy gRPC 包、分库边界审计 + dbctx 契约测试、ServiceContext 端口锚点（ports.Permissions）。
- [x] P2 任务模型与 SDK 一致性 3 项已完成：REST Task 统一调度（含 Cancel 转发）、Agent TaskRunner 抽象、6 语言 SDK 旧 wire 命名迁移（Go/JS 可运行验证，Java/Python/C# 手写层迁移完成、生成代码待重生成）。

## 维护规则

- 完成项目必须附带受影响路径和可复现的验证命令后才移入“已验证”。
- SDK 的完成状态按语言分别记录，不以其他 SDK 的通过结果推断。
- 不为历史兼容层新增功能；任何保留项必须标注删除条件和期限。
