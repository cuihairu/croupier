package ops

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/ops/provision"
	"github.com/cuihairu/croupier/internal/tasks"
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
	// errProvisionDestroyProtected / errProvisionTasksInflight /
	// errProvisionAgentsNotDrained 是销毁守卫拒绝（OPS-002 缩容与排水
	// 保守版）：资源状态冲突 409，非客户端输入错误。
	errProvisionDestroyProtected = &errorx.CodeError{Code: http.StatusConflict, Message: "供给运行已开启数据盘护栏，禁止自动销毁", StableCode: "provision_destroy_protected"}
	errProvisionTasksInflight    = &errorx.CodeError{Code: http.StatusConflict, Message: "目标机仍有在途任务，须先排水", StableCode: "provision_tasks_inflight"}
	errProvisionAgentsNotDrained = &errorx.CodeError{Code: http.StatusConflict, Message: "目标机 agent 未排空，禁止销毁", StableCode: "provision_agents_not_drained"}
	errProvisionLoadUnverifiable = &errorx.CodeError{Code: http.StatusInternalServerError, Message: "无法校验目标机负载（数据源未就绪）", StableCode: "provision_load_unverifiable"}
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
	// 作用域受限用户不得向越权游戏/环境供给云资源（body 的 gameId/env 与
	// 已认证 scope 比对，fail-closed）。
	if err := enforceRequestScope(ctx, req.GameID, req.Env); err != nil {
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
	// 网络级审批双模式（OPS-003）：workflow 强制 ticket（客户端不得以
	// direct 绕过审批）；auto 时客户端可显式选择 ticket 或默认 direct。
	if s.svcCtx.Config.Provision.EffectiveApproval() == config.ProvisionApprovalWorkflow {
		p.Policy = provision.PolicyTicket
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
//
// OPS-002 缩容与排水守卫（先行保守版）：销毁前置三闸——
//  1. 数据盘护栏：run.DestroyProtected=true 时拒绝（持久化标记禁止自动销毁）；
//  2. 无负载才允许销毁：运行名下目标机在途任务未归零时拒绝（先排水）；
//  3. 排空门：运行名下所有在线 agent 会话必须已 drain（否则新任务仍可路由
//     到目标机，与销毁竞态；离线会话无路由、不计）。
//
// 销毁走同一 Terraform 状态（Orchestrator.Destroy 复用 WorkDir/main.tf），
// 防漂移。成功落 audit provision.destroy。
func (s *Service) OpsProvisionDestroy(ctx context.Context, runID string) (*provision.Run, error) {
	orch, err := s.provisionOrchestrator()
	if err != nil {
		return nil, err
	}
	runID = strings.TrimSpace(runID)
	run, err := orch.Store.Get(ctx, runID)
	if err != nil {
		return nil, err
	}
	// 网络级审批双模式（OPS-003）：workflow 时销毁走申请单（创建→审批→
	// 执行），此端点返回 destroy_pending（不立即销毁）；auto 时直接执行。
	if s.svcCtx.Config.Provision.EffectiveApproval() == config.ProvisionApprovalWorkflow {
		return s.requestDestroy(ctx, orch, run)
	}
	if err := s.provisionDestroyGuards(ctx, run); err != nil {
		return nil, err
	}
	if err := orch.Destroy(ctx, runID); err != nil {
		return nil, err
	}
	recordOpsAudit(ctx, s.svcCtx, audit.EventProvisionDestroy, runID, "success", map[string]interface{}{
		"provider":  string(run.Request.Provider),
		"agents":    run.AgentIDs,
		"protected": run.DestroyProtected,
	})
	return orch.Store.Get(ctx, runID)
}

// requestDestroy 是 workflow 模式销毁申请单入口：run 须为 adopted，创建
// 审批记录并推进 destroy_pending（销毁须经批准后执行）。
func (s *Service) requestDestroy(ctx context.Context, orch *provision.Orchestrator, run *provision.Run) (*provision.Run, error) {
	// 前置闸：护栏/排空/在途任务在申请时校验（批准后执行时不再重复）。
	if err := s.provisionDestroyGuards(ctx, run); err != nil {
		return nil, err
	}
	requested, err := orch.RequestDestroy(ctx, run.ID)
	if err != nil {
		if errors.Is(err, provision.ErrRunNotDestroyPending) {
			return nil, errorx.NewConflictWithCode("provision_state_conflict", err.Error(), nil)
		}
		return nil, err
	}
	recordOpsAudit(ctx, s.svcCtx, audit.EventProvisionDestroyRequest, run.ID, "success", map[string]interface{}{
		"provider": string(run.Request.Provider),
		"agents":   run.AgentIDs,
	})
	return requested, nil
}

// OpsProvisionDestroyResume 是 workflow 模式销毁审批通过后的执行（同步
// terraform destroy 原语；未批准/非 destroy_pending 返回 409）。
func (s *Service) OpsProvisionDestroyResume(ctx context.Context, runID string) (*provision.Run, error) {
	orch, err := s.provisionOrchestrator()
	if err != nil {
		return nil, err
	}
	run, err := orch.DestroyResume(ctx, strings.TrimSpace(runID))
	if err != nil {
		if errors.Is(err, provision.ErrRunNotDestroyPending) || errors.Is(err, provision.ErrRunNotApproved) {
			return nil, errorx.NewConflictWithCode("provision_state_conflict", err.Error(), nil)
		}
		return nil, err
	}
	recordOpsAudit(ctx, s.svcCtx, audit.EventProvisionDestroy, run.ID, "success", map[string]interface{}{
		"provider": string(run.Request.Provider),
		"agents":   run.AgentIDs,
	})
	return run, nil
}

// provisionDestroyGuards 是销毁前置三闸（见 OpsProvisionDestroy）。顺序即
// 运维处置顺序：护栏 → 排空（内存门，停新路由）→ 在途任务（等在途归零）。
// 在途任务计数经 game scope context 解析到 per-game DB（task_runs 在 game
// 库）；RegistryStore 为空时无在线会话可校验（排空门恒过，负载闸仍生效）。
func (s *Service) provisionDestroyGuards(ctx context.Context, run *provision.Run) error {
	if run.DestroyProtected {
		return errProvisionDestroyProtected
	}
	// 排空门（先于在途检查：未排空的机新任务仍在进来，等在途归零无意义）。
	if s.svcCtx != nil && s.svcCtx.RegistryStore != nil && len(run.AgentIDs) > 0 {
		store := s.svcCtx.RegistryStore
		store.Mu().RLock()
		sessions := store.AgentsUnsafe()
		for _, id := range run.AgentIDs {
			if sess, ok := sessions[id]; ok && sess != nil && !sess.Draining {
				store.Mu().RUnlock()
				return &errorx.CodeError{
					Code:       http.StatusConflict,
					Message:    fmt.Sprintf("目标机 agent %s 未排空，禁止销毁", id),
					StableCode: "provision_agents_not_drained",
				}
			}
		}
		store.Mu().RUnlock()
	}
	// 无负载才允许销毁（保守版 drain）：目标机在途任务未归零时拒绝。
	if len(run.AgentIDs) > 0 {
		if s.svcCtx == nil || s.svcCtx.DB == nil {
			return errProvisionLoadUnverifiable
		}
		taskCtx := s.svcCtx.GameScopeContext(run.Request.GameID, run.Request.Env)
		active, err := model.NewTaskRunModel(s.svcCtx.DB).CountActiveByAgents(taskCtx, run.AgentIDs, tasks.ReapableStatuses())
		if err != nil {
			return errProvisionLoadUnverifiable
		}
		if active > 0 {
			return &errorx.CodeError{
				Code:       http.StatusConflict,
				Message:    fmt.Sprintf("目标机仍有 %d 个在途任务，须先排水", active),
				StableCode: "provision_tasks_inflight",
			}
		}
	}
	return nil
}

// OpsProvisionProtection 设置/清除供给运行的数据盘护栏（OPS-002 持久化
// 标记禁止自动销毁）。仅 adopted 状态可护栏（销毁/失败/销毁后不可再改）；
// 成功落 audit provision.protect / provision.unprotect。
func (s *Service) OpsProvisionProtection(ctx context.Context, runID string, protected bool) (*provision.Run, error) {
	orch, err := s.provisionOrchestrator()
	if err != nil {
		return nil, err
	}
	runID = strings.TrimSpace(runID)
	run, err := orch.Store.Get(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run.State != provision.StateAdopted {
		return nil, &errorx.CodeError{
			Code:       http.StatusConflict,
			Message:    fmt.Sprintf("供给运行状态 %s 不允许修改护栏", run.State),
			StableCode: "provision_state_conflict",
		}
	}
	run.DestroyProtected = protected
	run.UpdatedAt = time.Now()
	if err := orch.Store.Update(ctx, run); err != nil {
		return nil, err
	}
	event := audit.EventProvisionUnprotect
	if protected {
		event = audit.EventProvisionProtect
	}
	recordOpsAudit(ctx, s.svcCtx, event, runID, "success", map[string]interface{}{
		"provider":  string(run.Request.Provider),
		"protected": protected,
	})
	return run, nil
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
	response.Accepted(c, OpsProvisionRunResponse{Run: run})
}

// ProvisionResume POST /ops/provision/:runId/resume。
func (h *Handler) ProvisionResume(c *gin.Context) {
	run, err := h.service.OpsProvisionResume(c.Request.Context(), c.Param("runId"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Accepted(c, OpsProvisionRunResponse{Run: run})
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

// ProvisionDestroyResume POST /ops/provision/:runId/destroy（OPS-003 审批
// 双模式 workflow：销毁审批通过后执行 terraform destroy 原语）。
func (h *Handler) ProvisionDestroyResume(c *gin.Context) {
	run, err := h.service.OpsProvisionDestroyResume(c.Request.Context(), c.Param("runId"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, OpsProvisionRunResponse{Run: run})
}

// OpsProvisionProtectionRequest PUT /ops/provision/:runId/protection 入参。
type OpsProvisionProtectionRequest struct {
	// DestroyProtected true=开启数据盘护栏（禁止自动销毁），false=清除。
	DestroyProtected bool `json:"destroyProtected"`
}

// ProvisionProtection PUT /ops/provision/:runId/protection（OPS-002 数据盘
// 护栏：持久化标记禁止自动销毁）。
func (h *Handler) ProvisionProtection(c *gin.Context) {
	var req OpsProvisionProtectionRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	run, err := h.service.OpsProvisionProtection(c.Request.Context(), c.Param("runId"), req.DestroyProtected)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, OpsProvisionRunResponse{Run: run})
}
