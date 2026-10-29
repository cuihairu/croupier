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

// ReadinessRequest readiness probe request
type ReadinessRequest struct{}

// ReadinessResponse is the readiness probe payload (ARCH-004). ready=false
// maps to HTTP 503. degraded=true means the deployment required cluster
// (cluster.enabled) but its dependencies (coordination store / interconnect
// listener / lease) are unavailable AND cluster.allowStandaloneDegradation
// is explicitly set — the instance keeps serving standalone and the probe
// reports degraded instead of failing.
type ReadinessResponse struct {
	Ready     bool     `json:"ready"`
	Degraded  bool     `json:"degraded,omitempty"`
	Reasons   []string `json:"reasons,omitempty"`
	Timestamp string   `json:"timestamp"`
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
