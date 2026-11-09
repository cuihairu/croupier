package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ControlConfig 配置控制服务器（控制平面）
type ControlConfig struct {
	// Transport selects the control-plane transport implementation.
	// Only "tcp" is supported (旧传输/IPC removed).
	Transport string `json:"transport,omitempty" yaml:"transport,omitempty"`

	// ControlService 监听地址（默认 :19090，用于 SDK/Agent 连接）
	// 格式: ":port" 或 "host:port"，例如 ":19090" 或 "0.0.0.0:19090"
	Addr string `json:"addr" yaml:"addr"`

	// IPC 地址已废弃（保留用于配置兼容性，不再使用）
	// 旧传输/IPC 传输已被移除，所有连接现在使用 TCP
	IPCAddr string `json:"ipcAddr,omitempty" yaml:"ipcAddr,omitempty"`

	// TLS 证书配置。如果配置了 Cert 和 Key，将启用 TLS
	Cert string `json:"cert,omitempty" yaml:"cert,omitempty"`
	Key  string `json:"key,omitempty" yaml:"key,omitempty"`
	CA   string `json:"ca,omitempty" yaml:"ca,omitempty"` // 可选的 CA 证书，用于客户端验证
}

func (c *ControlConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain ControlConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	var compat struct {
		Transport string `yaml:"Transport,omitempty"`
		Addr      string `yaml:"Addr,omitempty"`
		IPCAddr   string `yaml:"IPCAddr,omitempty"`
		Cert      string `yaml:"Cert,omitempty"`
		Key       string `yaml:"Key,omitempty"`
		CA        string `yaml:"CA,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if decoded.Transport == "" {
		decoded.Transport = compat.Transport
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
	// MultiGame enables the database-per-game architecture. When true, the
	// DataSource is treated as the meta database (croupier_meta) and each
	// (game_id, env) pair gets its own physical database. When false (the
	// default for backward compatibility in dev), all data lives in the single
	// configured database with game_id columns providing row-level isolation.
	MultiGame bool `json:"multiGame,omitempty" yaml:"multiGame,omitempty"`
	// GameDBPrefix overrides the default "game_" prefix used when deriving a
	// physical database name from a game scope. When empty, DefaultGameDBName
	// in internal/db/router is used ("game_<gameID>_<env>").
	GameDBPrefix string `json:"gameDbPrefix,omitempty" yaml:"gameDbPrefix,omitempty"`
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
	// SDKVersionMinimums 按语言配置最低可注册 SDK 版本（language → 语义化
	// 版本，如 go: "0.2.0"）。自报版本低于配置值的 provider 独占声明的
	// 函数不注册（只产生告警），未配置的语言仅走滑动高水位门槛。
	// 语言键大小写不敏感，配置值解析失败时该条目不生效。
	SDKVersionMinimums map[string]string `json:"sdkVersionMinimums,omitempty" yaml:"sdkVersionMinimums,omitempty"`
}

// ClusterConfig 配置 Server 多实例 HA（docs/architecture/server-ha-multi-instance.md）。
// enabled=false（默认）时完全单实例行为，互联与成员表不启用。
// 成员发现走共享存储（database/cache），无 seed、无静态 peers；
// 唯一必填是 advertiseAddr（告诉对端怎么连我）。
type ClusterConfig struct {
	Enabled       bool   `json:"enabled" yaml:"enabled"`
	InstanceID    string `json:"instanceId,omitempty" yaml:"instanceId,omitempty"`
	AdvertiseAddr string `json:"advertiseAddr,omitempty" yaml:"advertiseAddr,omitempty"`
	// InterconnectAddr 是互联监听地址（默认 advertiseAddr）。
	InterconnectAddr string `json:"interconnectAddr,omitempty" yaml:"interconnectAddr,omitempty"`
	// HeartbeatInterval 成员租约续期间隔（默认 5s）。
	HeartbeatInterval string `json:"heartbeatInterval,omitempty" yaml:"heartbeatInterval,omitempty"`
	// LeaseTTL 成员租约 TTL（默认 15s，建议 3 倍心跳）。
	LeaseTTL string `json:"leaseTtl,omitempty" yaml:"leaseTtl,omitempty"`
	// OwnerTTL agent 归属记录存活窗口（默认 3m，Agent 心跳续期）。
	OwnerTTL string `json:"ownerTtl,omitempty" yaml:"ownerTtl,omitempty"`
	// LbPrometheusUrl LB 监控数据源（Prometheus API，如
	// http://prometheus:9090）——空则 /ops/lb 页面隐藏（docs/operations/
	// load-balancing.md「LB 监控」）。支持环境变量
	// CROUPIER_LB_PROMETHEUS_URL 覆盖。
	LbPrometheusUrl string `json:"lbPrometheusUrl,omitempty" yaml:"lbPrometheusUrl,omitempty"`
	// PeerPollInterval 对端发现轮询间隔（默认 10s）。
	PeerPollInterval string `json:"peerPollInterval,omitempty" yaml:"peerPollInterval,omitempty"`
	// InsecureSkipTLS 互联 TLS 校验跳过（仅开发联调）。
	InsecureSkipTLS bool `json:"insecureSkipTls,omitempty" yaml:"insecureSkipTls,omitempty"`
	// InterconnectTLS 专用互联证书面（ARCH-003）：cert/key 提供本实例在
	// 互联端口的服务器证书（双 EKU = server 角色），ca 提供对端证书链。
	// 三者都配置且非 insecureSkipTls 时互联走 mTLS；CR-FIX-03（P0-3）起
	// 集群启用但证书面不完整或 insecureSkipTls=true 时默认拒绝启动（fail-
	// closed），只有显式 AllowPlaintextInterconnect=true 才放行内网明文并在
	// 就绪面记录降级——不再静默降级明文。
	InterconnectTLS TLSClientConfig `json:"interconnectTls,omitempty" yaml:"interconnectTls,omitempty"`
	// AllowPlaintextInterconnect 显式二次确认（CR-FIX-03）：集群启用且证书
	// 面缺失/跳过校验时，只有置 true 才允许以明文互联启动（就绪面记录
	// 降级）；缺省 false = fail-closed，漏配证书时拒绝启动并明示原因。
	AllowPlaintextInterconnect bool `json:"allowPlaintextInterconnect,omitempty" yaml:"allowPlaintextInterconnect,omitempty"`
	// AllowStandaloneDegrade 显式允许 standalone 降级（ARCH-004）：集群
	// enabled 时协调库/成员注册/互联监听不可用，默认保持「要求集群」
	// 语义（就绪探针 503，实例不可服务）直到故障排除重启；只有显式置
	// true 才允许按单实例降级运行（降级状态在 /healthz 组件可观测）。
	AllowStandaloneDegrade bool `json:"allowStandaloneDegrade,omitempty" yaml:"allowStandaloneDegrade,omitempty"`
	// Store 协调面存储：db（默认，共享关系库）| redis。
	// 单实例部署不启用集群（enabled: false）时无协调面。
	Store string `json:"store,omitempty" yaml:"store,omitempty"`
	// RedisAddr 仅 store: redis 时生效；空则回退 cache 段的 redis 配置。
	RedisAddr     string `json:"redisAddr,omitempty" yaml:"redisAddr,omitempty"`
	RedisPassword string `json:"redisPassword,omitempty" yaml:"redisPassword,omitempty"`
	RedisDB       int    `json:"redisDb,omitempty" yaml:"redisDb,omitempty"`
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

// InterconnectTLSComplete reports whether the dedicated interconnect mTLS
// certificate surface is fully configured and not skipped (CR-FIX-03).
func (c *ClusterConfig) InterconnectTLSComplete() bool {
	return c.InterconnectTLS.EffectiveEnabled() &&
		!c.InsecureSkipTLS &&
		!c.InterconnectTLS.InsecureSkipVerify &&
		c.InterconnectTLS.CertFile != "" &&
		c.InterconnectTLS.KeyFile != "" &&
		c.InterconnectTLS.CAFile != ""
}

// ValidateInterconnectTLS implements CR-FIX-03 (P0-3) fail-closed: when the
// cluster is enabled but the interconnect certificate surface is incomplete or
// TLS verification is skipped, startup must be refused with a clear reason
// unless the operator explicitly confirms plaintext via
// AllowPlaintextInterconnect. This closes the silent plaintext fallback that
// previously left the mesh on an unauthenticated intranet path. Returns nil
// when the cluster is disabled, TLS is complete, or plaintext is explicitly
// allowed.
func (c *ClusterConfig) ValidateInterconnectTLS() error {
	if !c.Enabled {
		return nil
	}
	if c.InterconnectTLSComplete() {
		return nil
	}
	if c.AllowPlaintextInterconnect {
		return nil
	}
	var causes []string
	if !c.InterconnectTLS.EffectiveEnabled() {
		causes = append(causes, "interconnectTls.enabled=false")
	} else {
		var missing []string
		if c.InterconnectTLS.CertFile == "" {
			missing = append(missing, "certFile")
		}
		if c.InterconnectTLS.KeyFile == "" {
			missing = append(missing, "keyFile")
		}
		if c.InterconnectTLS.CAFile == "" {
			missing = append(missing, "caFile")
		}
		if len(missing) > 0 {
			causes = append(causes, "missing "+strings.Join(missing, "/"))
		}
	}
	if c.InsecureSkipTLS {
		causes = append(causes, "insecureSkipTls=true")
	}
	if c.InterconnectTLS.InsecureSkipVerify {
		causes = append(causes, "interconnectTls.insecureSkipVerify=true")
	}
	return fmt.Errorf("cluster.enabled=true but interconnect mTLS is not usable (%s); set cluster.allowPlaintextInterconnect=true only on a trusted isolated intranet to run plaintext (degraded), or configure the certificate surface for mTLS", strings.Join(causes, ", "))
}
