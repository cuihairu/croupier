package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// Agent operations methods

func (s *Service) OpsAgentsList(ctx context.Context, req *OpsAgentsListRequest) (*OpsAgentsListResponse, error) {
	return opsAgentsList(ctx, s.svcCtx, req)
}

func (s *Service) OpsAgentMeta(ctx context.Context, req *OpsAgentMetaRequest) (*OpsAgentMetaResponse, error) {
	return opsAgentMeta(ctx, s.svcCtx, req)
}

func (s *Service) OpsAgentMetrics(ctx context.Context, req *OpsAgentMetricsRequest) (*OpsAgentMetricsResponse, error) {
	return opsAgentMetrics(ctx, s.svcCtx, req)
}

func (s *Service) OpsAgentProcesses(ctx context.Context, req *OpsAgentProcessesRequest) (*OpsAgentProcessesResponse, error) {
	return opsAgentProcesses(ctx, s.svcCtx, req)
}

func (s *Service) OpsAgentSystemInfo(ctx context.Context, req *OpsAgentSystemInfoRequest) (*OpsAgentSystemInfoResponse, error) {
	return opsAgentSystemInfo(ctx, s.svcCtx, req)
}

func (s *Service) OpsAgentExecCommand(ctx context.Context, req *OpsExecCommandRequest) (*OpsExecCommandResponse, error) {
	return opsAgentExecCommand(ctx, s.svcCtx, req)
}

func (s *Service) OpsAgentProcessStart(ctx context.Context, req *OpsProcessStartRequest) (*OpsProcessStartResponse, error) {
	return opsAgentProcessStart(ctx, s.svcCtx, req)
}

func (s *Service) OpsAgentProcessStop(ctx context.Context, req *OpsProcessActionRequest) (*OpsProcessActionResponse, error) {
	return opsAgentProcessStop(ctx, s.svcCtx, req)
}

func (s *Service) OpsAgentProcessRestart(ctx context.Context, req *OpsProcessActionRequest) (*OpsProcessActionResponse, error) {
	return opsAgentProcessRestart(ctx, s.svcCtx, req)
}

// Backup operations methods

func (s *Service) OpsBackupsList(ctx context.Context, req *OpsBackupsListRequest) (*OpsBackupsListResponse, error) {
	return opsBackupsList(ctx, s.svcCtx, req)
}

func (s *Service) OpsBackupCreate(ctx context.Context, req *OpsBackupCreateRequest) (*OpsBackupCreateResponse, error) {
	return opsBackupCreate(ctx, s.svcCtx, req)
}

func (s *Service) OpsBackupDelete(ctx context.Context, req *OpsBackupDeleteRequest) (*OpsBackupDeleteResponse, error) {
	return opsBackupDelete(ctx, s.svcCtx, req)
}

func (s *Service) OpsBackupDownload(ctx context.Context, req *OpsBackupDownloadRequest) (*OpsBackupDownloadResponse, error) {
	return opsBackupDownload(ctx, s.svcCtx, req)
}

// Alert operations methods

func (s *Service) OpsAlerts(ctx context.Context, req *OpsAlertsRequest) (*OpsAlertsResponse, error) {
	return opsAlerts(ctx, s.svcCtx, req)
}

func (s *Service) OpsAlertSilence(ctx context.Context, req *OpsAlertSilenceRequest) (*OpsAlertSilenceResponse, error) {
	return opsAlertSilence(ctx, s.svcCtx, req)
}

func (s *Service) OpsSilenceDelete(ctx context.Context, req *OpsAlertSilenceRequest) (*OpsSilenceDeleteResponse, error) {
	return opsSilenceDelete(ctx, s.svcCtx, req)
}

func (s *Service) OpsSilences(ctx context.Context, req *OpsSilencesRequest) (*OpsSilencesResponse, error) {
	return opsSilences(ctx, s.svcCtx, req)
}

// Node operations methods

func (s *Service) OpsNodes(ctx context.Context, req *OpsNodesRequest) (*OpsNodesResponse, error) {
	return opsNodes(ctx, s.svcCtx, req)
}

func (s *Service) OpsNodeCommands(ctx context.Context, req *OpsNodeCommandsRequest) (*OpsNodeCommandsResponse, error) {
	return opsNodeCommands(ctx, s.svcCtx, req)
}

func (s *Service) OpsNodeDrain(ctx context.Context, req *OpsNodeCommandsRequest) (*OpsNodeDrainResponse, error) {
	return opsNodeDrain(ctx, s.svcCtx, req)
}

func (s *Service) OpsNodeMeta(ctx context.Context, req *OpsNodeMetaRequest) (*OpsNodeMetaResponse, error) {
	return opsNodeMeta(ctx, s.svcCtx, req)
}

func (s *Service) OpsNodeRestart(ctx context.Context, req *OpsNodeCommandsRequest) (*OpsNodeRestartResponse, error) {
	return opsNodeRestart(ctx, s.svcCtx, req)
}

func (s *Service) OpsNodeUndrain(ctx context.Context, req *OpsNodeCommandsRequest) (*OpsNodeUndrainResponse, error) {
	return opsNodeUndrain(ctx, s.svcCtx, req)
}

// Health and maintenance methods

func (s *Service) OpsHealthGet(ctx context.Context, req *OpsHealthGetRequest) (*OpsHealthGetResponse, error) {
	return opsHealthGet(ctx, s.svcCtx, req)
}

func (s *Service) OpsHealthRun(ctx context.Context, req *OpsHealthRunRequest) (*OpsHealthRunResponse, error) {
	return opsHealthRun(ctx, s.svcCtx, req)
}

func (s *Service) OpsHealthUpdate(ctx context.Context, req *OpsHealthUpdateRequest) (*OpsHealthUpdateResponse, error) {
	return opsHealthUpdate(ctx, s.svcCtx, req)
}

func (s *Service) OpsMaintenanceGet(ctx context.Context, req *OpsMaintenanceGetRequest) (*OpsMaintenanceGetResponse, error) {
	return opsMaintenanceGet(ctx, s.svcCtx, req)
}

func (s *Service) OpsMaintenanceUpdate(ctx context.Context, req *OpsMaintenanceUpdateRequest) (*OpsMaintenanceUpdateResponse, error) {
	return opsMaintenanceUpdate(ctx, s.svcCtx, req)
}

// Metrics and monitoring methods

func (s *Service) OpsMetrics(ctx context.Context, req *OpsMetricsRequest) (*OpsMetricsResponse, error) {
	return opsMetrics(ctx, s.svcCtx, req)
}

// Config and notifications methods

func (s *Service) OpsConfig(ctx context.Context, req *OpsConfigRequest) (*OpsConfigResponse, error) {
	return opsConfig(ctx, s.svcCtx, req)
}

func (s *Service) OpsNotificationsGet(ctx context.Context, req *OpsNotificationsGetRequest) (*OpsNotificationsGetResponse, error) {
	return opsNotificationsGet(ctx, s.svcCtx, req)
}

func (s *Service) OpsNotificationsUpdate(ctx context.Context, req *OpsNotificationsUpdateRequest) (*OpsNotificationsUpdateResponse, error) {
	return opsNotificationsUpdate(ctx, s.svcCtx, req)
}

// Services and functions methods

func (s *Service) OpsServices(ctx context.Context, req *OpsServicesRequest) (*OpsServicesResponse, error) {
	return opsServices(ctx, s.svcCtx, req)
}

func (s *Service) OpsFunctions(ctx context.Context, req *OpsFunctionsRequest) (*OpsFunctionsResponse, error) {
	return opsFunctions(ctx, s.svcCtx, req)
}

// MQ methods

func (s *Service) OpsMQ(ctx context.Context, req *OpsMQRequest) (*OpsMQResponse, error) {
	return opsMQ(ctx, s.svcCtx, req)
}
