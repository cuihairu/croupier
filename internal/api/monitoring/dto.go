package monitoring

// HealthzRequest health check request
type HealthzRequest struct{}

// MonitoringComponentStatus describes the status of a monitoring component.
type MonitoringComponentStatus map[string]interface{}

// MonitoringComponents groups the major subsystems reported by healthz.
type MonitoringComponents struct {
	Database MonitoringComponentStatus `json:"database"`
	Registry MonitoringComponentStatus `json:"registry"`
	Ops      MonitoringComponentStatus `json:"ops"`
}

// HealthzResponse matches the REST contract for GET /api/v1/monitoring/healthz.
type HealthzResponse struct {
	OK            bool                 `json:"ok"`
	Timestamp     string               `json:"timestamp"`
	UptimeSeconds int64                `json:"uptimeSeconds"`
	Components    MonitoringComponents `json:"components"`
}

// ReadyRequest readiness probe request。
type ReadyRequest struct{}

// ReadyResponse 就绪探针响应（/readyz）。健康探针契约允许最小负载；
// 这里额外带 reasons 供运维定位（503 时列出不可用组件原因）。
type ReadyResponse struct {
	Status    string                    `json:"status"` // ok | not_ready
	Timestamp string                    `json:"timestamp"`
	Reasons   []string                  `json:"reasons,omitempty"`
	Cluster   MonitoringComponentStatus `json:"cluster,omitempty"`
}

// MetricsRequest metrics request
type MetricsRequest struct{}

// MetricsResponse matches the REST contract for GET /api/v1/monitoring/metrics.
type MetricsResponse struct {
	Timestamp string                    `json:"timestamp"`
	Counts    map[string]interface{}    `json:"counts"`
	Database  MonitoringComponentStatus `json:"database"`
	Registry  map[string]interface{}    `json:"registry"`
	Ops       map[string]interface{}    `json:"ops"`
}

// StatusRequest status request
type StatusRequest struct{}

// StatusResponse matches the REST contract for GET /api/v1/monitoring/status.
type StatusResponse struct {
	OK            bool                      `json:"ok"`
	Timestamp     string                    `json:"timestamp"`
	UptimeSeconds int64                     `json:"uptimeSeconds"`
	Database      MonitoringComponentStatus `json:"database"`
	Registry      MonitoringComponentStatus `json:"registry"`
	Ops           MonitoringComponentStatus `json:"ops"`
	Agents        []map[string]interface{}  `json:"agents"`
}
