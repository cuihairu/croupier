package powerops

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---- test doubles ----

type fakeGate struct {
	mu        sync.Mutex
	approvals map[string]bool // approvalID → approved
	created   []string        // request ids
	// failCreate makes CreateApproval return an error.
	failCreate error
	// failLookup makes Approved return an error (fail-closed path).
	failLookup error
}

func (g *fakeGate) CreateApproval(ctx context.Context, a Action, requestID, requestedBy, reason string, n int) (string, error) {
	if g.failCreate != nil {
		return "", g.failCreate
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.created = append(g.created, requestID)
	id := "apr-" + requestID
	g.approvals[id] = false
	return id, nil
}

func (g *fakeGate) Approved(ctx context.Context, approvalID string) (bool, error) {
	if g.failLookup != nil {
		return false, g.failLookup
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	ok, exist := g.approvals[approvalID]
	if !exist {
		return false, nil
	}
	return ok, nil
}

func (g *fakeGate) approve(approvalID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.approvals[approvalID] = true
}

type mapResolver struct{ m map[string]Machine }

func (r mapResolver) Resolve(nodeID string) (Machine, bool) {
	m, ok := r.m[nodeID]
	return m, ok
}

type scriptedProvider struct {
	mu       sync.Mutex
	statuses []string // returned in order, last repeats
	calls    []string // status calls in order
	failOn   map[Action]error
	// failNode restricts failOn to one node (partial-failure scenarios);
	// empty = fail every node.
	failNode string
	powerOff func(force bool) error
}

func (p *scriptedProvider) fails(m Machine, a Action) bool {
	if p.failOn == nil || p.failOn[a] == nil {
		return false
	}
	return p.failNode == "" || p.failNode == m.NodeID
}

func (p *scriptedProvider) Kind() string { return ProviderIPMI }

func (p *scriptedProvider) nextStatus() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, "status")
	if len(p.statuses) == 0 {
		return "on"
	}
	st := p.statuses[0]
	if len(p.statuses) > 1 {
		p.statuses = p.statuses[1:]
	}
	return st
}

func (p *scriptedProvider) PowerOn(ctx context.Context, m Machine) (string, error) {
	if p.fails(m, ActionPowerOn) {
		return "", p.failOn[ActionPowerOn]
	}
	return "on sent", nil
}

func (p *scriptedProvider) PowerOff(ctx context.Context, m Machine, force bool) (string, error) {
	if p.powerOff != nil {
		if err := p.powerOff(force); err != nil {
			return "", err
		}
	}
	return "off sent", nil
}

func (p *scriptedProvider) Reboot(ctx context.Context, m Machine) (string, error) {
	if p.failOn != nil && p.failOn[ActionReboot] != nil {
		return "", p.failOn[ActionReboot]
	}
	return "reboot sent", nil
}

func (p *scriptedProvider) Status(ctx context.Context, m Machine) (string, error) {
	return p.nextStatus(), nil
}

// ---- harness ----

func newTestStore(t *testing.T) *GormStore {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1) // :memory: 单连接（test 基建坑）
	require.NoError(t, db.AutoMigrate(&model.PowerRequest{}, &model.PowerRequestTarget{}))
	return NewGormStore(db)
}

type harness struct {
	orch     *Orchestrator
	store    *GormStore
	gate     *fakeGate
	provider *scriptedProvider
}

func newHarness(t *testing.T, statuses []string) *harness {
	t.Helper()
	store := newTestStore(t)
	gate := &fakeGate{approvals: map[string]bool{}}
	prov := &scriptedProvider{statuses: statuses}
	orch := &Orchestrator{
		Store:          store,
		Approval:       gate,
		Providers:      map[string]Provider{ProviderIPMI: prov},
		VerifyTimeout:  400 * time.Millisecond,
		VerifyInterval: 30 * time.Millisecond,
	}
	return &harness{orch: orch, store: store, gate: gate, provider: prov}
}

func resolver() mapResolver {
	return mapResolver{m: map[string]Machine{
		"node-1":  {NodeID: "node-1", Provider: ProviderIPMI, BmcAddr: "10.0.0.7"},
		"node-2":  {NodeID: "node-2", Provider: ProviderIPMI, BmcAddr: "10.0.0.8"},
		"agent-1": {NodeID: "agent-1", Provider: ProviderAgent, AgentID: "ag-1"},
		"wol-1":   {NodeID: "wol-1", Provider: ProviderWOL, MAC: "aa:bb:cc:dd:ee:01"},
	}}
}

func (h *harness) submit(t *testing.T, action Action, nodes ...string) *model.PowerRequest {
	t.Helper()
	req, err := h.orch.Submit(context.Background(), resolver(), SubmitInput{
		NodeIDs: nodes, Action: action, Reason: "test", RequestedBy: "op1",
	})
	require.NoError(t, err)
	return req
}

// awaitTerminal polls the store until the request reaches a terminal state.
func awaitTerminal(t *testing.T, store *GormStore, requestID string) *model.PowerRequest {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		req, err := store.Get(requestID)
		require.NoError(t, err)
		switch req.State {
		case StateDone, StatePartial, StateFailed, StateRejected:
			return req
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("request never reached a terminal state")
	return nil
}

// ---- tests ----

func TestSubmitCreatesPendingRequestWithApprovalLink(t *testing.T) {
	h := newHarness(t, []string{"on"})
	req := h.submit(t, ActionPowerOn, "node-1", "node-2")

	assert.Equal(t, StatePending, req.State)
	assert.NotEmpty(t, req.ApprovalID)
	assert.Equal(t, 2, req.Total)
	assert.True(t, strings.HasPrefix(req.RequestID, "pw-"))
	require.Len(t, h.gate.created, 1)
	assert.Equal(t, req.RequestID, h.gate.created[0])

	// Targets snapshot keeps the approved scope immutable.
	machines, err := decodeTargets(req.TargetsJSON)
	require.NoError(t, err)
	require.Len(t, machines, 2)
	assert.Equal(t, "node-1", machines[0].NodeID)
}

func TestSubmitValidationRejectsBeforePersistence(t *testing.T) {
	h := newHarness(t, []string{"on"})
	ctx := context.Background()

	// Unknown machine.
	_, err := h.orch.Submit(ctx, resolver(), SubmitInput{NodeIDs: []string{"ghost"}, Action: ActionPowerOn})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")

	// Unsupported (machine, action) pair: agent machine force_off is refused.
	_, err = h.orch.Submit(ctx, resolver(), SubmitInput{NodeIDs: []string{"agent-1"}, Action: ActionForceOff})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support")

	// WOL cannot power_off.
	_, err = h.orch.Submit(ctx, resolver(), SubmitInput{NodeIDs: []string{"wol-1"}, Action: ActionPowerOff})
	require.Error(t, err)

	// Empty targets / empty action.
	_, err = h.orch.Submit(ctx, resolver(), SubmitInput{Action: ActionReboot})
	require.Error(t, err)
	_, err = h.orch.Submit(ctx, resolver(), SubmitInput{NodeIDs: []string{"node-1"}})
	require.Error(t, err)

	// Nothing persisted on any rejection path.
	rows, err := h.store.List(10)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestSubmitApprovalGateFailureLeavesPendingWithoutLink(t *testing.T) {
	h := newHarness(t, []string{"on"})
	h.gate.failCreate = errors.New("store down")
	_, err := h.orch.Submit(context.Background(), resolver(), SubmitInput{NodeIDs: []string{"node-1"}, Action: ActionPowerOn})
	require.Error(t, err)
	rows, _ := h.store.List(10)
	require.Len(t, rows, 1)
	assert.Equal(t, StatePending, rows[0].State)
	assert.Empty(t, rows[0].ApprovalID)
	// Resume without an approval link refuses.
	_, err = h.orch.ResumeAsync(context.Background(), rows[0].RequestID)
	assert.ErrorIs(t, err, ErrApprovalLinkMissing)
}

func TestResumeRequiresApprovedApproval(t *testing.T) {
	h := newHarness(t, []string{"on"})
	req := h.submit(t, ActionPowerOn, "node-1")

	// Unknown id.
	_, err := h.orch.ResumeAsync(context.Background(), "pw-missing")
	assert.ErrorIs(t, err, ErrRequestNotFound)

	// Not yet approved.
	_, err = h.orch.ResumeAsync(context.Background(), req.RequestID)
	assert.ErrorIs(t, err, ErrRequestNotApproved)

	// Fail-closed: approval lookup errors refuse execution.
	h.gate.failLookup = errors.New("db down")
	h.gate.approve(req.ApprovalID)
	_, err = h.orch.ResumeAsync(context.Background(), req.RequestID)
	assert.ErrorIs(t, err, ErrRequestNotApproved)
	h.gate.failLookup = nil

	// Approved → executes to done.
	h.gate.approve(req.ApprovalID)
	_, err = h.orch.ResumeAsync(context.Background(), req.RequestID)
	require.NoError(t, err)
	final := awaitTerminal(t, h.store, req.RequestID)
	assert.Equal(t, StateDone, final.State)
	assert.Equal(t, 1, final.Succeeded)

	// Already-terminal resume refuses (not pending).
	_, err = h.orch.ResumeAsync(context.Background(), req.RequestID)
	assert.ErrorIs(t, err, ErrRequestNotPending)
}

func TestRejectPathExecutesNothing(t *testing.T) {
	h := newHarness(t, []string{"off"})
	req := h.submit(t, ActionPowerOn, "node-1")

	final, err := h.orch.Reject(context.Background(), req.RequestID)
	require.NoError(t, err)
	assert.Equal(t, StateRejected, final.State)

	targets, _ := h.store.Targets(req.RequestID)
	assert.Empty(t, targets, "rejected request must never execute actions")
	assert.Len(t, h.provider.calls, 0, "no provider status calls on reject path")

	// Double reject / resume after reject both refuse.
	_, err = h.orch.Reject(context.Background(), req.RequestID)
	assert.ErrorIs(t, err, ErrRequestNotPending)
	_, err = h.orch.ResumeAsync(context.Background(), req.RequestID)
	assert.ErrorIs(t, err, ErrRequestNotPending)
}

func TestPartialFailureAggregation(t *testing.T) {
	h := newHarness(t, []string{"on"})
	h.provider.failOn = map[Action]error{ActionPowerOn: errors.New("bmc unreachable")}
	h.provider.failNode = "node-1"
	req := h.submit(t, ActionPowerOn, "node-1", "node-2")
	h.gate.approve(req.ApprovalID)
	_, err := h.orch.ResumeAsync(context.Background(), req.RequestID)
	require.NoError(t, err)

	final := awaitTerminal(t, h.store, req.RequestID)
	assert.Equal(t, StatePartial, final.State)
	assert.Equal(t, 2, final.Total)
	assert.Equal(t, 1, final.Succeeded)
	assert.Equal(t, 1, final.Failed)

	targets, _ := h.store.Targets(req.RequestID)
	require.Len(t, targets, 2)
	var failedNode string
	var okNode string
	for _, tgt := range targets {
		if tgt.Success {
			okNode = tgt.NodeID
			assert.True(t, tgt.Verified)
		} else {
			failedNode = tgt.NodeID
			assert.Contains(t, tgt.ErrorMsg, "bmc unreachable")
		}
	}
	assert.Equal(t, "node-1", failedNode)
	assert.Equal(t, "node-2", okNode)
}

func TestAllActionsFailedLandsFailed(t *testing.T) {
	h := newHarness(t, []string{"on"})
	h.provider.powerOff = func(force bool) error { return errors.New("bmc timeout") }
	req := h.submit(t, ActionForceOff, "node-1")
	h.gate.approve(req.ApprovalID)
	_, err := h.orch.ResumeAsync(context.Background(), req.RequestID)
	require.NoError(t, err)
	final := awaitTerminal(t, h.store, req.RequestID)
	assert.Equal(t, StateFailed, final.State)
	assert.Equal(t, 1, final.Failed)
}

func TestVerifyLoopConfirmsDelayedPowerOn(t *testing.T) {
	// Status says off twice (booting), then on — verify must keep polling
	// until confirmed instead of declaring done immediately.
	h := newHarness(t, []string{"off", "off", "on"})
	req := h.submit(t, ActionPowerOn, "node-1")
	h.gate.approve(req.ApprovalID)
	_, err := h.orch.ResumeAsync(context.Background(), req.RequestID)
	require.NoError(t, err)
	final := awaitTerminal(t, h.store, req.RequestID)
	assert.Equal(t, StateDone, final.State)
	assert.True(t, len(h.provider.calls) >= 2, "verify must poll more than once, calls=%d", len(h.provider.calls))
}

func TestVerifyTimeoutKeepsUnverifiedHonest(t *testing.T) {
	// Status never reaches "off": the force-off action ran, but the machine
	// did not confirm — terminal is failed, target stays unverified.
	h := newHarness(t, []string{"on"})
	req := h.submit(t, ActionForceOff, "node-1")
	h.gate.approve(req.ApprovalID)
	_, err := h.orch.ResumeAsync(context.Background(), req.RequestID)
	require.NoError(t, err)
	final := awaitTerminal(t, h.store, req.RequestID)
	assert.Equal(t, StateFailed, final.State)
	targets, _ := h.store.Targets(req.RequestID)
	require.Len(t, targets, 1)
	assert.True(t, targets[0].Success, "the action itself did run")
	assert.False(t, targets[0].Verified, "unconfirmed machines stay unverified")
}

func TestWOLWakeIsSuccessButNeverVerified(t *testing.T) {
	store := newTestStore(t)
	gate := &fakeGate{approvals: map[string]bool{}}
	wol := NewWOLProvider()
	wol.Send = func(ctx context.Context, addr string, packet []byte) error { return nil }
	orch := &Orchestrator{
		Store: store, Approval: gate,
		Providers:      map[string]Provider{ProviderWOL: wol},
		VerifyTimeout:  100 * time.Millisecond,
		VerifyInterval: 20 * time.Millisecond,
	}
	req, err := orch.Submit(context.Background(), resolver(), SubmitInput{
		NodeIDs: []string{"wol-1"}, Action: ActionPowerOn, RequestedBy: "op1",
	})
	require.NoError(t, err)
	gate.approve(req.ApprovalID)
	_, err = orch.ResumeAsync(context.Background(), req.RequestID)
	require.NoError(t, err)
	final := awaitTerminal(t, store, req.RequestID)
	// WOL is unverifiable by design: the packet sent counts as success, the
	// machine stays unverified, and the terminal state is done (design §3).
	assert.Equal(t, StateDone, final.State)
	targets, _ := store.Targets(req.RequestID)
	require.Len(t, targets, 1)
	assert.True(t, targets[0].Success)
	assert.False(t, targets[0].Verified)
}

func TestListLimitClamps(t *testing.T) {
	h := newHarness(t, []string{"on"})
	for i := 0; i < 3; i++ {
		h.submit(t, ActionPowerOn, "node-1")
	}
	rows, err := h.store.List(2)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
}
