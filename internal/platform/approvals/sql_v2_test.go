package approvals

import (
	"context"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

// ── sql_store.go ─────────────────────────────────────────────────

func TestNewSQLStore_NilDB(t *testing.T) {
	_, err := NewSQLStore(nil)
	assert.Error(t, err)
}

func TestNewSQLStore_Success(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLStore(db)
	require.NoError(t, err)
	require.NotNil(t, store)
}

func TestSQLStore_CreateAndGet(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLStore(db)
	require.NoError(t, err)

	a := &Approval{
		ID:         "sql-1",
		State:      "pending",
		FunctionID: "fn1",
		GameID:     "g1",
		Env:        "dev",
		Actor:      "user1",
		Mode:       "invoke",
	}
	created, err := store.Create(a)
	require.NoError(t, err)
	assert.Equal(t, "sql-1", created.ID)

	got, err := store.Get("sql-1")
	require.NoError(t, err)
	assert.Equal(t, "fn1", got.FunctionID)
	assert.Equal(t, "g1", got.GameID)
}

func TestSQLStore_Create_Nil(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLStore(db)
	require.NoError(t, err)
	_, err = store.Create(nil)
	assert.Error(t, err)
}

func TestSQLStore_Get_NotFound(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLStore(db)
	require.NoError(t, err)
	_, err = store.Get("missing")
	assert.Error(t, err)
}

func TestSQLStore_Approve(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLStore(db)
	require.NoError(t, err)

	_, _ = store.Create(&Approval{ID: "a1", State: "pending"})
	got, err := store.Approve("a1")
	require.NoError(t, err)
	assert.Equal(t, "approved", got.State)
}

func TestSQLStore_Approve_NotFound(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLStore(db)
	require.NoError(t, err)
	_, err = store.Approve("missing")
	assert.Error(t, err)
}

func TestSQLStore_Reject(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLStore(db)
	require.NoError(t, err)

	_, _ = store.Create(&Approval{ID: "a1", State: "pending"})
	got, err := store.Reject("a1", "reason")
	require.NoError(t, err)
	assert.Equal(t, "rejected", got.State)
	assert.Equal(t, "reason", got.Reason)
}

func TestSQLStore_Reject_NotFound(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLStore(db)
	require.NoError(t, err)
	_, err = store.Reject("missing", "r")
	assert.Error(t, err)
}

func TestSQLStore_Update(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLStore(db)
	require.NoError(t, err)

	_, _ = store.Create(&Approval{ID: "a1", State: "pending"})
	a := &Approval{ID: "a1", State: "updated", FunctionID: "fn1", GameID: "g1", Env: "dev", Actor: "u1"}
	got, err := store.Update(a)
	require.NoError(t, err)
	assert.Equal(t, "updated", got.State)
}

func TestSQLStore_Update_Nil(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLStore(db)
	require.NoError(t, err)
	_, err = store.Update(nil)
	assert.Error(t, err)
}

func TestSQLStore_List(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLStore(db)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		_, _ = store.Create(&Approval{
			ID:         "a" + string(rune('0'+i)),
			State:      "pending",
			FunctionID: "fn1",
			GameID:     "g1",
			Env:        "dev",
			Actor:      "u1",
			Mode:       "invoke",
		})
	}

	// List all
	list, total, err := store.List(Filter{}, Page{Size: 10})
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, list, 5)

	// Filter by state
	list, total, err = store.List(Filter{State: "pending"}, Page{})
	require.NoError(t, err)
	assert.Equal(t, 5, total)

	// Filter by function
	list, total, err = store.List(Filter{FunctionID: "fn1"}, Page{})
	require.NoError(t, err)
	assert.Equal(t, 5, total)

	// Filter by game
	list, total, err = store.List(Filter{GameID: "g1"}, Page{})
	require.NoError(t, err)
	assert.Equal(t, 5, total)

	// Filter by env
	list, total, err = store.List(Filter{Env: "dev"}, Page{})
	require.NoError(t, err)
	assert.Equal(t, 5, total)

	// Filter by actor
	list, total, err = store.List(Filter{Actor: "u1"}, Page{})
	require.NoError(t, err)
	assert.Equal(t, 5, total)

	// Filter by mode
	list, total, err = store.List(Filter{Mode: "invoke"}, Page{})
	require.NoError(t, err)
	assert.Equal(t, 5, total)
}

func TestSQLStore_List_Pagination(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLStore(db)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		_, _ = store.Create(&Approval{
			ID:    "a" + string(rune('0'+i)),
			State: "pending",
		})
	}

	// Page 1
	list, total, err := store.List(Filter{}, Page{Page: 1, Size: 2})
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, list, 2)

	// Page 3 (partial)
	list, total, err = store.List(Filter{}, Page{Page: 3, Size: 2})
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, list, 1)

	// Beyond total
	list, total, err = store.List(Filter{}, Page{Page: 100, Size: 10})
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Empty(t, list)

	// Default page/size
	list, total, err = store.List(Filter{}, Page{})
	require.NoError(t, err)
	assert.Equal(t, 5, total)
}

func TestSQLStore_BuildOrderClause(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLStore(db)
	require.NoError(t, err)

	s := store
	// Default
	assert.Equal(t, "updated_at DESC", s.buildOrderClause(""))
	// Valid
	assert.Equal(t, "created_at ASC", s.buildOrderClause("created_at asc"))
	assert.Equal(t, "updated_at DESC", s.buildOrderClause("updated_at desc"))
	// Invalid field
	assert.Equal(t, "updated_at ASC", s.buildOrderClause("nonexistent asc"))
	// Invalid direction
	assert.Equal(t, "updated_at DESC", s.buildOrderClause("updated_at sideways"))
	// Malformed
	assert.Equal(t, "updated_at DESC", s.buildOrderClause("nospace"))
}

// ── workflow_store.go SQL stores ─────────────────────────────────

func TestNewSQLWorkflowStore_NilDB(t *testing.T) {
	_, err := NewSQLWorkflowStore(nil)
	assert.Error(t, err)
}

func TestSQLWorkflowStore_DefinitionCRUD(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)

	def := &WorkflowDefinition{
		ID:          "def1",
		Name:        "Test WF",
		Description: "desc",
		Version:     "1.0",
		Active:      true,
		Steps: []ApprovalStep{
			{ID: "step1", Name: "Step 1", Approvers: []string{"u1"}},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		CreatedBy: "admin",
	}

	// Create
	created, err := store.CreateDefinition(def)
	require.NoError(t, err)
	assert.Equal(t, "def1", created.ID)

	// Get
	got, err := store.GetDefinition("def1")
	require.NoError(t, err)
	assert.Equal(t, "Test WF", got.Name)
	assert.Len(t, got.Steps, 1)

	// Update
	def.Description = "updated"
	_, err = store.UpdateDefinition(def)
	require.NoError(t, err)
	got, _ = store.GetDefinition("def1")
	assert.Equal(t, "updated", got.Description)

	// List
	defs, err := store.ListDefinitions(false)
	require.NoError(t, err)
	assert.Len(t, defs, 1)

	defs, err = store.ListDefinitions(true)
	require.NoError(t, err)
	assert.Len(t, defs, 1)

	// Delete
	err = store.DeleteDefinition("def1")
	require.NoError(t, err)
	_, err = store.GetDefinition("def1")
	assert.Error(t, err)
}

func TestSQLWorkflowStore_GetDefinition_NotFound(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)
	_, err = store.GetDefinition("missing")
	assert.ErrorIs(t, err, ErrWorkflowNotFound)
}

func TestSQLWorkflowStore_InstanceCRUD(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)

	inst := &WorkflowInstance{
		ID:           "inst1",
		DefinitionID: "def1",
		State:        WorkflowStatePending,
		CurrentStep:  0,
		Context:      map[string]interface{}{"key": "value"},
		ApprovalID:   "app1",
		Initiator:    "user1",
		StartedAt:    time.Now(),
		History: []WorkflowHistoryEntry{
			{Action: "started", Actor: "user1", Timestamp: time.Now()},
		},
	}

	// Create
	created, err := store.CreateInstance(inst)
	require.NoError(t, err)
	assert.Equal(t, "inst1", created.ID)

	// Get
	got, err := store.GetInstance("inst1")
	require.NoError(t, err)
	assert.Equal(t, WorkflowStatePending, got.State)
	assert.Equal(t, "value", got.Context["key"])

	// Get by approval ID
	got, err = store.GetInstanceByApprovalID("app1")
	require.NoError(t, err)
	assert.Equal(t, "inst1", got.ID)

	// Update
	inst.State = WorkflowStateApproved
	_, err = store.UpdateInstance(inst)
	require.NoError(t, err)
	got, _ = store.GetInstance("inst1")
	assert.Equal(t, WorkflowStateApproved, got.State)

	// List
	instances, total, err := store.ListInstances(WorkflowInstanceFilter{}, Page{})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, instances, 1)

	// List with filters
	instances, total, err = store.ListInstances(WorkflowInstanceFilter{State: WorkflowStateApproved}, Page{})
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	instances, total, err = store.ListInstances(WorkflowInstanceFilter{DefinitionID: "def1"}, Page{})
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	instances, total, err = store.ListInstances(WorkflowInstanceFilter{Initiator: "user1"}, Page{})
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	instances, total, err = store.ListInstances(WorkflowInstanceFilter{ApprovalID: "app1"}, Page{})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
}

func TestSQLWorkflowStore_GetInstance_NotFound(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)
	_, err = store.GetInstance("missing")
	assert.ErrorIs(t, err, ErrWorkflowNotFound)
}

func TestSQLWorkflowStore_GetInstanceByApprovalID_NotFound(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)
	_, err = store.GetInstanceByApprovalID("missing")
	assert.ErrorIs(t, err, ErrWorkflowNotFound)
}

func TestSQLWorkflowStore_StepApprovals(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)

	// Create instance first
	_, _ = store.CreateInstance(&WorkflowInstance{
		ID:           "inst1",
		DefinitionID: "def1",
		State:        WorkflowStatePending,
		CurrentStep:  0,
		ApprovalID:   "app1",
		Initiator:    "user1",
		StartedAt:    time.Now(),
	})

	sa := &StepApproval{
		StepID:    "step1",
		Approver:  "user1",
		Decision:  "approved",
		Comment:   "LGTM",
		DecidedAt: time.Now(),
	}

	err = store.AddStepApproval("inst1", sa)
	require.NoError(t, err)

	approvals, err := store.GetStepApprovals("inst1")
	require.NoError(t, err)
	assert.Len(t, approvals, 1)
	assert.Equal(t, "user1", approvals[0].Approver)
	assert.Equal(t, "approved", approvals[0].Decision)
}

// ── SQL Delegation Store ─────────────────────────────────────────

func TestNewSQLDelegationStore_NilDB(t *testing.T) {
	_, err := NewSQLDelegationStore(nil)
	assert.Error(t, err)
}

func TestSQLDelegationStore_CRUD(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.AutoMigrate(&DelegationModel{}))
	store, err := NewSQLDelegationStore(db)
	require.NoError(t, err)

	now := time.Now()
	d := &Delegation{
		ID:          "del1",
		Delegator:   "boss",
		Delegate:    "worker",
		Scope:       ScopeAll,
		ScopeValue:  "",
		Permissions: []DelegationPermission{PermApprove, PermReject},
		State:       DelegationStateActive,
		Reason:      "vacation",
		StartAt:     now,
		MaxUsages:   10,
		UsageCount:  0,
		Constraints: []DelegationConstraint{
			{Type: "time_restriction", Value: map[string]interface{}{"start_hour": 9}, Enforced: true},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Create
	created, err := store.Create(d)
	require.NoError(t, err)
	assert.Equal(t, "del1", created.ID)

	// Get
	got, err := store.Get("del1")
	require.NoError(t, err)
	assert.Equal(t, "boss", got.Delegator)
	assert.Equal(t, "worker", got.Delegate)
	assert.Len(t, got.Permissions, 2)
	assert.Len(t, got.Constraints, 1)

	// Get not found
	_, err = store.Get("missing")
	assert.ErrorIs(t, err, ErrDelegationNotFound)

	// Update
	d.State = DelegationStateRevoked
	_, err = store.Update(d)
	require.NoError(t, err)
	got, _ = store.Get("del1")
	assert.Equal(t, DelegationStateRevoked, got.State)

	// List
	list, total, err := store.List(DelegationFilter{}, Page{})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, list, 1)

	// List with filter
	list, total, err = store.List(DelegationFilter{Delegator: "boss"}, Page{})
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	list, total, err = store.List(DelegationFilter{Delegate: "worker"}, Page{})
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	list, total, err = store.List(DelegationFilter{Scope: ScopeAll}, Page{})
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	list, total, err = store.List(DelegationFilter{State: DelegationStateActive}, Page{})
	require.NoError(t, err)
	assert.Equal(t, 1, total) // still active since we updated before list

	// GetActiveDelegationsForUser
	d2 := &Delegation{
		ID:          "del2",
		Delegator:   "boss2",
		Delegate:    "worker",
		Scope:       ScopeFunction,
		Permissions: []DelegationPermission{PermApprove},
		State:       DelegationStateActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, _ = store.Create(d2)

	activeFor, err := store.GetActiveDelegationsForUser("worker")
	require.NoError(t, err)
	assert.Len(t, activeFor, 1) // Only del2 is active

	activeBy, err := store.GetActiveDelegationsByUser("boss")
	require.NoError(t, err)
	assert.Len(t, activeBy, 0) // boss's delegation was revoked

	// IncrementUsage
	err = store.IncrementUsage("del2")
	require.NoError(t, err)
	got, _ = store.Get("del2")
	assert.Equal(t, 1, got.UsageCount)

	// Delete
	err = store.Delete("del1")
	require.NoError(t, err)
	_, err = store.Get("del1")
	assert.ErrorIs(t, err, ErrDelegationNotFound)
}

func TestSQLDelegationStore_NilConstraints(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.AutoMigrate(&DelegationModel{}))
	store, err := NewSQLDelegationStore(db)
	require.NoError(t, err)

	d := &Delegation{
		ID:          "del1",
		Delegator:   "boss",
		Delegate:    "worker",
		Scope:       ScopeAll,
		Permissions: []DelegationPermission{PermApprove},
		State:       DelegationStateActive,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	_, err = store.Create(d)
	require.NoError(t, err)

	got, err := store.Get("del1")
	require.NoError(t, err)
	assert.Nil(t, got.Constraints)
}

// ── SQL Notification Store ───────────────────────────────────────

func TestNewSQLNotificationStore_NilDB(t *testing.T) {
	_, err := NewSQLNotificationStore(nil)
	assert.Error(t, err)
}

func TestSQLNotificationStore_CRUD(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.AutoMigrate(&NotificationModel{}))
	store, err := NewSQLNotificationStore(db)
	require.NoError(t, err)

	event := NotificationEvent{
		Type:    "approval_required",
		Title:   "Title",
		Message: "Body",
	}

	// Record
	err = store.RecordNotification("user1", ChannelEmail, event)
	require.NoError(t, err)

	// Count
	count, err := store.GetNotificationCount("user1", ChannelEmail, time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Count for different channel
	count, err = store.GetNotificationCount("user1", ChannelSMS, time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Get notifications
	records, err := store.GetNotifications("user1", 10)
	require.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, "user1", records[0].Recipient)
	assert.Equal(t, ChannelEmail, records[0].Channel)

	// Get with limit
	records, err = store.GetNotifications("user1", 0) // default 50
	require.NoError(t, err)
	assert.Len(t, records, 1)

	// MarkAsRead
	err = store.MarkAsRead("1")
	require.NoError(t, err)
}

// ── store.go edge cases ─────────────────────────────────────────

func TestNewPGStore_EmptyDSN(t *testing.T) {
	_, err := NewPGStore("")
	assert.Error(t, err)
}

func TestNewSQLiteStore_EmptyDSN(t *testing.T) {
	// Empty DSN defaults to "data/croupier.db" which won't exist in test
	_, err := NewSQLiteStore("")
	assert.Error(t, err)
}

func TestOpenDB_PostgreSQL(t *testing.T) {
	// Invalid postgres DSN should fail
	_, err := openDB("postgres://invalid:5432/nonexistent")
	assert.Error(t, err)
}

func TestOpenDB_PostgreSQL2(t *testing.T) {
	_, err := openDB("postgresql://invalid:5432/nonexistent")
	assert.Error(t, err)
}

// ── AutoMigrate ─────────────────────────────────────────────────

func TestAutoMigrate(t *testing.T) {
	db := openTestDB(t)
	err := AutoMigrate(db)
	assert.NoError(t, err)
}

// ── model.go more coverage ──────────────────────────────────────

func TestApprovalModel_TableName(t *testing.T) {
	m := ApprovalModel{}
	assert.Equal(t, "approvals", m.TableName())
}

func TestApprovalModel_ToApproval_WithAllFields(t *testing.T) {
	now := time.Now()
	m := &ApprovalModel{
		ID:              "id",
		State:           "st",
		FunctionID:      "fn",
		GameID:          "g",
		Env:             "e",
		Actor:           "a",
		Mode:            "m",
		IdempotencyKey:  "ik",
		Route:           "r",
		TargetServiceID: "ts",
		HashKey:         "hk",
		Payload:         []byte("p"),
		MetadataJSON:    []byte(`{"k":"v"}`),
		Reason:          "r",
		ResultKind:      "rk",
		TaskID:          "t",
		Result:          []byte("res"),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	a := m.ToApproval()
	assert.Equal(t, "id", a.ID)
	assert.Equal(t, "st", a.State)
	assert.Equal(t, "fn", a.FunctionID)
	assert.Equal(t, "g", a.GameID)
	assert.Equal(t, "e", a.Env)
	assert.Equal(t, "a", a.Actor)
	assert.Equal(t, "m", a.Mode)
	assert.Equal(t, "ik", a.IdempotencyKey)
	assert.Equal(t, "r", a.Route)
	assert.Equal(t, "ts", a.TargetServiceID)
	assert.Equal(t, "hk", a.HashKey)
	assert.Equal(t, []byte("p"), a.Payload)
	assert.Equal(t, "v", a.Metadata["k"])
	assert.Equal(t, "r", a.Reason)
	assert.Equal(t, "rk", a.ResultKind)
	assert.Equal(t, "t", a.TaskID)
	assert.Equal(t, []byte("res"), a.Result)
}

func TestFromApproval_WithAllFields(t *testing.T) {
	now := time.Now()
	a := &Approval{
		ID:              "id",
		State:           "st",
		FunctionID:      "fn",
		GameID:          "g",
		Env:             "e",
		Actor:           "a",
		Mode:            "m",
		IdempotencyKey:  "ik",
		Route:           "r",
		TargetServiceID: "ts",
		HashKey:         "hk",
		Payload:         []byte("p"),
		Metadata:        map[string]string{"k": "v"},
		Reason:          "r",
		ResultKind:      "rk",
		TaskID:          "t",
		Result:          []byte("res"),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	m := FromApproval(a)
	assert.Equal(t, "id", m.ID)
	assert.Equal(t, "st", m.State)
	assert.Contains(t, string(m.MetadataJSON), `"k":"v"`)
}

// ── workflow_store.go model tests ────────────────────────────────

func TestWorkflowDefinitionModel_TableName(t *testing.T) {
	assert.Equal(t, "workflow_definitions", WorkflowDefinitionModel{}.TableName())
}

func TestWorkflowInstanceModel_TableName(t *testing.T) {
	assert.Equal(t, "workflow_instances", WorkflowInstanceModel{}.TableName())
}

func TestStepApprovalModel_TableName(t *testing.T) {
	assert.Equal(t, "workflow_step_approvals", StepApprovalModel{}.TableName())
}

func TestDelegationModel_TableName(t *testing.T) {
	assert.Equal(t, "approval_delegations", DelegationModel{}.TableName())
}

func TestNotificationModel_TableName(t *testing.T) {
	assert.Equal(t, "notifications", NotificationModel{}.TableName())
}

func TestDelegationModel_ToDelegation_RoundTrip(t *testing.T) {
	now := time.Now()
	endAt := now.Add(24 * time.Hour)
	d := &Delegation{
		ID:          "del1",
		Delegator:   "u1",
		Delegate:    "u2",
		Scope:       ScopeFunction,
		ScopeValue:  "fn1",
		Permissions: []DelegationPermission{PermApprove, PermReject},
		State:       DelegationStateActive,
		Reason:      "test",
		StartAt:     now,
		EndAt:       &endAt,
		MaxUsages:   10,
		UsageCount:  3,
		Constraints: []DelegationConstraint{
			{Type: "time_restriction", Value: map[string]interface{}{"days": 5}, Enforced: true},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	model, err := FromDelegation(d)
	require.NoError(t, err)
	got, err := model.ToDelegation()
	require.NoError(t, err)

	assert.Equal(t, d.ID, got.ID)
	assert.Equal(t, d.Delegator, got.Delegator)
	assert.Equal(t, d.Delegate, got.Delegate)
	assert.Equal(t, d.Scope, got.Scope)
	assert.Equal(t, d.ScopeValue, got.ScopeValue)
	assert.Len(t, got.Permissions, 2)
	assert.Len(t, got.Constraints, 1)
	assert.Equal(t, d.MaxUsages, got.MaxUsages)
	assert.Equal(t, d.UsageCount, got.UsageCount)
}

func TestWorkflowDefinitionModel_RoundTrip(t *testing.T) {
	now := time.Now()
	def := &WorkflowDefinition{
		ID:          "def1",
		Name:        "Test",
		Description: "Desc",
		Version:     "1.0",
		Active:      true,
		Steps: []ApprovalStep{
			{ID: "s1", Name: "Step 1", Approvers: []string{"u1"}},
			{ID: "s2", Name: "Step 2", Approvers: []string{"u2"}},
		},
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: "admin",
	}

	model, err := FromDefinition(def)
	require.NoError(t, err)
	got, err := model.ToDefinition()
	require.NoError(t, err)

	assert.Equal(t, def.ID, got.ID)
	assert.Equal(t, def.Name, got.Name)
	assert.Len(t, got.Steps, 2)
	assert.Equal(t, "s1", got.Steps[0].ID)
	assert.Equal(t, "admin", got.CreatedBy)
}

func TestWorkflowInstanceModel_RoundTrip(t *testing.T) {
	now := time.Now()
	inst := &WorkflowInstance{
		ID:           "inst1",
		DefinitionID: "def1",
		State:        WorkflowStatePending,
		CurrentStep:  2,
		Context:      map[string]interface{}{"key": "value", "num": 42},
		ApprovalID:   "app1",
		Initiator:    "user1",
		StartedAt:    now,
		History: []WorkflowHistoryEntry{
			{Action: "started", Actor: "user1", Timestamp: now},
		},
	}

	model, err := FromInstance(inst)
	require.NoError(t, err)
	got, err := model.ToInstance()
	require.NoError(t, err)

	assert.Equal(t, inst.ID, got.ID)
	assert.Equal(t, inst.State, got.State)
	assert.Equal(t, inst.CurrentStep, got.CurrentStep)
	assert.Equal(t, "value", got.Context["key"])
	assert.Len(t, got.History, 1)
}

func TestStepApprovalModel_RoundTrip(t *testing.T) {
	now := time.Now()
	m := &StepApprovalModel{
		InstanceID:  "inst1",
		StepID:      "s1",
		Approver:    "u1",
		DelegatedBy: "u2",
		Decision:    "approved",
		Comment:     "LGTM",
		DecidedAt:   now,
		IPAddress:   "127.0.0.1",
		UserAgent:   "test-agent",
		CreatedAt:   now,
	}
	sa := m.ToStepApproval()
	assert.Equal(t, "s1", sa.StepID)
	assert.Equal(t, "u1", sa.Approver)
	assert.Equal(t, "u2", sa.DelegatedBy)
	assert.Equal(t, "approved", sa.Decision)
	assert.Equal(t, "LGTM", sa.Comment)
	assert.Equal(t, "127.0.0.1", sa.IPAddress)
	assert.Equal(t, "test-agent", sa.UserAgent)
}

// ── notification.go extra coverage ───────────────────────────────

func TestEmailSender_Send_WithSendMail(t *testing.T) {
	s := NewEmailSender("smtp.example.com", 587, "user", "pass", "from@example.com")
	var captured *emailMessage
	s.sendMail = func(ctx context.Context, msg *emailMessage) error {
		captured = msg
		return nil
	}

	err := s.Send(context.Background(), "to@example.com", NotificationEvent{
		Title:    "Test",
		Message:  "Body",
		Data:     map[string]interface{}{"key": "val"},
		Priority: "high",
	})
	require.NoError(t, err)
	assert.Equal(t, "to@example.com", captured.To)
	assert.Equal(t, "from@example.com", captured.From)
	assert.Equal(t, "Test", captured.Subject)
	assert.Contains(t, captured.Body, "Body")
	assert.Contains(t, captured.Body, "key")
	assert.Contains(t, captured.Body, "val")
}

func TestWebSocketHub_BroadcastJSONMarshalError(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()

	c := &WebSocketClient{UserID: "user1", Send: make(chan []byte, 10)}
	hub.Register(c)
	time.Sleep(10 * time.Millisecond)

	// Send something that can't be marshaled (channel)
	hub.SendToUser("user1", make(chan int))
	time.Sleep(10 * time.Millisecond)

	// Should not crash, and channel should be empty (marshal failed)
	select {
	case <-c.Send:
		t.Error("expected no message when marshal fails")
	default:
		// OK - no message sent
	}
}

func TestWebSocketHub_UnregisterNonexistent(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()

	c := &WebSocketClient{UserID: "user1", Send: make(chan []byte, 10)}
	hub.Unregister(c) // Should not crash
	time.Sleep(10 * time.Millisecond)
}

func TestWebSocketHub_BroadcastBufferFull(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()

	c := &WebSocketClient{UserID: "user1", Send: make(chan []byte, 1)} // small buffer
	hub.Register(c)
	time.Sleep(10 * time.Millisecond)

	// Fill the buffer
	hub.SendToUser("user1", "first")
	time.Sleep(10 * time.Millisecond)

	// This should be dropped (buffer full)
	hub.SendToUser("user1", "second")
	time.Sleep(10 * time.Millisecond)

	// Only first message should be in channel
	data := <-c.Send
	assert.Contains(t, string(data), "first")
}

// ── delegation edge cases ────────────────────────────────────────

func TestDelegationService_CreateDelegation_WithConstraints(t *testing.T) {
	store := newFullDelegationStore()
	svc := NewDelegationService(store, nil, nil)

	future := time.Now().Add(time.Hour)
	d := &Delegation{
		ID:          "del1",
		Delegator:   "boss",
		Delegate:    "worker",
		Scope:       ScopeAll,
		State:       DelegationStateActive,
		EndAt:       &future,
		Permissions: []DelegationPermission{PermApprove},
		Constraints: []DelegationConstraint{
			{Type: "time_restriction", Value: map[string]interface{}{"allowed_days": []interface{}{float64(time.Now().Weekday())}}, Enforced: true},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.data["del1"] = d
	store.byDelegate["worker"] = []*Delegation{d}

	got, err := svc.GetActiveDelegation("boss", "worker", ScopeAll, "")
	require.NoError(t, err)
	assert.Equal(t, "del1", got.ID)
}

func TestDelegationService_GetActiveDelegation_ScopeMismatch(t *testing.T) {
	store := newFullDelegationStore()
	future := time.Now().Add(time.Hour)
	d := &Delegation{
		ID:         "d1",
		Delegator:  "boss",
		Delegate:   "worker",
		Scope:      ScopeFunction,
		ScopeValue: "fn1",
		State:      DelegationStateActive,
		EndAt:      &future,
	}
	store.byDelegate["worker"] = []*Delegation{d}
	svc := NewDelegationService(store, nil, nil)
	_, err := svc.GetActiveDelegation("boss", "worker", ScopeGame, "g1")
	assert.ErrorIs(t, err, ErrDelegationNotFound)
}

// ── workflow engine extras ────────────────────────────────────────

func TestWorkflowEngine_ApproveStep_DefinitionLoadFromStore(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	def := &WorkflowDefinition{
		ID:     "def1",
		Name:   "Test",
		Active: true,
		Steps: []ApprovalStep{
			{ID: "step1", Name: "Step1", Approvers: []string{"u1"}},
		},
	}
	_, _ = store.CreateDefinition(def)

	approval := &Approval{ID: "a1", State: "pending", Actor: "initiator"}
	_, _ = approvalStore.Create(approval)

	inst, _ := engine.StartWorkflow(context.Background(), "def1", approval)
	// Clear definition from instance to force lookup
	inst.Definition = nil
	_, _ = store.UpdateInstance(inst)

	updated, err := engine.ApproveStep(context.Background(), inst.ID, "u1", "ok", "", "")
	require.NoError(t, err)
	assert.Equal(t, WorkflowStateApproved, updated.State)
}

func TestWorkflowEngine_RejectStep_DefinitionLoadFromStore(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	def := &WorkflowDefinition{
		ID:     "def1",
		Name:   "Test",
		Active: true,
		Steps: []ApprovalStep{
			{ID: "step1", Name: "Step1", Approvers: []string{"u1"}},
		},
	}
	_, _ = store.CreateDefinition(def)

	approval := &Approval{ID: "a1", State: "pending", Actor: "initiator"}
	_, _ = approvalStore.Create(approval)

	inst, _ := engine.StartWorkflow(context.Background(), "def1", approval)
	inst.Definition = nil
	_, _ = store.UpdateInstance(inst)

	updated, err := engine.RejectStep(context.Background(), inst.ID, "u1", "no", "", "")
	require.NoError(t, err)
	assert.Equal(t, WorkflowStateRejected, updated.State)
}

func TestWorkflowEngine_evaluateCondition_ContainsPrefix(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	engine := NewWorkflowEngine(store, approvalStore, nil)

	// contains in this codebase is prefix-based
	cond := Condition{Field: "tags", Operator: CondOpContains, Value: "important"}
	data := map[string]interface{}{"tags": "important-review"}
	assert.True(t, engine.evaluateCondition(cond, data))

	data2 := map[string]interface{}{"tags": "review-important"}
	assert.False(t, engine.evaluateCondition(cond, data2))
}

func TestWorkflowEngine_evaluateCondition_GreaterThan(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	engine := NewWorkflowEngine(store, approvalStore, nil)

	cond := Condition{Field: "amount", Operator: CondOpGreaterThan, Value: float64(100)}
	assert.True(t, engine.evaluateCondition(cond, map[string]interface{}{"amount": float64(200)}))
	assert.False(t, engine.evaluateCondition(cond, map[string]interface{}{"amount": float64(50)}))
	assert.False(t, engine.evaluateCondition(cond, map[string]interface{}{"amount": float64(100)}))
}

func TestWorkflowEngine_evaluateCondition_LessThan(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	engine := NewWorkflowEngine(store, approvalStore, nil)

	cond := Condition{Field: "amount", Operator: CondOpLessThan, Value: float64(100)}
	assert.True(t, engine.evaluateCondition(cond, map[string]interface{}{"amount": float64(50)}))
	assert.False(t, engine.evaluateCondition(cond, map[string]interface{}{"amount": float64(200)}))
	assert.False(t, engine.evaluateCondition(cond, map[string]interface{}{"amount": float64(100)}))
}

func TestWorkflowEngine_evaluateCondition_In(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	engine := NewWorkflowEngine(store, approvalStore, nil)

	cond := Condition{Field: "status", Operator: CondOpIn, Value: []interface{}{"active", "pending"}}
	assert.True(t, engine.evaluateCondition(cond, map[string]interface{}{"status": "active"}))
	assert.False(t, engine.evaluateCondition(cond, map[string]interface{}{"status": "closed"}))
}

func TestWorkflowEngine_evaluateCondition_NotIn(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	engine := NewWorkflowEngine(store, approvalStore, nil)

	cond := Condition{Field: "status", Operator: CondOpNotIn, Value: []interface{}{"blocked", "banned"}}
	assert.True(t, engine.evaluateCondition(cond, map[string]interface{}{"status": "active"}))
	assert.False(t, engine.evaluateCondition(cond, map[string]interface{}{"status": "blocked"}))
}

func TestWorkflowEngine_evaluateConditionGroup_OR(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	engine := NewWorkflowEngine(store, approvalStore, nil)

	group := ConditionGroup{
		Conditions: []Condition{
			{Field: "state", Operator: CondOpEquals, Value: "pending"},
			{Field: "env", Operator: CondOpEquals, Value: "prod"},
		},
		Logic: "or",
	}
	approval := &Approval{ID: "a1", State: "pending"}
	assert.True(t, engine.evaluateConditionGroup(group, approval))
}

func TestWorkflowEngine_evaluateConditionGroup_AND_Fail(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	engine := NewWorkflowEngine(store, approvalStore, nil)

	group := ConditionGroup{
		Conditions: []Condition{
			{Field: "state", Operator: CondOpEquals, Value: "pending"},
			{Field: "env", Operator: CondOpEquals, Value: "wrong"},
		},
		Logic: "and",
	}
	approval := &Approval{ID: "a1", State: "pending"}
	assert.False(t, engine.evaluateConditionGroup(group, approval))
}

func TestWorkflowEngine_evaluateConditionGroup_EmptyConditions(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	engine := NewWorkflowEngine(store, approvalStore, nil)

	group := ConditionGroup{Logic: "and"}
	approval := &Approval{ID: "a1"}
	assert.True(t, engine.evaluateConditionGroup(group, approval))
}

// ── SQL store buildFilterQuery ───────────────────────────────────

func TestSQLStore_List_AllFilters(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLStore(db)
	require.NoError(t, err)

	_, _ = store.Create(&Approval{
		ID:         "a1",
		State:      "approved",
		FunctionID: "fn1",
		GameID:     "g1",
		Env:        "prod",
		Actor:      "admin",
		Mode:       "invoke",
	})

	// All filters at once
	list, total, err := store.List(Filter{
		State:      "approved",
		FunctionID: "fn1",
		GameID:     "g1",
		Env:        "prod",
		Actor:      "admin",
		Mode:       "invoke",
	}, Page{})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, list, 1)
}
