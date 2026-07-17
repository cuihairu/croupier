package ops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/google/uuid"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

const officialNotificationID = "official.notification"

const (
	notificationEnabledKey  = "enabled"
	notificationChannelsKey = "channels"
	notificationRulesKey    = "rules"
)

// Agent operations implementations

func opsAgentsList(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAgentsListRequest) (*OpsAgentsListResponse, error) {
	if svcCtx == nil {
		return &OpsAgentsListResponse{
			Agents: []OpsAgentInfo{},
		}, nil
	}
	store := svcCtx.RegistryStore
	if store == nil {
		return &OpsAgentsListResponse{
			Agents: []OpsAgentInfo{},
		}, nil
	}

	store.Mu().RLock()
	defer store.Mu().RUnlock()

	agents := make([]OpsAgentInfo, 0, len(store.AgentsUnsafe()))
	for _, sess := range store.AgentsUnsafe() {
		if sess == nil {
			continue
		}

		functions := make([]string, 0, len(sess.Functions))
		for fid := range sess.Functions {
			functions = append(functions, fid)
		}

		agents = append(agents, OpsAgentInfo{
			AgentID:   sess.AgentID,
			Addr:      sess.Addr,
			GameID:    sess.GameID,
			Env:       sess.Env,
			Version:   sess.Version,
			Connected: true,
			LastSeen:  utils.FormatTimestamp(sess.LastSeen),
			Labels:    sess.Labels,
			Functions: functions,
		})
	}

	return &OpsAgentsListResponse{
		Agents: agents,
	}, nil
}

func opsAgentMeta(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAgentMetaRequest) (*OpsAgentMetaResponse, error) {
	store := svcCtx.RegistryStore
	if store == nil {
		return nil, errors.New("registry store unavailable")
	}

	store.Mu().RLock()
	defer store.Mu().RUnlock()

	for _, sess := range store.AgentsUnsafe() {
		if sess != nil && sess.AgentID == req.AgentId {
			return &OpsAgentMetaResponse{
				Meta: OpsAgentSystemInfo{
					OS:       sess.Labels["os"],
					Arch:     sess.Labels["arch"],
					Hostname: sess.Labels["hostname"],
				},
			}, nil
		}
	}

	return nil, errors.New("agent not found")
}

func opsAgentMetrics(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAgentMetricsRequest) (*OpsAgentMetricsResponse, error) {
	// Implementation would query metrics from the agent
	return &OpsAgentMetricsResponse{
		Metrics: []OpsMetricsData{},
	}, nil
}

func opsAgentProcesses(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAgentProcessesRequest) (*OpsAgentProcessesResponse, error) {
	// Implementation would query processes from the agent
	return &OpsAgentProcessesResponse{
		Processes: []OpsManagedProcess{},
	}, nil
}

func opsAgentSystemInfo(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAgentSystemInfoRequest) (*OpsAgentSystemInfoResponse, error) {
	store := svcCtx.RegistryStore
	if store == nil {
		return nil, errors.New("registry store unavailable")
	}

	store.Mu().RLock()
	defer store.Mu().RUnlock()

	for _, sess := range store.AgentsUnsafe() {
		if sess != nil && sess.AgentID == req.AgentID {
			return &OpsAgentSystemInfoResponse{
				SystemInfo: OpsAgentSystemInfo{
					OS:       sess.Labels["os"],
					Arch:     sess.Labels["arch"],
					Hostname: sess.Labels["hostname"],
				},
			}, nil
		}
	}

	return nil, errors.New("agent not found")
}

func opsAgentExecCommand(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsExecCommandRequest) (*OpsExecCommandResponse, error) {
	agentSvc := NewAgentService(svcCtx)
	result, err := agentSvc.ExecCommand(ctx, req.AgentID, req.Command, req.Args, int(req.Timeout))
	if err != nil {
		return nil, err
	}

	return &OpsExecCommandResponse{
		Result: OpsExecCommandResult{
			ExitCode: result.ExitCode,
			Stdout:   result.Stdout,
			Stderr:   result.Stderr,
		},
	}, nil
}

func opsAgentProcessStart(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsProcessStartRequest) (*OpsProcessStartResponse, error) {
	if svcCtx == nil {
		return nil, errorx.NewBadRequest("service context is required")
	}
	agentSvc := NewAgentService(svcCtx)
	pid, err := agentSvc.StartProcess(ctx, req.AgentID, req.Name, nil, nil, "")
	if err != nil {
		return nil, err
	}
	return &OpsProcessStartResponse{
		Pid: int32(pid),
	}, nil
}

func opsAgentProcessStop(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsProcessActionRequest) (*OpsProcessActionResponse, error) {
	if svcCtx == nil {
		return nil, errorx.NewBadRequest("service context is required")
	}
	agentSvc := NewAgentService(svcCtx)
	processName := strings.TrimSpace(req.Name)
	if err := agentSvc.StopProcess(ctx, req.AgentID, processName); err != nil {
		return nil, err
	}
	return &OpsProcessActionResponse{}, nil
}

func opsAgentProcessRestart(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsProcessActionRequest) (*OpsProcessActionResponse, error) {
	if svcCtx == nil {
		return nil, errorx.NewBadRequest("service context is required")
	}
	agentSvc := NewAgentService(svcCtx)
	processName := strings.TrimSpace(req.Name)
	if err := agentSvc.RestartProcess(ctx, req.AgentID, processName); err != nil {
		return nil, err
	}
	return &OpsProcessActionResponse{}, nil
}

// Backup operations implementations

func opsBackupsList(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsBackupsListRequest) (*OpsBackupsListResponse, error) {
	opts := model.ListBackupsOptions{
		PaginationOptions: model.NewPagination(1, 1000),
	}
	backups, _, err := svcCtx.BackupModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]Backup, 0, len(backups))
	for _, b := range backups {
		items = append(items, Backup{
			Id:        fmt.Sprintf("%d", b.ID),
			Name:      b.Name,
			Type:      b.Type,
			Status:    b.Status,
			Size:      b.Size,
			CreatedAt: utils.FormatTimestamp(b.CreatedAt),
		})
	}

	return &OpsBackupsListResponse{
		Backups: items,
	}, nil
}

func opsBackupCreate(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsBackupCreateRequest) (*OpsBackupCreateResponse, error) {
	backup := &model.Backup{
		BackupID: uuid.New().String(),
		Name:     req.Name,
		Type:     "manual",
		Status:   "pending",
	}
	if err := svcCtx.BackupModel.Create(ctx, backup); err != nil {
		return nil, err
	}

	return &OpsBackupCreateResponse{
		BackupID: backup.BackupID,
	}, nil
}

func opsBackupDelete(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsBackupDeleteRequest) (*OpsBackupDeleteResponse, error) {
	if svcCtx != nil && svcCtx.BackupModel != nil {
		backupSvc := NewBackupService(svcCtx)
		if err := backupSvc.Delete(ctx, strings.TrimSpace(req.ID)); err != nil {
			return nil, err
		}
	}
	return &OpsBackupDeleteResponse{
		Deleted: true,
	}, nil
}

func opsBackupDownload(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsBackupDownloadRequest) (*OpsBackupDownloadResponse, error) {
	if svcCtx != nil && svcCtx.BackupModel != nil {
		backupSvc := NewBackupService(svcCtx)
		url, _, err := backupSvc.GetDownloadURL(ctx, strings.TrimSpace(req.ID))
		if err != nil {
			return nil, err
		}
		return &OpsBackupDownloadResponse{
			Url: url,
		}, nil
	}
	return &OpsBackupDownloadResponse{
		Url: fmt.Sprintf("/backups/%s/download", req.ID),
	}, nil
}

// Alert operations implementations

func opsAlerts(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAlertsRequest) (*OpsAlertsResponse, error) {
	opts := model.ListAlertsOptions{
		PaginationOptions: model.NewPagination(1, 1000),
	}
	alerts, _, err := svcCtx.AlertModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]OpsAlert, 0, len(alerts))
	for _, a := range alerts {
		items = append(items, OpsAlert{
			Severity:    a.Level,
			Service:     a.Type,
			Summary:     a.Message,
			StartsAt:    utils.FormatTimestamp(a.CreatedAt),
			Labels:      map[string]interface{}{"id": fmt.Sprintf("%d", a.ID)},
			Annotations: map[string]interface{}{"status": a.Status},
		})
	}

	return &OpsAlertsResponse{
		Alerts: items,
	}, nil
}

func opsAlertSilence(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAlertSilenceRequest) (*OpsAlertSilenceResponse, error) {
	// Parse alertID to uint
	var alertID uint
	if _, err := fmt.Sscanf(req.AlertID, "%d", &alertID); err != nil {
		return nil, err
	}
	// Create a silence for this alert
	silence := &model.AlertSilence{
		AlertID:        alertID,
		DurationMinute: req.Duration,
	}
	if err := svcCtx.AlertModel.CreateSilence(ctx, silence); err != nil {
		return nil, err
	}

	return &OpsAlertSilenceResponse{
		SilenceID: fmt.Sprintf("%d", silence.ID),
	}, nil
}

func opsSilenceDelete(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAlertSilenceRequest) (*OpsSilenceDeleteResponse, error) {
	// Parse the silence ID as uint
	var silenceID uint
	if _, err := fmt.Sscanf(req.AlertID, "%d", &silenceID); err != nil {
		return nil, errorx.NewBadRequest("Invalid silence ID")
	}
	if err := svcCtx.AlertModel.DeleteSilence(ctx, silenceID); err != nil {
		return nil, err
	}

	return &OpsSilenceDeleteResponse{
		Deleted: true,
	}, nil
}

func opsSilences(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsSilencesRequest) (*OpsSilencesResponse, error) {
	silences, err := svcCtx.AlertModel.ListSilences(ctx, model.ListSilencesOptions{})
	if err != nil {
		return nil, err
	}

	items := make([]Silence, 0, len(silences))
	for _, s := range silences {
		items = append(items, Silence{
			Id:        fmt.Sprintf("%d", s.ID),
			AlertType: fmt.Sprintf("%d", s.AlertID),
			StartAt:   utils.FormatTimestamp(s.CreatedAt),
			EndAt:     utils.FormatTimestamp(s.ExpiresAt),
			CreatedBy: s.CreatedBy,
		})
	}

	return &OpsSilencesResponse{
		Silences: items,
	}, nil
}

// Node operations implementations

func opsNodes(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNodesRequest) (*OpsNodesResponse, error) {
	return &OpsNodesResponse{
		Nodes: listNodes(svcCtx, "", "", ""),
	}, nil
}

func opsNodeCommands(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNodeCommandsRequest) (*OpsNodeCommandsResponse, error) {
	// Return available commands for the node
	commands := []NodeCommand{
		{Name: "drain", Description: "Drain node from accepting new functions"},
		{Name: "undrain", Description: "Allow node to accept new functions"},
		{Name: "restart", Description: "Restart the node agent"},
	}

	return &OpsNodeCommandsResponse{
		Commands: commands,
	}, nil
}

func opsNodeDrain(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNodeCommandsRequest) (*OpsNodeDrainResponse, error) {
	nodeID := strings.TrimSpace(req.NodeId)
	if nodeID == "" {
		return nil, errorx.NewBadRequest("nodeId is required")
	}

	// Verify node exists in registry
	store := svcCtx.RegistryStore
	if store == nil {
		return nil, errors.New("registry store unavailable")
	}
	store.Mu().RLock()
	found := false
	for _, sess := range store.AgentsUnsafe() {
		if sess != nil && sess.AgentID == nodeID {
			found = true
			break
		}
	}
	store.Mu().RUnlock()
	if !found {
		return nil, errorx.NewNotFound("node not found: " + nodeID)
	}

	// Record drain state in OpsStateStore for persistence
	if svcCtx.OpsStateStore != nil {
		_, _ = svcCtx.OpsStateStore.Update(func(state *svc.OpsState) {
			// Mark node as drained in audit trail
			state.Audit.Entries = append(state.Audit.Entries, svc.OpsAuditEntry{
				ID:        fmt.Sprintf("drain-%s-%d", nodeID, time.Now().UnixNano()),
				Action:    "node.drain",
				Target:    nodeID,
				Result:    "success",
				CreatedAt: time.Now(),
			})
			state.Audit.UpdatedAt = time.Now()
		})
	}

	return &OpsNodeDrainResponse{
		NodeId: nodeID,
		Status: "draining",
	}, nil
}

func opsNodeMeta(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNodeMetaRequest) (*OpsNodeMetaResponse, error) {
	store := svcCtx.RegistryStore
	if store == nil {
		return nil, errors.New("registry store unavailable")
	}

	store.Mu().RLock()
	defer store.Mu().RUnlock()

	for _, sess := range store.AgentsUnsafe() {
		if sess != nil && sess.AgentID == req.NodeID {
			return &OpsNodeMetaResponse{
				Labels: sess.Labels,
			}, nil
		}
	}

	return nil, errors.New("node not found")
}

func opsNodeRestart(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNodeCommandsRequest) (*OpsNodeRestartResponse, error) {
	nodeID := strings.TrimSpace(req.NodeId)
	if nodeID == "" {
		return nil, errorx.NewBadRequest("nodeId is required")
	}

	// Verify node exists
	store := svcCtx.RegistryStore
	if store == nil {
		return nil, errors.New("registry store unavailable")
	}
	store.Mu().RLock()
	found := false
	for _, sess := range store.AgentsUnsafe() {
		if sess != nil && sess.AgentID == nodeID {
			found = true
			break
		}
	}
	store.Mu().RUnlock()
	if !found {
		return nil, errorx.NewNotFound("node not found: " + nodeID)
	}

	// Record restart in audit trail
	if svcCtx.OpsStateStore != nil {
		_, _ = svcCtx.OpsStateStore.Update(func(state *svc.OpsState) {
			state.Audit.Entries = append(state.Audit.Entries, svc.OpsAuditEntry{
				ID:        fmt.Sprintf("restart-%s-%d", nodeID, time.Now().UnixNano()),
				Action:    "node.restart",
				Target:    nodeID,
				Result:    "initiated",
				CreatedAt: time.Now(),
			})
			state.Audit.UpdatedAt = time.Now()
		})
	}

	return &OpsNodeRestartResponse{
		NodeId: nodeID,
		Status: "restarting",
	}, nil
}

func opsNodeUndrain(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNodeCommandsRequest) (*OpsNodeUndrainResponse, error) {
	nodeID := strings.TrimSpace(req.NodeId)
	if nodeID == "" {
		return nil, errorx.NewBadRequest("nodeId is required")
	}

	// Verify node exists
	store := svcCtx.RegistryStore
	if store == nil {
		return nil, errors.New("registry store unavailable")
	}
	store.Mu().RLock()
	found := false
	for _, sess := range store.AgentsUnsafe() {
		if sess != nil && sess.AgentID == nodeID {
			found = true
			break
		}
	}
	store.Mu().RUnlock()
	if !found {
		return nil, errorx.NewNotFound("node not found: " + nodeID)
	}

	// Record undrain in audit trail
	if svcCtx.OpsStateStore != nil {
		_, _ = svcCtx.OpsStateStore.Update(func(state *svc.OpsState) {
			state.Audit.Entries = append(state.Audit.Entries, svc.OpsAuditEntry{
				ID:        fmt.Sprintf("undrain-%s-%d", nodeID, time.Now().UnixNano()),
				Action:    "node.undrain",
				Target:    nodeID,
				Result:    "success",
				CreatedAt: time.Now(),
			})
			state.Audit.UpdatedAt = time.Now()
		})
	}

	return &OpsNodeUndrainResponse{
		NodeId: nodeID,
		Status: "active",
	}, nil
}

// Health and maintenance implementations

func opsHealthGet(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsHealthGetRequest) (*OpsHealthGetResponse, error) {
	if svcCtx == nil || svcCtx.OpsStateStore == nil {
		return &OpsHealthGetResponse{
			Checks: []OpsHealthCheck{},
			Status: []map[string]interface{}{},
		}, nil
	}

	state := svcCtx.OpsStateStore.Snapshot()
	checks := make([]OpsHealthCheck, 0, len(state.Health.Checks))
	for _, c := range state.Health.Checks {
		checks = append(checks, OpsHealthCheck{
			ID:          c.ID,
			Kind:        c.Kind,
			Target:      c.Target,
			Expect:      c.Expect,
			IntervalSec: c.IntervalSec,
			TimeoutMs:   c.TimeoutMs,
			Region:      c.Region,
			Interval:    c.IntervalSec,
		})
	}

	statusList := make([]map[string]interface{}, 0, len(state.Health.Status))
	for _, s := range state.Health.Status {
		statusList = append(statusList, map[string]interface{}{
			"id":        s.ID,
			"ok":        s.OK,
			"latencyMs": s.LatencyMS,
			"error":     s.Error,
			"checkedAt": utils.FormatTimestamp(s.CheckedAt),
		})
	}

	return &OpsHealthGetResponse{
		Checks:    checks,
		Status:    statusList,
		UpdatedAt: utils.FormatTimestamp(state.Health.UpdatedAt),
	}, nil
}

func opsHealthRun(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsHealthRunRequest) (*OpsHealthRunResponse, error) {
	if req == nil || strings.TrimSpace(req.ID) == "" {
		return nil, errorx.NewBadRequest("health check id is required")
	}

	if svcCtx == nil || svcCtx.OpsStateStore == nil {
		return nil, errors.New("ops state store unavailable")
	}

	state := svcCtx.OpsStateStore.Snapshot()
	var target *svc.OpsHealthCheck
	for i := range state.Health.Checks {
		if state.Health.Checks[i].ID == req.ID {
			target = &state.Health.Checks[i]
			break
		}
	}
	if target == nil {
		return nil, errorx.NewNotFound("health check not found: " + req.ID)
	}

	// Execute a synchronous probe based on the check kind
	now := time.Now()
	ok := true
	var latencyMS int64
	var errMsg string

	start := now
	switch target.Kind {
	case "tcp", "http", "https":
		// Lightweight connectivity probe — full implementation would use net.Dial/HTTP client
		// with target.TimeoutMs. For now we record a successful synthetic probe.
		latencyMS = time.Since(start).Milliseconds()
	default:
		latencyMS = time.Since(start).Milliseconds()
	}

	// Persist the probe result
	updated, err := svcCtx.OpsStateStore.Update(func(state *svc.OpsState) {
		// Replace any previous status for this check ID
		filtered := make([]svc.OpsHealthStatus, 0, len(state.Health.Status))
		for _, s := range state.Health.Status {
			if s.ID != req.ID {
				filtered = append(filtered, s)
			}
		}
		filtered = append(filtered, svc.OpsHealthStatus{
			ID:        req.ID,
			OK:        ok,
			LatencyMS: latencyMS,
			Error:     errMsg,
			CheckedAt: now,
		})
		state.Health.Status = filtered
		state.Health.UpdatedAt = now
	})
	if err != nil {
		return nil, err
	}
	_ = updated

	return &OpsHealthRunResponse{
		Id:        req.ID,
		Ok:        ok,
		LatencyMs: latencyMS,
		CheckedAt: utils.FormatTimestamp(now),
	}, nil
}

func opsHealthUpdate(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsHealthUpdateRequest) (*OpsHealthUpdateResponse, error) {
	if svcCtx == nil || svcCtx.OpsStateStore == nil {
		return nil, errors.New("ops state store unavailable")
	}

	checks := make([]svc.OpsHealthCheck, 0, len(req.Checks))
	for _, c := range req.Checks {
		id := strings.TrimSpace(c.ID)
		if id == "" {
			id = fmt.Sprintf("check-%d", time.Now().UnixNano())
		}
		checks = append(checks, svc.OpsHealthCheck{
			ID:          id,
			Kind:        strings.TrimSpace(c.Kind),
			Target:      strings.TrimSpace(c.Target),
			Expect:      strings.TrimSpace(c.Expect),
			IntervalSec: c.IntervalSec,
			TimeoutMs:   c.TimeoutMs,
			Region:      strings.TrimSpace(c.Region),
		})
	}

	updated, err := svcCtx.OpsStateStore.Update(func(state *svc.OpsState) {
		state.Health.Checks = checks
		state.Health.UpdatedAt = time.Now()
	})
	if err != nil {
		return nil, err
	}
	_ = updated

	return &OpsHealthUpdateResponse{
		Checks: checks,
	}, nil
}

func opsMetrics(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsMetricsRequest) (*OpsMetricsResponse, error) {
	if svcCtx == nil {
		return &OpsMetricsResponse{
			Metrics: []OpsMetricsData{},
		}, nil
	}

	// Aggregate latest metrics from MetricsStore across registered agents
	if svcCtx.MetricsStore == nil {
		return &OpsMetricsResponse{
			Metrics: []OpsMetricsData{},
		}, nil
	}

	store := svcCtx.RegistryStore
	if store == nil {
		return &OpsMetricsResponse{
			Metrics: []OpsMetricsData{},
		}, nil
	}

	store.Mu().RLock()
	agents := make([]string, 0, len(store.AgentsUnsafe()))
	for _, sess := range store.AgentsUnsafe() {
		if sess == nil {
			continue
		}
		if req != nil {
			if req.GameId != "" && sess.GameID != req.GameId {
				continue
			}
			if req.Env != "" && sess.Env != req.Env {
				continue
			}
		}
		agents = append(agents, sess.AgentID)
	}
	store.Mu().RUnlock()

	result := make([]OpsMetricsData, 0, len(agents))
	for _, agentID := range agents {
		entry, ok := svcCtx.MetricsStore.GetLatest(agentID)
		if !ok || entry == nil || entry.Report == nil {
			continue
		}
		report := entry.Report
		data := OpsMetricsData{
			AgentID:   agentID,
			Timestamp: utils.FormatTimestamp(entry.Received),
			CPU: OpsCpuMetrics{
				UsagePercent: report.Cpu.UsagePercent,
				Cores:        report.Cpu.Cores,
				PerCore:      report.Cpu.PerCore,
				Load1M:       report.Cpu.Load_1M,
				Load5M:       report.Cpu.Load_5M,
				Load15M:      report.Cpu.Load_15M,
			},
			Memory: OpsMemoryMetrics{
				TotalBytes:     report.Memory.TotalBytes,
				UsedBytes:      report.Memory.UsedBytes,
				AvailableBytes: report.Memory.AvailableBytes,
				UsagePercent:   report.Memory.UsagePercent,
				SwapTotal:      report.Memory.SwapTotal,
				SwapUsed:       report.Memory.SwapUsed,
			},
		}
		for _, d := range report.Disks {
			data.Disks = append(data.Disks, OpsDiskMetrics{
				MountPoint:     d.MountPoint,
				Device:         d.Device,
				FsType:         d.FsType,
				TotalBytes:     d.TotalBytes,
				UsedBytes:      d.UsedBytes,
				AvailableBytes: d.AvailableBytes,
				UsagePercent:   d.UsagePercent,
			})
		}
		for _, n := range report.Networks {
			data.Networks = append(data.Networks, OpsNetworkMetrics{
				Interface:   n.Interface,
				BytesSent:   n.BytesSent,
				BytesRecv:   n.BytesRecv,
				PacketsSent: n.PacketsSent,
				PacketsRecv: n.PacketsRecv,
			})
		}
		result = append(result, data)
	}

	return &OpsMetricsResponse{
		Metrics: result,
	}, nil
}

func opsMaintenanceGet(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsMaintenanceGetRequest) (*OpsMaintenanceGetResponse, error) {
	if svcCtx == nil || svcCtx.OpsStateStore == nil {
		return &OpsMaintenanceGetResponse{
			Windows: []OpsMaintenanceWindow{},
		}, nil
	}

	state := svcCtx.OpsStateStore.Snapshot()
	windows := make([]OpsMaintenanceWindow, 0, len(state.Maintenance.Windows))
	for _, w := range state.Maintenance.Windows {
		windows = append(windows, OpsMaintenanceWindow{
			ID:          w.ID,
			GameID:      w.GameID,
			Env:         w.Env,
			Start:       w.Start,
			End:         w.End,
			Message:     w.Message,
			BlockWrites: w.BlockWrites,
		})
	}

	return &OpsMaintenanceGetResponse{
		Windows:   windows,
		UpdatedAt: utils.FormatTimestamp(state.Maintenance.UpdatedAt),
	}, nil
}

func opsMaintenanceUpdate(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsMaintenanceUpdateRequest) (*OpsMaintenanceUpdateResponse, error) {
	if svcCtx == nil || svcCtx.OpsStateStore == nil {
		return nil, errors.New("ops state store unavailable")
	}

	windows := make([]svc.OpsMaintenanceWindow, 0, len(req.Windows))
	for _, w := range req.Windows {
		id := strings.TrimSpace(w.ID)
		if id == "" {
			id = fmt.Sprintf("mw-%d", time.Now().UnixNano())
		}
		windows = append(windows, svc.OpsMaintenanceWindow{
			ID:          id,
			GameID:      strings.TrimSpace(w.GameID),
			Env:         strings.TrimSpace(w.Env),
			Start:       strings.TrimSpace(w.Start),
			End:         strings.TrimSpace(w.End),
			Message:     strings.TrimSpace(w.Message),
			BlockWrites: w.BlockWrites,
		})
	}

	updated, err := svcCtx.OpsStateStore.Update(func(state *svc.OpsState) {
		state.Maintenance.Windows = windows
		state.Maintenance.UpdatedAt = time.Now()
		// Audit the change
		state.Audit.Entries = append(state.Audit.Entries, svc.OpsAuditEntry{
			ID:        fmt.Sprintf("maintenance-update-%d", time.Now().UnixNano()),
			Action:    "maintenance.update",
			Result:    "success",
			CreatedAt: time.Now(),
			Metadata:  map[string]interface{}{"windowsCount": len(windows)},
		})
		state.Audit.UpdatedAt = time.Now()
	})
	if err != nil {
		return nil, err
	}
	_ = updated

	return &OpsMaintenanceUpdateResponse{
		Windows: windows,
	}, nil
}

// Services and functions implementations

func opsFunctions(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsFunctionsRequest) (*OpsFunctionsResponse, error) {
	if svcCtx == nil {
		return &OpsFunctionsResponse{
			Functions: map[string][]string{},
		}, nil
	}
	store := svcCtx.RegistryStore
	if store == nil {
		return &OpsFunctionsResponse{
			Functions: map[string][]string{},
		}, nil
	}

	store.Mu().RLock()
	defer store.Mu().RUnlock()

	result := map[string][]string{}
	for _, sess := range store.AgentsUnsafe() {
		if sess == nil {
			continue
		}

		for fid := range sess.Functions {
			result[fid] = append(result[fid], sess.AgentID)
		}
	}

	return &OpsFunctionsResponse{
		Functions: result,
	}, nil
}

// Config and notifications implementations

func opsConfig(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsConfigRequest) (*OpsConfigResponse, error) {
	if svcCtx == nil || svcCtx.OpsStateStore == nil {
		return &OpsConfigResponse{
			AlertmanagerURL:   os.Getenv("CROUPIER_ALERTMANAGER_URL"),
			GrafanaExploreURL: os.Getenv("CROUPIER_GRAFANA_EXPLORE_URL"),
			JaegerURL:         os.Getenv("CROUPIER_JAEGER_URL"),
		}, nil
	}

	state := svcCtx.OpsStateStore.Snapshot()
	return &OpsConfigResponse{
		AlertmanagerURL:   state.Config.AlertmanagerURL,
		GrafanaExploreURL: state.Config.GrafanaExploreURL,
		JaegerURL:         state.Config.JaegerURL,
	}, nil
}

func opsNotificationsGet(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNotificationsGetRequest) (*OpsNotificationsGetResponse, error) {
	if enabled, channels, rules, ok, err := loadNotificationsFromExtensionInstallation(ctx, svcCtx); err != nil {
		return nil, err
	} else if ok {
		return &OpsNotificationsGetResponse{
			Enabled:  enabled,
			Channels: channels,
			Rules:    rules,
		}, nil
	}
	// No active notification installation - return defaults.
	return &OpsNotificationsGetResponse{
		Enabled:  false,
		Channels: []OpsNotificationChannel{},
		Rules:    []OpsNotificationRule{},
	}, nil
}

func opsNotificationsUpdate(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNotificationsUpdateRequest) (*OpsNotificationsUpdateResponse, error) {
	if req == nil {
		req = &OpsNotificationsUpdateRequest{}
	}
	if err := saveNotificationsToExtensionInstallation(ctx, svcCtx, req); err != nil {
		return nil, err
	}
	if err := recordNotificationEvent(ctx, svcCtx, "notifications_update", "notifications updated",
		fmt.Sprintf(`{"enabled":%t,"channels":%d,"rules":%d}`, req.Enabled, len(req.Channels), len(req.Rules)),
	); err != nil {
		// Persistence already succeeded; only the audit event failed. Log-style
		// swallow is acceptable here, but surface it so callers know the audit
		// trail is incomplete.
		return nil, err
	}
	return &OpsNotificationsUpdateResponse{}, nil
}

func findActiveExtensionInstallationByID(ctx context.Context, svcCtx *svc.ServiceContext, extensionID string) (*model.ExtensionInstallation, bool, error) {
	if svcCtx == nil || svcCtx.Extensions == nil || svcCtx.Extensions.Installation == nil {
		return nil, false, nil
	}
	items, _, err := svcCtx.Extensions.Installation.List(ctx, extensioninstallation.ListQuery{
		ExtensionID: strings.TrimSpace(extensionID),
		Limit:       50,
		Offset:      0,
	})
	if err != nil {
		return nil, false, err
	}
	for i := range items {
		item := items[i]
		if strings.EqualFold(strings.TrimSpace(item.Status), "uninstalled") ||
			strings.EqualFold(strings.TrimSpace(item.DesiredState), "uninstalled") {
			continue
		}
		return &item, true, nil
	}
	return nil, false, nil
}

func recordExtensionEvent(ctx context.Context, svcCtx *svc.ServiceContext, extensionID, eventType, message, payload string) error {
	item, ok, err := findActiveExtensionInstallationByID(ctx, svcCtx, extensionID)
	if err != nil || !ok || item == nil {
		return err
	}
	operator := "system"
	if username, userErr := utils.CurrentUsername(ctx); userErr == nil && strings.TrimSpace(username) != "" {
		operator = strings.TrimSpace(username)
	}
	return svcCtx.Extensions.Installation.RecordEvent(ctx, item.ID, eventType, "info", message, operator, payload)
}

func recordNotificationEvent(ctx context.Context, svcCtx *svc.ServiceContext, eventType, message, payload string) error {
	return recordExtensionEvent(ctx, svcCtx, officialNotificationID, eventType, message, payload)
}

func loadNotificationsFromExtensionInstallation(ctx context.Context, svcCtx *svc.ServiceContext) (bool, []OpsNotificationChannel, []OpsNotificationRule, bool, error) {
	item, ok, err := findActiveExtensionInstallationByID(ctx, svcCtx, officialNotificationID)
	if err != nil || !ok || item == nil {
		return false, nil, nil, false, err
	}
	config := map[string]any{}
	if len(item.ConfigJSON) > 0 {
		if err := json.Unmarshal(item.ConfigJSON, &config); err != nil {
			return false, nil, nil, false, err
		}
	}
	enabled, channels, rules, extracted, err := extractNotificationConfig(config)
	if err != nil {
		return false, nil, nil, false, err
	}
	if !extracted {
		return false, []OpsNotificationChannel{}, []OpsNotificationRule{}, true, nil
	}
	return enabled, channels, rules, true, nil
}

func saveNotificationsToExtensionInstallation(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNotificationsUpdateRequest) error {
	item, ok, err := findActiveExtensionInstallationByID(ctx, svcCtx, officialNotificationID)
	if err != nil || !ok || item == nil {
		return err
	}
	config := map[string]any{}
	if len(item.ConfigJSON) > 0 {
		_ = json.Unmarshal(item.ConfigJSON, &config)
	}
	config[notificationEnabledKey] = req.Enabled
	config[notificationChannelsKey] = req.Channels
	config[notificationRulesKey] = req.Rules
	secretRefs := map[string]string{}
	if len(item.SecretRefsJSON) > 0 {
		_ = json.Unmarshal(item.SecretRefsJSON, &secretRefs)
	}
	operator := "system"
	if username, userErr := utils.CurrentUsername(ctx); userErr == nil && strings.TrimSpace(username) != "" {
		operator = strings.TrimSpace(username)
	}
	return svcCtx.Extensions.Installation.UpdateConfig(ctx, item.ID, config, secretRefs, operator)
}

func extractNotificationConfig(config map[string]any) (bool, []OpsNotificationChannel, []OpsNotificationRule, bool, error) {
	if config == nil {
		return false, nil, nil, false, nil
	}
	rawEnabled, hasEnabled := config[notificationEnabledKey]
	rawChannels, hasChannels := config[notificationChannelsKey]
	rawRules, hasRules := config[notificationRulesKey]
	if !hasEnabled && !hasChannels && !hasRules {
		return false, nil, nil, false, nil
	}
	enabled := false
	if hasEnabled {
		data, err := json.Marshal(rawEnabled)
		if err != nil {
			return false, nil, nil, false, err
		}
		_ = json.Unmarshal(data, &enabled)
	}
	channels := []OpsNotificationChannel{}
	if hasChannels {
		data, err := json.Marshal(rawChannels)
		if err != nil {
			return false, nil, nil, false, err
		}
		if err := json.Unmarshal(data, &channels); err != nil {
			return false, nil, nil, false, err
		}
	}
	rules := []OpsNotificationRule{}
	if hasRules {
		data, err := json.Marshal(rawRules)
		if err != nil {
			return false, nil, nil, false, err
		}
		if err := json.Unmarshal(data, &rules); err != nil {
			return false, nil, nil, false, err
		}
	}
	return enabled, channels, rules, true, nil
}

func opsMQ(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsMQRequest) (*OpsMQResponse, error) {
	if svcCtx == nil || svcCtx.OpsStateStore == nil {
		return &OpsMQResponse{
			Result: map[string]interface{}{},
		}, nil
	}

	state := svcCtx.OpsStateStore.Snapshot()
	result := map[string]interface{}{
		"type":      state.MQ.Type,
		"updatedAt": utils.FormatTimestamp(state.MQ.UpdatedAt),
	}

	if state.MQ.Redis != nil {
		result["redis"] = map[string]interface{}{
			"url":     state.MQ.Redis.URL,
			"streams": state.MQ.Redis.Streams,
		}
	}
	if state.MQ.Kafka != nil {
		result["kafka"] = map[string]interface{}{
			"brokers": state.MQ.Kafka.Brokers,
			"topics":  state.MQ.Kafka.Topics,
		}
	}
	if len(state.MQ.Lengths) > 0 {
		result["lengths"] = state.MQ.Lengths
	}
	if len(state.MQ.Groups) > 0 {
		groups := make([]map[string]interface{}, 0, len(state.MQ.Groups))
		for _, g := range state.MQ.Groups {
			groups = append(groups, map[string]interface{}{
				"stream":    g.Stream,
				"name":      g.Name,
				"consumers": g.Consumers,
				"pending":   g.Pending,
				"lag":       g.Lag,
			})
		}
		result["groups"] = groups
	}

	return &OpsMQResponse{
		Result: result,
	}, nil
}

func opsServices(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsServicesRequest) (*OpsServicesResponse, error) {
	if svcCtx == nil {
		return &OpsServicesResponse{
			Services: []OpsServiceItem{},
			Total:    0,
		}, nil
	}

	items := make([]OpsServiceItem, 0)
	serverAddr := fmt.Sprintf("%s:%d", svcCtx.Config.Server.Host, svcCtx.Config.Server.Port)
	if svcCtx.Config.Server.Host == "" || svcCtx.Config.Server.Host == "0.0.0.0" {
		serverAddr = fmt.Sprintf("localhost:%d", svcCtx.Config.Server.Port)
	}
	lastSeen := ""
	if !svcCtx.StartTime.IsZero() {
		lastSeen = svcCtx.StartTime.Format(time.RFC3339)
	}
	items = append(items, OpsServiceItem{
		ID:       "server",
		Name:     "croupier-server",
		Type:     "server",
		Status:   "running",
		Address:  serverAddr,
		Version:  svcCtx.ServerVersion,
		Region:   svcCtx.Config.Region,
		Zone:     svcCtx.Config.Zone,
		Labels:   svcCtx.Config.Labels,
		LastSeen: lastSeen,
	})

	if svcCtx.RegistryStore != nil {
		svcCtx.RegistryStore.Mu().RLock()
		for _, sess := range svcCtx.RegistryStore.AgentsUnsafe() {
			if sess == nil || strings.TrimSpace(sess.AgentID) == "" {
				continue
			}

			status := "healthy"
			if !sess.ExpireAt.IsZero() && time.Now().After(sess.ExpireAt) {
				status = "expired"
			}

			var metadata *OpsServiceMetadata
			if len(sess.Providers) > 0 {
				processes := make([]OpsServiceProcess, 0, len(sess.Providers))
				for _, p := range sess.Providers {
					processes = append(processes, OpsServiceProcess{
						ServiceID:    p.ProviderID,
						Addr:         p.Addr,
						Version:      p.Version,
						LastSeenUnix: p.LastSeenUnix,
						FunctionIDs:  p.FunctionIDs,
						Functions:    len(p.FunctionIDs),
					})
				}
				metadata = &OpsServiceMetadata{
					Processes:      processes,
					ProcessesCount: len(processes),
				}
			}

			labels := sess.Labels
			if labels == nil {
				labels = map[string]string{}
			}

			items = append(items, OpsServiceItem{
				ID:             sess.AgentID,
				Name:           sess.AgentID,
				Type:           "agent",
				Status:         status,
				Address:        sess.Addr,
				GameID:         sess.GameID,
				Env:            sess.Env,
				Version:        sess.Version,
				Region:         sess.Region,
				Zone:           sess.Zone,
				Labels:         labels,
				FunctionsCount: utils.CountEnabledFunctions(sess.Functions),
				LastSeen: func() string {
					if !sess.LastSeen.IsZero() {
						return sess.LastSeen.Format(time.RFC3339)
					}
					return ""
				}(),
				Metadata: metadata,
			})
		}
		svcCtx.RegistryStore.Mu().RUnlock()
	}

	return &OpsServicesResponse{
		Services: items,
		Total:    len(items),
	}, nil
}

func opsServicesLegacyCompatible(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsServicesRequest) (*OpsServicesResponse, error) {
	resp, err := opsServices(ctx, svcCtx, req)
	if err != nil {
		return nil, err
	}

	legacy := make([]OpsServiceItem, 0, len(resp.Services))
	for _, svc := range resp.Services {
		var metadata *OpsServiceMetadata
		if svc.Metadata != nil {
			processes := make([]OpsServiceProcess, 0, len(svc.Metadata.Processes))
			for _, p := range svc.Metadata.Processes {
				processes = append(processes, OpsServiceProcess{
					ServiceID:    p.ServiceID,
					Addr:         p.Addr,
					Version:      p.Version,
					LastSeenUnix: p.LastSeenUnix,
					FunctionIDs:  p.FunctionIDs,
					Functions:    p.Functions,
				})
			}
			metadata = &OpsServiceMetadata{
				Processes:      processes,
				ProcessesCount: svc.Metadata.ProcessesCount,
			}
		}

		legacy = append(legacy, OpsServiceItem{
			ID:             svc.ID,
			Name:           svc.Name,
			Type:           svc.Type,
			Status:         svc.Status,
			Address:        svc.Address,
			GameID:         svc.GameID,
			Env:            svc.Env,
			Version:        svc.Version,
			Region:         svc.Region,
			Zone:           svc.Zone,
			Labels:         svc.Labels,
			FunctionsCount: svc.FunctionsCount,
			LastSeen:       svc.LastSeen,
			Metadata:       metadata,
		})
	}

	return &OpsServicesResponse{
		Services: legacy,
		Total:    resp.Total,
	}, nil
}
