# Extension Route Registration Draft

更新时间：2026-03-15
状态：草案

本文件定义扩展系统第一阶段在 `internal/handler/routes.go` 中的路由注册方案。

---

## 1. 目标

新增 `extension` API，同时不打断现有 API 结构和旧平台兼容入口。

要求：

- 新增 `extensions` 路由分组
- 保持与现有 `platform`、`analytics` 等旧路由并存
- 后续扩展迁移时，逐步把旧业务转移到扩展入口

---

## 2. 路由分组建议

建议新增：

- `/api/v1/extensions/catalog`
- `/api/v1/extensions/installations`
- `/api/v1/extensions/install`

完整建议：

- `GET /api/v1/extensions/catalog`
- `GET /api/v1/extensions/catalog/:id`
- `GET /api/v1/extensions/installations`
- `POST /api/v1/extensions/install`
- `GET /api/v1/extensions/installations/:id`
- `PUT /api/v1/extensions/installations/:id/config`
- `POST /api/v1/extensions/installations/:id/enable`
- `POST /api/v1/extensions/installations/:id/disable`
- `POST /api/v1/extensions/installations/:id/upgrade`
- `POST /api/v1/extensions/installations/:id/reconcile`
- `DELETE /api/v1/extensions/installations/:id`
- `GET /api/v1/extensions/installations/:id/events`

---

## 3. 路由权限建议

`extensions` 全部走受保护路由：

```go
protected := v1.Group("/")
protected.Use(serverCtx.Authority)
```

不建议第一阶段开放匿名 catalog。

原因：

- 扩展目录本质是后台运维能力的一部分
- 内部商店和安装管理不属于公开 API

---

## 4. routes.go 修改建议

### 4.1 import 新增

建议新增：

```go
import "github.com/cuihairu/croupier/internal/api/extension"
```

### 4.2 RegisterHandlers 中新增

建议放在核心基础模块附近，紧挨 `config / ops / platform` 一带。

伪代码：

```go
protected := v1.Group("/")
protected.Use(serverCtx.Authority)
{
    ...
    registerExtensionRoutes(protected.Group("/extensions"), serverCtx)
    ...
}
```

### 4.3 新增注册函数

```go
func registerExtensionRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
    extensionSvc := extension.NewService(ctx)
    extensionHandler := extension.NewHandler(extensionSvc)

    g.GET("/catalog", extensionHandler.CatalogList)
    g.GET("/catalog/:id", extensionHandler.CatalogDetail)

    g.GET("/installations", extensionHandler.InstallationList)
    g.POST("/install", extensionHandler.Install)
    g.GET("/installations/:id", extensionHandler.InstallationDetail)
    g.PUT("/installations/:id/config", extensionHandler.UpdateConfig)
    g.POST("/installations/:id/enable", extensionHandler.Enable)
    g.POST("/installations/:id/disable", extensionHandler.Disable)
    g.POST("/installations/:id/upgrade", extensionHandler.Upgrade)
    g.POST("/installations/:id/reconcile", extensionHandler.Reconcile)
    g.DELETE("/installations/:id", extensionHandler.Uninstall)
    g.GET("/installations/:id/events", extensionHandler.Events)
}
```

---

## 5. 与旧路由的关系

### 5.1 第一阶段保留

以下旧路由继续保留：

- `/api/v1/platforms`
- `/api/v1/analytics`
- `/api/v1/alerts`
- `/api/v1/approvals`

### 5.2 第二阶段策略

随着扩展迁移推进：

- `platform` 相关安装与管理界面改走 `/extensions`
- 原 `/platforms` 保留能力调用和兼容入口

### 5.3 第三阶段策略

当对应模块完全扩展化后，再评估是否收缩旧路由。

---

## 6. Handler 结构建议

`internal/api/extension/handler.go` 第一阶段建议只保留一个 handler：

```go
type Handler struct {
    svc *Service
}
```

方法：

- `CatalogList`
- `CatalogDetail`
- `InstallationList`
- `Install`
- `InstallationDetail`
- `UpdateConfig`
- `Enable`
- `Disable`
- `Upgrade`
- `Reconcile`
- `Uninstall`
- `Events`

不要过早拆多个 handler 文件。

---

## 7. Service 结构建议

`internal/api/extension/service.go` 不直接做复杂逻辑，而是组装：

- `ctx.Extensions.Catalog`
- `ctx.Extensions.Installation`
- `ctx.Extensions.Runtime`

这样 API 层只是聚合和 DTO 转换，不会把业务逻辑再次堆进 handler。

---

## 8. 第一阶段路由验收标准

- 新增 `/api/v1/extensions/*`
- 能完成 catalog list/detail
- 能完成 installation install/list/detail
- 能完成 enable/disable/reconcile/uninstall
- 不影响旧 `/platforms` 等接口

---

## 9. 下一步

下一步需要补：

- `internal/api/extension` 代码骨架
- handler DTO 到 service request 的映射规则
- 权限码与路由动作对照表
