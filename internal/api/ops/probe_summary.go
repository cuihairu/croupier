package ops

// ops 在线汇总（agent-health-recovery-review §13.2，R3）：状态矩阵 + 证据，
// 不出单一合成分（权重无真实数据支撑）。数据口径：
//   - 探活结果 = 各 agent 最近一份 MetricsReport 的 probe_stats 快照
//     （30s 批量面新鲜度）+ server 本机探针面当前状态；
//   - 在线趋势 = 灰态「未接入在线数据」（SDK metric 入口未排期，
//     §13.4 随第一个真实游戏接入；复用 §5.5.3 not-integrated 一等状态）。
//   - 协议标注 = 行级 probeType/protocol/phase 透传（§5.5.3 协议四态的
//     标注面）：protocol 行据此区分 不可达 / 握手失败 / 已验证。
//
// scope 维度走 X-Game-ID/X-Env 头（GameScopeFromContext）：带 scope 时显示
// 同 scope 行 + 平台级行（空 game/env），不带 scope 显示全部——与告警列表
// 4862 口径一致。

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/svc"
)

// ProbeSummaryRequest 汇总查询请求（scope 走 X-Game-ID/X-Env 头）。
// TargetCategory 类别筛选（目标分类设计 §4.4）：空 = 全部；
// game|third_party 由服务端按 scope+类别过滤行（服务端聚合，非前端筛）；
// 未知值 400 拒绝。未分类历史行（旧版 agent 未携带类别）只作受权诊断——
// 不计入任一类别筛选，仅在「全部」下可见。
type ProbeSummaryRequest struct {
	TargetCategory string `form:"targetCategory"`
}

// 目标类别闭集（与 core/healthprobe.TargetCategory* 同口径；API 面独立闭集
// 校验，不引公共包依赖）。
const (
	probeCategoryAll     = ""
	probeCategoryGame    = "game"
	probeCategoryPartner = "third_party"
)

// probeCategoryValid 校验类别筛选值；空 = 全部。
func probeCategoryValid(v string) bool {
	switch v {
	case probeCategoryAll, probeCategoryGame, probeCategoryPartner:
		return true
	}
	return false
}

// probeCategoryMatches 行级类别匹配：无筛选恒真；有筛选按执行时快照严格
// 相等——未分类行（空类别）不计入任一类别（不默认游戏）。
func probeCategoryMatches(filter, category string) bool {
	if filter == probeCategoryAll {
		return true
	}
	return category == filter
}

// ProbeSummaryRow 状态矩阵单行（目标 × 执行位置）。
type ProbeSummaryRow struct {
	AgentID string `json:"agentId"`
	Target  string `json:"target"`
	// Semantics 视角声明（readiness 等）。
	Semantics string `json:"semantics"`
	// GameID/Env 目标 scope（空 = 平台级）。
	GameID string `json:"gameId,omitempty"`
	Env    string `json:"env,omitempty"`
	// TargetCategory 目标归属（game|third_party），由配置快照随统计透传。
	TargetCategory string `json:"targetCategory,omitempty"`
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
	// ProbeType checker 类型（http|tcp|db|protocol）；Protocol 游戏协议
	// 适配器名（仅 type=protocol 且走适配器）；Phase 最近一次失败的阶段
	// 标记（协议 Detailing 出口）。三者把 §5.5.3「协议四态」带到面板：
	// protocol 行可区分 不可达（无 phase）/ 握手失败（phase=login 等）/
	// 已验证（Available=true），不必解析 lastReason 文本。
	ProbeType string `json:"probeType,omitempty"`
	Protocol  string `json:"protocol,omitempty"`
	Phase     string `json:"phase,omitempty"`
	// ProtocolExpected 目标声明了协议期望但只落在 http/tcp/db 层
	// （§5.5.3 reachable-unverified 触发位；面板据此标「可达≠可用」灰态）。
	ProtocolExpected bool `json:"protocolExpected,omitempty"`
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
	if req == nil {
		req = &ProbeSummaryRequest{}
	}
	if !probeCategoryValid(req.TargetCategory) {
		return nil, fmt.Errorf("targetCategory must be game|third_party (got %q)", req.TargetCategory)
	}
	category := req.TargetCategory
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
				if !probeCategoryMatches(category, st.GetTargetCategory()) {
					continue
				}
				resp.Rows = append(resp.Rows, ProbeSummaryRow{
					AgentID:          agentID,
					Target:           st.GetTarget(),
					Semantics:        st.GetSemantics(),
					GameID:           st.GetGameId(),
					Env:              st.GetEnv(),
					TargetCategory:   st.GetTargetCategory(),
					Available:        !st.GetWindowOpen(),
					WindowOpen:       st.GetWindowOpen(),
					WindowDurationMs: st.GetWindowDurationMs(),
					Checked:          st.GetChecked(),
					Failed:           st.GetFailed(),
					SuccessRate:      st.GetSuccessRate(),
					LastReason:       st.GetLastReason(),
					LastReportAt:     entry.Received.UTC().Format(time.RFC3339),
					ProbeType:        st.GetProbeType(),
					Protocol:         st.GetProtocol(),
					Phase:            st.GetPhase(),
					ProtocolExpected: st.GetProtocolExpected(),
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
			if !probeCategoryMatches(category, st.Category) {
				continue
			}
			resp.Rows = append(resp.Rows, ProbeSummaryRow{
				AgentID:          svc.ServerProbeAgentID,
				Target:           st.Target,
				Semantics:        st.Semantics,
				TargetCategory:   st.Category,
				Available:        st.Available,
				WindowOpen:       st.WindowOpen,
				WindowDurationMs: st.DurationMS,
				LastReason:       st.LastReason,
				LastReportAt:     now,
				ProbeType:        st.ProbeType,
				Protocol:         st.Protocol,
				Phase:            st.Phase,
				ProtocolExpected: st.ProtocolExpected,
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

// ProbeSummaryHandler handles GET /api/v1/ops/probe/summary?targetCategory=…
// 类别筛选值闭集校验在 handler 面 400 拒绝（目标分类设计 §4.4：未知值拒绝）。
func (h *Handler) ProbeSummaryHandler(c *gin.Context) {
	var req ProbeSummaryRequest
	if err := bindOpsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	if !probeCategoryValid(req.TargetCategory) {
		response.BadRequest(c, fmt.Sprintf("targetCategory must be game|third_party (got %q)", req.TargetCategory))
		return
	}
	resp, err := h.service.ProbeSummary(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}
