package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DATA-029: facade query parameters carry explicit semantics. Every domain
// declares the metrics/filters/time window it consumes (facadeDomainSpecs);
// Query applies the supported ones, narrows the payload on metrics, and
// answers anything outside the contract with a field-level diagnostic instead
// of silently ignoring it.

func TestFacadeQueryMetricsNarrowsCanonicalPayload(t *testing.T) {
	t.Parallel()

	f := allOK()
	ctx := context.Background()
	base := AnalyticsQuery{Domain: DomainOverview, Filter: map[string]string{"gameId": "demo", "env": "prod"}}

	// No metrics: the canonical typed payload comes back whole.
	data, err := f.Query(ctx, &base)
	require.NoError(t, err)
	_, ok := data.(*OverviewResponse)
	require.True(t, ok)

	// metrics picks the requested top-level keys; keys the domain did not
	// produce (peakOnline here) are simply absent from the narrowed payload.
	q := base
	q.Metrics = []string{"metrics", "peakOnline"}
	data, err = f.Query(ctx, &q)
	require.NoError(t, err)
	narrowed, ok := data.(map[string]interface{})
	require.True(t, ok)
	require.Contains(t, narrowed, "metrics")
	require.NotContains(t, narrowed, "peakOnline")
	metrics, ok := narrowed["metrics"].(map[string]interface{})
	require.True(t, ok)
	assert.InEpsilon(t, 42.0, metrics["dau"], 1e-9)
	assert.InEpsilon(t, 7.0, metrics["mau"], 1e-9)
	assert.InEpsilon(t, 3.0, metrics["newUsers"], 1e-9)

	// behavior narrows to topActions only.
	qb := AnalyticsQuery{Domain: DomainBehavior, Filter: map[string]string{"gameId": "demo"}, Metrics: []string{"topActions"}}
	data, err = f.Query(ctx, &qb)
	require.NoError(t, err)
	narrowed, ok = data.(map[string]interface{})
	require.True(t, ok)
	require.Len(t, narrowed, 1)
	require.Contains(t, narrowed, "topActions")
}

func TestFacadeQueryDiagnosticsForUnsupportedParams(t *testing.T) {
	t.Parallel()

	f := allOK()
	ctx := context.Background()

	// Unknown metric key -> 422 with a metrics detail naming the offender and
	// the supported vocabulary.
	q := &AnalyticsQuery{Domain: DomainOverview, Filter: map[string]string{"gameId": "demo"}, Metrics: []string{"nope", "trends"}}
	_, err := f.Query(ctx, q)
	require.Error(t, err)
	details := validationDetails(t, err)
	require.Contains(t, details, "metrics")
	// "nope" is enumerated as unsupported; the supported vocabulary follows.
	assert.Contains(t, details["metrics"], "unsupported for overview: nope; supported: metrics, trends, peakOnline")

	// Filter key the domain does not consume (cohort is retention-only).
	q2 := &AnalyticsQuery{Domain: DomainOverview, Filter: map[string]string{"gameId": "demo", "cohort": "2026-11-01"}}
	_, err = f.Query(ctx, q2)
	require.Error(t, err)
	details = validationDetails(t, err)
	require.Contains(t, details, "filter")
	assert.Contains(t, details["filter"], "cohort")

	// All violations land in one payload, each under its field key.
	q3 := &AnalyticsQuery{
		Domain:    DomainRealtime,
		Filter:    map[string]string{"gameId": "demo", "days": "7"},
		Metrics:   []string{"points"},
		TimeRange: FacadeTimeRange{StartDate: "2026-11-01", EndDate: "2026-11-02"},
	}
	_, err = f.Query(ctx, q3)
	require.Error(t, err)
	details = validationDetails(t, err)
	assert.Contains(t, details, "metrics")
	assert.Contains(t, details, "filter")
	require.Contains(t, details, "timeRange")
	assert.Contains(t, details["timeRange"], DomainRealtime)

	// Retention still accepts its declared cohort filter.
	data, err := f.Query(ctx, &AnalyticsQuery{Domain: DomainRetention, Filter: map[string]string{"gameId": "demo", "cohort": "2026-11-01"}})
	require.NoError(t, err)
	_, ok := data.(*RetentionResponse)
	require.True(t, ok)
}

func TestFacadeQueryFixedWindowDomainsRejectTimeRange(t *testing.T) {
	t.Parallel()

	f := allOK()
	ctx := context.Background()
	tr := FacadeTimeRange{StartDate: "2026-11-01", EndDate: "2026-11-02"}

	for _, domain := range []string{DomainRealtime, DomainWarehouse} {
		q := &AnalyticsQuery{Domain: domain, Filter: map[string]string{"gameId": "demo"}, TimeRange: tr}
		_, err := f.Query(ctx, q)
		require.Error(t, err, domain)
		details := validationDetails(t, err)
		require.Contains(t, details, "timeRange", domain)
	}

	// An empty timeRange keeps passing (the mobile client omits empty keys).
	data, err := f.Query(ctx, &AnalyticsQuery{Domain: DomainRealtime, Filter: map[string]string{"gameId": "demo"}, TimeRange: FacadeTimeRange{StartDate: "", EndDate: ""}})
	require.NoError(t, err)
	_, ok := data.(*RealtimeResponse)
	require.True(t, ok)
}

func TestFacadeQueryInvocationsMapsTimeRangeToLookbackHours(t *testing.T) {
	t.Parallel()

	f := allOK()
	ctx := context.Background()
	inv := f.Invocations.(*stubInvocations)

	// Different windows map to different lookback spans.
	_, err := f.Query(ctx, &AnalyticsQuery{Domain: DomainInvocations, Filter: map[string]string{"gameId": "demo"}, TimeRange: FacadeTimeRange{StartDate: "2026-11-01", EndDate: "2026-11-08"}})
	require.NoError(t, err)
	assert.Equal(t, 24*7, inv.hours)

	_, err = f.Query(ctx, &AnalyticsQuery{Domain: DomainInvocations, Filter: map[string]string{"gameId": "demo"}, TimeRange: FacadeTimeRange{StartDate: "2026-11-01", EndDate: "2026-11-02"}})
	require.NoError(t, err)
	assert.Equal(t, 24, inv.hours)

	// No timeRange: Hours stays 0 so the summary keeps its default window.
	inv.hours = 0
	_, err = f.Query(ctx, &AnalyticsQuery{Domain: DomainInvocations, Filter: map[string]string{"gameId": "demo"}})
	require.NoError(t, err)
	assert.Equal(t, 0, inv.hours)

	// A window beyond the domain's 30-day cap clamps to the cap instead of
	// letting the summary silently snap back to its default 24h.
	_, err = f.Query(ctx, &AnalyticsQuery{Domain: DomainInvocations, Filter: map[string]string{"gameId": "demo"}, TimeRange: FacadeTimeRange{StartDate: "2026-01-01", EndDate: "2026-12-31"}})
	require.NoError(t, err)
	assert.Equal(t, 24*30, inv.hours)
}

func TestFacadeQueryInvocationsLookbackValidation(t *testing.T) {
	t.Parallel()

	f := allOK()
	ctx := context.Background()
	base := AnalyticsQuery{Domain: DomainInvocations, Filter: map[string]string{"gameId": "demo"}}

	// One bound missing.
	q := base
	q.TimeRange = FacadeTimeRange{StartDate: "2026-11-01"}
	_, err := f.Query(ctx, &q)
	require.Error(t, err)
	assert.Contains(t, validationDetails(t, err), "timeRange")

	// Unparseable bound.
	q = base
	q.TimeRange = FacadeTimeRange{StartDate: "2026-11-01", EndDate: "soon"}
	_, err = f.Query(ctx, &q)
	require.Error(t, err)

	// end not after start.
	q = base
	q.TimeRange = FacadeTimeRange{StartDate: "2026-11-02", EndDate: "2026-11-01"}
	_, err = f.Query(ctx, &q)
	require.Error(t, err)
	details := validationDetails(t, err)
	require.Contains(t, details, "timeRange")
	assert.Contains(t, details["timeRange"], "after")
}

func TestFacadeQueryDifferentWindowsReturnDifferentResults(t *testing.T) {
	db := setupServiceTestDB(t)
	svcCtx := setupFullServiceTestContext(t, db)
	f := NewFacade(NewService(svcCtx))
	ctx := context.Background()

	at := func(day, hour int) time.Time {
		return time.Date(2026, 3, day, hour, 0, 30, 0, time.UTC)
	}
	seed := func(eventType, userID string, at time.Time) {
		ev := createTestEventFull(eventType, userID, "game1", "prod", at, nil)
		require.NoError(t, svcCtx.BehaviorModel.RecordEvent(ctx, &ev))
	}
	// Window A (03-10): three logins; window B (03-11): one purchase.
	seed("login", "u1", at(10, 9))
	seed("login", "u2", at(10, 12))
	seed("login", "u3", at(10, 15))
	seed("purchase", "u9", at(11, 10))

	query := func(day string) map[string]interface{} {
		data, err := f.Query(ctx, &AnalyticsQuery{
			Domain:    DomainBehavior,
			Filter:    map[string]string{"gameId": "game1", "env": "prod"},
			TimeRange: FacadeTimeRange{StartDate: day, EndDate: day},
		})
		require.NoError(t, err)
		raw, err := json.Marshal(data)
		require.NoError(t, err)
		var payload map[string]interface{}
		require.NoError(t, json.Unmarshal(raw, &payload))
		return payload
	}

	windowA := query("2026-03-10")
	windowB := query("2026-03-11")

	// The facade applies the requested window: each response only contains
	// its own day's actions.
	actionsA, err := json.Marshal(windowA["topActions"])
	require.NoError(t, err)
	assert.JSONEq(t, `[{"label":"login","value":3}]`, string(actionsA))
	actionsB, err := json.Marshal(windowB["topActions"])
	require.NoError(t, err)
	assert.JSONEq(t, `[{"label":"purchase","value":1}]`, string(actionsB))
}

func TestFacadeHandlerSurfacesParameterDiagnostics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(nil), config.SSEConfig{})
	h.facade = allOK()

	// Unsupported metric -> 422 validation_failed with a metrics detail.
	body := `{"domain":"overview","filter":{"gameId":"demo"},"metrics":["nope"]}`
	c, rec := newFacadeContext(http.MethodPost, "/api/v1/analytics/facade/query", body)
	h.FacadeQuery(c)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	payload := rec.Body.String()
	assert.Contains(t, payload, "validation_failed")
	assert.Contains(t, payload, `"metrics"`)

	// metrics narrowing reaches the wire: only the requested key survives.
	body = `{"domain":"overview","filter":{"gameId":"demo","env":"prod"},"metrics":["metrics"]}`
	c2, rec2 := newFacadeContext(http.MethodPost, "/api/v1/analytics/facade/query", body)
	h.FacadeQuery(c2)
	require.Equal(t, http.StatusOK, rec2.Code)
	narrowed := rec2.Body.String()
	assert.Contains(t, narrowed, `"dau":42`)
	assert.NotContains(t, narrowed, "trends")

	// Fixed-window domain with a timeRange -> 422 with a timeRange detail.
	body = `{"domain":"realtime","filter":{"gameId":"demo"},"timeRange":{"startDate":"2026-11-01","endDate":"2026-11-02"}}`
	c3, rec3 := newFacadeContext(http.MethodPost, "/api/v1/analytics/facade/query", body)
	h.FacadeQuery(c3)
	assert.Equal(t, http.StatusUnprocessableEntity, rec3.Code)
	assert.Contains(t, rec3.Body.String(), `"timeRange"`)
}

// validationDetails extracts the field-keyed details from a validation error.
func validationDetails(t *testing.T, err error) map[string]string {
	t.Helper()
	require.Error(t, err)
	var ve *errorx.ValidationError
	require.True(t, errors.As(err, &ve), "expected a validation error, got: %v", err)
	return ve.Details
}
