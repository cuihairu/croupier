package config

import (
	"strings"
)

// FeatureFlagsConfig maps flag names to enabled state. Absent = enabled.
type FeatureFlagsConfig map[string]bool

// Enabled reports whether the named feature is enabled. Unknown and unset
// flags are enabled (fail-open): a typo in config must not silently lock an
// entire domain, and new flags ship on until explicitly disabled.
func (f FeatureFlagsConfig) Enabled(name string) bool {
	if f == nil {
		return true
	}
	if v, ok := f[name]; ok {
		return v
	}
	return true
}

type TelemetryConfig struct {
	Enabled        bool                   `json:"enabled" yaml:"enabled"`
	ServiceName    string                 `json:"serviceName" yaml:"service_name"`
	ServiceVersion string                 `json:"serviceVersion" yaml:"service_version"`
	Environment    string                 `json:"environment" yaml:"environment"`
	CollectorURL   string                 `json:"collectorUrl" yaml:"collector_url"`
	GameID         string                 `json:"gameId" yaml:"game_id"`
	EnableTracing  bool                   `json:"enableTracing" yaml:"enable_tracing"`
	EnableMetrics  bool                   `json:"enableMetrics" yaml:"enable_metrics"`
	SamplingRatio  float64                `json:"samplingRatio" yaml:"sampling_ratio"`
	UseTLS         bool                   `json:"useTls" yaml:"use_tls"`
	Headers        string                 `json:"headers" yaml:"headers"`
	Analytics      TelemetryAnalyticsConf `json:"analytics" yaml:"analytics"`
	// Prometheus 暴露标准 exposition 端点（默认关闭；开启后无需部署
	// OTel Collector 即可被 Prometheus 抓取）。与 OTLP 推送链路互不影响。
	Prometheus TelemetryPrometheusConfig `json:"prometheus" yaml:"prometheus"`
}

// TelemetryPrometheusConfig 控制 Prometheus exposition 端点。
type TelemetryPrometheusConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Path 是 exposition 挂载路径；默认 /metrics/prometheus（避开既有
	// JSON /metrics API）。
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
}

// PrometheusPath 返回应用默认值后的挂载路径。
func (c TelemetryPrometheusConfig) PrometheusPath() string {
	if strings.TrimSpace(c.Path) == "" {
		return "/metrics/prometheus"
	}
	return c.Path
}

type TelemetryAnalyticsConf struct {
	Enabled        bool   `json:"enabled" yaml:"enabled"`
	RedisAddr      string `json:"redisAddr" yaml:"redis_addr"`
	RedisPassword  string `json:"redisPassword" yaml:"redis_password"`
	RedisDB        int    `json:"redisDb" yaml:"redis_db"`
	TopicPrefix    string `json:"topicPrefix" yaml:"topic_prefix"`
	RetentionHours int    `json:"retentionHours" yaml:"retention_hours"`
	BatchSize      int    `json:"batchSize" yaml:"batch_size"`
	FlushInterval  string `json:"flushInterval" yaml:"flush_interval"`
}
