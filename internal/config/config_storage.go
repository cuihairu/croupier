package config

import (
	"gopkg.in/yaml.v3"
)

// AuditConfig configures the audit subsystem.
type AuditConfig struct {
	// WriteFailurePolicy decides what happens when an audit-record write
	// fails after the chain-conflict retries are exhausted (ARCH-007 D4):
	// "strict" (default, fail-closed) fails the audited operation so the
	// operator learns the trail has a gap; "best_effort" logs the gap at
	// WARN, fires a rate-limited alert and continues, trading trail
	// integrity for control-plane availability during an audit-store
	// outage. Unknown values fall back to strict.
	WriteFailurePolicy string `json:"writeFailurePolicy,omitempty" yaml:"writeFailurePolicy,omitempty"`
}

type StorageConfig struct {
	Driver         string `json:"driver,omitempty" yaml:"driver,omitempty"`
	Bucket         string `json:"bucket,omitempty" yaml:"bucket,omitempty"`
	Region         string `json:"region,omitempty" yaml:"region,omitempty"`
	Endpoint       string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	AccessKey      string `json:"accessKey,omitempty" yaml:"accessKey,omitempty"`
	SecretKey      string `json:"secretKey,omitempty" yaml:"secretKey,omitempty"`
	ForcePathStyle bool   `json:"forcePathStyle,omitempty" yaml:"forcePathStyle,omitempty"`
	SignedURLTTL   string `json:"signedUrlTTL,omitempty" yaml:"signedUrlTTL,omitempty"`
	BaseDir        string `json:"baseDir,omitempty" yaml:"baseDir,omitempty"`
}

func (c *StorageConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain StorageConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}

	var compat struct {
		Driver         string `yaml:"Driver,omitempty"`
		Bucket         string `yaml:"Bucket,omitempty"`
		Region         string `yaml:"Region,omitempty"`
		Endpoint       string `yaml:"Endpoint,omitempty"`
		AccessKey      string `yaml:"AccessKey,omitempty"`
		SecretKey      string `yaml:"SecretKey,omitempty"`
		ForcePathStyle *bool  `yaml:"ForcePathStyle,omitempty"`
		SignedURLTTL   string `yaml:"SignedURLTTL,omitempty"`
		BaseDir        string `yaml:"BaseDir,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}

	if decoded.Driver == "" {
		decoded.Driver = compat.Driver
	}
	if decoded.Bucket == "" {
		decoded.Bucket = compat.Bucket
	}
	if decoded.Region == "" {
		decoded.Region = compat.Region
	}
	if decoded.Endpoint == "" {
		decoded.Endpoint = compat.Endpoint
	}
	if decoded.AccessKey == "" {
		decoded.AccessKey = compat.AccessKey
	}
	if decoded.SecretKey == "" {
		decoded.SecretKey = compat.SecretKey
	}
	if !decoded.ForcePathStyle && compat.ForcePathStyle != nil {
		decoded.ForcePathStyle = *compat.ForcePathStyle
	}
	if decoded.SignedURLTTL == "" {
		decoded.SignedURLTTL = compat.SignedURLTTL
	}
	if decoded.BaseDir == "" {
		decoded.BaseDir = compat.BaseDir
	}

	*c = StorageConfig(decoded)
	return nil
}

type MetricsConfig struct {
	PerFunction   bool `json:"perFunction,omitempty" yaml:"perFunction,omitempty"`
	PerGameDenies bool `json:"perGameDenies,omitempty" yaml:"perGameDenies,omitempty"`
}

func (c *MetricsConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain MetricsConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	var compat struct {
		PerFunction   *bool `yaml:"PerFunction,omitempty"`
		PerGameDenies *bool `yaml:"PerGameDenies,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if !decoded.PerFunction && compat.PerFunction != nil {
		decoded.PerFunction = *compat.PerFunction
	}
	if !decoded.PerGameDenies && compat.PerGameDenies != nil {
		decoded.PerGameDenies = *compat.PerGameDenies
	}
	*c = MetricsConfig(decoded)
	return nil
}

type ProfileConfig struct {
	Log     map[string]interface{} `json:"log" yaml:"log"`
	DB      map[string]interface{} `json:"db" yaml:"db"`
	Storage map[string]interface{} `json:"storage" yaml:"storage"`
}

type CacheConfig struct {
	Enabled  bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`   // 是否启用缓存
	Type     string `json:"type,omitempty" yaml:"type,omitempty"`         // 缓存类型: redis, local
	Addr     string `json:"addr,omitempty" yaml:"addr,omitempty"`         // Redis 地址 (host:port)
	Password string `json:"password,omitempty" yaml:"password,omitempty"` // Redis 密码
	DB       int    `json:"db,omitempty" yaml:"db,omitempty"`             // Redis 数据库编号
	PoolSize int    `json:"poolSize,omitempty" yaml:"poolSize,omitempty"` // Redis 连接池大小
	TTL      string `json:"ttl,omitempty" yaml:"ttl,omitempty"`           // 默认过期时间 (例如: "5m", "1h")
	MaxItems int    `json:"maxItems,omitempty" yaml:"maxItems,omitempty"` // 本地缓存最大条目数
	EvictTTL string `json:"evictTTL,omitempty" yaml:"evictTTL,omitempty"` // 本地缓存清理间隔
}

func (c *CacheConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain CacheConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	var compat struct {
		Enabled  *bool  `yaml:"Enabled,omitempty"`
		Type     string `yaml:"Type,omitempty"`
		Addr     string `yaml:"Addr,omitempty"`
		Password string `yaml:"Password,omitempty"`
		DB       *int   `yaml:"DB,omitempty"`
		PoolSize *int   `yaml:"PoolSize,omitempty"`
		TTL      string `yaml:"TTL,omitempty"`
		MaxItems *int   `yaml:"MaxItems,omitempty"`
		EvictTTL string `yaml:"EvictTTL,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if !decoded.Enabled && compat.Enabled != nil {
		decoded.Enabled = *compat.Enabled
	}
	if decoded.Type == "" {
		decoded.Type = compat.Type
	}
	if decoded.Addr == "" {
		decoded.Addr = compat.Addr
	}
	if decoded.Password == "" {
		decoded.Password = compat.Password
	}
	if decoded.DB == 0 && compat.DB != nil {
		decoded.DB = *compat.DB
	}
	if decoded.PoolSize == 0 && compat.PoolSize != nil {
		decoded.PoolSize = *compat.PoolSize
	}
	if decoded.TTL == "" {
		decoded.TTL = compat.TTL
	}
	if decoded.MaxItems == 0 && compat.MaxItems != nil {
		decoded.MaxItems = *compat.MaxItems
	}
	if decoded.EvictTTL == "" {
		decoded.EvictTTL = compat.EvictTTL
	}
	*c = CacheConfig(decoded)
	return nil
}

// SSEConfig 配置 Server-Sent Events (SSE) 推送
type SSEConfig struct {
	UpdateInterval    int `json:"updateInterval,omitempty" yaml:"updateInterval,omitempty"`       // 消息更新间隔（秒），默认 60
	KeepAliveInterval int `json:"keepAliveInterval,omitempty" yaml:"keepAliveInterval,omitempty"` // Keep-alive 间隔（秒），默认 30
}

// GetUpdateInterval returns the configured update interval in seconds,
// defaulting to 60 if not set.
func (c *SSEConfig) GetUpdateInterval() int {
	if c.UpdateInterval > 0 {
		return c.UpdateInterval
	}
	return 60
}

func (c *SSEConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain SSEConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	var compat struct {
		UpdateInterval    *int `yaml:"UpdateInterval,omitempty"`
		KeepAliveInterval *int `yaml:"KeepAliveInterval,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if decoded.UpdateInterval == 0 && compat.UpdateInterval != nil {
		decoded.UpdateInterval = *compat.UpdateInterval
	}
	if decoded.KeepAliveInterval == 0 && compat.KeepAliveInterval != nil {
		decoded.KeepAliveInterval = *compat.KeepAliveInterval
	}
	*c = SSEConfig(decoded)
	return nil
}
