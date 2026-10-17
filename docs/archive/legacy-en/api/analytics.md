# 数据分析 API

### 1. "获取行为分析"

1. route definition

- Url: /api/v1/analytics/behavior
- Method: GET
- Request: `AnalyticsBehaviorRequest`
- Response: `AnalyticsBehaviorResponse`

2. request definition

```go
type AnalyticsBehaviorRequest struct {
	GameID string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	StartDate string `form:"startDate,optional"`
	EndDate string `form:"endDate,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 2. "获取功能采用率"

1. route definition

- Url: /api/v1/analytics/behavior/adoption
- Method: GET
- Request: `AnalyticsBehaviorAdoptionRequest`
- Response: `AnalyticsBehaviorAdoptionResponse`

2. request definition

```go
type AnalyticsBehaviorAdoptionRequest struct {
	GameID string `form:"gameId,optional"`
	Env string `form:"env,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 3. "获取功能采用率明细"

1. route definition

- Url: /api/v1/analytics/behavior/adoption/breakdown
- Method: GET
- Request: `AnalyticsBehaviorAdoptionBreakdownRequest`
- Response: `AnalyticsBehaviorAdoptionBreakdownResponse`

2. request definition

```go
type AnalyticsBehaviorAdoptionBreakdownRequest struct {
	GameID string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	Feature string `form:"feature,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 4. "获取行为事件"

1. route definition

- Url: /api/v1/analytics/behavior/events
- Method: GET
- Request: `AnalyticsBehaviorEventsRequest`
- Response: `AnalyticsBehaviorEventsResponse`

2. request definition

```go
type AnalyticsBehaviorEventsRequest struct {
	GameID string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	EventType string `form:"eventType,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 5. "获取行为漏斗"

1. route definition

- Url: /api/v1/analytics/behavior/funnel
- Method: GET
- Request: `AnalyticsBehaviorFunnelRequest`
- Response: `AnalyticsBehaviorFunnelResponse`

2. request definition

```go
type AnalyticsBehaviorFunnelRequest struct {
	GameID string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	Steps []string `form:"steps,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 6. "获取行为路径"

1. route definition

- Url: /api/v1/analytics/behavior/paths
- Method: GET
- Request: `AnalyticsBehaviorPathsRequest`
- Response: `AnalyticsBehaviorPathsResponse`

2. request definition

```go
type AnalyticsBehaviorPathsRequest struct {
	GameID string `form:"gameId,optional"`
	Env string `form:"env,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 7. "获取分析过滤器"

1. route definition

- Url: /api/v1/analytics/filters
- Method: GET
- Request: `AnalyticsFiltersGetRequest`
- Response: `AnalyticsFiltersGetResponse`

2. request definition

```go
type AnalyticsFiltersGetRequest struct {
	GameID string `form:"gameId,optional"`
	Env string `form:"env,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 8. "更新分析过滤器"

1. route definition

- Url: /api/v1/analytics/filters
- Method: PUT
- Request: `AnalyticsFiltersUpdateRequest`
- Response: `AnalyticsFiltersUpdateResponse`

2. request definition

```go
type AnalyticsFiltersUpdateRequest struct {
	GameID string `json:"gameId"`
	Env string `json:"env"`
	Events []string `json:"events,optional"`
	PaymentsEnabled bool `json:"paymentsEnabled,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 9. "采集分析数据"

1. route definition

- Url: /api/v1/analytics/ingest
- Method: POST
- Request: `AnalyticsIngestRequest`
- Response: `AnalyticsIngestResponse`

2. request definition

```go
type AnalyticsIngestRequest struct {
	GameID string `json:"gameId"`
	Env string `json:"env"`
	Events []interface{} `json:"events"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 10. "获取等级分析"

1. route definition

- Url: /api/v1/analytics/levels
- Method: GET
- Request: `AnalyticsLevelsRequest`
- Response: `AnalyticsLevelsResponse`

2. request definition

```go
type AnalyticsLevelsRequest struct {
	GameID string `form:"gameId,optional"`
	Env string `form:"env,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 11. "获取关卡分析"

1. route definition

- Url: /api/v1/analytics/levels/episodes
- Method: GET
- Request: `AnalyticsLevelsEpisodesRequest`
- Response: `AnalyticsLevelsEpisodesResponse`

2. request definition

```go
type AnalyticsLevelsEpisodesRequest struct {
	GameID string `form:"gameId,optional"`
	Env string `form:"env,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 12. "获取地图分析"

1. route definition

- Url: /api/v1/analytics/levels/maps
- Method: GET
- Request: `AnalyticsLevelsMapsRequest`
- Response: `AnalyticsLevelsMapsResponse`

2. request definition

```go
type AnalyticsLevelsMapsRequest struct {
	GameID string `form:"gameId,optional"`
	Env string `form:"env,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 13. "获取分析概览"

1. route definition

- Url: /api/v1/analytics/overview
- Method: GET
- Request: `AnalyticsOverviewRequest`
- Response: `AnalyticsOverviewResponse`

2. request definition

```go
type AnalyticsOverviewRequest struct {
	GameID string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	StartDate string `form:"startDate,optional"`
	EndDate string `form:"endDate,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 14. "获取支付分析"

1. route definition

- Url: /api/v1/analytics/payments
- Method: GET
- Request: `AnalyticsPaymentsRequest`
- Response: `AnalyticsPaymentsResponse`

2. request definition

```go
type AnalyticsPaymentsRequest struct {
	GameID string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	StartDate string `form:"startDate,optional"`
	EndDate string `form:"endDate,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 15. "采集支付数据"

1. route definition

- Url: /api/v1/analytics/payments/ingest
- Method: POST
- Request: `AnalyticsPaymentsIngestRequest`
- Response: `AnalyticsPaymentsIngestResponse`

2. request definition

```go
type AnalyticsPaymentsIngestRequest struct {
	GameID string `json:"gameId"`
	Env string `json:"env"`
	Transactions []interface{} `json:"transactions"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 16. "获取产品趋势"

1. route definition

- Url: /api/v1/analytics/payments/product-trend
- Method: GET
- Request: `AnalyticsPaymentsProductTrendRequest`
- Response: `AnalyticsPaymentsProductTrendResponse`

2. request definition

```go
type AnalyticsPaymentsProductTrendRequest struct {
	GameID string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	ProductID string `form:"productId,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 17. "获取支付摘要"

1. route definition

- Url: /api/v1/analytics/payments/summary
- Method: GET
- Request: `AnalyticsPaymentsSummaryRequest`
- Response: `AnalyticsPaymentsSummaryResponse`

2. request definition

```go
type AnalyticsPaymentsSummaryRequest struct {
	GameID string `form:"gameId,optional"`
	Env string `form:"env,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 18. "获取支付交易列表"

1. route definition

- Url: /api/v1/analytics/payments/transactions
- Method: GET
- Request: `AnalyticsPaymentsTransactionsRequest`
- Response: `AnalyticsPaymentsTransactionsResponse`

2. request definition

```go
type AnalyticsPaymentsTransactionsRequest struct {
	GameID string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	Page int `form:"page,optional"`
	PageSize int `form:"pageSize,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 19. "获取实时数据"

1. route definition

- Url: /api/v1/analytics/realtime
- Method: GET
- Request: `AnalyticsRealtimeRequest`
- Response: `AnalyticsRealtimeResponse`

2. request definition

```go
type AnalyticsRealtimeRequest struct {
	GameID string `form:"gameId,optional"`
	Env string `form:"env,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 20. "获取实时序列数据"

1. route definition

- Url: /api/v1/analytics/realtime/series
- Method: GET
- Request: `AnalyticsRealtimeSeriesRequest`
- Response: `AnalyticsRealtimeSeriesResponse`

2. request definition

```go
type AnalyticsRealtimeSeriesRequest struct {
	GameID string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	Interval string `form:"interval,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 21. "获取留存分析"

1. route definition

- Url: /api/v1/analytics/retention
- Method: GET
- Request: `AnalyticsRetentionRequest`
- Response: `AnalyticsRetentionResponse`

2. request definition

```go
type AnalyticsRetentionRequest struct {
	GameID string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	StartDate string `form:"startDate,optional"`
	EndDate string `form:"endDate,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```
