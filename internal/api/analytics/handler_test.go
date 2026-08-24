package analytics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

func newAnalyticsTestContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	return ctx, rec
}

func TestBindAnalyticsRequestUsesQueryForGet(t *testing.T) {
	t.Parallel()

	ctx, _ := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/overview?gameId=tower&env=prod", "")
	var req OverviewRequest
	if err := bindAnalyticsRequest(ctx, &req); err != nil {
		t.Fatalf("bindAnalyticsRequest() error = %v", err)
	}
	if req.GameId != "tower" {
		t.Fatalf("expected gameId=tower, got %q", req.GameId)
	}
	if req.Env != "prod" {
		t.Fatalf("expected env=prod, got %q", req.Env)
	}
}

func TestBindAnalyticsRequestUsesJSONForPost(t *testing.T) {
	t.Parallel()

	ctx, _ := newAnalyticsTestContext(http.MethodPost, "/api/v1/analytics/filters", `{"gameId":"tower","filters":{"env":"prod"}}`)
	var req FiltersUpdateRequest
	if err := bindAnalyticsRequest(ctx, &req); err != nil {
		t.Fatalf("bindAnalyticsRequest() error = %v", err)
	}
	if req.GameId != "tower" {
		t.Fatalf("expected gameId=tower, got %q", req.GameId)
	}
}

func TestAnalyticsHandlersRejectMalformedJSON(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	cases := []struct {
		name string
		fn   func(*gin.Context)
	}{
		{name: "Behavior", fn: h.Behavior},
		{name: "BehaviorEvents", fn: h.BehaviorEvents},
		{name: "BehaviorAdoption", fn: h.BehaviorAdoption},
		{name: "BehaviorAdoptionBreakdown", fn: h.BehaviorAdoptionBreakdown},
		{name: "BehaviorFunnel", fn: h.BehaviorFunnel},
		{name: "BehaviorPaths", fn: h.BehaviorPaths},
		{name: "Overview", fn: h.Overview},
		{name: "RealtimeSeries", fn: h.RealtimeSeries},
		{name: "Ingest", fn: h.Ingest},
		{name: "FiltersGet", fn: h.FiltersGet},
		{name: "FiltersUpdate", fn: h.FiltersUpdate},
		{name: "Payments", fn: h.Payments},
		{name: "PaymentsIngest", fn: h.PaymentsIngest},
		{name: "PaymentsProductTrend", fn: h.PaymentsProductTrend},
		{name: "PaymentsSummary", fn: h.PaymentsSummary},
		{name: "PaymentsTransactions", fn: h.PaymentsTransactions},
		{name: "Retention", fn: h.Retention},
		{name: "Levels", fn: h.Levels},
		{name: "LevelsEpisodes", fn: h.LevelsEpisodes},
		{name: "LevelsMaps", fn: h.LevelsMaps},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, rec := newAnalyticsTestContext(http.MethodPost, "/api/v1/analytics", "{")
			tc.fn(ctx)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status=400, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestOverviewUsesQueryBindingOnGet(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/overview?gameId=tower&env=prod", "")
	h.Overview(ctx)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status=500, got %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "validation") {
		t.Fatalf("expected query binding to succeed before service failure, got %s", rec.Body.String())
	}
}

func TestRealtimeRejectsMissingRequiredQueryOnGet(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/realtime?env=prod", "")
	h.Realtime(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status=400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "GameId") {
		t.Fatalf("expected missing GameId error, got %s", rec.Body.String())
	}
}

// Additional handler tests for coverage

func TestNewHandler(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service, config.SSEConfig{})

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	if handler.service != service {
		t.Fatal("expected service to be set")
	}
}

func TestBehaviorHandler_BindingSuccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodPost, "/api/v1/analytics/behavior", `{"gameId":"tower","env":"prod"}`)
	h.Behavior(ctx)

	// Service will fail due to empty context, but binding should succeed
	if rec.Code == http.StatusBadRequest {
		t.Fatalf("binding should succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBehaviorEventsHandler_GETBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/behavior/events?gameId=tower&env=prod", "")
	h.BehaviorEvents(ctx)

	// Should not be a bad request (binding succeeded)
	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBehaviorAdoptionHandler_GETBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/behavior/adoption?gameId=tower", "")
	h.BehaviorAdoption(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBehaviorFunnelHandler_POSTBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodPost, "/api/v1/analytics/behavior/funnel", `{"gameId":"tower","steps":["login"]}`)
	h.BehaviorFunnel(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBehaviorPathsHandler_POSTBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodPost, "/api/v1/analytics/behavior/paths", `{"gameId":"tower","eventType":"login"}`)
	h.BehaviorPaths(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOverviewHandler_GETBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/overview?gameId=tower&env=prod", "")
	h.Overview(ctx)

	// Binding succeeded if we get past validation
	if rec.Code == http.StatusBadRequest && !strings.Contains(rec.Body.String(), "validation") {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRealtimeSeriesHandler_GETBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/realtime/series?gameId=tower", "")
	h.RealtimeSeries(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestIngestHandler_JSONBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodPost, "/api/v1/analytics/ingest", `{"gameId":"tower","events":[]}`)
	h.Ingest(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFiltersGetHandler_GETBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/filters?gameId=tower", "")
	h.FiltersGet(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFiltersUpdateHandler_JSONBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodPost, "/api/v1/analytics/filters", `{"gameId":"tower","filters":{}}`)
	h.FiltersUpdate(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPaymentsHandler_GETBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/payments?gameId=tower&env=prod", "")
	h.Payments(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPaymentsIngestHandler_JSONBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodPost, "/api/v1/analytics/payments/ingest", `{"gameId":"tower","transactions":[]}`)
	h.PaymentsIngest(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPaymentsProductTrendHandler_GETBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/payments/product-trend?gameId=tower", "")
	h.PaymentsProductTrend(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPaymentsSummaryHandler_GETBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/payments/summary?gameId=tower", "")
	h.PaymentsSummary(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPaymentsTransactionsHandler_GETBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/payments/transactions?gameId=tower&page=1", "")
	h.PaymentsTransactions(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRetentionHandler_GETBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/retention?gameId=tower&env=prod", "")
	h.Retention(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestLevelsHandler_GETBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/levels?gameId=tower", "")
	h.Levels(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestLevelsEpisodesHandler_GETBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/levels/episodes?gameId=tower&levelId=1", "")
	h.LevelsEpisodes(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestLevelsMapsHandler_GETBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/levels/maps?gameId=tower&levelId=1", "")
	h.LevelsMaps(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// Additional tests for low-coverage handlers

func TestBehaviorAdoptionBreakdownHandler_POSTBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodPost, "/api/v1/analytics/behavior/adoption-breakdown", `{"gameId":"tower","env":"prod","feature":"login_reward"}`)
	h.BehaviorAdoptionBreakdown(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBehaviorAdoptionBreakdownHandler_GETBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/behavior/adoption-breakdown?gameId=tower&env=prod&feature=login_reward", "")
	h.BehaviorAdoptionBreakdown(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBehaviorFunnelHandler_GETBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/behavior/funnel?gameId=tower&steps=login&steps=pay", "")
	h.BehaviorFunnel(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPaymentsIngestHandler_POSTBinding(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodPost, "/api/v1/analytics/payments/ingest", `{"gameId":"tower","transactions":[]}`)
	h.PaymentsIngest(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPaymentsIngestHandler_InvalidJSON(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodPost, "/api/v1/analytics/payments/ingest", "{invalid")
	h.PaymentsIngest(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", rec.Code)
	}
}

func TestRealtimeHandler_MissingGameId(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/realtime?env=prod", "")
	h.Realtime(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing gameId, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRealtimeHandler_EmptyGameId(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/realtime?gameId=", "")
	h.Realtime(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty gameId, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// Additional tests for coverage

func TestBehaviorHandler_WithAllParams(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodPost, "/api/v1/analytics/behavior", `{"gameId":"tower","env":"prod","startDate":"2024-01-01","endDate":"2024-01-31"}`)
	h.Behavior(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d", rec.Code)
	}
}

func TestBehaviorEventsHandler_WithAllParams(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/behavior/events?gameId=tower&env=prod&startDate=2024-01-01", "")
	h.BehaviorEvents(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d", rec.Code)
	}
}

func TestBehaviorPathsHandler_WithAllParams(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodPost, "/api/v1/analytics/behavior/paths", `{"gameId":"tower","eventType":"level_complete"}`)
	h.BehaviorPaths(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d", rec.Code)
	}
}

func TestIngestHandler_WithMultipleEvents(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodPost, "/api/v1/analytics/ingest", `{"gameId":"tower","events":[{"eventName":"test"},{"eventName":"test2"}]}`)
	h.Ingest(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d", rec.Code)
	}
}

func TestPaymentsHandler_WithDateRange(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/payments?gameId=tower&startDate=2024-01-01&endDate=2024-01-31", "")
	h.Payments(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d", rec.Code)
	}
}

func TestPaymentsSummaryHandler_WithAllParams(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/payments/summary?gameId=tower&env=prod&startDate=2024-01-01", "")
	h.PaymentsSummary(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d", rec.Code)
	}
}

func TestRetentionHandler_WithAllParams(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/retention?gameId=tower&env=prod&firstOpenDate=2024-01-01", "")
	h.Retention(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d", rec.Code)
	}
}

func TestLevelsHandler_WithAllParams(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/levels?gameId=tower&env=prod", "")
	h.Levels(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d", rec.Code)
	}
}

func TestLevelsEpisodesHandler_WithAllParams(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/levels/episodes?gameId=tower&levelId=1&env=prod", "")
	h.LevelsEpisodes(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d", rec.Code)
	}
}

func TestLevelsMapsHandler_WithAllParams(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/levels/maps?gameId=tower&levelId=1&episodeId=1", "")
	h.LevelsMaps(ctx)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected binding to succeed, got %d", rec.Code)
	}
}
