# 运维核心 API

> **状态**：Compatibility — 拆分/兼容参考，不是 canonical。运维域当前主入口请用 [运维 API](./ops.md)，本页不能新增独立语义。

### 1. "获取运维配置"

1. route definition

- Url: /api/v1/ops/config
- Method: GET
- Request: `OpsConfigRequest`
- Response: `OpsConfigResponse`

2. request definition



```go
type OpsConfigRequest struct {
}
```


3. response definition



```go
type OpsConfigResponse struct {
	MaintenanceMode bool `json:"maintenanceMode"`
	HealthCheck interface{} `json:"healthCheck"`
	Notifications interface{} `json:"notifications"`
}

type OpsConfig struct {
	MaintenanceMode bool `json:"maintenanceMode"`
	HealthCheck interface{} `json:"healthCheck"`
	Notifications interface{} `json:"notifications"`
}
```

### 2. "获取函数列表"

1. route definition

- Url: /api/v1/ops/functions
- Method: GET
- Request: `OpsFunctionsRequest`
- Response: `OpsFunctionsResponse`

2. request definition



```go
type OpsFunctionsRequest struct {
	GameId string `form:"gameId,optional"`
}
```


3. response definition



```go
type OpsFunctionsResponse struct {
	Items []OpsFunction `json:"items"`
}
```

### 3. "获取健康状态"

1. route definition

- Url: /api/v1/ops/health
- Method: GET
- Request: `OpsHealthGetRequest`
- Response: `OpsHealthResponse`

2. request definition



```go
type OpsHealthGetRequest struct {
}
```


3. response definition



```go
type OpsHealthResponse struct {
	Status string `json:"status"`
	Checks interface{} `json:"checks"`
}
```

### 4. "更新健康检查配置"

1. route definition

- Url: /api/v1/ops/health
- Method: PUT
- Request: `OpsHealthUpdateRequest`
- Response: `OpsHealthResponse`

2. request definition



```go
type OpsHealthUpdateRequest struct {
	Config interface{} `json:"config"`
}
```


3. response definition



```go
type OpsHealthResponse struct {
	Status string `json:"status"`
	Checks interface{} `json:"checks"`
}
```

### 5. "运行健康检查"

1. route definition

- Url: /api/v1/ops/health/run
- Method: POST
- Request: `OpsHealthRunRequest`
- Response: `OpsHealthResponse`

2. request definition



```go
type OpsHealthRunRequest struct {
}
```


3. response definition



```go
type OpsHealthResponse struct {
	Status string `json:"status"`
	Checks interface{} `json:"checks"`
}
```

### 6. "获取维护模式状态"

1. route definition

- Url: /api/v1/ops/maintenance
- Method: GET
- Request: `OpsMaintenanceGetRequest`
- Response: `OpsMaintenanceResponse`

2. request definition



```go
type OpsMaintenanceGetRequest struct {
}
```


3. response definition



```go
type OpsMaintenanceResponse struct {
	Enabled bool `json:"enabled"`
	Reason string `json:"reason"`
	StartAt string `json:"startAt"`
	EndAt string `json:"endAt"`
}
```

### 7. "更新维护模式"

1. route definition

- Url: /api/v1/ops/maintenance
- Method: PUT
- Request: `OpsMaintenanceUpdateRequest`
- Response: `OpsMaintenanceResponse`

2. request definition



```go
type OpsMaintenanceUpdateRequest struct {
	Enabled bool `json:"enabled"`
	Reason string `json:"reason,optional"`
	EndAt string `json:"endAt,optional"`
}
```


3. response definition



```go
type OpsMaintenanceResponse struct {
	Enabled bool `json:"enabled"`
	Reason string `json:"reason"`
	StartAt string `json:"startAt"`
	EndAt string `json:"endAt"`
}
```

### 8. "获取系统指标"

1. route definition

- Url: /api/v1/ops/metrics
- Method: GET
- Request: `OpsMetricsRequest`
- Response: `OpsMetricsResponse`

2. request definition



```go
type OpsMetricsRequest struct {
	From string `form:"from,optional"`
	To string `form:"to,optional"`
}
```


3. response definition



```go
type OpsMetricsResponse struct {
	CPU interface{} `json:"cpu"`
	Memory interface{} `json:"memory"`
	QPS interface{} `json:"qps"`
}
```

### 9. "获取消息队列状态"

1. route definition

- Url: /api/v1/ops/mq
- Method: GET
- Request: `OpsMQRequest`
- Response: `OpsMQResponse`

2. request definition



```go
type OpsMQRequest struct {
}
```


3. response definition



```go
type OpsMQResponse struct {
	Status interface{} `json:"status"`
}
```

### 10. "获取通知配置"

1. route definition

- Url: /api/v1/ops/notifications
- Method: GET
- Request: `OpsNotificationsGetRequest`
- Response: `OpsNotificationsResponse`

2. request definition



```go
type OpsNotificationsGetRequest struct {
}
```


3. response definition



```go
type OpsNotificationsResponse struct {
	Email interface{} `json:"email"`
	Webhook interface{} `json:"webhook"`
	Slack interface{} `json:"slack"`
}
```

### 11. "更新通知配置"

1. route definition

- Url: /api/v1/ops/notifications
- Method: PUT
- Request: `OpsNotificationsUpdateRequest`
- Response: `OpsNotificationsResponse`

2. request definition



```go
type OpsNotificationsUpdateRequest struct {
	Email interface{} `json:"email,optional"`
	Webhook interface{} `json:"webhook,optional"`
	Slack interface{} `json:"slack,optional"`
}
```


3. response definition



```go
type OpsNotificationsResponse struct {
	Email interface{} `json:"email"`
	Webhook interface{} `json:"webhook"`
	Slack interface{} `json:"slack"`
}
```

### 12. "获取服务列表"

1. route definition

- Url: /api/v1/ops/services
- Method: GET
- Request: `OpsServicesRequest`
- Response: `OpsServicesResponse`

2. request definition



```go
type OpsServicesRequest struct {
}
```


3. response definition



```go
type OpsServicesResponse struct {
	Items []OpsService `json:"items"`
}
```

