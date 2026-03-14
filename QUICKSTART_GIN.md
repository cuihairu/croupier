# 快速开始：Gin 迁移

## 📦 第一步：安装依赖

```bash
cd services/server

# 安装 Gin 和相关依赖
go get -u github.com/gin-gonic/gin
go get -u go.uber.org/zap
go get -u github.com/spf13/viper

# 更新 go.mod
go mod tidy
```

## 🚀 第二步：启动 Gin 服务

```bash
# 构建 Gin 服务
go build -o bin/croupier-server-gin ./cmd/server-gin

# 运行（使用不同端口，避免与 go-zero 冲突）
./bin/croupier-server-gin --port 18781
```

## 🧪 第三步：测试 API

### 测试登录
```bash
curl -X POST http://localhost:18781/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "admin123"
  }'
```

### 测试获取 Profile
```bash
# 使用上一步返回的 token
TOKEN="your-token-here"

curl -X GET http://localhost:18781/api/v1/profile \
  -H "Authorization: Bearer $TOKEN"
```

### 测试获取游戏列表
```bash
curl -X GET http://localhost:18781/api/v1/profile/games \
  -H "Authorization: Bearer $TOKEN"
```

## 📝 第四步：迁移其他模块

使用代码生成脚本快速创建新模块：

```bash
# 生成 Admin 模块
./scripts/gen-gin-module.sh admin

# 生成 Game 模块
./scripts/gen-gin-module.sh game

# 生成 Player 模块
./scripts/gen-gin-module.sh player
```

然后按照以下步骤完成迁移：

1. **编辑 `dto.go`**：定义请求/响应结构
2. **编辑 `service.go`**：实现业务逻辑（从 go-zero logic 复制）
3. **编辑 `handler.go`**：实现 HTTP 处理器
4. **编辑 `routes.go`**：注册路由
5. **在 `router/router.go` 中注册模块路由**

## 🔧 常见问题

### Q1: 如何复用现有的 Model 层？
A: 直接使用，无需修改。Gin 服务和 go-zero 服务共享同一个 Model 层。

```go
// 在 Service 中注入 Model
type Service struct {
    adminModel *model.AdminModel
}

func NewService(adminModel *model.AdminModel) *Service {
    return &Service{adminModel: adminModel}
}
```

### Q2: 如何处理权限控制？
A: 使用 Casbin 中间件（保持与 go-zero 一致）

```go
// 在路由中应用权限中间件
authenticated.Use(middleware.Permission("admin:read"))
```

### Q3: 如何处理文件上传？
A: 使用 Gin 的 `c.FormFile()` 或 `c.MultipartForm()`

```go
func (h *Handler) Upload(c *gin.Context) {
    file, err := c.FormFile("file")
    if err != nil {
        response.BadRequest(c, "文件上传失败")
        return
    }

    // 保存文件
    c.SaveUploadedFile(file, "/path/to/save")
}
```

### Q4: 如何处理 SSE（Server-Sent Events）？
A: 使用 Gin 的流式响应

```go
func (h *Handler) Stream(c *gin.Context) {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")

    for {
        c.SSEvent("message", "data")
        c.Writer.Flush()
        time.Sleep(time.Second)
    }
}
```

## 📊 迁移进度跟踪

创建一个 checklist 来跟踪迁移进度：

```markdown
## 模块迁移状态

- [x] 基础框架搭建
- [x] Auth 模块（登录/登出）
- [x] Profile 模块（个人资料/游戏列表）
- [ ] Admin 模块
- [ ] Game 模块
- [ ] Player 模块
- [ ] Function 模块
- [ ] Analytics 模块
- [ ] Workspace 模块
- [ ] ... 其他模块
```

## 🎯 下一步

1. **完善 Auth 和 Profile 模块**：修复 Model 方法调用错误
2. **迁移 Admin 模块**：管理员 CRUD 操作
3. **迁移 Game 模块**：游戏管理
4. **编写测试用例**：确保功能正确
5. **性能测试**：对比 go-zero 和 Gin 的性能

## 💡 最佳实践

1. **保持 API 兼容**：路径和响应格式与 go-zero 保持一致
2. **复用现有代码**：Model 层、工具函数等直接复用
3. **渐进式迁移**：一个模块一个模块地迁移，降低风险
4. **编写测试**：每个模块迁移完成后编写测试用例
5. **监控指标**：关注错误率、响应时间、内存使用等

## 📚 参考资源

- [Gin 官方文档](https://gin-gonic.com/docs/)
- [GORM 文档](https://gorm.io/docs/)
- [Zap 日志库](https://github.com/uber-go/zap)
- [Viper 配置管理](https://github.com/spf13/viper)
- [迁移计划详细文档](./MIGRATION_TO_GIN.md)
