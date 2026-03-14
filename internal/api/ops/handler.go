package ops

import (
	"github.com/cuihairu/croupier/internal/common/requestbind"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

func bindOpsRequest(c *gin.Context, req interface{}) error {
	if c.Request.Method == "GET" {
		return requestbind.BindQueryCompat(c, req)
	}
	return c.ShouldBindJSON(req)
}

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Agent operations handlers

func (h *Handler) OpsAgentsList(c *gin.Context) {
	var req OpsAgentsListRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsAgentsList(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsAgentMeta(c *gin.Context) {
	var req OpsAgentMetaRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsAgentMeta(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsAgentMetrics(c *gin.Context) {
	var req OpsAgentMetricsRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsAgentMetrics(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsAgentProcesses(c *gin.Context) {
	var req OpsAgentProcessesRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsAgentProcesses(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsAgentSystemInfo(c *gin.Context) {
	var req OpsAgentSystemInfoRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsAgentSystemInfo(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsAgentExecCommand(c *gin.Context) {
	var req OpsExecCommandRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsAgentExecCommand(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsAgentProcessStart(c *gin.Context) {
	var req OpsProcessStartRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsAgentProcessStart(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsAgentProcessStop(c *gin.Context) {
	var req OpsProcessActionRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsAgentProcessStop(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsAgentProcessRestart(c *gin.Context) {
	var req OpsProcessActionRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsAgentProcessRestart(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Backup operations handlers

func (h *Handler) OpsBackupsList(c *gin.Context) {
	var req OpsBackupsListRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsBackupsList(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsBackupCreate(c *gin.Context) {
	var req OpsBackupCreateRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsBackupCreate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsBackupDelete(c *gin.Context) {
	var req OpsBackupDeleteRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsBackupDelete(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsBackupDownload(c *gin.Context) {
	var req OpsBackupDownloadRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsBackupDownload(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Alert operations handlers

func (h *Handler) OpsAlerts(c *gin.Context) {
	var req OpsAlertsRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsAlerts(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsAlertSilence(c *gin.Context) {
	var req OpsAlertSilenceRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsAlertSilence(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsSilenceDelete(c *gin.Context) {
	var req OpsAlertSilenceRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsSilenceDelete(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsSilences(c *gin.Context) {
	var req OpsSilencesRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsSilences(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Node operations handlers

func (h *Handler) OpsNodes(c *gin.Context) {
	var req OpsNodesRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsNodes(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsNodeCommands(c *gin.Context) {
	var req OpsNodeCommandsRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsNodeCommands(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsNodeDrain(c *gin.Context) {
	var req OpsNodeCommandsRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsNodeDrain(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsNodeMeta(c *gin.Context) {
	var req OpsNodeMetaRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsNodeMeta(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsNodeRestart(c *gin.Context) {
	var req OpsNodeCommandsRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsNodeRestart(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsNodeUndrain(c *gin.Context) {
	var req OpsNodeCommandsRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsNodeUndrain(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Health and maintenance handlers

func (h *Handler) OpsHealthGet(c *gin.Context) {
	var req OpsHealthGetRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsHealthGet(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsHealthRun(c *gin.Context) {
	var req OpsHealthRunRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsHealthRun(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsHealthUpdate(c *gin.Context) {
	var req OpsHealthUpdateRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsHealthUpdate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsMaintenanceGet(c *gin.Context) {
	var req OpsMaintenanceGetRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsMaintenanceGet(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsMaintenanceUpdate(c *gin.Context) {
	var req OpsMaintenanceUpdateRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsMaintenanceUpdate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Metrics and monitoring handlers

func (h *Handler) OpsMetrics(c *gin.Context) {
	var req OpsMetricsRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsMetrics(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Config and notifications handlers

func (h *Handler) OpsConfig(c *gin.Context) {
	var req OpsConfigRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsConfig(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsNotificationsGet(c *gin.Context) {
	var req OpsNotificationsGetRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsNotificationsGet(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsNotificationsUpdate(c *gin.Context) {
	var req OpsNotificationsUpdateRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsNotificationsUpdate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Services and functions handlers

func (h *Handler) OpsServices(c *gin.Context) {
	var req OpsServicesRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsServices(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsFunctions(c *gin.Context) {
	var req OpsFunctionsRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsFunctions(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// MQ handler

func (h *Handler) OpsMQ(c *gin.Context) {
	var req OpsMQRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.OpsMQ(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Alias methods for route compatibility

func (h *Handler) AgentsList(c *gin.Context) {
	h.OpsAgentsList(c)
}

func (h *Handler) AgentMeta(c *gin.Context) {
	h.OpsAgentMeta(c)
}

func (h *Handler) AgentMetrics(c *gin.Context) {
	h.OpsAgentMetrics(c)
}

func (h *Handler) AgentProcesses(c *gin.Context) {
	h.OpsAgentProcesses(c)
}

func (h *Handler) AgentSystemInfo(c *gin.Context) {
	h.OpsAgentSystemInfo(c)
}

func (h *Handler) AgentProcessStart(c *gin.Context) {
	h.OpsAgentProcessStart(c)
}

func (h *Handler) AgentProcessStop(c *gin.Context) {
	h.OpsAgentProcessStop(c)
}

func (h *Handler) AgentProcessRestart(c *gin.Context) {
	h.OpsAgentProcessRestart(c)
}

func (h *Handler) AgentExecCommand(c *gin.Context) {
	h.OpsAgentExecCommand(c)
}

func (h *Handler) BackupsList(c *gin.Context) {
	h.OpsBackupsList(c)
}

func (h *Handler) BackupCreate(c *gin.Context) {
	h.OpsBackupCreate(c)
}

func (h *Handler) BackupDelete(c *gin.Context) {
	h.OpsBackupDelete(c)
}

func (h *Handler) BackupDownload(c *gin.Context) {
	h.OpsBackupDownload(c)
}

func (h *Handler) Alerts(c *gin.Context) {
	h.OpsAlerts(c)
}

func (h *Handler) AlertSilence(c *gin.Context) {
	h.OpsAlertSilence(c)
}

func (h *Handler) SilenceDelete(c *gin.Context) {
	h.OpsSilenceDelete(c)
}

func (h *Handler) Silences(c *gin.Context) {
	h.OpsSilences(c)
}

func (h *Handler) Nodes(c *gin.Context) {
	h.OpsNodes(c)
}

func (h *Handler) NodeCommands(c *gin.Context) {
	h.OpsNodeCommands(c)
}

func (h *Handler) NodeDrain(c *gin.Context) {
	h.OpsNodeDrain(c)
}

func (h *Handler) NodeMeta(c *gin.Context) {
	h.OpsNodeMeta(c)
}

func (h *Handler) NodeRestart(c *gin.Context) {
	h.OpsNodeRestart(c)
}

func (h *Handler) NodeUndrain(c *gin.Context) {
	h.OpsNodeUndrain(c)
}

func (h *Handler) HealthGet(c *gin.Context) {
	h.OpsHealthGet(c)
}

func (h *Handler) HealthRun(c *gin.Context) {
	h.OpsHealthRun(c)
}

func (h *Handler) HealthUpdate(c *gin.Context) {
	h.OpsHealthUpdate(c)
}

func (h *Handler) MaintenanceGet(c *gin.Context) {
	h.OpsMaintenanceGet(c)
}

func (h *Handler) MaintenanceUpdate(c *gin.Context) {
	h.OpsMaintenanceUpdate(c)
}

func (h *Handler) Metrics(c *gin.Context) {
	h.OpsMetrics(c)
}

func (h *Handler) MQ(c *gin.Context) {
	h.OpsMQ(c)
}

func (h *Handler) NotificationsGet(c *gin.Context) {
	h.OpsNotificationsGet(c)
}

func (h *Handler) NotificationsUpdate(c *gin.Context) {
	h.OpsNotificationsUpdate(c)
}

func (h *Handler) Services(c *gin.Context) {
	h.OpsServices(c)
}

func (h *Handler) Functions(c *gin.Context) {
	h.OpsFunctions(c)
}

func (h *Handler) Config(c *gin.Context) {
	h.OpsConfig(c)
}
