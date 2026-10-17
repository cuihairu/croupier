# Agent API

### 1. "获取分析过滤器"

1. route definition

- Url: /api/v1/agent/analytics-filters
- Method: GET
- Request: `AnalyticsFiltersQuery`
- Response: `AgentAnalyticsFiltersResponse`

2. request definition

```go
type AnalyticsFiltersQuery struct {
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 2. "上报代理元数据"

1. route definition

- Url: /api/v1/agent/meta
- Method: POST
- Request: `AgentMetaReportRequest`
- Response: `AgentMetaResponse`

2. request definition

```go
type AgentMetaReportRequest struct {
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```
