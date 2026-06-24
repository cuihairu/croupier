# Croupier Task / Provider Session 重构清单

更新时间：2026-06-24

## 当前结论

- 本轮重构不做兼容层，不保留 `job -> task`、`RegisterLocal -> ProviderConnect`、`HeartbeatLocal -> ProviderHeartbeat` 的别名入口。
- `job` 只允许出现在 CI workflow 的通用 `jobs:` 语义、cron/system job 文档语义，不能作为长任务领域名残留。
- `Local*` 只允许保留在 `LocalFunctionDescriptor` 这类“本地函数描述”语义中；`RegisterLocal`、`HeartbeatLocal`、`ListLocal` 不再是协议/API 名。
- NNG/native transport 已经用全仓扫描确认清零，后续不得重新引入。

## 已验收

- [x] 主仓 NNG / nanomsg / pynng / mangos / nng.NET / go.nanomsg 全仓扫描 0 命中。
- [x] 主仓 `agent/v1/job.proto` 删除，新增 `agent/v1/task.proto`。
- [x] 主仓 `pkg/pb/croupier/agent/v1/task.pb.go` 已重新生成，生成代码不再引用 `job_proto`。
- [x] 主仓 `internal/api/job` 删除，新增 `internal/api/task`。
- [x] 主仓 `job_routing_store` 删除，新增 `task_routing_store`。
- [x] 主仓 `pkg/protocol/message.go` 的 `MsgStartJob*` / `MsgJobEvent` / `MsgCancelJob*` 已改为 `Task` 命名。
- [x] 主仓 `pkg/protocol/message.go` 的 `RegisterLocal` / `HeartbeatLocal` 兼容别名已删除。
- [x] Java SDK public invoker API 已改为 `TaskEventInfo`、`startTask`、`streamTask`、`cancelTask`。
- [x] Java SDK `./gradlew test` 已通过。
- [x] C++ SDK public invoker API 已改为 `TaskEvent`、`StartTask`、`StreamTask`、`CancelTask`、`task_id`。
- [x] C++ SDK protobuf 已重新生成，`cmake --build build -j 4` 已通过。
- [x] C++ SDK protocol-focused invoker tests 已通过。
- [x] 主仓删除 `RegisterLocal` / `HeartbeatLocal` / `ListLocal` 的旧 handler 入口。
- [x] 主仓 targeted Go tests 已通过：`go test ./pkg/protocol ./internal/platform/dispatch ./internal/api/task ./internal/app/agent`。
- [x] Go SDK stale `pkg/pb` 生成代码已修复，`file_croupier_agent_v1_job_proto` 扫描 0 命中。
- [x] Go SDK 示例已从 `StartJob` / `StreamJob` / `CancelJob` 改为 `StartTask` / `StreamTask` / `CancelTask`。
- [x] Go SDK provider-session 协议已改为 `ProviderConnect` / `ProviderHeartbeat` / `ProviderDrain`，`RegisterLocal` / `HeartbeatLocal` / `ListLocal` 扫描 0 命中。
- [x] Go SDK `go test ./...` 已通过。
- [x] Java SDK provider proto/protocol/test 已改为 `Provider*`，旧 provider local 命名扫描 0 命中，`./gradlew test` 已通过。
- [x] C++ SDK provider proto/generated/protocol/test 已改为 `Provider*`，旧 provider local 命名扫描 0 命中，`cmake --build build -j 4` 已通过。
- [x] JS/TS SDK runtime、proto、tests 已改为 `Provider*`，删除 `localListen/rpcAddr` 回拨字段，`pnpm exec jest --runInBand` 通过 142 个测试。
- [x] Python SDK runtime/tests 已改为 `Provider*`，删除 `local_listen/rpc_addr` 回拨字段和旧 `agent/local/v1` generated proto，`PYTHONPATH=. pytest -q` 通过 247 个测试、跳过 8 个。
- [x] C# SDK runtime/proto/generated/tests 已改为 `Task*` 与 `Provider*`，`dotnet test Croupier.Sdk.sln --no-restore` 通过 330 个测试。
- [x] SDK 文档中的 `Job*` / `RegisterLocal` / `local_listen` 表述已全部改为 `Task*` / `Provider*` 并删除旧配置字段（API docs、integration guides、proto READMEs、配置文档）。
- [x] 架构文档中的 `Job*` / `StartJobRequest` / `CancelJobRequest` / `JobEvent` 已全部改为 `Task*` 命名（SDK_SPECIFICATION.md、架构设计文档、proto READMEs）。
- [x] Go SDK 配置层已删除 `local_listen` 字段，`StartJob`/`StreamJob`/`CancelJob` 改为 `StartTask`/`StreamTask`/`CancelTask`，`JobEvent` 改为 `TaskEvent`。
- [x] C++ SDK 配置层已删除 `local_listen` 字段，`StartJob`/`StreamJob`/`CancelJob` 改为 `StartTask`/`StreamTask`/`CancelTask`，`JobEvent` 改为 `TaskEvent`。
- [x] Java SDK 配置层已删除 `localListen` 字段（ClientConfig、CroupierProperties）。
- [x] Java SDK 协议层已更新：`MSG_START_JOB_*` → `MSG_START_TASK_*`，`MSG_REGISTER_LOCAL_*` → `MSG_PROVIDER_CONNECT_*`。
- [x] Java SDK `SdkWireMessages` 已重构：删除 `RegisterLocal*` 消息，添加 `Provider*` 消息，`Job*` → `Task*`。
- [x] Java SDK `TaskEventInfo` 已从 `JobEventInfo` 重命名，`taskId` 字段统一。
- [x] Java SDK 测试已全部通过（338 个测试）。
- [x] JS/TS SDK 配置层和协议层已更新：`proto/provider.proto` 改为 `Provider*` 协议，`src/protocol.ts` 和 `src/index.ts` 已删除 `RegisterLocal`/`rpc_addr` 引用，测试已更新。
- [x] C# SDK 配置层和协议层已更新：`proto/provider.proto` 和 `proto/invocation.proto` 已改为 `Provider*`/`Task*`，`ClientConfig` 已删除 `LocalAddr`，`CroupierClient` 已删除本地服务器相关代码，测试已更新。
- [x] Java SDK `proto/croupier/sdk/v1/provider.proto` 已更新为 `Provider*` 协议。
- [x] 全仓 SDK 源代码扫描验证：Go/Java/C++/Python/JS/TS/C# SDK 的 src/ 目录 `RegisterLocal`/`rpc_addr`/`local_listen` 0 命中。

## 正在推进

- [x] 清理 Go SDK 旧 agent register proto/generated：删除 `pkg/pb/croupier/agent/local` 和 `pkg/pb/croupier/control`，更新 `invocation.proto` 使用 `Task*` 命名，更新 `nng_manager.go` 使用 `ProviderConnect` 协议。
- [x] 清理 C++ SDK 旧 proto/generated：更新 `provider.proto` 和 `invocation.proto` 使用 `Provider*` 和 `Task*` 命名，重新生成 protobuf 代码，更新 `croupier_client.cpp` 和 `protocol.h`。

## 未完成

### Transport 实现

主仓库已完成 TCP transport 实现：

- ✅ **主仓库**: `internal/transport/tcp/` 已实现共享 TCP transport
- ✅ **主仓库**: 全仓无 NNG/mangos/nanomsg 引用（2026-04-19 验证）

注：各语言 SDK 为独立仓库，其 transport 迁移状态需在各 SDK 仓库中跟踪。

### 其他任务

- [x] **TaskEvent 上行接收后回写 task_runs / task_events 的闭环**（核心已修复）：`Dispatcher.StartTaskRequest` 现在生成服务器端 task ID（UUID）、在 dispatch 前创建 `task_runs` 行、通过 `InvokeRequest.metadata["task_id"]` 传递给 agent；agent 使用该 ID 而非自生成。agent 回传的事件因此能正确匹配到 `task_runs` 行并更新状态。
- [x] `Dispatcher.StreamTask` 已改为基于持久化 `task_events` / `task_runs` 的正式查询路径：`TaskEventQuery` 返回带 `Seq` 的事件记录和明确的 `TaskRunState`，`StreamTaskRealtime` 会推进 `afterSeq`，`GetRun` 使用 `ErrTaskRunNotFound` 区分 not found 与 DB 错误。
- [x] 单公司多游戏 scope 已继续收敛：扩展安装 API 拒绝 `organization/workspace` 这类 SaaS/组织作用域，只允许 `system/global/game/env/node-group/node`。
- [ ] `TaskRunner` / `TaskContext` 仍需抽象，当前 agent task 执行还不是最终结构。
- [ ] REST API `POST /api/v1/tasks` 的 Start 方法仍不 dispatch（仅创建行），需与函数调用路径统一。
- [ ] shared session runtime 仍未从 Agent-Server 与 SDK-Agent 两条链路中完全抽取复用。
- [ ] JS SDK 未提供 L3 Invoker（`sdks/SDK_FEATURE_MATRIX.md` 中标注的 `❌`），需补齐 `invoke` / `startTask` / `streamTask` / `cancelTask`。
- [ ] **L3 Invoker 命名漂移（CI 已可追踪）**：Go/Python/Java/C# 四个 SDK 的 invoker 仍暴露 `StartJob`/`StreamJob`/`CancelJob`（及 `JobEventInfo` / `JobStatus` 等衍生类型），与矩阵 §四 目标 `*Task*` 不一致；仅 C++ 已对齐。`scripts/check-sdk-matrix.sh::check_invoker_naming` 已强制告警。需按矩阵 §四 迁移规则引入 canonical `*Task*` 方法，并把 `*Job*` 收为 deprecated 别名（每个 SDK 至少保留一个版本），别名需登记到脚本的 allowlist，避免被 `check_wire_name_hygiene` 误判。
- [ ] **Java SDK 协议层 Protocol.java 严重落后**（前置 L3 迁移）：当前 `MSG_*_JOB_*` / `MSG_REGISTER_LOCAL_*` 仍是主常量名，无 canonical 别名；`MSG_LIST_LOCAL_REQUEST` (0x050105/0x050106) 与主仓 `ProviderDrain` 冲突；`SdkWireMessages.java` 缺少 ProviderDrain 与 ProviderHeartbeat 处理。需重写 Protocol.java 引入 `MSG_*_TASK_*` / `MSG_PROVIDER_*` 主常量 + Job/RegisterLocal deprecated alias；将 LIST_LOCAL 删除并改为 ProviderDrain；同步重写 SdkWireMessages 实现 Drain/Heartbeat。受影响 13 个文件中 9 个为 Java（含 InvokerImpl、CroupierClientImpl 与对应测试）。
- [ ] **C++ SDK 协议层 protocol.h 内部不一致**（前置 L3 迁移）：invoker API 已用 Task 命名，但协议常量仍是 `MSG_*_JOB_*`；`msgIdString` 把 `MSG_START_JOB_REQUEST` 硬编码返回 `"StartTaskRequest"`，表里不一。需引入 canonical `MSG_*_TASK_*` 常量，把 Job 名降为 deprecated alias，msgIdString 同步切换。受影响 4 个文件（croupier_client.cpp、test_invoker.cpp、test_protocol.cpp、protocol.h 自身）。
- [ ] **Python SDK generated proto 严重过时**：`sdks/python/generated/croupier/sdk/v1/provider_pb2.py` 仍含 `RegisterLocalRequest` / `rpc_addr` / `HeartbeatRequest` / `ListLocalRequest`，与 root proto（`ProviderConnectRequest` / `ProviderHeartbeatRequest` / `ProviderDrainRequest`）不同步。需重新 `buf generate` 并迁移 `croupier/__init__.py` 的 `get_register_request` 与握手逻辑。
- [ ] **Java SDK wire 层仍用 RegisterLocal 命名**：`SdkWireMessages.java` 大量手写编解码使用老命名，与 `sdk-wire-protocol.md` 不一致，需要单独重构 PR。
- [ ] Go SDK README 的 `Mock gRPC Mode` / `Real gRPC Mode` 命名是历史残留（实际已 TCP），需要清理 Makefile 与构建 tag 命名。

注：各语言 SDK 的文档和生成代码更新需在各 SDK 仓库中跟踪。

## 最近一次验证结果（2026-04-19）

### 主仓库

- 主仓 targeted Go tests：通过。
- 全仓 `RegisterLocal`/`rpc_addr`/`local_listen` 扫描 0 命中（agent `register.proto` 中的 `rpc_addr` 字段标注 DEV ONLY 保留）。
- 全仓 NNG/mangos/nanomsg 扫描 0 命中。
- 主仓文档：核心概念文档已更新为使用 Provider Session 术语。
- TCP transport：`internal/transport/tcp/` 已实现。

### 各语言 SDK（独立仓库）

各语言 SDK 的验证结果需在各 SDK 仓库中跟踪：
- Go SDK（cuihairu/croupier-sdk-go）
- Java SDK（cuihairu/croupier-sdk-java）
- C++ SDK（cuihairu/croupier-sdk-cpp）
- JS/TS SDK（cuihairu/croupier-sdk-js）
- Python SDK（cuihairu/croupier-sdk-python）
- C# SDK（cuihairu/croupier-sdk-csharp）

## 下一步执行顺序

1. ~~TaskEvent 上行接收后回写 task_runs / task_events 的闭环。~~ ✅ 已修复
2. ~~`Dispatcher.StreamTask` 改为基于持久化 `task_events` 的正式查询路径（修复 afterSeq 推进和 GetRun 类型）。~~ ✅ 已修复
3. REST API `POST /api/v1/tasks` 的 Start 方法与函数调用路径统一（dispatch + 持久化）。
4. `TaskRunner` / `TaskContext` 抽象优化。
5. shared session runtime 从 Agent-Server 与 SDK-Agent 两条链路中完全抽取复用。

注：各语言 SDK 的 proto 代码更新和测试需在各 SDK 仓库中跟踪。
