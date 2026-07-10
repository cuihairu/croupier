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

- [ ] **完成控制面优雅停机。** 将 TCP listener、session 清理、ControlService 后台任务纳入 server 根 `context`；停机顺序为停止接收、drain 在途请求、超时关闭会话与后台任务。
  - 位置：`cmd/server/root.go`

- [ ] **完成 shared session runtime 抽取。** `AgentSession` 与 `ProviderSession`、两个 Store 和 listener 生命周期逻辑仍重复；`internal/transport/session` 目前只提供未接入的基础抽象。抽取可复用的身份键控、compare-and-remove、心跳、drain 和生命周期接口，业务字段留在子协议层。
  - 位置：`internal/server/agent_session.go`、`internal/agent/provider_session.go`、`internal/transport/session/`

- [ ] **避免 Router 全局锁内执行 I/O。** `GameDB` 在写锁范围内执行建库、连接与迁移，会阻塞所有其他游戏环境首次初始化。改用按 scope 的初始化协调（如 singleflight）并保持 cache 的并发安全。
  - 位置：`internal/db/router/router.go`

- [ ] **收敛历史 gRPC / rpc_addr 兼容层。** 明确保留期限和删除计划；主路径不得依赖反向回拨。优先审计 `connpool`、interceptor、json codec、TLS helper、`rpc_addr` proto 字段和 Registry/UI 映射。
  - 位置：`internal/connpool/`、`internal/transport/interceptors/`、`internal/transport/jsoncodec/`、`internal/platform/tlsutil/`、`internal/logic/utils/registry_helpers.go`

## P1：作用域与依赖边界

- [ ] **将分库边界落实到所有 game-scoped 访问。** 统一使用 request/background context 中的 DB resolver；审计直接使用 `svcCtx.DB` 的服务，区分元数据操作与游戏数据操作，禁止后者绕过 scope。
  - 重点位置：`internal/api/extension/service.go`、`internal/api/task/service.go`、`internal/logic/utils/game_scope.go`

- [ ] **拆分 `ServiceContext`。** 当前对象同时承担组合根、基础设施容器和领域服务定位器职责，并直接暴露大量 Model。先以 API/logic 所需的窄接口或领域 facade 替换直接依赖，避免一次性重构。
  - 位置：`internal/svc/service_context.go`

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

## 维护规则

- 完成项目必须附带受影响路径和可复现的验证命令后才移入“已验证”。
- SDK 的完成状态按语言分别记录，不以其他 SDK 的通过结果推断。
- 不为历史兼容层新增功能；任何保留项必须标注删除条件和期限。
