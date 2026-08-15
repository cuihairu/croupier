# 函数调用兼容 API

> **状态**：Compatibility — 兼容历史调用，不是 canonical。当前函数管理请用 [函数 API](./function.md)，任务与调用请用 [任务 API](./task.md)。

本页描述 Dashboard 兼容使用的 `/api/v1/function-calls*` 接口。

这些接口当前复用 `tasks` 能力构建兼容视图，目标是消除 refactor 后的前端 404，并保持基础查询与取消能力可用。

## 1. "函数调用列表"

1. route definition

- Url: /api/v1/function-calls
- Method: GET
- Auth: Bearer Token
- Request: `ListRequest`
- Response: `ListResponse`

2. request definition

```go
type ListRequest struct {
	FunctionID string `form:"function_id"`
	GameID     string `form:"game_id"`
	Env        string `form:"env"`
	Status     string `form:"status"`
	Page       int    `form:"page"`
	PageSize   int    `form:"page_size"`
}
```

3. response definition

```go
type ListResponse struct {
	Calls    []Item `json:"calls"`
	Total    int    `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}
```

## 2. "函数调用详情"

1. route definition

- Url: /api/v1/function-calls/:id
- Method: GET
- Auth: Bearer Token
- Response: `Item`

## 3. "函数调用统计"

1. route definition

- Url: /api/v1/function-calls/stats
- Method: GET
- Auth: Bearer Token
- Response: `StatsResponse`

## 4. "取消函数调用"

1. route definition

- Url: /api/v1/function-calls/:id/cancel
- Method: POST
- Auth: Bearer Token

## 5. "重跑函数调用"

1. route definition

- Url: /api/v1/function-calls/:id/rerun
- Method: POST
- Auth: Bearer Token

## 当前限制

- 当前实现基于 `tasks` 兼容转换，不包含独立的调用历史表。
- `rerun` 暂未实现真实重跑，当前会返回业务错误提示“暂不支持从调用历史重跑”。
- 详情与统计字段会优先复用现有 `tasks` 数据，部分历史字段可能为空。
