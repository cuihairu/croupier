# 告警 API

### 1. "获取告警列表"

1. route definition

- Url: /api/v1/alerts
- Method: GET
- Request: `AlertsListRequest`
- Response: `AlertsListResponse`

2. request definition



```golang
type AlertsListRequest struct {
	Page int `form:"page,optional,default=1"`
	PageSize int `form:"pageSize,optional,default=20"`
	Level string `form:"level,optional"`
	Status string `form:"status,optional"`
}
```


3. response definition



```golang
type AlertsListResponse struct {
	Items []Alert `json:"items"`
	Total int64 `json:"total"`
	Page int `json:"page"`
	Size int `json:"pageSize"`
}
```

### 2. "静默告警"

1. route definition

- Url: /api/v1/alerts/:id/silence
- Method: POST
- Request: `AlertSilenceRequest`
- Response: `-`

2. request definition



```golang
type AlertSilenceRequest struct {
	ID string `path:"id"`
	Duration int `json:"duration"` // 分钟
	Reason string `json:"reason,optional"`
}
```


3. response definition


### 3. "获取静默规则列表"

1. route definition

- Url: /api/v1/alerts/silences
- Method: GET
- Request: `SilencesListRequest`
- Response: `SilencesListResponse`

2. request definition



```golang
type SilencesListRequest struct {
}
```


3. response definition



```golang
type SilencesListResponse struct {
	Items []Silence `json:"items"`
}
```

### 4. "删除静默规则"

1. route definition

- Url: /api/v1/alerts/silences/:id
- Method: DELETE
- Request: `SilenceDeleteRequest`
- Response: `-`

2. request definition



```golang
type SilenceDeleteRequest struct {
	ID string `path:"id"`
}
```


3. response definition


