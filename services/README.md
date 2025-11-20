# Go-zero 服务规划

本仓库后续的服务端/Agent/Edge 将统一切换到 go-zero。目录约定如下：

```
services/
  api/          # 主后台 API（替换原 Gin server）
  ops/          # 运维接口（可视需要拆分，暂未创建）
  agent/        # Agent 进程 go-zero 化
  edge/         # Edge/Proxy go-zero 化
  analytics/
    ingest/     # 埋点/支付写入
    worker/     # 异步计算任务
    export/     # 导出脚本
```

所有服务共享以下规范：

1. **配置**：使用 go-zero `etc/*.yaml`，字段命名与旧配置保持一致（如 `Host/Port`, `Log`, `Auth`, `TLS` 等），在 README 中记录映射关系。示例：`services/server/etc/server.yaml` 包含：
   - `Registry.AssignmentsPath`：指向 `assignments.json`
   - `Auth.JWTSecret`：JWT HS256 密钥
   - `Descriptors.Dir`：函数描述文件目录（默认 `packs`）
   - `Components.DataDir`：组件数据目录（默认 `data`，内部含 installed/disabled）
   - `Components.StagingDir`：组件安装暂存目录（默认 `data/components/staging`）
   - `Schemas.Dir`：UI/schema 存储目录（默认 `packs/ui`）
2. **API DSL**：HTTP 接口通过 `*.api` 描述；生成 handler/logic 后记得补充测试与 OpenAPI（`goctl api plugin`）。
3. **中间件**：认证、RBAC、审计、追踪、cors/限流等统一放在 `internal/middleware` 方便复用。
4. **共享依赖**：数据库、缓存、对象存储、gRPC SDK 等放置在仓库 `internal/` 或 `pkg/`，通过 `go-zero` 的 `ServiceContext` 注入。

当前状态：

- `api/` 已由 `goctl api new services/api` 初始化，可用于迁移 Registry/OPS/API 逻辑。
- 其余服务尚未创建，待完成 API 迁移后逐步落地。
