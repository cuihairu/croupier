# 监控 API

### 1. "健康检查"

1. route definition

- Url: /healthz（根路径，探针/compose healthcheck 用）
- Url: /api/v1/monitoring/healthz（同语义，网关内路径）
- Method: GET
- Request: `HealthzRequest`
- Response: `HealthzResponse`

2. request definition

```go
type HealthzRequest struct {
}
```

3. response definition

```go
// 实际响应（裸 payload，无 envelope）：
// { "ok": true, "timestamp": "...", "uptimeSeconds": 123, "components": {...} }
type HealthzResponse struct {
	OK            bool                `json:"ok"`
	Timestamp     string              `json:"timestamp"`
	UptimeSeconds int64               `json:"uptimeSeconds"`
	Components    MonitoringComponents `json:"components"`
}
```

### 2. "获取系统指标"

1. route definition

- Url: /api/v1/monitoring/metrics
- Method: GET
- Request: `MetricsRequest`
- Response: `MetricsResponse`

2. request definition

```go
type MetricsRequest struct {
}
```

3. response definition

```go
// 实际响应为裸 payload（系统指标键值），无 code/message envelope。
```

### 3. "获取系统状态"

1. route definition

- Url: /api/v1/monitoring/status
- Method: GET
- Request: `StatusRequest`
- Response: `StatusResponse`

2. request definition

```go
type StatusRequest struct {
}
```

3. response definition

```go
// 实际响应为裸 payload（服务/组件状态键值），无 code/message envelope。
```
