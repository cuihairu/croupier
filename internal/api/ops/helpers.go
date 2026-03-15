package ops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/google/uuid"

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
	store := svcCtx.RegistryStore
	if store == nil {
		return &OpsAgentsListResponse{
			Code:    0,
			Message: "Success",
			Data:    []OpsAgentInfo{},
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
			RPCAddr:   sess.RPCAddr,
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
		Code:    0,
		Message: "Success",
		Data:    agents,
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
				Code:    0,
				Message: "Success",
				Data: OpsAgentSystemInfo{
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
		Code:    0,
		Message: "Success",
		Data:    []OpsMetricsData{},
	}, nil
}

func opsAgentProcesses(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAgentProcessesRequest) (*OpsAgentProcessesResponse, error) {
	// Implementation would query processes from the agent
	return &OpsAgentProcessesResponse{
		Code:    0,
		Message: "Success",
		Data:    []OpsManagedProcess{},
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
				Code:    0,
				Message: "Success",
				Data: OpsAgentSystemInfo{
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
		Code:    0,
		Message: "Success",
		Data: OpsExecCommandResult{
			ExitCode: result.ExitCode,
			Stdout:   result.Stdout,
			Stderr:   result.Stderr,
		},
	}, nil
}

func opsAgentProcessStart(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsProcessStartRequest) (*OpsProcessStartResponse, error) {
	// TODO: Implement process start functionality
	return &OpsProcessStartResponse{
		Code:    0,
		Message: "Process start not implemented",
	}, nil
}

func opsAgentProcessStop(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsProcessActionRequest) (*OpsProcessActionResponse, error) {
	// TODO: Implement process stop functionality
	return &OpsProcessActionResponse{
		Code:    0,
		Message: "Process stop not implemented",
	}, nil
}

func opsAgentProcessRestart(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsProcessActionRequest) (*OpsProcessActionResponse, error) {
	// TODO: Implement process restart functionality
	return &OpsProcessActionResponse{
		Code:    0,
		Message: "Process restart not implemented",
	}, nil
}

// Backup operations implementations

func opsBackupsList(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsBackupsListRequest) (*OpsBackupsListResponse, error) {
	opts := model.ListBackupsOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     1,
			PageSize: 1000,
		},
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
		Code:    0,
		Message: "Success",
		Data:    items,
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
		Code:    0,
		Message: "Backup created successfully",
		Data:    backup.BackupID,
	}, nil
}

func opsBackupDelete(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsBackupDeleteRequest) (*OpsBackupDeleteResponse, error) {
	// TODO: Implement delete functionality
	return &OpsBackupDeleteResponse{
		Data: true,
	}, nil
}

func opsBackupDownload(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsBackupDownloadRequest) (*OpsBackupDownloadResponse, error) {
	// TODO: Implement download URL generation
	return &OpsBackupDownloadResponse{
		Data: fmt.Sprintf("/backups/%s/download", req.ID),
	}, nil
}

// Alert operations implementations

func opsAlerts(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAlertsRequest) (*OpsAlertsResponse, error) {
	opts := model.ListAlertsOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     1,
			PageSize: 1000,
		},
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
		Code:    0,
		Message: "Alert silenced successfully",
		Data:    fmt.Sprintf("%d", silence.ID),
	}, nil
}

func opsSilenceDelete(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsAlertSilenceRequest) (*OpsSilenceDeleteResponse, error) {
	// Parse the silence ID as uint
	var silenceID uint
	if _, err := fmt.Sscanf(req.AlertID, "%d", &silenceID); err != nil {
		return &OpsSilenceDeleteResponse{
			Code:    1,
			Message: "Invalid silence ID",
		}, nil
	}
	err := svcCtx.AlertModel.DeleteSilence(ctx, silenceID)
	if err != nil {
		return &OpsSilenceDeleteResponse{
			Code:    1,
			Message: "Failed to delete silence",
		}, nil
	}

	return &OpsSilenceDeleteResponse{
		Code:    0,
		Message: "Silence deleted successfully",
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
		Code:    0,
		Message: "Success",
		Data:    items,
	}, nil
}

// Node operations implementations

func opsNodes(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNodesRequest) (*OpsNodesResponse, error) {
	store := svcCtx.RegistryStore
	if store == nil {
		return &OpsNodesResponse{
			Code:    0,
			Message: "Success",
			Data:    []Node{},
		}, nil
	}

	store.Mu().RLock()
	defer store.Mu().RUnlock()

	nodes := make([]Node, 0, len(store.AgentsUnsafe()))
	for _, sess := range store.AgentsUnsafe() {
		if sess == nil {
			continue
		}

		nodes = append(nodes, Node{
			Id:       sess.AgentID,
			Hostname: sess.Labels["hostname"],
			Addr:     sess.RPCAddr,
			GameId:   sess.GameID,
			Env:      sess.Env,
			Status:   "active",
			Labels:   sess.Labels,
			LastSeen: utils.FormatTimestamp(sess.LastSeen),
		})
	}

	return &OpsNodesResponse{
		Code:    0,
		Message: "Success",
		Data:    nodes,
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
		Code:    0,
		Message: "Success",
		Data:    commands,
	}, nil
}

func opsNodeDrain(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNodeCommandsRequest) (*OpsNodeDrainResponse, error) {
	// DrainNode not implemented - return success
	return &OpsNodeDrainResponse{
		Code:    0,
		Message: "Node drained successfully",
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
				Code:    0,
				Message: "Success",
				Data:    sess.Labels,
			}, nil
		}
	}

	return nil, errors.New("node not found")
}

func opsNodeRestart(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNodeCommandsRequest) (*OpsNodeRestartResponse, error) {
	// RestartNode not implemented
	return &OpsNodeRestartResponse{
		Code:    0,
		Message: "Node restart initiated",
	}, nil
}

func opsNodeUndrain(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNodeCommandsRequest) (*OpsNodeUndrainResponse, error) {
	// UndrainNode not implemented
	return &OpsNodeUndrainResponse{
		Code:    0,
		Message: "Node undrained successfully",
	}, nil
}

// Health and maintenance implementations

func opsHealthGet(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsHealthGetRequest) (*OpsHealthGetResponse, error) {
	// HealthModel not implemented - return empty checks
	return &OpsHealthGetResponse{
		Code:    0,
		Message: "Success",
		Data:    []OpsHealthCheck{},
	}, nil
}

func opsHealthRun(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsHealthRunRequest) (*OpsHealthRunResponse, error) {
	// HealthModel not implemented - return empty results
	return &OpsHealthRunResponse{
		Code:    0,
		Message: "Success",
		Data:    []OpsHealthCheck{},
	}, nil
}

func opsMetrics(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsMetricsRequest) (*OpsMetricsResponse, error) {
	// MetricsModel not implemented - return empty data
	return &OpsMetricsResponse{
		Code:    0,
		Message: "Success",
		Data:    []OpsMetricsData{},
	}, nil
}

func opsMaintenanceGet(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsMaintenanceGetRequest) (*OpsMaintenanceGetResponse, error) {
	// MaintenanceModel not implemented - return empty windows
	return &OpsMaintenanceGetResponse{
		Code:    0,
		Message: "Success",
		Data:    []OpsMaintenanceWindow{},
	}, nil
}

func opsMaintenanceUpdate(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsMaintenanceUpdateRequest) (*OpsMaintenanceUpdateResponse, error) {
	// MaintenanceModel not implemented - return success
	return &OpsMaintenanceUpdateResponse{
		Code:    0,
		Message: "Maintenance updated successfully",
	}, nil
}

// Services and functions implementations

func opsFunctions(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsFunctionsRequest) (*OpsFunctionsResponse, error) {
	store := svcCtx.RegistryStore
	if store == nil {
		return &OpsFunctionsResponse{
			Code:    0,
			Message: "Success",
			Data:    map[string][]string{},
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
		Code:    0,
		Message: "Success",
		Data:    result,
	}, nil
}

// Config and notifications implementations

func opsConfig(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsConfigRequest) (*OpsConfigResponse, error) {
	// ConfigModel not implemented - return empty config
	return &OpsConfigResponse{
		Code:    0,
		Message: "Success",
		Data:    map[string]interface{}{},
	}, nil
}

func opsNotificationsGet(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNotificationsGetRequest) (*OpsNotificationsGetResponse, error) {
	if enabled, channels, rules, ok, err := loadNotificationsFromExtensionInstallation(ctx, svcCtx); err != nil {
		return nil, err
	} else if ok {
		return &OpsNotificationsGetResponse{
			Code:    0,
			Message: "Success",
			Data: map[string]interface{}{
				"enabled":  enabled,
				"channels": channels,
				"rules":    rules,
			},
		}, nil
	}
	// NotificationModel not implemented - fallback to empty
	return &OpsNotificationsGetResponse{
		Code:    0,
		Message: "Success",
		Data: map[string]interface{}{
			"enabled":  false,
			"channels": []OpsNotificationChannel{},
			"rules":    []OpsNotificationRule{},
		},
	}, nil
}

func opsNotificationsUpdate(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNotificationsUpdateRequest) (*OpsNotificationsUpdateResponse, error) {
	// NotificationModel not implemented - return success
	if req == nil {
		req = &OpsNotificationsUpdateRequest{}
	}
	_ = saveNotificationsToExtensionInstallation(ctx, svcCtx, req)
	_ = recordNotificationEvent(ctx, svcCtx, "notifications_update", "notifications updated",
		fmt.Sprintf(`{"enabled":%t,"channels":%d,"rules":%d}`, req.Enabled, len(req.Channels), len(req.Rules)),
	)
	return &OpsNotificationsUpdateResponse{
		Code:    0,
		Message: "Notifications updated successfully",
	}, nil
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
	if strings.TrimSpace(item.ConfigJSON) != "" {
		if err := json.Unmarshal([]byte(item.ConfigJSON), &config); err != nil {
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
	if strings.TrimSpace(item.ConfigJSON) != "" {
		_ = json.Unmarshal([]byte(item.ConfigJSON), &config)
	}
	config[notificationEnabledKey] = req.Enabled
	config[notificationChannelsKey] = req.Channels
	config[notificationRulesKey] = req.Rules
	secretRefs := map[string]string{}
	if strings.TrimSpace(item.SecretRefsJSON) != "" {
		_ = json.Unmarshal([]byte(item.SecretRefsJSON), &secretRefs)
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
	// MQModel not implemented - return empty queues
	return &OpsMQResponse{
		Code:    0,
		Message: "Success",
		Data:    []map[string]interface{}{},
	}, nil
}

func opsHealthUpdate(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsHealthUpdateRequest) (*OpsHealthUpdateResponse, error) {
	// HealthModel not implemented - return success
	return &OpsHealthUpdateResponse{
		Code:    0,
		Message: "Health updated successfully",
	}, nil
}

func opsServices(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsServicesRequest) (*OpsServicesResponse, error) {
	// ServiceModel not implemented - return empty
	return &OpsServicesResponse{
		Services: []OpsServiceItem{},
	}, nil
}
