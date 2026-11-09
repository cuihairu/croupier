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
	Cluster       ClusterConfig            `json:"cluster" yaml:"cluster"`
	AgentDispatch AgentDispatchConfig      `json:"agentDispatch" yaml:"agentDispatch"`
	Auth          AuthConfig               `json:"auth" yaml:"auth"`
	Approval      ApprovalConfig           `json:"approval" yaml:"approval"`
	ExecutionLog  ExecutionLogConfig       `json:"executionLog" yaml:"executionLog"`
	TaskLog       TaskLogConfig            `json:"taskLog" yaml:"taskLog"`
	BootstrapData BootstrapDataConfig      `json:"bootstrapData" yaml:"bootstrapData"`
	Descriptors   DescriptorConfig         `json:"descriptors" yaml:"descriptors"`
	Schemas       SchemasConfig            `json:"schemas" yaml:"schemas"`
	Storage       StorageConfig            `json:"storage" yaml:"storage"`
	Cache         CacheConfig              `json:"cache" yaml:"cache"`
	Audit         AuditConfig              `json:"audit,omitempty" yaml:"audit,omitempty"`
	Logging       common.LogConfig         `json:"log,omitempty" yaml:"log"`
	Metrics       MetricsConfig            `json:"metrics" yaml:"metrics"`
	Telemetry     TelemetryConfig          `json:"telemetry" yaml:"telemetry"`
	Profiles      map[string]ProfileConfig `json:"profiles" yaml:"profiles"`
	SSE           SSEConfig                `json:"sse" yaml:"sse"`
	// Pages 页面发布分级（T10/D5）：composite 保存是否跳过人工提案审核。
	Pages PagesConfig `json:"pages" yaml:"pages"`
	// OpenAPI 上传管线护栏（M6）：大文档提示阈值（warn 级，不阻断）。
	OpenAPI OpenAPIConfig `json:"openapi,omitempty" yaml:"openapi,omitempty"`
	// FeatureFlags switches optional product domains on/off at the control
	// plane (API routes + dashboard menus). Unset flags default to enabled;
	// only explicit `false` disables a domain. Data-plane components (agent,
	// analytics worker, ingest) are not governed by these flags.
	FeatureFlags FeatureFlagsConfig `json:"featureFlags,omitempty" yaml:"featureFlags,omitempty"`
	// Herald 告警出口（Outlet 第一内置实现，plugin-mechanism 设计 §5/M2）：
	// 告警经 herald 投递（courier 式，渠道/收件人 herald 侧配置）。
	// Enabled 缺省 false——关=纯 croupier 告警页，行为与现状一致。
	Herald HeraldConfig `json:"herald,omitempty" yaml:"herald,omitempty"`
	// AlertInbound 告警入站 webhook（plugin-mechanism 设计 §5.3/M3）：
	// 第三方告警系统推入 croupier 落现有告警存储。每来源一个 secretEnv
	//（环境变量引用密钥，同 herald tokenEnv 口径）；alertmanager 来源的
	// labels→信封字段映射（配置驱动，不写死供应商字段名）。
	AlertInbound AlertInboundConfig `json:"alertInbound,omitempty" yaml:"alertInbound,omitempty"`
	// ExternalAPI 对外 REST API（incident-reports §9）：调用令牌鉴权 + 按
	// 令牌名限流。rateLimitPerMinute 缺省 30。
	ExternalAPI ExternalAPIConfig `json:"externalAPI,omitempty" yaml:"externalAPI,omitempty"`
	// ServerStatus 服务器维护状态 Provider（server-status-provider §3.2）：
	// agent 探活异常的维护窗口 gate。enabled 缺省 false=gate 整体旁路；
	// provider 缺省 noop（零配置=零行为变化+零外发）。
	ServerStatus ServerStatusConfig `json:"serverStatus,omitempty" yaml:"serverStatus,omitempty"`
	// Vault 密码箱（vault-provider 设计 §3）：渠道/支付商户密钥、上游服务
	// 与数据库凭据等敏感配置的统一存取面。enabled 缺省 false=现状行为
	//（凭据仍走各接入点的环境变量引用）；提供商切换=只改 provider 键。
	Vault VaultConfig `json:"vault,omitempty" yaml:"vault,omitempty"`
	// Probe server 本机探针（agent-health-recovery-review §4.2/§4.3，R3
	// server 侧内核装配）：周期探测只有 server 视角才是运维真值的目标
	//（server 自身依赖、只有 server 能访问的第三方平台 API），窗口事件走
	// 与 agent 上行同一条 Alert 生命周期（视角=server）。空段=不装配（现状）。
	Probe ServerProbeConfig `json:"probe,omitempty" yaml:"probe,omitempty"`
	// Provision 云资源供给引擎（OPS-001）：terraform-exec 驱动 tofu/
	// terraform，provider 模板内嵌。enabled 缺省 false=不装配（现状，
	// /ops/provision 端点如实报未配置）。
	Provision ProvisionConfig `json:"provision,omitempty" yaml:"provision,omitempty"`
	// Backup 数据面备份（OPS-008）域配置。schedule.enabled 缺省 false=
	// 不装配（仅显式 true 才启动定期备份后台循环）；interval 缺省 6h。
	Backup BackupConfig `json:"backup,omitempty" yaml:"backup,omitempty"`
	// Server metadata for registration
	Region string            `json:"region,omitempty" yaml:"region,omitempty"`
	Zone   string            `json:"zone,omitempty" yaml:"zone,omitempty"`
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

// 发布分级取值（T10/D5）。
const (
	// PublishReviewAuto 保存即发布（composite 保存链保存成功后自动落
	// published_page_specs，跳过人工提案接受；error 级诊断仍拒绝发布）。
	PublishReviewAuto = "auto"
	// PublishReviewRequired 提案 → 人工审核 → 发布（现状链路）。
	PublishReviewRequired = "required"
)

// Feature flag names. Keep in sync with web/src/access.ts.
const (
	FlagDev        = "dev"        // 研发域（缺陷追踪/任务安排）
	FlagSupport    = "support"    // 客服域（工单/FAQ/反馈）
	FlagAnalytics  = "analytics"  // 数据分析域
	FlagOps        = "ops"        // 运维中心域
	FlagExtensions = "extensions" // 扩展中心域
)

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
		Logging       legacyLogConfig     `yaml:"Log"`
	}
	var canonical struct {
		Logging canonicalLogConfig `yaml:"log"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if err := value.Decode(&canonical); err != nil {
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
		switch {
		case !canonical.Logging.isZero():
			decoded.Logging = canonical.Logging.toCommon()
		case !compat.Logging.isZero():
			decoded.Logging = compat.Logging.toCommon()
		}
	}

	*c = Config(decoded)
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

type canonicalLogConfig struct {
	Level      string `yaml:"level,omitempty"`
	Format     string `yaml:"format,omitempty"`
	Output     string `yaml:"output,omitempty"`
	File       string `yaml:"file,omitempty"`
	MaxSize    int    `yaml:"maxSize,omitempty"`
	MaxBackups int    `yaml:"maxBackups,omitempty"`
	MaxAge     int    `yaml:"maxAge,omitempty"`
	Compress   bool   `yaml:"compress,omitempty"`
}

func (c canonicalLogConfig) isZero() bool {
	return c == (canonicalLogConfig{})
}

func (c canonicalLogConfig) toCommon() common.LogConfig {
	return common.LogConfig{
		Level:      c.Level,
		Format:     c.Format,
		Output:     c.Output,
		File:       c.File,
		MaxSize:    c.MaxSize,
		MaxBackups: c.MaxBackups,
		MaxAge:     c.MaxAge,
		Compress:   c.Compress,
	}
}

type legacyLogConfig struct {
	Level      string `yaml:"Level,omitempty"`
	Format     string `yaml:"Format,omitempty"`
	Output     string `yaml:"Output,omitempty"`
	File       string `yaml:"File,omitempty"`
	MaxSize    int    `yaml:"MaxSize,omitempty"`
	MaxBackups int    `yaml:"MaxBackups,omitempty"`
	MaxAge     int    `yaml:"MaxAge,omitempty"`
	Compress   bool   `yaml:"Compress,omitempty"`
}

func (c legacyLogConfig) isZero() bool {
	return c == (legacyLogConfig{})
}

func (c legacyLogConfig) toCommon() common.LogConfig {
	return common.LogConfig{
		Level:      c.Level,
		Format:     c.Format,
		Output:     c.Output,
		File:       c.File,
		MaxSize:    c.MaxSize,
		MaxBackups: c.MaxBackups,
		MaxAge:     c.MaxAge,
		Compress:   c.Compress,
	}
}
