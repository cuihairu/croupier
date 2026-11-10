package analytics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- test doubles：每域独立 mock（facade 只依赖接口的验收面） ----

type stubOverview struct{ err error }

func (s stubOverview) Overview(context.Context, *OverviewRequest) (*OverviewResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &OverviewResponse{Metrics: OverviewMetrics{DAU: 42, MAU: 7, NewUsers: 3}}, nil
}

type stubBehavior struct{}

func (stubBehavior) Behavior(context.Context, *BehaviorRequest) (*BehaviorResponse, error) {
	return &BehaviorResponse{TopActions: []map[string]interface{}{{"action": "login"}}}, nil
}

type stubPayments struct{ err error }

func (s stubPayments) PaymentsSummary(context.Context, *PaymentsSummaryRequest) (*PaymentsSummaryResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &PaymentsSummaryResponse{Items: []PaymentsSummary{{Date: "2026-11-11", Revenue: 9.9}}}, nil
}

type stubRetention struct{}

func (stubRetention) Retention(context.Context, *RetentionRequest) (*RetentionResponse, error) {
	return &RetentionResponse{Cohorts: []RetentionCohort{{Cohort: "2026-11-01", Users: 10}}}, nil
}

type stubRealtime struct{}

func (stubRealtime) Realtime(context.Context, *RealtimeRequest) (*RealtimeResponse, error) {
	return &RealtimeResponse{RealtimeMetrics: RealtimeMetrics{OnlineUsers: 5}}, nil
}

type stubInvocations struct{}

func (stubInvocations) InvocationsSummary(context.Context, *InvocationsSummaryRequest) (*InvocationsSummaryResponse, error) {
	return &InvocationsSummaryResponse{Total: 3, SuccessRate: 1}, nil
}

type stubLevels struct{}

func (stubLevels) Levels(context.Context, *LevelsRequest) (*LevelsResponse, error) {
	return &LevelsResponse{Levels: []LevelMetrics{{LevelId: "1-1", CompletionRate: 0.5}}}, nil
}

type stubWarehouse struct{ err error }

func (s stubWarehouse) WarehouseDAU(context.Context, *WarehouseDAURequest) (*WarehouseDAUResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &WarehouseDAUResponse{Points: []WarehouseDAUPoint{{Date: "2026-11-11"}}}, nil
}

func allOK() *Facade {
	return &Facade{
		Overview:    stubOverview{},
		Behavior:    stubBehavior{},
		Payments:    stubPayments{},
		Retention:   stubRetention{},
		Realtime:    stubRealtime{},
		Invocations: stubInvocations{},
		Levels:      stubLevels{},
		Warehouse:   stubWarehouse{},
	}
}

func cardOf(resp *FacadeSummaryResponse, domain string) *DomainCard {
	for _, c := range resp.Domains {
		if c.Domain == domain {
			return c
		}
	}
	return nil
}

func TestFacadeSummaryAggregatesAllDomains(t *testing.T) {
	resp := allOK().Summary(context.Background(), "demo", "prod")

	assert.Equal(t, "demo", resp.GameId)
	assert.Equal(t, "prod", resp.Env)
	assert.NotEmpty(t, resp.GeneratedAt)

	// 8 已建域全部 ok + 5 未建域 pending_data_source。
	require.Len(t, resp.Domains, 13)
	for _, domain := range []string{DomainOverview, DomainBehavior, DomainPayments, DomainRetention, DomainRealtime, DomainInvocations, DomainLevels, DomainWarehouse} {
		card := cardOf(resp, domain)
		require.NotNil(t, card, domain)
		assert.Equal(t, CardStatusOK, card.Status, domain)
		assert.NotNil(t, card.Data, domain)
		assert.Empty(t, card.Error, domain)
	}
	for _, domain := range PendingDomains {
		card := cardOf(resp, domain)
		require.NotNil(t, card, domain)
		assert.Equal(t, CardStatusPendingData, card.Status, domain)
		assert.Nil(t, card.Data, domain)
	}

	// 摘要数据是真实域响应（非合成占位）。
	overview, ok := cardOf(resp, DomainOverview).Data.(*OverviewResponse)
	require.True(t, ok)
	assert.Equal(t, 42, overview.Metrics.DAU)
}

func TestFacadeSummaryIsolatesPerDomainFailure(t *testing.T) {
	f := allOK()
	f.Payments = stubPayments{err: errors.New("payments db down")}
	f.Warehouse = stubWarehouse{err: errors.New("clickhouse unreachable")}

	resp := f.Summary(context.Background(), "demo", "")

	pay := cardOf(resp, DomainPayments)
	require.NotNil(t, pay)
	assert.Equal(t, CardStatusUnavailable, pay.Status)
	assert.Contains(t, pay.Error, "payments db down")
	assert.Nil(t, pay.Data)

	wh := cardOf(resp, DomainWarehouse)
	require.NotNil(t, wh)
	assert.Equal(t, CardStatusUnavailable, wh.Status)
	assert.Contains(t, wh.Error, "clickhouse unreachable")

	// 其余域不受单域故障影响。
	assert.Equal(t, CardStatusOK, cardOf(resp, DomainOverview).Status)
	assert.Equal(t, CardStatusOK, cardOf(resp, DomainRealtime).Status)
}

func TestFacadeSummaryNilDomainStaysUnavailable(t *testing.T) {
	f := allOK()
	f.Levels = nil
	resp := f.Summary(context.Background(), "demo", "")
	card := cardOf(resp, DomainLevels)
	require.NotNil(t, card)
	assert.Equal(t, CardStatusUnavailable, card.Status)
}

func TestFacadeQueryRoutesPerDomain(t *testing.T) {
	f := allOK()
	ctx := context.Background()

	// overview：timeRange 透传。
	data, err := f.Query(ctx, &AnalyticsQuery{Domain: DomainOverview, Filter: map[string]string{"gameId": "demo", "env": "prod"}, TimeRange: FacadeTimeRange{StartDate: "2026-11-01", EndDate: "2026-11-11"}})
	require.NoError(t, err)
	overview, ok := data.(*OverviewResponse)
	require.True(t, ok)
	assert.Equal(t, 42, overview.Metrics.DAU)

	// retention：filter.cohort 透传。
	data, err = f.Query(ctx, &AnalyticsQuery{Domain: DomainRetention, Filter: map[string]string{"gameId": "demo", "cohort": "2026-11-01"}})
	require.NoError(t, err)
	ret, ok := data.(*RetentionResponse)
	require.True(t, ok)
	assert.Equal(t, "2026-11-01", ret.Cohorts[0].Cohort)

	// warehouse：filter.days 透传。
	data, err = f.Query(ctx, &AnalyticsQuery{Domain: DomainWarehouse, Filter: map[string]string{"gameId": "demo", "days": "7"}})
	require.NoError(t, err)
	wh, ok := data.(*WarehouseDAUResponse)
	require.True(t, ok)
	assert.Len(t, wh.Points, 1)

	// 其余域逐个路由（类型断言即路由正确性）。
	data, err = f.Query(ctx, &AnalyticsQuery{Domain: DomainBehavior, Filter: map[string]string{"gameId": "demo"}})
	require.NoError(t, err)
	_, ok = data.(*BehaviorResponse)
	assert.True(t, ok)

	data, err = f.Query(ctx, &AnalyticsQuery{Domain: DomainPayments, Filter: map[string]string{"gameId": "demo"}})
	require.NoError(t, err)
	_, ok = data.(*PaymentsSummaryResponse)
	assert.True(t, ok)

	data, err = f.Query(ctx, &AnalyticsQuery{Domain: DomainRealtime, Filter: map[string]string{"gameId": "demo"}})
	require.NoError(t, err)
	_, ok = data.(*RealtimeResponse)
	assert.True(t, ok)

	data, err = f.Query(ctx, &AnalyticsQuery{Domain: DomainInvocations, Filter: map[string]string{"gameId": "demo"}})
	require.NoError(t, err)
	_, ok = data.(*InvocationsSummaryResponse)
	assert.True(t, ok)

	data, err = f.Query(ctx, &AnalyticsQuery{Domain: DomainLevels, Filter: map[string]string{"gameId": "demo"}})
	require.NoError(t, err)
	_, ok = data.(*LevelsResponse)
	assert.True(t, ok)
}

func TestFacadeQueryValidation(t *testing.T) {
	f := allOK()
	ctx := context.Background()

	// 缺 domain。
	_, err := f.Query(ctx, &AnalyticsQuery{Filter: map[string]string{"gameId": "demo"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "domain is required")

	// 缺 gameId。
	_, err = f.Query(ctx, &AnalyticsQuery{Domain: DomainOverview})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "filter.gameId is required")

	// 未知域 422。
	_, err = f.Query(ctx, &AnalyticsQuery{Domain: "nope", Filter: map[string]string{"gameId": "demo"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown domain")

	// 未建域判定。
	assert.True(t, isPendingDomain("economy"))
	assert.True(t, isPendingDomain("campaign"))
	assert.False(t, isPendingDomain(DomainOverview))
}

func TestFacadeHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(nil), config.SSEConfig{})
	h.facade = allOK() // 同包注入 stub 门面（不触真数据源）

	// summary：缺 gameId 422。
	c, rec := newFacadeContext(http.MethodGet, "/api/v1/analytics/facade/summary", "")
	h.FacadeSummary(c)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, rec.Body.String(), "validation_failed")

	// summary：带 gameId 返回聚合卡组（含 pending_data_source 域）。
	c2, rec2 := newFacadeContext(http.MethodGet, "/api/v1/analytics/facade/summary?gameId=demo&env=prod", "")
	h.FacadeSummary(c2)
	require.Equal(t, http.StatusOK, rec2.Code)
	payload := rec2.Body.String()
	assert.Contains(t, payload, `"gameId":"demo"`)
	assert.Contains(t, payload, `"status":"ok"`)
	assert.Contains(t, payload, `"status":"pending_data_source"`)

	// query：JSON body 路由到 stub 域。
	body := `{"domain":"overview","filter":{"gameId":"demo","env":"prod"},"timeRange":{"startDate":"2026-11-01","endDate":"2026-11-11"}}`
	c3, rec3 := newFacadeContext(http.MethodPost, "/api/v1/analytics/facade/query", body)
	h.FacadeQuery(c3)
	require.Equal(t, http.StatusOK, rec3.Code)
	assert.Contains(t, rec3.Body.String(), `"domain":"overview"`)
	assert.Contains(t, rec3.Body.String(), `"dau":42`)

	// query：未建域 422 domain_pending。
	c4, rec4 := newFacadeContext(http.MethodPost, "/api/v1/analytics/facade/query", `{"domain":"economy","filter":{"gameId":"demo"}}`)
	h.FacadeQuery(c4)
	assert.Equal(t, http.StatusUnprocessableEntity, rec4.Code)
	assert.Contains(t, rec4.Body.String(), "domain_pending")

	// query：未知域 422。
	c5, rec5 := newFacadeContext(http.MethodPost, "/api/v1/analytics/facade/query", `{"domain":"nope","filter":{"gameId":"demo"}}`)
	h.FacadeQuery(c5)
	assert.Equal(t, http.StatusUnprocessableEntity, rec5.Code)
	assert.Contains(t, rec5.Body.String(), "unknown domain")

	// query：缺 gameId 422。
	c6, rec6 := newFacadeContext(http.MethodPost, "/api/v1/analytics/facade/query", `{"domain":"overview"}`)
	h.FacadeQuery(c6)
	assert.Equal(t, http.StatusUnprocessableEntity, rec6.Code)

	// query：坏 JSON 400。
	c7, rec7 := newFacadeContext(http.MethodPost, "/api/v1/analytics/facade/query", `not-json`)
	h.FacadeQuery(c7)
	assert.Equal(t, http.StatusBadRequest, rec7.Code)
}

func newFacadeContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	return ctx, rec
}
