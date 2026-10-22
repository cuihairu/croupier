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
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
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
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```
