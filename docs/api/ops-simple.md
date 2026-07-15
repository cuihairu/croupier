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
type OpsAgentMetaResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsAlertSilenceResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsBackupCreateResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsBackupsListResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsBackupDeleteResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsBackupDownloadResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsConfigResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsFunctionsResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsHealthGetResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsHealthUpdateResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsHealthRunResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsMaintenanceGetResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsMaintenanceUpdateResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsMetricsResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsMQResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsNodesResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsNodeDrainResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsNodeMetaResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsNodeRestartResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsNodeUndrainResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsNodeCommandsResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsNotificationsGetResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsNotificationsUpdateResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsServicesResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsSilencesResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
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
type OpsSilenceDeleteResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

