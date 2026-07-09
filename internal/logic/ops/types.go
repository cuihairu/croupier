package ops

// Standard response wrapper for ops API
type OpsResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// Agent related types

type OpsAgentsListRequest struct {
	GameID string `json:"gameId"`
	Env    string `json:"env"`
}

type OpsAgentsListResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    []OpsAgentInfo `json:"data"`
}

type OpsAgentInfo struct {
	AgentID   string            `json:"agentId"`
	GameID    string            `json:"gameId"`
	Env       string            `json:"env"`
	Version   string            `json:"version"`
	Addr      string            `json:"addr"`
	RPCAddr   string            `json:"rpcAddr"` // compatibility alias; prefer "addr"
	Connected bool              `json:"connected"`
	LastSeen  string            `json:"lastSeen"`
	Functions []string          `json:"functions"`
	Processes []string          `json:"processes"`
	Labels    map[string]string `json:"labels"`
}

type OpsAgentMetaRequest struct {
	AgentID string `json:"agentId"`
}

type OpsAgentMetaResponse struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    OpsAgentMetaData `json:"data"`
}

type OpsAgentMetaData struct {
	AgentID  string            `json:"agentId"`
	Labels   map[string]string `json:"labels"`
	Metadata map[string]string `json:"metadata"`
}

type OpsAgentMetaUpdateRequest struct {
	AgentID  string            `json:"agentId"`
	Labels   map[string]string `json:"labels"`
	Metadata map[string]string `json:"metadata"`
}

type OpsAgentSystemInfoRequest struct {
	AgentID string `json:"agentId"`
}

type OpsAgentSystemInfoResponse struct {
	Code    int                `json:"code"`
	Message string             `json:"message"`
	Data    OpsAgentSystemInfo `json:"data"`
}

type OpsAgentSystemInfo struct {
	Hostname      string              `json:"hostname"`
	OS            string              `json:"os"`
	OSVersion     string              `json:"osVersion"`
	KernelVersion string              `json:"kernelVersion"`
	Arch          string              `json:"arch"`
	Uptime        int64               `json:"uptime"`
	CPUCores      int32               `json:"cpuCores"`
	TotalMemory   uint64              `json:"totalMemory"`
	BootTime      string              `json:"bootTime"`
	AgentVersion  string              `json:"agentVersion"`
	CPU           OpsCpuMetrics       `json:"cpu"`
	Memory        OpsMemoryMetrics    `json:"memory"`
	Disk          []OpsDiskMetrics    `json:"disk"`
	Network       []OpsNetworkMetrics `json:"network"`
	Agents        []OpsMetricsData    `json:"agents,omitempty"`
}

type OpsAgentMetricsRequest struct {
	Since   string `json:"since"`
	Limit   int    `json:"limit"`
	AgentID string `json:"agentId"`
}

type OpsAgentMetricsResponse struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    []OpsMetricsData `json:"data"`
}

type OpsAgentProcessesRequest struct {
	AgentID string `json:"agentId"`
}

type OpsAgentProcessesResponse struct {
	Code    int                 `json:"code"`
	Message string              `json:"message"`
	Data    []OpsManagedProcess `json:"data"`
}

type OpsProcessInfo struct {
	PID     int    `json:"pid"`
	Name    string `json:"name"`
	Command string `json:"command"`
	Status  string `json:"status"`
}

type OpsExecCommandRequest struct {
	AgentID string            `json:"agentId"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	Timeout int32             `json:"timeout"`
}

type OpsExecCommandResponse struct {
	Code    int           `json:"code"`
	Message string        `json:"message"`
	Data    OpsExecResult `json:"data"`
}

type OpsExecResult struct {
	ExitCode int32  `json:"exitCode"`
	StdOut   string `json:"stdout"`
	StdErr   string `json:"stderr"`
}

// Metrics types

type OpsCpuMetrics struct {
	Usage        float64   `json:"usage"`
	CoreCount    int32     `json:"cores"`
	Cores        int32     `json:"-"`
	UsagePercent float64   `json:"usagePercent"`
	Load1M       float64   `json:"load1m"`
	Load5M       float64   `json:"load5m"`
	Load15M      float64   `json:"load15m"`
	PerCore      []float64 `json:"perCore,omitempty"`
}

type OpsMemoryMetrics struct {
	Total          uint64  `json:"total"`
	Used           uint64  `json:"used"`
	Available      uint64  `json:"available"`
	Usage          float64 `json:"usage"`
	TotalBytes     uint64  `json:"totalBytes"`
	UsedBytes      uint64  `json:"usedBytes"`
	AvailableBytes uint64  `json:"availableBytes"`
	UsagePercent   float64 `json:"usagePercent"`
	SwapTotal      uint64  `json:"swapTotal"`
	SwapUsed       uint64  `json:"swapUsed"`
}

type OpsDiskMetrics struct {
	Device         string  `json:"device"`
	Mount          string  `json:"mount"`
	Total          uint64  `json:"total"`
	Used           uint64  `json:"used"`
	Usage          float64 `json:"usage"`
	MountPoint     string  `json:"mountPoint"`
	FsType         string  `json:"fsType"`
	TotalBytes     uint64  `json:"totalBytes"`
	UsedBytes      uint64  `json:"usedBytes"`
	AvailableBytes uint64  `json:"availableBytes"`
	UsagePercent   float64 `json:"usagePercent"`
}

type OpsNetworkMetrics struct {
	Interface   string `json:"interface"`
	BytesSent   uint64 `json:"bytesSent"`
	BytesRecv   uint64 `json:"bytesRecv"`
	PacketsSent uint64 `json:"packetsSent"`
	PacketsRecv uint64 `json:"packetsRecv"`
	Errors      uint64 `json:"errors"`
	Drops       uint64 `json:"drops"`
}

// Node related types

type OpsNodesListRequest struct {
	GameID string `json:"gameId"`
	Env    string `json:"env"`
}

type OpsNodesListResponse struct {
	Code    int           `json:"code"`
	Message string        `json:"message"`
	Data    []OpsNodeInfo `json:"data"`
}

type OpsNodeInfo struct {
	NodeID string            `json:"nodeId"`
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Status string            `json:"status"`
	IP     string            `json:"ip"`
	Port   int               `json:"port"`
	Labels map[string]string `json:"labels"`
}

type OpsNodeMetaRequest struct {
	NodeID string `json:"nodeId"`
}

type OpsNodeMetaResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    OpsNodeInfo `json:"data"`
}

type OpsNodeCommandsRequest struct {
	NodeID string `json:"nodeId"`
}

type OpsNodeCommandsResponse struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    []OpsNodeCommand `json:"data"`
}

type OpsNodeCommand struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

type OpsNodeDrainRequest struct {
	NodeID string `json:"nodeId"`
}

type OpsNodeDrainResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type OpsNodeUndrainRequest struct {
	NodeID string `json:"nodeId"`
}

type OpsNodeUndrainResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type OpsNodeRestartRequest struct {
	NodeID string `json:"nodeId"`
}

type OpsNodeRestartResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Function related types

type OpsFunctionsRequest struct {
	GameID string `json:"gameId"`
}

type OpsFunctionsResponse struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Data    []OpsFunctionInfo `json:"data"`
}

type OpsFunctionInfo struct {
	FunctionID string `json:"functionId"`
	Name       string `json:"name"`
	Category   string `json:"category"`
	Version    string `json:"version"`
	Instances  int    `json:"instances"`
}

// Config related types

type OpsConfigRequest struct {
	AgentID string `json:"agentId"`
}

type OpsConfigResponse struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Data    map[string]string `json:"data"`
}

// Health related types
// NOTE: Health operations are served by internal/api/ops (helpers.go) directly;
// the logic-layer stubs have been removed.

// Maintenance related types

type OpsMaintenanceGetRequest struct {
	Target string `json:"target"`
}

type OpsMaintenanceGetResponse struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Data    []OpsMaintenanceWindow `json:"data"`
}

type OpsMaintenanceWindow struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Reason    string `json:"reason"`
}

type OpsMaintenanceUpdateRequest struct {
	Target  string                 `json:"target"`
	Windows []OpsMaintenanceWindow `json:"windows"`
}

type OpsMaintenanceUpdateResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MQ related types

type OpsMQRequest struct {
	GameID string `json:"gameId"`
}

type OpsMQResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    []OpsQueueInfo `json:"data"`
}

type OpsQueueInfo struct {
	Name      string `json:"name"`
	Messages  int    `json:"messages"`
	Consumers int    `json:"consumers"`
}

// Metrics related types
// NOTE: OpsMetricsData / OpsMetricPoint are still used by OpsAgentMetricsLogic.
// The logic-layer OpsMetrics stub has been removed; metrics aggregation flows
// through internal/api/ops.

type OpsMetricPoint struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}

// Backup related types
// NOTE: Backup operations are served exclusively by internal/api/ops (BackupService)
// and internal/api/backup (Service). The former logic-layer stubs were removed;
// these request/response types are intentionally not re-declared here.

// Alert related types

type OpsAlertsRequest struct {
	GameID string `json:"gameId"`
}

type OpsAlertsResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    []OpsAlertInfo `json:"data"`
}

type OpsAlertInfo struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Status   string `json:"status"`
	Created  string `json:"created"`
}

type OpsAlertSilenceRequest struct {
	AlertID  string `json:"alertId"`
	Duration int    `json:"duration"`
	Comment  string `json:"comment"`
}

type OpsAlertSilenceResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    OpsSilenceInfo `json:"data"`
}

type OpsSilenceInfo struct {
	ID      string `json:"id"`
	AlertID string `json:"alertId"`
	Expires string `json:"expires"`
	Comment string `json:"comment"`
}

type OpsAlertSilenceDeleteRequest struct {
	ID string `json:"id"`
}

type OpsAlertSilenceDeleteResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type OpsSilencesRequest struct {
	AlertID string `json:"alertId"`
}

type OpsSilencesResponse struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    []OpsSilenceInfo `json:"data"`
}

// Notification related types

type OpsNotificationsGetRequest struct {
	GameID string `json:"gameId"`
}

type OpsNotificationsGetResponse struct {
	Code    int                   `json:"code"`
	Message string                `json:"message"`
	Data    []OpsNotificationInfo `json:"data"`
}

type OpsNotificationInfo struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
	Read     bool   `json:"read"`
	Created  string `json:"created"`
}

type OpsNotificationsUpdateRequest struct {
	ID   string `json:"id"`
	Read bool   `json:"read"`
}

type OpsNotificationsUpdateResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Services related types

type OpsServicesRequest struct {
	GameID string `json:"gameId"`
}

type OpsServicesResponse struct {
	Services []OpsServiceItem `json:"services"`
	Total    int              `json:"total"`
}

type OpsServiceInfo struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	Instances int    `json:"instances"`
}

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
	Metadata       *OpsServiceMetadata `json:"metadata,omitempty"`
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

// Additional query types

type OpsNodeCommandsQuery struct {
	NodeID string `json:"nodeId"`
}

type OpsNodeActionRequest struct {
	NodeID string `json:"nodeId"`
}

type OpsProcessActionRequest struct {
	AgentID string `json:"agentId"`
	PID     int    `json:"pid"`
	Action  string `json:"action"`
	Name    string `json:"name"`
	Force   bool   `json:"force"`
}

type OpsProcessActionResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type OpsProcessStartRequest struct {
	AgentID string            `json:"agentId"`
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

type OpsProcessStartResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    int32  `json:"data"`
}

// Additional request/response types

type OpsNodesRequest struct {
	GameID string `json:"gameId"`
}

type OpsNodesResponse struct {
	Code    int           `json:"code"`
	Message string        `json:"message"`
	Data    []OpsNodeInfo `json:"data"`
}

type OpsSilenceDeleteResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type OpsExecCommandResult struct {
	Success  bool   `json:"success"`
	ExitCode int32  `json:"exitCode"`
	StdOut   string `json:"stdout"`
	StdErr   string `json:"stderr"`
}

type OpsMetricsData struct {
	Metrics   []OpsMetricPoint    `json:"metrics"`
	Summary   map[string]float64  `json:"summary"`
	AgentID   string              `json:"agentId"`
	Timestamp string              `json:"timestamp"`
	CPU       OpsCpuMetrics       `json:"cpu"`
	Memory    OpsMemoryMetrics    `json:"memory"`
	Disks     []OpsDiskMetrics    `json:"disks"`
	Networks  []OpsNetworkMetrics `json:"networks"`
}

// Corrected and additional types

type OpsAgentMetricsRequestFixed struct {
	AgentID string
	Since   string
	Limit   int
}

// Additional types for agent system info

type OpsAgentSystemInfoFull struct {
	Hostname      string              `json:"hostname"`
	OS            string              `json:"os"`
	Arch          string              `json:"arch"`
	Uptime        int64               `json:"uptime"`
	OSVersion     string              `json:"osVersion"`
	KernelVersion string              `json:"kernelVersion"`
	CPUCores      int32               `json:"cpuCores"`
	TotalMemory   uint64              `json:"totalMemory"`
	CPU           OpsCpuMetrics       `json:"cpu"`
	Memory        OpsMemoryMetrics    `json:"memory"`
	Disk          []OpsDiskMetrics    `json:"disk"`
	Network       []OpsNetworkMetrics `json:"network"`
	Agents        []OpsMetricsData    `json:"agents"`
}

type OpsManagedProcess struct {
	PID          int32   `json:"pid"`
	Name         string  `json:"name"`
	Command      string  `json:"command"`
	Status       string  `json:"status"`
	ParentPID    int32   `json:"parentPid"`
	Memory       uint64  `json:"memory"`
	CPUPercent   float64 `json:"cpuPercent"`
	WorkingDir   string  `json:"workingDir"`
	State        string  `json:"state"`
	Pid          int32   `json:"-"`
	RestartCount int32   `json:"restartCount"`
	LastStart    string  `json:"lastStart"`
}
