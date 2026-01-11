### 1. "任务列表"

1. route definition

- Url: /api/v1/jobs
- Method: GET
- Request: `JobListRequest`
- Response: `JobListResponse`

2. request definition



```golang
type JobListRequest struct {
	Status string `form:"status,optional"`
	FunctionID string `form:"function_id,optional"`
	Actor string `form:"actor,optional"`
	GameID string `form:"game_id,optional"`
	Env string `form:"env,optional"`
	Page int `form:"page,optional,default=1"`
	Size int `form:"size,optional,default=20"`
}
```


3. response definition



```golang
type JobListResponse struct {
	Jobs []JobItem `json:"jobs"`
	Total int `json:"total"`
}
```

### 2. "启动任务"

1. route definition

- Url: /api/v1/jobs
- Method: POST
- Request: `JobStartRequest`
- Response: `JobStartResponse`

2. request definition



```golang
type JobStartRequest struct {
	FunctionID string `json:"functionId"` // 函数ID
	Params interface{} `json:"params,optional"`
}
```


3. response definition



```golang
type JobStartResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 3. "取消任务"

1. route definition

- Url: /api/v1/jobs/:id/cancel
- Method: POST
- Request: `JobCancelRequest`
- Response: `JobCancelResponse`

2. request definition



```golang
type JobCancelRequest struct {
	ID string `path:"id"` // 任务ID
}
```


3. response definition



```golang
type JobCancelResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 4. "获取任务结果"

1. route definition

- Url: /api/v1/jobs/:id/result
- Method: GET
- Request: `JobResultRequest`
- Response: `JobResultResponse`

2. request definition



```golang
type JobResultRequest struct {
	ID string `path:"id"` // 任务ID
}
```


3. response definition



```golang
type JobResultResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 5. "任务流（实时状态和日志）"

1. route definition

- Url: /api/v1/jobs/:jobId/stream
- Method: GET
- Request: `StreamJobRequest`
- Response: `StreamJobResponse`

2. request definition



```golang
type StreamJobRequest struct {
	JobID string `path:"jobId"`
}
```


3. response definition



```golang
type StreamJobResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

