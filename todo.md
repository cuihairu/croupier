# Croupier Task / Provider Session 重构清单

更新时间：2026-04-19

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

### Transport 迁移（高优先级）

经过 2026-04-19 检查，发现除 Python SDK 外，所有 SDK 仍在使用 NNG transport，未完成 TCP session 迁移：

- [ ] **Go SDK**: `pkg/croupier/transport/` 仍在使用 `go.nanomsg.org/mangos/v3`
- [ ] **JS SDK**: `src/transport.ts` 仍在使用 `@rustup/nng`（Jest 警告的根本原因）
- [ ] **Java SDK**: `transport/NNGTransport.java` 仍在使用 NNG
- [ ] **C# SDK**: `Transport/NNGTransport.cs` 仍在使用 NNG
- [ ] **C++ SDK**: `src/nng_transport.cpp` 仍在使用 NNG

参考实现：
- ✅ **Python SDK**: `croupier/transport/tcp.py` 已完成 TCP session 迁移
- ✅ **主仓库**: `internal/transport/tcp/` 已有共享 TCP transport 实现

### 其他任务

- [ ] SDK 文档中提到历史 `StartJob` / `RegisterLocal` / `local_listen` 的内容需要统一改写为当前设计说明（主要文档已更新为说明废弃概念）。
- [ ] C# SDK 生成代码 (Gen 目录) 需要重新生成以反映最新的 proto 更改。
- [ ] `TaskEvent` 上行接收后回写 `task_runs` / `task_events` 的闭环仍需完成。
- [ ] `Dispatcher.StreamTask` 仍需改为基于持久化 `task_events` 的正式查询路径。
- [ ] `TaskRunner` / `TaskContext` 仍需抽象，当前 agent task 执行还不是最终结构。
- [ ] shared session runtime 仍未从 Agent-Server 与 SDK-Agent 两条链路中完全抽取复用。

## 最近一次验证结果（2026-04-19）

- 主仓 targeted Go tests：通过。
- Go SDK：`go test ./...` 通过；`src/` 目录 `RegisterLocal`/`rpc_addr`/`local_listen` 扫描 0 命中。
- Java SDK：`./gradlew test` 通过；`src/` 目录 `RegisterLocal`/`rpc_addr`/`local_listen` 扫描 0 命中；`proto/croupier/sdk/v1/provider.proto` 已更新。
- C++ SDK：`cmake --build build -j 4` 通过；`src/` 目录 `RegisterLocal`/`rpc_addr`/`local_listen` 扫描 0 命中。
- JS/TS SDK：`src/` 目录 `RegisterLocal`/`rpc_addr` 扫描 0 命中，proto 已更新。
- Python SDK：`PYTHONPATH=. pytest -q` 通过 247 个测试、跳过 8 个；`src/` 目录 `RegisterLocal`/`rpc_addr`/`local_listen` 扫描 0 命中。
- C# SDK：`src/Croupier.Sdk/` 目录 `RegisterLocal`/`LocalAddr` 扫描 0 命中，proto 已更新。
- 主仓文档：核心概念文档已更新为使用 Provider Session 术语。

**说明**：agent `register.proto` 中的 `rpc_addr` 字段（标注 DEV ONLY）保留，因为这是 agent 注册协议的一部分，不属于 provider session 协议范畴。

## 下一步执行顺序

1. Java SDK 重新生成 proto 代码（`proto/croupier/sdk/v1/provider.proto` 已更新）。
2. C# SDK 重新生成 proto 代码（`proto/croupier/sdk/v1/provider.proto` 已更新）。
3. JS Jest 测试异步句柄警告处理。
4. `TaskEvent` 上行接收后回写 `task_runs` / `task_events` 的闭环。
