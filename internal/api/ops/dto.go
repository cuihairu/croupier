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

// Alert operations DTOs

type OpsAlert struct {
	Severity    string                 `json:"severity,omitempty"`
	Service     string                 `json:"service,omitempty"`
	Instance    string                 `json:"instance,omitempty"`
	Summary     string                 `json:"summary,omitempty"`
	StartsAt    string                 `json:"starts_at,omitempty"`
	EndsAt      string                 `json:"ends_at,omitempty"`
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
	AlertmanagerURL   string `json:"alertmanager_url,omitempty"`
	GrafanaExploreURL string `json:"grafana_explore_url,omitempty"`
	JaegerURL         string `json:"jaeger_url,omitempty"`
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

type OpsNodeMetaRequest struct {
	NodeID string `uri:"nodeId"`
}

type OpsNodeMetaResponse struct {
	Labels map[string]string `json:"labels"`
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
	ServiceID    string   `json:"service_id"`
	Addr         string   `json:"addr"`
	Version      string   `json:"version"`
	LastSeenUnix int64    `json:"last_seen_unix"`
	FunctionIDs  []string `json:"function_ids"`
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

// OpsNodeCommandsRequest is used to get node commands
type OpsNodeCommandsRequest struct {
	NodeId string `json:"nodeId" binding:"required"`
}

// Node represents a node in the system
type Node struct {
	Id       string            `json:"id"`
	Hostname string            `json:"hostname"`
	Addr     string            `json:"addr"`
	GameId   string            `json:"gameId"`
	Env      string            `json:"env"`
	Status   string            `json:"status"`
	Labels   map[string]string `json:"labels"`
	LastSeen string            `json:"lastSeen"`
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
