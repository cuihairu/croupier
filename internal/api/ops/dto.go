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
	// OwnerInstance is the owning instance this agent connects to in cluster mode (empty when connected to this instance directly).
	OwnerInstance string `json:"ownerInstance,omitempty"`
	// Supervisor is the supervision aggregate light (ok/warn/error + counts); nil when the agent has not reported.
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
	// AgentBinarySha256 is the sha256 of the agent binary contents (the §10.5
	// fingerprint: an anchor for lost-connection triage; empty = the agent has not
	// reported it or the binary is unreadable).
	AgentBinarySha256 string `json:"agentBinarySha256,omitempty"`
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

// Supervisor views (S1 read-only: snapshots come from the SupervisedProcessSnapshot
// piggybacked on agent metrics reports, the MetricsStore's latest report held in
// server memory).

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
	// LastEventUnix is the time of the most recent supervisor event (0 = none);
	// NextRestartAtUnix is non-zero only in the BACKOFF state (the panel renders a
	// backoff countdown). SnapshotProfile is the S3 crash snapshot profile
	// (none/go/node/python/jvm, empty = unknown). Kind is the managed-object
	// category (program/probe-adapter, §10.1); AdapterId is non-empty when
	// kind=probe-adapter.
	LastEventUnix     int64  `json:"lastEventUnix"`
	NextRestartAtUnix int64  `json:"nextRestartAtUnix"`
	SnapshotProfile   string `json:"snapshotProfile,omitempty"`
	Kind              string `json:"kind"`
	AdapterId         string `json:"adapterId,omitempty"`
	// NextScheduleAtUnix/NextScheduleAction is the scheduling state (§10.6): the
	// next scheduled trigger time and action, zero values when there is no schedule
	// entry (visible in advance, so the panel can render a countdown).
	NextScheduleAtUnix int64  `json:"nextScheduleAtUnix,omitempty"`
	NextScheduleAction string `json:"nextScheduleAction,omitempty"`
	// Fingerprint is the sha256 of the running instance's binary (the §10.4
	// fingerprint, computed once at spawn); empty = not running or not computed.
	// Alert/probe timelines and update records align on (target, time, fingerprint).
	Fingerprint string `json:"fingerprint,omitempty"`
	// ScheduleSuspended is the scheduling-paused state (§10.2 pause and resume;
	// what is paused is the schedule): cron is frozen inside a maintenance window
	// while process run states are untouched; always false with no schedule entry.
	ScheduleSuspended bool `json:"scheduleSuspended,omitempty"`
}

type OpsSupervisorSummary struct {
	// Status aggregate light: ok (all RUNNING and no flags) / warn (any non-RUNNING
	// or over-limit flags) / error (any FAILED/BROKEN).
	Status  string `json:"status"`
	Total   int    `json:"total"`
	Running int    `json:"running"`
}

type OpsAgentSupervisorResponse struct {
	AgentID string `json:"agentId"`
	// Timestamp is when the server received the latest metrics report (the panel uses it to judge data freshness).
	Timestamp string                 `json:"timestamp"`
	Processes []OpsSupervisedProcess `json:"processes"`
	Summary   OpsSupervisorSummary   `json:"summary"`
}

// Supervisor event log (S2: events are produced by the agent and piggybacked on
// metrics reports, cached in the server's in-memory MetricsStore ring; the panel
// uses seq as its incremental-pull cursor).

type OpsAgentSupervisorEventsRequest struct {
	AgentID  string `uri:"agentId" form:"agentId"`
	SinceSeq int64  `form:"sinceSeq"`
	Limit    int    `form:"limit"`
}

type OpsSupervisorEvent struct {
	Seq int64  `json:"seq"`
	Ts  string `json:"ts"`
	// TsUnix is the event's raw unix seconds (for localized panel rendering; ts is the server-formatted string).
	TsUnix       int64  `json:"tsUnix,omitempty"`
	Process      string `json:"process"`
	Event        string `json:"event"`
	OldPid       int32  `json:"oldPid"`
	NewPid       int32  `json:"newPid"`
	ExitCode     int32  `json:"exitCode"`
	Signal       string `json:"signal"`
	RestartCount int32  `json:"restartCount"`
	Message      string `json:"message,omitempty"`
	// Event-time context (best-effort diagnostics): lastHeartbeat is the last
	// heartbeat, lastError the process's final error output, oomSuspect the OOM
	// suspicion heuristic, lastRssBytes the last RSS.
	// S3: detect_down carries the crash snapshot directory and artifact file names (best-effort).
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
	// LatestSeq is the newest seq in the server's ring (0 = no events); the panel uses it as its incremental cursor.
	LatestSeq int64 `json:"latestSeq"`
}

// OpsAgentSupervisorLogRequest fetches the agent-local supervisor event log file
// (tail-truncated, streamed back as raw binary, not a JSON envelope).
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
	// Workflow state (risk-event extension, risk-control-panel-design.md §3.3):
	// independent of annotations.status (inbound lifecycle firing/resolved).
	HandleStatus  string `json:"handleStatus,omitempty"`
	HandledBy     string `json:"handledBy,omitempty"`
	HandledRemark string `json:"handledRemark,omitempty"`
	HandledAt     string `json:"handledAt,omitempty"`
}

type OpsAlertSilenceDeleteRequest struct {
	ID string `uri:"id"`
}

type OpsAlertSilenceRequest struct {
	AlertID  string `json:"alertId"`
	Duration int    `json:"duration"` // silence duration (minutes)
}

type OpsAlertSilenceResponse struct {
	SilenceID string `json:"silenceId"`
}

type OpsAlertsRequest struct {
	// HandleStatus filters by workflow state (pending/acknowledged/ignored/escalated).
	HandleStatus string `form:"handleStatus" json:"handleStatus,omitempty"`
}

// OpsAlertHandleRequest drives the risk-event workflow (POST /ops/alerts/:id/handle,
// where :id is the alert primary key). Action: acknowledge acknowledged / ignore ignored /
// escalate escalated; the remark is recorded in the audit trail with the transition
// (the handler comes from the auth context, the time from the DB clock).
type OpsAlertHandleRequest struct {
	Action string `json:"action"`
	Remark string `json:"remark,omitempty"`
}

type OpsAlertHandleResponse struct {
	AlertID      string `json:"alertId"`
	HandleStatus string `json:"handleStatus"`
	HandledBy    string `json:"handledBy"`
	HandledAt    string `json:"handledAt"`
}

type OpsAlertsResponse struct {
	Alerts []OpsAlert `json:"alerts"`
}

// Backup operations DTOs

type Backup struct {
	Id         string `json:"id"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	CreatedAt  string `json:"createdAt"`
	GameId     string `json:"gameId,omitempty"`
	Env        string `json:"env,omitempty"`
	SourceName string `json:"sourceName,omitempty"`
	Checksum   string `json:"checksum,omitempty"`
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
	Type string `json:"type"` // full, incremental (empty = full)
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

// GET uses BindQueryCompat, which only honors form/json tags (not uri tags); the
// handler falls back to c.Param for path parameters (same as OpsNodeDetailRequest).
type OpsNodeMetaRequest struct {
	NodeID string `form:"nodeId" uri:"nodeId"`
}

type OpsNodeMetaResponse struct {
	Labels map[string]string `json:"labels"`
}

// OpsNodeDetailRequest/Response back GET /ops/nodes/:nodeId single-node detail:
// same source as the list (listNodes), filtered by id, identical field set. GET
// uses BindQueryCompat, which only honors form/json tags (not uri tags); the
// handler falls back to c.Param for path parameters.
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

type OpsProcessUpdateRequest struct {
	AgentID        string `uri:"agentId"`
	Name           string `uri:"name"`
	StagedPath     string `json:"stagedPath"`
	ExpectedSha256 string `json:"expectedSha256"`
}

type OpsProcessSchedulePauseRequest struct {
	AgentID string `uri:"agentId"`
	Name    string `uri:"name"`
	Suspend bool   `json:"suspend"`
}

type OpsProcessSchedulePauseResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	Suspended bool   `json:"suspended"`
}

type OpsProcessUpdateResponse struct {
	Success        bool   `json:"success"`
	Message        string `json:"message,omitempty"`
	OldFingerprint string `json:"oldFingerprint,omitempty"`
	NewFingerprint string `json:"newFingerprint,omitempty"`
	RolledBack     bool   `json:"rolledBack"`
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
	// Window selects the storage face: ""|"raw" = agent_metrics_history
	// (7 days), "hourly" = agent_metrics_hourly (90-day aggregation).
	Window string `json:"window"`
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
	// OPS-010's three faces: network rx/tx rates, process list, listening port
	// list. The raw face lands in the agent_metrics_history columns; the hourly
	// aggregation face only reconstructs Networks (mean rates).
	Networks  []NetworkHistoryMetrics `json:"networks,omitempty"`
	Processes []ProcessHistoryMetrics `json:"processes,omitempty"`
	Ports     []PortHistoryMetrics    `json:"ports,omitempty"`
}

// NetworkHistoryMetrics is the per-interface network face of one entry.
type NetworkHistoryMetrics struct {
	Interface     string  `json:"interface"`
	BytesSent     uint64  `json:"bytesSent"`
	BytesRecv     uint64  `json:"bytesRecv"`
	BytesSentRate float64 `json:"bytesSentRate"`
	BytesRecvRate float64 `json:"bytesRecvRate"`
}

// ProcessHistoryMetrics is one process in the top-N process list face.
type ProcessHistoryMetrics struct {
	Pid            int32   `json:"pid"`
	Name           string  `json:"name"`
	Status         string  `json:"status,omitempty"`
	CpuPercent     float64 `json:"cpuPercent,omitempty"`
	MemoryBytes    uint64  `json:"memoryBytes"`
	Threads        int32   `json:"threads,omitempty"`
	StartTime      string  `json:"startTime,omitempty"`
	ListeningPorts []int32 `json:"listeningPorts,omitempty"`
}

// PortHistoryMetrics is one listener in the port list face.
type PortHistoryMetrics struct {
	Port        int32  `json:"port"`
	Protocol    string `json:"protocol"`
	Pid         int32  `json:"pid,omitempty"`
	ProcessName string `json:"processName,omitempty"`
	State       string `json:"state,omitempty"`
}

// AgentLogSearchRequest is used to query agent application log entries
// (OPS-005 log retrieval). AgentID is required for scope isolation.
type AgentLogSearchRequest struct {
	AgentID string `json:"agentId" binding:"required"`
	File    string `json:"file"`
	Level   string `json:"level"`
	Since   string `json:"since"` // ISO 8601, inclusive
	Until   string `json:"until"` // ISO 8601, inclusive
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
}

// AgentLogSearchResponse is the response for a log search.
type AgentLogSearchResponse struct {
	AgentID string              `json:"agentId"`
	Items   []AgentLogEntryView `json:"items"`
	Total   int64               `json:"total"`
}

// AgentLogEntryView is one search result row.
type AgentLogEntryView struct {
	File       string `json:"file"`
	Level      string `json:"level"`
	Message    string `json:"message"`
	TsUnixMs   int64  `json:"tsUnixMs"`
	ReceivedAt string `json:"receivedAt"`
	Seq        int64  `json:"seq"`
}

// BatchOpExecuteRequest POST /ops/batch/:id/execute. DryRun=true only previews the
// target set, nothing is dispatched.
type BatchOpExecuteRequest struct {
	DryRun bool `json:"dryRun"`
}

// BatchOpListRequest GET /ops/batch.
type BatchOpListRequest struct {
	Limit int `form:"limit,optional"`
}

// RunbookListRequest GET /ops/runbooks.
type RunbookListRequest struct {
	Limit int `form:"limit,optional"`
}

// RunbookRunRequest POST /ops/runbooks/:id/run. Params provide the {{param}}
// substitutions for parameterized steps.
type RunbookRunRequest struct {
	Params map[string]string `json:"params"`
}

// RunbookRunListRequest GET /ops/runbook-runs.
type RunbookRunListRequest struct {
	RunbookID string `form:"runbookId,optional"`
	Limit     int    `form:"limit,optional"`
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
	Version      string            `json:"version,omitempty"` // agent binary version (agent_sessions.Version)
	Functions    int               `json:"functions"`
	ExpiresInSec int64             `json:"expiresInSec"`
	// System metrics (from SystemInfoCache)
	CPU    *CpuMetrics    `json:"cpu,omitempty"`
	Memory *MemoryMetrics `json:"memory,omitempty"`
	Disks  []DiskMetrics  `json:"disks,omitempty"`
	// Supervisor is the supervision aggregate light (data source same as
	// CPU/Memory: the MetricsStore's latest report of SupervisedProcesses); nil
	// when there is no report or the agent is not on the local instance.
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
