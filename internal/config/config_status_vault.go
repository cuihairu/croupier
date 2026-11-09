package config

import (
	"github.com/cuihairu/croupier/core/healthprobe"
)

// ServerStatusConfig 服务器维护状态 Provider 配置（server-status-provider
// 设计 §3.2）。零配置（缺省）= gate 整体旁路，行为与现状一致。
type ServerStatusConfig struct {
	// Enabled 缺省 false：关=gate 整体旁路（全部照报，行为与现状一致）。
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// Provider 状态源注册表键（noop | atlas | http）。空缺省 atlas（批 S4：
	// 启用时选源零配置，只需 providers.atlas.baseUrl）；noop=无维护信息
	// gate 直通。
	Provider string `json:"provider,omitempty" yaml:"provider,omitempty"`
	// MatchBy 服务器定位匹配键（agentId | host | ip；缺省 agentId）。
	MatchBy string `json:"matchBy,omitempty" yaml:"matchBy,omitempty"`
	// CacheTtlSeconds 状态查询缓存 TTL 秒数；<=0 取默认 60。
	CacheTtlSeconds int `json:"cacheTtlSeconds,omitempty" yaml:"cacheTtlSeconds,omitempty"`
	// Providers 各状态源实现专属配置（key=注册表键）。
	Providers map[string]ServerStatusSourceConfig `json:"providers,omitempty" yaml:"providers,omitempty"`
}

// ServerStatusSourceConfig 单个状态源实现的专属配置。
type ServerStatusSourceConfig struct {
	// BaseURL 状态源服务地址（atlas 等真实源必填）。
	BaseURL string `json:"baseUrl,omitempty" yaml:"baseUrl,omitempty"`
	// TokenEnv 持 API token 的环境变量名（凭证走环境变量引用，不落配置
	// 文件——herald tokenEnv 同款口径）。
	TokenEnv string `json:"tokenEnv,omitempty" yaml:"tokenEnv,omitempty"`
	// TimeoutMs 单次查询超时毫秒；<=0 取实现默认 3000。
	TimeoutMs int `json:"timeoutMs,omitempty" yaml:"timeoutMs,omitempty"`
	// StatusPath 状态查询路径模板（REST 源通用；atlas 默认
	// /v1/discovery/servers/{id}，http 默认空=baseUrl 即端点；
	// {id} 占位符=路径参数替换）。
	StatusPath string `json:"statusPath,omitempty" yaml:"statusPath,omitempty"`
	// MatchKey 定位参数名（REST 源通用；默认 agentId）。
	MatchKey string `json:"matchKey,omitempty" yaml:"matchKey,omitempty"`
	// InMaintenanceStatus 状态枚举值口径（REST 源通用）：映射到的
	// inMaintenance 字段不是 bool 时按此字符串判等；atlas 默认
	// "maintenance"，http 默认 "true"。
	InMaintenanceStatus string `json:"inMaintenanceStatus,omitempty" yaml:"inMaintenanceStatus,omitempty"`
	// FieldMapping 响应字段映射（REST 源通用：契约字段名→状态源 JSON 键；
	// 配置驱动，不写死供应商字段名——alertInbound labelMapping 同款）。
	FieldMapping map[string]string `json:"fieldMapping,omitempty" yaml:"fieldMapping,omitempty"`
}

// VaultConfig 密码箱配置（vault-provider 设计 §3，todo 4249/4252）。
// 主口令与凭据值本身不落配置文件——口令走环境变量（passphraseEnv），
// env 提供商凭据走带前缀环境变量。
type VaultConfig struct {
	// Enabled 缺省 false：关=密码箱不装配（ctx.Vault 为 nil），各接入点
	// 行为与现状一致。
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// Provider 提供商注册表键（local | env）。空缺省 local。
	Provider string `json:"provider,omitempty" yaml:"provider,omitempty"`
	// SessionTtlSeconds 解锁会话空闲超时秒数（local；滑动续期，<=0 取
	// 实现默认 900）。
	SessionTtlSeconds int `json:"sessionTtlSeconds,omitempty" yaml:"sessionTtlSeconds,omitempty"`
	// Providers 各提供商实现专属配置（key=注册表键，取
	// Providers[Provider] 装配）。
	Providers map[string]VaultSourceConfig `json:"providers,omitempty" yaml:"providers,omitempty"`
}

// ServerProbeConfig server `probe:` 段（agent-health-recovery-review §4.2/
// §4.3）：目标契约与 agent probe.targets 同一份（core/healthprobe.TargetConfig，
// 键一律小驼峰）。只声明「探什么」，阈值/语义/间隔由装配侧给显式默认
// （failThreshold=3、recoverThreshold=1、semantics=readiness、interval=30s）。
type ServerProbeConfig struct {
	// Targets 目标列表；重名/未知类型装配整体报错（启动失败，与 agent 同口径）。
	Targets []healthprobe.TargetConfig `json:"targets,omitempty" yaml:"targets,omitempty"`
	// MaxDetailsPerWindow 每故障窗口每目标的失败明细条数上限（<=0 用默认 10）。
	MaxDetailsPerWindow int `json:"maxDetailsPerWindow,omitempty" yaml:"maxDetailsPerWindow,omitempty"`
}

// VaultSourceConfig 单个提供商实现的专属配置。
type VaultSourceConfig struct {
	// Path local：加密存储文件路径（缺省 data/vault.json；目录 0700、
	// 文件 0600）。
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
	// PassphraseEnv local：持主口令的环境变量名（缺省
	// CROUPIER_VAULT_PASSPHRASE；口令不落配置文件、不落日志）。
	PassphraseEnv string `json:"passphraseEnv,omitempty" yaml:"passphraseEnv,omitempty"`
	// Prefix env：环境变量名前缀（缺省 CROUPIER_VAULT_ENV_；凭据路径
	// smtp/password → CROUPIER_VAULT_ENV_SMTP_PASSWORD）。
	Prefix string `json:"prefix,omitempty" yaml:"prefix,omitempty"`
}
