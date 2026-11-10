package ops

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/powerops"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
)

// ============================================================================
// 远程电源管理（OPS 电源管理）HTTP 面。
//
// 契约：powerops.enabled=false 时编排器不装配（svcCtx.Powerops=nil），各端点
// 如实报 503 powerops_not_configured。提交链：POST /ops/power 创建 pending
// 请求+审批记录（self-approval 由审批服务拒绝，force_off 批准时走 OTP
// step-up）→ 审批通过后 POST /ops/power/:id/resume 执行 → 每台机器结果回填
// power_request_targets。拒绝走 POST /ops/power/:id/reject；查询 GET
// /ops/power/:id（pending 请求惰性同步审批拒绝状态）。
//
// 机器清单挂现有 nodes.meta（power 子对象：provider/bmcAddr/bmcPort/mac/
// cloudRef/agentId），PUT /ops/nodes/:nodeId/power-config 写入，不建新表。
// ============================================================================

// OpsPowerSubmitRequest POST /ops/power 入参（契约键 lowerCamelCase）。
type OpsPowerSubmitRequest struct {
	// NodeIDs 目标机器（nodes.node_id，须已在 power-config 登记电源信息）。
	NodeIDs []string `json:"nodeIds"`
	// Action power_on / power_off / force_off / reboot / status。
	Action string `json:"action"`
	// Reason 请求原因（进审批元数据，审计追溯）。
	Reason string `json:"reason"`
	// GameID/Env 可选审计上下文（电源操作本身不游戏分域）。
	GameID string `json:"gameId"`
	Env    string `json:"env"`
}

// OpsPowerListRequest GET /ops/power 查询（limit 可选，默认 50）。
type OpsPowerListRequest struct {
	Limit int `form:"limit"`
}

// OpsPowerRequestResponse 单请求响应（请求行直接作为业务 payload）。
type OpsPowerRequestResponse struct {
	Request *model.PowerRequest `json:"request"`
}

// OpsPowerRequestsResponse 请求列表。
type OpsPowerRequestsResponse struct {
	Requests []*model.PowerRequest `json:"requests"`
}

// OpsPowerTargetsResponse 每台机器执行结果。
type OpsPowerTargetsResponse struct {
	Targets []*model.PowerRequestTarget `json:"targets"`
}

// OpsPowerConfigRequest PUT /ops/nodes/:nodeId/power-config 入参：机器电源
// 信息（provider 必填；其余按 provider 语义选填——ipmi 要 bmcAddr，wol 要
// mac，cloud 要 cloudRef，agent 要 agentId）。
type OpsPowerConfigRequest struct {
	Provider string `json:"provider"`
	BmcAddr  string `json:"bmcAddr"`
	BmcPort  int    `json:"bmcPort"`
	MAC      string `json:"mac"`
	CloudRef string `json:"cloudRef"`
	AgentID  string `json:"agentId"`
}

// errPoweropsNotConfigured 是 server 侧装配错误（503），与客户端 4xx 区分。
var errPoweropsNotConfigured = &errorx.CodeError{Code: http.StatusServiceUnavailable, Message: "电源管理未配置", StableCode: "powerops_not_configured"}

// powerOrchestrator 返回装配的编排器；未装配返回 503 哨兵错误。
func (s *Service) powerOrchestrator() (*powerops.Orchestrator, error) {
	if s.svcCtx == nil || s.svcCtx.Powerops == nil {
		return nil, errPoweropsNotConfigured
	}
	return s.svcCtx.Powerops, nil
}

// powerResolver 以 svcCtx.DB 直查 nodes.meta.power（wiring 侧同源解析）。
func (s *Service) powerResolver() powerops.MachineResolver {
	return svc.NodePowerResolver{DB: s.svcCtx.DB}
}

// mapPowerError 把编排器错误映射到 API 契约状态码。
func mapPowerError(err error) error {
	switch {
	case errors.Is(err, powerops.ErrInvalidSubmit):
		return errorx.NewValidationErrorWithDetails(err.Error(), nil)
	case errors.Is(err, powerops.ErrRequestNotFound):
		return errorx.NewNotFound(err.Error())
	case errors.Is(err, powerops.ErrRequestNotPending),
		errors.Is(err, powerops.ErrRequestNotApproved),
		errors.Is(err, powerops.ErrApprovalLinkMissing):
		return errorx.NewConflictWithCode("power_state_conflict", err.Error(), nil)
	}
	return err
}

// ============================================================================
// Service 方法（handler 经此调；本层只做 DTO 映射与审批惰性同步）
// ============================================================================

// OpsPowerSubmit 提交电源请求（不阻塞：持久化 pending + 创建审批记录即返回）。
func (s *Service) OpsPowerSubmit(ctx context.Context, req *OpsPowerSubmitRequest, requestedBy string) (*model.PowerRequest, error) {
	orch, err := s.powerOrchestrator()
	if err != nil {
		return nil, err
	}
	out, err := orch.Submit(ctx, s.powerResolver(), powerops.SubmitInput{
		NodeIDs:     req.NodeIDs,
		Action:      powerops.Action(strings.TrimSpace(req.Action)),
		Reason:      req.Reason,
		RequestedBy: requestedBy,
		GameID:      req.GameID,
		Env:         req.Env,
	})
	if err != nil {
		return nil, mapPowerError(err)
	}
	recordOpsAudit(ctx, s.svcCtx, audit.EventPowerRequest, out.RequestID, "pending", map[string]interface{}{
		"action":     out.Action,
		"targets":    out.Total,
		"approvalId": out.ApprovalID,
	})
	return out, nil
}

// OpsPowerResume 审批通过后续行（不阻塞：后台执行 + 验证闭环）。
// 未批准/非 pending 返回 409（资源状态冲突，非客户端输入错误）。
func (s *Service) OpsPowerResume(ctx context.Context, requestID string) (*model.PowerRequest, error) {
	orch, err := s.powerOrchestrator()
	if err != nil {
		return nil, err
	}
	req, err := orch.ResumeAsync(ctx, strings.TrimSpace(requestID))
	if err != nil {
		return nil, mapPowerError(err)
	}
	return req, nil
}

// OpsPowerReject 拒绝 pending 请求（两宽规则的人为出口：审批单拒绝后请求
// 侧同步落 rejected，不再可执行）。
func (s *Service) OpsPowerReject(ctx context.Context, requestID string) (*model.PowerRequest, error) {
	orch, err := s.powerOrchestrator()
	if err != nil {
		return nil, err
	}
	req, err := orch.Reject(ctx, strings.TrimSpace(requestID))
	if err != nil {
		return nil, mapPowerError(err)
	}
	recordOpsAudit(ctx, s.svcCtx, audit.EventPowerReject, req.RequestID, "rejected", map[string]interface{}{
		"action": req.Action,
	})
	return req, nil
}

// OpsPowerGet 查询单请求（含每台机器结果）。pending 请求惰性同步审批拒绝：
// 审批服务里已 rejected 的单据在此落 rejected（无轮询依赖，读时收敛）。
func (s *Service) OpsPowerGet(ctx context.Context, requestID string) (*model.PowerRequest, []*model.PowerRequestTarget, error) {
	orch, err := s.powerOrchestrator()
	if err != nil {
		return nil, nil, err
	}
	requestID = strings.TrimSpace(requestID)
	req, err := orch.Store.Get(requestID)
	if err != nil {
		return nil, nil, err
	}
	if req == nil {
		return nil, nil, errorx.NewNotFound("powerops: request not found: " + requestID)
	}
	s.syncRejectedFromApproval(ctx, orch, req)
	targets, err := orch.Store.Targets(requestID)
	if err != nil {
		return nil, nil, err
	}
	return req, targets, nil
}

// OpsPowerList 列出请求（limit<=0 用默认 50）。
func (s *Service) OpsPowerList(ctx context.Context, limit int) ([]*model.PowerRequest, error) {
	orch, err := s.powerOrchestrator()
	if err != nil {
		return nil, err
	}
	return orch.Store.List(limit)
}

// syncRejectedFromApproval 把审批记录的 rejected 状态惰性同步进请求行
// （pending 且有审批链时才查 ApprovalsStore；任何查询错误不影响读路径，
// 仅跳过同步——读时收敛，无轮询依赖）。
func (s *Service) syncRejectedFromApproval(ctx context.Context, orch *powerops.Orchestrator, req *model.PowerRequest) {
	if req.State != powerops.StatePending || req.ApprovalID == "" || s.svcCtx.ApprovalsStore == nil {
		return
	}
	approval, err := s.svcCtx.ApprovalsStore.Get(req.ApprovalID)
	if err != nil || approval == nil || approval.State != "rejected" {
		return
	}
	if _, err := orch.Reject(ctx, req.RequestID); err == nil {
		if refreshed, err := orch.Store.Get(req.RequestID); err == nil && refreshed != nil {
			*req = *refreshed
		}
	}
}

// OpsPowerConfigPut 写入机器电源信息（nodes.meta.power 子对象；读-改-写
// 合并，不覆盖 meta 里其它键）。域未装配报 503，节点不存在报 404。
func (s *Service) OpsPowerConfigPut(ctx context.Context, nodeID string, req *OpsPowerConfigRequest) error {
	if _, err := s.powerOrchestrator(); err != nil {
		return err
	}
	if s.svcCtx.NodeModel == nil {
		return errorx.NewInternalError("node model unavailable")
	}
	nodeID = strings.TrimSpace(nodeID)
	node, err := s.svcCtx.NodeModel.FindByNodeID(ctx, nodeID)
	if err != nil {
		return errorx.NewNotFound("node not found: " + nodeID)
	}
	provider := strings.TrimSpace(req.Provider)
	if provider == "" {
		return errorx.NewValidationErrorWithDetails("provider is required", nil)
	}
	switch provider {
	case powerops.ProviderIPMI, powerops.ProviderWOL, powerops.ProviderCloud, powerops.ProviderAgent:
	default:
		return errorx.NewValidationErrorWithDetails("unknown provider: "+provider, nil)
	}
	if node.Meta == nil {
		node.Meta = datatypes.JSONMap{}
	}
	node.Meta["power"] = svc.MachineToMeta(powerops.Machine{
		NodeID:   nodeID,
		Provider: provider,
		BmcAddr:  strings.TrimSpace(req.BmcAddr),
		BmcPort:  req.BmcPort,
		MAC:      strings.TrimSpace(req.MAC),
		CloudRef: strings.TrimSpace(req.CloudRef),
		AgentID:  strings.TrimSpace(req.AgentID),
	})
	if err := s.svcCtx.NodeModel.UpdateMeta(ctx, nodeID, map[string]interface{}{"meta": node.Meta}); err != nil {
		return err
	}
	recordOpsAudit(ctx, s.svcCtx, audit.EventPowerConfigUpdate, nodeID, "success", map[string]interface{}{
		"provider": provider,
	})
	return nil
}

// ============================================================================
// Handler
// ============================================================================

// PowerSubmit POST /ops/power。
func (h *Handler) PowerSubmit(c *gin.Context) {
	var req OpsPowerSubmitRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	req2, err := h.service.OpsPowerSubmit(c.Request.Context(), &req, c.GetString("username"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Accepted(c, OpsPowerRequestResponse{Request: req2})
}

// PowerResume POST /ops/power/:requestId/resume。
func (h *Handler) PowerResume(c *gin.Context) {
	req, err := h.service.OpsPowerResume(c.Request.Context(), c.Param("requestId"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Accepted(c, OpsPowerRequestResponse{Request: req})
}

// PowerReject POST /ops/power/:requestId/reject。
func (h *Handler) PowerReject(c *gin.Context) {
	req, err := h.service.OpsPowerReject(c.Request.Context(), c.Param("requestId"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, OpsPowerRequestResponse{Request: req})
}

// PowerGet GET /ops/power/:requestId。
func (h *Handler) PowerGet(c *gin.Context) {
	req, targets, err := h.service.OpsPowerGet(c.Request.Context(), c.Param("requestId"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"request": req, "targets": targets})
}

// PowerList GET /ops/power。
func (h *Handler) PowerList(c *gin.Context) {
	var req OpsPowerListRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	requests, err := h.service.OpsPowerList(c.Request.Context(), req.Limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, OpsPowerRequestsResponse{Requests: requests})
}

// PowerConfigPut PUT /ops/nodes/:nodeId/power-config。
func (h *Handler) PowerConfigPut(c *gin.Context) {
	var req OpsPowerConfigRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	if err := h.service.OpsPowerConfigPut(c.Request.Context(), c.Param("nodeId"), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"status": "ok"})
}
