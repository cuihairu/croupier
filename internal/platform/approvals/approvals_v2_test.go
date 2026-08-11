package approvals

import (
	"context"
	"testing"
	"time"
)

// ── model.go helpers ──────────────────────────────────────────────

func TestEncodeMetadataJSON_Empty(t *testing.T) {
	if got := encodeMetadataJSON(nil); got != nil {
		t.Errorf("nil map → expected nil, got %v", got)
	}
	if got := encodeMetadataJSON(map[string]string{}); got != nil {
		t.Errorf("empty map → expected nil, got %v", got)
	}
}

func TestDecodeMetadataJSON_Empty(t *testing.T) {
	if got := decodeMetadataJSON(nil); got != nil {
		t.Errorf("nil bytes → expected nil, got %v", got)
	}
	if got := decodeMetadataJSON([]byte{}); got != nil {
		t.Errorf("empty bytes → expected nil, got %v", got)
	}
}

func TestDecodeMetadataJSON_InvalidJSON(t *testing.T) {
	if got := decodeMetadataJSON([]byte("{bad")); got != nil {
		t.Errorf("invalid JSON → expected nil, got %v", got)
	}
}

func TestDecodeMetadataJSON_EmptyMap(t *testing.T) {
	if got := decodeMetadataJSON([]byte("{}")); got != nil {
		t.Errorf("empty JSON object → expected nil, got %v", got)
	}
}

func TestEncodeDecodeMetadataRoundTrip(t *testing.T) {
	m := map[string]string{"a": "1", "b": "2"}
	raw := encodeMetadataJSON(m)
	got := decodeMetadataJSON(raw)
	if len(got) != 2 || got["a"] != "1" || got["b"] != "2" {
		t.Errorf("round-trip failed: %v", got)
	}
}

func TestToApproval_ModelWithMetadata(t *testing.T) {
	raw := []byte(`{"k":"v"}`)
	model := &ApprovalModel{
		ID:           "x",
		MetadataJSON: raw,
	}
	a := model.ToApproval()
	if a.Metadata["k"] != "v" {
		t.Errorf("expected metadata k=v, got %v", a.Metadata)
	}
}

func TestFromApproval_WithMetadata(t *testing.T) {
	a := &Approval{
		ID:       "x",
		Metadata: map[string]string{"k": "v"},
	}
	model := FromApproval(a)
	if model == nil {
		t.Fatal("expected non-nil")
	}
	// round-trip
	a2 := model.ToApproval()
	if a2.Metadata["k"] != "v" {
		t.Errorf("round-trip metadata failed")
	}
}

// ── store.go MemStore ─────────────────────────────────────────────

func TestMemStore_Update(t *testing.T) {
	s := NewMemStore()
	_, _ = s.Create(&Approval{ID: "a1", State: "pending"})
	a := &Approval{ID: "a1", State: "updated"}
	got, err := s.Update(a)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "updated" {
		t.Errorf("state = %q", got.State)
	}
}

func TestMemStore_Update_Nil(t *testing.T) {
	s := NewMemStore()
	_, err := s.Update(nil)
	if err == nil {
		t.Error("expected error for nil")
	}
}

func TestMemStore_Update_EmptyID(t *testing.T) {
	s := NewMemStore()
	_, err := s.Update(&Approval{})
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestMemStore_Update_NotFound(t *testing.T) {
	s := NewMemStore()
	_, err := s.Update(&Approval{ID: "missing"})
	if err == nil {
		t.Error("expected error for missing ID")
	}
}

func TestMemStore_Create_Nil(t *testing.T) {
	s := NewMemStore()
	_, err := s.Create(nil)
	if err == nil {
		t.Error("expected error for nil")
	}
}

func TestMemStore_Create_EmptyID(t *testing.T) {
	s := NewMemStore()
	_, err := s.Create(&Approval{})
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestMemStore_Create_DuplicateID(t *testing.T) {
	s := NewMemStore()
	_, _ = s.Create(&Approval{ID: "a1", State: "pending"})
	_, err := s.Create(&Approval{ID: "a1", State: "pending"})
	if err == nil {
		t.Error("expected error for duplicate ID")
	}
}

func TestMemStore_Approve_NotFound(t *testing.T) {
	s := NewMemStore()
	_, err := s.Approve("missing")
	if err == nil {
		t.Error("expected error")
	}
}

func TestMemStore_Reject_NotFound(t *testing.T) {
	s := NewMemStore()
	_, err := s.Reject("missing", "reason")
	if err == nil {
		t.Error("expected error")
	}
}

func TestMemStore_Get_NotFound(t *testing.T) {
	s := NewMemStore()
	_, err := s.Get("missing")
	if err == nil {
		t.Error("expected error")
	}
}

func TestMemStore_List_FilterByFunctionID(t *testing.T) {
	s := NewMemStore()
	_, _ = s.Create(&Approval{ID: "a1", FunctionID: "fn1", Actor: "u1"})
	_, _ = s.Create(&Approval{ID: "a2", FunctionID: "fn2", Actor: "u1"})
	list, total, err := s.List(Filter{FunctionID: "fn1"}, Page{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || list[0].ID != "a1" {
		t.Errorf("expected 1 result for fn1, got %d", total)
	}
}

func TestMemStore_List_FilterByGameID(t *testing.T) {
	s := NewMemStore()
	_, _ = s.Create(&Approval{ID: "a1", GameID: "g1"})
	_, _ = s.Create(&Approval{ID: "a2", GameID: "g2"})
	list, _, _ := s.List(Filter{GameID: "g1"}, Page{})
	if len(list) != 1 || list[0].ID != "a1" {
		t.Error("filter by gameID failed")
	}
}

func TestMemStore_List_FilterByEnv(t *testing.T) {
	s := NewMemStore()
	_, _ = s.Create(&Approval{ID: "a1", Env: "prod"})
	_, _ = s.Create(&Approval{ID: "a2", Env: "dev"})
	list, _, _ := s.List(Filter{Env: "prod"}, Page{})
	if len(list) != 1 || list[0].ID != "a1" {
		t.Error("filter by env failed")
	}
}

func TestMemStore_List_FilterByMode(t *testing.T) {
	s := NewMemStore()
	_, _ = s.Create(&Approval{ID: "a1", Mode: "invoke"})
	_, _ = s.Create(&Approval{ID: "a2", Mode: "batch"})
	list, _, _ := s.List(Filter{Mode: "invoke"}, Page{})
	if len(list) != 1 || list[0].ID != "a1" {
		t.Error("filter by mode failed")
	}
}

func TestMemStore_List_PaginationBeyondTotal(t *testing.T) {
	s := NewMemStore()
	_, _ = s.Create(&Approval{ID: "a1"})
	list, total, err := s.List(Filter{}, Page{Page: 100, Size: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 0 {
		t.Errorf("expected empty page, got %d items, total %d", len(list), total)
	}
}

// ── delegation.go ────────────────────────────────────────────────

// fullDelegationStore provides all methods needed for comprehensive delegation tests.
type fullDelegationStore struct {
	byDelegate map[string][]*Delegation
	byDelegat  map[string][]*Delegation
	byID       map[string]*Delegation
	data       map[string]*Delegation
}

func newFullDelegationStore() *fullDelegationStore {
	return &fullDelegationStore{
		byDelegate: map[string][]*Delegation{},
		byDelegat:  map[string][]*Delegation{},
		byID:       map[string]*Delegation{},
		data:       map[string]*Delegation{},
	}
}

func (s *fullDelegationStore) Create(d *Delegation) (*Delegation, error) {
	s.data[d.ID] = d
	return d, nil
}
func (s *fullDelegationStore) Get(id string) (*Delegation, error) {
	if d, ok := s.byID[id]; ok {
		return d, nil
	}
	return nil, ErrDelegationNotFound
}
func (s *fullDelegationStore) Update(d *Delegation) (*Delegation, error) {
	s.data[d.ID] = d
	s.byID[d.ID] = d
	return d, nil
}
func (s *fullDelegationStore) Delete(id string) error {
	delete(s.data, id)
	return nil
}
func (s *fullDelegationStore) List(f DelegationFilter, p Page) ([]*Delegation, int, error) {
	var out []*Delegation
	for _, d := range s.data {
		if f.State != "" && d.State != f.State {
			continue
		}
		out = append(out, d)
	}
	return out, len(out), nil
}
func (s *fullDelegationStore) GetActiveDelegationsForUser(userID string) ([]*Delegation, error) {
	return s.byDelegate[userID], nil
}
func (s *fullDelegationStore) GetActiveDelegationsByUser(userID string) ([]*Delegation, error) {
	return s.byDelegat[userID], nil
}
func (s *fullDelegationStore) IncrementUsage(id string) error {
	if d, ok := s.data[id]; ok {
		d.UsageCount++
	}
	return nil
}

func TestValidateDelegationRequest(t *testing.T) {
	svc := NewDelegationService(newFullDelegationStore(), nil, nil)

	tests := []struct {
		name    string
		req     *DelegationRequest
		wantErr string
	}{
		{"missing delegator", &DelegationRequest{Delegate: "b", Permissions: []DelegationPermission{PermApprove}}, "delegator is required"},
		{"missing delegate", &DelegationRequest{Delegator: "a", Permissions: []DelegationPermission{PermApprove}}, "delegate is required"},
		{"self delegation", &DelegationRequest{Delegator: "a", Delegate: "a", Permissions: []DelegationPermission{PermApprove}}, "cannot delegate to yourself"},
		{"no permissions", &DelegationRequest{Delegator: "a", Delegate: "b"}, "at least one permission"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateDelegation(context.Background(), tt.req)
			if err == nil || !contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestCheckCircularDelegation(t *testing.T) {
	ds := newFullDelegationStore()
	// b delegates to a (b→a), so when checking a→b we find circular
	ds.byDelegat["b"] = []*Delegation{
		{ID: "d1", Delegator: "b", Delegate: "a"},
	}
	svc := NewDelegationService(ds, nil, nil)
	err := svc.checkCircularDelegation("a", "b")
	if err == nil {
		t.Error("expected circular delegation error")
	}
}

func TestCheckCircularRecursive_Visited(t *testing.T) {
	svc := NewDelegationService(newFullDelegationStore(), nil, nil)
	visited := map[string]bool{"d1": true}
	err := svc.checkCircularRecursive("target", []*Delegation{{ID: "d1"}}, visited)
	if err != nil {
		t.Errorf("expected nil for visited delegation, got %v", err)
	}
}

func TestCheckCircularRecursive_StoresError(t *testing.T) {
	// Simulate store error by using stub that returns error
	errStore := &errorDelegationStore{}
	svc := NewDelegationService(errStore, nil, nil)
	err := svc.checkCircularDelegation("a", "b")
	if err != nil {
		t.Errorf("expected nil when store fails, got %v", err)
	}
}

type errorDelegationStore struct{}

func (e *errorDelegationStore) Create(d *Delegation) (*Delegation, error) { return d, nil }
func (e *errorDelegationStore) Get(string) (*Delegation, error)           { return nil, nil }
func (e *errorDelegationStore) Update(*Delegation) (*Delegation, error)   { return nil, nil }
func (e *errorDelegationStore) Delete(string) error                       { return nil }
func (e *errorDelegationStore) List(DelegationFilter, Page) ([]*Delegation, int, error) {
	return nil, 0, nil
}
func (e *errorDelegationStore) GetActiveDelegationsForUser(string) ([]*Delegation, error) {
	return nil, context.DeadlineExceeded
}
func (e *errorDelegationStore) GetActiveDelegationsByUser(string) ([]*Delegation, error) {
	return nil, context.DeadlineExceeded
}
func (e *errorDelegationStore) IncrementUsage(string) error { return nil }

func TestRevokeDelegation_Success(t *testing.T) {
	store := newFullDelegationStore()
	now := time.Now()
	d := &Delegation{
		ID:          "d1",
		Delegator:   "boss",
		Delegate:    "worker",
		State:       DelegationStateActive,
		Permissions: []DelegationPermission{PermApprove},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	store.byID["d1"] = d
	svc := NewDelegationService(store, nil, nil)

	got, err := svc.RevokeDelegation(context.Background(), "d1", "boss", "no longer needed")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != DelegationStateRevoked {
		t.Errorf("state = %q", got.State)
	}
	if got.RevokedBy != "boss" {
		t.Errorf("revokedBy = %q", got.RevokedBy)
	}
}

func TestRevokeDelegation_NotActive(t *testing.T) {
	store := newFullDelegationStore()
	d := &Delegation{ID: "d1", Delegator: "boss", State: DelegationStateRevoked}
	store.byID["d1"] = d
	svc := NewDelegationService(store, nil, nil)
	_, err := svc.RevokeDelegation(context.Background(), "d1", "boss", "reason")
	if err != ErrDelegationNotActive {
		t.Errorf("expected ErrDelegationNotActive, got %v", err)
	}
}

func TestRevokeDelegation_NotDelegator(t *testing.T) {
	store := newFullDelegationStore()
	d := &Delegation{ID: "d1", Delegator: "boss", State: DelegationStateActive}
	store.byID["d1"] = d
	svc := NewDelegationService(store, nil, nil)
	_, err := svc.RevokeDelegation(context.Background(), "d1", "other", "reason")
	if err == nil {
		t.Error("expected error when not delegator")
	}
}

func TestRevokeDelegation_StoreError(t *testing.T) {
	store := newFullDelegationStore()
	d := &Delegation{ID: "d1", Delegator: "boss", State: DelegationStateActive}
	store.byID["d1"] = d
	// Override Update to fail
	svc := NewDelegationService(&failingUpdateStore{store}, nil, nil)
	_, err := svc.RevokeDelegation(context.Background(), "d1", "boss", "reason")
	if err == nil {
		t.Error("expected error from store")
	}
}

type failingUpdateStore struct {
	*fullDelegationStore
}

func (f *failingUpdateStore) Update(d *Delegation) (*Delegation, error) {
	return nil, context.DeadlineExceeded
}

func TestRevokeDelegation_WithNotifier(t *testing.T) {
	store := newFullDelegationStore()
	d := &Delegation{ID: "d1", Delegator: "boss", Delegate: "worker", State: DelegationStateActive}
	store.byID["d1"] = d
	sent := make(chan NotificationEvent, 1)
	recipient := ""
	notifier := &capturingNotifier{recipient: &recipient, sent: sent}

	svc := NewDelegationService(store, nil, notifier)
	_, _ = svc.RevokeDelegation(context.Background(), "d1", "boss", "reason")

	select {
	case ev := <-sent:
		if ev.Type != "delegation_revoked" {
			t.Errorf("event type = %q", ev.Type)
		}
	case <-time.After(time.Second):
		t.Error("notifier not called")
	}
}

func TestGetActiveDelegation(t *testing.T) {
	store := newFullDelegationStore()
	now := time.Now()
	future := now.Add(time.Hour)
	d := &Delegation{
		ID:          "d1",
		Delegator:   "boss",
		Delegate:    "worker",
		Scope:       ScopeAll,
		State:       DelegationStateActive,
		EndAt:       &future,
		Permissions: []DelegationPermission{PermApprove},
	}
	store.byDelegate["worker"] = []*Delegation{d}

	svc := NewDelegationService(store, nil, nil)
	got, err := svc.GetActiveDelegation("boss", "worker", ScopeAll, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "d1" {
		t.Errorf("expected d1, got %v", got.ID)
	}
}

func TestGetActiveDelegation_NotFound(t *testing.T) {
	store := newFullDelegationStore()
	svc := NewDelegationService(store, nil, nil)
	_, err := svc.GetActiveDelegation("boss", "nobody", ScopeAll, "")
	if err != ErrDelegationNotFound {
		t.Errorf("expected ErrDelegationNotFound, got %v", err)
	}
}

func TestGetActiveDelegation_Expired(t *testing.T) {
	store := newFullDelegationStore()
	past := time.Now().Add(-time.Hour)
	d := &Delegation{
		ID:        "d1",
		Delegator: "boss",
		Delegate:  "worker",
		Scope:     ScopeAll,
		State:     DelegationStateActive,
		EndAt:     &past,
	}
	store.byDelegate["worker"] = []*Delegation{d}

	svc := NewDelegationService(store, nil, nil)
	_, err := svc.GetActiveDelegation("boss", "worker", ScopeAll, "")
	if err != ErrDelegationNotFound {
		t.Errorf("expected ErrDelegationNotFound for expired delegation, got %v", err)
	}
}

func TestGetActiveDelegation_MaxUsagesReached(t *testing.T) {
	store := newFullDelegationStore()
	future := time.Now().Add(time.Hour)
	d := &Delegation{
		ID:          "d1",
		Delegator:   "boss",
		Delegate:    "worker",
		Scope:       ScopeAll,
		State:       DelegationStateActive,
		EndAt:       &future,
		MaxUsages:   1,
		UsageCount:  1,
		Permissions: []DelegationPermission{PermApprove},
	}
	store.byDelegate["worker"] = []*Delegation{d}
	svc := NewDelegationService(store, nil, nil)
	_, err := svc.GetActiveDelegation("boss", "worker", ScopeAll, "")
	if err != ErrDelegationNotFound {
		t.Errorf("expected ErrDelegationNotFound when max usages reached, got %v", err)
	}
}

func TestMatchesScope(t *testing.T) {
	svc := NewDelegationService(nil, nil, nil)

	d := &Delegation{Scope: ScopeAll}
	if !svc.matchesScope(d, ScopeFunction, "fn1") {
		t.Error("ScopeAll should match everything")
	}

	d = &Delegation{Scope: ScopeFunction, ScopeValue: "fn1"}
	if !svc.matchesScope(d, ScopeFunction, "fn1") {
		t.Error("should match same function")
	}
	if svc.matchesScope(d, ScopeFunction, "fn2") {
		t.Error("should not match different function")
	}

	d = &Delegation{Scope: ScopeFunction, ScopeValue: ""}
	if !svc.matchesScope(d, ScopeFunction, "fn1") {
		t.Error("empty scope value should match all")
	}

	d = &Delegation{Scope: ScopeGame, ScopeValue: "g1"}
	if svc.matchesScope(d, ScopeFunction, "fn1") {
		t.Error("game scope should not match function")
	}
}

func TestMatchesApprovalScope(t *testing.T) {
	svc := NewDelegationService(nil, nil, nil)
	a := &Approval{FunctionID: "fn1", GameID: "g1", Env: "prod"}

	tests := []struct {
		scope    DelegationScope
		val      string
		expected bool
	}{
		{ScopeAll, "", true},
		{ScopeFunction, "fn1", true},
		{ScopeFunction, "fn2", false},
		{ScopeFunction, "", true},
		{ScopeGame, "g1", true},
		{ScopeGame, "g2", false},
		{ScopeEnv, "prod", true},
		{ScopeEnv, "dev", false},
		{ScopeWorkflow, "w1", false},
	}
	for _, tt := range tests {
		d := &Delegation{Scope: tt.scope, ScopeValue: tt.val}
		got := svc.matchesApprovalScope(d, a)
		if got != tt.expected {
			t.Errorf("scope=%s val=%q: got %v, want %v", tt.scope, tt.val, got, tt.expected)
		}
	}
}

func TestHasPermission(t *testing.T) {
	svc := NewDelegationService(nil, nil, nil)
	d := &Delegation{Permissions: []DelegationPermission{PermApprove, PermView}}
	if !svc.hasPermission(d, PermApprove) {
		t.Error("expected hasPerm approve")
	}
	if !svc.hasPermission(d, PermView) {
		t.Error("expected hasPerm view")
	}
	if svc.hasPermission(d, PermReject) {
		t.Error("expected !hasPerm reject")
	}
}

func TestCheckConstraints_NoConstraints(t *testing.T) {
	svc := NewDelegationService(nil, nil, nil)
	d := &Delegation{}
	if !svc.checkConstraints(d, time.Now()) {
		t.Error("empty constraints should pass")
	}
}

func TestCheckConstraints_Unenforced(t *testing.T) {
	svc := NewDelegationService(nil, nil, nil)
	d := &Delegation{
		Constraints: []DelegationConstraint{
			{Type: "time_restriction", Enforced: false},
		},
	}
	if !svc.checkConstraints(d, time.Now()) {
		t.Error("unenforced constraint should pass")
	}
}

func TestCheckTimeRestriction_NonMapValue(t *testing.T) {
	svc := NewDelegationService(nil, nil, nil)
	if !svc.checkTimeRestriction("not a map", time.Now()) {
		t.Error("non-map value should return true")
	}
}

func TestCheckTimeRestriction_AllowedDay(t *testing.T) {
	svc := NewDelegationService(nil, nil, nil)
	now := time.Now()
	weekday := int(now.Weekday())
	value := map[string]interface{}{
		"allowed_days": []interface{}{float64(weekday)},
	}
	if !svc.checkTimeRestriction(value, now) {
		t.Error("current day should be allowed")
	}
}

func TestCheckTimeRestriction_DisallowedDay(t *testing.T) {
	svc := NewDelegationService(nil, nil, nil)
	now := time.Now()
	weekday := int(now.Weekday())
	wrongDay := (weekday + 1) % 7
	value := map[string]interface{}{
		"allowed_days": []interface{}{float64(wrongDay)},
	}
	if svc.checkTimeRestriction(value, now) {
		t.Error("wrong day should be disallowed")
	}
}

func TestCheckTimeRestriction_TimeRange(t *testing.T) {
	svc := NewDelegationService(nil, nil, nil)
	now := time.Now()
	currentTime := now.Format("15:04")
	// Range that includes current time
	value := map[string]interface{}{
		"allowed_start": "00:00",
		"allowed_end":   "23:59",
	}
	if !svc.checkTimeRestriction(value, now) {
		t.Error("current time should be in range")
	}
	// Range that excludes current time
	value2 := map[string]interface{}{
		"allowed_start": "23:00",
		"allowed_end":   "23:30",
	}
	if currentTime >= "23:00" && currentTime <= "23:30" {
		// If we're actually in the range, skip
		t.Skip("current time in range")
	}
	if svc.checkTimeRestriction(value2, now) {
		t.Error("time outside range should be disallowed")
	}
}

func TestUseDelegation_Success(t *testing.T) {
	store := newFullDelegationStore()
	future := time.Now().Add(time.Hour)
	d := &Delegation{ID: "d1", State: DelegationStateActive, EndAt: &future, MaxUsages: 10, UsageCount: 0}
	store.byID["d1"] = d
	store.data["d1"] = d
	svc := NewDelegationService(store, nil, nil)
	err := svc.UseDelegation(context.Background(), "d1")
	if err != nil {
		t.Fatal(err)
	}
	if d.UsageCount != 1 {
		t.Errorf("usage = %d, want 1", d.UsageCount)
	}
}

func TestUseDelegation_NotActive(t *testing.T) {
	store := newFullDelegationStore()
	store.byID["d1"] = &Delegation{ID: "d1", State: DelegationStateRevoked}
	svc := NewDelegationService(store, nil, nil)
	err := svc.UseDelegation(context.Background(), "d1")
	if err != ErrDelegationNotActive {
		t.Errorf("expected ErrDelegationNotActive, got %v", err)
	}
}

func TestUseDelegation_Expired(t *testing.T) {
	store := newFullDelegationStore()
	past := time.Now().Add(-time.Hour)
	store.byID["d1"] = &Delegation{ID: "d1", State: DelegationStateActive, EndAt: &past}
	svc := NewDelegationService(store, nil, nil)
	err := svc.UseDelegation(context.Background(), "d1")
	if err != ErrDelegationExpired {
		t.Errorf("expected ErrDelegationExpired, got %v", err)
	}
}

func TestUseDelegation_MaxUsagesReached(t *testing.T) {
	store := newFullDelegationStore()
	future := time.Now().Add(time.Hour)
	store.byID["d1"] = &Delegation{ID: "d1", State: DelegationStateActive, EndAt: &future, MaxUsages: 1, UsageCount: 1}
	svc := NewDelegationService(store, nil, nil)
	err := svc.UseDelegation(context.Background(), "d1")
	if err == nil {
		t.Error("expected error for max usages reached")
	}
}

func TestListDelegations(t *testing.T) {
	store := newFullDelegationStore()
	svc := NewDelegationService(store, nil, nil)
	list, _, err := svc.ListDelegations(DelegationFilter{}, Page{})
	if err != nil {
		t.Fatal(err)
	}
	if list != nil {
		t.Error("expected empty list")
	}
}

func TestGetUserDelegations(t *testing.T) {
	store := newFullDelegationStore()
	store.byDelegat["u1"] = []*Delegation{{ID: "d1"}}
	store.byDelegate["u1"] = []*Delegation{{ID: "d2"}}
	svc := NewDelegationService(store, nil, nil)
	delegated, received, err := svc.GetUserDelegations("u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(delegated) != 1 || len(received) != 1 {
		t.Errorf("expected 1 each, got delegated=%d received=%d", len(delegated), len(received))
	}
}

func TestCleanupExpiredDelegations(t *testing.T) {
	store := newFullDelegationStore()
	past := time.Now().Add(-time.Hour)
	d := &Delegation{
		ID:    "d1",
		State: DelegationStateActive,
		EndAt: &past,
	}
	store.data["d1"] = d
	svc := NewDelegationService(store, nil, nil)
	count, err := svc.CleanupExpiredDelegations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 cleaned up, got %d", count)
	}
	if d.State != DelegationStateExpired {
		t.Errorf("expected expired state, got %q", d.State)
	}
}

func TestGetDelegationChain(t *testing.T) {
	store := newFullDelegationStore()
	now := time.Now()
	// c -> b -> a
	store.byDelegate["c"] = []*Delegation{
		{ID: "d1", Delegator: "b", Delegate: "c", CreatedAt: now.Add(-2 * time.Hour)},
	}
	store.byDelegate["b"] = []*Delegation{
		{ID: "d2", Delegator: "a", Delegate: "b", CreatedAt: now.Add(-1 * time.Hour)},
	}
	svc := NewDelegationService(store, nil, nil)
	chain, err := svc.GetDelegationChain("c")
	if err != nil {
		t.Fatal(err)
	}
	if chain.OriginalDelegator != "a" {
		t.Errorf("original delegator = %q, want a", chain.OriginalDelegator)
	}
	if chain.TotalDepth != 2 {
		t.Errorf("depth = %d, want 2", chain.TotalDepth)
	}
}

func TestGetDelegationChain_Empty(t *testing.T) {
	store := newFullDelegationStore()
	svc := NewDelegationService(store, nil, nil)
	chain, err := svc.GetDelegationChain("lonely")
	if err != nil {
		t.Fatal(err)
	}
	if chain.TotalDepth != 0 {
		t.Errorf("depth = %d, want 0", chain.TotalDepth)
	}
}

// ── notification.go ──────────────────────────────────────────────

func TestInAppNotifier_Send_NoStore(t *testing.T) {
	n := NewInAppNotifier(nil, nil)
	err := n.Send(context.Background(), "user1", NotificationEvent{Title: "Test"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildEmailBody_NoTitleNoMessage(t *testing.T) {
	body := buildEmailBody(NotificationEvent{Data: map[string]interface{}{"k": "v"}})
	if !contains(body, "<html>") {
		t.Error("expected HTML wrapper")
	}
	if !contains(body, "k") {
		t.Error("expected data key")
	}
}

func TestHtmlEscape(t *testing.T) {
	got := htmlEscape(`a"b'c<d>e&f`)
	// Verify special chars are escaped (appear as entities, not raw)
	if contains(got, `a"b`) {
		t.Errorf("double quote not escaped: %q", got)
	}
	if contains(got, "<d>") {
		t.Errorf("angle brackets not escaped: %q", got)
	}
	if !contains(got, "&amp;") {
		t.Errorf("ampersand not escaped: %q", got)
	}
}

// ── workflow.go extras ───────────────────────────────────────────

func TestMarshalJSON_WithCompletedAndExpires(t *testing.T) {
	now := time.Now()
	completed := now.Add(time.Hour)
	expires := now.Add(2 * time.Hour)
	inst := &WorkflowInstance{
		ID:          "x",
		StartedAt:   now,
		CompletedAt: &completed,
		ExpiresAt:   &expires,
	}
	data, err := inst.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !contains(s, "started_at") || !contains(s, "completed_at") || !contains(s, "expires_at") {
		t.Errorf("missing time fields: %s", s)
	}
}

func TestMarshalJSON_NilCompletedAndExpires(t *testing.T) {
	now := time.Now()
	inst := &WorkflowInstance{
		ID:        "x",
		StartedAt: now,
	}
	data, err := inst.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if contains(s, "completed_at") || contains(s, "expires_at") {
		t.Errorf("should not contain nil time fields: %s", s)
	}
}

func TestNow(t *testing.T) {
	timer := &realTimer{}
	now := timer.Now()
	if now.IsZero() {
		t.Error("expected non-zero time")
	}
}

func TestEvaluateCondition_UnknownOperator(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	engine := NewWorkflowEngine(store, approvalStore, nil)
	cond := Condition{Field: "x", Operator: ConditionOperator("unknown"), Value: "v"}
	data := map[string]interface{}{"x": "v"}
	if engine.evaluateCondition(cond, data) {
		t.Error("unknown operator should return false")
	}
}

func TestEvaluateCondition_FieldNotExists(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	engine := NewWorkflowEngine(store, approvalStore, nil)
	cond := Condition{Field: "missing", Operator: CondOpEquals, Value: "v"}
	if engine.evaluateCondition(cond, map[string]interface{}{}) {
		t.Error("missing field should return false")
	}
}

func TestIsStepComplete_ErrorFetchingApprovals(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	engine := NewWorkflowEngine(store, approvalStore, nil)
	step := ApprovalStep{ID: "s1", Type: StepTypeSequential, Approvers: []string{"u1"}}
	// Instance not in store → error
	inst := &WorkflowInstance{ID: "missing"}
	if engine.isStepComplete(step, inst) {
		t.Error("error fetching approvals should return false")
	}
}

func TestIsStepComplete_ErrorFetchingApprovals2(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	engine := NewWorkflowEngine(store, approvalStore, nil)
	step := ApprovalStep{ID: "s1", Type: StepTypeSequential, Approvers: []string{"u1"}}
	inst := &WorkflowInstance{ID: "missing"}
	_ = engine.isStepComplete(step, inst) // should not panic
}

func TestIsAuthorizedApprover_ErrorFetchingApprovals(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	engine := NewWorkflowEngine(store, approvalStore, nil)
	step := ApprovalStep{ID: "s1", Approvers: []string{"u1"}}
	inst := &WorkflowInstance{ID: "missing"} // not in store → error
	// Use user NOT in approvers list to hit the store error path
	if engine.isAuthorizedApprover(step, "u2", inst) {
		t.Error("should not be authorized when store errors")
	}
}

func TestProcessTimeouts_Escalate_NoStep(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	engine := NewWorkflowEngine(store, approvalStore, nil)

	def := &WorkflowDefinition{
		ID:     "esc-wf",
		Name:   "Escalation",
		Active: true,
		Steps: []ApprovalStep{
			{ID: "step1", Name: "Step1", Approvers: []string{"u1"}},
			{ID: "step2", Name: "Step2", Approvers: []string{"u2"}},
		},
	}
	_, _ = store.CreateDefinition(def)

	approval := &Approval{ID: "a1", State: "pending", Actor: "initiator"}
	_, _ = approvalStore.Create(approval)

	inst, _ := engine.StartWorkflow(context.Background(), "esc-wf", approval)
	// Set escalate to non-existent step, will fallback to reject
	// Modify the definition to have escalate_to pointing to a valid step
	def.Steps[0].TimeoutAction = "escalate"
	def.Steps[0].EscalateTo = "step2"
	_, _ = store.UpdateDefinition(def)

	// Set expiration in the past
	past := time.Now().Add(-time.Hour)
	inst.ExpiresAt = &past
	_, _ = store.UpdateInstance(inst)

	_, err := engine.ProcessTimeouts(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProcessTimeouts_Escalate_EmptyEscalateTo(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	engine := NewWorkflowEngine(store, approvalStore, nil)

	def := &WorkflowDefinition{
		ID:     "esc-wf2",
		Name:   "Escalation",
		Active: true,
		Steps: []ApprovalStep{
			{ID: "step1", Name: "Step1", Approvers: []string{"u1"}, TimeoutAction: "escalate", EscalateTo: ""},
		},
	}
	_, _ = store.CreateDefinition(def)

	approval := &Approval{ID: "a2", State: "pending", Actor: "initiator"}
	_, _ = approvalStore.Create(approval)

	inst, _ := engine.StartWorkflow(context.Background(), "esc-wf2", approval)
	past := time.Now().Add(-time.Hour)
	inst.ExpiresAt = &past
	_, _ = store.UpdateInstance(inst)

	processed, err := engine.ProcessTimeouts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(processed) != 1 {
		t.Errorf("expected 1 processed, got %d", len(processed))
	}
	// Should have fallen back to reject (expired)
	if processed[0].State != WorkflowStateExpired {
		t.Errorf("expected expired state, got %q", processed[0].State)
	}
}

func TestProcessTimeouts_Escalate_NilDefinition(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	engine := NewWorkflowEngine(store, approvalStore, nil)

	def := &WorkflowDefinition{
		ID:     "esc-wf3",
		Name:   "Escalation",
		Active: true,
		Steps: []ApprovalStep{
			{ID: "step1", Name: "Step1", Approvers: []string{"u1"}, TimeoutAction: "escalate", EscalateTo: "step2"},
			{ID: "step2", Name: "Step2", Approvers: []string{"u2"}},
		},
	}
	_, _ = store.CreateDefinition(def)

	approval := &Approval{ID: "a3", State: "pending", Actor: "initiator"}
	_, _ = approvalStore.Create(approval)

	inst, _ := engine.StartWorkflow(context.Background(), "esc-wf3", approval)
	past := time.Now().Add(-time.Hour)
	inst.ExpiresAt = &past
	inst.Definition = nil // Force definition lookup
	_, _ = store.UpdateInstance(inst)

	processed, err := engine.ProcessTimeouts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(processed) != 1 {
		t.Errorf("expected 1 processed, got %d", len(processed))
	}
}

func TestProcessTimeouts_DefaultActionReject(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	engine := NewWorkflowEngine(store, approvalStore, nil)

	def := &WorkflowDefinition{
		ID:     "default-wf",
		Name:   "Default Reject",
		Active: true,
		Steps: []ApprovalStep{
			{ID: "step1", Name: "Step1", Approvers: []string{"u1"}, TimeoutAction: "unknown_action"},
		},
	}
	_, _ = store.CreateDefinition(def)

	approval := &Approval{ID: "a4", State: "pending", Actor: "initiator"}
	_, _ = approvalStore.Create(approval)

	inst, _ := engine.StartWorkflow(context.Background(), "default-wf", approval)
	past := time.Now().Add(-time.Hour)
	inst.ExpiresAt = &past
	_, _ = store.UpdateInstance(inst)

	processed, err := engine.ProcessTimeouts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(processed) != 1 {
		t.Errorf("expected 1 processed, got %d", len(processed))
	}
}

func TestProcessTimeouts_DefinitionLookupError(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	engine := NewWorkflowEngine(store, approvalStore, nil)

	def := &WorkflowDefinition{
		ID:     "lookup-err-wf",
		Name:   "Lookup Error",
		Active: true,
		Steps: []ApprovalStep{
			{ID: "step1", Name: "Step1", Approvers: []string{"u1"}, TimeoutAction: "reject"},
		},
	}
	_, _ = store.CreateDefinition(def)

	approval := &Approval{ID: "a5", State: "pending", Actor: "initiator"}
	_, _ = approvalStore.Create(approval)

	inst, _ := engine.StartWorkflow(context.Background(), "lookup-err-wf", approval)
	past := time.Now().Add(-time.Hour)
	inst.ExpiresAt = &past
	inst.Definition = nil
	// Delete definition so GetDefinition fails
	_ = store.DeleteDefinition("lookup-err-wf")
	_, _ = store.UpdateInstance(inst)

	processed, err := engine.ProcessTimeouts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(processed) != 0 {
		t.Errorf("expected 0 processed (definition gone), got %d", len(processed))
	}
}

func TestNotifyApprovers_NilNotifier(t *testing.T) {
	engine := NewWorkflowEngine(nil, nil, nil)
	// Should not panic
	engine.notifyApprovers(context.Background(), &WorkflowInstance{}, &ApprovalStep{}, "approval_required")
}

func TestNotifyApprovers_Escalated(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	engine.notifyApprovers(context.Background(), &WorkflowInstance{ID: "x"}, &ApprovalStep{Approvers: []string{"u1"}, Name: "Step"}, "approval_escalated")
	if len(notifier.events) != 1 {
		t.Errorf("expected 1 event, got %d", len(notifier.events))
	}
}

func TestNotifyApprovers_Reminder(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	engine.notifyApprovers(context.Background(), &WorkflowInstance{ID: "x"}, &ApprovalStep{Approvers: []string{"u1"}, Name: "Step"}, "reminder")
	if len(notifier.events) != 1 {
		t.Errorf("expected 1 event, got %d", len(notifier.events))
	}
}

// contains is a helper for substring matching.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findSubstringIdx(s, substr) >= 0)
}

func findSubstringIdx(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
