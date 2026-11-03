package ops

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/ops/costcontrol"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CostEstimateRequest POST /ops/cost/estimate 入参。
type CostEstimateRequest struct {
	Provider     string  `json:"provider"`
	InstanceType string  `json:"instanceType"`
	Count        int     `json:"count"`
	Hours        float64 `json:"hours"`
	DiskSizeGB   int     `json:"diskSizeGB"`
}

// CostSummaryRequest GET /ops/cost/summary 入参（可选）。
type CostSummaryRequest struct {
	Limit int `form:"limit"`
}

// CostBudgetUpsertRequest PUT /ops/cost/budget 入参。
type CostBudgetUpsertRequest struct {
	MonthlyUSD     float64  `json:"monthlyUsd"`
	NotifyChannels []string `json:"notifyChannels"`
}

// CostBudgetGetRequest GET /ops/cost/budget 入参（scope 从 ctx）。
type CostBudgetGetRequest struct{}

// CostIdleRequest GET /ops/cost/idle 入参。
type CostIdleRequest struct {
	Limit int `form:"limit"`
}

// costOrchestrator 返回成本控制编排器（依赖 DB 与指标库）。
func (s *Service) costOrchestrator() (*costcontrol.IdleEvaluator, *model.CostBudgetModel, error) {
	if s.svcCtx == nil || s.svcCtx.DB == nil {
		return nil, nil, &errorx.CodeError{Code: http.StatusServiceUnavailable, Message: "成本控制未配置", StableCode: "cost_not_configured"}
	}
	policy := costcontrol.DefaultIdlePolicy()
	eval := costcontrol.NewIdleEvaluator(policy, nil)
	if s.svcCtx.MetricsStore != nil {
		eval = costcontrol.NewIdleEvaluator(policy, func(agentID string, since time.Time) ([]float64, bool) {
			entries := s.svcCtx.MetricsStore.GetHistory(agentID, since, 200)
			out := make([]float64, 0, len(entries))
			for _, e := range entries {
				if e.Report.GetCpu() != nil {
					out = append(out, e.Report.GetCpu().GetUsagePercent())
				}
			}
			return out, len(out) > 0
		})
	}
	return eval, model.NewCostBudgetModel(s.svcCtx.DB), nil
}

// CostEstimate 创建前成本估算。
func (s *Service) CostEstimate(ctx context.Context, req *CostEstimateRequest) (*costcontrol.Estimate, error) {
	if req == nil {
		return nil, errorx.NewValidationError("estimate request required")
	}
	est, err := costcontrol.ComputeEstimate(costcontrol.EstimateInput{
		Provider:     strings.TrimSpace(req.Provider),
		InstanceType: strings.TrimSpace(req.InstanceType),
		Count:        req.Count,
		Hours:        req.Hours,
		DiskSizeGB:   req.DiskSizeGB,
	})
	if err != nil {
		return nil, errorx.NewValidationError(err.Error())
	}
	return est, nil
}

// CostSummary 成本总览（含闲置标记）。
func (s *Service) CostSummary(ctx context.Context, gameID, env string, limit int) (*costcontrol.Summary, error) {
	_, budgetModel, err := s.costOrchestrator()
	if err != nil {
		return nil, err
	}
	_ = budgetModel
	if s.svcCtx == nil || s.svcCtx.DB == nil {
		return nil, errors.New("cost store unavailable")
	}
	runModel := model.NewProvisionRunModel(s.svcCtx.DB)
	limit = normalizeLimit(limit, 200)
	// scope/state 过滤必须下推 SQL：若先按全局 limit 截断再内存过滤，
	// 目标 scope 的较早 adopted 运行落在最新 200 条之外会被静默漏计。
	runs, err := runModel.ListAdoptedScope(ctx, gameID, env, limit)
	if err != nil {
		return nil, err
	}
	scoped := runs
	now := time.Now()
	eval, _, err := s.costOrchestrator()
	if err != nil {
		return nil, err
	}
	idle := map[string]bool{}
	idleHours := map[string]float64{}
	for _, r := range scoped {
		var agents []string
		_ = json.Unmarshal(r.AgentIDsJSON, &agents)
		if len(agents) == 0 {
			continue
		}
		for _, a := range agents {
			st := eval.Evaluate(a, now)
			if st.Idle {
				idle[a] = true
				idleHours[a] = st.IdleHours
			}
		}
	}
	return costcontrol.BuildSummary(scoped, idle, idleHours, now), nil
}

// CostBudgetUpsert 设置月度预算。
func (s *Service) CostBudgetUpsert(ctx context.Context, gameID, env string, req *CostBudgetUpsertRequest, by string) (*costcontrol.Budget, error) {
	_, budgetModel, err := s.costOrchestrator()
	if err != nil {
		return nil, err
	}
	if gameID == "" || env == "" {
		return nil, errorx.NewValidationError("scope required")
	}
	if req == nil || req.MonthlyUSD < 0 {
		return nil, errorx.NewValidationError("monthlyUsd must be >= 0")
	}
	chJSON, _ := json.Marshal(req.NotifyChannels)
	b := &model.CostBudget{
		GameID: gameID, Env: env, MonthlyUSD: req.MonthlyUSD,
		NotifyChannelsJSON: string(chJSON), UpdatedBy: by,
	}
	if err := budgetModel.Upsert(ctx, b); err != nil {
		return nil, err
	}
	return &costcontrol.Budget{
		GameID: gameID, Env: env, MonthlyUSD: req.MonthlyUSD,
		NotifyChannels: req.NotifyChannels, UpdatedBy: by, UpdatedAt: time.Now().UTC(),
	}, nil
}

// CostBudgetGet 读取预算。
func (s *Service) CostBudgetGet(ctx context.Context, gameID, env string) (*costcontrol.Budget, error) {
	_, budgetModel, err := s.costOrchestrator()
	if err != nil {
		return nil, err
	}
	if gameID == "" || env == "" {
		return nil, errorx.NewValidationError("scope required")
	}
	b, err := budgetModel.Get(ctx, gameID, env)
	if err != nil {
		// 仅「未配置」返回 nil；任何其它 DB 错误（含文案含 "not found" 的
		// 真实故障）必须上抛，不得误判为无预算而掩盖故障。
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var channels []string
	_ = json.Unmarshal([]byte(b.NotifyChannelsJSON), &channels)
	return &costcontrol.Budget{
		GameID: b.GameID, Env: b.Env, MonthlyUSD: b.MonthlyUSD,
		NotifyChannels: channels, UpdatedBy: b.UpdatedBy, UpdatedAt: b.UpdatedAt,
	}, nil
}

// CostIdle 闲置节点列表（提醒/可自动回收）。
func (s *Service) CostIdle(ctx context.Context, gameID, env string, limit int) (*costcontrol.Summary, error) {
	return s.CostSummary(ctx, gameID, env, limit)
}

func normalizeLimit(v, def int) int {
	if v <= 0 || v > 1000 {
		return def
	}
	return v
}

// ============================================================================
// Handler
// ============================================================================

// CostEstimateHandler POST /ops/cost/estimate.
func (h *Handler) CostEstimateHandler(c *gin.Context) {
	var req CostEstimateRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	est, err := h.service.CostEstimate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, est)
}

// CostSummaryHandler GET /ops/cost/summary.
func (h *Handler) CostSummaryHandler(c *gin.Context) {
	scope := svc.GameScopeFromContext(c.Request.Context())
	var req CostSummaryRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	sum, err := h.service.CostSummary(c.Request.Context(), scope.GameID, scope.Env, req.Limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, sum)
}

// CostBudgetPutHandler PUT /ops/cost/budget.
func (h *Handler) CostBudgetPutHandler(c *gin.Context) {
	scope := svc.GameScopeFromContext(c.Request.Context())
	var req CostBudgetUpsertRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	b, err := h.service.CostBudgetUpsert(c.Request.Context(), scope.GameID, scope.Env, &req, c.GetString("username"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, b)
}

// CostBudgetGetHandler GET /ops/cost/budget.
func (h *Handler) CostBudgetGetHandler(c *gin.Context) {
	scope := svc.GameScopeFromContext(c.Request.Context())
	b, err := h.service.CostBudgetGet(c.Request.Context(), scope.GameID, scope.Env)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, b)
}

// CostIdleHandler GET /ops/cost/idle.
func (h *Handler) CostIdleHandler(c *gin.Context) {
	scope := svc.GameScopeFromContext(c.Request.Context())
	var req CostIdleRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	sum, err := h.service.CostIdle(c.Request.Context(), scope.GameID, scope.Env, req.Limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, sum)
}
