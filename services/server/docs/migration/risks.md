# 风险与注意事项

## 高风险点

### 1. httpx.Parse 多源绑定

**风险**：go-zero 的 `httpx.Parse` 会自动合并 path/query/body 参数，Gin 需要分步绑定。

**影响范围**：所有同时包含 `uri:` 和 `json:`/`form:` tag 的 Request struct。

**示例**：
```go
type AdminUpdateRequest struct {
    ID   string `uri:"id"`      // 来自 URL 路径
    Name string `json:"name"`   // 来自 JSON body
}
```

**解决方案**：
```go
// 错误：只绑定一次会丢失数据
if err := c.ShouldBind(&req); err != nil { ... }

// 正确：分两步绑定
if err := c.ShouldBindUri(&req); err != nil { ... }
if err := c.ShouldBindJSON(&req); err != nil { ... }
```

**检测方法**：
```bash
# 查找同时包含 uri: 和 json: 的 struct
grep -A 5 "type.*Request struct" internal/types/types.go | grep -B 3 "uri:" | grep "json:"
```

---

### 2. SSE 超时处理

**风险**：go-zero 的 `RestConf.Timeout` 控制全局超时，Gin 需要单独配置 `http.Server.WriteTimeout`。

**影响范围**：`job/stream_job_handler.go`、`function/function_invoke_handler.go`、`ops/*_handler.go`。

**当前实现问题**：
- `StreamJobLogic.StreamJob` 是同步收集所有事件，不是真正的流式推送
- 如果任务执行时间超过 `WriteTimeout`，连接会被强制断开

**解决方案**：

#### 方案 A：保持当前同步模式（简单）
```go
func StreamJobHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req types.StreamJobRequest
        if err := c.ShouldBindQuery(&req); err != nil {
            response.Error(c, errorx.NewBadRequest(err.Error()))
            return
        }
        l := logic.NewStreamJobLogic(c.Request.Context(), svcCtx)
        resp, err := l.StreamJob(&req)
        if err != nil {
            response.Error(c, err)
        } else {
            response.Success(c, resp) // 一次性返回所有事件
        }
    }
}
```

#### 方案 B：改为真正的 SSE 流式推送（推荐）
```go
func StreamJobHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req types.StreamJobRequest
        if err := c.ShouldBindQuery(&req); err != nil {
            response.Error(c, errorx.NewBadRequest(err.Error()))
            return
        }

        c.Header("Content-Type", "text/event-stream")
        c.Header("Cache-Control", "no-cache")
        c.Header("Connection", "keep-alive")
        c.Header("X-Accel-Buffering", "no")

        l := logic.NewStreamJobLogic(c.Request.Context(), svcCtx)

        // 实时推送事件
        done, err := svcCtx.Dispatcher.StreamJobRealtime(c.Request.Context(), req.JobID, func(evt *sdkv1.JobEvent) bool {
            if evt == nil {
                return true
            }
            data, _ := json.Marshal(map[string]interface{}{
                "type":     evt.Type,
                "message":  evt.Message,
                "progress": evt.GetProgress(),
                "payload":  evt.Payload,
            })
            fmt.Fprintf(c.Writer, "data: %s\n\n", data)
            c.Writer.Flush()
            return true
        })

        if err != nil {
            fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", err.Error())
        } else {
            fmt.Fprintf(c.Writer, "event: done\ndata: {\"done\":%t}\n\n", done)
        }
    }
}
```

**cmd/root.go 超时配置**：
```go
srv := &http.Server{
    Addr:         addr,
    Handler:      r,
    ReadTimeout:  10 * time.Second,
    WriteTimeout: 10 * time.Minute, // SSE 需要长超时
    IdleTimeout:  120 * time.Second,
}
return srv.ListenAndServe()
```

---

### 3. conf.MustLoad 环境变量展开

**风险**：go-zero 的 `conf.UseEnv()` 支持 `${VAR}` 语法，标准 `yaml.Unmarshal` 不支持。

**影响范围**：`etc/server.yaml` 中所有使用 `${...}` 的字段。

**示例**：
```yaml
Database:
  datasource: ${DB_DSN:postgres://localhost/croupier}
Auth:
  JWTSecret: ${JWT_SECRET}
```

**解决方案**：
```go
// cmd/root.go
data, err := os.ReadFile(cfgFile)
if err != nil {
    return fmt.Errorf("读取配置文件失败: %w", err)
}

// 展开环境变量（支持 ${VAR:default} 语法）
expanded := os.ExpandEnv(string(data))

var c config.Config
if err := yaml.Unmarshal([]byte(expanded), &c); err != nil {
    return fmt.Errorf("解析配置文件失败: %w", err)
}
```

**注意**：`os.ExpandEnv` 只支持 `${VAR}` 和 `$VAR`，不支持 `${VAR:default}` 默认值语法。
如需默认值，使用 Viper 或手动实现。

---

### 4. YAML 字段名变更

**风险**：`config.Config` 从嵌入 `rest.RestConf` 改为独立 `Server` 字段，YAML 结构变化。

**Before**：
```yaml
Name: croupier-server
Host: 0.0.0.0
Port: 8080
Mode: dev
Timeout: 600000
```

**After**：
```yaml
Server:
  Host: 0.0.0.0
  Port: 8080
  Mode: dev
  Timeout: 600000
```

**影响**：所有环境的配置文件（`etc/server.yaml`、`etc/server-prod.yaml` 等）需要同步更新。

**迁移检查清单**：
- [ ] `etc/server.yaml`
- [ ] `etc/server-prod.yaml`（如存在）
- [ ] `etc/server-test.yaml`（如存在）
- [ ] Docker/K8s ConfigMap 中的配置
- [ ] CI/CD 环境变量配置

---

### 5. cli/common 包依赖

**风险**：`github.com/cuihairu/croupier/internal/cli/common` 包可能依赖 go-zero。

**检测方法**：
```bash
grep -r "zeromicro" /c/Users/cui/Workspaces/croupier/croupier/internal/cli/common/
```

**可能的依赖**：
- `logx` 日志
- `conf.MustLoad` 配置加载
- `rest.RestConf` 配置结构

**解决方案**：
- 如果 `common` 包是共享包，需要同步重构
- 如果只有 `server` 使用，可以将相关代码移到 `server/internal` 下

---

## 中风险点

### 6. 认证中间件签名变更

**风险**：`svc.ServiceContext.Authority` 从 `rest.Middleware` 改为 `gin.HandlerFunc`。

**影响范围**：`internal/svc/service_context.go`、`internal/middleware/auth.go`。

**检查点**：
- `NewAuthMiddleware` 返回类型是否为 `gin.HandlerFunc`
- 中间件内部是否使用 `*gin.Context` 而非 `http.ResponseWriter`

---

### 7. 错误响应格式一致性

**风险**：go-zero 的 `httpx.Error` 自动识别 `CodeError` 并设置状态码，Gin 需要手动处理。

**当前实现**：
```go
// internal/common/errorx/errors.go
type CodeError struct {
    Code int
    Msg  string
}

func (e *CodeError) HTTPStatus() int {
    // 根据 Code 映射 HTTP 状态码
}
```

**Gin response.Error 实现**：
```go
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

**测试要点**：
- 400 Bad Request
- 401 Unauthorized
- 403 Forbidden
- 404 Not Found
- 500 Internal Server Error

---

### 8. 路由参数命名

**风险**：go-zero 使用 `:id`，Gin 也使用 `:id`，但绑定方式不同。

**go-zero**：`httpx.Parse` 自动绑定到 `path:"id"` tag。
**Gin**：需要 `c.ShouldBindUri` 绑定到 `uri:"id"` tag。

**检查点**：
- 所有 `path:"id"` 已改为 `uri:"id"`
- Handler 中使用 `ShouldBindUri` 而非 `ShouldBind`

---

## 低风险点

### 9. 日志格式变更

**风险**：`logx` 改为 `slog`，日志格式和字段名可能不同。

**影响**：日志聚合系统（如 ELK）的查询语句可能需要调整。

---

### 10. 性能差异

**风险**：Gin 和 go-zero 的性能特性不同。

**建议**：迁移后进行压测，对比 QPS、延迟、内存占用。

---

## 回滚计划

如果迁移失败，需要快速回滚：

1. **保留 main 分支**：不要合并 `feature/migrate-gin` 直到完全验证通过
2. **分阶段合并**：每个 Phase 完成后创建 tag（如 `migrate-phase1`）
3. **数据库兼容性**：迁移不涉及数据库 schema 变更，可以直接回滚代码
4. **配置文件版本**：保留旧版 `etc/server.yaml.bak`

**回滚命令**：
```bash
git checkout main
git branch -D feature/migrate-gin
# 重新部署 main 分支
```
