package ops

// Ops-related DTOs extracted from internal/types/types.go
// These types preserve all original struct tags for backward compatibility

// Agent operations DTOs

type OpsAgentInfo struct {
	AgentID   string            `json:"agentId"`
	GameID    string            `json:"gameId"`
	Env       string            `json:"env"`
	Version   string            `json:"version"`
	Addr      string            `json:"addr"`
	Connected bool              `json:"connected"`
	LastSeen  string            `json:"lastSeen"`
	Functions []string          `json:"functions"`
	Processes []string          `json:"processes"`
	Labels    map[string]string `json:"labels"`
	// OwnerInstance 集群模式下该 agent 连接的持有实例（本实例直连时为空）。
	OwnerInstance string `json:"ownerInstance,omitempty"`
	// Supervisor 监管聚合灯（ok/warn/error + 计数）；agent 无上报时为 nil。
	Supervisor *OpsSupervisorSummary `json:"supervisor,omitempty"`
}

type OpsAgentMetaResponse struct {
	Meta interface{} `json:"meta"`
}

type OpsAgentMetaUpdateRequest struct {
	AgentID string      `json:"agentId"`
	Meta    interface{} `json:"meta"`
}

type OpsAgentMetricsRequest struct {
	AgentID string `form:"agentId"`
	Since   string `form:"since"`
	Limit   int    `form:"limit"`
}

type OpsAgentMetricsResponse struct {
	Metrics []OpsMetricsData `json:"metrics"`
}

type OpsAgentProcessesRequest struct {
	AgentID string `uri:"agentId"`
}

type OpsAgentProcessesResponse struct {
	Processes []OpsManagedProcess `json:"processes"`
}

type OpsAgentSystemInfo struct {
	Hostname      string `json:"hostname"`
	OS            string `json:"os"`
	OSVersion     string `json:"osVersion"`
	KernelVersion string `json:"kernelVersion"`
	Arch          string `json:"arch"`
	CPUCores      int32  `json:"cpuCores"`
	TotalMemory   uint64 `json:"totalMemory"`
	BootTime      string `json:"bootTime"`
	AgentVersion  string `json:"agentVersion"`
}

type OpsAgentSystemInfoRequest struct {
	AgentID string `uri:"agentId"`
}

type OpsAgentSystemInfoResponse struct {
	SystemInfo OpsAgentSystemInfo `json:"systemInfo"`
}

type OpsAgentsListRequest struct {
}

type OpsAgentsListResponse struct {
	Agents []OpsAgentInfo `json:"agents"`
}

// Supervisor 监管视图（S1 只读：快照来自 agent metrics 上报捎带的
// SupervisedProcessSnapshot，server 内存 MetricsStore 最新一报）。

type OpsAgentSupervisorRequest struct {
	AgentID string `uri:"agentId" form:"agentId"`
}

type OpsSupervisedProcess struct {
	Name          string   `json:"name"`
	Pid           int32    `json:"pid"`
	State         string   `json:"state"`
	UptimeSeconds int64    `json:"uptimeSeconds"`
	RestartCount  int32    `json:"restartCount"`
	RssBytes      int64    `json:"rssBytes"`
	CpuPercent    float64  `json:"cpuPercent"`
	Flags         []string `json:"flags"`
	// LastEventUnix 最近一次 supervisor 事件时间（0=无）；NextRestartAtUnix
	// 仅 BACKOFF 态非零（面板渲染退避倒计时）。SnapshotProfile 是 S3 崩溃
	// 快照档（none/go/node/python/jvm，空=未知）。
	LastEventUnix     int64  `json:"lastEventUnix"`
	NextRestartAtUnix int64  `json:"nextRestartAtUnix"`
	SnapshotProfile   string `json:"snapshotProfile,omitempty"`
}

type OpsSupervisorSummary struct {
	// Status 聚合灯：ok（全部 RUNNING 且无 flags）/ warn（有非 RUNNING 或
	// 有超限标记）/ error（有 FAILED/BROKEN）。
	Status  string `json:"status"`
	Total   int    `json:"total"`
	Running int    `json:"running"`
}

type OpsAgentSupervisorResponse struct {
	AgentID string `json:"agentId"`
	// Timestamp 是 server 收到最新 metrics 上报的时间（面板据此判断数据新鲜度）。
	Timestamp string                 `json:"timestamp"`
	Processes []OpsSupervisedProcess `json:"processes"`
	Summary   OpsSupervisorSummary   `json:"summary"`
}

// Supervisor 事件日志（S2：事件由 agent 产生、metrics 上报捎带，server 端
// MetricsStore 内存环缓存；面板增量拉取游标为 seq）。

type OpsAgentSupervisorEventsRequest struct {
	AgentID  string `uri:"agentId" form:"agentId"`
	SinceSeq int64  `form:"sinceSeq"`
	Limit    int    `form:"limit"`
}

type OpsSupervisorEvent struct {
	Seq int64  `json:"seq"`
	Ts  string `json:"ts"`
	// TsUnix 事件原始 unix 秒（面板本地化渲染用；ts 是 server 格式化串）。
	TsUnix       int64  `json:"tsUnix,omitempty"`
	Process      string `json:"process"`
	Event        string `json:"event"`
	OldPid       int32  `json:"oldPid"`
	NewPid       int32  `json:"newPid"`
	ExitCode     int32  `json:"exitCode"`
	Signal       string `json:"signal"`
	RestartCount int32  `json:"restartCount"`
	Message      string `json:"message,omitempty"`
	// 事件时点上下文（尽力诊断）：lastHeartbeat 最后心跳、lastError 进程
	// 最后错误输出、oomSuspect OOM 疑似启发式、lastRssBytes 最后 RSS。
	// S3：detect_down 附带崩溃快照目录与产物文件名（best-effort）。
	LastHeartbeat string   `json:"lastHeartbeat,omitempty"`
	LastError     string   `json:"lastError,omitempty"`
	OomSuspect    bool     `json:"oomSuspect"`
	LastRssBytes  int64    `json:"lastRssBytes"`
	SnapshotDir   string   `json:"snapshotDir,omitempty"`
	SnapshotFiles []string `json:"snapshotFiles,omitempty"`
}

type OpsAgentSupervisorEventsResponse struct {
	AgentID string               `json:"agentId"`
	Events  []OpsSupervisorEvent `json:"events"`
	// LatestSeq server 环内最新 seq（0=无事件）；面板以此为增量游标。
	LatestSeq int64 `json:"latestSeq"`
}

// OpsAgentSupervisorLogRequest 拉取 agent 本地 supervisor 事件日志文件
// （tail 截断，二进制直传响应，非 JSON envelope）。
type OpsAgentSupervisorLogRequest struct {
	AgentID  string `uri:"agentId" form:"agentId"`
	MaxBytes int    `form:"maxBytes"`
}

// Alert operations DTOs

type OpsAlert struct {
	Severity    string                 `json:"severity,omitempty"`
	Service     string                 `json:"service,omitempty"`
	Instance    string                 `json:"instance,omitempty"`
	Summary     string                 `json:"summary,omitempty"`
	StartsAt    string                 `json:"startsAt,omitempty"`
	EndsAt      string                 `json:"endsAt,omitempty"`
	Duration    string                 `json:"duration,omitempty"`
	Silenced    bool                   `json:"silenced,omitempty"`
	Labels      map[string]interface{} `json:"labels,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
}

type OpsAlertSilenceDeleteRequest struct {
	ID string `uri:"id"`
}

type OpsAlertSilenceRequest struct {
	AlertID  string `json:"alertId"`
	Duration int    `json:"duration"` // 静默时长（分钟）
}

type OpsAlertSilenceResponse struct {
	SilenceID string `json:"silenceId"`
}

type OpsAlertsRequest struct {
}

type OpsAlertsResponse struct {
	Alerts []OpsAlert `json:"alerts"`
}

// Backup operations DTOs

type Backup struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

type BackupCreateRequest struct {
	Name string `json:"name"`
	Type string `json:"type"` // full, incremental
}

type BackupDeleteRequest struct {
	ID string `uri:"id"`
}

type BackupDetailResponse struct {
	Backup
}

type BackupDownloadRequest struct {
	ID string `uri:"id"`
}

type BackupsListRequest struct {
	Page     int    `form:"page,optional,default=1"`
	PageSize int    `form:"pageSize,optional,default=20"`
	Type     string `form:"type"`
}

type BackupsListResponse struct {
	Items []Backup `json:"items"`
	Total int64    `json:"total"`
	Page  int      `json:"page"`
	Size  int      `json:"pageSize"`
}

type OpsBackupCreateRequest struct {
	Name string `json:"name"`
}

type OpsBackupCreateResponse struct {
	BackupID string `json:"backupId"`
}

type OpsBackupDeleteRequest struct {
	ID string `uri:"id"`
}

type OpsBackupDeleteResponse struct {
	Deleted bool `json:"deleted"`
}

type OpsBackupDownloadRequest struct {
	ID string `uri:"id"`
}

type OpsBackupDownloadResponse struct {
	Url string `json:"url"`
}

type OpsBackupsListRequest struct {
	Page     int `form:"page"`
	PageSize int `form:"pageSize"`
}

type OpsBackupsListResponse struct {
	Backups []Backup `json:"backups"`
}

// Config and operations DTOs

type OpsConfigRequest struct {
}

type OpsConfigResponse struct {
	AlertmanagerURL   string `json:"alertmanagerUrl,omitempty"`
	GrafanaExploreURL string `json:"grafanaExploreUrl,omitempty"`
	JaegerURL         string `json:"jaegerUrl,omitempty"`
}

// Metrics DTOs

type OpsCpuMetrics struct {
	UsagePercent float64   `json:"usagePercent"`
	Cores        int32     `json:"cores"`
	PerCore      []float64 `json:"perCore,omitempty"`
	Load1M       float64   `json:"load1m"`
	Load5M       float64   `json:"load5m"`
	Load15M      float64   `json:"load15m"`
}

type OpsDiskMetrics struct {
	MountPoint     string  `json:"mountPoint"`
	Device         string  `json:"device"`
	FsType         string  `json:"fsType"`
	TotalBytes     uint64  `json:"totalBytes"`
	UsedBytes      uint64  `json:"usedBytes"`
	AvailableBytes uint64  `json:"availableBytes"`
	UsagePercent   float64 `json:"usagePercent"`
	InodeTotal     uint64  `json:"inodeTotal,omitempty"`
	InodeUsed      uint64  `json:"inodeUsed,omitempty"`
}

type OpsMemoryMetrics struct {
	TotalBytes     uint64  `json:"totalBytes"`
	UsedBytes      uint64  `json:"usedBytes"`
	AvailableBytes uint64  `json:"availableBytes"`
	UsagePercent   float64 `json:"usagePercent"`
	SwapTotal      uint64  `json:"swapTotal"`
	SwapUsed       uint64  `json:"swapUsed"`
}

type OpsMetricsData struct {
	AgentID   string              `json:"agentId"`
	Timestamp string              `json:"timestamp"`
	CPU       OpsCpuMetrics       `json:"cpu"`
	Memory    OpsMemoryMetrics    `json:"memory"`
	Disks     []OpsDiskMetrics    `json:"disks,omitempty"`
	Networks  []OpsNetworkMetrics `json:"networks,omitempty"`
}

type OpsMetricsQuery struct {
	Start string `form:"start"`
	End   string `form:"end"`
}

type OpsMetricsResponse struct {
	Metrics []OpsMetricsData `json:"metrics"`
}

type OpsNetworkMetrics struct {
	Interface   string `json:"interface"`
	BytesSent   uint64 `json:"bytesSent"`
	BytesRecv   uint64 `json:"bytesRecv"`
	PacketsSent uint64 `json:"packetsSent"`
	PacketsRecv uint64 `json:"packetsRecv"`
}

// Exec command DTOs

type OpsExecCommandRequest struct {
	AgentID string   `uri:"agentId"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Timeout int32    `json:"timeout"`
}

type OpsExecCommandResponse struct {
	Result OpsExecCommandResult `json:"result"`
}

type OpsExecCommandResult struct {
	Success  bool   `json:"success"`
	ExitCode int32  `json:"exitCode"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

// Functions DTOs

type OpsFunctionsRequest struct {
}

type OpsFunctionsResponse struct {
	Functions map[string][]string `json:"functions"`
}

// Health operations DTOs

type OpsHealthCheck struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	Type        string `json:"type"`
	Kind        string `json:"kind"`
	Target      string `json:"target"`
	Expect      string `json:"expect"`
	Region      string `json:"region"`
	Interval    int    `json:"interval"`
	IntervalSec int    `json:"intervalSec"`
	TimeoutMs   int    `json:"timeoutMs"`
}

type OpsHealthGetRequest struct {
}

type OpsHealthGetResponse struct {
	Checks    []OpsHealthCheck         `json:"checks"`
	Status    []map[string]interface{} `json:"status"`
	UpdatedAt string                   `json:"updatedAt"`
}

type OpsHealthRunRequest struct {
	ID string `json:"id"`
}

type OpsHealthRunResponse struct {
	Id        string `json:"id"`
	Ok        bool   `json:"ok"`
	LatencyMs int64  `json:"latencyMs"`
	CheckedAt string `json:"checkedAt"`
}

type OpsHealthUpdateRequest struct {
	Enabled bool             `json:"enabled"`
	Checks  []OpsHealthCheck `json:"checks"`
}

type OpsHealthUpdateResponse struct {
	Checks interface{} `json:"checks"`
}

// Maintenance operations DTOs

type OpsMaintenanceGetRequest struct {
}

type OpsMaintenanceGetResponse struct {
	Windows   []OpsMaintenanceWindow `json:"windows"`
	UpdatedAt string                 `json:"updatedAt"`
}

type OpsMaintenanceUpdateRequest struct {
	Enabled bool                   `json:"enabled"`
	Message string                 `json:"message"`
	Windows []OpsMaintenanceWindow `json:"windows"`
}

type OpsMaintenanceUpdateResponse struct {
	Windows interface{} `json:"windows"`
}

type OpsMaintenanceWindow struct {
	ID          string `json:"id"`
	GameID      string `json:"gameId"`
	Env         string `json:"env"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Message     string `json:"message"`
	BlockWrites bool   `json:"blockWrites"`
}

// MQ operations DTOs

type OpsMQRequest struct {
}

type OpsMQResponse struct {
	Result interface{} `json:"result"`
}

// Node operations DTOs

type OpsNodeActionRequest struct {
	NodeID string `uri:"nodeId"`
}

type OpsNodeCommandsQuery struct {
	NodeID string `form:"nodeId"`
}

type OpsNodeCommandsResponse struct {
	Commands []NodeCommand `json:"commands"`
}

type OpsNodeDrainResponse struct {
	NodeId string `json:"nodeId"`
	Status string `json:"status"`
}

// GET 走 BindQueryCompat 只认 form/json tag（不绑 uri tag），handler 侧以
// c.Param 兜底路径参数（同 OpsNodeDetailRequest）。
type OpsNodeMetaRequest struct {
	NodeID string `form:"nodeId" uri:"nodeId"`
}

type OpsNodeMetaResponse struct {
	Labels map[string]string `json:"labels"`
}

// OpsNodeDetailRequest/Response 支撑 GET /ops/nodes/:nodeId 单设备详情：
// 与列表同源（listNodes），按 id 过滤，字段集一致。GET 走 BindQueryCompat
// 只认 form/json tag（不绑 uri tag），handler 侧以 c.Param 兜底路径参数。
type OpsNodeDetailRequest struct {
	NodeID string `form:"nodeId" uri:"nodeId"`
}

type OpsNodeDetailResponse struct {
	Node Node `json:"node"`
}

type OpsNodeRestartResponse struct {
	NodeId string `json:"nodeId"`
	Status string `json:"status"`
}

type OpsNodeUndrainResponse struct {
	NodeId string `json:"nodeId"`
	Status string `json:"status"`
}

type OpsNodesRequest struct {
}

type OpsNodesResponse struct {
	Nodes []Node `json:"nodes"`
}

// Notification operations DTOs

type OpsNotificationChannel struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	URL    string `json:"url"`
	Secret string `json:"secret"`
}

type OpsNotificationRule struct {
	Event         string   `json:"event"`
	Channels      []string `json:"channels"`
	ThresholdDays int      `json:"thresholdDays"`
}

type OpsNotificationsGetRequest struct {
}

type OpsNotificationsGetResponse struct {
	Enabled  bool                     `json:"enabled"`
	Channels []OpsNotificationChannel `json:"channels"`
	Rules    []OpsNotificationRule    `json:"rules"`
}

type OpsNotificationsUpdateRequest struct {
	Enabled  bool                     `json:"enabled"`
	Channels []OpsNotificationChannel `json:"channels"`
	Rules    []OpsNotificationRule    `json:"rules"`
}

type OpsNotificationsUpdateResponse struct {
}

// Process operations DTOs

type OpsManagedProcess struct {
	Name         string `json:"name"`
	Command      string `json:"command"`
	WorkingDir   string `json:"workingDir"`
	State        string `json:"state"`
	Pid          int32  `json:"pid"`
	RestartCount int32  `json:"restartCount"`
	LastStart    string `json:"lastStart,omitempty"`
}

type OpsProcessActionRequest struct {
	AgentID string `uri:"agentId"`
	Name    string `uri:"name"`
	Force   bool   `json:"force"`
}

type OpsProcessActionResponse struct {
	Pid int32 `json:"pid,omitempty"`
}

type OpsProcessStartRequest struct {
	AgentID string `uri:"agentId"`
	Name    string `uri:"name"`
}

type OpsProcessStartResponse struct {
	Pid int32 `json:"pid,omitempty"`
}

// Service operations DTOs

type OpsServiceItem struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	Type           string              `json:"type"`
	Status         string              `json:"status"`
	Address        string              `json:"address"`
	GameID         string              `json:"gameId"`
	Env            string              `json:"env"`
	Version        string              `json:"version"`
	Region         string              `json:"region"`
	Zone           string              `json:"zone"`
	Labels         map[string]string   `json:"labels"`
	FunctionsCount int                 `json:"functionsCount"`
	LastSeen       string              `json:"lastSeen"`
	Metadata       *OpsServiceMetadata `json:"metadata"`
}

type OpsServiceMetadata struct {
	Processes      []OpsServiceProcess `json:"processes"`
	ProcessesCount int                 `json:"processesCount"`
}

type OpsServiceProcess struct {
	ServiceID    string   `json:"serviceId"`
	Addr         string   `json:"addr"`
	Version      string   `json:"version"`
	LastSeenUnix int64    `json:"lastSeenUnix"`
	FunctionIDs  []string `json:"functionIds"`
	Functions    int      `json:"functions"`
}

type OpsServicesRequest struct {
}

type OpsServicesResponse struct {
	Services []OpsServiceItem `json:"services"`
	Total    int              `json:"total"`
}

// Silence operations DTOs

// Silence represents a silence rule
type Silence struct {
	Id        string      `json:"id"`
	AlertType string      `json:"alertType"`
	Matchers  interface{} `json:"matchers"`
	StartAt   string      `json:"startAt"`
	EndAt     string      `json:"endAt"`
	CreatedBy string      `json:"createdBy"`
}

// SilenceDeleteRequest represents the request to delete a silence
type SilenceDeleteRequest struct {
	ID string `uri:"id"`
}

// SilencesListRequest represents the request to list silences
type SilencesListRequest struct{}

// SilencesListResponse represents the response with a list of silences
type SilencesListResponse struct {
	Items []Silence `json:"items"`
}

type OpsSilenceDeleteResponse struct {
	Deleted bool `json:"deleted"`
}

type OpsSilencesRequest struct {
}

type OpsSilencesResponse struct {
	Silences []Silence `json:"silences"`
}

// Additional DTOs used by the ops module helpers

// OpsAgentMetaRequest is used to get agent metadata
type OpsAgentMetaRequest struct {
	AgentId string `json:"agentId" binding:"required"`
}

// OpsMetricsRequest is used to query metrics
type OpsMetricsRequest struct {
	GameId      string `json:"gameId" binding:"required"`
	Env         string `json:"env"`
	Metric      string `json:"metric"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Aggregation string `json:"aggregation"`
}

// AgentMetricsHistoryRequest is used to query historical metrics for an agent
type AgentMetricsHistoryRequest struct {
	AgentID string `json:"agentId" binding:"required"`
	Since   string `json:"since"` // ISO 8601 timestamp
	Limit   int    `json:"limit"` // Max entries to return (default: 100)
}

// AgentMetricsHistoryResponse is the response for historical metrics
type AgentMetricsHistoryResponse struct {
	AgentID string                `json:"agentId"`
	Entries []MetricsHistoryEntry `json:"entries"`
}

// MetricsHistoryEntry represents a single historical metrics entry
type MetricsHistoryEntry struct {
	Timestamp string         `json:"timestamp"`
	CPU       *CpuMetrics    `json:"cpu,omitempty"`
	Memory    *MemoryMetrics `json:"memory,omitempty"`
	Disks     []DiskMetrics  `json:"disks,omitempty"`
}

// OpsNodeCommandsRequest is used to get node commands
type OpsNodeCommandsRequest struct {
	NodeId string `json:"nodeId" binding:"required"`
}

// Node represents a node in the system
type Node struct {
	Id           string            `json:"id"`
	Hostname     string            `json:"hostname"`
	Addr         string            `json:"addr"`
	GameId       string            `json:"gameId"`
	Env          string            `json:"env"`
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	LastSeen     string            `json:"lastSeen"`
	SDKLanguage  string            `json:"sdkLanguage,omitempty"`
	SDKVersion   string            `json:"sdkVersion,omitempty"`
	SDKName      string            `json:"sdkName,omitempty"`
	Version      string            `json:"version,omitempty"` // agent 二进制版本（agent_sessions.Version）
	Functions    int               `json:"functions"`
	ExpiresInSec int64             `json:"expiresInSec"`
	// System metrics (from SystemInfoCache)
	CPU    *CpuMetrics    `json:"cpu,omitempty"`
	Memory *MemoryMetrics `json:"memory,omitempty"`
	Disks  []DiskMetrics  `json:"disks,omitempty"`
	// Supervisor 监管聚合灯（数据源同 CPU/Memory：MetricsStore 最新一报的
	// SupervisedProcesses）；无上报或 agent 不在本地实例时为 nil。
	Supervisor *OpsSupervisorSummary `json:"supervisor,omitempty"`
}

// NodeCommand represents a command that can be executed on a node
type NodeCommand struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Params      map[string]string `json:"params"`
}

// Type aliases for backward compatibility with types package
// These allow existing code using types.OpsAgentInfo, etc. to continue working

type AgentInfo = OpsAgentInfo
type AlertSilenceDeleteRequest = OpsAlertSilenceDeleteRequest
type AlertSilenceRequest = OpsAlertSilenceRequest
type AlertSilenceResponse = OpsAlertSilenceResponse
type AlertsRequest = OpsAlertsRequest
type AlertsResponse = OpsAlertsResponse
type AgentMetaResponse = OpsAgentMetaResponse
type AgentMetaUpdateRequest = OpsAgentMetaUpdateRequest
type AgentMetricsRequest = OpsAgentMetricsRequest
type AgentMetricsResponse = OpsAgentMetricsResponse
type AgentProcessesRequest = OpsAgentProcessesRequest
type AgentProcessesResponse = OpsAgentProcessesResponse
type AgentSystemInfo = OpsAgentSystemInfo
type AgentSystemInfoRequest = OpsAgentSystemInfoRequest
type AgentSystemInfoResponse = OpsAgentSystemInfoResponse
type AgentsListRequest = OpsAgentsListRequest
type AgentsListResponse = OpsAgentsListResponse
type CpuMetrics = OpsCpuMetrics
type DiskMetrics = OpsDiskMetrics
type ExecCommandRequest = OpsExecCommandRequest
type ExecCommandResponse = OpsExecCommandResponse
type ExecCommandResult = OpsExecCommandResult
type FunctionsRequest = OpsFunctionsRequest
type FunctionsResponse = OpsFunctionsResponse
type HealthCheck = OpsHealthCheck
type HealthGetRequest = OpsHealthGetRequest
type HealthGetResponse = OpsHealthGetResponse
type HealthRunRequest = OpsHealthRunRequest
type HealthRunResponse = OpsHealthRunResponse
type HealthUpdateRequest = OpsHealthUpdateRequest
type HealthUpdateResponse = OpsHealthUpdateResponse
type MaintenanceGetRequest = OpsMaintenanceGetRequest
type MaintenanceGetResponse = OpsMaintenanceGetResponse
type MaintenanceUpdateRequest = OpsMaintenanceUpdateRequest
type MaintenanceUpdateResponse = OpsMaintenanceUpdateResponse
type MaintenanceWindow = OpsMaintenanceWindow
type ManagedProcess = OpsManagedProcess
type MemoryMetrics = OpsMemoryMetrics
type MetricsData = OpsMetricsData
type MetricsQuery = OpsMetricsQuery
type MetricsResponse = OpsMetricsResponse
type NetworkMetrics = OpsNetworkMetrics
type NodeActionRequest = OpsNodeActionRequest
type NodeCommandsQuery = OpsNodeCommandsQuery
type NodeCommandsResponse = OpsNodeCommandsResponse
type NodeDrainResponse = OpsNodeDrainResponse
type NodeMetaRequest = OpsNodeMetaRequest
type NodeMetaResponse = OpsNodeMetaResponse
type NodeDetailRequest = OpsNodeDetailRequest
type NodeDetailResponse = OpsNodeDetailResponse
type NodeRestartResponse = OpsNodeRestartResponse
type NodeUndrainResponse = OpsNodeUndrainResponse
type NodesRequest = OpsNodesRequest
type NodesResponse = OpsNodesResponse
type NotificationChannel = OpsNotificationChannel
type NotificationRule = OpsNotificationRule
type NotificationsGetRequest = OpsNotificationsGetRequest
type NotificationsGetResponse = OpsNotificationsGetResponse
type NotificationsUpdateRequest = OpsNotificationsUpdateRequest
type NotificationsUpdateResponse = OpsNotificationsUpdateResponse
type ProcessActionRequest = OpsProcessActionRequest
type ProcessActionResponse = OpsProcessActionResponse
type ProcessStartRequest = OpsProcessStartRequest
type ProcessStartResponse = OpsProcessStartResponse
type ServiceItem = OpsServiceItem
type ServiceMetadata = OpsServiceMetadata
type ServiceProcess = OpsServiceProcess
type ServicesRequest = OpsServicesRequest
type ServicesResponse = OpsServicesResponse
type SilenceDeleteResponse = OpsSilenceDeleteResponse
type SilencesRequest = OpsSilencesRequest
type SilencesResponse = OpsSilencesResponse
