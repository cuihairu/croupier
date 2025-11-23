package config

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cuihairu/croupier/internal/errors"
	ctxmanager "github.com/cuihairu/croupier/internal/context"
)

// ExampleConfigUsage 演示配置系统的使用方式
func ExampleConfigUsage() {
	ctx := context.Background()

	// 1. 创建配置管理器
	manager, err := NewManager(ctx,
		WithErrorFactory(errors.NewErrorFactory("example")),
		WithContextManager(ctxmanager.NewManager(30*time.Second)),
	)
	if err != nil {
		log.Fatal("创建配置管理器失败:", err)
	}
	defer manager.Close()

	// 2. 从多个源加载配置
	sources := []*ConfigSource{
		// 基础配置文件（必需）
		NewConfigSource(SourceTypeFile, "configs/app.yaml", true),
		// 环境特定配置文件
		NewConfigSource(SourceTypeFile, "configs/app.prod.yaml", false),
		// 环境变量覆盖
		NewEnvConfigSource("CROUPIER_", false),
		// 远程配置中心
		NewRemoteConfigSource("https://config.example.com/api/v1/config", map[string]string{
			"Authorization": "Bearer your-token",
		}, false),
	}

	err = manager.LoadFromMultiple(sources)
	if err != nil {
		log.Fatal("加载配置失败:", err)
	}

	// 3. 获取不同类型的配置
	appConfig := manager.GetAppConfig()
	networkConfig := manager.GetNetworkConfig()
	dbConfig := manager.GetDatabaseConfig()
	securityConfig := manager.GetSecurityConfig()

	// 4. 使用配置
	fmt.Printf("应用: %s v%s (环境: %s)\n", appConfig.Name, appConfig.Version, appConfig.Env)
	fmt.Printf("HTTP服务: %s:%d\n", networkConfig.Server.Host, networkConfig.Server.HTTPPort)
	fmt.Printf("gRPC服务: %s:%d\n", networkConfig.Server.Host, networkConfig.Server.GRPCPort)
	fmt.Printf("数据库: %s:%d/%s\n", dbConfig.Primary.Host, dbConfig.Primary.Port, dbConfig.Primary.Database)

	if securityConfig.JWT.Enabled {
		fmt.Printf("JWT认证已启用，过期时间: %v\n", securityConfig.JWT.Expiry)
	}

	// 5. 监听配置变更
	manager.WatchConfig(ctx, func(config *Config, err error) {
		if err != nil {
			log.Printf("配置监听错误: %v", err)
			return
		}
		log.Printf("配置已更新: 应用名称 = %s", config.App.Name)
	})

	// 6. 动态更新配置
	err = manager.UpdateConfig(func(config *Config) error {
		// 启用调试模式
		config.App.Debug = true
		// 更新端口
		config.Network.Server.HTTPPort = 8081
		return nil
	})
	if err != nil {
		log.Printf("更新配置失败: %v", err)
	}

	// 7. 导出配置
	jsonData, err := manager.Export("json")
	if err != nil {
		log.Printf("导出JSON配置失败: %v", err)
	} else {
		fmt.Printf("当前配置(JSON): %s\n", string(jsonData[:100]) + "...")
	}
}

// ExampleEnvironmentConfigUsage 演示环境变量配置的使用
func ExampleEnvironmentConfigUsage() {
	// 创建环境变量管理器
	envManager := NewEnvManager("CROUPIER_")

	// 添加自定义转换器
	envManager.AddTransformer(&URLTransformer{})
	envManager.AddTransformer(&LowerCaseTransformer{})

	// 配置结构体
	type AppConfig struct {
		Name        string        `env:"NAME"`
		Env         string        `env:"ENV"`
		Debug       bool          `env:"DEBUG"`
		Port        int           `env:"PORT"`
		Timeout     time.Duration `env:"TIMEOUT"`
		DatabaseURL string        `env:"DATABASE_URL"`
		Features    []string      `env:"FEATURES"`
	}

	var config AppConfig

	// 从环境变量加载配置
	err := envManager.LoadFromEnv(&config)
	if err != nil {
		log.Printf("从环境变量加载配置失败: %v", err)
		return
	}

	fmt.Printf("从环境变量加载的配置:\n")
	fmt.Printf("  应用名称: %s\n", config.Name)
	fmt.Printf("  环境: %s\n", config.Env)
	fmt.Printf("  调试模式: %v\n", config.Debug)
	fmt.Printf("  端口: %d\n", config.Port)
	fmt.Printf("  超时: %v\n", config.Timeout)
	fmt.Printf("  数据库URL: %s\n", config.DatabaseURL)
	fmt.Printf("  功能特性: %v\n", config.Features)

	// 获取环境变量信息（敏感信息会被掩码）
	envInfo := envManager.GetEnvInfo()
	fmt.Printf("\n环境变量信息:\n")
	for key, info := range envInfo {
		fmt.Printf("  %s = %s\n", key, info.Value)
	}

	// 导出配置到环境变量
	err = envManager.ExportToEnv(&config)
	if err != nil {
		log.Printf("导出配置到环境变量失败: %v", err)
	}
}

// ExampleCustomValidation 演示自定义配置验证
func ExampleCustomValidation() {
	// 创建验证器
	validator := NewDefaultValidator()

	// 添加自定义验证规则
	validator.AddRule(NewCustomValidationRule(
		"CustomPortValidation",
		"验证端口范围和可用性",
		func(config *Config) error {
			// HTTP和gRPC端口不能相同
			if config.Network.Server.HTTPPort == config.Network.Server.GRPCPort {
				return fmt.Errorf("HTTP端口(%d)和gRPC端口(%d)不能相同",
					config.Network.Server.HTTPPort, config.Network.Server.GRPCPort)
			}
			return nil
		},
	))

	// 添加端口范围验证
	validator.AddRule(NewCustomValidationRule(
		"PortRangeValidation",
		"验证端口在有效范围内",
		func(config *Config) error {
			if err := ValidatePort(config.Network.Server.HTTPPort); err != nil {
				return fmt.Errorf("HTTP端口无效: %w", err)
			}
			if err := ValidatePort(config.Network.Server.GRPCPort); err != nil {
				return fmt.Errorf("gRPC端口无效: %w", err)
			}
			return nil
		},
	))

	// 测试验证
	testConfigs := []struct {
		name   string
		config *Config
	}{
		{
			name: "有效配置",
			config: &Config{
				App: AppConfig{
					Name:    "test-app",
					Version: "1.0.0",
					Env:     "development",
				},
				Network: NetworkConfig{
					Server: ServerConfig{
						HTTPPort: 8080,
						GRPCPort: 9090,
					},
				},
			},
		},
		{
			name: "端口冲突配置",
			config: &Config{
				App: AppConfig{
					Name:    "test-app",
					Version: "1.0.0",
					Env:     "development",
				},
				Network: NetworkConfig{
					Server: ServerConfig{
						HTTPPort: 8080,
						GRPCPort: 8080, // 端口冲突
					},
				},
			},
		},
	}

	for _, test := range testConfigs {
		fmt.Printf("验证配置: %s\n", test.name)
		err := validator.Validate(test.config)
		if err != nil {
			fmt.Printf("  ❌ 验证失败: %v\n", err)
		} else {
			fmt.Printf("  ✅ 验证通过\n")
		}
	}
}

// ExampleConfigurationTemplate 演示配置模板
var ExampleConfigurationTemplate = `
# Croupier 应用配置模板
app:
  name: "croupier"
  version: "1.0.0"
  env: "development"  # development, testing, staging, production
  debug: false

network:
  server:
    host: "localhost"
    http_port: 8080
    grpc_port: 9090
    tls:
      enabled: false
      cert_file: ""
      key_file: ""
    cors:
      enabled: true
      allowed_origins:
        - "*"
      allowed_methods:
        - "GET"
        - "POST"
        - "PUT"
        - "DELETE"
    rate_limit:
      enabled: true
      requests: 1000
      window: "1m"

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

security:
  jwt:
    enabled: true
    secret: "your-very-long-secret-key-here"
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

observability:
  logging:
    enabled: true
    level: "info"  # debug, info, warn, error
    format: "json"
  metrics:
    enabled: true
    port: 9090
    path: "/metrics"
  tracing:
    enabled: false
    jaeger:
      enabled: false
      endpoint: ""
  health_check:
    enabled: true
    port: 8081

business:
  games:
    max_concurrent_games: 1000
    max_players_per_game: 10
    default_game_timeout: "1h"
  functions:
    registry:
      max_size: 10000
    execution:
      default_timeout: "30s"
      max_timeout: "5m"
  jobs:
    queue:
      max_size: 5000
    retry:
      max_attempts: 3
      initial_delay: "1s"

storage:
  files:
    enabled: true
    base_path: "./data/files"
  objects:
    enabled: false
    provider: "s3"  # s3, minio, oss
    s3:
      bucket: "croupier-files"
      region: "us-east-1"
      access_key: ""
      secret_key: ""
`

// ExampleEnvironmentVariables 演示环境变量配置
var ExampleEnvironmentVariables = `
# Croupier 应用环境变量配置
# 复制到 .env 文件或直接设置

# 应用配置
CROUPIER_NAME=croupier
CROUPIER_VERSION=1.0.0
CROUPIER_ENV=production
CROUPIER_DEBUG=false

# 网络配置
CROUPIER_NETWORK_SERVER_HOST=0.0.0.0
CROUPIER_NETWORK_SERVER_HTTP_PORT=8080
CROUPIER_NETWORK_SERVER_GRPC_PORT=9090
CROUPIER_NETWORK_SERVER_TLS_ENABLED=true
CROUPIER_NETWORK_SERVER_TLS_CERT_FILE=/etc/certs/server.crt
CROUPIER_NETWORK_SERVER_TLS_KEY_FILE=/etc/certs/server.key

# 数据库配置
CROUPIER_DATABASE_ENABLED=true
CROUPIER_DATABASE_PRIMARY_HOST=db.example.com
CROUPIER_DATABASE_PRIMARY_PORT=5432
CROUPIER_DATABASE_PRIMARY_DATABASE=croupier
CROUPIER_DATABASE_PRIMARY_USERNAME=croupier
CROUPIER_DATABASE_PRIMARY_PASSWORD=your-database-password

# 安全配置
CROUPIER_SECURITY_JWT_ENABLED=true
CROUPIER_SECURITY_JWT_SECRET=your-very-long-jwt-secret-key-here
CROUPIER_SECURITY_JWT_EXPIRY=1h
CROUPIER_SECURITY_JWT_REFRESH_EXPIRY=24h

# 可观测性配置
CROUPIER_OBSERVABILITY_LOGGING_ENABLED=true
CROUPIER_OBSERVABILITY_LOGGING_LEVEL=info
CROUPIER_OBSERVABILITY_LOGGING_FORMAT=json
CROUPIER_OBSERVABILITY_METRICS_ENABLED=true
CROUPIER_OBSERVABILITY_METRICS_PORT=9090

# 业务配置
CROUPIER_BUSINESS_GAMES_MAX_CONCURRENT_GAMES=1000
CROUPIER_BUSINESS_GAMES_MAX_PLAYERS_PER_GAME=10
CROUPIER_BUSINESS_FUNCTIONS_REGISTRY_MAX_SIZE=10000
`

// ExampleAdvancedUsage 演示高级使用方式
func ExampleAdvancedUsage() {
	ctx := context.Background()

	// 创建带自定义验证器的管理器
	manager, err := NewManager(ctx,
		WithErrorFactory(errors.NewErrorFactory("advanced")),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 设置配置变更监听
	restartChan := manager.RestartChan()

	go func() {
		for range restartChan {
			log.Println("收到配置重启信号，准备重启服务...")
			// 这里可以实现服务重启逻辑
		}
	}()

	// 动态添加配置源
	remoteSource := NewRemoteConfigSource(
		"https://config-center.example.com/api/v1/apps/croupier",
		map[string]string{
			"X-API-Key": "your-api-key",
			"X-Version": "v1",
		},
		false,
	)

	// 可以在运行时添加新的配置源
	// 注意：这需要在Manager中添加AddSource方法
	sources := []*ConfigSource{
		NewConfigSource(SourceTypeFile, "/etc/croupier/base.yaml", true),
		remoteSource,
		NewEnvConfigSource("CROUPIER_", false),
	}

	err = manager.LoadFromMultiple(sources)
	if err != nil {
		log.Fatal("加载配置失败:", err)
	}

	// 配置信息统计
	configSources := manager.GetConfigSources()
	fmt.Printf("配置源数量: %d\n", len(configSources))
	for i, source := range configSources {
		fmt.Printf("  [%d] 类型: %s, 路径: %s, 加载时间: %v\n",
			i+1, source.Type, source.Path, source.Loaded)
	}

	// 定期重新加载配置（用于远程配置更新）
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := manager.Reload()
			if err != nil {
				log.Printf("重新加载配置失败: %v", err)
			} else {
				log.Println("配置重新加载成功")
			}
		case <-ctx.Done():
			log.Println("配置管理器停止")
			return
		}
	}
}