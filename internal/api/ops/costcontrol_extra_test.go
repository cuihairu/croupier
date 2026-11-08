package ops

// costcontrol.go 补测：costOrchestrator 指标闭包低载样本分支、CostSummary
// 空 agents 直通与 ListAdoptedScope 错误透传、预算 upsert/get 守卫与落库
// 错误、CostIdle 委托、normalizeLimit 边界，及五个 cost handler 的往返与
// 坏载荷面。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/ops/costcontrol"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCostSummary_IdleFromLowCpuSamples(t *testing.T) {
	ctx := context.Background()
	s := &Service{svcCtx: costTestSvcCtx(t)}
	runModel := model.NewProvisionRunModel(s.svcCtx.DB)
	require.NoError(t, runModel.Create(ctx, &model.ProvisionRun{
		RunID: "prov-i", Provider: "hetzner", Region: "eu", GameID: "g", Env: "e",
		State: "adopted", Count: 1, CreatedAt: time.Now().Add(-time.Hour),
		SpecJSON: model.JSON(`{"instanceType":"cx21"}`), AgentIDsJSON: model.JSON(`["a1"]`),
	}))
	// 空 agents 名单 → 直通分支
	require.NoError(t, runModel.Create(ctx, &model.ProvisionRun{
		RunID: "prov-e", Provider: "hetzner", Region: "eu", GameID: "g", Env: "e",
		State: "adopted", Count: 1, CreatedAt: time.Now().Add(-time.Hour),
		SpecJSON: model.JSON(`{"instanceType":"cx21"}`), AgentIDsJSON: model.JSON(`[]`),
	}))

	// 窗口内低载样本 → 判闲置（窗口内任一超阈样本即取消）
	s.svcCtx.MetricsStore.Add("a1", &opsv1.MetricsReport{
		AgentId: "a1", Cpu: &opsv1.CpuMetrics{UsagePercent: 1.5},
	})

	sum, err := s.CostSummary(ctx, "g", "e", 100)
	require.NoError(t, err)
	require.Equal(t, 2, sum.TotalNodes)
	require.Equal(t, 1, sum.IdleCount, "low-cpu agent must be flagged idle")
}

func TestCostSummary_ListAdoptedErrorPropagates(t *testing.T) {
	s := &Service{svcCtx: costTestSvcCtx(t)}
	require.NoError(t, s.svcCtx.DB.Migrator().DropTable("provision_runs"))
	_, err := s.CostSummary(context.Background(), "g", "e", 100)
	require.Error(t, err)
}

func TestCostBudgetUpsertGuardsAndDBError(t *testing.T) {
	ctx := context.Background()

	// 未配置 DB → 503 形态
	s := &Service{svcCtx: &svc.ServiceContext{}}
	_, err := s.CostBudgetUpsert(ctx, "g", "e", &CostBudgetUpsertRequest{MonthlyUSD: 10}, "ops")
	require.ErrorContains(t, err, "成本控制未配置")

	// scope 缺失 / 负预算 / nil 请求
	s2 := &Service{svcCtx: costTestSvcCtx(t)}
	_, err = s2.CostBudgetUpsert(ctx, "", "e", &CostBudgetUpsertRequest{MonthlyUSD: 10}, "ops")
	require.ErrorContains(t, err, "scope required")
	_, err = s2.CostBudgetUpsert(ctx, "g", "", &CostBudgetUpsertRequest{MonthlyUSD: 10}, "ops")
	require.ErrorContains(t, err, "scope required")
	_, err = s2.CostBudgetUpsert(ctx, "g", "e", nil, "ops")
	require.ErrorContains(t, err, "monthlyUsd must be >= 0")
	_, err = s2.CostBudgetUpsert(ctx, "g", "e", &CostBudgetUpsertRequest{MonthlyUSD: -1}, "ops")
	require.ErrorContains(t, err, "monthlyUsd must be >= 0")

	// 落库错误（表被摘除）上抛
	require.NoError(t, s2.svcCtx.DB.Migrator().DropTable("cost_budgets"))
	_, err = s2.CostBudgetUpsert(ctx, "g", "e", &CostBudgetUpsertRequest{MonthlyUSD: 10}, "ops")
	require.Error(t, err)
}

func TestCostBudgetGetGuards(t *testing.T) {
	ctx := context.Background()

	s := &Service{svcCtx: &svc.ServiceContext{}}
	_, err := s.CostBudgetGet(ctx, "g", "e")
	require.ErrorContains(t, err, "成本控制未配置")

	s2 := &Service{svcCtx: costTestSvcCtx(t)}
	_, err = s2.CostBudgetGet(ctx, "", "e")
	require.ErrorContains(t, err, "scope required")
	_, err = s2.CostBudgetGet(ctx, "g", "")
	require.ErrorContains(t, err, "scope required")
}

func TestCostIdleDelegatesToSummary(t *testing.T) {
	ctx := context.Background()
	s := &Service{svcCtx: costTestSvcCtx(t)}
	sum, err := s.CostIdle(ctx, "g", "e", 0)
	require.NoError(t, err)
	require.NotNil(t, sum)
}

func TestNormalizeLimitBoundaries(t *testing.T) {
	assert.Equal(t, 200, normalizeLimit(0, 200))
	assert.Equal(t, 200, normalizeLimit(-5, 200))
	assert.Equal(t, 200, normalizeLimit(1001, 200))
	assert.Equal(t, 500, normalizeLimit(500, 200))
}

func TestCostHandlersRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svcCtx := costTestSvcCtx(t)
	h := NewHandler(NewService(svcCtx))

	mkCtx := func(t *testing.T, method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
		t.Helper()
		req := httptest.NewRequest(method, target, nil)
		if body != "" {
			req = httptest.NewRequest(method, target, strings.NewReader(body))
		}
		req = req.WithContext(svc.WithGameScope(req.Context(), svc.GameScope{GameID: "g", Env: "e"}))
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("username", "ops")
		return c, w
	}

	// estimate 坏载荷 → 错误响应
	c, w := mkCtx(t, http.MethodPost, "/ops/cost/estimate", "{bad")
	h.CostEstimateHandler(c)
	require.NotEqual(t, http.StatusOK, w.Code)

	// estimate 合法 → 200
	c, w = mkCtx(t, http.MethodPost, "/ops/cost/estimate",
		`{"provider":"hetzner","instanceType":"cx21","count":2,"hours":10}`)
	h.CostEstimateHandler(c)
	require.Equal(t, http.StatusOK, w.Code)
	var est costcontrol.Estimate
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &est))
	require.InDelta(t, 0.12, est.TotalUSD, 1e-9)

	// summary（query 绑定 limit）→ 200
	c, w = mkCtx(t, http.MethodGet, "/ops/cost/summary?limit=50", "")
	h.CostSummaryHandler(c)
	require.Equal(t, http.StatusOK, w.Code)

	// budget upsert → 200；get → 200
	c, w = mkCtx(t, http.MethodPut, "/ops/cost/budget", `{"monthlyUsd":99,"notifyChannels":["webhook"]}`)
	h.CostBudgetPutHandler(c)
	require.Equal(t, http.StatusOK, w.Code)
	var budget map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &budget))
	require.InDelta(t, 99.0, budget["monthlyUsd"].(float64), 1e-9)

	c, w = mkCtx(t, http.MethodGet, "/ops/cost/budget", "")
	h.CostBudgetGetHandler(c)
	require.Equal(t, http.StatusOK, w.Code)

	// idle → 200
	c, w = mkCtx(t, http.MethodGet, "/ops/cost/idle?limit=0", "")
	h.CostIdleHandler(c)
	require.Equal(t, http.StatusOK, w.Code)
}
