# 运维 API

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

### 2. "获取 Agent 列表"

1. route definition

- Url: /api/v1/ops/agents
- Method: GET
- Request: `OpsAgentsListRequest`
- Response: `OpsAgentsListResponse`

2. request definition

```go
type OpsAgentsListRequest struct {
}
```

3. response definition

```go
type OpsAgentsListResponse struct {
	// 响应为裸 payload（去掉 envelope 的 code/message 两行）
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

```go
type OpsExecCommandRequest struct {
	AgentID string `path:"agentId"`
	Command string `json:"command"`
	Args []string `json:"args,optional"`
	Timeout int32 `json:"timeout,optional"`
}
```

3. response definition

```go
type OpsExecCommandResponse struct {
	// 响应为裸 payload（去掉 envelope 的 code/message 两行）
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

```go
type OpsAgentProcessesRequest struct {
	AgentID string `path:"agentId"`
}
```

3. response definition

```go
type OpsAgentProcessesResponse struct {
	// 响应为裸 payload（去掉 envelope 的 code/message 两行）
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

```go
type OpsProcessActionRequest struct {
	AgentID string `path:"agentId"`
	Name string `path:"name"`
	Force bool `json:"force,optional"`
}
```

3. response definition

```go
type OpsProcessActionResponse struct {
	// 响应为裸 payload（去掉 envelope 的 code/message 两行）
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

```go
type OpsProcessStartRequest struct {
	AgentID string `path:"agentId"`
	Name string `path:"name"`
}
```

3. response definition

```go
type OpsProcessStartResponse struct {
	// 响应为裸 payload（去掉 envelope 的 code/message 两行）
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

```go
type OpsProcessActionRequest struct {
	AgentID string `path:"agentId"`
	Name string `path:"name"`
	Force bool `json:"force,optional"`
}
```

3. response definition

```go
type OpsProcessActionResponse struct {
	// 响应为裸 payload（去掉 envelope 的 code/message 两行）
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

```go
type OpsAgentSystemInfoRequest struct {
	AgentID string `path:"agentId"`
}
```

3. response definition

```go
type OpsAgentSystemInfoResponse struct {
	// 响应为裸 payload（去掉 envelope 的 code/message 两行）
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

```go
type OpsAgentMetricsRequest struct {
	AgentID string `form:"agentId,optional"`
	Since string `form:"since,optional"`
	Limit int `form:"limit,optional"`
}
```

3. response definition

```go
type OpsAgentMetricsResponse struct {
	// 响应为裸 payload（去掉 envelope 的 code/message 两行）
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

```go
type OpsAlertsRequest struct {
}
```

3. response definition

```go
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

### 12. "创建备份"

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

### 13. "获取备份列表"

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

### 14. "删除备份"

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

### 15. "下载备份"

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

### 16. "获取运维配置"

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

### 17. "获取函数列表"

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

### 18. "获取健康状态"

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

### 19. "更新健康检查配置"

1. route definition

- Url: /api/v1/ops/health
- Method: PUT
- Request: `OpsHealthUpdateRequest`
- Response: `OpsHealthUpdateResponse`

2. request definition

```go
type OpsHealthUpdateRequest struct {
	Enabled bool `json:"enabled"`
	Checks []OpsHealthCheck `json:"checks,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 20. "运行健康检查"

1. route definition

- Url: /api/v1/ops/health/run
- Method: POST
- Request: `OpsHealthRunRequest`
- Response: `OpsHealthRunResponse`

2. request definition

```go
type OpsHealthRunRequest struct {
	ID string `json:"id,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 21. "获取维护模式状态"

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

### 22. "更新维护模式"

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
	Windows []OpsMaintenanceWindow `json:"windows,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 23. "获取指标"

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

### 24. "获取消息队列状态"

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

### 25. "获取节点列表"

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
	// 响应为裸 payload（去掉 envelope 的 code/message 两行）
	Data interface{} `json:"data,omitempty"`
}

// Data 实际为 []Node。Node 描述一个接入的 Agent 节点；
// SDKLanguage / SDKVersion 来自该 Agent 上 provider 的元数据（Instance.Metadata 端到端透传）。
type Node struct {
	Id          string            `json:"id"`
	Hostname    string            `json:"hostname"`
	Addr        string            `json:"addr"`
	GameId      string            `json:"gameId"`                 // 作用域：游戏
	Env         string            `json:"env"`                    // 作用域：环境
	Status      string            `json:"status"`                 // active / inactive
	Labels      map[string]string `json:"labels"`
	LastSeen    string            `json:"lastSeen"`               // RFC3339
	SDKLanguage string            `json:"sdkLanguage,omitempty"`  // go/java/python/cpp/csharp/node/custom
	SDKVersion  string            `json:"sdkVersion,omitempty"`
}
```

### 26. "排空节点"

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

### 27. "获取节点元数据"

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

### 28. "重启节点"

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

### 29. "取消排空节点"

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

### 30. "获取节点命令"

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

### 31. "获取通知配置"

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

### 32. "更新通知配置"

1. route definition

- Url: /api/v1/ops/notifications
- Method: PUT
- Request: `OpsNotificationsUpdateRequest`
- Response: `OpsNotificationsUpdateResponse`

2. request definition

```go
type OpsNotificationsUpdateRequest struct {
	Enabled bool `json:"enabled"`
	Channels []OpsNotificationChannel `json:"channels,optional"`
	Rules []OpsNotificationRule `json:"rules,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 33. "获取服务列表"

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

### 34. "获取静默规则列表"

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

### 35. "删除静默规则"

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

### 36. "获取系统运行信息"

1. route definition

- Url: /api/v1/ops/system/runtime
- Method: GET
- Request: 无
- Response: `SystemRuntimeResponse`

2. request definition

```go
// 无请求体
```

3. response definition

```go
type SystemRuntimeResponse struct {
	Version       string `json:"version"`
	GitCommit     string `json:"gitCommit"`
	BuildTime     string `json:"buildTime"`
	StartedAt     string `json:"startedAt"`     // RFC3339；进程启动时间未知时为空
	UptimeSeconds int64  `json:"uptimeSeconds"`
}
```

### 37. "检查更新"

1. route definition

- Url: /api/v1/ops/system/check-update
- Method: POST
- Request: 无
- Response: `SystemCheckUpdateResponse`

2. request definition

```go
// 无请求体。更新源取 L3 键 system.updateCheckUrl（未配置回落 GitHub releases/latest）；
// 拉取经出站守卫（sec.*）与调用策略（net.*）。
```

3. response definition

```go
type SystemCheckUpdateResponse struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion,omitempty"`
	HasUpdate      bool   `json:"hasUpdate"`
	Checked        bool   `json:"checked"`
	CheckedAt      string `json:"checkedAt"`
	Note           string `json:"note"`
}
```

> 边界：只检查并回写版本注记，不执行升级。

### 38. "获取性能参数快照"

1. route definition

- Url: /api/v1/ops/performance
- Method: GET
- Request: 无
- Response: `PerformanceSnapshotResponse`

2. request definition

```go
// 无请求体
```

3. response definition

```go
type PerformanceSnapshotResponse struct {
	Settings PerformanceSettingsSnapshot `json:"settings"` // 生效值（L2∧L3 合成）+ 逐键来源
	Runtime  PerformanceRuntime          `json:"runtime"`  // Go 运行时（goMaxProcs/goroutines/堆/GC/在线时长）
	Host     PerformanceHost             `json:"host"`     // 宿主机 CPU/内存/磁盘（gopsutil）
	Overload PerformanceOverload         `json:"overload"` // 阈值超限判定（阈值 0 = 不启用）
}

type PerformanceSettingsSnapshot struct {
	MaxCpuPct      int               `json:"maxCpuPct"`
	MaxMemoryPct   int               `json:"maxMemoryPct"`
	MaxDiskPct     int               `json:"maxDiskPct"`
	MaxConcurrent  int               `json:"maxConcurrent"`
	MaxThreadCount int               `json:"maxThreadCount"`
	CacheSize      int64             `json:"cacheSize"`
	Sources        map[string]string `json:"sources"`
}
```

### 39. "更新性能参数"

1. route definition

- Url: /api/v1/ops/performance
- Method: PUT
- Request: `map[string]jsonNumber`（`perf.*` 键逐键 L3 覆盖）
- Response: `PerformanceSnapshotResponse`（更新后快照）

2. request definition

```go
// { "perf.maxCpuPct": 90, "perf.cacheSize": 268435456 }
// 仅接受 perf.* 白名单键，未知键/非法值 400
```

3. response definition

```go
// 同 GET /api/v1/ops/performance
```

### 40. "获取日志维护快照"

1. route definition

- Url: /api/v1/ops/logs
- Method: GET
- Request: 无
- Response: `LogsSnapshotResponse`

2. request definition

```go
// 无请求体
```

3. response definition

```go
type LogsSnapshotResponse struct {
	Settings  LogsSettingsView  `json:"settings"`  // log.retentionDays L3 覆盖值 + 来源
	Effective LogsEffectiveView `json:"effective"` // 实际生效保留期（executionLogDays/taskLogDays）
	ServerLog ServerLogView     `json:"serverLog"` // 服务器日志轮转参数（配置文件级只读视图）
	Tables    []LogTableView    `json:"tables"`    // execution_logs/task_runs/task_events 体量
}
```

### 41. "更新日志保留参数"

1. route definition

- Url: /api/v1/ops/logs
- Method: PUT
- Request: `map[string]jsonNumber`（仅收 `log.retentionDays` 一键，0-36500，0 = 跟随配置文件）
- Response: `LogsSnapshotResponse`（更新后快照）

2. request definition

```go
// { "log.retentionDays": 30 }
// 其他键写入拒绝（cleanupCron/copierDir/copierKeep 为未接线占位键，拒绝以免假开关）
```

3. response definition

```go
// 同 GET /api/v1/ops/logs
```

### 42. "手动清理日志"

1. route definition

- Url: /api/v1/ops/logs/cleanup
- Method: POST
- Request: `LogsCleanupRequest`
- Response: `LogsCleanupResponse`

2. request definition

```go
type LogsCleanupRequest struct {
	Scope       string `json:"scope"`       // execution | task | all
	BeforeHours int    `json:"beforeHours"` // 清理该时刻之前的记录（24/168/720；上限 87600）
}
```

3. response definition

```go
type LogsCleanupResponse struct {
	Scope                string `json:"scope"`
	Cutoff               string `json:"cutoff"` // RFC3339
	ExecutionLogsDeleted int64  `json:"executionLogsDeleted"`
	TaskRunsDeleted      int64  `json:"taskRunsDeleted"`
	TaskEventsDeleted    int64  `json:"taskEventsDeleted"`
}
```

### 43. "第三方服务健康探针"

1. route definition

- Url: /api/v1/ops/probes/:channel
- Method: POST
- Request: 无（channel ∈ dingtalk|wecom|feishu|webhook|update|smtp）
- Response: `probeView`

2. request definition

```go
// 无请求体。webhook 四渠道 GET 只读探测对应 notification.* URL；
// update 探测 system.updateCheckUrl；smtp 走 TCP+EHLO 探活（不发信不认证）。
// 未配置目标返回 configured=false（200）；非法渠道 404。
// 探测经出站守卫（sec.*）与调用策略（net.* 超时/重试）。
```

3. response definition

```go
type probeView struct {
	Channel    string `json:"channel"`
	Configured bool   `json:"configured"`
	OK         bool   `json:"ok"`
	Status     int    `json:"status"`
	LatencyMs  int64  `json:"latencyMs"`
	Error      string `json:"error,omitempty"`
}
```

## 集群与 LB 监控端点

| 方法 | 路径                                | 说明                                                              |
| ---- | ----------------------------------- | ----------------------------------------------------------------- |
| GET  | `/api/v1/ops/cluster`               | 集群拓扑（成员表、owner 分布、实例互联状态）——页面 `/ops/cluster` |
| POST | `/api/v1/ops/cluster/lb-stats`      | LB 监控（HAProxy 统计 + agent 三方对账）——页面 `/ops/lb`          |
| GET  | `/api/v1/ops/agent/metrics/history` | agent 指标历史（节点详情页用）                                    |

LB 监控依赖 `ops.prometheusUrl`（env `CROUPIER_LB_PROMETHEUS_URL`），
详见 [负载均衡监控](../operations/load-balancing.md)。
