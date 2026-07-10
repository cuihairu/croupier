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

- [ ] **统一 REST Task Start 与函数调用调度路径。** 确认 `POST /api/v1/tasks` 是否仍只创建记录；如是，复用 Dispatcher 的任务 ID、持久化与事件闭环，避免两套任务语义。
  - 位置：`internal/api/task/`、`internal/platform/dispatch/`

- [ ] **抽象 Agent `TaskRunner` / `TaskContext`。** 将任务执行、取消、状态和事件上报从 Agent handler 中分离，保持 Task 的单一职责与可测试性。

- [ ] **按 SDK 独立验证并迁移旧 wire 命名。** 当前 Java SDK 源码仍含 `StartJob` / `RegisterLocal`；Python 仍有旧 `agent/local` 生成代码。以根 `proto/` 和 `sdk-wire-protocol.md` 为单一协议源，生成或更新各 SDK，逐语言运行测试。
  - 位置：`sdks/java/`、`sdks/python/`
  - 验收：SDK 源码和生成代码不存在非兼容目的的 `RegisterLocal` / `HeartbeatLocal` / `ListLocal`；Task/Provider 命名与根协议一致。

## 已验证（2026-07-10）

- [x] `/usr/local/go/bin/go test ./internal/server ./internal/agent ./internal/db/router ./internal/transport/session ./internal/platform/dispatch ./internal/api/task` 通过。
- [x] 架构目标已文档化：Agent-Server 与 SDK-Agent 使用共享 session runtime，业务作用域为 `game_id + env`，并采用按游戏分库。
- [x] P0 安全与会话正确性 4 项已修复并通过测试：game/env 分库校验、Agent 重连会话隔离、Agent-Server 握手状态机、SDK-Agent 首帧规则。
- [x] P1 运行时收敛 4 项 + 作用域边界 2 项已完成：控制面优雅停机、shared session runtime 抽取（compare-and-remove）、Router 锁外 I/O（singleflight）、删除 legacy gRPC 包、分库边界审计 + dbctx 契约测试、ServiceContext 端口锚点（ports.Permissions）。

## 维护规则

- 完成项目必须附带受影响路径和可复现的验证命令后才移入“已验证”。
- SDK 的完成状态按语言分别记录，不以其他 SDK 的通过结果推断。
- 不为历史兼容层新增功能；任何保留项必须标注删除条件和期限。
