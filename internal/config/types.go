package config

import (
	"time"
)

// Config 简化的配置结构
type Config struct {
	App         AppConfig        `yaml:"app" json:"app"`
	Network     NetworkConfig    `yaml:"network" json:"network"`
	Database    DatabaseConfig   `yaml:"database" json:"database"`
	Security    SecurityConfig    `yaml:"security" json:"security"`
	Observability ObservabilityConfig `yaml:"observability" json:"observability"`
	Business    BusinessConfig    `yaml:"business" json:"business"`
	Storage     StorageConfig     `yaml:"storage" json:"storage"`
}

// AppConfig 应用配置
type AppConfig struct {
	Name    string `yaml:"name" json:"name"`
	Version string `yaml:"version" json:"version"`
	Env     string `yaml:"env" json:"env"`
	Debug   bool   `yaml:"debug" json:"debug"`
}

// NetworkConfig 网络配置
type NetworkConfig struct {
	Server ServerConfig `yaml:"server" json:"server"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host     string      `yaml:"host" json:"host"`
	HTTPPort int         `yaml:"http_port" json:"http_port"`
	GRPCPort int         `yaml:"grpc_port" json:"grpc_port"`
	TLS      TLSConfig   `yaml:"tls" json:"tls"`
	CORS     CORSConfig  `yaml:"cors" json:"cors"`
	RateLimit RateLimitConfig `yaml:"rate_limit" json:"rate_limit"`
}

// TLSConfig TLS配置
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	CertFile string `yaml:"cert_file" json:"cert_file"`
	KeyFile  string `yaml:"key_file" json:"key_file"`
}

// CORSConfig CORS配置
type CORSConfig struct {
	Enabled        bool     `yaml:"enabled" json:"enabled"`
	AllowedOrigins []string `yaml:"allowed_origins" json:"allowed_origins"`
	AllowedMethods []string `yaml:"allowed_methods" json:"allowed_methods"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled  bool          `yaml:"enabled" json:"enabled"`
	Requests int           `yaml:"requests" json:"requests"`
	Window   time.Duration `yaml:"window" json:"window"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Enabled       bool                `yaml:"enabled" json:"enabled"`
	Primary       DatabaseInstance    `yaml:"primary" json:"primary"`
	ReadOnly      ReadOnlyConfig      `yaml:"read_only" json:"read_only"`
	ConnectionPool ConnectionPoolConfig `yaml:"connection_pool" json:"connection_pool"`
	Migration     MigrationConfig     `yaml:"migration" json:"migration"`
}

// DatabaseInstance 数据库实例配置
type DatabaseInstance struct {
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	Database string `yaml:"database" json:"database"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
	SSLMode  string `yaml:"ssl_mode" json:"ssl_mode"`
}

// ReadOnlyConfig 只读配置
type ReadOnlyConfig struct {
	Enabled   bool                `yaml:"enabled" json:"enabled"`
	Replicas  []DatabaseInstance  `yaml:"replicas" json:"replicas"`
	LoadBalance string            `yaml:"load_balance" json:"load_balance"`
}

// ConnectionPoolConfig 连接池配置
type ConnectionPoolConfig struct {
	MaxOpenConns    int           `yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
}

// MigrationConfig 迁移配置
type MigrationConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Path    string `yaml:"path" json:"path"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	JWT           JWTConfig     `yaml:"jwt" json:"jwt"`
	TOTP          TOTPConfig    `yaml:"totp" json:"totp"`
	PasswordPolicy PasswordPolicyConfig `yaml:"password_policy" json:"password_policy"`
	APIKeys       APIKeysConfig `yaml:"api_keys" json:"api_keys"`
	Audit         AuditConfig   `yaml:"audit" json:"audit"`
}

// JWTConfig JWT配置
type JWTConfig struct {
	Enabled        bool          `yaml:"enabled" json:"enabled"`
	Secret         string        `yaml:"secret" json:"secret"`
	Expiry         time.Duration `yaml:"expiry" json:"expiry"`
	RefreshExpiry  time.Duration `yaml:"refresh_expiry" json:"refresh_expiry"`
}

// TOTPConfig TOTP配置
type TOTPConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Issuer  string `yaml:"issuer" json:"issuer"`
}

// PasswordPolicyConfig 密码策略配置
type PasswordPolicyConfig struct {
	MinLength         int      `yaml:"min_length" json:"min_length"`
	RequireUppercase  bool     `yaml:"require_uppercase" json:"require_uppercase"`
	RequireLowercase  bool     `yaml:"require_lowercase" json:"require_lowercase"`
	RequireNumbers    bool     `yaml:"require_numbers" json:"require_numbers"`
	RequireSymbols    bool     `yaml:"require_symbols" json:"require_symbols"`
	ForbiddenPatterns []string `yaml:"forbidden_patterns" json:"forbidden_patterns"`
}

// APIKeysConfig API密钥配置
type APIKeysConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	HeaderName string `yaml:"header_name" json:"header_name"`
}

// AuditConfig 审计配置
type AuditConfig struct {
	Enabled  bool          `yaml:"enabled" json:"enabled"`
	Retention time.Duration `yaml:"retention" json:"retention"`
}

// ObservabilityConfig 可观测性配置
type ObservabilityConfig struct {
	Logging    LoggingConfig    `yaml:"logging" json:"logging"`
	Metrics    MetricsConfig    `yaml:"metrics" json:"metrics"`
	Tracing    TracingConfig    `yaml:"tracing" json:"tracing"`
	HealthCheck HealthCheckConfig `yaml:"health_check" json:"health_check"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Level   string `yaml:"level" json:"level"`
	Format  string `yaml:"format" json:"format"`
}

// MetricsConfig 指标配置
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Port    int    `yaml:"port" json:"port"`
	Path    string `yaml:"path" json:"path"`
}

// TracingConfig 追踪配置
type TracingConfig struct {
	Enabled bool      `yaml:"enabled" json:"enabled"`
	Jaeger  JaegerConfig `yaml:"jaeger" json:"jaeger"`
	Zipkin  ZipkinConfig `yaml:"zipkin" json:"zipkin"`
}

// JaegerConfig Jaeger配置
type JaegerConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Endpoint string `yaml:"endpoint" json:"endpoint"`
}

// ZipkinConfig Zipkin配置
type ZipkinConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	URL     string `yaml:"url" json:"url"`
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	Port    int  `yaml:"port" json:"port"`
}

// BusinessConfig 业务配置
type BusinessConfig struct {
	Games     GamesConfig     `yaml:"games" json:"games"`
	Functions FunctionsConfig `yaml:"functions" json:"functions"`
	Jobs      JobsConfig      `yaml:"jobs" json:"jobs"`
}

// GamesConfig 游戏配置
type GamesConfig struct {
	MaxConcurrentGames int           `yaml:"max_concurrent_games" json:"max_concurrent_games"`
	MaxPlayersPerGame  int           `yaml:"max_players_per_game" json:"max_players_per_game"`
	DefaultGameTimeout time.Duration `yaml:"default_game_timeout" json:"default_game_timeout"`
}

// FunctionsConfig 函数配置
type FunctionsConfig struct {
	Registry   RegistryConfig   `yaml:"registry" json:"registry"`
	Execution  ExecutionConfig  `yaml:"execution" json:"execution"`
}

// RegistryConfig 注册表配置
type RegistryConfig struct {
	MaxSize int `yaml:"max_size" json:"max_size"`
}

// ExecutionConfig 执行配置
type ExecutionConfig struct {
	DefaultTimeout time.Duration `yaml:"default_timeout" json:"default_timeout"`
	MaxTimeout     time.Duration `yaml:"max_timeout" json:"max_timeout"`
}

// JobsConfig 任务配置
type JobsConfig struct {
	Queue QueueConfig `yaml:"queue" json:"queue"`
	Retry RetryConfig `yaml:"retry" json:"retry"`
}

// QueueConfig 队列配置
type QueueConfig struct {
	MaxSize int `yaml:"max_size" json:"max_size"`
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxAttempts  int           `yaml:"max_attempts" json:"max_attempts"`
	InitialDelay time.Duration `yaml:"initial_delay" json:"initial_delay"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	Files   FilesConfig   `yaml:"files" json:"files"`
	Objects ObjectsConfig `yaml:"objects" json:"objects"`
}

// FilesConfig 文件存储配置
type FilesConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	BasePath string `yaml:"base_path" json:"base_path"`
}

// ObjectsConfig 对象存储配置
type ObjectsConfig struct {
	Enabled  bool      `yaml:"enabled" json:"enabled"`
	Provider string    `yaml:"provider" json:"provider"`
	S3       S3Config  `yaml:"s3" json:"s3"`
	Minio    MinioConfig `yaml:"minio" json:"minio"`
	OSS      OSSConfig `yaml:"oss" json:"oss"`
}

// S3Config S3配置
type S3Config struct {
	Bucket    string `yaml:"bucket" json:"bucket"`
	Region    string `yaml:"region" json:"region"`
	AccessKey string `yaml:"access_key" json:"access_key"`
	SecretKey string `yaml:"secret_key" json:"secret_key"`
}

// MinioConfig MinIO配置
type MinioConfig struct {
	Endpoint  string `yaml:"endpoint" json:"endpoint"`
	Bucket    string `yaml:"bucket" json:"bucket"`
	AccessKey string `yaml:"access_key" json:"access_key"`
	SecretKey string `yaml:"secret_key" json:"secret_key"`
}

// OSSConfig OSS配置
type OSSConfig struct {
	Endpoint  string `yaml:"endpoint" json:"endpoint"`
	Bucket    string `yaml:"bucket" json:"bucket"`
	AccessKey string `yaml:"access_key" json:"access_key"`
	SecretKey string `yaml:"secret_key" json:"secret_key"`
}