# 真实 Dashboard E2E

`real-dashboard` 是唯一用于 Server、Agent、SDK 和 OpenAPI 真实链路验收的 Playwright 项目。它与 `mock-dashboard` 分离，前端以 `MOCK=none` 启动；任何真实链路测试都不得在 mock 项目中运行。

## 环境变量

真实项目需要以下地址。未设置时仅使用本地开发默认值，CI 必须显式设置全部变量：

| 变量                             | 含义                                                              | 本地默认值               |
| -------------------------------- | ----------------------------------------------------------------- | ------------------------ |
| `REAL_DASHBOARD_SERVER_BASE_URL` | fixture 启动的 Croupier Server API 地址，前端开发代理的上游       | `http://localhost:18780` |
| `REAL_DASHBOARD_WEB_BASE_URL`    | 无 Mock 前端地址                                                  | `http://localhost:8001`  |
| `CROUPIER_SERVER_BASE_URL`       | 前端开发代理读取的后端地址；由 Playwright 为 real web server 注入 | 同上                     |

## 生命周期

真实项目的 fixture 负责启动和停止 Server、真实 Agent 与 OpenAPI provider，并只清理 fixture 创建的 game/env scope。测试不得连接生产环境、插入 Contract/Proposal/PageSpec 派生数据，也不得清空共享数据库。

本地列出真实场景：

```bash
pnpm --dir "web" exec playwright test --project=real-dashboard --list
```

启动 fixture 后执行：

```bash
REAL_DASHBOARD_SERVER_BASE_URL="http://127.0.0.1:18780" \
REAL_DASHBOARD_WEB_BASE_URL="http://127.0.0.1:8001" \
pnpm --dir "web" run test:e2e:real
```
