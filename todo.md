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

## 正在推进

- [ ] 清理剩余文档中的旧 `Job*` / `RegisterLocal` / `local_listen` / `rpc_addr` 表述。
- [ ] 清理各 SDK 旧 agent register proto/generated 中的 `rpc_addr`，或删除不再使用的旧 register proto mirror。
- [ ] 清理 Go/C++/Java 配置层仍保留的 `local_listen/localListen/LocalAddr` 回拨字段。

## 未完成

- [ ] SDK 文档中提到历史 `StartJob` / `RegisterLocal` / `local_listen` 的内容需要统一改写为当前设计说明。
- [ ] JS Jest 测试虽然通过，但仍报告未关闭异步句柄警告，需要单独收口测试清理。
- [ ] `TaskEvent` 上行接收后回写 `task_runs` / `task_events` 的闭环仍需完成。
- [ ] `Dispatcher.StreamTask` 仍需改为基于持久化 `task_events` 的正式查询路径。
- [ ] `TaskRunner` / `TaskContext` 仍需抽象，当前 agent task 执行还不是最终结构。
- [ ] shared session runtime 仍未从 Agent-Server 与 SDK-Agent 两条链路中完全抽取复用。

## 最近一次验证结果

- 主仓 targeted Go tests：通过。
- Go SDK：`go test ./...` 通过。
- Java SDK：`./gradlew test` 通过。
- C++ SDK：`cmake --build build -j 4` 通过；此前 `ctest` 仍有 3 个依赖外部 agent/server 行为的测试失败。
- JS/TS SDK：`pnpm exec jest --runInBand` 通过 142 个测试，但 Jest 报告未关闭异步句柄警告。
- Python SDK：`PYTHONPATH=. pytest -q` 通过 247 个测试、跳过 8 个。
- C# SDK：`dotnet test Croupier.Sdk.sln --no-restore` 通过 330 个测试。

## 下一步执行顺序

1. 清理 SDK 文档、示例文档、README 中的旧 `Job*` / `RegisterLocal` / `local_listen` 表述。
2. 清理 SDK 配置层的旧本地监听字段，尤其是 Go/C++/Java 仍保留的 `local_listen/localListen/LocalAddr`。
3. 处理旧 agent register proto mirror 中的 `rpc_addr`。
4. 最后做全仓扫描，按“长任务 job 残留”“旧 provider local 协议残留”“旧回拨本地监听字段”分别验收。
