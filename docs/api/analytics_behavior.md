# 行为分析 API

### 1. "获取行为分析"

1. route definition

- Url: /api/v1/analytics/behavior
- Method: GET
- Request: `BehaviorRequest`
- Response: `BehaviorResponse`

2. request definition



```go
type BehaviorRequest struct {
	GameId string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	StartDate string `form:"startDate,optional"`
	EndDate string `form:"endDate,optional"`
}
```


3. response definition



```go
type BehaviorResponse struct {
	TopActions interface{} `json:"topActions"`
	UserFlows interface{} `json:"userFlows"`
	HeatMap interface{} `json:"heatMap"`
}
```

### 2. "获取功能采用率"

1. route definition

- Url: /api/v1/analytics/behavior/adoption
- Method: GET
- Request: `BehaviorAdoptionRequest`
- Response: `BehaviorAdoptionResponse`

2. request definition



```go
type BehaviorAdoptionRequest struct {
	GameId string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	Feature string `form:"feature,optional"`
	StartDate string `form:"startDate,optional"`
	EndDate string `form:"endDate,optional"`
}
```


3. response definition



```go
type BehaviorAdoptionResponse struct {
	Features []FeatureAdoption `json:"features"`
}
```

### 3. "获取采用率明细"

1. route definition

- Url: /api/v1/analytics/behavior/adoption/breakdown
- Method: GET
- Request: `BehaviorAdoptionBreakdownRequest`
- Response: `BehaviorAdoptionBreakdownResponse`

2. request definition



```go
type BehaviorAdoptionBreakdownRequest struct {
	GameId string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	Feature string `form:"feature"`
	StartDate string `form:"startDate,optional"`
	EndDate string `form:"endDate,optional"`
}
```


3. response definition



```go
type BehaviorAdoptionBreakdownResponse struct {
	BySegment interface{} `json:"bySegment"`
	ByTime interface{} `json:"byTime"`
}
```

### 4. "获取行为事件"

1. route definition

- Url: /api/v1/analytics/behavior/events
- Method: GET
- Request: `BehaviorEventsRequest`
- Response: `BehaviorEventsResponse`

2. request definition



```go
type BehaviorEventsRequest struct {
	GameId string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	EventType string `form:"eventType,optional"`
	StartDate string `form:"startDate,optional"`
	EndDate string `form:"endDate,optional"`
	Limit int `form:"limit,optional,default=100"`
}
```


3. response definition



```go
type BehaviorEventsResponse struct {
	Items []BehaviorEvent `json:"items"`
	Total int64 `json:"total"`
}
```

### 5. "获取行为漏斗"

1. route definition

- Url: /api/v1/analytics/behavior/funnel
- Method: POST
- Request: `BehaviorFunnelRequest`
- Response: `BehaviorFunnelResponse`

2. request definition



```go
type BehaviorFunnelRequest struct {
	GameId string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	Steps []string `json:"steps"`
	StartDate string `form:"startDate,optional"`
	EndDate string `form:"endDate,optional"`
}
```


3. response definition



```go
type BehaviorFunnelResponse struct {
	Steps []FunnelStep `json:"steps"`
}
```

### 6. "获取行为路径"

1. route definition

- Url: /api/v1/analytics/behavior/paths
- Method: GET
- Request: `BehaviorPathsRequest`
- Response: `BehaviorPathsResponse`

2. request definition



```go
type BehaviorPathsRequest struct {
	GameId string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	StartDate string `form:"startDate,optional"`
	EndDate string `form:"endDate,optional"`
	Depth int `form:"depth,optional,default=5"`
}
```


3. response definition



```go
type BehaviorPathsResponse struct {
	Paths interface{} `json:"paths"`
}
```

