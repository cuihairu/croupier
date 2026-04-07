# 函数注册链路完成度与验收报告

## 范围

本报告覆盖以下链路的静态分析与运行态验证：

- `SDK -> Agent -> Server -> Functions API -> UI 配置 -> History -> Rollback`
- 本地 E2E 脚本可执行性
- 健康检查与 Agent 状态查询入口
- 前后端接口契约一致性

报告时间：2026-04-08  
验证环境：本地 SQLite 配置 `configs/test-sqlite.yaml`

## 结论

当前主链路完成度可评估为 `95%+`，已经达到“可运行、可验收、主流程闭环”的状态。

已确认：

- Server 可在 SQLite 配置下启动。
- Agent 可连接并向 Server 同步函数。
- Go SDK 示例可向 Agent 注册函数。
- Server 可返回函数列表、descriptor、OpenAPI、UI 配置、UI 历史、UI 回滚结果。
- 现有 E2E 脚本已修正为匹配当前鉴权与路由模型，并可实际跑通。

## 运行态验证

### 启动验证

实际使用以下组件完成联调：

- Server：`./bin/croupier-server --config configs/test-sqlite.yaml`
- Agent：`./bin/croupier-agent --config configs/agent.local.yaml`
- SDK 示例：`go run ./examples/basic`

### 实际通过的验证项

- `go test ./...`
- `make build`
- `go test ./internal/api/function ./internal/logic/function ./internal/api/monitoring ./internal/handler`
- `go test ./internal/logic/utils ./internal/logic/function`
- `bash scripts/e2e/health-check.sh`
- `bash scripts/e2e/test-performance.sh`
- `bash scripts/e2e/test-ui-configuration.sh`

### 实测通过的关键接口

- `GET /healthz`
- `GET /api/v1/ops/agents`
- `GET /api/v1/functions`
- `GET /api/v1/functions/descriptors`
- `GET /api/v1/functions/{id}/openapi`
- `GET /api/v1/functions/{id}/ui`
- `PUT /api/v1/functions/{id}/ui`
- `GET /api/v1/functions/{id}/ui/history`
- `POST /api/v1/functions/{id}/ui/rollback`
- `POST /api/v1/functions/{id}/invoke`

## 本次修复

### 后端

- 接通 UI history 包装层，不再返回空数据。
- 接通 UI rollback 包装层，不再返回空成功体。
- 让 `PUT /api/v1/functions/{id}/ui` 返回完整 UI 文档，而不是 `{}`。
- 注册 monitoring 路由。
- 将 `/healthz` 放入鉴权白名单。
- 清理公开函数列表的误导性日志噪声。

涉及文件：

- `internal/api/function/helpers.go`
- `internal/api/function/service.go`
- `internal/api/function/handler.go`
- `internal/api/function/service_test.go`
- `internal/handler/routes.go`
- `internal/svc/service_context.go`
- `internal/logic/function/function_u_i_history_logic.go`
- `internal/logic/function/function_u_i_rollback_logic.go`
- `internal/logic/function/functions_list_logic.go`
- `internal/logic/utils/profile_helpers.go`

### 脚本

- E2E 脚本已切换为“先登录，再调用真实接口”。
- Agent 状态查询入口已从旧路径对齐到 `/api/v1/ops/agents`。
- UI 配置脚本已按当前返回结构校验 `items`、`appliedVersion`、对象型 `layout`。

涉及文件：

- `scripts/e2e/health-check.sh`
- `scripts/e2e/test-performance.sh`
- `scripts/e2e/test-ui-configuration.sh`

### Dashboard

- 前端 `saveFunctionUiSchema` 已对齐为接收后端返回的 UI 文档，而不是 `void`。

涉及文件：

- `croupier-dashboard/src/services/api/functions.ts`

## 当前剩余风险

这些问题不再阻塞主流程验收，但仍建议后续处理：

- 仓库内存在与本次任务无关的脏数据文件，如 `internal/api/audit/ops_state.json`、`internal/api/profile/ops_state.json`、`test-data/croupier.db`。
- 工作区存在未跟踪目录，如 `packs/`、`services/`、多个 `sdks/` 目录；提交前需要人工确认边界。
- `croupier-dashboard` 仓库内还有未跟踪文件 `debug-functions.html`，不应混入本次提交。
- 本地运行使用的是 SQLite 测试配置；若要作为生产验收，还需要在 MySQL/Redis 目标环境复验一次。

## 建议的提交边界

建议将本次交付拆分为两个提交：

1. 后端与 E2E 修复
   - API 包装层
   - monitoring 路由与 healthz
   - 脚本更新
   - 相关测试

2. Dashboard 契约对齐
   - `src/services/api/functions.ts`

## 验收判断

如果验收标准是“函数注册到展示链路是否闭环并可重复执行”，当前答案是：`是`。  
如果验收标准是“仓库是否已经完全清洁并可直接提交”，当前答案是：`还需要整理工作区边界`。
