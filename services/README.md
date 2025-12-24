# Services 目录说明（当前实现）

本仓库的 go-zero 服务已落地在 `services/*` 下（并非仅规划）。当前目录结构如下：

```
services/
  server/       # 控制面 Server（go-zero REST + gRPC 控制面）
  agent/        # Agent 进程（go-zero REST + 内嵌 gRPC core）
  edge/         # Edge/Proxy（go-zero REST + gRPC tunnel）
  ingest/       # Analytics ingestion（HTTP/JSON -> MQ）
  demo/         # OpenTelemetry demo（用于 telemetry compose）
```

说明：

1. **配置**：
   - 服务自身配置：`services/<name>/etc/*.yaml`（用于本地运行/开发）
   - Docker/Compose 配置：`configs/{server,agent,edge}.yaml`（用于 `docker/docker-compose.yml` 默认启动）
2. **API DSL**：go-zero HTTP 接口通过 `*.api` 描述（例如 `services/server/server.api`，以及 `services/server/modules/*.api` 的模块拆分）。
3. **中间件/共享依赖**：共享逻辑在仓库根的 `internal/` 与 `pkg/` 下，通过各服务的 `internal/svc` 注入。

如果你正在寻找“统一 CLI（croupier …）”或“services/api”目录：目前仓库并未落地一个独立的 `services/api` 服务，功能集中在 `services/server` 内。
