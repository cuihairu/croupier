package analytics

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DATA-030: the facade summary gives every analytics domain its own time
// budget. A domain blocking past its budget reports timeout while the other
// domains still return in time; caller cancellation stops the wait; and the
// number of concurrently running domain summaries is capped.

type slowRealtime struct{ delay time.Duration }

func (s slowRealtime) Realtime(context.Context, *RealtimeRequest) (*RealtimeResponse, error) {
	time.Sleep(s.delay) // deliberately ignores ctx: only the budget can release the wait
	return &RealtimeResponse{RealtimeMetrics: RealtimeMetrics{OnlineUsers: 5}}, nil
}

type slowWarehouse struct{ delay time.Duration }

func (s slowWarehouse) WarehouseDAU(context.Context, *WarehouseDAURequest) (*WarehouseDAUResponse, error) {
	time.Sleep(s.delay)
	return &WarehouseDAUResponse{Points: []WarehouseDAUPoint{{Date: "2026-11-26"}}}, nil
}

func TestFacadeSummaryTimeoutMarksOnlySlowDomain(t *testing.T) {
	// Mutates the shared budget var: never parallel.
	oldBudget := facadeDomainBudget
	facadeDomainBudget = 60 * time.Millisecond
	defer func() { facadeDomainBudget = oldBudget }()

	f := allOK()
	f.Realtime = slowRealtime{delay: 400 * time.Millisecond}

	start := time.Now()
	resp := f.Summary(context.Background(), "demo", "prod")
	elapsed := time.Since(start)

	// The wait releases on the budget, long before the slow domain finishes.
	require.Less(t, elapsed, 300*time.Millisecond, "summary must return on the budget, not on the slow domain")

	// The slow domain carries the recognizable timeout status.
	rt := cardOf(resp, DomainRealtime)
	require.NotNil(t, rt)
	assert.Equal(t, CardStatusTimeout, rt.Status)
	assert.Contains(t, rt.Error, "budget")
	assert.Nil(t, rt.Data)

	// Fast domains are unaffected and every card is still present.
	assert.Equal(t, CardStatusOK, cardOf(resp, DomainOverview).Status)
	assert.Equal(t, CardStatusOK, cardOf(resp, DomainPayments).Status)
	assert.Equal(t, CardStatusOK, cardOf(resp, DomainWarehouse).Status)
	require.Len(t, resp.Domains, 13)
	for _, domain := range PendingDomains {
		assert.Equal(t, CardStatusPendingData, cardOf(resp, domain).Status, domain)
	}
}

func TestFacadeSummaryCallerCancelStopsTheWait(t *testing.T) {
	f := allOK()
	f.Warehouse = slowWarehouse{delay: 500 * time.Millisecond}

	// Pre-cancelled caller context: every domain wait releases immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	resp := f.Summary(ctx, "demo", "prod")
	require.Less(t, time.Since(start), 400*time.Millisecond, "a cancelled caller must not wait for domains")
	for _, domain := range []string{DomainOverview, DomainBehavior, DomainPayments, DomainRetention, DomainRealtime, DomainInvocations, DomainLevels, DomainWarehouse} {
		card := cardOf(resp, domain)
		require.NotNil(t, card, domain)
		assert.Equal(t, CardStatusUnavailable, card.Status, domain)
		assert.Contains(t, card.Error, "caller context ended", domain)
	}
	for _, domain := range PendingDomains {
		assert.Equal(t, CardStatusPendingData, cardOf(resp, domain).Status, domain)
	}

	// Cancellation mid-flight: the summary returns while the slow domain is
	// still blocked, and the abandoned domain is reported, not dropped.
	ctx2, cancel2 := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel2()
	}()
	start = time.Now()
	resp2 := f.Summary(ctx2, "demo", "prod")
	require.Less(t, time.Since(start), 400*time.Millisecond, "mid-flight cancellation must stop the wait")

	wh := cardOf(resp2, DomainWarehouse)
	require.NotNil(t, wh)
	assert.Equal(t, CardStatusUnavailable, wh.Status)
	assert.Contains(t, wh.Error, "caller context ended")

	// A fast domain that finished before the cancel is still ok.
	assert.Equal(t, CardStatusOK, cardOf(resp2, DomainOverview).Status)
}

type countingDomain struct {
	inFlight int32
	maxSeen  int32
	delay    time.Duration
}

func (c *countingDomain) enter() {
	n := atomic.AddInt32(&c.inFlight, 1)
	for {
		m := atomic.LoadInt32(&c.maxSeen)
		if n <= m || atomic.CompareAndSwapInt32(&c.maxSeen, m, n) {
			break
		}
	}
	time.Sleep(c.delay)
	atomic.AddInt32(&c.inFlight, -1)
}

func (c *countingDomain) Overview(context.Context, *OverviewRequest) (*OverviewResponse, error) {
	c.enter()
	return &OverviewResponse{}, nil
}

func (c *countingDomain) Behavior(context.Context, *BehaviorRequest) (*BehaviorResponse, error) {
	c.enter()
	return &BehaviorResponse{}, nil
}

func (c *countingDomain) PaymentsSummary(context.Context, *PaymentsSummaryRequest) (*PaymentsSummaryResponse, error) {
	c.enter()
	return &PaymentsSummaryResponse{}, nil
}

func (c *countingDomain) Retention(context.Context, *RetentionRequest) (*RetentionResponse, error) {
	c.enter()
	return &RetentionResponse{}, nil
}

func (c *countingDomain) Realtime(context.Context, *RealtimeRequest) (*RealtimeResponse, error) {
	c.enter()
	return &RealtimeResponse{}, nil
}

func (c *countingDomain) InvocationsSummary(context.Context, *InvocationsSummaryRequest) (*InvocationsSummaryResponse, error) {
	c.enter()
	return &InvocationsSummaryResponse{}, nil
}

func (c *countingDomain) Levels(context.Context, *LevelsRequest) (*LevelsResponse, error) {
	c.enter()
	return &LevelsResponse{}, nil
}

func (c *countingDomain) WarehouseDAU(context.Context, *WarehouseDAURequest) (*WarehouseDAUResponse, error) {
	c.enter()
	return &WarehouseDAUResponse{}, nil
}

func TestFacadeSummaryConcurrencyCap(t *testing.T) {
	// Mutates the shared concurrency var: never parallel.
	oldCap := facadeSummaryConcurrency
	facadeSummaryConcurrency = 3
	defer func() { facadeSummaryConcurrency = oldCap }()

	domain := &countingDomain{delay: 15 * time.Millisecond}
	f := &Facade{
		Overview:    domain,
		Behavior:    domain,
		Payments:    domain,
		Retention:   domain,
		Realtime:    domain,
		Invocations: domain,
		Levels:      domain,
		Warehouse:   domain,
	}

	resp := f.Summary(context.Background(), "demo", "prod")

	// All domains still returned.
	require.Len(t, resp.Domains, 13)
	assert.Equal(t, CardStatusOK, cardOf(resp, DomainOverview).Status)
	assert.Equal(t, CardStatusOK, cardOf(resp, DomainWarehouse).Status)

	// In-flight domain summaries never exceeded the cap, but did run
	// concurrently (a strictly serial pool would make maxSeen 1).
	maxSeen := atomic.LoadInt32(&domain.maxSeen)
	require.LessOrEqual(t, maxSeen, int32(3), "domain summaries must respect the concurrency cap")
	assert.GreaterOrEqual(t, maxSeen, int32(2), "the pool must actually run domains concurrently")
}
