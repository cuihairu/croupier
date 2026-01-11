### 1. "健康检查"

1. route definition

- Url: /api/v1/healthz
- Method: GET
- Request: `HealthzRequest`
- Response: `HealthzResponse`

2. request definition



```golang
type HealthzRequest struct {
}
```


3. response definition



```golang
type HealthzResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 2. "获取系统指标"

1. route definition

- Url: /api/v1/metrics
- Method: GET
- Request: `MetricsRequest`
- Response: `MetricsResponse`

2. request definition



```golang
type MetricsRequest struct {
}
```


3. response definition



```golang
type MetricsResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 3. "获取系统状态"

1. route definition

- Url: /api/v1/status
- Method: GET
- Request: `StatusRequest`
- Response: `StatusResponse`

2. request definition



```golang
type StatusRequest struct {
}
```


3. response definition



```golang
type StatusResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

