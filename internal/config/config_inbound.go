package config

// AlertInboundConfig 告警入站 webhook 配置（plugin-mechanism 设计 §5.3/M3）。
// 来源闭集起步 alertmanager / generic；每来源独立密钥与映射。
type AlertInboundConfig struct {
	// Sources 来源配置表（key=来源名，闭集 alertmanager/generic）。
	Sources map[string]AlertInboundSourceConfig `json:"sources,omitempty" yaml:"sources,omitempty"`
}

// AlertInboundSourceConfig 单个入站来源配置。
type AlertInboundSourceConfig struct {
	// SecretEnv 持 HMAC 共享密钥的环境变量名（凭证走环境变量引用，
	// 不落配置文件；空=该来源未启用）。
	SecretEnv string `json:"secretEnv,omitempty" yaml:"secretEnv,omitempty"`
	// LabelMapping alertmanager 来源的 labels 键→信封字段名映射
	//（配置驱动，不写死供应商字段名；generic 来源直接用 §5.2 信封，
	// 不需要此映射）。
	LabelMapping map[string]string `json:"labelMapping,omitempty" yaml:"labelMapping,omitempty"`
}

// ExternalAPIConfig 对外 REST API 配置（incident-reports §9）。
type ExternalAPIConfig struct {
	// RateLimitPerMinute 按令牌名分桶的滑动窗口限流上限；<=0 取默认 30。
	RateLimitPerMinute int `json:"rateLimitPerMinute,omitempty" yaml:"rateLimitPerMinute,omitempty"`
}
