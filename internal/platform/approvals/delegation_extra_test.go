package approvals

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// configurableDelegationStore is a fake DelegationStore with injectable
// results and errors, used to exercise DelegationService branches.
type configurableDelegationStore struct {
	byID      map[string]*Delegation
	activeFor map[string][]*Delegation
	activeBy  map[string][]*Delegation
	listed    []*Delegation

	getErr        error
	createErr     error
	updateErr     error
	incrementErr  error
	listErr       error
	activeForErr  error
	activeByErr   error
	updated       []*Delegation
	incrementedID string
}

func (s *configurableDelegationStore) Create(d *Delegation) (*Delegation, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	if s.byID == nil {
		s.byID = map[string]*Delegation{}
	}
	s.byID[d.ID] = d
	return d, nil
}

func (s *configurableDelegationStore) Get(id string) (*Delegation, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if d, ok := s.byID[id]; ok {
		return d, nil
	}
	return nil, ErrDelegationNotFound
}

func (s *configurableDelegationStore) Update(d *Delegation) (*Delegation, error) {
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	s.updated = append(s.updated, d)
	return d, nil
}

func (s *configurableDelegationStore) Delete(string) error { return nil }

func (s *configurableDelegationStore) List(DelegationFilter, Page) ([]*Delegation, int, error) {
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	return s.listed, len(s.listed), nil
}

func (s *configurableDelegationStore) GetActiveDelegationsForUser(userID string) ([]*Delegation, error) {
	if s.activeForErr != nil {
		return nil, s.activeForErr
	}
	return s.activeFor[userID], nil
}

func (s *configurableDelegationStore) GetActiveDelegationsByUser(userID string) ([]*Delegation, error) {
	if s.activeByErr != nil {
		return nil, s.activeByErr
	}
	return s.activeBy[userID], nil
}

func (s *configurableDelegationStore) IncrementUsage(id string) error {
	s.incrementedID = id
	return s.incrementErr
}

func validDelegationRequest() *DelegationRequest {
	return &DelegationRequest{
		Delegator:   "boss",
		Delegate:    "delegate-1",
		Scope:       ScopeAll,
		Permissions: []DelegationPermission{PermApprove},
		Reason:      "vacation",
	}
}

func TestDelegationService_CreateDelegation_Validation(t *testing.T) {
	svc := NewDelegationService(&configurableDelegationStore{}, nil, nil)

	cases := []struct {
		name string
		req  *DelegationRequest
		want string
	}{
		{"missing delegator", &DelegationRequest{Delegate: "d", Permissions: []DelegationPermission{PermApprove}}, "delegator is required"},
		{"missing delegate", &DelegationRequest{Delegator: "a", Permissions: []DelegationPermission{PermApprove}}, "delegate is required"},
		{"self delegation", &DelegationRequest{Delegator: "a", Delegate: "a", Permissions: []DelegationPermission{PermApprove}}, "cannot delegate to yourself"},
		{"no permissions", &DelegationRequest{Delegator: "a", Delegate: "b"}, "at least one permission"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateDelegation(context.Background(), tc.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestDelegationService_CreateDelegation_SetsDuration(t *testing.T) {
	store := &configurableDelegationStore{}
	svc := NewDelegationService(store, nil, nil)

	created, err := svc.CreateDelegation(context.Background(), &DelegationRequest{
		Delegator:   "boss",
		Delegate:    "delegate-1",
		Scope:       ScopeAll,
		Permissions: []DelegationPermission{PermApprove},
		Duration:    2 * time.Hour,
		MaxUsages:   5,
	})
	require.NoError(t, err)
	require.NotNil(t, created.EndAt)
	assert.True(t, created.EndAt.After(time.Now()))
	assert.Equal(t, 5, created.MaxUsages)
	assert.Equal(t, DelegationStateActive, created.State)
}

func TestDelegationService_CreateDelegation_StoreError(t *testing.T) {
	svc := NewDelegationService(&configurableDelegationStore{createErr: errors.New("db down")}, nil, nil)

	_, err := svc.CreateDelegation(context.Background(), validDelegationRequest())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create delegation")
}

func TestDelegationService_CreateDelegation_Circular(t *testing.T) {
	// delegate-1 already delegated to boss, so boss -> delegate-1 closes a cycle.
	store := &configurableDelegationStore{
		activeBy: map[string][]*Delegation{
			"delegate-1": {{
				ID:        "del-existing",
				Delegator: "delegate-1",
				Delegate:  "boss",
				State:     DelegationStateActive,
			}},
		},
	}
	svc := NewDelegationService(store, nil, nil)

	_, err := svc.CreateDelegation(context.Background(), &DelegationRequest{
		Delegator:   "boss",
		Delegate:    "delegate-1",
		Scope:       ScopeAll,
		Permissions: []DelegationPermission{PermApprove},
	})
	require.ErrorIs(t, err, ErrCircularDelegation)
}

func TestDelegationService_CreateDelegation_CircularLookupErrorIsAllowed(t *testing.T) {
	store := &configurableDelegationStore{activeByErr: errors.New("lookup failed")}
	svc := NewDelegationService(store, nil, nil)

	_, err := svc.CreateDelegation(context.Background(), validDelegationRequest())
	require.NoError(t, err)
}

func TestDelegationService_checkCircularRecursive_VisitedGuards(t *testing.T) {
	// Build a chain that eventually revisits an already-visited delegation.
	shared := &Delegation{
		ID:        "del-shared",
		Delegator: "a",
		Delegate:  "boss", // direct cycle back to target
		State:     DelegationStateActive,
	}
	store := &configurableDelegationStore{
		activeBy: map[string][]*Delegation{
			"delegate-1": {
				{
					ID:        "del-1",
					Delegator: "delegate-1",
					Delegate:  "middle",
					State:     DelegationStateActive,
				},
				shared,
			},
			"middle": {shared}, // shared is revisited via another path
		},
	}
	svc := NewDelegationService(store, nil, nil)

	err := svc.checkCircularDelegation("boss", "delegate-1")
	require.ErrorIs(t, err, ErrCircularDelegation)
}

func TestDelegationService_RevokeDelegation_Branches(t *testing.T) {
	now := time.Now()

	t.Run("get error", func(t *testing.T) {
		svc := NewDelegationService(&configurableDelegationStore{getErr: errors.New("boom")}, nil, nil)
		_, err := svc.RevokeDelegation(context.Background(), "del-1", "boss", "")
		require.Error(t, err)
	})

	t.Run("not active", func(t *testing.T) {
		store := &configurableDelegationStore{byID: map[string]*Delegation{
			"del-1": {ID: "del-1", Delegator: "boss", State: DelegationStateRevoked},
		}}
		svc := NewDelegationService(store, nil, nil)
		_, err := svc.RevokeDelegation(context.Background(), "del-1", "boss", "")
		require.ErrorIs(t, err, ErrDelegationNotActive)
	})

	t.Run("only delegator can revoke", func(t *testing.T) {
		store := &configurableDelegationStore{byID: map[string]*Delegation{
			"del-1": {ID: "del-1", Delegator: "boss", State: DelegationStateActive},
		}}
		svc := NewDelegationService(store, nil, nil)
		_, err := svc.RevokeDelegation(context.Background(), "del-1", "someone-else", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only the delegator")
	})

	t.Run("update error", func(t *testing.T) {
		store := &configurableDelegationStore{
			byID: map[string]*Delegation{
				"del-1": {ID: "del-1", Delegator: "boss", Delegate: "delegate-1", State: DelegationStateActive},
			},
			updateErr: errors.New("db down"),
		}
		svc := NewDelegationService(store, nil, nil)
		_, err := svc.RevokeDelegation(context.Background(), "del-1", "boss", "policy")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to revoke delegation")
	})

	t.Run("success with notification", func(t *testing.T) {
		store := &configurableDelegationStore{byID: map[string]*Delegation{
			"del-1": {ID: "del-1", Delegator: "boss", Delegate: "delegate-1", State: DelegationStateActive, CreatedAt: now},
		}}
		var recipient string
		sent := make(chan NotificationEvent, 1)
		notifier := &capturingNotifier{recipient: &recipient, sent: sent}
		svc := NewDelegationService(store, nil, notifier)

		updated, err := svc.RevokeDelegation(context.Background(), "del-1", "boss", "no longer needed")
		require.NoError(t, err)
		assert.Equal(t, DelegationStateRevoked, updated.State)
		assert.Equal(t, "boss", updated.RevokedBy)
		assert.Equal(t, "delegate-1", recipient)

		select {
		case event := <-sent:
			assert.Equal(t, "delegation_revoked", event.Type)
		case <-time.After(time.Second):
			t.Fatal("delegate never notified of revocation")
		}
	})
}

func TestDelegationService_GetActiveDelegation_Branches(t *testing.T) {
	now := time.Now()
	expired := now.Add(-time.Minute)

	activeDelegation := func(mutate func(*Delegation)) *Delegation {
		d := &Delegation{
			ID:          "del-1",
			Delegator:   "boss",
			Delegate:    "delegate-1",
			Scope:       ScopeAll,
			Permissions: []DelegationPermission{PermApprove},
			State:       DelegationStateActive,
		}
		if mutate != nil {
			mutate(d)
		}
		return d
	}

	cases := []struct {
		name      string
		items     []*Delegation
		wantErr   error
		wantFound bool
	}{
		{"store error", nil, errors.New("db down"), false},
		{"other delegator", []*Delegation{activeDelegation(func(d *Delegation) { d.Delegator = "someone" })}, nil, false},
		{"inactive state", []*Delegation{activeDelegation(func(d *Delegation) { d.State = DelegationStateRevoked })}, nil, false},
		{"expired", []*Delegation{activeDelegation(func(d *Delegation) { d.EndAt = &expired })}, nil, false},
		{"usage exhausted", []*Delegation{activeDelegation(func(d *Delegation) { d.MaxUsages = 1; d.UsageCount = 1 })}, nil, false},
		{"scope mismatch", []*Delegation{activeDelegation(func(d *Delegation) { d.Scope = ScopeFunction; d.ScopeValue = "fn-other" })}, nil, false},
		{"constraint blocks", []*Delegation{activeDelegation(func(d *Delegation) {
			d.Constraints = []DelegationConstraint{{
				Type:     "time_restriction",
				Value:    map[string]interface{}{"allowed_days": []interface{}{float64(int(now.Weekday()) + 7)}},
				Enforced: true,
			}}
		})}, nil, false},
		{"match", []*Delegation{activeDelegation(nil)}, nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &configurableDelegationStore{activeFor: map[string][]*Delegation{"delegate-1": tc.items}}
			if tc.name == "store error" {
				store.activeForErr = tc.wantErr
			}
			svc := NewDelegationService(store, nil, nil)

			got, err := svc.GetActiveDelegation("boss", "delegate-1", ScopeAll, "")
			if !tc.wantFound {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "del-1", got.ID)
		})
	}
}

func TestDelegationService_GetActiveDelegation_ScopeMatching(t *testing.T) {
	newStore := func(d *Delegation) *configurableDelegationStore {
		return &configurableDelegationStore{activeFor: map[string][]*Delegation{"delegate-1": {d}}}
	}
	svcFor := func(d *Delegation) *DelegationService {
		return NewDelegationService(newStore(d), nil, nil)
	}

	// Wildcard scope values match anything within the same scope type.
	got, err := svcFor(&Delegation{ID: "w", Delegator: "boss", Delegate: "delegate-1", Scope: ScopeFunction, ScopeValue: "*", State: DelegationStateActive}).
		GetActiveDelegation("boss", "delegate-1", ScopeFunction, "fn-any")
	require.NoError(t, err)
	assert.Equal(t, "w", got.ID)

	got, err = svcFor(&Delegation{ID: "e", Delegator: "boss", Delegate: "delegate-1", Scope: ScopeFunction, ScopeValue: "", State: DelegationStateActive}).
		GetActiveDelegation("boss", "delegate-1", ScopeFunction, "fn-any")
	require.NoError(t, err)
	assert.Equal(t, "e", got.ID)

	// Different scope type never matches.
	_, err = svcFor(&Delegation{ID: "g", Delegator: "boss", Delegate: "delegate-1", Scope: ScopeGame, ScopeValue: "game-1", State: DelegationStateActive}).
		GetActiveDelegation("boss", "delegate-1", ScopeFunction, "game-1")
	assert.ErrorIs(t, err, ErrDelegationNotFound)
}

func TestDelegationService_CanDelegate_Branches(t *testing.T) {
	approvalStore := NewMemStore()
	_, err := approvalStore.Create(&Approval{ID: "approval-1", State: "pending", FunctionID: "fn-1", GameID: "game-1", Env: "prod"})
	require.NoError(t, err)

	now := time.Now()
	expired := now.Add(-time.Minute)
	future := now.Add(time.Hour)

	cases := []struct {
		name  string
		items []*Delegation
		want  bool
	}{
		{"inactive", []*Delegation{{ID: "d", Delegator: "boss", Delegate: "delegate-1", Scope: ScopeAll, Permissions: []DelegationPermission{PermApprove}, State: DelegationStateRevoked}}, false},
		{"expired", []*Delegation{{ID: "d", Delegator: "boss", Delegate: "delegate-1", Scope: ScopeAll, Permissions: []DelegationPermission{PermApprove}, State: DelegationStateActive, EndAt: &expired}}, false},
		{"usage exhausted", []*Delegation{{ID: "d", Delegator: "boss", Delegate: "delegate-1", Scope: ScopeAll, Permissions: []DelegationPermission{PermApprove}, State: DelegationStateActive, MaxUsages: 1, UsageCount: 1}}, false},
		{"missing permission", []*Delegation{{ID: "d", Delegator: "boss", Delegate: "delegate-1", Scope: ScopeAll, Permissions: []DelegationPermission{PermView}, State: DelegationStateActive}}, false},
		{"env scope mismatch", []*Delegation{{ID: "d", Delegator: "boss", Delegate: "delegate-1", Scope: ScopeEnv, ScopeValue: "staging", Permissions: []DelegationPermission{PermApprove}, State: DelegationStateActive}}, false},
		{"unknown scope", []*Delegation{{ID: "d", Delegator: "boss", Delegate: "delegate-1", Scope: DelegationScope("weird"), Permissions: []DelegationPermission{PermApprove}, State: DelegationStateActive}}, false},
		{"wildcard game scope", []*Delegation{{ID: "d", Delegator: "boss", Delegate: "delegate-1", Scope: ScopeGame, ScopeValue: "", Permissions: []DelegationPermission{PermApprove}, State: DelegationStateActive}}, true},
		{"env scope match", []*Delegation{{ID: "d", Delegator: "boss", Delegate: "delegate-1", Scope: ScopeEnv, ScopeValue: "prod", Permissions: []DelegationPermission{PermApprove}, State: DelegationStateActive}}, true},
		{"future end", []*Delegation{{ID: "d", Delegator: "boss", Delegate: "delegate-1", Scope: ScopeAll, Permissions: []DelegationPermission{PermApprove}, State: DelegationStateActive, EndAt: &future}}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &configurableDelegationStore{activeFor: map[string][]*Delegation{"delegate-1": tc.items}}
			svc := NewDelegationService(store, nil, nil)
			svc.SetApprovalStore(approvalStore)

			ok, delegation, err := svc.CanDelegate("delegate-1", "approval-1", PermApprove)
			require.NoError(t, err)
			assert.Equal(t, tc.want, ok)
			if tc.want {
				require.NotNil(t, delegation)
			} else {
				assert.Nil(t, delegation)
			}
		})
	}
}

func TestDelegationService_CanDelegate_StoreErrors(t *testing.T) {
	svc := NewDelegationService(&configurableDelegationStore{activeForErr: errors.New("db down")}, nil, nil)
	svc.SetApprovalStore(NewMemStore())
	_, _, err := svc.CanDelegate("delegate-1", "approval-1", PermApprove)
	require.Error(t, err)
}

func TestDelegationService_UseDelegation_Branches(t *testing.T) {
	now := time.Now()
	expired := now.Add(-time.Minute)

	t.Run("not found", func(t *testing.T) {
		svc := NewDelegationService(&configurableDelegationStore{}, nil, nil)
		err := svc.UseDelegation(context.Background(), "missing")
		require.ErrorIs(t, err, ErrDelegationNotFound)
	})

	t.Run("not active", func(t *testing.T) {
		svc := NewDelegationService(&configurableDelegationStore{byID: map[string]*Delegation{
			"d": {ID: "d", State: DelegationStateCompleted},
		}}, nil, nil)
		err := svc.UseDelegation(context.Background(), "d")
		require.ErrorIs(t, err, ErrDelegationNotActive)
	})

	t.Run("expired", func(t *testing.T) {
		svc := NewDelegationService(&configurableDelegationStore{byID: map[string]*Delegation{
			"d": {ID: "d", State: DelegationStateActive, EndAt: &expired},
		}}, nil, nil)
		err := svc.UseDelegation(context.Background(), "d")
		require.ErrorIs(t, err, ErrDelegationExpired)
	})

	t.Run("usage limit reached", func(t *testing.T) {
		svc := NewDelegationService(&configurableDelegationStore{byID: map[string]*Delegation{
			"d": {ID: "d", State: DelegationStateActive, MaxUsages: 2, UsageCount: 2},
		}}, nil, nil)
		err := svc.UseDelegation(context.Background(), "d")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "usage limit reached")
	})

	t.Run("increment error", func(t *testing.T) {
		svc := NewDelegationService(&configurableDelegationStore{
			byID:         map[string]*Delegation{"d": {ID: "d", State: DelegationStateActive}},
			incrementErr: errors.New("db down"),
		}, nil, nil)
		err := svc.UseDelegation(context.Background(), "d")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to increment usage")
	})

	t.Run("success", func(t *testing.T) {
		store := &configurableDelegationStore{byID: map[string]*Delegation{"d": {ID: "d", State: DelegationStateActive}}}
		svc := NewDelegationService(store, nil, nil)
		require.NoError(t, svc.UseDelegation(context.Background(), "d"))
		assert.Equal(t, "d", store.incrementedID)
	})
}

func TestDelegationService_GetUserDelegations(t *testing.T) {
	t.Run("by-user error", func(t *testing.T) {
		svc := NewDelegationService(&configurableDelegationStore{activeByErr: errors.New("x")}, nil, nil)
		_, _, err := svc.GetUserDelegations("u")
		require.Error(t, err)
	})

	t.Run("for-user error", func(t *testing.T) {
		svc := NewDelegationService(&configurableDelegationStore{activeForErr: errors.New("x")}, nil, nil)
		_, _, err := svc.GetUserDelegations("u")
		require.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		store := &configurableDelegationStore{
			activeBy:  map[string][]*Delegation{"u": {{ID: "out"}}},
			activeFor: map[string][]*Delegation{"u": {{ID: "in"}}},
		}
		svc := NewDelegationService(store, nil, nil)
		delegated, received, err := svc.GetUserDelegations("u")
		require.NoError(t, err)
		require.Len(t, delegated, 1)
		require.Len(t, received, 1)
		assert.Equal(t, "out", delegated[0].ID)
		assert.Equal(t, "in", received[0].ID)
	})
}

func TestDelegationService_CleanupExpiredDelegations(t *testing.T) {
	t.Run("list error", func(t *testing.T) {
		svc := NewDelegationService(&configurableDelegationStore{listErr: errors.New("x")}, nil, nil)
		_, err := svc.CleanupExpiredDelegations(context.Background())
		require.Error(t, err)
	})

	t.Run("marks only expired", func(t *testing.T) {
		now := time.Now()
		past := now.Add(-time.Hour)
		future := now.Add(time.Hour)
		store := &configurableDelegationStore{listed: []*Delegation{
			{ID: "expired-1", State: DelegationStateActive, EndAt: &past},
			{ID: "expired-2", State: DelegationStateActive, EndAt: &past},
			{ID: "live", State: DelegationStateActive, EndAt: nil},
			{ID: "future", State: DelegationStateActive, EndAt: &future},
		}}
		svc := NewDelegationService(store, nil, nil)

		count, err := svc.CleanupExpiredDelegations(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 2, count)
		require.Len(t, store.updated, 2)
		for _, d := range store.updated {
			assert.Equal(t, DelegationStateExpired, d.State)
		}
	})

	t.Run("update failure is skipped", func(t *testing.T) {
		past := time.Now().Add(-time.Hour)
		store := &configurableDelegationStore{
			listed:    []*Delegation{{ID: "expired-1", State: DelegationStateActive, EndAt: &past}},
			updateErr: errors.New("db down"),
		}
		svc := NewDelegationService(store, nil, nil)
		count, err := svc.CleanupExpiredDelegations(context.Background())
		require.NoError(t, err)
		assert.Zero(t, count)
	})
}

func TestDelegationService_GetDelegationChain(t *testing.T) {
	now := time.Now()
	store := &configurableDelegationStore{
		activeFor: map[string][]*Delegation{
			"delegate-2": {{ID: "d2", Delegator: "delegate-1", Delegate: "delegate-2", CreatedAt: now}},
			"delegate-1": {{ID: "d1", Delegator: "boss", Delegate: "delegate-1", CreatedAt: now}},
			"boss":       {{ID: "d0", Delegator: "delegate-2", Delegate: "boss", CreatedAt: now}}, // cycle guard
		},
	}
	svc := NewDelegationService(store, nil, nil)

	chain, err := svc.GetDelegationChain("delegate-2")
	require.NoError(t, err)
	require.NotNil(t, chain)
	assert.Equal(t, "delegate-2", chain.CurrentDelegate)
	// The visited set stops the walk once delegate-2 reappears, so the chain
	// keeps the full recorded path.
	assert.Equal(t, "delegate-2", chain.OriginalDelegator)
	assert.Equal(t, 3, chain.TotalDepth)
	require.Len(t, chain.Chain, 3)
	assert.Equal(t, "delegate-2", chain.Chain[0].Delegator)
	assert.Equal(t, "boss", chain.Chain[1].Delegator)
	assert.Equal(t, "delegate-1", chain.Chain[2].Delegator)
}

func TestDelegationService_CheckTimeRestriction(t *testing.T) {
	svc := NewDelegationService(&configurableDelegationStore{}, nil, nil)
	now := time.Now()

	t.Run("non-map value allows", func(t *testing.T) {
		assert.True(t, svc.checkTimeRestriction("not-a-map", now))
	})

	t.Run("day not allowed", func(t *testing.T) {
		assert.False(t, svc.checkTimeRestriction(map[string]interface{}{
			"allowed_days": []interface{}{float64(int(now.Weekday()+1) % 7)},
		}, now))
	})

	t.Run("day allowed without time window", func(t *testing.T) {
		assert.True(t, svc.checkTimeRestriction(map[string]interface{}{
			"allowed_days": []interface{}{float64(int(now.Weekday()))},
		}, now))
	})

	t.Run("outside time window", func(t *testing.T) {
		assert.False(t, svc.checkTimeRestriction(map[string]interface{}{
			"allowed_start": "00:00",
			"allowed_end":   "01:00",
		}, now))
	})

	t.Run("inside time window", func(t *testing.T) {
		current := now.Format("15:04")
		assert.True(t, svc.checkTimeRestriction(map[string]interface{}{
			"allowed_start": "00:00",
			"allowed_end":   current,
		}, now))
	})
}

func TestDelegationService_MatchesApprovalScopeAndPermission(t *testing.T) {
	svc := NewDelegationService(&configurableDelegationStore{}, nil, nil)
	approval := &Approval{FunctionID: "fn-1", GameID: "game-1", Env: "prod"}

	assert.True(t, svc.matchesApprovalScope(&Delegation{Scope: ScopeAll}, approval))
	assert.True(t, svc.matchesApprovalScope(&Delegation{Scope: ScopeFunction, ScopeValue: "fn-1"}, approval))
	assert.True(t, svc.matchesApprovalScope(&Delegation{Scope: ScopeFunction}, approval))
	assert.False(t, svc.matchesApprovalScope(&Delegation{Scope: ScopeFunction, ScopeValue: "fn-2"}, approval))
	assert.True(t, svc.matchesApprovalScope(&Delegation{Scope: ScopeGame, ScopeValue: "game-1"}, approval))
	assert.True(t, svc.matchesApprovalScope(&Delegation{Scope: ScopeEnv, ScopeValue: "prod"}, approval))
	assert.False(t, svc.matchesApprovalScope(&Delegation{Scope: ScopeEnv, ScopeValue: "dev"}, approval))
	assert.False(t, svc.matchesApprovalScope(&Delegation{Scope: ScopeWorkflow}, approval))

	d := &Delegation{Permissions: []DelegationPermission{PermApprove, PermReject}}
	assert.True(t, svc.hasPermission(d, PermApprove))
	assert.False(t, svc.hasPermission(d, PermDelegate))
}
