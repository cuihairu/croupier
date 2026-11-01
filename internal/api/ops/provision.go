package ops

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/ops/provision"
	"github.com/gin-gonic/gin"
)

// ============================================================================
// 云资源供给引擎（OPS-001）HTTP 面。
//
// 契约：provision.enabled=false 时引擎不装配（svcCtx.Provision=nil），各端点
// 如实报 503 provision_not_configured，绝不假装已开启。bootstrap token 不收
// 客户端入参——来自 server 配置的 BootstrapTokenEnv 环境变量引用（提交时
// 读取，token 可轮换）；hub 地址来自配置。两者缺失均属 server 侧装配
// 不完整，报 503（非客户端输入错误）。
// ============================================================================

// OpsProvisionSpec 实例规格（契约键 lowerCamelCase）。
type OpsProvisionSpec struct {
	InstanceType string `json:"instanceType"`
	Image        string `json:"image"`
	DiskSizeGB   int    `json:"diskSizeGB"`
}

// OpsProvisionSubmitRequest POST /ops/provision 入参。
type OpsProvisionSubmitRequest struct {
	Provider       string            `json:"provider"`
	Region         string            `json:"region"`
	Count          int               `json:"count"`
	Spec           OpsProvisionSpec  `json:"spec"`
	GameID         string            `json:"gameId"`
	Env            string            `json:"env"`
	Policy         string            `json:"policy"`
	Labels         map[string]string `json:"labels"`
	CloudInitExtra string            `json:"cloudInitExtra"`
}

// OpsProvisionListRequest GET /ops/provision 查询（limit 可选，默认 50）。
type OpsProvisionListRequest struct {
	Limit int `form:"limit"`
}

// OpsProvisionRunResponse 单运行响应（run 直接作为业务 payload）。
type OpsProvisionRunResponse struct {
	Run *provision.Run `json:"run"`
}

// OpsProvisionRunsResponse 运行列表。
type OpsProvisionRunsResponse struct {
	Runs []*provision.Run `json:"runs"`
}

// provisionNotConfigured / provisionTokenNotConfigured / provisionHubNotConfigured
// 是 server 侧装配错误（503），与客户端 4xx 区分（契约错误语义）。
var (
	errProvisionNotConfigured    = &errorx.CodeError{Code: http.StatusServiceUnavailable, Message: "供给引擎未配置", StableCode: "provision_not_configured"}
	errProvisionTokenMissing     = &errorx.CodeError{Code: http.StatusServiceUnavailable, Message: "供给引擎缺少 bootstrap token（未设置环境变量）", StableCode: "provision_token_not_configured"}
	errProvisionHubNotConfigured = &errorx.CodeError{Code: http.StatusServiceUnavailable, Message: "供给引擎缺少 hub 地址（provision.hubAddr 未配置）", StableCode: "provision_hub_not_configured"}
)

// provisionOrchestrator 返回装配的引擎；未装配返回 503 哨兵错误。
func (s *Service) provisionOrchestrator() (*provision.Orchestrator, error) {
	if s.svcCtx == nil || s.svcCtx.Provision == nil {
		return nil, errProvisionNotConfigured
	}
	return s.svcCtx.Provision, nil
}

// provisionServerParams 从 server 配置组装请求所需的敏感/拓扑参数。
func (s *Service) provisionServerParams(req *provision.Request) error {
	cfg := s.svcCtx.Config.Provision
	req.HubAddr = strings.TrimSpace(cfg.HubAddr)
	if req.HubAddr == "" {
		return errProvisionHubNotConfigured
	}
	token := strings.TrimSpace(os.Getenv(cfg.EffectiveBootstrapTokenEnv()))
	if token == "" {
		return errProvisionTokenMissing
	}
	req.BootstrapToken = token
	return nil
}

// ============================================================================
// Service 方法（handler 经此调；编排在 ops/provision 包，本层只做 DTO 映射）
// ============================================================================

// OpsProvisionSubmit 提交扩容请求（不阻塞：direct 后台执行，ticket 建审批
// 记录即返回 pending）。请求校验失败返回 422（语义无效但形态合法）。
func (s *Service) OpsProvisionSubmit(ctx context.Context, req *OpsProvisionSubmitRequest, requestedBy string) (*provision.Run, error) {
	orch, err := s.provisionOrchestrator()
	if err != nil {
		return nil, err
	}
	p := &provision.Request{
		Provider:       provision.Provider(strings.TrimSpace(req.Provider)),
		Region:         strings.TrimSpace(req.Region),
		Count:          req.Count,
		GameID:         req.GameID,
		Env:            req.Env,
		Policy:         provision.PolicyMode(req.Policy),
		Labels:         req.Labels,
		CloudInitExtra: req.CloudInitExtra,
		RequestedBy:    requestedBy,
	}
	if req.Spec.InstanceType != "" {
		p.Spec = provision.Spec{
			InstanceType: req.Spec.InstanceType,
			Image:        req.Spec.Image,
			DiskSizeGB:   req.Spec.DiskSizeGB,
		}
	}
	if err := s.provisionServerParams(p); err != nil {
		return nil, err
	}
	if _, err := provision.Validate(p); err != nil {
		return nil, errorx.NewValidationErrorWithDetails(err.Error(), nil)
	}
	return orch.SubmitAsync(ctx, p)
}

// OpsProvisionResume 审批通过后续行（不阻塞：后台执行 apply 全闭环）。
// 未批准/非 pending 返回 409（资源状态冲突，非客户端输入错误）。
func (s *Service) OpsProvisionResume(ctx context.Context, runID string) (*provision.Run, error) {
	orch, err := s.provisionOrchestrator()
	if err != nil {
		return nil, err
	}
	run, err := orch.ResumeAsync(ctx, strings.TrimSpace(runID))
	if err != nil {
		if errors.Is(err, provision.ErrRunNotPending) || errors.Is(err, provision.ErrRunNotApproved) {
			return nil, errorx.NewConflictWithCode("provision_state_conflict", err.Error(), nil)
		}
		return nil, err
	}
	return run, nil
}

// OpsProvisionGet 查询单运行。
func (s *Service) OpsProvisionGet(ctx context.Context, runID string) (*provision.Run, error) {
	orch, err := s.provisionOrchestrator()
	if err != nil {
		return nil, err
	}
	return orch.Store.Get(ctx, strings.TrimSpace(runID))
}

// OpsProvisionList 列出运行（limit<=0 用默认 50）。
func (s *Service) OpsProvisionList(ctx context.Context, limit int) ([]*provision.Run, error) {
	orch, err := s.provisionOrchestrator()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	return orch.Store.List(ctx, limit)
}

// OpsProvisionDestroy 销毁运行资源（同步 terraform destroy 原语；无后续
// 等注册段，时长通常分钟级内，不走异步）。
func (s *Service) OpsProvisionDestroy(ctx context.Context, runID string) (*provision.Run, error) {
	orch, err := s.provisionOrchestrator()
	if err != nil {
		return nil, err
	}
	if err := orch.Destroy(ctx, strings.TrimSpace(runID)); err != nil {
		return nil, err
	}
	return orch.Store.Get(ctx, strings.TrimSpace(runID))
}

// ============================================================================
// Handler
// ============================================================================

// ProvisionSubmit POST /ops/provision。
func (h *Handler) ProvisionSubmit(c *gin.Context) {
	var req OpsProvisionSubmitRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	run, err := h.service.OpsProvisionSubmit(c.Request.Context(), &req, c.GetString("username"))
	if err != nil {
		response.Error(c, err)
		return
	}
	c.JSON(http.StatusAccepted, OpsProvisionRunResponse{Run: run})
}

// ProvisionResume POST /ops/provision/:runId/resume。
func (h *Handler) ProvisionResume(c *gin.Context) {
	run, err := h.service.OpsProvisionResume(c.Request.Context(), c.Param("runId"))
	if err != nil {
		response.Error(c, err)
		return
	}
	c.JSON(http.StatusAccepted, OpsProvisionRunResponse{Run: run})
}

// ProvisionGet GET /ops/provision/:runId。
func (h *Handler) ProvisionGet(c *gin.Context) {
	run, err := h.service.OpsProvisionGet(c.Request.Context(), c.Param("runId"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, OpsProvisionRunResponse{Run: run})
}

// ProvisionList GET /ops/provision。
func (h *Handler) ProvisionList(c *gin.Context) {
	var req OpsProvisionListRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	runs, err := h.service.OpsProvisionList(c.Request.Context(), req.Limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, OpsProvisionRunsResponse{Runs: runs})
}

// ProvisionDestroy DELETE /ops/provision/:runId。
func (h *Handler) ProvisionDestroy(c *gin.Context) {
	run, err := h.service.OpsProvisionDestroy(c.Request.Context(), c.Param("runId"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, OpsProvisionRunResponse{Run: run})
}
