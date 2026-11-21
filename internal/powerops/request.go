package powerops

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/cuihairu/croupier/internal/model"
)

// Request states (design §6).
const (
	StatePending   = "pending"
	StateExecuting = "executing"
	StateVerifying = "verifying"
	StateDone      = "done"
	StatePartial   = "partial"
	StateFailed    = "failed"
	StateRejected  = "rejected"
)

// Per-target progress markers (PWR-003): they drive crash recovery. A target
// row is written as attempting before its action and flipped to done after the
// outcome is recorded; a crash mid-batch leaves the in-flight row attempting,
// which recovery treats as an unknown outcome and never replays.
const (
	TargetAttempting = "attempting"
	TargetDone       = "done"
)

// ErrInterrupted is the per-target failure recorded by startup recovery for a
// machine whose outcome is unknown after a process crash (PWR-003). Destructive
// operations are never blindly replayed: the request converges to a terminal
// state and the operator decides.
const ErrInterrupted = "powerops: interrupted by restart, outcome unknown (not replayed)"

// ErrNotAttempted is the per-target failure for machines the execution loop
// never reached before the crash — nothing was sent, and recovery still refuses
// to replay (fail-closed).
const ErrNotAttempted = "powerops: interrupted by restart before attempt (not replayed)"

// Sentinel errors — the API layer maps them to conflict/not-found responses
// (provision resume precedent).
var (
	ErrRequestNotFound     = errors.New("powerops: request not found")
	ErrRequestNotPending   = errors.New("powerops: request is not pending")
	ErrRequestNotApproved  = errors.New("powerops: approval not granted")
	ErrApprovalLinkMissing = errors.New("powerops: request has no approval record")
	// ErrInvalidSubmit 是提交校验失败族（动作缺失/目标为空/机器未注册/
	// (机器,动作) 不支持）的 wrap 根；API 层据此映射 422。
	ErrInvalidSubmit = errors.New("powerops: invalid request")
)

// RequestStore persists power requests and their per-machine targets.
type RequestStore interface {
	Create(r *model.PowerRequest) error
	Update(r *model.PowerRequest) error
	Get(requestID string) (*model.PowerRequest, error)
	List(limit int) ([]*model.PowerRequest, error)
	// ListInFlight returns requests still in executing/verifying — the set a
	// process crash leaves behind, which startup recovery converges without
	// replaying any action (PWR-003).
	ListInFlight() ([]*model.PowerRequest, error)
	// Transit claims a state transition with a single conditional update
	// (state: fromState → toState). True means this caller performed the
	// transition; false means the row already moved elsewhere — the CAS
	// loser must not proceed to side effects (PWR-001: concurrent resume
	// must never double-execute a provider action). startedAt persists
	// StartedAt when claiming executing; nil leaves it untouched.
	Transit(requestID, fromState, toState string, startedAt *time.Time) (bool, error)
	// ReplaceTargets rewrites the per-machine rows for a request (execution
	// and verification both refresh them).
	ReplaceTargets(requestID string, targets []*model.PowerRequestTarget) error
	Targets(requestID string) ([]*model.PowerRequestTarget, error)
}

// ApprovalGate connects the request state machine to the server approvals
// store (the API wiring adapts approvals.Store, provisionApprovalGate
// precedent). The approved scope lives in the request snapshot, so the gate
// only needs the action and request id for the record's metadata.
type ApprovalGate interface {
	CreateApproval(ctx context.Context, action Action, requestID, requestedBy, reason string, targetCount int) (string, error)
	Approved(ctx context.Context, approvalID string) (bool, error)
}

// MachineResolver resolves node ids to power machines from the inventory
// (nodes.meta.power) at submit time.
type MachineResolver interface {
	Resolve(nodeID string) (Machine, bool)
}

// Orchestrator runs the request state machine (design §6). HTTP layers stay
// DTO shells; execution runs on disconnect-proof background contexts.
type Orchestrator struct {
	Store          RequestStore
	Approval       ApprovalGate
	Providers      map[string]Provider
	VerifyTimeout  time.Duration // default 120s
	VerifyInterval time.Duration // default 2s
	// ActionTimeout bounds a single provider action/status call (default 30s);
	// a hung provider cannot hold the request open (PWR-002).
	ActionTimeout time.Duration
	// RequestTimeout bounds the whole execution run (actions + verification,
	// default 5m); beyond it the request lands in an observable terminal state.
	RequestTimeout time.Duration
	Now            func() time.Time
}

func (o *Orchestrator) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

func (o *Orchestrator) verifyTimeout() time.Duration {
	if o.VerifyTimeout > 0 {
		return o.VerifyTimeout
	}
	return 120 * time.Second
}

func (o *Orchestrator) verifyInterval() time.Duration {
	if o.VerifyInterval > 0 {
		return o.VerifyInterval
	}
	return 2 * time.Second
}

func (o *Orchestrator) actionTimeout() time.Duration {
	if o.ActionTimeout > 0 {
		return o.ActionTimeout
	}
	return 30 * time.Second
}

func (o *Orchestrator) requestTimeout() time.Duration {
	if o.RequestTimeout > 0 {
		return o.RequestTimeout
	}
	return 5 * time.Minute
}

// NewRequestID mints a public request id (time-ordered, random suffix).
func NewRequestID(now time.Time) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("pw-%013d-%s", now.UnixMilli(), hex.EncodeToString(b[:]))
}

// SubmitInput is a validated submit payload (node ids are resolved through
// MachineResolver, never trusted raw).
type SubmitInput struct {
	NodeIDs     []string
	Action      Action
	Reason      string
	RequestedBy string
	GameID      string
	Env         string
}

// resolveSubmitMachines validates a submit payload and resolves node ids to
// power machines through the resolver (never trusted raw — SubmitInput.NodeIDs
// are just public ids). Unknown machines and unsupported (machine, action)
// pairs are rejected up front so an approval never gets created for a request
// that cannot execute. Status queries share this validation so a malformed
// query is rejected before any provider call.
func resolveSubmitMachines(resolver MachineResolver, in SubmitInput) ([]Machine, error) {
	if in.Action == "" {
		return nil, fmt.Errorf("%w: action is required", ErrInvalidSubmit)
	}
	if in.Action != ActionStatus && len(in.NodeIDs) == 0 {
		return nil, fmt.Errorf("%w: at least one target machine is required", ErrInvalidSubmit)
	}
	seen := make(map[string]bool, len(in.NodeIDs))
	machines := make([]Machine, 0, len(in.NodeIDs))
	for _, id := range in.NodeIDs {
		id = trimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		m, ok := resolver.Resolve(id)
		if !ok {
			return nil, fmt.Errorf("%w: machine %s not registered", ErrInvalidSubmit, id)
		}
		if !Supports(m.Provider, in.Action) {
			return nil, fmt.Errorf("%w: machine %s (provider %s) does not support %s", ErrInvalidSubmit, id, m.Provider, in.Action)
		}
		machines = append(machines, m)
	}
	if len(machines) == 0 {
		return nil, fmt.Errorf("%w: at least one target machine is required", ErrInvalidSubmit)
	}
	return machines, nil
}

// Submit persists a pending request and creates its approval record.
// Validation happens before any persistence: unknown machines and
// unsupported (machine, action) pairs are rejected up front so an approval
// never gets created for a request that cannot execute.
//
// status is not an approval-gated change (PWR-005, design §5) — callers must
// route it to QueryStatus; this guard makes the separation structural rather
// than a convention the caller has to remember.
func (o *Orchestrator) Submit(ctx context.Context, resolver MachineResolver, in SubmitInput) (*model.PowerRequest, error) {
	if in.Action == ActionStatus {
		return nil, fmt.Errorf("%w: status is a read-only query (use QueryStatus, no approval)", ErrInvalidSubmit)
	}
	machines, err := resolveSubmitMachines(resolver, in)
	if err != nil {
		return nil, err
	}

	now := o.now()
	targetsJSON, err := json.Marshal(machines)
	if err != nil {
		return nil, fmt.Errorf("powerops: marshal targets: %w", err)
	}
	req := &model.PowerRequest{
		RequestID:   NewRequestID(now),
		Action:      string(in.Action),
		Reason:      trimSpace(in.Reason),
		RequestedBy: in.RequestedBy,
		State:       StatePending,
		TargetsJSON: string(targetsJSON),
		Total:       len(machines),
		GameID:      in.GameID,
		Env:         in.Env,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := o.Store.Create(req); err != nil {
		return nil, fmt.Errorf("powerops: persist pending request: %w", err)
	}
	approvalID, err := o.Approval.CreateApproval(ctx, in.Action, req.RequestID, in.RequestedBy, req.Reason, len(machines))
	if err != nil {
		req.ErrorMsg = "approval gate: " + err.Error()
		req.UpdatedAt = o.now()
		_ = o.Store.Update(req)
		return nil, fmt.Errorf("powerops: create approval: %w", err)
	}
	req.ApprovalID = approvalID
	req.UpdatedAt = o.now()
	if err := o.Store.Update(req); err != nil {
		return nil, fmt.Errorf("powerops: persist approval link: %w", err)
	}
	return req, nil
}

// QueryStatus runs power.status as a read-only snapshot (design §5, PWR-005):
// a status query needs authentication and machine scope only — no approval
// record, no pending/executing/verifying states. One bounded Status call per
// machine (the PWR-002 per-call bound governs latency, never verifyTimeout),
// then the request lands directly in its terminal state with the per-machine
// rows and is returned synchronously. Nothing is left in flight: the row is
// written once at the end, so a crash mid-query leaves no residue to recover
// and — being a read — there is no side effect to replay.
func (o *Orchestrator) QueryStatus(ctx context.Context, resolver MachineResolver, in SubmitInput) (*model.PowerRequest, []*model.PowerRequestTarget, error) {
	if in.Action != ActionStatus {
		return nil, nil, fmt.Errorf("%w: QueryStatus only accepts the status action", ErrInvalidSubmit)
	}
	machines, err := resolveSubmitMachines(resolver, in)
	if err != nil {
		return nil, nil, err
	}
	now := o.now()
	requestID := NewRequestID(now)
	targets := make([]*model.PowerRequestTarget, 0, len(machines))
	for _, m := range machines {
		t := &model.PowerRequestTarget{
			RequestID: requestID,
			NodeID:    m.NodeID,
			Provider:  m.Provider,
			Status:    TargetDone,
			AttemptAt: &now,
		}
		p, ok := o.Providers[m.Provider]
		if !ok {
			t.ErrorMsg = "powerops: no provider registered for kind " + m.Provider
			targets = append(targets, t)
			continue
		}
		st, qerr := o.statusBounded(ctx, p, m)
		switch {
		case qerr != nil:
			// Query failure (timeout, unreachable, inconclusive provider
			// output): an honest failure row; the provider may still report
			// "unknown" alongside the error, which is preserved.
			t.ErrorMsg = truncate(qerr.Error(), 512)
			t.State = st
		case st == "" || st == "unknown":
			// The provider answered but cannot tell — reported as unknown,
			// never smoothed into on/off (design §9 honesty).
			t.ErrorMsg = ErrUnknownMachineState.Error()
			t.State = st
		default:
			t.Success = true
			t.State = st
		}
		targets = append(targets, t)
	}
	targetsJSON, err := json.Marshal(machines)
	if err != nil {
		return nil, nil, fmt.Errorf("powerops: marshal targets: %w", err)
	}
	succeeded := confirmedCount(ActionStatus, targets)
	req := &model.PowerRequest{
		RequestID:   requestID,
		Action:      string(ActionStatus),
		Reason:      trimSpace(in.Reason),
		RequestedBy: in.RequestedBy,
		State:       terminalState(ActionStatus, targets),
		TargetsJSON: string(targetsJSON),
		Total:       len(machines),
		Succeeded:   succeeded,
		Failed:      len(machines) - succeeded,
		GameID:      in.GameID,
		Env:         in.Env,
		CreatedAt:   now,
		UpdatedAt:   now,
		DoneAt:      &now,
	}
	if err := o.Store.Create(req); err != nil {
		return nil, nil, fmt.Errorf("powerops: persist status query: %w", err)
	}
	if err := o.Store.ReplaceTargets(requestID, targets); err != nil {
		// The header row is already terminal; losing the per-machine rows on
		// a write failure is logged, not turned into a phantom error.
		slog.Error("powerops: persist status targets", "request", requestID, "error", err)
	}
	return req, targets, nil
}

// ResumeAsync proceeds after approval: re-checks approved state (fail-closed,
// provision resume precedent), marks executing and runs the request on a
// disconnect-proof background context. It returns the executing snapshot —
// never the live record, which executeRequest keeps mutating in its own
// goroutine (State/UpdatedAt per phase). Handing the caller that pointer is a
// data race: the HTTP handler marshals the response off the same field the
// background goroutine writes (-race 实证：PowerResume 响应序列化撞
// executeRequest 的 verifying 落库写，2026-11-12 nightly deep-test 捕获)。
func (o *Orchestrator) ResumeAsync(ctx context.Context, requestID string) (*model.PowerRequest, error) {
	req, err := o.BeginResume(ctx, requestID)
	if err != nil {
		return nil, err
	}
	snapshot := copyPowerRequest(req)
	bg := context.WithoutCancel(ctx)
	go o.executeRequest(bg, req)
	return snapshot, nil
}

// copyPowerRequest 返回可安全跨 goroutine 共享的副本。结构体浅拷后补拷
// 三个时间指针（指针目标本身不可变，但字段赋值是竞态面，副本须自带字段）。
func copyPowerRequest(src *model.PowerRequest) *model.PowerRequest {
	if src == nil {
		return nil
	}
	dst := *src
	if src.StartedAt != nil {
		v := *src.StartedAt
		dst.StartedAt = &v
	}
	if src.DoneAt != nil {
		v := *src.DoneAt
		dst.DoneAt = &v
	}
	return &dst
}

// BeginResume is the synchronous half of resume: state checks + pending →
// executing. Exposed for tests and for callers that drive execution
// themselves.
func (o *Orchestrator) BeginResume(ctx context.Context, requestID string) (*model.PowerRequest, error) {
	req, err := o.Store.Get(trimSpace(requestID))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRequestNotFound, err)
	}
	if req == nil {
		return nil, ErrRequestNotFound
	}
	if req.State != StatePending {
		return nil, fmt.Errorf("%w: state=%s", ErrRequestNotPending, req.State)
	}
	if req.ApprovalID == "" {
		return nil, ErrApprovalLinkMissing
	}
	ok, err := o.Approval.Approved(ctx, req.ApprovalID)
	if err != nil {
		// Fail-closed: approval store errors must not execute the request.
		return nil, fmt.Errorf("%w: approval lookup failed: %v", ErrRequestNotApproved, err)
	}
	if !ok {
		return nil, ErrRequestNotApproved
	}
	now := o.now()
	req.State = StateExecuting
	req.StartedAt = &now
	claimed, err := o.Store.Transit(req.RequestID, StatePending, StateExecuting, &now)
	if err != nil {
		return nil, fmt.Errorf("powerops: persist executing state: %w", err)
	}
	if !claimed {
		// CAS 输家：行已被并发 resume/reject 移走，稳定冲突（API 映射 409）。
		return nil, claimConflict(o.Store, req.RequestID)
	}
	req.UpdatedAt = now
	return req, nil
}

// claimConflict reports the losing side of a pending transition race: the
// row is re-read so the message names the winning state, while the sentinel
// keeps the stable conflict mapping (ErrRequestNotPending → 409).
func claimConflict(store RequestStore, requestID string) error {
	state := "unknown"
	if current, err := store.Get(requestID); err == nil && current != nil {
		state = current.State
	}
	return fmt.Errorf("%w: state=%s (claimed by a concurrent resume/reject)", ErrRequestNotPending, state)
}

// Reject marks a request rejected when its approval record is denied. The
// request must still be pending; nothing has executed, so no provider call
// is ever made on this path. The pending → rejected move is also a CAS
// claim: a concurrent approved resume that already flipped the row to
// executing cannot be overwritten back to rejected.
func (o *Orchestrator) Reject(ctx context.Context, requestID string) (*model.PowerRequest, error) {
	req, err := o.Store.Get(trimSpace(requestID))
	if err != nil || req == nil {
		return nil, ErrRequestNotFound
	}
	if req.State != StatePending {
		return nil, fmt.Errorf("%w: state=%s", ErrRequestNotPending, req.State)
	}
	now := o.now()
	claimed, err := o.Store.Transit(req.RequestID, StatePending, StateRejected, nil)
	if err != nil {
		return nil, fmt.Errorf("powerops: persist rejected state: %w", err)
	}
	if !claimed {
		return nil, claimConflict(o.Store, req.RequestID)
	}
	req.State = StateRejected
	req.UpdatedAt = now
	return req, nil
}

// expectedState is the post-action state that counts as verified.
func expectedState(a Action) string {
	switch a {
	case ActionPowerOn, ActionReboot:
		return "on"
	case ActionPowerOff, ActionForceOff:
		return "off"
	default:
		return ""
	}
}

// verifiable reports whether the provider kind can observe power state at
// all (WOL cannot — design §3; wake confirmation is agent reconnection).
func verifiable(provider string) bool {
	return provider == ProviderIPMI || provider == ProviderCloud || provider == ProviderAgent
}

// verifiableFor reports whether a specific machine's post-action state can be
// observed at all. WOL itself cannot (design §3), but a WOL machine that
// carries an agent reference is confirmable by agent reconnection (PWR-006):
// the verify loop polls the referenced agent's session instead of the WOL
// provider, which is the only proof a wake actually brought the host up.
func verifiableFor(m Machine) bool {
	if m.Provider == ProviderWOL {
		return m.AgentID != ""
	}
	return verifiable(m.Provider)
}

// executeRequest runs the actions, then verifies, then lands the terminal
// state. The whole run is bounded by RequestTimeout (PWR-002) so a hung
// provider cannot keep the request in executing/verifying forever; each
// provider call additionally carries its own ActionTimeout. Per-machine rows
// are refreshed after every mutation so a crash leaves durable partial
// progress, not a fabricated one.
func (o *Orchestrator) executeRequest(ctx context.Context, req *model.PowerRequest) {
	runCtx, cancelRun := context.WithTimeout(ctx, o.requestTimeout())
	defer cancelRun()

	machines, err := decodeTargets(req.TargetsJSON)
	if err != nil || len(machines) == 0 {
		o.failRequest(req, "decode targets: "+errString(err))
		return
	}
	action := Action(req.Action)
	now := o.now()
	targets := make([]*model.PowerRequestTarget, 0, len(machines))
	for _, m := range machines {
		// PWR-003: persist this machine as attempting BEFORE the action — a
		// crash mid-batch leaves a durable "outcome unknown" marker, so startup
		// recovery can converge without ever replaying a destructive operation.
		t := &model.PowerRequestTarget{
			RequestID: req.RequestID,
			NodeID:    m.NodeID,
			Provider:  m.Provider,
			Status:    TargetAttempting,
		}
		targets = append(targets, t)
		o.persistTargets(req.RequestID, targets)

		res := o.runActionBounded(runCtx, m, action, now)
		t.Success = res.Success
		t.State = res.State
		t.Output = truncate(res.Output, 512)
		t.ErrorMsg = truncate(res.Error, 512)
		t.AttemptAt = &now
		t.Status = TargetDone
		o.persistTargets(req.RequestID, targets)
	}
	o.persistTargets(req.RequestID, targets)

	if action == ActionStatus {
		// PWR-005: a status query is its own observation — nothing to verify
		// and no approval gate, so it must not enter verifying nor wait out the
		// verify timeout. New status requests never reach here (Submit routes
		// them to QueryStatus); this guards a legacy request submitted before
		// the separation and resumed afterwards.
		o.finish(req, targets, terminalState(action, targets))
		return
	}

	anySuccess := false
	for _, t := range targets {
		if t.Success {
			anySuccess = true
			break
		}
	}
	if !anySuccess {
		o.finish(req, targets, StateFailed)
		return
	}

	req.State = StateVerifying
	req.UpdatedAt = o.now()
	if err := o.Store.Update(req); err != nil {
		slog.Error("powerops: persist verifying state", "request", req.RequestID, "error", err)
	}
	o.verify(runCtx, req, machines, targets, action)
}

// persistTargets rewrites the per-machine rows, logging (never failing) a write
// error: a crash mid-batch still leaves the most recent durable slice, and a
// write failure here must not abort the run — the request keeps converging and
// the durable progress is what recovery reads.
func (o *Orchestrator) persistTargets(requestID string, targets []*model.PowerRequestTarget) {
	if err := o.Store.ReplaceTargets(requestID, targets); err != nil {
		slog.Error("powerops: persist targets", "request", requestID, "error", err)
	}
}

// RecoverInFlight converges requests a previous process left in executing or
// verifying (PWR-003). It never re-runs actions: a machine whose outcome is
// unknown (attempting at crash) is marked failed and the request lands in an
// observable terminal state for the operator — no blind replay of destructive
// operations. Completed targets are preserved; machines never reached are
// marked interrupted.
func (o *Orchestrator) RecoverInFlight() {
	reqs, err := o.Store.ListInFlight()
	if err != nil {
		slog.Error("powerops: list in-flight requests", "error", err)
		return
	}
	for _, req := range reqs {
		o.recoverRequest(req)
	}
}

// recoverRequest converges one interrupted request. Only rows whose Status is
// done are kept as-is (the action outcome was recorded); attempting rows are
// failed with the unknown-outcome marker; machines absent from the targets
// table (the loop never reached them) are failed as never attempted. No
// provider call is made on this path.
func (o *Orchestrator) recoverRequest(req *model.PowerRequest) {
	machines, err := decodeTargets(req.TargetsJSON)
	if err != nil || len(machines) == 0 {
		o.failRequest(req, "decode targets: "+errString(err))
		return
	}
	existing, err := o.Store.Targets(req.RequestID)
	if err != nil {
		o.failRequest(req, "load targets: "+err.Error())
		return
	}
	byNode := make(map[string]*model.PowerRequestTarget, len(existing))
	for _, t := range existing {
		byNode[t.NodeID] = t
	}
	targets := make([]*model.PowerRequestTarget, 0, len(machines))
	unknown := 0
	for _, m := range machines {
		t, ok := byNode[m.NodeID]
		if ok && t.Status == TargetDone {
			targets = append(targets, t)
			continue
		}
		msg := ErrInterrupted
		if ok {
			unknown++ // attempting at crash: outcome unknown
		} else {
			msg = ErrNotAttempted // loop never reached this machine
		}
		targets = append(targets, &model.PowerRequestTarget{
			RequestID: req.RequestID,
			NodeID:    m.NodeID,
			Provider:  m.Provider,
			Status:    TargetDone,
			ErrorMsg:  msg,
		})
	}
	msg := "powerops: interrupted by restart, outcome unknown (not replayed)"
	if unknown == 0 {
		msg = "powerops: interrupted by restart before completion (not replayed)"
	}
	req.ErrorMsg = truncate(msg, 512)
	o.finish(req, targets, terminalState(Action(req.Action), targets))
}

// runActionBounded runs one provider action with its own ActionTimeout
// (PWR-002): the call runs in a goroutine and a deadline lands it as an
// honest failure row, so a provider that never returns cannot hold the
// request open. Providers are expected to honor ctx; one that ignores it is
// abandoned at the deadline and the request keeps converging.
func (o *Orchestrator) runActionBounded(ctx context.Context, m Machine, a Action, at time.Time) ActionResult {
	bound := o.actionTimeout()
	actCtx, cancel := context.WithTimeout(ctx, bound)
	defer cancel()

	type result struct {
		res ActionResult
	}
	ch := make(chan result, 1)
	go func() {
		ch <- result{res: RunAction(actCtx, o.Providers, m, a, at)}
	}()
	select {
	case r := <-ch:
		return r.res
	case <-actCtx.Done():
		return ActionResult{
			NodeID: m.NodeID,
			At:     at,
			Error:  "powerops: action timed out after " + bound.String(),
		}
	}
}

// verify polls each actionable machine until it reaches the expected state
// or the deadline expires; unconfirmed machines stay honestly unverified and
// count as failed in the final aggregation (a machine that did not come back
// is not "done" — design §6).
func (o *Orchestrator) verify(ctx context.Context, req *model.PowerRequest, machines []Machine, targets []*model.PowerRequestTarget, action Action) {
	want := expectedState(action)
	deadline := o.now().Add(o.verifyTimeout())
	byNode := make(map[string]*model.PowerRequestTarget, len(targets))
	for _, t := range targets {
		byNode[t.NodeID] = t
	}
	byMachine := make(map[string]Machine, len(machines))
	for _, m := range machines {
		byMachine[m.NodeID] = m
	}

	for o.now().Before(deadline) && ctx.Err() == nil {
		pending := false
		for _, t := range targets {
			m := byMachine[t.NodeID]
			if !t.Success || t.Verified || !verifiableFor(m) {
				continue
			}
			p, ok := o.Providers[t.Provider]
			if !ok || m == (Machine{}) {
				continue
			}
			// PWR-006: a WOL machine cannot observe its own power state; the
			// wake is confirmed by the referenced agent's session coming back
			// online, so the poll routes to the agent provider using the WOL
			// machine's agent id (the only proof the host came up).
			if t.Provider == ProviderWOL {
				p, ok = o.Providers[ProviderAgent]
				if !ok {
					continue
				}
			}
			st, err := o.statusBounded(ctx, p, m)
			if err == nil && st == want {
				at := o.now()
				t.Verified = true
				t.State = st
				t.VerifiedAt = &at
			} else {
				pending = true
				if err == nil {
					t.State = st
				}
			}
		}
		_ = o.Store.ReplaceTargets(req.RequestID, targets)
		if !pending {
			break
		}
		sleepCtx(ctx, o.verifyInterval())
	}
	if ctx.Err() != nil {
		req.ErrorMsg = truncate("powerops: request exceeded overall bound: "+ctx.Err().Error(), 512)
	} else if note := wolUnverifiedNote(byMachine, targets); note != "" {
		// PWR-006: name dispatched-but-unconfirmed WOL wakes so the honest
		// "packet sent, machine not confirmed up" outcome is actionable
		// instead of a bare failed/partial.
		req.ErrorMsg = truncate(note, 512)
	}
	o.finish(req, targets, terminalState(action, targets))
}

// wolUnverifiedNote names the WOL wakes that left but never confirmed
// (PWR-006), splitting the two causes an operator must act on differently:
// no agent reference to confirm against, versus a referenced agent that never
// reconnected. The packet send is not machine-up in either case.
func wolUnverifiedNote(byMachine map[string]Machine, targets []*model.PowerRequestTarget) string {
	noAgent, noReconnect := 0, 0
	for _, t := range targets {
		if t.Provider != ProviderWOL || !t.Success || t.Verified {
			continue
		}
		if byMachine[t.NodeID].AgentID == "" {
			noAgent++
		} else {
			noReconnect++
		}
	}
	switch {
	case noAgent > 0 && noReconnect > 0:
		return fmt.Sprintf("powerops: %d WOL wake(s) dispatched without an agent to confirm, %d not confirmed by agent reconnect", noAgent, noReconnect)
	case noAgent > 0:
		return fmt.Sprintf("powerops: %d WOL wake(s) dispatched but no agent configured to confirm (machine left unverified)", noAgent)
	case noReconnect > 0:
		return fmt.Sprintf("powerops: %d WOL wake(s) dispatched but agent did not reconnect (machine left unverified)", noReconnect)
	default:
		return ""
	}
}

// statusBounded bounds one Status call with the same per-call ActionTimeout
// (PWR-002) so a hung status poll cannot stall the verification loop; the
// loop keeps retrying until the overall bound or the verify deadline.
func (o *Orchestrator) statusBounded(ctx context.Context, p Provider, m Machine) (string, error) {
	bound := o.actionTimeout()
	stCtx, cancel := context.WithTimeout(ctx, bound)
	defer cancel()

	type result struct {
		state string
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		s, e := p.Status(stCtx, m)
		ch <- result{state: s, err: e}
	}()
	select {
	case r := <-ch:
		return r.state, r.err
	case <-stCtx.Done():
		return "", fmt.Errorf("powerops: status timed out after %s", bound)
	}
}

// confirmed reports whether a target row counts as a confirmed outcome for
// header counters and terminal aggregation (PWR-004 unified definition,
// extended by the status query path — PWR-005, and the WOL dispatch/confirm
// split — PWR-006). A mutating action is confirmed when the post-action state
// was observed (Verified) or the provider cannot observe at all — except WOL,
// where the packet send is never machine-up: only an agent reconnect
// (Verified) confirms the wake. A status query is its own observation: a
// definitive on/off answer confirms the machine; an unknown answer or a
// failed query does not (the honest answer "I cannot tell" is not a
// confirmed power state).
func confirmed(t *model.PowerRequestTarget, a Action) bool {
	if a == ActionStatus {
		return t.Success && t.State != "" && t.State != "unknown"
	}
	if t.Provider == ProviderWOL {
		// PWR-006: the magic packet proves the packet left, never that the
		// host came up — confirmed only on agent reconnection (with or without
		// an agent reference; without one it can never confirm).
		return t.Success && t.Verified
	}
	return t.Success && (t.Verified || !verifiable(t.Provider))
}

func confirmedCount(a Action, targets []*model.PowerRequestTarget) int {
	n := 0
	for _, t := range targets {
		if confirmed(t, a) {
			n++
		}
	}
	return n
}

// terminalState aggregates: succeeded = confirmed targets; failed = the rest.
// all confirmed → done, none → failed, mixed → partial. status queries share
// this aggregation with their own confirmation definition (see confirmed).
func terminalState(a Action, targets []*model.PowerRequestTarget) string {
	succeeded := confirmedCount(a, targets)
	switch {
	case succeeded == len(targets) && succeeded > 0:
		return StateDone
	case succeeded == 0:
		return StateFailed
	default:
		return StatePartial
	}
}

func (o *Orchestrator) finish(req *model.PowerRequest, targets []*model.PowerRequestTarget, state string) {
	// PWR-004: header counters must use the SAME success definition as the
	// terminal aggregation (terminalState) — a machine counts as succeeded only
	// when confirmed (verified, or the provider cannot verify at all). Counting
	// action-sent (Success) instead made a request where every action ran but
	// none was verified land as state=failed with failed=0. The "action sent"
	// vs "state verified" distinction stays expressed per-target (Success and
	// Verified fields).
	confirmed := confirmedCount(Action(req.Action), targets)
	now := o.now()
	req.State = state
	req.Succeeded = confirmed
	req.Failed = len(targets) - confirmed
	req.DoneAt = &now
	req.UpdatedAt = now
	if err := o.Store.Update(req); err != nil {
		slog.Error("powerops: persist terminal state", "request", req.RequestID, "error", err)
	}
	_ = o.Store.ReplaceTargets(req.RequestID, targets)
}

func (o *Orchestrator) failRequest(req *model.PowerRequest, msg string) {
	now := o.now()
	req.State = StateFailed
	req.ErrorMsg = truncate(msg, 512)
	req.DoneAt = &now
	req.UpdatedAt = now
	_ = o.Store.Update(req)
}

func decodeTargets(raw string) ([]Machine, error) {
	var machines []Machine
	if err := json.Unmarshal([]byte(raw), &machines); err != nil {
		return nil, err
	}
	return machines, nil
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
