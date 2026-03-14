# Croupier Server 迁移计划：从 go-zero 到 Gin

## 📋 项目概况

**当前状态分析：**
- 总代码文件：~690 个 Go 文件
- 框架：go-zero (REST) + NNG (控制平面)
- 业务模块：40+ 个独立模块
- 数据库：GORM (MySQL/PostgreSQL/SQLite/SQLServer)
- 认证：JWT + RBAC/ABAC
- 核心依赖：Casbin, Redis, ClickHouse, Kafka, OpenTelemetry

**迁移原因：**
1. go-zero 代码生成机制频繁导致业务逻辑被覆盖/清空
2. handler/logic 分层过于冗余，维护成本高
3. 类型定义与实际模型不匹配，频繁出现编译错误
4. 缺乏灵活性，难以进行细粒度控制

---

## 🎯 迁移目标

### 核心目标
1. **稳定性优先**：避免代码生成工具破坏已有业务逻辑
2. **简化架构**：减少不必要的抽象层，提高代码可维护性
3. **保持兼容**：API 接口保持不变，前端无需修改
4. **渐进式迁移**：分模块逐步迁移，降低风险

### 技术选型
- **Web 框架**：Gin (高性能、简洁、社区活跃)
- **ORM**：GORM (保持不变)
- **认证**：JWT + 自定义中间件
- **权限**：Casbin (保持不变)
- **配置**：Viper (替代 go-zero conf)
- **日志**：Zap (替代 go-zero logx)
- **验证**：go-playground/validator (Gin 内置)

---

## 🏗️ 新架构设计

### 目录结构（推荐）

```
services/server/
├── cmd/
│   └── server/
│       └── main.go                 # 入口文件
├── internal/
│   ├── api/                        # API 路由和处理器（合并 handler + logic）
│   │   ├── admin/                  # 管理员模块
│   │   │   ├── handler.go          # HTTP 处理器
│   │   │   ├── service.go          # 业务逻辑
│   │   │   └── dto.go              # 请求/响应 DTO
│   │   ├── auth/                   # 认证模块
│   │   ├── game/                   # 游戏模块
│   │   ├── player/                 # 玩家模块
│   │   ├── function/               # 函数管理
│   │   ├── analytics/              # 数据分析
│   │   └── ...                     # 其他模块
│   ├── middleware/                 # 中间件
│   │   ├── auth.go                 # 认证中间件
│   │   ├── permission.go           # 权限中间件
│   │   ├── cors.go                 # CORS
│   │   ├── logger.go               # 日志中间件
│   │   └── recovery.go             # 错误恢复
│   ├── model/                      # 数据模型（保持不变）
│   ├── repository/                 # 数据访问层（可选，封装 model）
│   ├── service/                    # 共享服务（权限、缓存等）
│   ├── pkg/                        # 工具包
│   │   ├── response/               # 统一响应格式
│   │   ├── errors/                 # 错误定义
│   │   ├── jwt/                    # JWT 工具
│   │   ├── validator/              # 验证器
│   │   └── logger/                 # 日志工具
│   └── config/                     # 配置结构
├── configs/                        # 配置文件
│   ├── server.yaml
│   └── server.dev.yaml
├── docs/                           # API 文档
└── scripts/                        # 脚本工具
```

### 架构对比

**go-zero 架构（当前）：**
```
Request → Handler → Logic → Model → Database
          ↓         ↓
        types    svc.Context
```
- **问题**：Handler 只做参数解析，Logic 做业务逻辑，层次过多
- **问题**：types 定义与 model 不一致，频繁转换

**Gin 架构（目标）：**
```
Request → Handler (含业务逻辑) → Service (可选) → Repository/Model → Database
          ↓
        Middleware (Auth, Permission, Logger)
```
- **优势**：Handler 直接处理业务逻辑，减少抽象层
- **优势**：DTO 与 Model 分离清晰，按需转换
- **优势**：中间件链式调用，灵活可控

---

## 📝 迁移策略

### 阶段 1：基础设施准备（1-2 天）

**目标**：搭建 Gin 基础框架，不影响现有服务

#### 1.1 创建新的 Gin 服务入口
```go
// cmd/server-gin/main.go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/cuihairu/croupier/services/server/internal/config"
    "github.com/cuihairu/croupier/services/server/internal/router"
)

func main() {
    // 加载配置
    cfg := config.Load()

    // 初始化 Gin
    r := gin.New()

    // 注册中间件
    r.Use(gin.Recovery())
    r.Use(middleware.Logger())
    r.Use(middleware.CORS())

    // 注册路由
    router.RegisterRoutes(r, cfg)

    // 启动服务（使用不同端口，如 18781）
    r.Run(":18781")
}
```

#### 1.2 实现核心中间件
- **认证中间件**：从 go-zero 的 AuthMiddleware 迁移
- **权限中间件**：集成 Casbin
- **日志中间件**：使用 Zap
- **错误处理中间件**：统一错误响应格式

#### 1.3 统一响应格式
```go
// internal/pkg/response/response.go
type Response struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

func Success(c *gin.Context, data interface{}) {
    c.JSON(200, Response{Code: 0, Message: "success", Data: data})
}

func Error(c *gin.Context, code int, message string) {
    c.JSON(200, Response{Code: code, Message: message})
}
```

---

### 阶段 2：核心模块迁移（3-5 天）

**优先级排序**：
1. **Auth 模块**（登录/登出）- 最高优先级
2. **Profile 模块**（用户信息/游戏列表）- 高优先级
3. **Admin 模块**（管理员 CRUD）
4. **Game 模块**（游戏管理）
5. **Function 模块**（函数管理）

#### 2.1 Auth 模块迁移示例

**步骤 1：定义 DTO**
```go
// internal/api/auth/dto.go
package auth

type LoginRequest struct {
    Username string `json:"username" binding:"required"`
    Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
    Token string   `json:"token"`
    User  UserInfo `json:"user"`
}

type UserInfo struct {
    Username string   `json:"username"`
    Nickname string   `json:"nickname"`
    Roles    []string `json:"roles"`
}
```

**步骤 2：实现 Service**
```go
// internal/api/auth/service.go
package auth

import (
    "context"
    "errors"
    "github.com/cuihairu/croupier/services/server/internal/model"
    "github.com/cuihairu/croupier/services/server/internal/pkg/jwt"
)

type Service struct {
    adminModel *model.AdminModel
}

func NewService(adminModel *model.AdminModel) *Service {
    return &Service{adminModel: adminModel}
}

func (s *Service) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
    // 验证用户
    admin, err := s.adminModel.ValidatePassword(ctx, req.Username, req.Password)
    if err != nil {
        return nil, errors.New("用户名或密码错误")
    }

    // 获取角色
    roles, err := s.adminModel.GetAdminRoles(ctx, admin.ID)
    if err != nil {
        return nil, err
    }

    // 生成 Token
    token, err := jwt.GenerateToken(admin.Username, roles)
    if err != nil {
        return nil, err
    }

    return &LoginResponse{
        Token: token,
        User: UserInfo{
            Username: admin.Username,
            Nickname: admin.Nickname,
            Roles:    extractRoleNames(roles),
        },
    }, nil
}
```

**步骤 3：实现 Handler**
```go
// internal/api/auth/handler.go
package auth

import (
    "github.com/gin-gonic/gin"
    "github.com/cuihairu/croupier/services/server/internal/pkg/response"
)

type Handler struct {
    service *Service
}

func NewHandler(service *Service) *Handler {
    return &Handler{service: service}
}

func (h *Handler) Login(c *gin.Context) {
    var req LoginRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        response.Error(c, 400, "参数错误: "+err.Error())
        return
    }

    resp, err := h.service.Login(c.Request.Context(), &req)
    if err != nil {
        response.Error(c, 401, err.Error())
        return
    }

    response.Success(c, resp)
}

func (h *Handler) Logout(c *gin.Context) {
    // 登出逻辑（如果需要）
    response.Success(c, nil)
}
```

**步骤 4：注册路由**
```go
// internal/router/auth.go
package router

import (
    "github.com/gin-gonic/gin"
    "github.com/cuihairu/croupier/services/server/internal/api/auth"
)

func RegisterAuthRoutes(r *gin.RouterGroup, handler *auth.Handler) {
    authGroup := r.Group("/auth")
    {
        authGroup.POST("/login", handler.Login)
        authGroup.POST("/logout", handler.Logout)
    }
}
```

#### 2.2 Profile 模块迁移

**关键点**：
- 需要认证中间件保护
- 从 JWT token 中提取用户信息
- 查询用户的游戏权限列表

```go
// internal/api/profile/handler.go
func (h *Handler) GetProfile(c *gin.Context) {
    // 从 context 中获取当前用户（由认证中间件注入）
    username := c.GetString("username")

    profile, err := h.service.GetProfile(c.Request.Context(), username)
    if err != nil {
        response.Error(c, 500, err.Error())
        return
    }

    response.Success(c, profile)
}

func (h *Handler) GetGames(c *gin.Context) {
    username := c.GetString("username")

    games, err := h.service.GetUserGames(c.Request.Context(), username)
    if err != nil {
        response.Error(c, 500, err.Error())
        return
    }

    response.Success(c, games)
}
```

---

### 阶段 3：批量迁移其他模块（5-10 天）

**策略**：
1. **按模块优先级迁移**：先迁移高频使用的模块
2. **复用现有 Model 层**：GORM 模型无需修改
3. **保持 API 兼容**：路由路径和响应格式保持一致
4. **编写迁移脚本**：自动化生成 Handler/Service 模板

#### 3.1 模块迁移模板

创建代码生成脚本：
```bash
# scripts/gen-module.sh
#!/bin/bash
MODULE_NAME=$1

mkdir -p internal/api/$MODULE_NAME
cat > internal/api/$MODULE_NAME/dto.go << EOF
package $MODULE_NAME

// TODO: 定义请求/响应结构
EOF

cat > internal/api/$MODULE_NAME/service.go << EOF
package $MODULE_NAME

import (
    "context"
    "github.com/cuihairu/croupier/services/server/internal/model"
)

type Service struct {
    // TODO: 注入依赖
}

func NewService() *Service {
    return &Service{}
}
EOF

cat > internal/api/$MODULE_NAME/handler.go << EOF
package $MODULE_NAME

import (
    "github.com/gin-gonic/gin"
    "github.com/cuihairu/croupier/services/server/internal/pkg/response"
)

type Handler struct {
    service *Service
}

func NewHandler(service *Service) *Handler {
    return &Handler{service: service}
}
EOF

echo "Module $MODULE_NAME created successfully!"
```

#### 3.2 迁移检查清单

每个模块迁移完成后检查：
- [ ] DTO 定义完整
- [ ] Service 业务逻辑正确
- [ ] Handler 参数验证完善
- [ ] 路由注册正确
- [ ] 中间件应用正确（认证/权限）
- [ ] 错误处理完善
- [ ] 单元测试通过
- [ ] API 测试通过

---

### 阶段 4：测试与切换（2-3 天）

#### 4.1 并行运行测试
- go-zero 服务运行在 `:18780`
- Gin 服务运行在 `:18781`
- 使用 Nginx 进行流量分流测试

#### 4.2 API 兼容性测试
```bash
# 测试脚本
#!/bin/bash
BASE_URL_OLD="http://localhost:18780"
BASE_URL_NEW="http://localhost:18781"

# 测试登录
curl -X POST $BASE_URL_NEW/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'

# 测试获取 Profile
TOKEN="..."
curl -X GET $BASE_URL_NEW/api/v1/profile \
  -H "Authorization: Bearer $TOKEN"
```

#### 4.3 性能对比测试
使用 Apache Bench 或 wrk 进行压测：
```bash
# 测试 go-zero
ab -n 10000 -c 100 http://localhost:18780/api/v1/monitoring/health

# 测试 Gin
ab -n 10000 -c 100 http://localhost:18781/api/v1/monitoring/health
```

#### 4.4 切换策略
1. **灰度发布**：先切换 10% 流量到 Gin
2. **监控指标**：错误率、响应时间、CPU/内存
3. **逐步扩大**：10% → 50% → 100%
4. **回滚预案**：保留 go-zero 服务 1 周

---

## 🔧 技术细节

### 依赖注入（推荐使用 Wire）

```go
// internal/wire/wire.go
//go:build wireinject
// +build wireinject

package wire

import (
    "github.com/google/wire"
    "github.com/cuihairu/croupier/services/server/internal/api/auth"
    "github.com/cuihairu/croupier/services/server/internal/model"
)

func InitializeAuthHandler(db *gorm.DB) *auth.Handler {
    wire.Build(
        model.NewAdminModel,
        auth.NewService,
        auth.NewHandler,
    )
    return nil
}
```

### 配置管理（Viper）

```go
// internal/config/config.go
package config

import (
    "github.com/spf13/viper"
)

type Config struct {
    Server   ServerConfig
    Database DatabaseConfig
    Redis    RedisConfig
    JWT      JWTConfig
    // ... 其他配置
}

func Load() *Config {
    viper.SetConfigName("server")
    viper.SetConfigType("yaml")
    viper.AddConfigPath("./configs")

    if err := viper.ReadInConfig(); err != nil {
        panic(err)
    }

    var cfg Config
    if err := viper.Unmarshal(&cfg); err != nil {
        panic(err)
    }

    return &cfg
}
```

### 日志（Zap）

```go
// internal/pkg/logger/logger.go
package logger

import (
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

func Init(level string) {
    config := zap.NewProductionConfig()
    config.EncoderConfig.TimeKey = "timestamp"
    config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

    var err error
    Logger, err = config.Build()
    if err != nil {
        panic(err)
    }
}

func Info(msg string, fields ...zap.Field) {
    Logger.Info(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
    Logger.Error(msg, fields...)
}
```

---

## 📊 迁移进度跟踪

### 模块迁移状态表

| 模块 | 优先级 | 状态 | 负责人 | 预计完成 | 实际完成 |
|------|--------|------|--------|----------|----------|
| Auth | P0 | 待开始 | - | Day 2 | - |
| Profile | P0 | 待开始 | - | Day 2 | - |
| Admin | P1 | 待开始 | - | Day 3 | - |
| Game | P1 | 待开始 | - | Day 4 | - |
| Player | P1 | 待开始 | - | Day 4 | - |
| Function | P1 | 待开始 | - | Day 5 | - |
| Analytics | P2 | 待开始 | - | Day 7 | - |
| Workspace | P2 | 待开始 | - | Day 8 | - |
| ... | ... | ... | ... | ... | ... |

---

## ⚠️ 风险与应对

### 风险 1：迁移过程中业务中断
**应对**：
- 并行运行两个服务
- 使用 Nginx 进行流量切换
- 保留回滚能力

### 风险 2：API 不兼容导致前端报错
**应对**：
- 严格保持 API 路径和响应格式一致
- 编写自动化测试脚本验证兼容性
- 前端使用环境变量切换 API 地址

### 风险 3：性能下降
**应对**：
- 迁移前后进行性能对比测试
- 使用 pprof 分析性能瓶颈
- 优化数据库查询和缓存策略

### 风险 4：权限控制失效
**应对**：
- 优先迁移认证和权限中间件
- 编写权限测试用例
- 人工验证关键操作的权限控制

---

## 📚 参考资源

### Gin 最佳实践
- [Gin 官方文档](https://gin-gonic.com/docs/)
- [Gin 项目结构最佳实践](https://github.com/golang-standards/project-layout)
- [Gin + GORM 示例项目](https://github.com/go-admin-team/go-admin)

### 工具推荐
- **Wire**：依赖注入
- **Viper**：配置管理
- **Zap**：高性能日志
- **Validator**：参数验证
- **Swag**：API 文档生成

---

## 🎯 成功标准

迁移完成后应达到以下标准：

1. **功能完整性**：所有 API 功能正常工作
2. **性能指标**：响应时间 < 100ms (P95)，吞吐量 > 1000 QPS
3. **稳定性**：错误率 < 0.1%，无内存泄漏
4. **可维护性**：代码结构清晰，易于扩展
5. **文档完善**：API 文档、架构文档、部署文档齐全

---

## 📅 时间线（总计 15-20 天）

```
Week 1: 基础设施 + 核心模块
├── Day 1-2:  搭建 Gin 框架，实现中间件
├── Day 3-4:  迁移 Auth + Profile 模块
└── Day 5-7:  迁移 Admin + Game + Player 模块

Week 2: 批量迁移 + 测试
├── Day 8-10:  迁移 Function + Analytics 模块
├── Day 11-12: 迁移 Workspace + 其他模块
└── Day 13-14: 集成测试 + 性能测试

Week 3: 灰度发布 + 监控
├── Day 15-16: 灰度发布 10% → 50%
├── Day 17-18: 全量切换 + 监控
└── Day 19-20: 优化 + 文档整理
```

---

## 🚀 下一步行动

### 立即开始（今天）
1. **创建新分支**：`git checkout -b feature/migrate-to-gin`
2. **搭建基础框架**：创建 `cmd/server-gin/main.go`
3. **实现核心中间件**：认证、日志、错误处理
4. **迁移 Auth 模块**：登录/登出功能

### 本周目标
- [ ] 完成基础框架搭建
- [ ] 迁移 Auth + Profile 模块
- [ ] 编写迁移文档和测试用例
- [ ] 并行运行两个服务进行测试

---

## 💡 建议

1. **不要一次性全部迁移**：分模块逐步迁移，降低风险
2. **保持 API 兼容**：前端无需修改，降低协调成本
3. **编写测试用例**：确保迁移后功能正确
4. **监控关键指标**：及时发现问题
5. **保留回滚能力**：至少保留 go-zero 服务 1 周

---

**最后更新**：2026-03-13
**文档版本**：v1.0
**维护者**：Claude Opus 4.6
