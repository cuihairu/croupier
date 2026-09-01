# 任务 API

### 1. "任务列表"

1. route definition

- Url: /api/v1/tasks
- Method: GET
- Request: `TaskListRequest`
- Response: `TaskListResponse`

2. request definition

```go
type TaskListRequest struct {
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

```go
type TaskListResponse struct {
	Items []TaskItem `json:"items"`
	Total int `json:"total"`
}
```

### 2. "启动任务"

1. route definition

- Url: /api/v1/tasks
- Method: POST
- Request: `TaskStartRequest`
- Response: `TaskStartResponse`

2. request definition

```go
type TaskStartRequest struct {
	FunctionID string `json:"functionId"`
	Params interface{} `json:"params,optional"`
}
```

3. response definition

```go
type TaskStartResponse struct {
	TaskID string `json:"taskId"`
	Status string `json:"status"`
}
```

### 3. "取消任务"

1. route definition

- Url: /api/v1/tasks/:id/cancel（路径参数取消）
- Url: /api/v1/tasks/cancel（body 取消：`{ "taskId": "..." }`，OpenAPI 客户端友好）
- Method: POST
- Request: `TaskCancelRequest`

2. request definition

```go
type TaskCancelRequest struct {
	ID string `path:"id"`
}
```

### 4. "获取任务详情"

1. route definition

- Url: /api/v1/tasks/:id
- Method: GET
- Request: `TaskDetailRequest`

2. request definition

```go
type TaskDetailRequest struct {
	ID string `path:"id"`
}
```

### 5. "获取任务事件"

1. route definition

- Url: /api/v1/tasks/:id/events
- Method: GET
- Request: `TaskEventsRequest`

2. request definition

```go
type TaskEventsRequest struct {
	ID string `path:"id"`
	AfterSeq int64 `form:"after_seq,optional"`
}
```

### 兼容说明

- Dashboard 仍会调用 `/api/v1/function-calls*` 读取函数调用历史兼容视图。
- 当前服务端基于 `tasks` 数据构建兼容视图，详见 [function_call.md](./function_call.md)。
