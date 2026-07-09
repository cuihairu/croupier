package approvals

import (
	"context"
	"strings"
	"testing"
	"time"
)

// stubDelegationStore satisfies DelegationStore with only the pieces
// exercised by DelegationService's getApproval / CanDelegate path. Other
// methods return zero values so the stub stays focused on the unit under
// test.
type stubDelegationStore struct {
	activeFor map[string][]*Delegation
	byID      map[string]*Delegation
	usageInc  int
}

func (s *stubDelegationStore) Create(d *Delegation) (*Delegation, error) { return d, nil }
func (s *stubDelegationStore) Get(id string) (*Delegation, error) {
	if d, ok := s.byID[id]; ok {
		return d, nil
	}
	return nil, ErrDelegationNotFound
}
func (s *stubDelegationStore) Update(*Delegation) (*Delegation, error) { return nil, nil }
func (s *stubDelegationStore) Delete(string) error                     { return nil }
func (s *stubDelegationStore) List(DelegationFilter, Page) ([]*Delegation, int, error) {
	return nil, 0, nil
}
func (s *stubDelegationStore) GetActiveDelegationsForUser(userID string) ([]*Delegation, error) {
	return s.activeFor[userID], nil
}
func (s *stubDelegationStore) GetActiveDelegationsByUser(string) ([]*Delegation, error) {
	return nil, nil
}
func (s *stubDelegationStore) IncrementUsage(string) error {
	s.usageInc++
	return nil
}

func TestDelegationService_SetApprovalStore(t *testing.T) {
	s := NewDelegationService(nil, nil, nil)
	s.SetApprovalStore(NewMemStore())

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.approvalStore == nil {
		t.Fatal("expected approval store to be wired after setter call")
	}
}

func TestDelegationService_getApproval_NoStore(t *testing.T) {
	s := NewDelegationService(nil, nil, nil)
	_, err := s.getApproval("approval-1")
	if err == nil {
		t.Fatal("expected error when approval store is not configured")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestDelegationService_getApproval_WithStore(t *testing.T) {
	store := NewMemStore()
	seed := &Approval{
		ID:         "approval-1",
		State:      "pending",
		FunctionID: "fn-1",
		GameID:     "game-1",
		Env:        "prod",
	}
	if _, err := store.Create(seed); err != nil {
		t.Fatalf("seed approval: %v", err)
	}

	s := NewDelegationService(nil, nil, nil)
	s.SetApprovalStore(store)

	got, err := s.getApproval("approval-1")
	if err != nil {
		t.Fatalf("getApproval: %v", err)
	}
	if got.FunctionID != "fn-1" || got.GameID != "game-1" || got.Env != "prod" {
		t.Fatalf("returned approval lost fields: %+v", got)
	}
}

func TestDelegationService_getApproval_NotFound(t *testing.T) {
	s := NewDelegationService(nil, nil, nil)
	s.SetApprovalStore(NewMemStore())

	_, err := s.getApproval("missing")
	if err == nil {
		t.Fatal("expected error for missing approval")
	}
}

func TestDelegationService_CanDelegate_NoStore(t *testing.T) {
	s := NewDelegationService(&stubDelegationStore{}, nil, nil)
	ok, _, err := s.CanDelegate("delegate-1", "approval-1", PermApprove)
	if ok {
		t.Error("expected ok=false when approval store is not configured")
	}
	if err == nil {
		t.Error("expected error when approval store is not configured")
	}
}

func TestDelegationService_CanDelegate_ScopeFunctionMatch(t *testing.T) {
	approvalStore := NewMemStore()
	if _, err := approvalStore.Create(&Approval{
		ID:         "approval-1",
		State:      "pending",
		FunctionID: "fn-1",
		GameID:     "game-1",
		Env:        "prod",
	}); err != nil {
		t.Fatalf("seed approval: %v", err)
	}

	delegationStore := &stubDelegationStore{
		activeFor: map[string][]*Delegation{
			"delegate-1": {
				{
					ID:          "del-1",
					Delegator:   "boss",
					Delegate:    "delegate-1",
					Scope:       ScopeFunction,
					ScopeValue:  "fn-1",
					Permissions: []DelegationPermission{PermApprove},
					State:       DelegationStateActive,
					StartAt:     time.Now().Add(-time.Hour),
				},
			},
		},
	}

	s := NewDelegationService(delegationStore, nil, nil)
	s.SetApprovalStore(approvalStore)

	ok, _, err := s.CanDelegate("delegate-1", "approval-1", PermApprove)
	if err != nil {
		t.Fatalf("CanDelegate: %v", err)
	}
	if !ok {
		t.Error("expected delegation to match the function-scoped approval")
	}
}

func TestDelegationService_CanDelegate_ScopeFunctionMismatch(t *testing.T) {
	approvalStore := NewMemStore()
	if _, err := approvalStore.Create(&Approval{
		ID:         "approval-1",
		FunctionID: "fn-other",
		GameID:     "game-1",
		Env:        "prod",
	}); err != nil {
		t.Fatalf("seed approval: %v", err)
	}

	delegationStore := &stubDelegationStore{
		activeFor: map[string][]*Delegation{
			"delegate-1": {
				{
					ID:          "del-1",
					Delegator:   "boss",
					Delegate:    "delegate-1",
					Scope:       ScopeFunction,
					ScopeValue:  "fn-1",
					Permissions: []DelegationPermission{PermApprove},
					State:       DelegationStateActive,
					StartAt:     time.Now().Add(-time.Hour),
				},
			},
		},
	}

	s := NewDelegationService(delegationStore, nil, nil)
	s.SetApprovalStore(approvalStore)

	ok, _, err := s.CanDelegate("delegate-1", "approval-1", PermApprove)
	if err != nil {
		t.Fatalf("CanDelegate: %v", err)
	}
	if ok {
		t.Error("expected no match when scope value differs from approval function")
	}
}

func TestDelegationService_CreateDelegation_Notifies(t *testing.T) {
	recipient := ""
	sent := make(chan NotificationEvent, 1)

	notifier := &capturingNotifier{
		recipient: &recipient,
		sent:      sent,
	}

	store := &stubDelegationStore{byID: map[string]*Delegation{}}
	s := NewDelegationService(store, nil, notifier)

	_, err := s.CreateDelegation(context.Background(), &DelegationRequest{
		Delegator:   "boss",
		Delegate:    "delegate-1",
		Scope:       ScopeAll,
		Permissions: []DelegationPermission{PermApprove},
		Reason:      "vacation",
	})
	if err != nil {
		t.Fatalf("CreateDelegation: %v", err)
	}

	select {
	case event := <-sent:
		if event.Type != "delegation_created" {
			t.Errorf("event type = %q", event.Type)
		}
	case <-time.After(time.Second):
		t.Error("delegate never notified")
	}
}

// capturingNotifier captures a single recipient's notification so we can
// assert the delegation service actually fires notifications without pulling
// in the full MultiChannelNotifier.
type capturingNotifier struct {
	recipient *string
	sent      chan<- NotificationEvent
}

func (n *capturingNotifier) Notify(ctx context.Context, recipients []string, event NotificationEvent) error {
	if len(recipients) == 0 {
		return nil
	}
	*n.recipient = recipients[0]
	n.sent <- event
	return nil
}

func (n *capturingNotifier) NotifyWithChannels(ctx context.Context, recipients []string, event NotificationEvent, channels []NotificationChannel) error {
	return n.Notify(ctx, recipients, event)
}

// Compile-time assertion that capturingNotifier satisfies Notifier.
var _ Notifier = (*capturingNotifier)(nil)
