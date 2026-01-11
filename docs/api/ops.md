### 1. "更新代理元数据"

1. route definition

- Url: /api/v1/ops/agent-meta
- Method: PUT
- Request: `OpsAgentMetaUpdateRequest`
- Response: `OpsAgentMetaResponse`

2. request definition



```golang
type OpsAgentMetaUpdateRequest struct {
	AgentID string `json:"agentId"`
	Meta interface{} `json:"meta"`
}
```


3. response definition



```golang
type OpsAgentMetaResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 2. "获取 Agent 列表"

1. route definition

- Url: /api/v1/ops/agents
- Method: GET
- Request: `OpsAgentsListRequest`
- Response: `OpsAgentsListResponse`

2. request definition



```golang
type OpsAgentsListRequest struct {
}
```


3. response definition



```golang
type OpsAgentsListResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data []OpsAgentInfo `json:"data,omitempty"`
}
```

### 3. "在 Agent 上执行命令（高风险）"

1. route definition

- Url: /api/v1/ops/agents/:agentId/exec
- Method: POST
- Request: `OpsExecCommandRequest`
- Response: `OpsExecCommandResponse`

2. request definition



```golang
type OpsExecCommandRequest struct {
	AgentID string `path:"agentId"`
	Command string `json:"command"`
	Args []string `json:"args,optional"`
	Timeout int32 `json:"timeout,optional"`
}
```


3. response definition



```golang
type OpsExecCommandResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data OpsExecCommandResult `json:"data,omitempty"`
}

type OpsExecCommandResult struct {
	Success bool `json:"success"`
	ExitCode int32 `json:"exitCode"`
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}
```

### 4. "获取 Agent 进程列表"

1. route definition

- Url: /api/v1/ops/agents/:agentId/processes
- Method: GET
- Request: `OpsAgentProcessesRequest`
- Response: `OpsAgentProcessesResponse`

2. request definition



```golang
type OpsAgentProcessesRequest struct {
	AgentID string `path:"agentId"`
}
```


3. response definition



```golang
type OpsAgentProcessesResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data []OpsManagedProcess `json:"data,omitempty"`
}
```

### 5. "重启 Agent 进程"

1. route definition

- Url: /api/v1/ops/agents/:agentId/processes/:name/restart
- Method: POST
- Request: `OpsProcessActionRequest`
- Response: `OpsProcessActionResponse`

2. request definition



```golang
type OpsProcessActionRequest struct {
	AgentID string `path:"agentId"`
	Name string `path:"name"`
	Force bool `json:"force,optional"`
}
```


3. response definition



```golang
type OpsProcessActionResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data int32 `json:"pid,omitempty"`
}
```

### 6. "启动 Agent 进程"

1. route definition

- Url: /api/v1/ops/agents/:agentId/processes/:name/start
- Method: POST
- Request: `OpsProcessStartRequest`
- Response: `OpsProcessStartResponse`

2. request definition



```golang
type OpsProcessStartRequest struct {
	AgentID string `path:"agentId"`
	Name string `path:"name"`
}
```


3. response definition



```golang
type OpsProcessStartResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data int32 `json:"pid,omitempty"`
}
```

### 7. "停止 Agent 进程"

1. route definition

- Url: /api/v1/ops/agents/:agentId/processes/:name/stop
- Method: POST
- Request: `OpsProcessActionRequest`
- Response: `OpsProcessActionResponse`

2. request definition



```golang
type OpsProcessActionRequest struct {
	AgentID string `path:"agentId"`
	Name string `path:"name"`
	Force bool `json:"force,optional"`
}
```


3. response definition



```golang
type OpsProcessActionResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data int32 `json:"pid,omitempty"`
}
```

### 8. "获取 Agent 系统信息"

1. route definition

- Url: /api/v1/ops/agents/:agentId/system-info
- Method: GET
- Request: `OpsAgentSystemInfoRequest`
- Response: `OpsAgentSystemInfoResponse`

2. request definition



```golang
type OpsAgentSystemInfoRequest struct {
	AgentID string `path:"agentId"`
}
```


3. response definition



```golang
type OpsAgentSystemInfoResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data OpsAgentSystemInfo `json:"data,omitempty"`
}

type OpsAgentSystemInfo struct {
	Hostname string `json:"hostname"`
	OS string `json:"os"`
	OSVersion string `json:"osVersion"`
	KernelVersion string `json:"kernelVersion"`
	Arch string `json:"arch"`
	CPUCores int32 `json:"cpuCores"`
	TotalMemory uint64 `json:"totalMemory"`
	BootTime string `json:"bootTime"`
	AgentVersion string `json:"agentVersion"`
}
```

### 9. "获取 Agent 指标"

1. route definition

- Url: /api/v1/ops/agents/metrics
- Method: GET
- Request: `OpsAgentMetricsRequest`
- Response: `OpsAgentMetricsResponse`

2. request definition



```golang
type OpsAgentMetricsRequest struct {
	AgentID string `form:"agentId,optional"`
	Since string `form:"since,optional"`
	Limit int `form:"limit,optional"`
}
```


3. response definition



```golang
type OpsAgentMetricsResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data []OpsMetricsData `json:"data,omitempty"`
}
```

### 10. "获取告警列表"

1. route definition

- Url: /api/v1/ops/alerts
- Method: GET
- Request: `OpsAlertsRequest`
- Response: `OpsAlertsResponse`

2. request definition



```golang
type OpsAlertsRequest struct {
}
```


3. response definition



```golang
type OpsAlertsResponse struct {
	Alerts []OpsAlert `json:"alerts"`
}
```

### 11. "静默告警"

1. route definition

- Url: /api/v1/ops/alerts/silence
- Method: POST
- Request: `OpsAlertSilenceRequest`
- Response: `OpsAlertSilenceResponse`

2. request definition



```golang
type OpsAlertSilenceRequest struct {
	AlertID string `json:"alertId"`
	Duration int `json:"duration"` // 静默时长（分钟）
}
```


3. response definition



```golang
type OpsAlertSilenceResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 12. "创建备份"

1. route definition

- Url: /api/v1/ops/backups
- Method: POST
- Request: `OpsBackupCreateRequest`
- Response: `OpsBackupCreateResponse`

2. request definition



```golang
type OpsBackupCreateRequest struct {
	Name string `json:"name,optional"`
}
```


3. response definition



```golang
type OpsBackupCreateResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 13. "获取备份列表"

1. route definition

- Url: /api/v1/ops/backups
- Method: GET
- Request: `OpsBackupsListRequest`
- Response: `OpsBackupsListResponse`

2. request definition



```golang
type OpsBackupsListRequest struct {
	Page int `form:"page,optional"`
	PageSize int `form:"pageSize,optional"`
}
```


3. response definition



```golang
type OpsBackupsListResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 14. "删除备份"

1. route definition

- Url: /api/v1/ops/backups/:id
- Method: DELETE
- Request: `OpsBackupDeleteRequest`
- Response: `OpsBackupDeleteResponse`

2. request definition



```golang
type OpsBackupDeleteRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type OpsBackupDeleteResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 15. "下载备份"

1. route definition

- Url: /api/v1/ops/backups/:id/download
- Method: GET
- Request: `OpsBackupDownloadRequest`
- Response: `OpsBackupDownloadResponse`

2. request definition



```golang
type OpsBackupDownloadRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type OpsBackupDownloadResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 16. "获取运维配置"

1. route definition

- Url: /api/v1/ops/config
- Method: GET
- Request: `OpsConfigRequest`
- Response: `OpsConfigResponse`

2. request definition



```golang
type OpsConfigRequest struct {
}
```


3. response definition



```golang
type OpsConfigResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 17. "获取函数列表"

1. route definition

- Url: /api/v1/ops/functions
- Method: GET
- Request: `OpsFunctionsRequest`
- Response: `OpsFunctionsResponse`

2. request definition



```golang
type OpsFunctionsRequest struct {
}
```


3. response definition



```golang
type OpsFunctionsResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 18. "获取健康状态"

1. route definition

- Url: /api/v1/ops/health
- Method: GET
- Request: `OpsHealthGetRequest`
- Response: `OpsHealthGetResponse`

2. request definition



```golang
type OpsHealthGetRequest struct {
}
```


3. response definition



```golang
type OpsHealthGetResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 19. "更新健康检查配置"

1. route definition

- Url: /api/v1/ops/health
- Method: PUT
- Request: `OpsHealthUpdateRequest`
- Response: `OpsHealthUpdateResponse`

2. request definition



```golang
type OpsHealthUpdateRequest struct {
	Enabled bool `json:"enabled"`
	Checks []OpsHealthCheck `json:"checks,optional"`
}
```


3. response definition



```golang
type OpsHealthUpdateResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 20. "运行健康检查"

1. route definition

- Url: /api/v1/ops/health/run
- Method: POST
- Request: `OpsHealthRunRequest`
- Response: `OpsHealthRunResponse`

2. request definition



```golang
type OpsHealthRunRequest struct {
	ID string `json:"id,optional"`
}
```


3. response definition



```golang
type OpsHealthRunResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 21. "获取维护模式状态"

1. route definition

- Url: /api/v1/ops/maintenance
- Method: GET
- Request: `OpsMaintenanceGetRequest`
- Response: `OpsMaintenanceGetResponse`

2. request definition



```golang
type OpsMaintenanceGetRequest struct {
}
```


3. response definition



```golang
type OpsMaintenanceGetResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 22. "更新维护模式"

1. route definition

- Url: /api/v1/ops/maintenance
- Method: PUT
- Request: `OpsMaintenanceUpdateRequest`
- Response: `OpsMaintenanceUpdateResponse`

2. request definition



```golang
type OpsMaintenanceUpdateRequest struct {
	Enabled bool `json:"enabled"`
	Message string `json:"message,optional"`
	Windows []OpsMaintenanceWindow `json:"windows,optional"`
}
```


3. response definition



```golang
type OpsMaintenanceUpdateResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 23. "获取指标"

1. route definition

- Url: /api/v1/ops/metrics
- Method: GET
- Request: `OpsMetricsQuery`
- Response: `OpsMetricsResponse`

2. request definition



```golang
type OpsMetricsQuery struct {
	Start string `form:"start,optional"`
	End string `form:"end,optional"`
}
```


3. response definition



```golang
type OpsMetricsResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 24. "获取消息队列状态"

1. route definition

- Url: /api/v1/ops/mq
- Method: GET
- Request: `OpsMQRequest`
- Response: `OpsMQResponse`

2. request definition



```golang
type OpsMQRequest struct {
}
```


3. response definition



```golang
type OpsMQResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 25. "获取节点列表"

1. route definition

- Url: /api/v1/ops/nodes
- Method: GET
- Request: `OpsNodesRequest`
- Response: `OpsNodesResponse`

2. request definition



```golang
type OpsNodesRequest struct {
}
```


3. response definition



```golang
type OpsNodesResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 26. "排空节点"

1. route definition

- Url: /api/v1/ops/nodes/:nodeId/drain
- Method: POST
- Request: `OpsNodeActionRequest`
- Response: `OpsNodeDrainResponse`

2. request definition



```golang
type OpsNodeActionRequest struct {
	NodeID string `path:"nodeId"`
}
```


3. response definition



```golang
type OpsNodeDrainResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 27. "获取节点元数据"

1. route definition

- Url: /api/v1/ops/nodes/:nodeId/meta
- Method: GET
- Request: `OpsNodeMetaRequest`
- Response: `OpsNodeMetaResponse`

2. request definition



```golang
type OpsNodeMetaRequest struct {
	NodeID string `path:"nodeId"`
}
```


3. response definition



```golang
type OpsNodeMetaResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 28. "重启节点"

1. route definition

- Url: /api/v1/ops/nodes/:nodeId/restart
- Method: POST
- Request: `OpsNodeActionRequest`
- Response: `OpsNodeRestartResponse`

2. request definition



```golang
type OpsNodeActionRequest struct {
	NodeID string `path:"nodeId"`
}
```


3. response definition



```golang
type OpsNodeRestartResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 29. "取消排空节点"

1. route definition

- Url: /api/v1/ops/nodes/:nodeId/undrain
- Method: POST
- Request: `OpsNodeActionRequest`
- Response: `OpsNodeUndrainResponse`

2. request definition



```golang
type OpsNodeActionRequest struct {
	NodeID string `path:"nodeId"`
}
```


3. response definition



```golang
type OpsNodeUndrainResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 30. "获取节点命令"

1. route definition

- Url: /api/v1/ops/nodes/commands
- Method: GET
- Request: `OpsNodeCommandsQuery`
- Response: `OpsNodeCommandsResponse`

2. request definition



```golang
type OpsNodeCommandsQuery struct {
	NodeID string `form:"nodeId"`
}
```


3. response definition



```golang
type OpsNodeCommandsResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 31. "获取通知配置"

1. route definition

- Url: /api/v1/ops/notifications
- Method: GET
- Request: `OpsNotificationsGetRequest`
- Response: `OpsNotificationsGetResponse`

2. request definition



```golang
type OpsNotificationsGetRequest struct {
}
```


3. response definition



```golang
type OpsNotificationsGetResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 32. "更新通知配置"

1. route definition

- Url: /api/v1/ops/notifications
- Method: PUT
- Request: `OpsNotificationsUpdateRequest`
- Response: `OpsNotificationsUpdateResponse`

2. request definition



```golang
type OpsNotificationsUpdateRequest struct {
	Enabled bool `json:"enabled"`
	Channels []OpsNotificationChannel `json:"channels,optional"`
	Rules []OpsNotificationRule `json:"rules,optional"`
}
```


3. response definition



```golang
type OpsNotificationsUpdateResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 33. "获取服务列表"

1. route definition

- Url: /api/v1/ops/services
- Method: GET
- Request: `OpsServicesRequest`
- Response: `OpsServicesResponse`

2. request definition



```golang
type OpsServicesRequest struct {
}
```


3. response definition



```golang
type OpsServicesResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 34. "获取静默规则列表"

1. route definition

- Url: /api/v1/ops/silences
- Method: GET
- Request: `OpsSilencesRequest`
- Response: `OpsSilencesResponse`

2. request definition



```golang
type OpsSilencesRequest struct {
}
```


3. response definition



```golang
type OpsSilencesResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

### 35. "删除静默规则"

1. route definition

- Url: /api/v1/ops/silences/:id
- Method: DELETE
- Request: `OpsAlertSilenceDeleteRequest`
- Response: `OpsSilenceDeleteResponse`

2. request definition



```golang
type OpsAlertSilenceDeleteRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```golang
type OpsSilenceDeleteResponse struct {
	Code int `json:"code"`
	Message string `json:"message"`
	Data interface{} `json:"data,omitempty"`
}
```

