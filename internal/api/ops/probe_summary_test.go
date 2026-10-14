package ops

// probe 在线汇总端点测试（评审简档 §13.2）：
//   - agent 侧行来自最近一份 MetricsReport 的 probe_stats 快照
//   - server 本机行来自 ServerProbeManager.Statuses（读现状不注入新事实）
//   - scope 过滤（X-Game-ID/X-Env 头口径：同 scope 行 + 平台级行）
//   - 在线趋势灰态：onlineDataIntegrated 恒 false（SDK metric 入口未接入）

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/core/healthprobe"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

func summaryService(store *registry.MetricsStore, mgr *svc.ServerProbeManager) *Service {
	return &Service{svcCtx: &svc.ServiceContext{MetricsStore: store, ServerProbes: mgr}}
}

// summaryRows 按 (agent,target) 建索引。
func summaryRows(rows []ProbeSummaryRow) map[string]ProbeSummaryRow {
	out := make(map[string]ProbeSummaryRow, len(rows))
	for _, r := range rows {
		out[r.AgentID+"|"+r.Target] = r
	}
	return out
}

func TestProbeSummary_EmptyWhenNoSurfaces(t *testing.T) {
	s := summaryService(nil, nil)
	resp, err := s.ProbeSummary(context.Background(), &ProbeSummaryRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Rows)
	// 在线趋势列是「未接入」一等状态，不是零值伪装成 0。
	assert.False(t, resp.OnlineDataIntegrated)
}

func TestProbeSummary_AgentRowsFromLatestReport(t *testing.T) {
	store := registry.NewMetricsStore()
	store.Add("agent-a", &opsv1.MetricsReport{ProbeStats: []*opsv1.ProbeStats{
		{Target: "redis", Semantics: "readiness", GameId: "demo", Env: "prod",
			Checked: 10, Failed: 2, SuccessRate: 0.8, WindowOpen: true,
			WindowDurationMs: 4200, LastReason: "dial tcp: i/o timeout"},
		{Target: "mysql", Semantics: "liveness", GameId: "demo", Env: "prod",
			Checked: 10, Failed: 0, SuccessRate: 1},
		{Target: "reset", Semantics: "liveness", GameId: "demo", Env: "prod"},
		nil, // 空条目防御：不 panic、不出行
	}})

	s := summaryService(store, nil)
	resp, err := s.ProbeSummary(context.Background(), &ProbeSummaryRequest{})
	require.NoError(t, err)

	byKey := summaryRows(resp.Rows)
	require.Len(t, resp.Rows, 3, "nil ProbeStats 条目不得产生行")
	require.Contains(t, byKey, "agent-a|redis")
	row := byKey["agent-a|redis"]
	assert.Equal(t, "readiness", row.Semantics)
	assert.Equal(t, "demo", row.GameID)
	assert.Equal(t, "prod", row.Env)
	assert.True(t, row.WindowOpen)
	assert.False(t, row.Available, "窗口开着 = 不可用（两态内核）")
	assert.Equal(t, int64(4200), row.WindowDurationMs)
	assert.Equal(t, int64(10), row.Checked)
	assert.Equal(t, int64(2), row.Failed)
	assert.InDelta(t, 0.8, row.SuccessRate, 1e-9)
	assert.Equal(t, "dial tcp: i/o timeout", row.LastReason)
	assert.NotEmpty(t, row.LastReportAt, "数据新鲜度锚点必须回传")

	// 窗口关着的行 = 可用，计数照实透传。
	ok := byKey["agent-a|mysql"]
	assert.True(t, ok.Available)
	assert.False(t, ok.WindowOpen)
	assert.Zero(t, ok.WindowDurationMs)
}

func TestProbeSummary_ServerRowsReadCurrentState(t *testing.T) {
	mgr, err := svc.NewServerProbeManager([]healthprobe.TargetConfig{
		{Name: "dead-tcp", Type: healthprobe.TargetTypeTCP, Address: "127.0.0.1:1",
			FailThreshold: 1, Interval: time.Minute},
	}, nil, 0, nil)
	require.NoError(t, err)

	s := summaryService(nil, mgr)
	resp, err := s.ProbeSummary(context.Background(), &ProbeSummaryRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Rows, 1)

	row := resp.Rows[0]
	assert.Equal(t, svc.ServerProbeAgentID, row.AgentID)
	assert.Equal(t, "dead-tcp", row.Target)
	assert.NotEmpty(t, row.Semantics, "server 行也带视角声明")
	// 读侧不探测：尚未探测过 = 无窗口 = 可用（事实缺失不谎报故障）。
	assert.True(t, row.Available)
	assert.False(t, row.WindowOpen)
	assert.NotEmpty(t, row.LastReportAt)
}

func TestProbeSummary_ScopeFiltering(t *testing.T) {
	store := registry.NewMetricsStore()
	store.Add("agent-a", &opsv1.MetricsReport{ProbeStats: []*opsv1.ProbeStats{
		{Target: "t-demo-prod", GameId: "demo", Env: "prod"},
		{Target: "t-demo-dev", GameId: "demo", Env: "dev"},
		{Target: "t-other-prod", GameId: "other", Env: "prod"},
		{Target: "t-platform"}, // 平台级行：任何 scope 下都可见
	}})

	s := summaryService(store, nil)

	// 不带 scope：全部可见。
	all, err := s.ProbeSummary(context.Background(), &ProbeSummaryRequest{})
	require.NoError(t, err)
	assert.Len(t, all.Rows, 4)

	// 带 scope demo/prod：同 scope 行 + 平台级行（告警列表 4862 同口径）。
	scoped, err := s.ProbeSummary(svc.WithGameScope(context.Background(),
		svc.GameScope{GameID: "demo", Env: "prod"}), &ProbeSummaryRequest{})
	require.NoError(t, err)
	byKey := summaryRows(scoped.Rows)
	require.Len(t, scoped.Rows, 2)
	assert.Contains(t, byKey, "agent-a|t-demo-prod")
	assert.Contains(t, byKey, "agent-a|t-platform")
}

func TestProbeSummary_ServerRowsArePlatformScopeOnly(t *testing.T) {
	mgr, err := svc.NewServerProbeManager([]healthprobe.TargetConfig{
		{Name: "dead-tcp", Type: healthprobe.TargetTypeTCP, Address: "127.0.0.1:1",
			FailThreshold: 1, Interval: time.Minute},
	}, nil, 0, nil)
	require.NoError(t, err)

	s := summaryService(nil, mgr)
	// server 探的是自身依赖，天然平台级：带 scope 时作为平台级行可见。
	resp, err := s.ProbeSummary(svc.WithGameScope(context.Background(),
		svc.GameScope{GameID: "demo", Env: "prod"}), &ProbeSummaryRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Rows, 1)
	assert.Equal(t, svc.ServerProbeAgentID, resp.Rows[0].AgentID)
	assert.Empty(t, resp.Rows[0].GameID)
	assert.Empty(t, resp.Rows[0].Env)
}

func TestScopeMatches_Matrix(t *testing.T) {
	cases := []struct {
		name        string
		scope       svc.GameScope
		gameID, env string
		want        bool
	}{
		{"no scope sees all", svc.GameScope{}, "demo", "prod", true},
		{"exact match", svc.GameScope{GameID: "demo", Env: "prod"}, "demo", "prod", true},
		{"game mismatch", svc.GameScope{GameID: "demo", Env: "prod"}, "other", "prod", false},
		{"env mismatch", svc.GameScope{GameID: "demo", Env: "prod"}, "demo", "dev", false},
		{"platform row always visible", svc.GameScope{GameID: "demo", Env: "prod"}, "", "", true},
		{"game-only scope matches env-any", svc.GameScope{GameID: "demo"}, "demo", "dev", true},
		{"game-only scope rejects other game", svc.GameScope{GameID: "demo"}, "other", "prod", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, scopeMatches(tc.scope, tc.gameID, tc.env))
		})
	}
}
