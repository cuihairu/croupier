# 分配 API

### 1. "获取分配列表"

1. route definition

- Url: /api/v1/assignments
- Method: GET
- Request: `AssignmentsListRequest`
- Response: `AssignmentsListResponse`

2. request definition



```go
type AssignmentsListRequest struct {
	Page int `form:"page,optional"`
	PageSize int `form:"pageSize,optional"`
	GameId string `form:"game_id,optional"`
	Env string `form:"env,optional"`
}
```


3. response definition



```go
type AssignmentsListResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 2. "更新分配"

1. route definition

- Url: /api/v1/assignments
- Method: PUT
- Request: `AssignmentsUpdateRequest`
- Response: `AssignmentsUpdateResponse`

2. request definition



```go
type AssignmentsUpdateRequest struct {
	GameId string `json:"game_id"`
	Env string `json:"env,optional"`
	Functions []string `json:"functions"`
}
```


3. response definition



```go
type AssignmentsUpdateResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

