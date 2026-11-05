package otelrec

// TelemetryConfig 是 agent 的 OTel 开关段（嵌入各 agent config 的
// `telemetry:` 段，命名 lowerCamelCase 契约）。
//
// 语义与 server 遥测一致：Enabled=false（缺省）时 cmd 入口不建任何
// exporter / 周期导出任务，调用点走 NoopRecorder（零分配直通）；Enabled=true
// 且 no_otel 裁剪构建时 cmd 入口 fail-closed 报错（不静默无导出）。
//
// OTLPEndpoint 为空且 Enabled=true 时视为「未配置采集端」：不向默认远端
// 导出，仅告警（与 server 的 warnCollectorMissing 同语义）。
type TelemetryConfig struct {
	// Enabled 总开关（缺省 false）。
	Enabled bool `yaml:"enabled" json:"enabled"`
	// OTLPEndpoint OTLP HTTP 采集端（含 scheme，如 http://collector:4318）。
	OTLPEndpoint string `yaml:"otlpEndpoint" json:"otlpEndpoint"`
	// Insecure 使用 http（false=https/grpc 安全）。与 server telemetry.useTls 同语义。
	Insecure bool `yaml:"insecure" json:"insecure"`
	// ServiceName 覆盖资源 service.name；空=按 agent role 派生。
	ServiceName string `yaml:"serviceName" json:"serviceName"`
}
