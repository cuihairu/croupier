# 代码模板参考

## 标准 Handler 模板

### GET 列表（Query 参数）

```go
func XxxListHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req types.XxxListRequest
        if err := c.ShouldBindQuery(&req); err != nil {
            response.Error(c, errorx.NewBadRequest(err.Error()))
            return
        }
        l := logic.NewXxxListLogic(c.Request.Context(), svcCtx)
        resp, err := l.XxxList(&req)
        if err != nil {
            response.Error(c, err)
        } else {
            response.Success(c, resp)
        }
    }
}
```

### GET 详情（Path 参数）

```go
func XxxDetailHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req types.XxxDetailRequest
        if err := c.ShouldBindUri(&req); err != nil {
            response.Error(c, errorx.NewBadRequest(err.Error()))
            return
        }
        l := logic.NewXxxDetailLogic(c.Request.Context(), svcCtx)
        resp, err := l.XxxDetail(&req)
        if err != nil {
            response.Error(c, err)
        } else {
            response.Success(c, resp)
        }
    }
}
```

### POST/PUT（JSON Body）

```go
func XxxCreateHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req types.XxxCreateRequest
        if err := c.ShouldBindJSON(&req); err != nil {
            response.Error(c, errorx.NewBadRequest(err.Error()))
            return
        }
        l := logic.NewXxxCreateLogic(c.Request.Context(), svcCtx)
        resp, err := l.XxxCreate(&req)
        if err != nil {
            response.Error(c, err)
        } else {
            response.Success(c, resp)
        }
    }
}
```

### PUT（Path + JSON Body）

```go
func XxxUpdateHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req types.XxxUpdateRequest
        if err := c.ShouldBindUri(&req); err != nil {
            response.Error(c, errorx.NewBadRequest(err.Error()))
            return
        }
        if err := c.ShouldBindJSON(&req); err != nil {
            response.Error(c, errorx.NewBadRequest(err.Error()))
            return
        }
        l := logic.NewXxxUpdateLogic(c.Request.Context(), svcCtx)
        resp, err := l.XxxUpdate(&req)
        if err != nil {
            response.Error(c, err)
        } else {
            response.Success(c, resp)
        }
    }
}
```

### DELETE（Path 参数）

```go
func XxxDeleteHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req types.XxxDeleteRequest
        if err := c.ShouldBindUri(&req); err != nil {
            response.Error(c, errorx.NewBadRequest(err.Error()))
            return
        }
        l := logic.NewXxxDeleteLogic(c.Request.Context(), svcCtx)
        resp, err := l.XxxDelete(&req)
        if err != nil {
            response.Error(c, err)
        } else {
            response.Success(c, resp)
        }
    }
}
```

---

## SSE Handler 模板

SSE（Server-Sent Events）handler 需要特殊处理，不能使用 `response.Success`。

```go
func StreamJobHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req types.StreamJobRequest
        if err := c.ShouldBindQuery(&req); err != nil {
            response.Error(c, errorx.NewBadRequest(err.Error()))
            return
        }

        // 设置 SSE 响应头
        c.Header("Content-Type", "text/event-stream")
        c.Header("Cache-Control", "no-cache")
        c.Header("Connection", "keep-alive")
        c.Header("X-Accel-Buffering", "no") // 禁用 Nginx 缓冲

        l := logic.NewStreamJobLogic(c.Request.Context(), svcCtx)

        // 使用 c.Stream 推送事件
        c.Stream(func(w io.Writer) bool {
            select {
            case <-c.Request.Context().Done():
                return false // 客户端断开
            default:
                evt, done, err := l.NextEvent()
                if err != nil {
                    fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
                    return false
                }
                if done {
                    fmt.Fprintf(w, "event: done\ndata: {}\n\n")
                    return false
                }
                data, _ := json.Marshal(evt)
                fmt.Fprintf(w, "data: %s\n\n", data)
                return true
            }
        })
    }
}
```

> 注意：SSE logic 层需要重构为迭代器模式（`NextEvent()`），而不是一次性收集所有事件。
> 当前 `StreamJobLogic.StreamJob` 是同步收集，迁移时需要评估是否改为真正的流式推送。

---

## response 包模板

```go
// internal/common/response/response.go
package response

import (
    "errors"
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/cuihairu/croupier/services/server/internal/common/errorx"
)

func Success(c *gin.Context, data interface{}) {
    c.JSON(http.StatusOK, data)
}

func Created(c *gin.Context, data interface{}) {
    c.JSON(http.StatusCreated, data)
}

func NoContent(c *gin.Context) {
    c.Status(http.StatusNoContent)
}

func Error(c *gin.Context, err error) {
    var codeErr *errorx.CodeError
    if errors.As(err, &codeErr) {
        c.JSON(codeErr.HTTPStatus(), gin.H{
            "code":    codeErr.Code,
            "message": codeErr.Msg,
        })
        return
    }
    c.JSON(http.StatusInternalServerError, gin.H{
        "code":    500,
        "message": err.Error(),
    })
}

type ListResponse struct {
    Items interface{} `json:"items"`
    Total int64       `json:"total"`
    Page  int         `json:"page"`
    Size  int         `json:"pageSize"`
}

func SuccessList(c *gin.Context, items interface{}, total int64, page, size int) {
    Success(c, ListResponse{
        Items: items,
        Total: total,
        Page:  page,
        Size:  size,
    })
}
```

---

## 路由组模板

```go
// internal/handler/routes.go
package handler

import (
    "github.com/gin-gonic/gin"
    "github.com/cuihairu/croupier/services/server/internal/handler/admin"
    "github.com/cuihairu/croupier/services/server/internal/handler/auth"
    // ... 其他 handler 包
    "github.com/cuihairu/croupier/services/server/internal/svc"
)

func RegisterHandlers(r *gin.Engine, serverCtx *svc.ServiceContext) {
    v1 := r.Group("/api/v1")

    registerAuthRoutes(v1, serverCtx)
    registerAdminRoutes(v1, serverCtx)
    // ... 其他路由注册函数
}

func registerAuthRoutes(v1 *gin.RouterGroup, ctx *svc.ServiceContext) {
    g := v1.Group("/auth")
    g.POST("/login", auth.LoginHandler(ctx))
    g.POST("/logout", auth.LogoutHandler(ctx))
}

func registerAdminRoutes(v1 *gin.RouterGroup, ctx *svc.ServiceContext) {
    g := v1.Group("/admin")
    g.GET("/", admin.AdminsListHandler(ctx))
    g.POST("/", admin.AdminCreateHandler(ctx))
    g.GET("/:id", admin.AdminDetailHandler(ctx))
    g.PUT("/:id", admin.AdminUpdateHandler(ctx))
    g.DELETE("/:id", admin.AdminDeleteHandler(ctx))
    g.POST("/:id/password-reset", admin.AdminPasswordResetHandler(ctx))
}
```
