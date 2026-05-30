# 留存分析 API

### 1. "获取关卡分析"

1. route definition

- Url: /api/v1/analytics/levels
- Method: GET
- Request: `LevelsRequest`
- Response: `LevelsResponse`

2. request definition



```go
type LevelsRequest struct {
	GameId string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	StartDate string `form:"startDate,optional"`
	EndDate string `form:"endDate,optional"`
}
```


3. response definition



```go
type LevelsResponse struct {
	Levels []LevelMetrics `json:"levels"`
}
```

### 2. "获取章节分析"

1. route definition

- Url: /api/v1/analytics/levels/episodes
- Method: GET
- Request: `LevelsEpisodesRequest`
- Response: `LevelsEpisodesResponse`

2. request definition



```go
type LevelsEpisodesRequest struct {
	GameId string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	StartDate string `form:"startDate,optional"`
	EndDate string `form:"endDate,optional"`
}
```


3. response definition



```go
type LevelsEpisodesResponse struct {
	Episodes []EpisodeMetrics `json:"episodes"`
}
```

### 3. "获取地图分析"

1. route definition

- Url: /api/v1/analytics/levels/maps
- Method: GET
- Request: `LevelsMapsRequest`
- Response: `LevelsMapsResponse`

2. request definition



```go
type LevelsMapsRequest struct {
	GameId string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	StartDate string `form:"startDate,optional"`
	EndDate string `form:"endDate,optional"`
}
```


3. response definition



```go
type LevelsMapsResponse struct {
	Maps []MapMetrics `json:"maps"`
}
```

### 4. "获取留存分析"

1. route definition

- Url: /api/v1/analytics/retention
- Method: GET
- Request: `RetentionRequest`
- Response: `RetentionResponse`

2. request definition



```go
type RetentionRequest struct {
	GameId string `form:"gameId,optional"`
	Env string `form:"env,optional"`
	Cohort string `form:"cohort,optional"` // daily, weekly, monthly
	StartDate string `form:"startDate,optional"`
	EndDate string `form:"endDate,optional"`
}
```


3. response definition



```go
type RetentionResponse struct {
	Cohorts []RetentionCohort `json:"cohorts"`
}
```

