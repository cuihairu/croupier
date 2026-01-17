// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf
	Server        ServerConfig             `json:"server" yaml:"server"`
	Registry      RegistryConfig           `json:"registry" yaml:"registry"`
	Dispatch      DispatchConfig           `json:"dispatch" yaml:"dispatch"`
	Auth          AuthConfig               `json:"auth" yaml:"auth"`
	BootstrapData BootstrapDataConfig      `json:"bootstrap_data" yaml:"bootstrap_data"`
	Descriptors   DescriptorConfig         `json:"descriptors" yaml:"descriptors"`
	Components    ComponentsConfig         `json:"components" yaml:"components"`
	Schemas       SchemasConfig            `json:"schemas" yaml:"schemas"`
	Packs         PacksConfig              `json:"packs" yaml:"packs"`
	Storage       StorageConfig            `json:"storage" yaml:"storage"`
	Cache         CacheConfig              `json:"cache" yaml:"cache"`
	CroupierLog   CroupierLogConfig        `json:"croupier_log" yaml:"croupier_log"`
	Metrics       MetricsConfig            `json:"metrics" yaml:"metrics"`
	Platforms     PlatformConfig           `json:"platforms" yaml:"platforms"`
	Profiles      map[string]ProfileConfig `json:"profiles" yaml:"profiles"`
}

type ServerConfig struct {
	Addr     string         `json:"addr" yaml:"addr"`
	Cert     string         `json:"cert,optional" yaml:"cert,optional"`
	Key      string         `json:"key,optional" yaml:"key,optional"`
	CA       string         `json:"ca,optional" yaml:"ca,optional"`
	Database DatabaseConfig `json:"db" yaml:"db"`
}

type DatabaseConfig struct {
	Driver     string `json:"driver,optional" yaml:"driver,optional"`
	DataSource string `json:"datasource,optional" yaml:"datasource,optional"`
}

type RegistryConfig struct {
	AssignmentsPath      string `json:"assignments_path,optional" yaml:"assignments_path,optional"`
	AnalyticsFiltersPath string `json:"analytics_filters_path,optional" yaml:"analytics_filters_path,optional"`
	RateLimitsPath       string `json:"rate_limits_path,optional" yaml:"rate_limits_path,optional"`
}

type DispatchConfig struct {
	JobRoutingDir string          `json:"job_routing_dir,optional" yaml:"job_routing_dir,optional"`
	JobRoutingTTL string          `json:"job_routing_ttl,optional" yaml:"job_routing_ttl,optional"`
	AgentTLS      TLSClientConfig `json:"agent_tls,optional" yaml:"agent_tls,optional"`
}

type TLSClientConfig struct {
	Enabled            bool   `json:"enabled,optional" yaml:"enabled,optional"`
	CertFile           string `json:"cert_file,optional" yaml:"cert_file,optional"`
	KeyFile            string `json:"key_file,optional" yaml:"key_file,optional"`
	CAFile             string `json:"ca_file,optional" yaml:"ca_file,optional"`
	ServerName         string `json:"server_name,optional" yaml:"server_name,optional"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify,optional" yaml:"insecure_skip_verify,optional"`
}

type AuthConfig struct {
	JWTSecret   string `json:"jwt_secret,optional" yaml:"jwt_secret,optional"`
	RBACConfig  string `json:"rbac_config,optional" yaml:"rbac_config,optional"`
	UsersConfig string `json:"users_config,optional" yaml:"users_config,optional"`
	GamesConfig string `json:"games_config,optional" yaml:"games_config,optional"`
}

type BootstrapDataConfig struct {
	BaseDir string `json:"base_dir,optional" yaml:"base_dir,optional"`
}

type DescriptorConfig struct {
	Dir string `json:"dir,optional" yaml:"dir,optional"`
}

type ComponentsConfig struct {
	DataDir    string `json:"data_dir,optional" yaml:"data_dir,optional"`
	StagingDir string `json:"staging_dir,optional" yaml:"staging_dir,optional"`
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
	AccessKey      string `json:"access_key,optional" yaml:"access_key,optional"`
	SecretKey      string `json:"secret_key,optional" yaml:"secret_key,optional"`
	ForcePathStyle bool   `json:"force_path_style,optional" yaml:"force_path_style,optional"`
	SignedURLTTL   string `json:"signed_url_ttl,optional" yaml:"signed_url_ttl,optional"`
	BaseDir        string `json:"base_dir,optional" yaml:"base_dir,optional"`
}

type CroupierLogConfig struct {
	Level      string `json:"level,optional" yaml:"level,optional"`
	Format     string `json:"format,optional" yaml:"format,optional"`
	Output     string `json:"output,optional" yaml:"output,optional"`
	File       string `json:"file,optional" yaml:"file,optional"`
	MaxSize    int    `json:"max_size,optional" yaml:"max_size,optional"`
	MaxBackups int    `json:"max_backups,optional" yaml:"max_backups,optional"`
	MaxAge     int    `json:"max_age,optional" yaml:"max_age,optional"`
	Compress   bool   `json:"compress,optional" yaml:"compress,optional"`
}

type MetricsConfig struct {
	PerFunction   bool `json:"per_function,optional" yaml:"per_function,optional"`
	PerGameDenies bool `json:"per_game_denies,optional" yaml:"per_game_denies,optional"`
}

type ProfileConfig struct {
	Log     map[string]interface{} `json:"log" yaml:"log"`
	DB      map[string]interface{} `json:"db" yaml:"db"`
	Storage map[string]interface{} `json:"storage" yaml:"storage"`
}

type CacheConfig struct {
	Enabled  bool   `json:"enabled,optional" yaml:"enabled,optional"`     // 是否启用缓存
	Type     string `json:"type,optional" yaml:"type,optional"`           // 缓存类型: redis, local
	Addr     string `json:"addr,optional" yaml:"addr,optional"`           // Redis 地址 (host:port)
	Password string `json:"password,optional" yaml:"password,optional"`   // Redis 密码
	DB       int    `json:"db,optional" yaml:"db,optional"`               // Redis 数据库编号
	PoolSize int    `json:"pool_size,optional" yaml:"pool_size,optional"` // Redis 连接池大小
	TTL      string `json:"ttl,optional" yaml:"ttl,optional"`             // 默认过期时间 (例如: "5m", "1h")
	MaxItems int    `json:"max_items,optional" yaml:"max_items,optional"` // 本地缓存最大条目数
	EvictTTL string `json:"evict_ttl,optional" yaml:"evict_ttl,optional"` // 本地缓存清理间隔
}

type PlatformConfig struct {
	ConfigFile string `json:"config_file,optional" yaml:"config_file,optional"` // 配置文件路径
	Enabled    bool   `json:"enabled,optional" yaml:"enabled,optional"`         // 是否启用平台集成
}
