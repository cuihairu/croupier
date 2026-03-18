package config

import (
	"github.com/cuihairu/croupier/internal/cli/common"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server        ServerConfig             `json:"server" yaml:"server"`
	Database      DatabaseConfig           `json:"database" yaml:"database"`
	Control       ControlConfig            `json:"control" yaml:"control"`
	Registry      RegistryConfig           `json:"registry" yaml:"registry"`
	AgentDispatch AgentDispatchConfig      `json:"agentDispatch" yaml:"agentDispatch"`
	Auth          AuthConfig               `json:"auth" yaml:"auth"`
	BootstrapData BootstrapDataConfig      `json:"bootstrapData" yaml:"bootstrapData"`
	Descriptors   DescriptorConfig         `json:"descriptors" yaml:"descriptors"`
	Schemas       SchemasConfig            `json:"schemas" yaml:"schemas"`
	Storage       StorageConfig            `json:"storage" yaml:"storage"`
	Cache         CacheConfig              `json:"cache" yaml:"cache"`
	Logging       common.LogConfig         `json:"log,omitempty" yaml:"log"`
	Metrics       MetricsConfig            `json:"metrics" yaml:"metrics"`
	Profiles      map[string]ProfileConfig `json:"profiles" yaml:"profiles"`
	SSE           SSEConfig                `json:"sse" yaml:"sse"`
	// Server metadata for registration
	Region string            `json:"region,omitempty" yaml:"region,omitempty"`
	Zone   string            `json:"zone,omitempty" yaml:"zone,omitempty"`
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

func (c *Config) UnmarshalYAML(value *yaml.Node) error {
	type plain Config
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}

	var compat struct {
		Server        ServerConfig        `yaml:"Server"`
		Control       ControlConfig       `yaml:"Control"`
		AgentDispatch AgentDispatchConfig `yaml:"AgentDispatch"`
		BootstrapData BootstrapDataConfig `yaml:"BootstrapData"`
		Logging       common.LogConfig    `yaml:"Log"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}

	if isZeroServerConfig(decoded.Server) {
		decoded.Server = compat.Server
	}
	if isZeroControlConfig(decoded.Control) {
		decoded.Control = compat.Control
	}
	if isZeroAgentDispatchConfig(decoded.AgentDispatch) {
		decoded.AgentDispatch = compat.AgentDispatch
	}
	if isZeroBootstrapDataConfig(decoded.BootstrapData) {
		decoded.BootstrapData = compat.BootstrapData
	}
	if isZeroLogConfig(decoded.Logging) {
		decoded.Logging = compat.Logging
	}

	*c = Config(decoded)
	return nil
}

// ControlConfig 配置 NNG 控制服务器（控制平面）
type ControlConfig struct {
	// NNG ControlService 监听地址（默认 :19090，用于 SDK/Agent 连接）
	// 支持多传输层，可以使用逗号分隔多个地址
	// 例如: ":19090" 或 ":19090,ipc://croupier-server"
	// 支持的传输协议: tcp://, ipc:// (Windows Named Pipes / Unix Domain Socket)
	Addr string `json:"addr" yaml:"addr"`

	// IPC 地址（可选），用于本地高性能通信
	// Windows: ipc://croupier-server
	// Linux/Unix: ipc:///tmp/croupier-server.sock 或 ipc://@croupier-server (abstract)
	IPCAddr string `json:"ipcAddr,omitempty" yaml:"ipcAddr,omitempty"`

	// TLS 证书配置（保留用于未来 NNG TLS 支持）
	Cert string `json:"cert,omitempty" yaml:"cert,omitempty"`
	Key  string `json:"key,omitempty" yaml:"key,omitempty"`
	CA   string `json:"ca,omitempty" yaml:"ca,omitempty"`
}

func (c *ControlConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain ControlConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	var compat struct {
		Addr    string `yaml:"Addr,omitempty"`
		IPCAddr string `yaml:"IPCAddr,omitempty"`
		Cert    string `yaml:"Cert,omitempty"`
		Key     string `yaml:"Key,omitempty"`
		CA      string `yaml:"CA,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if decoded.Addr == "" {
		decoded.Addr = compat.Addr
	}
	if decoded.IPCAddr == "" {
		decoded.IPCAddr = compat.IPCAddr
	}
	if decoded.Cert == "" {
		decoded.Cert = compat.Cert
	}
	if decoded.Key == "" {
		decoded.Key = compat.Key
	}
	if decoded.CA == "" {
		decoded.CA = compat.CA
	}
	*c = ControlConfig(decoded)
	return nil
}

// ServerConfig HTTP 服务器配置
type ServerConfig struct {
	Host     string `json:"host" yaml:"host"`
	Port     int    `json:"port" yaml:"port"`
	Mode     string `json:"mode" yaml:"mode"`       // dev | test | prod
	Timeout  int64  `json:"timeout" yaml:"timeout"` // 毫秒
	MaxConns int    `json:"maxConns" yaml:"maxConns"`
}

func (c *ServerConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain ServerConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	var compat struct {
		Host     string `yaml:"Host,omitempty"`
		Port     int    `yaml:"Port,omitempty"`
		Mode     string `yaml:"Mode,omitempty"`
		Timeout  int64  `yaml:"Timeout,omitempty"`
		MaxConns int    `yaml:"MaxConns,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if decoded.Host == "" {
		decoded.Host = compat.Host
	}
	if decoded.Port == 0 {
		decoded.Port = compat.Port
	}
	if decoded.Mode == "" {
		decoded.Mode = compat.Mode
	}
	if decoded.Timeout == 0 {
		decoded.Timeout = compat.Timeout
	}
	if decoded.MaxConns == 0 {
		decoded.MaxConns = compat.MaxConns
	}
	*c = ServerConfig(decoded)
	return nil
}

// DatabaseConfig 配置数据库连接
type DatabaseConfig struct {
	Driver     string `json:"driver,omitempty" yaml:"driver,omitempty"`
	DataSource string `json:"dataSource,omitempty" yaml:"dataSource,omitempty"`
}

func (c *DatabaseConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain DatabaseConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	var compat struct {
		Driver     string `yaml:"Driver,omitempty"`
		DataSource string `yaml:"DataSource,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if decoded.Driver == "" {
		decoded.Driver = compat.Driver
	}
	if decoded.DataSource == "" {
		decoded.DataSource = compat.DataSource
	}
	*c = DatabaseConfig(decoded)
	return nil
}

type RegistryConfig struct {
	AssignmentsPath      string `json:"assignmentsPath,omitempty" yaml:"assignmentsPath,omitempty"`
	AnalyticsFiltersPath string `json:"analyticsFiltersPath,omitempty" yaml:"analyticsFiltersPath,omitempty"`
	RateLimitsPath       string `json:"rateLimitsPath,omitempty" yaml:"rateLimitsPath,omitempty"`
}

func (c *RegistryConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain RegistryConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	var compat struct {
		AssignmentsPath      string `yaml:"AssignmentsPath,omitempty"`
		AnalyticsFiltersPath string `yaml:"AnalyticsFiltersPath,omitempty"`
		RateLimitsPath       string `yaml:"RateLimitsPath,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if decoded.AssignmentsPath == "" {
		decoded.AssignmentsPath = compat.AssignmentsPath
	}
	if decoded.AnalyticsFiltersPath == "" {
		decoded.AnalyticsFiltersPath = compat.AnalyticsFiltersPath
	}
	if decoded.RateLimitsPath == "" {
		decoded.RateLimitsPath = compat.RateLimitsPath
	}
	*c = RegistryConfig(decoded)
	return nil
}

type AgentDispatchConfig struct {
	JobRoutingDir string          `json:"jobRoutingDir,omitempty" yaml:"jobRoutingDir,omitempty"`
	JobRoutingTTL string          `json:"jobRoutingTTL,omitempty" yaml:"jobRoutingTTL,omitempty"`
	ToAgentTLS    TLSClientConfig `json:"toAgentTLS,omitempty" yaml:"toAgentTLS,omitempty"` // Server → Agent TLS
}

func (c *AgentDispatchConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain AgentDispatchConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	var compat struct {
		JobRoutingDir string          `yaml:"JobRoutingDir,omitempty"`
		JobRoutingTTL string          `yaml:"JobRoutingTTL,omitempty"`
		ToAgentTLS    TLSClientConfig `yaml:"ToAgentTLS,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if decoded.JobRoutingDir == "" {
		decoded.JobRoutingDir = compat.JobRoutingDir
	}
	if decoded.JobRoutingTTL == "" {
		decoded.JobRoutingTTL = compat.JobRoutingTTL
	}
	if isZeroTLSClientConfig(decoded.ToAgentTLS) {
		decoded.ToAgentTLS = compat.ToAgentTLS
	}
	*c = AgentDispatchConfig(decoded)
	return nil
}

type TLSClientConfig struct {
	Enabled            bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	CertFile           string `json:"certFile,omitempty" yaml:"certFile,omitempty"`
	KeyFile            string `json:"keyFile,omitempty" yaml:"keyFile,omitempty"`
	CAFile             string `json:"caFile,omitempty" yaml:"caFile,omitempty"`
	ServerName         string `json:"serverName,omitempty" yaml:"serverName,omitempty"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty"`
}

func (c *TLSClientConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain TLSClientConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}

	var compat struct {
		Enabled            *bool  `yaml:"Enabled,omitempty"`
		CertFile           string `yaml:"CertFile,omitempty"`
		KeyFile            string `yaml:"KeyFile,omitempty"`
		CAFile             string `yaml:"CAFile,omitempty"`
		ServerName         string `yaml:"ServerName,omitempty"`
		InsecureSkipVerify *bool  `yaml:"InsecureSkipVerify,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}

	if !decoded.Enabled && compat.Enabled != nil {
		decoded.Enabled = *compat.Enabled
	}
	if decoded.CertFile == "" {
		decoded.CertFile = compat.CertFile
	}
	if decoded.KeyFile == "" {
		decoded.KeyFile = compat.KeyFile
	}
	if decoded.CAFile == "" {
		decoded.CAFile = compat.CAFile
	}
	if decoded.ServerName == "" {
		decoded.ServerName = compat.ServerName
	}
	if !decoded.InsecureSkipVerify && compat.InsecureSkipVerify != nil {
		decoded.InsecureSkipVerify = *compat.InsecureSkipVerify
	}

	*c = TLSClientConfig(decoded)
	return nil
}

type AuthConfig struct {
	JWTSecret   string `json:"jwtSecret,omitempty" yaml:"jwtSecret,omitempty"`
	RBACConfig  string `json:"rbacConfig,omitempty" yaml:"rbacConfig,omitempty"`
	UsersConfig string `json:"usersConfig,omitempty" yaml:"usersConfig,omitempty"`
	GamesConfig string `json:"gamesConfig,omitempty" yaml:"gamesConfig,omitempty"`
}

func (c *AuthConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain AuthConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	var compat struct {
		JWTSecret   string `yaml:"JWTSecret,omitempty"`
		RBACConfig  string `yaml:"RBACConfig,omitempty"`
		UsersConfig string `yaml:"UsersConfig,omitempty"`
		GamesConfig string `yaml:"GamesConfig,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if decoded.JWTSecret == "" {
		decoded.JWTSecret = compat.JWTSecret
	}
	if decoded.RBACConfig == "" {
		decoded.RBACConfig = compat.RBACConfig
	}
	if decoded.UsersConfig == "" {
		decoded.UsersConfig = compat.UsersConfig
	}
	if decoded.GamesConfig == "" {
		decoded.GamesConfig = compat.GamesConfig
	}
	*c = AuthConfig(decoded)
	return nil
}

type BootstrapDataConfig struct {
	BaseDir string `json:"baseDir,omitempty" yaml:"baseDir,omitempty"`
}

func (c *BootstrapDataConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain BootstrapDataConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	var compat struct {
		BaseDir string `yaml:"BaseDir,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if decoded.BaseDir == "" {
		decoded.BaseDir = compat.BaseDir
	}
	*c = BootstrapDataConfig(decoded)
	return nil
}

type DescriptorConfig struct {
	Dir string `json:"dir,omitempty" yaml:"dir,omitempty"`
}

func (c *DescriptorConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain DescriptorConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}

	var compat struct {
		Dir string `yaml:"Dir,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}

	if decoded.Dir == "" {
		decoded.Dir = compat.Dir
	}

	*c = DescriptorConfig(decoded)
	return nil
}

type SchemasConfig struct {
	Dir string `json:"dir,omitempty" yaml:"dir,omitempty"`
}

func (c *SchemasConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain SchemasConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}

	var compat struct {
		Dir string `yaml:"Dir,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}

	if decoded.Dir == "" {
		decoded.Dir = compat.Dir
	}

	*c = SchemasConfig(decoded)
	return nil
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
	UpdateInterval    int `json:"updateInterval,omitempty" yaml:"updateInterval,omitempty"`       // 消息更新间隔（秒），默认 2
	KeepAliveInterval int `json:"keepAliveInterval,omitempty" yaml:"keepAliveInterval,omitempty"` // Keep-alive 间隔（秒），默认 30
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

func isZeroServerConfig(cfg ServerConfig) bool {
	return cfg == (ServerConfig{})
}

func isZeroControlConfig(cfg ControlConfig) bool {
	return cfg == (ControlConfig{})
}

func isZeroAgentDispatchConfig(cfg AgentDispatchConfig) bool {
	return cfg == (AgentDispatchConfig{})
}

func isZeroBootstrapDataConfig(cfg BootstrapDataConfig) bool {
	return cfg == (BootstrapDataConfig{})
}

func isZeroTLSClientConfig(cfg TLSClientConfig) bool {
	return cfg == (TLSClientConfig{})
}

func isZeroLogConfig(cfg common.LogConfig) bool {
	return cfg == (common.LogConfig{})
}
