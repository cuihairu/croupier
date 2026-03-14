package analytics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

	h := NewHandler(NewService(&svc.ServiceContext{}))
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
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("expected status=500, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestOverviewUsesQueryBindingOnGet(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
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

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newAnalyticsTestContext(http.MethodGet, "/api/v1/analytics/realtime?env=prod", "")
	h.Realtime(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status=400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "GameId") {
		t.Fatalf("expected missing GameId error, got %s", rec.Body.String())
	}
}
