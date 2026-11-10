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

// Submit persists a pending request and creates its approval record.
// Validation happens before any persistence: unknown machines and
// unsupported (machine, action) pairs are rejected up front so an approval
// never gets created for a request that cannot execute.
func (o *Orchestrator) Submit(ctx context.Context, resolver MachineResolver, in SubmitInput) (*model.PowerRequest, error) {
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

// ResumeAsync proceeds after approval: re-checks approved state (fail-closed,
// provision resume precedent), marks executing and runs the request on a
// disconnect-proof background context. It returns the executing snapshot.
func (o *Orchestrator) ResumeAsync(ctx context.Context, requestID string) (*model.PowerRequest, error) {
	req, err := o.BeginResume(ctx, requestID)
	if err != nil {
		return nil, err
	}
	bg := context.WithoutCancel(ctx)
	go o.executeRequest(bg, req)
	return req, nil
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
	req.UpdatedAt = now
	if err := o.Store.Update(req); err != nil {
		return nil, fmt.Errorf("powerops: persist executing state: %w", err)
	}
	return req, nil
}

// Reject marks a request rejected when its approval record is denied. The
// request must still be pending; nothing has executed, so no provider call
// is ever made on this path.
func (o *Orchestrator) Reject(ctx context.Context, requestID string) (*model.PowerRequest, error) {
	req, err := o.Store.Get(trimSpace(requestID))
	if err != nil || req == nil {
		return nil, ErrRequestNotFound
	}
	if req.State != StatePending {
		return nil, fmt.Errorf("%w: state=%s", ErrRequestNotPending, req.State)
	}
	now := o.now()
	req.State = StateRejected
	req.UpdatedAt = now
	if err := o.Store.Update(req); err != nil {
		return nil, fmt.Errorf("powerops: persist rejected state: %w", err)
	}
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
// all (WOL cannot — design §3; wake confirmation is agent reconnection and
// stays honestly unverified in this phase).
func verifiable(provider string) bool {
	return provider == ProviderIPMI || provider == ProviderCloud || provider == ProviderAgent
}

// executeRequest runs the actions, then verifies, then lands the terminal
// state. Per-machine rows are refreshed after every mutation so a crash
// leaves durable partial progress, not a fabricated one.
func (o *Orchestrator) executeRequest(ctx context.Context, req *model.PowerRequest) {
	machines, err := decodeTargets(req.TargetsJSON)
	if err != nil || len(machines) == 0 {
		o.failRequest(req, "decode targets: "+errString(err))
		return
	}
	action := Action(req.Action)
	now := o.now()
	targets := make([]*model.PowerRequestTarget, 0, len(machines))
	for _, m := range machines {
		res := RunAction(ctx, o.Providers, m, action, now)
		t := &model.PowerRequestTarget{
			RequestID: req.RequestID,
			NodeID:    m.NodeID,
			Provider:  m.Provider,
			Success:   res.Success,
			State:     res.State,
			Output:    truncate(res.Output, 512),
			ErrorMsg:  truncate(res.Error, 512),
			AttemptAt: &now,
		}
		targets = append(targets, t)
	}
	if err := o.Store.ReplaceTargets(req.RequestID, targets); err != nil {
		slog.Error("powerops: persist targets", "request", req.RequestID, "error", err)
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
	o.verify(ctx, req, machines, targets, action)
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

	for o.now().Before(deadline) {
		pending := false
		for _, t := range targets {
			if !t.Success || t.Verified || !verifiable(t.Provider) {
				continue
			}
			m := byMachine[t.NodeID]
			p, ok := o.Providers[t.Provider]
			if !ok || m == (Machine{}) {
				continue
			}
			st, err := p.Status(ctx, m)
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
	o.finish(req, targets, terminalState(action, targets))
}

// terminalState aggregates: succeeded = executed and confirmed (or provider
// cannot confirm at all); failed = the rest. all confirmed → done, none →
// failed, mixed → partial.
func terminalState(a Action, targets []*model.PowerRequestTarget) string {
	if a == ActionStatus {
		return StateDone
	}
	succeeded := 0
	for _, t := range targets {
		if t.Success && (t.Verified || !verifiable(t.Provider)) {
			succeeded++
		}
	}
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
	succeeded := 0
	for _, t := range targets {
		if t.Success {
			succeeded++
		}
	}
	now := o.now()
	req.State = state
	req.Succeeded = succeeded
	req.Failed = len(targets) - succeeded
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
