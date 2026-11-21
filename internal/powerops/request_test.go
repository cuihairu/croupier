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
	// actionCalls records every mutating provider invocation (PWR-001:
	// the double-claim regression asserts this count is strictly 1).
	actionCalls []string
	// hangStatus makes Status never return — the PWR-002 "永不返回的
	// Runner" regression shape for the verification loop.
	hangStatus bool
	// hangActionNode makes the action on one node never return — the PWR-003
	// mid-batch crash shape: the row stays attempting until recovery converges.
	hangActionNode string
	failOn         map[Action]error
	// failNode restricts failOn to one node (partial-failure scenarios);
	// empty = fail every node.
	failNode string
	powerOff func(force bool) error
}

// record appends a mutating-action invocation (thread-safe).
func (p *scriptedProvider) record(a string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.actionCalls = append(p.actionCalls, a)
}

// actionCallCount returns how many mutating provider calls were made.
func (p *scriptedProvider) actionCallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.actionCalls)
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
	p.record("power_on")
	if p.hangActionNode != "" && p.hangActionNode == m.NodeID {
		<-make(chan struct{}) // PWR-003 mid-batch crash shape
	}
	if p.fails(m, ActionPowerOn) {
		return "", p.failOn[ActionPowerOn]
	}
	return "on sent", nil
}

func (p *scriptedProvider) PowerOff(ctx context.Context, m Machine, force bool) (string, error) {
	p.record("power_off")
	if p.hangActionNode != "" && p.hangActionNode == m.NodeID {
		<-make(chan struct{}) // PWR-003 mid-batch crash shape
	}
	if p.powerOff != nil {
		if err := p.powerOff(force); err != nil {
			return "", err
		}
	}
	return "off sent", nil
}

func (p *scriptedProvider) Reboot(ctx context.Context, m Machine) (string, error) {
	p.record("reboot")
	if p.hangActionNode != "" && p.hangActionNode == m.NodeID {
		<-make(chan struct{}) // PWR-003 mid-batch crash shape
	}
	if p.failOn != nil && p.failOn[ActionReboot] != nil {
		return "", p.failOn[ActionReboot]
	}
	return "reboot sent", nil
}

func (p *scriptedProvider) Status(ctx context.Context, m Machine) (string, error) {
	if p.hangStatus {
		<-make(chan struct{}) // block forever (PWR-002 hung Runner)
	}
	return p.nextStatus(), nil
}

// hungProvider never returns from any provider call — the PWR-002 regression
// shape proving a hung action cannot hold the request open.
type hungProvider struct{ kind string }

func (p *hungProvider) Kind() string { return p.kind }
func (p *hungProvider) PowerOn(ctx context.Context, m Machine) (string, error) {
	<-make(chan struct{})
	return "", nil
}
func (p *hungProvider) PowerOff(ctx context.Context, m Machine, force bool) (string, error) {
	<-make(chan struct{})
	return "", nil
}
func (p *hungProvider) Reboot(ctx context.Context, m Machine) (string, error) {
	<-make(chan struct{})
	return "", nil
}
func (p *hungProvider) Status(ctx context.Context, m Machine) (string, error) {
	<-make(chan struct{})
	return "", nil
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
		"node-3":  {NodeID: "node-3", Provider: ProviderIPMI, BmcAddr: "10.0.0.9"},
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

// PWR-001：并发 resume 只有一个人能领取 pending→executing（CAS），副作用
// 调用次数严格为 1；输家收到稳定冲突（ErrRequestNotPending → API 409）。
// 修复前形态：BeginResume 的 Get→检查→无条件 Save 让两个并发领取都成功。
func TestBeginResumeConcurrentClaimExecutesOnce(t *testing.T) {
	for _, action := range []Action{ActionForceOff, ActionReboot} {
		t.Run(string(action), func(t *testing.T) {
			for round := 0; round < 5; round++ {
				// 任意交错下结论一致；多轮防时序侥幸。
				h := newHarness(t, []string{"off", "on"})
				req := h.submit(t, action, "node-1")
				h.gate.approve(req.ApprovalID)

				const racers = 2
				start := make(chan struct{})
				errs := make(chan error, racers)
				for r := 0; r < racers; r++ {
					go func() {
						<-start
						// ResumeAsync 而非 BeginResume：领取成功即启动后台
						// executeRequest，CAS 破坏时双领取=双副作用。
						_, err := h.orch.ResumeAsync(context.Background(), req.RequestID)
						errs <- err
					}()
				}
				close(start)
				winners := 0
				for r := 0; r < racers; r++ {
					if err := <-errs; err == nil {
						winners++
					} else {
						assert.ErrorIs(t, err, ErrRequestNotPending)
					}
				}
				require.Equal(t, 1, winners, "exactly one racer must claim the request")

				final := awaitTerminal(t, h.store, req.RequestID)
				assert.Equal(t, StateDone, final.State)
				assert.Equal(t, 1, h.provider.actionCallCount(),
					"provider side effect must run exactly once regardless of racing")

				// 终态后再次 resume 仍然稳定冲突。
				_, err := h.orch.BeginResume(context.Background(), req.RequestID)
				assert.ErrorIs(t, err, ErrRequestNotPending)
			}
		})
	}
}

// PWR-001：批准与 reject 竞争——两者都从 pending CAS 领取，先到者赢；
// 落败方向收到稳定冲突，且 reject 绝不把已 executing 的请求翻回 rejected。
func TestResumeRejectRaceSingleTransition(t *testing.T) {
	// reject 先赢：pending→rejected 后 resume 冲突，零副作用。
	h := newHarness(t, []string{"off", "on"})
	req := h.submit(t, ActionPowerOff, "node-1")
	h.gate.approve(req.ApprovalID)
	_, err := h.orch.Reject(context.Background(), req.RequestID)
	require.NoError(t, err)
	_, err = h.orch.ResumeAsync(context.Background(), req.RequestID)
	assert.ErrorIs(t, err, ErrRequestNotPending)
	assert.Equal(t, 0, h.provider.actionCallCount())

	// resume 先赢：executing 之后 reject 冲突，请求照常收敛且副作用恰好一次。
	h2 := newHarness(t, []string{"off", "on"})
	req2 := h2.submit(t, ActionPowerOff, "node-1")
	h2.gate.approve(req2.ApprovalID)
	_, err = h2.orch.ResumeAsync(context.Background(), req2.RequestID)
	require.NoError(t, err)
	_, err = h2.orch.Reject(context.Background(), req2.RequestID)
	assert.ErrorIs(t, err, ErrRequestNotPending)
	final := awaitTerminal(t, h2.store, req2.RequestID)
	assert.Equal(t, StateDone, final.State)
	assert.Equal(t, 1, h2.provider.actionCallCount())
}

// PWR-002：永不返回的 provider 动作不得挂住请求——每次动作有独立
// ActionTimeout，到点记该行失败；请求在上限内落为可观察终态。
func TestHungActionTimesOutAndRequestConverges(t *testing.T) {
	h := newHarness(t, nil)
	h.orch.Providers = map[string]Provider{ProviderIPMI: &hungProvider{kind: ProviderIPMI}}
	h.orch.ActionTimeout = 40 * time.Millisecond
	h.orch.RequestTimeout = 500 * time.Millisecond

	req := h.submit(t, ActionForceOff, "node-1")
	h.gate.approve(req.ApprovalID)

	start := time.Now()
	_, err := h.orch.ResumeAsync(context.Background(), req.RequestID)
	require.NoError(t, err)
	final := awaitTerminal(t, h.store, req.RequestID)
	elapsed := time.Since(start)

	assert.Equal(t, StateFailed, final.State)
	assert.Less(t, elapsed, 1*time.Second,
		"a hung provider must not hold the request past the action bound")

	targets, err := h.store.Targets(req.RequestID)
	require.NoError(t, err)
	require.Len(t, targets, 1)
	assert.False(t, targets[0].Success)
	assert.Contains(t, targets[0].ErrorMsg, "timed out")
}

// PWR-002：状态确认卡在永不返回的 Runner 上时，整体 RequestTimeout 切断
// 验证循环，请求落为可观察失败（动作本身成功但从未被确认）。
func TestHungStatusCutByRequestBound(t *testing.T) {
	h := newHarness(t, []string{"on"})
	h.provider.hangStatus = true
	h.orch.ActionTimeout = 30 * time.Millisecond
	h.orch.RequestTimeout = 90 * time.Millisecond
	h.orch.VerifyTimeout = 30 * time.Second // 远大于请求上限：必须由整体上限切断
	h.orch.VerifyInterval = 20 * time.Millisecond

	req := h.submit(t, ActionPowerOn, "node-1")
	h.gate.approve(req.ApprovalID)

	start := time.Now()
	_, err := h.orch.ResumeAsync(context.Background(), req.RequestID)
	require.NoError(t, err)
	final := awaitTerminal(t, h.store, req.RequestID)
	elapsed := time.Since(start)

	assert.Equal(t, StateFailed, final.State)
	assert.Contains(t, final.ErrorMsg, "overall bound")
	assert.Less(t, elapsed, 2*time.Second,
		"the verification loop must stop at the overall request bound")
	// PWR-004: sent-but-never-verified stays failed with failed>0.
	assert.Equal(t, 0, final.Succeeded)
	assert.Equal(t, 1, final.Failed)

	targets, err := h.store.Targets(req.RequestID)
	require.NoError(t, err)
	require.Len(t, targets, 1)
	assert.True(t, targets[0].Success, "the action itself succeeded")
	assert.False(t, targets[0].Verified, "a hung status poll can never confirm")
}

// PWR-003：批次中途"崩溃"（node-2 动作永不返回、行停留 attempting）后启动
// 恢复：已完成目标保留、结果未知的目标记失败且绝不重放、未到达的目标补
// 失败行、请求收敛到可观察终态；恢复路径零 provider 调用。
func TestCrashMidBatchRecoveryKeepsCompletedTargets(t *testing.T) {
	h := newHarness(t, []string{"on"})
	h.provider.hangActionNode = "node-2"
	h.orch.ActionTimeout = 30 * time.Second // 崩溃前不让动作超时：行必须停留 attempting

	req := h.submit(t, ActionPowerOn, "node-1", "node-2", "node-3")
	h.gate.approve(req.ApprovalID)
	_, err := h.orch.ResumeAsync(context.Background(), req.RequestID)
	require.NoError(t, err)

	// 等"崩溃"形态落库：node-1 done、node-2 attempting（真实中途截断的持久化
	// 残留）；此后不再等待后台 goroutine——恢复读的就是这份残留。
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		rows, err := h.store.Targets(req.RequestID)
		require.NoError(t, err)
		byStatus := map[string]string{}
		for _, r := range rows {
			byStatus[r.NodeID] = r.Status
		}
		if byStatus["node-1"] == TargetDone && byStatus["node-2"] == TargetAttempting {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	// node-2 的 attempting 行先于其挂起调用落库（执行纪律），崩溃形态就绪时
	// node-2 的调用可能尚未记录——采样必须等计数稳定到崩溃窗口终值（node-1 +
	// node-2 = 2；node-3 永不触发）。否则在采样与断言之间落下的调用会被误判
	// 成"恢复重放"（预存 flake，base 0d5caef9b 上复现）。
	settle := time.Now().Add(3 * time.Second)
	for h.provider.actionCallCount() < 2 && time.Now().Before(settle) {
		time.Sleep(5 * time.Millisecond)
	}
	callsBefore := h.provider.actionCallCount()

	h.orch.RecoverInFlight() // 模拟重启后的启动恢复

	final, err := h.store.Get(req.RequestID)
	require.NoError(t, err)
	require.NotNil(t, final)
	assert.Equal(t, StateFailed, final.State, "a crashed batch converges to failed")
	assert.Contains(t, final.ErrorMsg, "interrupted by restart")
	assert.Equal(t, callsBefore, h.provider.actionCallCount(), "recovery must never replay actions")

	targets, err := h.store.Targets(req.RequestID)
	require.NoError(t, err)
	require.Len(t, targets, 3)
	byNode := map[string]*model.PowerRequestTarget{}
	for _, tgt := range targets {
		byNode[tgt.NodeID] = tgt
	}
	assert.True(t, byNode["node-1"].Success, "completed target must survive recovery")
	assert.Equal(t, TargetDone, byNode["node-1"].Status)
	assert.False(t, byNode["node-2"].Success, "unknown-outcome target fails")
	assert.Contains(t, byNode["node-2"].ErrorMsg, "outcome unknown")
	assert.False(t, byNode["node-3"].Success, "never-reached target fails")
	assert.Contains(t, byNode["node-3"].ErrorMsg, "before attempt")
}

// PWR-003：构造崩溃残留（executing + node-1 done / node-2 attempting）直接
// 验证恢复语义：保留已完成、未知记失败、请求收敛、零 provider 调用。
func TestRecoverInFlightMarksUnknownAndKeepsCompleted(t *testing.T) {
	h := newHarness(t, []string{"on"})
	req := h.submit(t, ActionPowerOn, "node-1", "node-2")
	h.gate.approve(req.ApprovalID)
	now := time.Now()
	claimed, err := h.store.Transit(req.RequestID, StatePending, StateExecuting, &now)
	require.NoError(t, err)
	require.True(t, claimed)
	at := now
	require.NoError(t, h.store.ReplaceTargets(req.RequestID, []*model.PowerRequestTarget{
		{RequestID: req.RequestID, NodeID: "node-1", Provider: ProviderIPMI, Success: true, State: "on", Status: TargetDone, AttemptAt: &at},
		{RequestID: req.RequestID, NodeID: "node-2", Provider: ProviderIPMI, Status: TargetAttempting},
	}))

	h.orch.RecoverInFlight()

	final, err := h.store.Get(req.RequestID)
	require.NoError(t, err)
	assert.Equal(t, StateFailed, final.State)
	assert.Contains(t, final.ErrorMsg, "interrupted by restart")
	assert.Equal(t, 0, h.provider.actionCallCount(), "recovery must never call providers")

	targets, err := h.store.Targets(req.RequestID)
	require.NoError(t, err)
	require.Len(t, targets, 2)
	byNode := map[string]*model.PowerRequestTarget{}
	for _, tgt := range targets {
		byNode[tgt.NodeID] = tgt
	}
	assert.True(t, byNode["node-1"].Success, "completed target must survive recovery")
	assert.Equal(t, TargetDone, byNode["node-1"].Status)
	assert.False(t, byNode["node-2"].Success)
	assert.Contains(t, byNode["node-2"].ErrorMsg, "outcome unknown")
}

// PWR-003：崩溃发生在首个 target 写入之前（executing、targets 表空）——
// 全部目标按"未尝试"记失败，请求收敛，绝不重放。
func TestRecoverInFlightNoTargetsMarksAllInterrupted(t *testing.T) {
	h := newHarness(t, []string{"on"})
	req := h.submit(t, ActionPowerOn, "node-1", "node-2")
	h.gate.approve(req.ApprovalID)
	now := time.Now()
	claimed, err := h.store.Transit(req.RequestID, StatePending, StateExecuting, &now)
	require.NoError(t, err)
	require.True(t, claimed)

	h.orch.RecoverInFlight()

	final, err := h.store.Get(req.RequestID)
	require.NoError(t, err)
	assert.Equal(t, StateFailed, final.State)
	assert.Contains(t, final.ErrorMsg, "interrupted by restart")
	assert.Equal(t, 0, h.provider.actionCallCount(), "recovery must never call providers")

	targets, err := h.store.Targets(req.RequestID)
	require.NoError(t, err)
	require.Len(t, targets, 2)
	for _, tgt := range targets {
		assert.False(t, tgt.Success)
		assert.Contains(t, tgt.ErrorMsg, "before attempt")
	}
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
	// PWR-004: header counters share the terminal aggregation definition — an
	// action that ran but was never verified is not a success (old defect:
	// state=failed with failed=0).
	assert.Equal(t, 0, final.Succeeded, "an unverified machine is not a success")
	assert.Equal(t, 1, final.Failed, "must not land state=failed with failed=0")
	assert.Equal(t, final.Total, final.Succeeded+final.Failed)
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
	// PWR-004: an unverifiable provider counts as confirmed (its definition of
	// success), so the header is done with all succeeded.
	assert.Equal(t, 1, final.Succeeded)
	assert.Equal(t, 0, final.Failed)
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

// ---- PWR-005：status 查询从审批与变更执行状态机分离 ----

// statusByNode scripts the per-node status answer for one provider kind
// (PWR-005 query paths across ipmi/cloud/agent). Mutating methods are never
// reached on the query path and stay unsupported.
type statusByNode struct {
	kind   string
	states map[string]string // nodeID → state ("on"/"off"/"unknown")
	errs   map[string]error  // nodeID → status error
	calls  []string
}

func (p *statusByNode) Kind() string { return p.kind }
func (p *statusByNode) PowerOn(context.Context, Machine) (string, error) {
	return "", ErrUnsupportedAction
}
func (p *statusByNode) PowerOff(context.Context, Machine, bool) (string, error) {
	return "", ErrUnsupportedAction
}
func (p *statusByNode) Reboot(context.Context, Machine) (string, error) {
	return "", ErrUnsupportedAction
}
func (p *statusByNode) Status(_ context.Context, m Machine) (string, error) {
	p.calls = append(p.calls, m.NodeID)
	if err := p.errs[m.NodeID]; err != nil {
		return p.states[m.NodeID], err
	}
	if st := p.states[m.NodeID]; st != "" {
		return st, nil
	}
	return "on", nil
}

func (h *harness) queryStatus(t *testing.T, nodes ...string) (*model.PowerRequest, []*model.PowerRequestTarget) {
	t.Helper()
	req, targets, err := h.orch.QueryStatus(context.Background(), resolver(), SubmitInput{
		NodeIDs: nodes, Action: ActionStatus, Reason: "health", RequestedBy: "op1",
	})
	require.NoError(t, err)
	return req, targets
}

// PWR-005：status 查询不建审批单、不进 executing/verifying——一次状态查询后
// 直接落终态并返回逐机结果（设计 §5：power.status 是查询，无需审批）。
func TestStatusQuerySkipsApprovalAndLandsTerminal(t *testing.T) {
	h := newHarness(t, []string{"on", "on"})
	h.orch.VerifyTimeout = 30 * time.Second // 延迟不得来自 verifyTimeout（另测）

	req, targets := h.queryStatus(t, "node-1", "node-2")

	require.Empty(t, h.gate.created, "a status query must not create an approval")
	assert.Empty(t, req.ApprovalID)

	assert.Equal(t, StateDone, req.State)
	assert.Equal(t, 2, req.Succeeded)
	assert.Equal(t, 0, req.Failed)
	require.NotNil(t, req.DoneAt)

	require.Len(t, targets, 2)
	for _, tgt := range targets {
		assert.Equal(t, TargetDone, tgt.Status)
		assert.True(t, tgt.Success)
		assert.Equal(t, "on", tgt.State)
		assert.False(t, tgt.Verified, "a query observes; it does not verify an action")
	}
	assert.Len(t, h.provider.calls, 2, "exactly one status call per machine, no polling")
}

// PWR-005：查询延迟由每次调用的 ActionTimeout 决定，绝不随 verifyTimeout
// 增长——挂死的 provider 在 per-call 界上落为失败行，查询整体快速返回。
func TestStatusQueryLatencyIndependentOfVerifyTimeout(t *testing.T) {
	h := newHarness(t, []string{"on"})
	h.provider.hangStatus = true
	h.orch.ActionTimeout = 50 * time.Millisecond
	h.orch.VerifyTimeout = 30 * time.Second // 远大于任何合理查询：绝不能等它
	h.orch.VerifyInterval = 20 * time.Millisecond

	start := time.Now()
	req, targets := h.queryStatus(t, "node-1")
	elapsed := time.Since(start)

	assert.Equal(t, StateFailed, req.State)
	require.Len(t, targets, 1)
	assert.False(t, targets[0].Success)
	assert.Contains(t, targets[0].ErrorMsg, "timed out")
	assert.Less(t, elapsed, 5*time.Second,
		"status query latency must not track verifyTimeout (the old verify loop waited it out)")
}

// PWR-005：IPMI/cloud/agent 三类查询失败与未知状态如实呈现——未知绝不伪装
// 成 on/off，失败行携带 provider 错误，请求头按同一确认口径聚合。
func TestStatusQueryPerProviderFailureAndUnknown(t *testing.T) {
	store := newTestStore(t)
	gate := &fakeGate{approvals: map[string]bool{}}
	ipmi := &statusByNode{kind: ProviderIPMI, states: map[string]string{"node-1": "on"}}
	cloud := &statusByNode{kind: ProviderCloud, errs: map[string]error{"node-2": errors.New("cloudctl: no such instance")}}
	agent := &statusByNode{kind: ProviderAgent,
		states: map[string]string{"node-3": "unknown"},
		errs:   map[string]error{"node-3": ErrUnknownMachineState}}
	orch := &Orchestrator{
		Store:         store,
		Approval:      gate,
		Providers:     map[string]Provider{ProviderIPMI: ipmi, ProviderCloud: cloud, ProviderAgent: agent},
		VerifyTimeout: 30 * time.Second,
	}
	res := mapResolver{m: map[string]Machine{
		"node-1": {NodeID: "node-1", Provider: ProviderIPMI, BmcAddr: "10.0.0.7"},
		"node-2": {NodeID: "node-2", Provider: ProviderCloud, CloudRef: "i-abc"},
		"node-3": {NodeID: "node-3", Provider: ProviderAgent, AgentID: "ag-1"},
	}}
	req, targets, err := orch.QueryStatus(context.Background(), res, SubmitInput{
		NodeIDs: []string{"node-1", "node-2", "node-3"}, Action: ActionStatus, RequestedBy: "op1",
	})
	require.NoError(t, err)

	// 一个 confirmed（ipmi on）、两个未确认（cloud 查询失败、agent 未知）。
	assert.Equal(t, StatePartial, req.State)
	assert.Equal(t, 1, req.Succeeded)
	assert.Equal(t, 2, req.Failed)

	byNode := map[string]*model.PowerRequestTarget{}
	for _, tgt := range targets {
		byNode[tgt.NodeID] = tgt
	}
	assert.True(t, byNode["node-1"].Success)
	assert.Equal(t, "on", byNode["node-1"].State)
	assert.False(t, byNode["node-2"].Success)
	assert.Contains(t, byNode["node-2"].ErrorMsg, "no such instance")
	assert.False(t, byNode["node-3"].Success, "an unknown observation is not a success")
	assert.Equal(t, "unknown", byNode["node-3"].State)
	assert.Contains(t, byNode["node-3"].ErrorMsg, "power state unknown")
}

// PWR-005：provider 答 "unknown"（无错误）时仍是未确认——诚实上报未知，落
// failed 而非伪装 done。
func TestStatusQueryUnknownWithoutErrorIsNotConfirmed(t *testing.T) {
	h := newHarness(t, []string{"unknown"})
	req, targets := h.queryStatus(t, "node-1")

	assert.Equal(t, StateFailed, req.State)
	assert.Equal(t, 0, req.Succeeded)
	assert.Equal(t, 1, req.Failed)
	require.Len(t, targets, 1)
	assert.False(t, targets[0].Success)
	assert.Equal(t, "unknown", targets[0].State)
	assert.Contains(t, targets[0].ErrorMsg, "power state unknown")
}

// PWR-005：confirmed 的定义——一个 Success=true 但 State=unknown 的目标行
// （legacy executeRequest 经 RunAction 的产物）不计为已确认：终态不因"动作
// 成功"而把未知观察谎报成 done。
func TestStatusTerminalAggregationTreatsUnknownAsUnconfirmed(t *testing.T) {
	rows := []*model.PowerRequestTarget{
		{NodeID: "n1", Provider: ProviderIPMI, Success: true, State: "on"},
		{NodeID: "n2", Provider: ProviderIPMI, Success: true, State: "unknown"},
	}
	assert.Equal(t, StatePartial, terminalState(ActionStatus, rows))
	assert.Equal(t, 1, confirmedCount(ActionStatus, rows))
}

// PWR-005：Submit 拒绝 status（结构上强制走只读查询），QueryStatus 拒绝变更
// 动作——分离是结构性约束，不是调用方约定。
func TestSubmitRefusesStatusAction(t *testing.T) {
	h := newHarness(t, []string{"on"})
	_, err := h.orch.Submit(context.Background(), resolver(), SubmitInput{
		NodeIDs: []string{"node-1"}, Action: ActionStatus,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read-only query")
	rows, _ := h.store.List(10)
	assert.Empty(t, rows, "no request row may be persisted for a refused status submit")

	_, _, err = h.orch.QueryStatus(context.Background(), resolver(), SubmitInput{
		NodeIDs: []string{"node-1"}, Action: ActionPowerOn,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only accepts the status action")
}
