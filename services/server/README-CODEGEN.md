# 代码生成指南

本项目使用 **go_zero** 命名风格（下划线分隔），符合 Go 官方规范。

## 📋 配置

项目已配置 `.goctl.yaml`，默认使用 `go_zero` 风格：

```yaml
style: go_zero
```

生成的文件名示例：
- ✅ `admin_user_handler.go`
- ✅ `user_profile_logic.go`
- ✅ `auth_login_handler.go`

## 🚀 使用方法

### 生成所有代码

```bash
# 方法 1：直接使用 goctl（会自动读取 .goctl.yaml）
goctl api go -api server.api -dir .

# 方法 2：使用生成脚本
./gen-api.sh
```

### 只验证 API 文件

```bash
goctl api validate --api server.api
```

### 生成 Swagger 文档

```bash
goctl api swagger --api server.api --dir .
```

## ⚙️ 配置说明

### .goctl.yaml 配置项

```yaml
# 全局命名风格
style: go_zero

# API 特定配置
api:
  style: go_zero
```

### 支持的命名风格

| 风格 | 示例 | 推荐 |
|------|------|------|
| `go_zero` | `user_handler.go` | ✅ 推荐（Go 官方规范） |
| `gozero` | `userhandler.go` | ⚠️ 默认（可读性差） |
| `goZero` | `userHandler.go` | ❌ 不符合规范 |
| `go-zero` | `user-handler.go` | ❌ 不推荐 |

## 📁 生成的文件结构

```
internal/
├── config/
│   └── config.go                 # 配置结构
├── handler/
│   ├── admin_user_handler.go     # HTTP 处理器
│   ├── user_profile_handler.go
│   └── auth_login_handler.go
├── logic/
│   ├── admin_user_logic.go       # 业务逻辑
│   ├── user_profile_logic.go
│   └── auth_login_logic.go
├── svc/
│   └── service_context.go        # 服务上下文
├── types/
│   └── types.go                  # 类型定义（自动生成，不要修改）
└── middleware/
    └── auth_middleware.go        # 中间件
```

## ⚠️ 注意事项

### 自动覆盖的文件
- `internal/types/types.go` - 每次生成都会覆盖

### 安全修改的文件
- `internal/logic/*.go` - 业务逻辑
- `internal/svc/service_context.go` - 添加依赖
- `internal/middleware/*.go` - 中间件实现

### 谨慎修改的文件
- `internal/handler/*.go` - 可以修改，但重新生成时会被覆盖

## 🔄 工作流程

### 1. 修改 API 定义

编辑 `server.api`：

```go
type UserRequest {
    ID int64 `path:"id"`
}

type UserResponse {
    ID   int64  `json:"id"`
    Name string `json:"name"`
}

@server (
    group: user
    prefix: /api/v1
)
service server-api {
    @handler GetUser
    get /user/:id (UserRequest) returns (UserResponse)
}
```

### 2. 验证 API 语法

```bash
goctl api validate --api server.api
```

### 3. 生成代码

```bash
# 会自动读取 .goctl.yaml 配置
goctl api go -api server.api -dir .
```

生成的文件：
- `internal/handler/get_user_handler.go` ✅
- `internal/logic/get_user_logic.go` ✅
- `internal/types/types.go` ✅

### 4. 实现业务逻辑

编辑 `internal/logic/get_user_logic.go`：

```go
func (l *GetUserLogic) GetUser(req *types.UserRequest) (resp *types.UserResponse, err error) {
    // TODO: 实现你的业务逻辑
    return &types.UserResponse{
        ID:   req.ID,
        Name: "User Name",
    }, nil
}
```

### 5. 测试

```bash
# 启动服务
go run croupier.go -f etc/server.yaml

# 测试接口
curl http://localhost:18780/api/v1/user/1
```

## 🛠️ 高级用法

### 自定义模板

如果需要自定义生成模板，可以在 `~/.goctl` 目录创建模板文件。

### 生成带注释的代码

在 API 定义中添加注释：

```go
// GetUser 获取用户信息
// @Summary 获取用户详情
// @Description 根据用户ID获取用户详细信息
@handler GetUser
get /user/:id (UserRequest) returns (UserResponse)
```

## 📚 参考资源

- go-zero 官方文档：https://go-zero.dev/
- goctl 工具文档：https://go-zero.dev/docs/tasks/cli/goctl
- API 语法参考：https://go-zero.dev/docs/tutorials/api/define

## 🔧 故障排查

### 问题：生成的文件还是全小写

**原因**：可能是 goctl 没有读取到配置文件

**解决**：
1. 确认 `.goctl.yaml` 在项目根目录
2. 手动指定风格：`goctl api go -api server.api -dir . --style=go_zero`
3. 检查 goctl 版本：`goctl --version`（需要 >= 1.3.0）

### 问题：types.go 被覆盖导致编译错误

**解决**：
1. types.go 应该由 API 定义生成，不要手动修改
2. 自定义类型应该定义在其他文件中
3. 使用 `git diff` 查看变更，谨慎合并

### 问题：handler 文件被覆盖

**解决**：
1. handler 应该是薄层，不要在里面写业务逻辑
2. 业务逻辑写在 logic 层
3. 如果必须自定义 handler，在重新生成前备份

## 📊 项目统计

```bash
# 查看生成的文件统计
find internal/handler -name "*.go" | wc -l
find internal/logic -name "*.go" | wc -l

# 查看代码行数
find internal -name "*.go" -exec wc -l {} + | tail -1
```

## 🎯 最佳实践

1. **保持 API 定义的整洁**：合理组织路由分组
2. **定期验证 API 文件**：避免语法错误累积
3. **使用版本控制**：生成前先提交现有代码
4. **遵循命名规范**：Handler 名称简洁清晰
5. **业务逻辑在 logic 层**：不要在 handler 层写复杂逻辑
