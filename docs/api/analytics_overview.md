# 分析概览 API

### 1. "获取分析过滤器"

1. route definition

- Url: /api/v1/analytics/filters
- Method: GET
- Request: `FiltersGetRequest`
- Response: `FiltersGetResponse`

2. request definition



```golang
type FiltersGetRequest struct {
	GameId string `form:"gameId,optional"`
}
```


3. response definition



```golang
type FiltersGetResponse struct {
	Items []AnalyticsFilters `json:"items"`
}
```

### 2. "更新分析过滤器"

1. route definition

- Url: /api/v1/analytics/filters
- Method: PUT
- Request: `FiltersUpdateRequest`
- Response: `FiltersGetResponse`

2. request definition



```golang
type FiltersUpdateRequest struct {
	GameId string `json:"gameId"`
	Filters interface{} `json:"filters"`
}
```


3. response definition



```golang
type FiltersGetResponse struct {
	Items []AnalyticsFilters `json:"items"`
}
```

### 3. "采集分析数据"

1. route definition

- Url: /api/v1/analytics/ingest
- Method: POST
- Request: `IngestRequest`
- Response: `IngestResponse`

2. request definition



```golang
type IngestRequest struct {
	GameId string `json:"gameId"`
	Env string `json:"env"`
	Events interface{} `json:"events"`
	Timestamp string `json:"timestamp,optional"`
}
```


3. response definition



```golang
type IngestResponse struct {
	Accepted int `json:"accepted"`
	Rejected int `json:"rejected"`
	BatchId string `json:"batchId"`
}
```

### 4. "获取分析概览"

1. route definition

- Url: /api/v1/analytics/overview
- Method: GET
- Request: `OverviewRequest`
- Response: `OverviewResponse`

2. request definition



```golang
type OverviewRequest struct {
	GameId string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	StartDate string `form:"startDate,optional"`
	EndDate string `form:"endDate,optional"`
}

type AnalyticsQuery struct {
	GameId string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	StartDate string `form:"startDate,optional"`
	EndDate string `form:"endDate,optional"`
}
```


3. response definition



```golang
type OverviewResponse struct {
	Metrics OverviewMetrics `json:"metrics"`
	Trends interface{} `json:"trends"`
}

type OverviewMetrics struct {
	DAU int `json:"dau"`
	MAU int `json:"mau"`
	NewUsers int `json:"newUsers"`
	Revenue float64 `json:"revenue"`
	ARPU float64 `json:"arpu"`
	ARPPU float64 `json:"arppu"`
	PayingRate float64 `json:"payingRate"`
}
```

### 5. "获取实时数据"

1. route definition

- Url: /api/v1/analytics/realtime
- Method: GET
- Request: `RealtimeRequest`
- Response: `RealtimeResponse`

2. request definition



```golang
type RealtimeRequest struct {
	GameId string `form:"gameId,optional"`
	Env string `form:"env,optional"`
}
```


3. response definition



```golang
type RealtimeResponse struct {
	OnlineUsers int `json:"onlineUsers"`
	ActiveSessions int `json:"activeSessions"`
	QPS float64 `json:"qps"`
	AvgLatency float64 `json:"avgLatency"`
	ErrorRate float64 `json:"errorRate"`
	TopEvents interface{} `json:"topEvents"`
	Timestamp string `json:"timestamp"`
}

type RealtimeMetrics struct {
	OnlineUsers int `json:"onlineUsers"`
	ActiveSessions int `json:"activeSessions"`
	QPS float64 `json:"qps"`
	AvgLatency float64 `json:"avgLatency"`
	ErrorRate float64 `json:"errorRate"`
	TopEvents interface{} `json:"topEvents"`
}
```

### 6. "获取实时序列数据"

1. route definition

- Url: /api/v1/analytics/realtime/series
- Method: GET
- Request: `RealtimeSeriesRequest`
- Response: `RealtimeSeriesResponse`

2. request definition



```golang
type RealtimeSeriesRequest struct {
	GameId string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	Interval string `form:"interval,optional,default=1m"` // 1m, 5m, 15m
	Duration int `form:"duration,optional,default=60"` // 分钟
}
```


3. response definition



```golang
type RealtimeSeriesResponse struct {
	Series interface{} `json:"series"`
}
```

