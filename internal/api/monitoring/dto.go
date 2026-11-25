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

// ReadyRequest is the readiness probe request.
type ReadyRequest struct{}

// ReadyResponse is the readiness probe response (/readyz). The health probe
// contract permits a minimal payload; this additionally carries reasons for
// ops triage (listing the unavailable components when returning 503).
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
