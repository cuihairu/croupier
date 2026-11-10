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
// Analytics 门面（MOB-ANA-001，docs/design/game-analytics-requirements.md §4）：
//
// 统一查询入口 + 每域独立接口。现有分域 handler/路由保留不动（加法不改造）；
// facade 只做两件事：
//  1. Summary：一次拉全部域摘要卡片（移动端 dashboard 一次请求），逐域错误
//     隔离——单域失败标 unavailable，其余域照常返回；未建数据源的域标
//     pending_data_source（诚实声明，不伪造数字）。
//  2. Query：AnalyticsQuery{domain, metrics, filter, timeRange} 按 domain
//     路由到对应域接口（供移动端分域详情页复用一个入口）。
//
// 新域接入 = 实现一个 Queryer 接口 + 在 Summary/Query 各加一行注册，门面
// 结构不变。
// ============================================================================

// Facade domain keys（移动端卡片/路由共用同一命名）。
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

// PendingDomains 是设计文档 §3 判定为 ❌ 缺、数据源待建的域：facade 声明
// 路由但如实报 pending_data_source，绝不返回合成数据。
var PendingDomains = []string{"economy", "social", "stability", "anticheat", "campaign"}

// 每域独立查询接口：*Service 隐式实现；facade 依赖接口不依赖实现。
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

// Facade 聚合各域查询接口（nil 域 = 该域卡片 unavailable，装配缺欠不炸门面）。
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

// NewFacade 以生产 *Service 装配门面（它隐式实现全部 Queryer）。
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

// DomainCard 是一个域的摘要卡片单元。
type DomainCard struct {
	Domain string      `json:"domain"`
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// FacadeSummaryResponse 是 GET /analytics/facade/summary 的聚合 payload。
type FacadeSummaryResponse struct {
	GameId      string        `json:"gameId"`
	Env         string        `json:"env"`
	GeneratedAt string        `json:"generatedAt"`
	Domains     []*DomainCard `json:"domains"`
}

// Summary 并发拉取全部已建域的摘要（逐域错误隔离），未建域声明
// pending_data_source。任一域失败不影响其它域（部分可用语义，移动端
// dashboard 不因单域后端故障整体白屏）。
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
			// 装配缺欠显式标 unavailable，不让卡片静默消失。
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

// summary* 闭包：queryer 未装配时返回 nil fn（query 据此标 unavailable，
// 卡片不静默消失）。
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

// FacadeTimeRange 是 AnalyticsQuery 的时间窗（域请求的 startDate/endDate 同源）。
type FacadeTimeRange struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// AnalyticsQuery 是统一查询入口的 wire 形态（契约键 lowerCamelCase）：
// domain 定位域接口，metrics 预留指标裁剪（当前各域返回规范形态，不裁剪），
// filter 携带 gameId/env 与域内过滤，timeRange 映射 startDate/endDate。
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

// isPendingDomain 报告该域是否为数据源待建（❌ 域）。
func isPendingDomain(domain string) bool {
	for _, d := range PendingDomains {
		if d == domain {
			return true
		}
	}
	return false
}

// Query 按 domain 路由到对应域接口。未知域/未建域 422（客户端输入语义，
// 非服务端故障）。
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

// nil 域守卫：装配缺欠返回 unavailable 错误而非 panic。
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

// ---- nil 域的 fail-closed 占位实现（调用即报装配缺欠） ----

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

// FacadeSummary GET /analytics/facade/summary?gameId=&env=（移动端 dashboard
// 一次拉全部域摘要卡片）。
func (h *Handler) FacadeSummary(c *gin.Context) {
	gameId := strings.TrimSpace(c.Query("gameId"))
	if gameId == "" {
		response.Error(c, errorx.NewValidationErrorWithDetails("gameId is required", nil))
		return
	}
	resp := h.facade.Summary(c.Request.Context(), gameId, strings.TrimSpace(c.Query("env")))
	response.Success(c, resp)
}

// FacadeQueryRequest POST /analytics/facade/query 入参。
type FacadeQueryRequest struct {
	AnalyticsQuery
}

// FacadeQuery POST /analytics/facade/query（统一查询路由，移动端分域详情页）。
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
