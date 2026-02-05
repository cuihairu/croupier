// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"github.com/cuihairu/croupier/internal/cli/common"
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf
	Database      DatabaseConfig           `json:"database" yaml:"database"`
	Control       ControlConfig            `json:"Control" yaml:"Control"`
	Registry      RegistryConfig           `json:"registry" yaml:"registry"`
	AgentDispatch AgentDispatchConfig      `json:"AgentDispatch" yaml:"AgentDispatch"`
	Auth          AuthConfig               `json:"auth" yaml:"auth"`
	BootstrapData BootstrapDataConfig      `json:"BootstrapData" yaml:"BootstrapData"`
	Descriptors   DescriptorConfig         `json:"descriptors" yaml:"descriptors"`
	Components    ComponentsConfig         `json:"components" yaml:"components"`
	Schemas       SchemasConfig            `json:"schemas" yaml:"schemas"`
	Packs         PacksConfig              `json:"packs" yaml:"packs"`
	Storage       StorageConfig            `json:"storage" yaml:"storage"`
	Cache         CacheConfig              `json:"cache" yaml:"cache"`
	Logging       common.LogConfig         `json:",optional" yaml:"Log"`
	Metrics       MetricsConfig            `json:"metrics" yaml:"metrics"`
	Platforms     PlatformConfig           `json:"platforms" yaml:"platforms"`
	Profiles      map[string]ProfileConfig `json:"profiles" yaml:"profiles"`
	SSE           SSEConfig                `json:"sse" yaml:"sse"`
	// Server metadata for registration
	Region string            `json:"region,optional" yaml:"region,optional"`
	Zone   string            `json:"zone,optional" yaml:"zone,optional"`
	Labels map[string]string `json:"labels,optional" yaml:"labels,optional"`
}

// ControlConfig 配置 NNG 控制服务器（控制平面）
type ControlConfig struct {
	// NNG ControlService 监听地址（默认 :19090，用于 SDK/Agent 连接）
	// 支持多传输层，可以使用逗号分隔多个地址
	// 例如: ":19090" 或 ":19090,ipc://croupier-server"
	// 支持的传输协议: tcp://, ipc:// (Windows Named Pipes / Unix Domain Socket)
	Addr string `json:"Addr" yaml:"Addr"`

	// IPC 地址（可选），用于本地高性能通信
	// Windows: ipc://croupier-server
	// Linux/Unix: ipc:///tmp/croupier-server.sock 或 ipc://@croupier-server (abstract)
	IPCAddr string `json:"IPCAddr,optional" yaml:"IPCAddr,optional"`

	// TLS 证书配置（保留用于未来 NNG TLS 支持）
	Cert string `json:"Cert,optional" yaml:"Cert,optional"`
	Key  string `json:"Key,optional" yaml:"Key,optional"`
	CA   string `json:"CA,optional" yaml:"CA,optional"`
}

// DatabaseConfig 配置数据库连接
type DatabaseConfig struct {
	Driver     string `json:"driver,optional" yaml:"driver,optional"`
	DataSource string `json:"datasource,optional" yaml:"datasource,optional"`
}

type RegistryConfig struct {
	AssignmentsPath      string `json:"AssignmentsPath,optional" yaml:"AssignmentsPath,optional"`
	AnalyticsFiltersPath string `json:"AnalyticsFiltersPath,optional" yaml:"AnalyticsFiltersPath,optional"`
	RateLimitsPath       string `json:"RateLimitsPath,optional" yaml:"RateLimitsPath,optional"`
}

type AgentDispatchConfig struct {
	JobRoutingDir string          `json:"JobRoutingDir,optional" yaml:"JobRoutingDir,optional"`
	JobRoutingTTL string          `json:"JobRoutingTTL,optional" yaml:"JobRoutingTTL,optional"`
	ToAgentTLS    TLSClientConfig `json:"ToAgentTLS,optional" yaml:"ToAgentTLS,optional"` // Server → Agent TLS
}

type TLSClientConfig struct {
	Enabled            bool   `json:"enabled,optional" yaml:"enabled,optional"`
	CertFile           string `json:"CertFile,optional" yaml:"CertFile,optional"`
	KeyFile            string `json:"KeyFile,optional" yaml:"KeyFile,optional"`
	CAFile             string `json:"CAFile,optional" yaml:"CAFile,optional"`
	ServerName         string `json:"ServerName,optional" yaml:"ServerName,optional"`
	InsecureSkipVerify bool   `json:"InsecureSkipVerify,optional" yaml:"InsecureSkipVerify,optional"`
}

type AuthConfig struct {
	JWTSecret   string `json:"JWTSecret,optional" yaml:"JWTSecret,optional"`
	RBACConfig  string `json:"RBACConfig,optional" yaml:"RBACConfig,optional"`
	UsersConfig string `json:"UsersConfig,optional" yaml:"UsersConfig,optional"`
	GamesConfig string `json:"GamesConfig,optional" yaml:"GamesConfig,optional"`
}

type BootstrapDataConfig struct {
	BaseDir string `json:"BaseDir,optional" yaml:"BaseDir,optional"`
}

type DescriptorConfig struct {
	Dir string `json:"dir,optional" yaml:"dir,optional"`
}

type ComponentsConfig struct {
	DataDir    string `json:"DataDir,optional" yaml:"DataDir,optional"`
	StagingDir string `json:"StagingDir,optional" yaml:"StagingDir,optional"`
}

type SchemasConfig struct {
	Dir string `json:"dir,optional" yaml:"dir,optional"`
}

type PacksConfig struct {
	Dir string `json:"dir,optional" yaml:"dir,optional"`
}

type StorageConfig struct {
	Driver         string `json:"driver,optional" yaml:"driver,optional"`
	Bucket         string `json:"bucket,optional" yaml:"bucket,optional"`
	Region         string `json:"region,optional" yaml:"region,optional"`
	Endpoint       string `json:"endpoint,optional" yaml:"endpoint,optional"`
	AccessKey      string `json:"AccessKey,optional" yaml:"AccessKey,optional"`
	SecretKey      string `json:"SecretKey,optional" yaml:"SecretKey,optional"`
	ForcePathStyle bool   `json:"ForcePathStyle,optional" yaml:"ForcePathStyle,optional"`
	SignedURLTTL   string `json:"SignedURLTTL,optional" yaml:"SignedURLTTL,optional"`
	BaseDir        string `json:"BaseDir,optional" yaml:"BaseDir,optional"`
}

type MetricsConfig struct {
	PerFunction   bool `json:"PerFunction,optional" yaml:"PerFunction,optional"`
	PerGameDenies bool `json:"PerGameDenies,optional" yaml:"PerGameDenies,optional"`
}

type ProfileConfig struct {
	Log     map[string]interface{} `json:"log" yaml:"log"`
	DB      map[string]interface{} `json:"db" yaml:"db"`
	Storage map[string]interface{} `json:"storage" yaml:"storage"`
}

type CacheConfig struct {
	Enabled  bool   `json:"Enabled,optional" yaml:"Enabled,optional"`   // 是否启用缓存
	Type     string `json:"Type,optional" yaml:"Type,optional"`         // 缓存类型: redis, local
	Addr     string `json:"Addr,optional" yaml:"Addr,optional"`         // Redis 地址 (host:port)
	Password string `json:"Password,optional" yaml:"Password,optional"` // Redis 密码
	DB       int    `json:"DB,optional" yaml:"DB,optional"`             // Redis 数据库编号
	PoolSize int    `json:"PoolSize,optional" yaml:"PoolSize,optional"` // Redis 连接池大小
	TTL      string `json:"TTL,optional" yaml:"TTL,optional"`           // 默认过期时间 (例如: "5m", "1h")
	MaxItems int    `json:"MaxItems,optional" yaml:"MaxItems,optional"` // 本地缓存最大条目数
	EvictTTL string `json:"EvictTTL,optional" yaml:"EvictTTL,optional"` // 本地缓存清理间隔
}

type PlatformConfig struct {
	ConfigFile string `json:"ConfigFile,optional" yaml:"ConfigFile,optional"` // 配置文件路径
	Enabled    bool   `json:"Enabled,optional" yaml:"Enabled,optional"`       // 是否启用平台集成
}

// SSEConfig 配置 Server-Sent Events (SSE) 推送
type SSEConfig struct {
	UpdateInterval    int `json:"UpdateInterval,optional" yaml:"UpdateInterval,optional"`       // 消息更新间隔（秒），默认 2
	KeepAliveInterval int `json:"KeepAliveInterval,optional" yaml:"KeepAliveInterval,optional"` // Keep-alive 间隔（秒），默认 30
}
