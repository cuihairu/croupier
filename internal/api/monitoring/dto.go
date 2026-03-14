package monitoring

// HealthzRequest health check request
type HealthzRequest struct{}

// HealthzResponse health check response
type HealthzResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MetricsRequest metrics request
type MetricsRequest struct{}

// MetricsResponse metrics response
type MetricsResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// StatusRequest status request
type StatusRequest struct{}

// StatusResponse status response
type StatusResponse struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data"`
}
