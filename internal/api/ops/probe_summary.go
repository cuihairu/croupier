package ops

// ops 在线汇总（agent-health-recovery-review §13.2，R3）：状态矩阵 + 证据，
// 不出单一合成分（权重无真实数据支撑）。数据口径：
//   - 探活结果 = 各 agent 最近一份 MetricsReport 的 probe_stats 快照
//     （30s 批量面新鲜度）+ server 本机探针面当前状态；
//   - 在线趋势 = 灰态「未接入在线数据」（SDK metric 入口未排期，
//     §13.4 随第一个真实游戏接入；复用 §5.5.3 not-integrated 一等状态）。
//
// scope 维度走 X-Game-ID/X-Env 头（GameScopeFromContext）：带 scope 时显示
// 同 scope 行 + 平台级行（空 game/env），不带 scope 显示全部——与告警列表
// 4862 口径一致。

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/svc"
)

// ProbeSummaryRequest 汇总查询请求（scope 走 X-Game-ID/X-Env 头，无查询参数）。
type ProbeSummaryRequest struct{}

// ProbeSummaryRow 状态矩阵单行（目标 × 执行位置）。
type ProbeSummaryRow struct {
	AgentID string `json:"agentId"`
	Target  string `json:"target"`
	// Semantics 视角声明（readiness 等）。
	Semantics string `json:"semantics"`
	// GameID/Env 目标 scope（空 = 平台级）。
	GameID string `json:"gameId,omitempty"`
	Env    string `json:"env,omitempty"`
	// Available 探活结果（内核两态：探测是事实不是渐变分数）。
	Available bool `json:"available"`
	// WindowOpen 报告时点故障窗口是否开着。
	WindowOpen bool `json:"windowOpen"`
	// WindowDurationMs 开着的窗口已持续时长。
	WindowDurationMs int64 `json:"windowDurationMs,omitempty"`
	// Checked/Failed/SuccessRate 最近窗口的聚合计数（正常态聚合口径 §13.1）。
	Checked     int64   `json:"checked"`
	Failed      int64   `json:"failed"`
	SuccessRate float64 `json:"successRate"`
	// LastReason 最近一次失败原因（异常态证据）。
	LastReason string `json:"lastReason,omitempty"`
	// LastReportAt 该行数据的新鲜度锚点（RFC3339；server 本机行=快照时点）。
	LastReportAt string `json:"lastReportAt,omitempty"`
}

// ProbeSummaryResponse 在线汇总响应。
type ProbeSummaryResponse struct {
	// OnlineDataIntegrated 在线数据入口是否已接入（§13.2 B#2：来源=游戏
	// 服务器自身上报，SDK metric 入口未排期——灰态一等状态，不造第二套）。
	OnlineDataIntegrated bool              `json:"onlineDataIntegrated"`
	Rows                 []ProbeSummaryRow `json:"rows"`
}

// ProbeSummary GET /api/v1/ops/probe/summary。
func (s *Service) ProbeSummary(ctx context.Context, req *ProbeSummaryRequest) (*ProbeSummaryResponse, error) {
	scope := svc.GameScopeFromContext(ctx)
	resp := &ProbeSummaryResponse{OnlineDataIntegrated: false, Rows: []ProbeSummaryRow{}}

	// agent 视角：最近一份上报的 probe_stats 快照。
	if s.svcCtx != nil && s.svcCtx.MetricsStore != nil {
		for _, agentID := range s.svcCtx.MetricsStore.ListAgents() {
			entry, ok := s.svcCtx.MetricsStore.GetLatest(agentID)
			if !ok || entry == nil || entry.Report == nil {
				continue
			}
			for _, st := range entry.Report.GetProbeStats() {
				if st == nil {
					continue
				}
				if !scopeMatches(scope, st.GetGameId(), st.GetEnv()) {
					continue
				}
				resp.Rows = append(resp.Rows, ProbeSummaryRow{
					AgentID:          agentID,
					Target:           st.GetTarget(),
					Semantics:        st.GetSemantics(),
					GameID:           st.GetGameId(),
					Env:              st.GetEnv(),
					Available:        !st.GetWindowOpen(),
					WindowOpen:       st.GetWindowOpen(),
					WindowDurationMs: st.GetWindowDurationMs(),
					Checked:          st.GetChecked(),
					Failed:           st.GetFailed(),
					SuccessRate:      st.GetSuccessRate(),
					LastReason:       st.GetLastReason(),
					LastReportAt:     entry.Received.UTC().Format(time.RFC3339),
				})
			}
		}
	}

	// server 视角：本机探针面当前状态（§4.3 多视角并存，agentID=server）。
	if s.svcCtx != nil && s.svcCtx.ServerProbes != nil {
		now := time.Now().UTC().Format(time.RFC3339)
		for _, st := range s.svcCtx.ServerProbes.Statuses() {
			// server 本机目标天然平台级（无 game/env 归属）。
			if !scopeMatches(scope, "", "") {
				continue
			}
			resp.Rows = append(resp.Rows, ProbeSummaryRow{
				AgentID:          svc.ServerProbeAgentID,
				Target:           st.Target,
				Semantics:        st.Semantics,
				Available:        st.Available,
				WindowOpen:       st.WindowOpen,
				WindowDurationMs: st.DurationMS,
				LastReason:       st.LastReason,
				LastReportAt:     now,
			})
		}
	}
	return resp, nil
}

// scopeMatches 带空 scope 的 ctx 显示全部；带 scope 显示同 scope 行 +
// 平台级行（空 game/env；告警列表 4862 同口径）。
func scopeMatches(scope svc.GameScope, gameID, env string) bool {
	if scope.GameID == "" && scope.Env == "" {
		return true
	}
	if gameID == "" && env == "" {
		return true
	}
	if scope.GameID != "" && gameID != scope.GameID {
		return false
	}
	if scope.Env != "" && env != scope.Env {
		return false
	}
	return true
}

// ProbeSummaryHandler handles GET /api/v1/ops/probe/summary.
func (h *Handler) ProbeSummaryHandler(c *gin.Context) {
	resp, err := h.service.ProbeSummary(c.Request.Context(), &ProbeSummaryRequest{})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}
