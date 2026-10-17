# Agent 公共能力

本目录提供可组合的 transport、register、backoff、logx、configsync、supervisable、healthprobe、report、metrics、execlog、crash 包，按角色需要接线；不承载 GM 函数、堡垒机会话或 CDC 业务。

公共包及 `agents/` 的生产代码不得 import `internal/`，协议契约使用 `pkg/pb` 与 `pkg/protocol`。双向 `MuxConn` 和完整 supervisor 目前仍在 internal，属于待抽取能力，不能假定已经公共化。

包职责、实际消费者、目标目录与迁移批次见 [运行时目录边界](../docs/development/agent-runtime-layout.md)。相关单测执行 `go test ./core/... ./agents/...`。
