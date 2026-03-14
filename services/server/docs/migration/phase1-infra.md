# Phase 1 — 基础设施替换

状态: ⬜ 待开始

## 目标

替换 go-zero 的核心基础设施：配置加载、响应工具、错误处理、ServiceContext。

## 检查清单

- [ ] 1.1 重构 `internal/config/config.go` — 移除 `rest.RestConf` 嵌入
- [ ] 1.2 更新 `etc/server.yaml` — 替换 go-zero 字段为标准字段
- [ ] 1.3 重写 `internal/common/response/response.go` — 改用 Gin context
- [ ] 1.4 重写 `internal/common/errorx/errors.go` — 保持 CodeError 结构，移除 httpx 依赖
- [ ] 1.5 更新 `internal/svc/service_context.go` — Authority 字段类型改为 `gin.HandlerFunc`

---

## 1.1 config.go 重构

### Before
```go
import "github.com/zeromicro/go-zero/rest"

type Config struct {
    rest.RestConf
    // ...
}
```

### After
```go
type Config struct {
    Server ServerConfig `yaml:"Server"`
    // ...
}

type ServerConfig struct {
    Host    string `yaml:"Host"`
    Port    int    `yaml:"Port"`
    Mode    string `yaml:"Mode"`    // dev | prod | test
    Timeout int    `yaml:"Timeout"` // 毫秒
}
```

---

## 1.2 server.yaml 字段变更

### Before (go-zero RestConf 字段)
```yaml
Name: croupier-server
Host: 0.0.0.0
Port: 8080
Mode: dev
Timeout: 600000
```

### After
```yaml
Server:
  Host: 0.0.0.0
  Port: 8080
  Mode: dev
  Timeout: 600000
```

---

## 1.3 response.go 重写

### Before (go-zero httpx)
```go
import "github.com/zeromicro/go-zero/rest/httpx"

func Success(w http.ResponseWriter, data interface{}) {
    httpx.OkJson(w, data)
}

func Error(w http.ResponseWriter, r *http.Request, err error) {
    httpx.ErrorCtx(r.Context(), w, err)
}
```

### After (Gin)
```go
import "github.com/gin-gonic/gin"

func Success(c *gin.Context, data interface{}) {
    c.JSON(http.StatusOK, data)
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
```

---

## 1.4 errorx/errors.go 变更

`CodeError` 结构体保持不变，只需移除对 `httpx` 的依赖（如有）。
确认 `HTTPStatus()` 方法存在并返回正确的 HTTP 状态码。

---

## 1.5 svc/service_context.go 变更

### Before
```go
import "github.com/zeromicro/go-zero/rest"

type ServiceContext struct {
    Authority rest.Middleware
    // ...
}
```

### After
```go
import "github.com/gin-gonic/gin"

type ServiceContext struct {
    Authority gin.HandlerFunc
    // ...
}
```
