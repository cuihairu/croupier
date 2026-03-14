package config

import (
	"github.com/cuihairu/croupier/internal/cli/common"
)

type Config struct {
	Server        ServerConfig             `json:"Server" yaml:"Server"`
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
	Logging       common.LogConfig         `json:",omitempty" yaml:"Log"`
	Metrics       MetricsConfig            `json:"metrics" yaml:"metrics"`
	Platforms     PlatformConfig           `json:"platforms" yaml:"platforms"`
	Profiles      map[string]ProfileConfig `json:"profiles" yaml:"profiles"`
	SSE           SSEConfig                `json:"sse" yaml:"sse"`
	// Server metadata for registration
	Region string            `json:"region,omitempty" yaml:"region,omitempty"`
	Zone   string            `json:"zone,omitempty" yaml:"zone,omitempty"`
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
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
	IPCAddr string `json:"IPCAddr,omitempty" yaml:"IPCAddr,omitempty"`

	// TLS 证书配置（保留用于未来 NNG TLS 支持）
	Cert string `json:"Cert,omitempty" yaml:"Cert,omitempty"`
	Key  string `json:"Key,omitempty" yaml:"Key,omitempty"`
	CA   string `json:"CA,omitempty" yaml:"CA,omitempty"`
}

// ServerConfig HTTP 服务器配置
type ServerConfig struct {
	Host     string `json:"Host" yaml:"Host"`
	Port     int    `json:"Port" yaml:"Port"`
	Mode     string `json:"Mode" yaml:"Mode"`       // dev | test | prod
	Timeout  int64  `json:"Timeout" yaml:"Timeout"` // 毫秒
	MaxConns int    `json:"MaxConns" yaml:"MaxConns"`
}

// DatabaseConfig 配置数据库连接
type DatabaseConfig struct {
	Driver     string `json:"driver,omitempty" yaml:"driver,omitempty"`
	DataSource string `json:"datasource,omitempty" yaml:"datasource,omitempty"`
}

type RegistryConfig struct {
	AssignmentsPath      string `json:"AssignmentsPath,omitempty" yaml:"AssignmentsPath,omitempty"`
	AnalyticsFiltersPath string `json:"AnalyticsFiltersPath,omitempty" yaml:"AnalyticsFiltersPath,omitempty"`
	RateLimitsPath       string `json:"RateLimitsPath,omitempty" yaml:"RateLimitsPath,omitempty"`
}

type AgentDispatchConfig struct {
	JobRoutingDir string          `json:"JobRoutingDir,omitempty" yaml:"JobRoutingDir,omitempty"`
	JobRoutingTTL string          `json:"JobRoutingTTL,omitempty" yaml:"JobRoutingTTL,omitempty"`
	ToAgentTLS    TLSClientConfig `json:"ToAgentTLS,omitempty" yaml:"ToAgentTLS,omitempty"` // Server → Agent TLS
}

type TLSClientConfig struct {
	Enabled            bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	CertFile           string `json:"CertFile,omitempty" yaml:"CertFile,omitempty"`
	KeyFile            string `json:"KeyFile,omitempty" yaml:"KeyFile,omitempty"`
	CAFile             string `json:"CAFile,omitempty" yaml:"CAFile,omitempty"`
	ServerName         string `json:"ServerName,omitempty" yaml:"ServerName,omitempty"`
	InsecureSkipVerify bool   `json:"InsecureSkipVerify,omitempty" yaml:"InsecureSkipVerify,omitempty"`
}

type AuthConfig struct {
	JWTSecret   string `json:"JWTSecret,omitempty" yaml:"JWTSecret,omitempty"`
	RBACConfig  string `json:"RBACConfig,omitempty" yaml:"RBACConfig,omitempty"`
	UsersConfig string `json:"UsersConfig,omitempty" yaml:"UsersConfig,omitempty"`
	GamesConfig string `json:"GamesConfig,omitempty" yaml:"GamesConfig,omitempty"`
}

type BootstrapDataConfig struct {
	BaseDir string `json:"BaseDir,omitempty" yaml:"BaseDir,omitempty"`
}

type DescriptorConfig struct {
	Dir string `json:"dir,omitempty" yaml:"dir,omitempty"`
}

type ComponentsConfig struct {
	DataDir    string `json:"DataDir,omitempty" yaml:"DataDir,omitempty"`
	StagingDir string `json:"StagingDir,omitempty" yaml:"StagingDir,omitempty"`
}

type SchemasConfig struct {
	Dir string `json:"dir,omitempty" yaml:"dir,omitempty"`
}

type PacksConfig struct {
	Dir string `json:"dir,omitempty" yaml:"dir,omitempty"`
}

type StorageConfig struct {
	Driver         string `json:"driver,omitempty" yaml:"driver,omitempty"`
	Bucket         string `json:"bucket,omitempty" yaml:"bucket,omitempty"`
	Region         string `json:"region,omitempty" yaml:"region,omitempty"`
	Endpoint       string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	AccessKey      string `json:"AccessKey,omitempty" yaml:"AccessKey,omitempty"`
	SecretKey      string `json:"SecretKey,omitempty" yaml:"SecretKey,omitempty"`
	ForcePathStyle bool   `json:"ForcePathStyle,omitempty" yaml:"ForcePathStyle,omitempty"`
	SignedURLTTL   string `json:"SignedURLTTL,omitempty" yaml:"SignedURLTTL,omitempty"`
	BaseDir        string `json:"BaseDir,omitempty" yaml:"BaseDir,omitempty"`
}

type MetricsConfig struct {
	PerFunction   bool `json:"PerFunction,omitempty" yaml:"PerFunction,omitempty"`
	PerGameDenies bool `json:"PerGameDenies,omitempty" yaml:"PerGameDenies,omitempty"`
}

type ProfileConfig struct {
	Log     map[string]interface{} `json:"log" yaml:"log"`
	DB      map[string]interface{} `json:"db" yaml:"db"`
	Storage map[string]interface{} `json:"storage" yaml:"storage"`
}

type CacheConfig struct {
	Enabled  bool   `json:"Enabled,omitempty" yaml:"Enabled,omitempty"`   // 是否启用缓存
	Type     string `json:"Type,omitempty" yaml:"Type,omitempty"`         // 缓存类型: redis, local
	Addr     string `json:"Addr,omitempty" yaml:"Addr,omitempty"`         // Redis 地址 (host:port)
	Password string `json:"Password,omitempty" yaml:"Password,omitempty"` // Redis 密码
	DB       int    `json:"DB,omitempty" yaml:"DB,omitempty"`             // Redis 数据库编号
	PoolSize int    `json:"PoolSize,omitempty" yaml:"PoolSize,omitempty"` // Redis 连接池大小
	TTL      string `json:"TTL,omitempty" yaml:"TTL,omitempty"`           // 默认过期时间 (例如: "5m", "1h")
	MaxItems int    `json:"MaxItems,omitempty" yaml:"MaxItems,omitempty"` // 本地缓存最大条目数
	EvictTTL string `json:"EvictTTL,omitempty" yaml:"EvictTTL,omitempty"` // 本地缓存清理间隔
}

type PlatformConfig struct {
	ConfigFile string `json:"ConfigFile,omitempty" yaml:"ConfigFile,omitempty"` // 配置文件路径
	Enabled    bool   `json:"Enabled,omitempty" yaml:"Enabled,omitempty"`       // 是否启用平台集成
}

// SSEConfig 配置 Server-Sent Events (SSE) 推送
type SSEConfig struct {
	UpdateInterval    int `json:"UpdateInterval,omitempty" yaml:"UpdateInterval,omitempty"`       // 消息更新间隔（秒），默认 2
	KeepAliveInterval int `json:"KeepAliveInterval,omitempty" yaml:"KeepAliveInterval,omitempty"` // Keep-alive 间隔（秒），默认 30
}
