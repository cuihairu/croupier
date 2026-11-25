package analytics

import (
	"context"
	"net/http"
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
// lowerCamelCase): domain locates the domain interface, metrics reserves metric
// narrowing (today every domain returns its canonical shape, no narrowing), filter
// carries gameId/env plus in-domain filters, and timeRange maps to startDate/endDate.
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
// give 422 (client-input semantics, not a server-side failure).
func (f *Facade) Query(ctx context.Context, q *AnalyticsQuery) (interface{}, error) {
	if q == nil || strings.TrimSpace(q.Domain) == "" {
		return nil, errorx.NewValidationErrorWithDetails("domain is required", nil)
	}
	gameId := q.gameId()
	if gameId == "" {
		return nil, errorx.NewValidationErrorWithDetails("filter.gameId is required", nil)
	}
	env := q.env()
	tr := q.TimeRange
	switch strings.TrimSpace(q.Domain) {
	case DomainOverview:
		return f.overviewQueryer().Overview(ctx, &OverviewRequest{GameId: gameId, Env: env, StartDate: tr.StartDate, EndDate: tr.EndDate})
	case DomainBehavior:
		return f.behaviorQueryer().Behavior(ctx, &BehaviorRequest{GameId: gameId, Env: env, StartDate: tr.StartDate, EndDate: tr.EndDate})
	case DomainPayments:
		return f.paymentsQueryer().PaymentsSummary(ctx, &PaymentsSummaryRequest{GameId: gameId, Env: env, StartDate: tr.StartDate, EndDate: tr.EndDate})
	case DomainRetention:
		return f.retentionQueryer().Retention(ctx, &RetentionRequest{GameId: gameId, Env: env, Cohort: q.filter("cohort"), StartDate: tr.StartDate, EndDate: tr.EndDate})
	case DomainRealtime:
		return f.realtimeQueryer().Realtime(ctx, &RealtimeRequest{GameId: gameId, Env: env})
	case DomainInvocations:
		return f.invocationsQueryer().InvocationsSummary(ctx, &InvocationsSummaryRequest{GameId: gameId, Env: env})
	case DomainLevels:
		return f.levelsQueryer().Levels(ctx, &LevelsRequest{GameId: gameId, Env: env, StartDate: tr.StartDate, EndDate: tr.EndDate})
	case DomainWarehouse:
		days := 0
		if v := q.filter("days"); v != "" {
			days = atoiSafe(v)
		}
		return f.warehouseQueryer().WarehouseDAU(ctx, &WarehouseDAURequest{GameId: gameId, Env: env, Days: days})
	default:
		if isPendingDomain(strings.TrimSpace(q.Domain)) {
			return nil, &errorx.CodeError{Code: http.StatusUnprocessableEntity, Message: "数据源待建：" + q.Domain, StableCode: "domain_pending"}
		}
		return nil, errorx.NewValidationErrorWithDetails("unknown domain: "+q.Domain, nil)
	}
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
