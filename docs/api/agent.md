# Agent API

### 1. "获取分析过滤器"

1. route definition

- Url: /api/v1/agent/analytics-filters
- Method: GET
- Request: `AnalyticsFiltersQuery`
- Response: `AgentAnalyticsFiltersResponse`

2. request definition



```golang
type AnalyticsFiltersQuery struct {
}
```


3. response definition



```golang
type AgentAnalyticsFiltersResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 2. "上报代理元数据"

1. route definition

- Url: /api/v1/agent/meta
- Method: POST
- Request: `AgentMetaReportRequest`
- Response: `AgentMetaResponse`

2. request definition



```golang
type AgentMetaReportRequest struct {
}
```


3. response definition



```golang
type AgentMetaResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

