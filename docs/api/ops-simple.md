# 运维简化 API

> **状态**：Compatibility — 拆分/兼容参考，不是 canonical。运维域当前主入口请用 [运维 API](./ops.md)，本页不能新增独立语义。

### 1. "更新代理元数据"

1. route definition

- Url: /api/v1/ops/agent-meta
- Method: PUT
- Request: `OpsAgentMetaUpdateRequest`
- Response: `OpsAgentMetaResponse`

2. request definition

```go
type OpsAgentMetaUpdateRequest struct {
	AgentID string `json:"agentId"`
	Meta interface{} `json:"meta"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 2. "获取告警列表"

1. route definition

- Url: /api/v1/ops/alerts
- Method: GET
- Request: `OpsAlertsRequest`
- Response: `OpsAlertsResponse`

2. request definition

```go
type OpsAlertsRequest struct {
}
```

3. response definition

```go
type OpsAlertsResponse struct {
	Alerts []string `json:"alerts"`
}
```

### 3. "静默告警"

1. route definition

- Url: /api/v1/ops/alerts/silence
- Method: POST
- Request: `OpsAlertSilenceRequest`
- Response: `OpsAlertSilenceResponse`

2. request definition

```go
type OpsAlertSilenceRequest struct {
	AlertID string `json:"alertId"`
	Duration int `json:"duration"` // 静默时长（分钟）
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 4. "创建备份"

1. route definition

- Url: /api/v1/ops/backups
- Method: POST
- Request: `OpsBackupCreateRequest`
- Response: `OpsBackupCreateResponse`

2. request definition

```go
type OpsBackupCreateRequest struct {
	Name string `json:"name,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 5. "获取备份列表"

1. route definition

- Url: /api/v1/ops/backups
- Method: GET
- Request: `OpsBackupsListRequest`
- Response: `OpsBackupsListResponse`

2. request definition

```go
type OpsBackupsListRequest struct {
	Page int `form:"page,optional"`
	PageSize int `form:"pageSize,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 6. "删除备份"

1. route definition

- Url: /api/v1/ops/backups/:id
- Method: DELETE
- Request: `OpsBackupDeleteRequest`
- Response: `OpsBackupDeleteResponse`

2. request definition

```go
type OpsBackupDeleteRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 7. "下载备份"

1. route definition

- Url: /api/v1/ops/backups/:id/download
- Method: GET
- Request: `OpsBackupDownloadRequest`
- Response: `OpsBackupDownloadResponse`

2. request definition

```go
type OpsBackupDownloadRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 8. "获取运维配置"

1. route definition

- Url: /api/v1/ops/config
- Method: GET
- Request: `OpsConfigRequest`
- Response: `OpsConfigResponse`

2. request definition

```go
type OpsConfigRequest struct {
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 9. "获取函数列表"

1. route definition

- Url: /api/v1/ops/functions
- Method: GET
- Request: `OpsFunctionsRequest`
- Response: `OpsFunctionsResponse`

2. request definition

```go
type OpsFunctionsRequest struct {
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 10. "获取健康状态"

1. route definition

- Url: /api/v1/ops/health
- Method: GET
- Request: `OpsHealthGetRequest`
- Response: `OpsHealthGetResponse`

2. request definition

```go
type OpsHealthGetRequest struct {
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 11. "更新健康检查配置"

1. route definition

- Url: /api/v1/ops/health
- Method: PUT
- Request: `OpsHealthUpdateRequest`
- Response: `OpsHealthUpdateResponse`

2. request definition

```go
type OpsHealthUpdateRequest struct {
	Enabled bool `json:"enabled"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 12. "运行健康检查"

1. route definition

- Url: /api/v1/ops/health/run
- Method: POST
- Request: `OpsHealthRunRequest`
- Response: `OpsHealthRunResponse`

2. request definition

```go
type OpsHealthRunRequest struct {
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 13. "获取维护模式状态"

1. route definition

- Url: /api/v1/ops/maintenance
- Method: GET
- Request: `OpsMaintenanceGetRequest`
- Response: `OpsMaintenanceGetResponse`

2. request definition

```go
type OpsMaintenanceGetRequest struct {
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 14. "更新维护模式"

1. route definition

- Url: /api/v1/ops/maintenance
- Method: PUT
- Request: `OpsMaintenanceUpdateRequest`
- Response: `OpsMaintenanceUpdateResponse`

2. request definition

```go
type OpsMaintenanceUpdateRequest struct {
	Enabled bool `json:"enabled"`
	Message string `json:"message,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 15. "获取指标"

1. route definition

- Url: /api/v1/ops/metrics
- Method: GET
- Request: `OpsMetricsQuery`
- Response: `OpsMetricsResponse`

2. request definition

```go
type OpsMetricsQuery struct {
	Start string `form:"start,optional"`
	End string `form:"end,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 16. "获取消息队列状态"

1. route definition

- Url: /api/v1/ops/mq
- Method: GET
- Request: `OpsMQRequest`
- Response: `OpsMQResponse`

2. request definition

```go
type OpsMQRequest struct {
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 17. "获取节点列表"

1. route definition

- Url: /api/v1/ops/nodes
- Method: GET
- Request: `OpsNodesRequest`
- Response: `OpsNodesResponse`

2. request definition

```go
type OpsNodesRequest struct {
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 18. "排空节点"

1. route definition

- Url: /api/v1/ops/nodes/:nodeId/drain
- Method: POST
- Request: `OpsNodeActionRequest`
- Response: `OpsNodeDrainResponse`

2. request definition

```go
type OpsNodeActionRequest struct {
	NodeID string `path:"nodeId"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 19. "获取节点元数据"

1. route definition

- Url: /api/v1/ops/nodes/:nodeId/meta
- Method: GET
- Request: `OpsNodeMetaRequest`
- Response: `OpsNodeMetaResponse`

2. request definition

```go
type OpsNodeMetaRequest struct {
	NodeID string `path:"nodeId"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 20. "重启节点"

1. route definition

- Url: /api/v1/ops/nodes/:nodeId/restart
- Method: POST
- Request: `OpsNodeActionRequest`
- Response: `OpsNodeRestartResponse`

2. request definition

```go
type OpsNodeActionRequest struct {
	NodeID string `path:"nodeId"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 21. "取消排空节点"

1. route definition

- Url: /api/v1/ops/nodes/:nodeId/undrain
- Method: POST
- Request: `OpsNodeActionRequest`
- Response: `OpsNodeUndrainResponse`

2. request definition

```go
type OpsNodeActionRequest struct {
	NodeID string `path:"nodeId"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 22. "获取节点命令"

1. route definition

- Url: /api/v1/ops/nodes/commands
- Method: GET
- Request: `OpsNodeCommandsQuery`
- Response: `OpsNodeCommandsResponse`

2. request definition

```go
type OpsNodeCommandsQuery struct {
	NodeID string `form:"nodeId"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 23. "获取通知配置"

1. route definition

- Url: /api/v1/ops/notifications
- Method: GET
- Request: `OpsNotificationsGetRequest`
- Response: `OpsNotificationsGetResponse`

2. request definition

```go
type OpsNotificationsGetRequest struct {
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 24. "更新通知配置"

1. route definition

- Url: /api/v1/ops/notifications
- Method: PUT
- Request: `OpsNotificationsUpdateRequest`
- Response: `OpsNotificationsUpdateResponse`

2. request definition

```go
type OpsNotificationsUpdateRequest struct {
	Enabled bool `json:"enabled"`
	Channels []string `json:"channels,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 25. "获取服务列表"

1. route definition

- Url: /api/v1/ops/services
- Method: GET
- Request: `OpsServicesRequest`
- Response: `OpsServicesResponse`

2. request definition

```go
type OpsServicesRequest struct {
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 26. "获取静默规则列表"

1. route definition

- Url: /api/v1/ops/silences
- Method: GET
- Request: `OpsSilencesRequest`
- Response: `OpsSilencesResponse`

2. request definition

```go
type OpsSilencesRequest struct {
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 27. "删除静默规则"

1. route definition

- Url: /api/v1/ops/silences/:id
- Method: DELETE
- Request: `OpsAlertSilenceDeleteRequest`
- Response: `OpsSilenceDeleteResponse`

2. request definition

```go
type OpsAlertSilenceDeleteRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```
