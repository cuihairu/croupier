package config

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"time"
)

// ConfigValidator 配置验证器接口
type ConfigValidator interface {
	Validate(config *Config) error
}

// DefaultValidator 默认配置验证器
type DefaultValidator struct {
	rules []ValidationRule
}

// ValidationRule 验证规则接口
type ValidationRule interface {
	Validate(config *Config) error
	Name() string
	Description() string
}

// NewDefaultValidator 创建默认验证器
func NewDefaultValidator() *DefaultValidator {
	v := &DefaultValidator{}

	// 添加默认验证规则
	v.rules = []ValidationRule{
		&AppValidationRule{},
		&NetworkValidationRule{},
		&DatabaseValidationRule{},
		&SecurityValidationRule{},
		&ObservabilityValidationRule{},
		&BusinessValidationRule{},
		&StorageValidationRule{},
	}

	return v
}

// Validate 验证配置
func (v *DefaultValidator) Validate(config *Config) error {
	for _, rule := range v.rules {
		if err := rule.Validate(config); err != nil {
			return fmt.Errorf("验证规则 '%s' 失败: %w", rule.Name(), err)
		}
	}
	return nil
}

// AddRule 添加验证规则
func (v *DefaultValidator) AddRule(rule ValidationRule) {
	v.rules = append(v.rules, rule)
}

// RemoveRule 移除验证规则
func (v *DefaultValidator) RemoveRule(ruleName string) {
	for i, rule := range v.rules {
		if rule.Name() == ruleName {
			v.rules = append(v.rules[:i], v.rules[i+1:]...)
			break
		}
	}
}

// AppValidationRule 应用配置验证规则
type AppValidationRule struct{}

func (r *AppValidationRule) Name() string {
	return "AppValidation"
}

func (r *AppValidationRule) Description() string {
	return "验证应用程序基本配置"
}

func (r *AppValidationRule) Validate(config *Config) error {
	app := config.App

	// 验证应用名称
	if app.Name == "" {
		return fmt.Errorf("应用名称不能为空")
	}

	if len(app.Name) > 100 {
		return fmt.Errorf("应用名称长度不能超过100个字符")
	}

	// 验证应用名称格式（只允许字母、数字、连字符和下划线）
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(app.Name) {
		return fmt.Errorf("应用名称只能包含字母、数字、连字符和下划线")
	}

	// 验证版本
	if app.Version == "" {
		return fmt.Errorf("应用版本不能为空")
	}

	// 验证版本格式（语义化版本）
	if !regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9-]+)?(\+[a-zA-Z0-9-]+)?$`).MatchString(app.Version) {
		return fmt.Errorf("应用版本必须符合语义化版本格式，如: 1.0.0")
	}

	// 验证环境
	if app.Env == "" {
		return fmt.Errorf("运行环境不能为空")
	}

	validEnvs := []string{"development", "testing", "staging", "production"}
	envValid := false
	for _, env := range validEnvs {
		if app.Env == env {
			envValid = true
			break
		}
	}
	if !envValid {
		return fmt.Errorf("无效的运行环境: %s，支持的值: %v", app.Env, validEnvs)
	}

	// 验证调试模式
	if app.Debug && app.Env == "production" {
		return fmt.Errorf("生产环境不能启用调试模式")
	}

	return nil
}

// NetworkValidationRule 网络配置验证规则
type NetworkValidationRule struct{}

func (r *NetworkValidationRule) Name() string {
	return "NetworkValidation"
}

func (r *NetworkValidationRule) Description() string {
	return "验证网络相关配置"
}

func (r *NetworkValidationRule) Validate(config *Config) error {
	network := config.Network

	// 验证服务器配置
	server := network.Server

	// 验证HTTP端口
	if server.HTTPPort <= 0 || server.HTTPPort > 65535 {
		return fmt.Errorf("HTTP端口必须在1-65535范围内，当前值: %d", server.HTTPPort)
	}

	// 验证gRPC端口
	if server.GRPCPort <= 0 || server.GRPCPort > 65535 {
		return fmt.Errorf("gRPC端口必须在1-65535范围内，当前值: %d", server.GRPCPort)
	}

	// 验证端口不重复
	if server.HTTPPort == server.GRPCPort {
		return fmt.Errorf("HTTP端口和gRPC端口不能相同")
	}

	// 验证主机地址
	if server.Host != "" {
		if net.ParseIP(server.Host) == nil && server.Host != "localhost" && server.Host != "0.0.0.0" {
			return fmt.Errorf("无效的主机地址: %s", server.Host)
		}
	}

	// 验证TLS配置
	if server.TLS.Enabled {
		if server.TLS.CertFile == "" {
			return fmt.Errorf("启用TLS时必须指定证书文件路径")
		}
		if server.TLS.KeyFile == "" {
			return fmt.Errorf("启用TLS时必须指定私钥文件路径")
		}
	}

	// 验证CORS配置
	if server.CORS.Enabled {
		// 验证允许的源
		for _, origin := range server.CORS.AllowedOrigins {
			if origin != "*" {
				parsed, err := url.Parse(origin)
				if err != nil || parsed.Scheme == "" || parsed.Host == "" {
					return fmt.Errorf("无效的CORS源: %s", origin)
				}
			}
		}

		// 验证允许的方法
		validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
		for _, method := range server.CORS.AllowedMethods {
			methodValid := false
			for _, validMethod := range validMethods {
				if method == validMethod {
					methodValid = true
					break
				}
			}
			if !methodValid {
				return fmt.Errorf("无效的CORS方法: %s，支持的方法: %v", method, validMethods)
			}
		}
	}

	// 验证限流配置
	if server.RateLimit.Enabled {
		if server.RateLimit.Requests <= 0 {
			return fmt.Errorf("启用限流时请求数必须大于0")
		}
		if server.RateLimit.Window <= 0 {
			return fmt.Errorf("启用限流时时间窗口必须大于0")
		}
	}

	return nil
}

// DatabaseValidationRule 数据库配置验证规则
type DatabaseValidationRule struct{}

func (r *DatabaseValidationRule) Name() string {
	return "DatabaseValidation"
}

func (r *DatabaseValidationRule) Description() string {
	return "验证数据库相关配置"
}

func (r *DatabaseValidationRule) Validate(config *Config) error {
	db := config.Database

	if !db.Enabled {
		return nil
	}

	// 验证主数据库配置
	primary := db.Primary
	if err := r.validateDatabaseConfig(&primary, "Primary"); err != nil {
		return err
	}

	// 验证只读数据库配置（如果启用）
	if db.ReadOnly.Enabled {
		for i, replica := range db.ReadOnly.Replicas {
			if err := r.validateDatabaseConfig(&replica, fmt.Sprintf("ReadOnly[%d]", i)); err != nil {
				return err
			}
		}
	}

	// 验证连接池配置
	pool := db.ConnectionPool
	if pool.MaxOpenConns <= 0 {
		return fmt.Errorf("最大连接数必须大于0")
	}
	if pool.MaxIdleConns <= 0 {
		return fmt.Errorf("最大空闲连接数必须大于0")
	}
	if pool.MaxIdleConns > pool.MaxOpenConns {
		return fmt.Errorf("最大空闲连接数不能超过最大连接数")
	}
	if pool.ConnMaxLifetime <= 0 {
		return fmt.Errorf("连接最大生存时间必须大于0")
	}

	// 验证迁移配置
	if db.Migration.Enabled {
		if db.Migration.Path == "" {
			return fmt.Errorf("启用迁移时必须指定迁移文件路径")
		}
	}

	return nil
}

func (r *DatabaseValidationRule) validateDatabaseConfig(config *DatabaseInstance, prefix string) error {
	// 验证主机
	if config.Host == "" {
		return fmt.Errorf("%s数据库主机不能为空", prefix)
	}

	// 验证端口
	if config.Port <= 0 || config.Port > 65535 {
		return fmt.Errorf("%s数据库端口必须在1-65535范围内，当前值: %d", prefix, config.Port)
	}

	// 验证数据库名
	if config.Database == "" {
		return fmt.Errorf("%s数据库名不能为空", prefix)
	}

	// 验证用户名
	if config.Username == "" {
		return fmt.Errorf("%s数据库用户名不能为空", prefix)
	}

	// 验证SSL配置
	if config.SSLMode != "" {
		validSSLModes := []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}
		sslModeValid := false
		for _, mode := range validSSLModes {
			if config.SSLMode == mode {
				sslModeValid = true
				break
			}
		}
		if !sslModeValid {
			return fmt.Errorf("%s无效的SSL模式: %s，支持的值: %v", prefix, config.SSLMode, validSSLModes)
		}
	}

	return nil
}

// SecurityValidationRule 安全配置验证规则
type SecurityValidationRule struct{}

func (r *SecurityValidationRule) Name() string {
	return "SecurityValidation"
}

func (r *SecurityValidationRule) Description() string {
	return "验证安全相关配置"
}

func (r *SecurityValidationRule) Validate(config *Config) error {
	security := config.Security

	// 验证JWT配置
	if security.JWT.Enabled {
		if security.JWT.Secret == "" {
			return fmt.Errorf("启用JWT时必须提供密钥")
		}
		if len(security.JWT.Secret) < 32 {
			return fmt.Errorf("JWT密钥长度不能少于32个字符")
		}
		if security.JWT.Expiry <= 0 {
			return fmt.Errorf("JWT过期时间必须大于0")
		}
		if security.JWT.RefreshExpiry <= 0 {
			return fmt.Errorf("JWT刷新过期时间必须大于0")
		}
		if security.JWT.RefreshExpiry <= security.JWT.Expiry {
			return fmt.Errorf("JWT刷新过期时间必须大于访问令牌过期时间")
		}
	}

	// 验证TOTP配置
	if security.TOTP.Enabled {
		if security.TOTP.Issuer == "" {
			return fmt.Errorf("启用TOTP时必须提供发行者名称")
		}
	}

	// 验证密码策略
	if security.PasswordPolicy.MinLength <= 0 {
		return fmt.Errorf("密码最小长度必须大于0")
	}
	if security.PasswordPolicy.MinLength > 128 {
		return fmt.Errorf("密码最小长度不能超过128")
	}
	if security.PasswordPolicy.RequireUppercase && security.PasswordPolicy.MinLength < 8 {
		return fmt.Errorf("要求大写字母时密码最小长度不能少于8")
	}

	// 验证API密钥配置
	if security.APIKeys.Enabled {
		if security.APIKeys.HeaderName == "" {
			return fmt.Errorf("启用API密钥时必须指定请求头名称")
		}
	}

	// 验证审计配置
	if security.Audit.Enabled {
		if security.Audit.Retention <= 0 {
			return fmt.Errorf("启用审计时保留时间必须大于0")
		}
	}

	return nil
}

// ObservabilityValidationRule 可观测性配置验证规则
type ObservabilityValidationRule struct{}

func (r *ObservabilityValidationRule) Name() string {
	return "ObservabilityValidation"
}

func (r *ObservabilityValidationRule) Description() string {
	return "验证可观测性相关配置"
}

func (r *ObservabilityValidationRule) Validate(config *Config) error {
	obs := config.Observability

	// 验证日志配置
	if obs.Logging.Enabled {
		if obs.Logging.Level == "" {
			return fmt.Errorf("启用日志时必须指定日志级别")
		}
		validLevels := []string{"debug", "info", "warn", "error", "fatal", "panic"}
		levelValid := false
		for _, level := range validLevels {
			if obs.Logging.Level == level {
				levelValid = true
				break
			}
		}
		if !levelValid {
			return fmt.Errorf("无效的日志级别: %s，支持的级别: %v", obs.Logging.Level, validLevels)
		}

		if obs.Logging.Format != "" {
			validFormats := []string{"json", "text"}
			formatValid := false
			for _, format := range validFormats {
				if obs.Logging.Format == format {
					formatValid = true
					break
				}
			}
			if !formatValid {
				return fmt.Errorf("无效的日志格式: %s，支持的格式: %v", obs.Logging.Format, validFormats)
			}
		}
	}

	// 验证指标配置
	if obs.Metrics.Enabled {
		if obs.Metrics.Port <= 0 || obs.Metrics.Port > 65535 {
			return fmt.Errorf("指标端口必须在1-65535范围内，当前值: %d", obs.Metrics.Port)
		}
		if obs.Metrics.Path == "" {
			return fmt.Errorf("启用指标时必须指定指标路径")
		}
	}

	// 验证追踪配置
	if obs.Tracing.Enabled {
		if obs.Tracing.Jaeger.Enabled {
			if obs.Tracing.Jaeger.Endpoint == "" {
				return fmt.Errorf("启用Jaeger追踪时必须指定端点")
			}
			if _, err := url.Parse(obs.Tracing.Jaeger.Endpoint); err != nil {
				return fmt.Errorf("无效的Jaeger端点URL: %s", obs.Tracing.Jaeger.Endpoint)
			}
		}

		if obs.Tracing.Zipkin.Enabled {
			if obs.Tracing.Zipkin.URL == "" {
				return fmt.Errorf("启用Zipkin追踪时必须指定URL")
			}
			if _, err := url.Parse(obs.Tracing.Zipkin.URL); err != nil {
				return fmt.Errorf("无效的Zipkin URL: %s", obs.Tracing.Zipkin.URL)
			}
		}
	}

	// 验证健康检查配置
	if obs.HealthCheck.Enabled {
		if obs.HealthCheck.Port <= 0 || obs.HealthCheck.Port > 65535 {
			return fmt.Errorf("健康检查端口必须在1-65535范围内，当前值: %d", obs.HealthCheck.Port)
		}
	}

	return nil
}

// BusinessValidationRule 业务配置验证规则
type BusinessValidationRule struct{}

func (r *BusinessValidationRule) Name() string {
	return "BusinessValidation"
}

func (r *BusinessValidationRule) Description() string {
	return "验证业务逻辑相关配置"
}

func (r *BusinessValidationRule) Validate(config *Config) error {
	business := config.Business

	// 验证游戏配置
	if business.Games.MaxConcurrentGames <= 0 {
		return fmt.Errorf("最大并发游戏数必须大于0")
	}
	if business.Games.MaxPlayersPerGame <= 0 {
		return fmt.Errorf("每游戏最大玩家数必须大于0")
	}
	if business.Games.DefaultGameTimeout <= 0 {
		return fmt.Errorf("默认游戏超时时间必须大于0")
	}

	// 验证函数配置
	if business.Functions.Registry.MaxSize <= 0 {
		return fmt.Errorf("函数注册表最大大小必须大于0")
	}
	if business.Functions.Execution.DefaultTimeout <= 0 {
		return fmt.Errorf("默认执行超时时间必须大于0")
	}
	if business.Functions.Execution.MaxTimeout <= 0 {
		return fmt.Errorf("最大执行超时时间必须大于0")
	}
	if business.Functions.Execution.MaxTimeout < business.Functions.Execution.DefaultTimeout {
		return fmt.Errorf("最大执行超时时间不能小于默认执行超时时间")
	}

	// 验证任务配置
	if business.Jobs.Queue.MaxSize <= 0 {
		return fmt.Errorf("任务队列最大大小必须大于0")
	}
	if business.Jobs.Retry.MaxAttempts <= 0 {
		return fmt.Errorf("最大重试次数必须大于0")
	}
	if business.Jobs.Retry.InitialDelay <= 0 {
		return fmt.Errorf("初始重试延迟必须大于0")
	}

	return nil
}

// StorageValidationRule 存储配置验证规则
type StorageValidationRule struct{}

func (r *StorageValidationRule) Name() string {
	return "StorageValidation"
}

func (r *StorageValidationRule) Description() string {
	return "验证存储相关配置"
}

func (r *StorageValidationRule) Validate(config *Config) error {
	storage := config.Storage

	// 验证文件存储配置
	if storage.Files.Enabled {
		if storage.Files.BasePath == "" {
			return fmt.Errorf("启用文件存储时必须指定基础路径")
		}
	}

	// 验证对象存储配置
	if storage.Objects.Enabled {
		provider := storage.Objects.Provider
		switch provider {
		case "s3":
			if storage.Objects.S3.Bucket == "" {
				return fmt.Errorf("S3存储必须指定桶名")
			}
			if storage.Objects.S3.Region == "" {
				return fmt.Errorf("S3存储必须指定区域")
			}
			if storage.Objects.S3.AccessKey == "" || storage.Objects.S3.SecretKey == "" {
				return fmt.Errorf("S3存储必须指定访问密钥")
			}
		case "minio":
			if storage.Objects.Minio.Endpoint == "" {
				return fmt.Errorf("MinIO存储必须指定端点")
			}
			if storage.Objects.Minio.Bucket == "" {
				return fmt.Errorf("MinIO存储必须指定桶名")
			}
			if storage.Objects.Minio.AccessKey == "" || storage.Objects.Minio.SecretKey == "" {
				return fmt.Errorf("MinIO存储必须指定访问密钥")
			}
		case "oss":
			if storage.Objects.OSS.Endpoint == "" {
				return fmt.Errorf("OSS存储必须指定端点")
			}
			if storage.Objects.OSS.Bucket == "" {
				return fmt.Errorf("OSS存储必须指定桶名")
			}
			if storage.Objects.OSS.AccessKey == "" || storage.Objects.OSS.SecretKey == "" {
				return fmt.Errorf("OSS存储必须指定访问密钥")
			}
		default:
			return fmt.Errorf("不支持的对象存储提供者: %s", provider)
		}
	}

	return nil
}

// CustomValidationRule 自定义验证规则
type CustomValidationRule struct {
	name        string
	description string
	validateFn  func(*Config) error
}

func NewCustomValidationRule(name, description string, validateFn func(*Config) error) *CustomValidationRule {
	return &CustomValidationRule{
		name:        name,
		description: description,
		validateFn:  validateFn,
	}
}

func (r *CustomValidationRule) Name() string {
	return r.name
}

func (r *CustomValidationRule) Description() string {
	return r.description
}

func (r *CustomValidationRule) Validate(config *Config) error {
	return r.validateFn(config)
}

// 便捷验证函数

// ValidatePort 验证端口
func ValidatePort(port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("端口必须在1-65535范围内，当前值: %d", port)
	}
	return nil
}

// ValidateURL 验证URL
func ValidateURL(urlStr string) error {
	parsed, err := url.Parse(urlStr)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("无效的URL: %s", urlStr)
	}
	return nil
}

// ValidateEmail 验证邮箱地址
func ValidateEmail(email string) error {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("无效的邮箱地址: %s", email)
	}
	return nil
}

// ValidateDuration 验证持续时间
func ValidateDuration(duration time.Duration) error {
	if duration <= 0 {
		return fmt.Errorf("持续时间必须大于0，当前值: %v", duration)
	}
	return nil
}

// ValidateRange 验证数值范围
func ValidateRange(value, min, max int, name string) error {
	if value < min || value > max {
		return fmt.Errorf("%s必须在%d-%d范围内，当前值: %d", name, min, max, value)
	}
	return nil
}

// ValidateStringEnum 验证字符串枚举
func ValidateStringEnum(value string, validValues []string, name string) error {
	for _, validValue := range validValues {
		if value == validValue {
			return nil
		}
	}
	return fmt.Errorf("无效的%s: %s，支持的值: %v", name, value, validValues)
}
