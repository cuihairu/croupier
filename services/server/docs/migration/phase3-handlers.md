# Phase 3 — Handler 批量迁移

状态: ⬜ 待开始

## 目标

将 268 个 handler 文件从 go-zero `httpx` 模式改为 Gin `*gin.Context` 模式。

## Handler 迁移模式

### Before (go-zero)
```go
func XxxHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req types.XxxRequest
        if err := httpx.Parse(r, &req); err != nil {
            httpx.ErrorCtx(r.Context(), w, err)
            return
        }
        l := logic.NewXxxLogic(r.Context(), svcCtx)
        resp, err := l.Xxx(&req)
        if err != nil {
            httpx.ErrorCtx(r.Context(), w, err)
        } else {
            httpx.OkJsonCtx(r.Context(), w, resp)
        }
    }
}
```

### After (Gin)
```go
func XxxHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req types.XxxRequest
        if err := c.ShouldBind(&req); err != nil {
            response.Error(c, errorx.NewBadRequest(err.Error()))
            return
        }
        l := logic.NewXxxLogic(c.Request.Context(), svcCtx)
        resp, err := l.Xxx(&req)
        if err != nil {
            response.Error(c, err)
        } else {
            response.Success(c, resp)
        }
    }
}
```

### `httpx.Parse` 对应关系

| 请求来源 | go-zero | Gin |
|---------|---------|-----|
| Query 参数 | `httpx.Parse` (自动) | `c.ShouldBindQuery(&req)` |
| JSON Body | `httpx.Parse` (自动) | `c.ShouldBindJSON(&req)` |
| Path 参数 | `httpx.Parse` (自动) | `c.ShouldBindUri(&req)` |
| 混合 (Path+Query) | `httpx.Parse` (自动) | 先 `ShouldBindUri` 再 `ShouldBindQuery` |
| 混合 (Path+Body) | `httpx.Parse` (自动) | 先 `ShouldBindUri` 再 `ShouldBindJSON` |

> 注意：`httpx.Parse` 会自动合并 path/query/body，Gin 需要分步绑定。
> 判断依据：看 `types.go` 中对应 struct 的 tag（有 `uri:` 则需要 `ShouldBindUri`）。

---

## 迁移批次

### Batch 1 — 简单模块（无复杂逻辑，优先迁移验证模式）

| 模块 | 目录 |
|------|------|
| meta | `handler/meta/` |
| monitoring | `handler/monitoring/` |
| openapi | `handler/openapi/` |
| message | `handler/message/` |
| alert | `handler/alert/` |

- [ ] meta
- [ ] monitoring
- [ ] openapi
- [ ] message
- [ ] alert

### Batch 2 — 认证与权限

| 模块 | 目录 |
|------|------|
| auth | `handler/auth/` |
| permission | `handler/permission/` |
| role | `handler/role/` |

- [ ] auth
- [ ] permission
- [ ] role

### Batch 3 — 管理员与用户

| 模块 | 目录 |
|------|------|
| admin | `handler/admin/` |
| adminGames | `handler/adminGames/` |
| player | `handler/player/` |
| profile | `handler/profile/` |

- [ ] admin
- [ ] adminGames
- [ ] player
- [ ] profile

### Batch 4 — 游戏与资源

| 模块 | 目录 |
|------|------|
| game | `handler/game/` |
| entity | `handler/entity/` |
| component | `handler/component/` |
| schema | `handler/schema/` |
| pack | `handler/pack/` |
| provider | `handler/provider/` |

- [ ] game
- [ ] entity
- [ ] component
- [ ] schema
- [ ] pack
- [ ] provider

### Batch 5 — 工作流与审计

| 模块 | 目录 |
|------|------|
| approval | `handler/approval/` |
| assignment | `handler/assignment/` |
| audit | `handler/audit/` |
| ticket | `handler/ticket/` |
| workspace | `handler/workspace/` |
| feedback | `handler/feedback/` |
| faq | `handler/faq/` |
| term | `handler/term/` |
| terms | `handler/terms/` |

- [ ] approval
- [ ] assignment
- [ ] audit
- [ ] ticket
- [ ] workspace
- [ ] feedback
- [ ] faq
- [ ] term
- [ ] terms

### Batch 6 — 基础设施与高级功能

| 模块 | 目录 |
|------|------|
| agent | `handler/agent/` |
| node | `handler/node/` |
| registry | `handler/registry/` |
| config | `handler/config/` |
| storage | `handler/storage/` |
| backup | `handler/backup/` |
| certificate | `handler/certificate/` |
| migrate | `handler/migrate/` |
| platform | `handler/platform/` |
| rate_limit | `handler/rate_limit/` |
| analytics_behavior | `handler/analytics_behavior/` |
| analytics_overview | `handler/analytics_overview/` |
| analytics_payments | `handler/analytics_payments/` |
| analytics_retention | `handler/analytics_retention/` |

- [ ] agent
- [ ] node
- [ ] registry
- [ ] config
- [ ] storage
- [ ] backup
- [ ] certificate
- [ ] migrate
- [ ] platform
- [ ] rate_limit
- [ ] analytics_*

### Batch 7 — SSE 流式响应（特殊处理）

| 模块 | 目录 | 特殊点 |
|------|------|--------|
| job | `handler/job/` | SSE streaming |
| function | `handler/function/` | SSE streaming |
| ops | `handler/ops/` | SSE streaming |

- [ ] job
- [ ] function
- [ ] ops

> SSE handler 迁移见 [templates.md](./templates.md#sse-handler)

---

## Logic 层变更

Logic 层本身**不需要修改**，只需将 `logx.Logger` 的 import 保留（或替换为标准 `log/slog`）。
Logic 接收的是 `context.Context`，与框架无关。

唯一需要注意：`logx` 在移除 go-zero 后需替换，但这是 Phase 6 的工作。
