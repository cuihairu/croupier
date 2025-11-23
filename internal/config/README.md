# 配置管理系统

Croupier配置管理系统是一个企业级的、高度可扩展的配置管理解决方案，支持多源配置、动态加载、热重载和全面验证。

## 🚀 核心特性

- **多源配置支持**：文件、环境变量、远程配置中心
- **智能配置合并**：深度合并多个配置源
- **热重载机制**：运行时动态更新配置
- **全面验证系统**：规则化配置验证
- **环境变量管理**：自动类型转换和敏感信息保护
- **多格式支持**：YAML、JSON、环境变量
- **配置监听**：配置变更回调通知
- **安全设计**：敏感信息掩码和加密支持

## 📁 文件结构

```
internal/config/
├── types.go          # 配置结构定义
├── loader.go         # 配置加载器
├── manager.go        # 配置管理器
├── validator.go      # 配置验证器
├── env.go           # 环境变量管理
├── example.go       # 使用示例
├── types_test.go     # 配置类型测试
├── loader_test.go    # 加载器测试
├── manager_test.go   # 管理器测试
├── validator_test.go # 验证器测试
├── env_test.go       # 环境变量测试
└── README.md         # 本文档
```

## 🔧 快速开始

### 1. 基本使用

```go
package main

import (
    "context"
    "log"

    "github.com/cuihairu/croupier/internal/config"
)

func main() {
    ctx := context.Background()

    // 创建配置管理器
    manager, err := config.NewManager(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer manager.Close()

    // 从文件加载配置
    err = manager.LoadFromFile("configs/app.yaml")
    if err != nil {
        log.Fatal(err)
    }

    // 获取配置
    appConfig := manager.GetAppConfig()
    log.Printf("应用名称: %s", appConfig.Name)
}
```

### 2. 多源配置

```go
// 从多个源加载配置
sources := []*config.ConfigSource{
    // 基础配置文件（必需）
    config.NewConfigSource("file", "/etc/croupier/base.yaml", true),
    // 环境变量覆盖
    config.NewEnvConfigSource("CROUPIER_", false),
    // 远程配置中心
    config.NewRemoteConfigSource(
        "https://config.example.com/api/v1/config",
        map[string]string{"Authorization": "Bearer token"},
        false,
    ),
}

err := manager.LoadFromMultiple(sources)
```

### 3. 环境变量配置

```go
// 创建环境变量管理器
envManager := config.NewEnvManager("CROUPIER_")

// 添加自定义转换器
envManager.AddTransformer(&config.URLTransformer{})

// 配置结构体
type AppConfig struct {
    Name        string        `env:"NAME"`
    Port        int           `env:"PORT"`
    DatabaseURL string        `env:"DATABASE_URL"`
    Timeout     time.Duration `env:"TIMEOUT"`
    Debug       bool          `env:"DEBUG"`
    Features    []string      `env:"FEATURES"`
}

var config AppConfig
err := envManager.LoadFromEnv(&config)
```

## 📋 配置结构

### 主配置结构

```go
type Config struct {
    App         AppConfig        `yaml:"app" json:"app"`
    Network     NetworkConfig    `yaml:"network" json:"network"`
    Database    DatabaseConfig   `yaml:"database" json:"database"`
    Security    SecurityConfig    `yaml:"security" json:"security"`
    Observability ObservabilityConfig `yaml:"observability" json:"observability"`
    Business    BusinessConfig    `yaml:"business" json:"business"`
    Storage     StorageConfig     `yaml:"storage" json:"storage"`
}
```

### 应用配置

```yaml
app:
  name: "croupier"           # 应用名称
  version: "1.0.0"          # 应用版本
  env: "development"        # 运行环境: development, testing, staging, production
  debug: false              # 调试模式
```

### 网络配置

```yaml
network:
  server:
    host: "localhost"       # 服务器地址
    http_port: 8080        # HTTP端口
    grpc_port: 9090        # gRPC端口
    tls:                   # TLS配置
      enabled: false
      cert_file: ""
      key_file: ""
    cors:                   # CORS配置
      enabled: true
      allowed_origins: ["*"]
      allowed_methods: ["GET", "POST", "PUT", "DELETE"]
    rate_limit:             # 限流配置
      enabled: true
      requests: 1000
      window: "1m"
```

### 数据库配置

```yaml
database:
  enabled: true
  primary:
    host: "localhost"
    port: 5432
    database: "croupier"
    username: "croupier"
    password: "password"
    ssl_mode: "disable"
  connection_pool:
    max_open_conns: 25
    max_idle_conns: 5
    conn_max_lifetime: "5m"
  migration:
    enabled: true
    path: "./migrations"
```

### 安全配置

```yaml
security:
  jwt:
    enabled: true
    secret: "your-very-long-secret-key"
    expiry: "1h"
    refresh_expiry: "24h"
  password_policy:
    min_length: 8
    require_uppercase: true
    require_lowercase: true
    require_numbers: true
    require_symbols: true
  audit:
    enabled: true
    retention: "90d"
```

## 🔐 安全特性

### 敏感信息保护

```go
// 环境变量敏感信息自动掩码
envManager := config.NewEnvManager("CROUPIER_")
envInfo := envManager.GetEnvInfo()

// 密码、密钥等敏感信息会被自动掩码
// CROUPIER_DB_PASSWORD = da****rd
// CROUPIER_JWT_SECRET = ve****ey
```

### JWT配置验证

```go
// JWT密钥长度自动验证
validator := config.NewDefaultValidator()
err := validator.Validate(configStruct)
// 如果密钥长度少于32字符，会返回验证错误
```

## 🔄 热重载

### 配置监听

```go
// 监听配置变更
manager.WatchConfig(ctx, func(config *config.Config, err error) {
    if err != nil {
        log.Printf("配置监听错误: %v", err)
        return
    }
    log.Printf("配置已更新，新端口: %d", config.Network.Server.HTTPPort)
})
```

### 重启信号

```go
// 监听配置重启信号
restartChan := manager.RestartChan()
go func() {
    for range restartChan {
        log.Println("收到重启信号，准备重启服务...")
        // 实现服务重启逻辑
    }
}()
```

## ✅ 验证系统

### 内置验证规则

1. **应用验证**：名称、版本、环境验证
2. **网络验证**：端口范围、TLS配置、CORS设置
3. **数据库验证**：连接参数、连接池配置
4. **安全验证**：JWT密钥长度、密码策略
5. **可观测性验证**：日志级别、指标配置
6. **业务验证**：游戏配置、函数配置
7. **存储验证**：存储提供商配置

### 自定义验证规则

```go
// 添加自定义验证规则
validator := config.NewDefaultValidator()
validator.AddRule(config.NewCustomValidationRule(
    "CustomPortValidation",
    "验证端口范围",
    func(config *config.Config) error {
        if config.Network.Server.HTTPPort == config.Network.Server.GRPCPort {
            return fmt.Errorf("HTTP和gRPC端口不能相同")
        }
        return nil
    },
))
```

## 🌍 环境变量

### 支持的类型转换

- **字符串**：直接转换
- **整数**：`strconv.Atoi`
- **浮点数**：`strconv.ParseFloat`
- **布尔值**：支持多种格式（true/false, 1/0, yes/no, on/off）
- **时间段**：`time.ParseDuration`
- **切片**：逗号、分号、空格分隔
- **映射**：key=value格式

### 转换器

```go
// URL转换器 - 自动添加http://前缀
envManager.AddTransformer(&config.URLTransformer{})

// 小写转换器 - 邮箱、用户名自动转小写
envManager.AddTransformer(&config.LowerCaseTransformer{})

// 去空格转换器 - 自动去除前后空格
envManager.AddTransformer(&config.TrimSpaceTransformer{})
```

## 🚨 错误处理

### 集成错误系统

```go
manager, err := config.NewManager(ctx,
    config.WithErrorFactory(errors.NewErrorFactory("config-manager")),
)
```

### 常见错误

- **配置文件不存在**：检查文件路径
- **格式错误**：验证YAML/JSON语法
- **验证失败**：检查配置值是否符合要求
- **权限错误**：检查文件读取权限
- **网络错误**：检查远程配置连接

## 📊 性能优化

### 并发安全

```go
// 所有配置操作都是并发安全的
go func() {
    config := manager.GetConfig()  // 安全读取
}()

go func() {
    manager.UpdateConfig(func(c *config.Config) error {
        c.Network.Server.HTTPPort = 8081
        return nil
    })  // 安全更新
}()
```

### 缓存机制

- 配置读取使用读写锁
- 配置深拷贝避免并发修改
- 配置源信息缓存

## 🔧 最佳实践

### 1. 配置分层

```
base.yaml (基础配置)
├── dev.yaml (开发环境覆盖)
├── testing.yaml (测试环境覆盖)
├── staging.yaml (预发布环境覆盖)
└── prod.yaml (生产环境覆盖)
```

### 2. 环境变量命名

```bash
# 使用统一的命名空间
CROUPIER_APP_NAME=my-app
CROUPIER_NETWORK_SERVER_HTTP_PORT=8080
CROUPIER_DATABASE_PRIMARY_HOST=localhost
```

### 3. 敏感信息管理

```yaml
# 不要在配置文件中直接存储敏感信息
security:
  jwt:
    secret: ${JWT_SECRET}  # 从环境变量读取
database:
  primary:
    password: ${DB_PASSWORD}  # 从环境变量读取
```

### 4. 配置验证

```go
// 总是验证配置
validator := config.NewDefaultValidator()
if err := validator.Validate(config); err != nil {
    log.Fatal("配置验证失败:", err)
}
```

## 🧪 测试

### 运行测试

```bash
# 运行所有测试
go test ./internal/config/... -v

# 运行特定测试
go test ./internal/config/manager_test.go -v

# 运行基准测试
go test ./internal/config/... -bench=. -v
```

### 测试覆盖率

```bash
# 生成测试覆盖率报告
go test ./internal/config/... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## 📝 示例

完整的使用示例请参考 `example.go` 文件，包含：

- 基本配置加载
- 多源配置合并
- 环境变量管理
- 自定义验证
- 高级用法

## 🤝 贡献

欢迎提交Issue和Pull Request来改进配置管理系统。

## 📄 许可证

本项目采用MIT许可证。