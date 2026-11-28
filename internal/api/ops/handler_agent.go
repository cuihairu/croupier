package ops

import (
	"net/http"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

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
	if req.AgentID == "" {
		req.AgentID = c.Param("agentId")
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
	if req.AgentID == "" {
		req.AgentID = c.Param("agentId")
	}

	resp, err := h.service.OpsAgentProcesses(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsAgentSupervisor(c *gin.Context) {
	var req OpsAgentSupervisorRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	// path-parameter fallback: BindQueryCompat only binds query, not the uri tag.
	if req.AgentID == "" {
		req.AgentID = c.Param("agentId")
	}

	resp, err := h.service.OpsAgentSupervisor(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsAgentSupervisorEvents(c *gin.Context) {
	var req OpsAgentSupervisorEventsRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	// path-parameter fallback: BindQueryCompat only binds query, not the uri tag (#59, same class).
	if req.AgentID == "" {
		req.AgentID = c.Param("agentId")
	}

	resp, err := h.service.OpsAgentSupervisorEvents(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// OpsAgentSupervisorLog proxies the download of the agent's local supervisor
// event log. File downloads are exempt from the JSON envelope per the CLAUDE.md
// API contract: raw byte stream + Content-Disposition.
func (h *Handler) OpsAgentSupervisorLog(c *gin.Context) {
	var req OpsAgentSupervisorLogRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	if req.AgentID == "" {
		req.AgentID = c.Param("agentId")
	}

	resp, err := h.service.OpsAgentSupervisorLog(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}

	fileName := resp.GetFileName()
	if fileName == "" {
		fileName = "supervisor-events.log"
	}
	c.Header("Content-Disposition", `attachment; filename="`+fileName+`"`)
	c.Header("X-Truncated", boolHeader(resp.GetTruncated()))
	c.Data(http.StatusOK, "text/plain; charset=utf-8", resp.GetContent())
}

func boolHeader(v bool) string {
	if v {
		return "true"
	}
	return "false"
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
	agentId, name := processActionParams(c)
	resp, err := h.service.OpsAgentProcessStart(c.Request.Context(),
		&OpsProcessStartRequest{AgentID: agentId, Name: name})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsAgentProcessStop(c *gin.Context) {
	agentId, name := processActionParams(c)
	resp, err := h.service.OpsAgentProcessStop(c.Request.Context(),
		&OpsProcessActionRequest{AgentID: agentId, Name: name})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsAgentProcessRestart(c *gin.Context) {
	agentId, name := processActionParams(c)
	resp, err := h.service.OpsAgentProcessRestart(c.Request.Context(),
		&OpsProcessActionRequest{AgentID: agentId, Name: name})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) OpsAgentProcessUpdate(c *gin.Context) {
	var req OpsProcessUpdateRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	// stagedPath/expectedSha256 come from the body, but agentId/name exist only
	// in the path — JSON binding ignores the uri tag, and ShouldBindJSON goes
	// straight to EOF on an empty body (#59, the same binding gap).
	req.AgentID, req.Name = processActionParams(c)

	resp, err := h.service.OpsAgentProcessUpdate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// opsAgentProcessSchedulePause suspends / resumes the cron schedule trigger of
// a supervised process (§10.2 suspend and resume, the suspended object is the
// schedule). The two routes reuse one handler; suspend is fixed by the route and
// never comes from the request body (the semantics live in the path, so the path
// and body semantics cannot diverge).
func (h *Handler) opsAgentProcessSchedulePause(c *gin.Context, suspend bool) {
	agentId, name := processActionParams(c)
	req := OpsProcessSchedulePauseRequest{AgentID: agentId, Name: name, Suspend: suspend}

	resp, err := h.service.OpsAgentProcessSchedulePause(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// OpsAgentProcessSchedulePause is the public suspend entry (shared by the route and the alias).
func (h *Handler) OpsAgentProcessSchedulePause(c *gin.Context) {
	h.opsAgentProcessSchedulePause(c, true)
}

// Backup operations handlers

func (h *Handler) AgentsList(c *gin.Context) {
	h.OpsAgentsList(c)
}

func (h *Handler) AgentMeta(c *gin.Context) {
	h.OpsAgentMeta(c)
}

func (h *Handler) AgentMetrics(c *gin.Context) {
	h.OpsAgentMetrics(c)
}

func (h *Handler) AgentMetricsHistory(c *gin.Context) {
	var req AgentMetricsHistoryRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.AgentMetricsHistory(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AgentLogSearch(c *gin.Context) {
	var req AgentLogSearchRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.AgentLogSearch(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
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

func (h *Handler) AgentSupervisor(c *gin.Context) {
	h.OpsAgentSupervisor(c)
}

func (h *Handler) AgentSupervisorEvents(c *gin.Context) {
	h.OpsAgentSupervisorEvents(c)
}

func (h *Handler) AgentSupervisorLog(c *gin.Context) {
	h.OpsAgentSupervisorLog(c)
}

func (h *Handler) AgentProcessStop(c *gin.Context) {
	h.OpsAgentProcessStop(c)
}

func (h *Handler) AgentProcessRestart(c *gin.Context) {
	h.OpsAgentProcessRestart(c)
}

func (h *Handler) AgentProcessUpdate(c *gin.Context) {
	h.OpsAgentProcessUpdate(c)
}

func (h *Handler) AgentProcessSchedulePause(c *gin.Context) {
	h.opsAgentProcessSchedulePause(c, true)
}

func (h *Handler) AgentProcessScheduleResume(c *gin.Context) {
	h.opsAgentProcessSchedulePause(c, false)
}

func (h *Handler) AgentExecCommand(c *gin.Context) {
	h.OpsAgentExecCommand(c)
}
