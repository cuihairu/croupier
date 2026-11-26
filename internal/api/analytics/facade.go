package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

// ============================================================================
// Analytics facade (MOB-ANA-001, docs/design/game-analytics-requirements.md §4):
//
// One unified query entry point plus a per-domain interface. The existing
// per-domain handlers and routes are left untouched (additive, not a refactor);
// the facade does exactly two things:
//  1. Summary: pulls every domain's summary card in one request (the mobile
//     dashboard), with per-domain error isolation -- a failing domain is marked
//     unavailable while the others still return; domains whose data source is not
//     built are marked pending_data_source (an honest declaration, no fabricated
//     numbers).
//  2. Query: AnalyticsQuery{domain, metrics, filter, timeRange} is routed to the
//     matching domain interface by domain (a single entry point for the mobile
//     per-domain detail pages).
//
// Onboarding a new domain = implementing one Queryer interface plus one
// registration line in each of Summary/Query; the facade structure is unchanged.
// ============================================================================

// Facade domain keys (the mobile cards and routes share this same naming).
const (
	DomainOverview    = "overview"
	DomainBehavior    = "behavior"
	DomainPayments    = "payments"
	DomainRetention   = "retention"
	DomainRealtime    = "realtime"
	DomainInvocations = "invocations"
	DomainLevels      = "levels"
	DomainWarehouse   = "warehouse"
)

// Domain card statuses.
const (
	CardStatusOK          = "ok"
	CardStatusUnavailable = "unavailable"
	CardStatusPendingData = "pending_data_source"
)

// PendingDomains lists the domains judged ❌ (missing) in design doc §3, whose data
// sources are not built yet: the facade declares the route but honestly reports
// pending_data_source and never returns synthetic data.
var PendingDomains = []string{"economy", "social", "stability", "anticheat", "campaign"}

// Per-domain query interfaces: *Service implements them implicitly; the facade
// depends on the interfaces, not on the implementations.
type (
	OverviewQueryer interface {
		Overview(ctx context.Context, req *OverviewRequest) (*OverviewResponse, error)
	}
	BehaviorQueryer interface {
		Behavior(ctx context.Context, req *BehaviorRequest) (*BehaviorResponse, error)
	}
	PaymentsQueryer interface {
		PaymentsSummary(ctx context.Context, req *PaymentsSummaryRequest) (*PaymentsSummaryResponse, error)
	}
	RetentionQueryer interface {
		Retention(ctx context.Context, req *RetentionRequest) (*RetentionResponse, error)
	}
	RealtimeQueryer interface {
		Realtime(ctx context.Context, req *RealtimeRequest) (*RealtimeResponse, error)
	}
	InvocationsQueryer interface {
		InvocationsSummary(ctx context.Context, req *InvocationsSummaryRequest) (*InvocationsSummaryResponse, error)
	}
	LevelsQueryer interface {
		Levels(ctx context.Context, req *LevelsRequest) (*LevelsResponse, error)
	}
	WarehouseQueryer interface {
		WarehouseDAU(ctx context.Context, req *WarehouseDAURequest) (*WarehouseDAUResponse, error)
	}
)

// Facade aggregates the per-domain query interfaces (a nil domain = that domain's
// card is unavailable; a wiring gap does not blow up the facade).
type Facade struct {
	Overview    OverviewQueryer
	Behavior    BehaviorQueryer
	Payments    PaymentsQueryer
	Retention   RetentionQueryer
	Realtime    RealtimeQueryer
	Invocations InvocationsQueryer
	Levels      LevelsQueryer
	Warehouse   WarehouseQueryer
}

// NewFacade wires the facade with the production *Service (it implements every Queryer implicitly).
func NewFacade(service *Service) *Facade {
	return &Facade{
		Overview:    service,
		Behavior:    service,
		Payments:    service,
		Retention:   service,
		Realtime:    service,
		Invocations: service,
		Levels:      service,
		Warehouse:   service,
	}
}

// DomainCard is the summary card unit of one domain.
type DomainCard struct {
	Domain string      `json:"domain"`
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// FacadeSummaryResponse is the aggregated payload of GET /analytics/facade/summary.
type FacadeSummaryResponse struct {
	GameId      string        `json:"gameId"`
	Env         string        `json:"env"`
	GeneratedAt string        `json:"generatedAt"`
	Domains     []*DomainCard `json:"domains"`
}

// Summary concurrently pulls summaries for every wired domain (per-domain error
// isolation) and declares pending_data_source for unwired domains. A failure in any
// one domain does not affect the others (partial-availability semantics, so the
// mobile dashboard does not go blank because of a single domain's backend failure).
func (f *Facade) Summary(ctx context.Context, gameId, env string) *FacadeSummaryResponse {
	cards := make([]*DomainCard, 0, 8+1)
	var mu sync.Mutex
	var wg sync.WaitGroup

	appendCard := func(card *DomainCard) {
		mu.Lock()
		cards = append(cards, card)
		mu.Unlock()
	}
	query := func(domain string, fn func() (interface{}, error)) {
		defer wg.Done()
		if fn == nil {
			// A wiring gap is explicitly marked unavailable so cards never silently disappear.
			appendCard(&DomainCard{Domain: domain, Status: CardStatusUnavailable, Error: "facade: domain not wired"})
			return
		}
		data, err := fn()
		if err != nil {
			appendCard(&DomainCard{Domain: domain, Status: CardStatusUnavailable, Error: err.Error()})
			return
		}
		appendCard(&DomainCard{Domain: domain, Status: CardStatusOK, Data: data})
	}

	type entry struct {
		domain string
		run    func() (interface{}, error)
	}
	entries := make([]entry, 0, 8)
	entries = append(entries, entry{DomainOverview, f.summaryOverview(ctx, gameId, env)})
	entries = append(entries, entry{DomainBehavior, f.summaryBehavior(ctx, gameId, env)})
	entries = append(entries, entry{DomainPayments, f.summaryPayments(ctx, gameId, env)})
	entries = append(entries, entry{DomainRetention, f.summaryRetention(ctx, gameId, env)})
	entries = append(entries, entry{DomainRealtime, f.summaryRealtime(ctx, gameId, env)})
	entries = append(entries, entry{DomainInvocations, f.summaryInvocations(ctx, gameId, env)})
	entries = append(entries, entry{DomainLevels, f.summaryLevels(ctx, gameId, env)})
	entries = append(entries, entry{DomainWarehouse, f.summaryWarehouse(ctx, gameId, env)})

	for _, e := range entries {
		wg.Add(1)
		go query(e.domain, e.run)
	}
	for _, domain := range PendingDomains {
		appendCard(&DomainCard{Domain: domain, Status: CardStatusPendingData})
	}
	wg.Wait()

	return &FacadeSummaryResponse{
		GameId:      gameId,
		Env:         env,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Domains:     cards,
	}
}

// summary* closures: return a nil fn when the queryer is not wired (query then marks it
// unavailable, so the card never silently disappears).
func (f *Facade) summaryOverview(ctx context.Context, gameId, env string) func() (interface{}, error) {
	if f.Overview == nil {
		return nil
	}
	return func() (interface{}, error) {
		return f.Overview.Overview(ctx, &OverviewRequest{GameId: gameId, Env: env})
	}
}

func (f *Facade) summaryBehavior(ctx context.Context, gameId, env string) func() (interface{}, error) {
	if f.Behavior == nil {
		return nil
	}
	return func() (interface{}, error) {
		return f.Behavior.Behavior(ctx, &BehaviorRequest{GameId: gameId, Env: env})
	}
}

func (f *Facade) summaryPayments(ctx context.Context, gameId, env string) func() (interface{}, error) {
	if f.Payments == nil {
		return nil
	}
	return func() (interface{}, error) {
		return f.Payments.PaymentsSummary(ctx, &PaymentsSummaryRequest{GameId: gameId, Env: env})
	}
}

func (f *Facade) summaryRetention(ctx context.Context, gameId, env string) func() (interface{}, error) {
	if f.Retention == nil {
		return nil
	}
	return func() (interface{}, error) {
		return f.Retention.Retention(ctx, &RetentionRequest{GameId: gameId, Env: env})
	}
}

func (f *Facade) summaryRealtime(ctx context.Context, gameId, env string) func() (interface{}, error) {
	if f.Realtime == nil {
		return nil
	}
	return func() (interface{}, error) {
		return f.Realtime.Realtime(ctx, &RealtimeRequest{GameId: gameId, Env: env})
	}
}

func (f *Facade) summaryInvocations(ctx context.Context, gameId, env string) func() (interface{}, error) {
	if f.Invocations == nil {
		return nil
	}
	return func() (interface{}, error) {
		return f.Invocations.InvocationsSummary(ctx, &InvocationsSummaryRequest{GameId: gameId, Env: env})
	}
}

func (f *Facade) summaryLevels(ctx context.Context, gameId, env string) func() (interface{}, error) {
	if f.Levels == nil {
		return nil
	}
	return func() (interface{}, error) {
		return f.Levels.Levels(ctx, &LevelsRequest{GameId: gameId, Env: env})
	}
}

func (f *Facade) summaryWarehouse(ctx context.Context, gameId, env string) func() (interface{}, error) {
	if f.Warehouse == nil {
		return nil
	}
	return func() (interface{}, error) {
		return f.Warehouse.WarehouseDAU(ctx, &WarehouseDAURequest{GameId: gameId, Env: env})
	}
}

// FacadeTimeRange is the time window of AnalyticsQuery (same source as the domain
// requests' startDate/endDate).
type FacadeTimeRange struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// AnalyticsQuery is the wire shape of the unified query entry point (contract keys are
// lowerCamelCase). Every parameter has an explicit per-domain contract (DATA-029,
// declared in facadeDomainSpecs): domain locates the domain interface; metrics
// narrows the response to a subset of the domain's declared top-level keys
// (absent/empty returns the canonical shape, unknown keys get a field-level
// diagnostic); filter carries gameId/env plus the in-domain filters the domain
// declares; timeRange maps to startDate/endDate for date-range domains, to an
// hour lookback for invocations, and is rejected for fixed-window domains
// (realtime, warehouse). Cross-domain metric-caliber unification remains DATA-006.
type AnalyticsQuery struct {
	Domain    string            `json:"domain"`
	Metrics   []string          `json:"metrics"`
	Filter    map[string]string `json:"filter"`
	TimeRange FacadeTimeRange   `json:"timeRange"`
}

func (q *AnalyticsQuery) gameId() string {
	if q.Filter == nil {
		return ""
	}
	return strings.TrimSpace(q.Filter["gameId"])
}

func (q *AnalyticsQuery) env() string {
	if q.Filter == nil {
		return ""
	}
	return strings.TrimSpace(q.Filter["env"])
}

func (q *AnalyticsQuery) filter(key string) string {
	if q.Filter == nil {
		return ""
	}
	return strings.TrimSpace(q.Filter[key])
}

// ============================================================================
// Per-domain parameter contract (DATA-029): every domain declares the query
// parameters it actually consumes. Query applies the supported ones and
// answers anything outside the contract with a field-level diagnostic (422,
// details keyed by the offending field) instead of silently ignoring it.
// ============================================================================

// facadeTimeWindow classifies how a domain consumes timeRange.
type facadeTimeWindow string

const (
	// timeWindowDateRange: startDate/endDate are consumed as the query window.
	timeWindowDateRange facadeTimeWindow = "date_range"
	// timeWindowFixed: the domain resolves its own window (the realtime 15m
	// window, warehouse filter.days); a non-empty timeRange is rejected.
	timeWindowFixed facadeTimeWindow = "fixed"
	// timeWindowLookback: startDate/endDate are mapped onto the domain's
	// lookback span (invocations Hours).
	timeWindowLookback facadeTimeWindow = "lookback"
)

// facadeDomainSpec declares one domain's parameter surface. Metric keys are the
// top-level keys of the domain's canonical response: metrics narrows the
// payload to that subset, absent/empty metrics returns the whole payload.
type facadeDomainSpec struct {
	metrics   []string
	filters   []string
	timeRange facadeTimeWindow
}

var facadeDomainSpecs = map[string]facadeDomainSpec{
	DomainOverview:    {metrics: []string{"metrics", "trends", "peakOnline"}, filters: []string{"gameId", "env"}, timeRange: timeWindowDateRange},
	DomainBehavior:    {metrics: []string{"topActions", "userFlows", "heatMap", "sample"}, filters: []string{"gameId", "env"}, timeRange: timeWindowDateRange},
	DomainPayments:    {metrics: []string{"items"}, filters: []string{"gameId", "env"}, timeRange: timeWindowDateRange},
	DomainRetention:   {metrics: []string{"cohorts"}, filters: []string{"gameId", "env", "cohort"}, timeRange: timeWindowDateRange},
	DomainRealtime:    {metrics: []string{"realtimeMetrics", "source", "timestamp"}, filters: []string{"gameId", "env"}, timeRange: timeWindowFixed},
	DomainInvocations: {metrics: []string{"total", "failed", "successRate", "avgDurationMs", "p95DurationMs", "topFunctions"}, filters: []string{"gameId", "env"}, timeRange: timeWindowLookback},
	DomainLevels:      {metrics: []string{"levels", "sample"}, filters: []string{"gameId", "env"}, timeRange: timeWindowDateRange},
	DomainWarehouse:   {metrics: []string{"points"}, filters: []string{"gameId", "env", "days"}, timeRange: timeWindowFixed},
}

func (s facadeDomainSpec) hasMetric(key string) bool {
	for _, m := range s.metrics {
		if m == key {
			return true
		}
	}
	return false
}

func (s facadeDomainSpec) hasFilter(key string) bool {
	for _, f := range s.filters {
		if f == key {
			return true
		}
	}
	return false
}

// validateAgainst checks the query against one domain's declared contract: every
// violation lands in details keyed by the offending field, so a caller sees all
// diagnostics for its input in one round trip.
func (q *AnalyticsQuery) validateAgainst(spec facadeDomainSpec, domain string) error {
	details := map[string]string{}
	var unknownMetrics []string
	for _, m := range q.Metrics {
		if !spec.hasMetric(strings.TrimSpace(m)) {
			unknownMetrics = append(unknownMetrics, strings.TrimSpace(m))
		}
	}
	if len(unknownMetrics) > 0 {
		details["metrics"] = fmt.Sprintf("unsupported for %s: %s; supported: %s", domain, strings.Join(unknownMetrics, ", "), strings.Join(spec.metrics, ", "))
	}
	var unknownFilters []string
	for key := range q.Filter {
		if !spec.hasFilter(key) {
			unknownFilters = append(unknownFilters, key)
		}
	}
	if len(unknownFilters) > 0 {
		sort.Strings(unknownFilters)
		details["filter"] = fmt.Sprintf("unsupported for %s: %s; supported: %s", domain, strings.Join(unknownFilters, ", "), strings.Join(spec.filters, ", "))
	}
	if spec.timeRange == timeWindowFixed &&
		(strings.TrimSpace(q.TimeRange.StartDate) != "" || strings.TrimSpace(q.TimeRange.EndDate) != "") {
		details["timeRange"] = fmt.Sprintf("%s resolves its own window; timeRange is not consumed", domain)
	}
	if len(details) == 0 {
		return nil
	}
	return errorx.NewValidationErrorWithDetails("query parameters outside the domain contract", details)
}

// facadeTimeLayout is the date-only layout facade timeRange values use (the
// same convention as the domain requests' startDate/endDate).
const facadeTimeLayout = "2006-01-02"

// maxFacadeLookbackHours mirrors the invocation summary's 30-day cap; clamping
// here keeps a too-wide window at the maximum lookback instead of letting the
// domain silently snap it back to its default 24h.
const maxFacadeLookbackHours = 24 * 30

// lookbackHours maps timeRange onto an hour count for lookback domains
// (invocations): both bounds are required in YYYY-MM-DD, the span rounds up to
// whole hours, and anything beyond the domain's cap clamps to the cap.
func (q *AnalyticsQuery) lookbackHours() (int, error) {
	start, errStart := time.Parse(facadeTimeLayout, strings.TrimSpace(q.TimeRange.StartDate))
	end, errEnd := time.Parse(facadeTimeLayout, strings.TrimSpace(q.TimeRange.EndDate))
	if errStart != nil || errEnd != nil {
		return 0, errorx.NewValidationErrorWithDetails("lookback timeRange needs startDate and endDate as YYYY-MM-DD",
			map[string]string{"timeRange": "expected YYYY-MM-DD for both bounds"})
	}
	if !end.After(start) {
		return 0, errorx.NewValidationErrorWithDetails("timeRange endDate must be after startDate",
			map[string]string{"timeRange": "endDate must be after startDate"})
	}
	hours := int(math.Ceil(end.Sub(start).Hours()))
	if hours > maxFacadeLookbackHours {
		hours = maxFacadeLookbackHours
	}
	return hours, nil
}

// narrowResponse prunes a canonical domain payload to the requested metric keys
// (already validated against the domain spec). The JSON round-trip keeps the
// wire shape of every kept key identical to the un-narrowed payload.
func narrowResponse(payload interface{}, metrics []string) (interface{}, error) {
	if payload == nil {
		return nil, nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var full map[string]interface{}
	if err := json.Unmarshal(raw, &full); err != nil {
		return nil, err
	}
	narrowed := make(map[string]interface{}, len(metrics))
	for _, key := range metrics {
		name := strings.TrimSpace(key)
		if v, ok := full[name]; ok {
			narrowed[name] = v
		}
	}
	return narrowed, nil
}

// isPendingDomain reports whether the domain is one whose data source is not built yet (a ❌ domain).
func isPendingDomain(domain string) bool {
	for _, d := range PendingDomains {
		if d == domain {
			return true
		}
	}
	return false
}

// Query routes to the matching domain interface by domain. Unknown or unwired domains
// give 422 (client-input semantics, not a server-side failure). Parameters are first
// checked against the domain's declared contract (facadeDomainSpecs, DATA-029):
// supported ones are applied, unsupported ones get a field-level diagnostic.
func (f *Facade) Query(ctx context.Context, q *AnalyticsQuery) (interface{}, error) {
	if q == nil || strings.TrimSpace(q.Domain) == "" {
		return nil, errorx.NewValidationErrorWithDetails("domain is required", nil)
	}
	gameId := q.gameId()
	if gameId == "" {
		return nil, errorx.NewValidationErrorWithDetails("filter.gameId is required", nil)
	}
	env := q.env()
	domain := strings.TrimSpace(q.Domain)
	tr := q.TimeRange
	if spec, known := facadeDomainSpecs[domain]; known {
		if err := q.validateAgainst(spec, domain); err != nil {
			return nil, err
		}
	}
	var (
		data interface{}
		err  error
	)
	switch domain {
	case DomainOverview:
		data, err = f.overviewQueryer().Overview(ctx, &OverviewRequest{GameId: gameId, Env: env, StartDate: tr.StartDate, EndDate: tr.EndDate})
	case DomainBehavior:
		data, err = f.behaviorQueryer().Behavior(ctx, &BehaviorRequest{GameId: gameId, Env: env, StartDate: tr.StartDate, EndDate: tr.EndDate})
	case DomainPayments:
		data, err = f.paymentsQueryer().PaymentsSummary(ctx, &PaymentsSummaryRequest{GameId: gameId, Env: env, StartDate: tr.StartDate, EndDate: tr.EndDate})
	case DomainRetention:
		data, err = f.retentionQueryer().Retention(ctx, &RetentionRequest{GameId: gameId, Env: env, Cohort: q.filter("cohort"), StartDate: tr.StartDate, EndDate: tr.EndDate})
	case DomainRealtime:
		data, err = f.realtimeQueryer().Realtime(ctx, &RealtimeRequest{GameId: gameId, Env: env})
	case DomainInvocations:
		hours := 0
		if strings.TrimSpace(tr.StartDate) != "" || strings.TrimSpace(tr.EndDate) != "" {
			hours, err = q.lookbackHours()
			if err != nil {
				return nil, err
			}
		}
		data, err = f.invocationsQueryer().InvocationsSummary(ctx, &InvocationsSummaryRequest{GameId: gameId, Env: env, Hours: hours})
	case DomainLevels:
		data, err = f.levelsQueryer().Levels(ctx, &LevelsRequest{GameId: gameId, Env: env, StartDate: tr.StartDate, EndDate: tr.EndDate})
	case DomainWarehouse:
		days := 0
		if v := q.filter("days"); v != "" {
			days = atoiSafe(v)
		}
		data, err = f.warehouseQueryer().WarehouseDAU(ctx, &WarehouseDAURequest{GameId: gameId, Env: env, Days: days})
	default:
		if isPendingDomain(domain) {
			return nil, &errorx.CodeError{Code: http.StatusUnprocessableEntity, Message: "数据源待建：" + domain, StableCode: "domain_pending"}
		}
		return nil, errorx.NewValidationErrorWithDetails("unknown domain: "+domain, nil)
	}
	if err != nil {
		return nil, err
	}
	// metrics narrows the canonical payload to the requested top-level keys
	// (pre-validated against the domain spec); absent/empty returns it whole.
	if len(q.Metrics) > 0 {
		return narrowResponse(data, q.Metrics)
	}
	return data, nil
}

// Nil-domain guard: a wiring gap returns an unavailable error instead of panicking.
func (f *Facade) overviewQueryer() OverviewQueryer {
	if f.Overview == nil {
		return unavailableOverview{}
	}
	return f.Overview
}

func (f *Facade) behaviorQueryer() BehaviorQueryer {
	if f.Behavior == nil {
		return unavailableBehavior{}
	}
	return f.Behavior
}

func (f *Facade) paymentsQueryer() PaymentsQueryer {
	if f.Payments == nil {
		return unavailablePayments{}
	}
	return f.Payments
}

func (f *Facade) retentionQueryer() RetentionQueryer {
	if f.Retention == nil {
		return unavailableRetention{}
	}
	return f.Retention
}

func (f *Facade) realtimeQueryer() RealtimeQueryer {
	if f.Realtime == nil {
		return unavailableRealtime{}
	}
	return f.Realtime
}

func (f *Facade) invocationsQueryer() InvocationsQueryer {
	if f.Invocations == nil {
		return unavailableInvocations{}
	}
	return f.Invocations
}

func (f *Facade) levelsQueryer() LevelsQueryer {
	if f.Levels == nil {
		return unavailableLevels{}
	}
	return f.Levels
}

func (f *Facade) warehouseQueryer() WarehouseQueryer {
	if f.Warehouse == nil {
		return unavailableWarehouse{}
	}
	return f.Warehouse
}

func atoiSafe(v string) int {
	n := 0
	for _, r := range v {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// ---- fail-closed placeholder implementations for nil domains (calling them reports the wiring gap) ----

type unavailableError struct{}

func (unavailableError) Error() string { return "facade: domain not wired" }

type unavailableOverview struct{}

func (unavailableOverview) Overview(context.Context, *OverviewRequest) (*OverviewResponse, error) {
	return nil, unavailableError{}
}

type unavailableBehavior struct{}

func (unavailableBehavior) Behavior(context.Context, *BehaviorRequest) (*BehaviorResponse, error) {
	return nil, unavailableError{}
}

type unavailablePayments struct{}

func (unavailablePayments) PaymentsSummary(context.Context, *PaymentsSummaryRequest) (*PaymentsSummaryResponse, error) {
	return nil, unavailableError{}
}

type unavailableRetention struct{}

func (unavailableRetention) Retention(context.Context, *RetentionRequest) (*RetentionResponse, error) {
	return nil, unavailableError{}
}

type unavailableRealtime struct{}

func (unavailableRealtime) Realtime(context.Context, *RealtimeRequest) (*RealtimeResponse, error) {
	return nil, unavailableError{}
}

type unavailableInvocations struct{}

func (unavailableInvocations) InvocationsSummary(context.Context, *InvocationsSummaryRequest) (*InvocationsSummaryResponse, error) {
	return nil, unavailableError{}
}

type unavailableLevels struct{}

func (unavailableLevels) Levels(context.Context, *LevelsRequest) (*LevelsResponse, error) {
	return nil, unavailableError{}
}

type unavailableWarehouse struct{}

func (unavailableWarehouse) WarehouseDAU(context.Context, *WarehouseDAURequest) (*WarehouseDAUResponse, error) {
	return nil, unavailableError{}
}

// ---- Handler ----

// FacadeSummary handles GET /analytics/facade/summary?gameId=&env= (the mobile
// dashboard pulls every domain's summary card in one request).
func (h *Handler) FacadeSummary(c *gin.Context) {
	gameId := strings.TrimSpace(c.Query("gameId"))
	if gameId == "" {
		response.Error(c, errorx.NewValidationErrorWithDetails("gameId is required", nil))
		return
	}
	resp := h.facade.Summary(c.Request.Context(), gameId, strings.TrimSpace(c.Query("env")))
	response.Success(c, resp)
}

// FacadeQueryRequest is the input of POST /analytics/facade/query.
type FacadeQueryRequest struct {
	AnalyticsQuery
}

// FacadeQuery handles POST /analytics/facade/query (the unified query route for the mobile per-domain detail pages).
func (h *Handler) FacadeQuery(c *gin.Context) {
	var req FacadeQueryRequest
	if err := bindAnalyticsRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	data, err := h.facade.Query(c.Request.Context(), &req.AnalyticsQuery)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"domain": strings.TrimSpace(req.Domain), "data": data})
}
