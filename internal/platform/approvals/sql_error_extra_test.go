package approvals

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// brokenStore drops every approvals table so SQL calls fail.
func brokenStore(t *testing.T) (*SQLStore, *gorm.DB) {
	t.Helper()
	db := openTestDB(t)
	store, err := NewSQLStore(db)
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable("approvals"))
	return store, db
}

func brokenWorkflowStore(t *testing.T) (*SQLWorkflowStore, *gorm.DB) {
	t.Helper()
	db := openTestDB(t)
	store, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable(
		"workflow_definitions", "workflow_instances", "workflow_step_approvals", "approval_delegations", "notifications",
	))
	return store, db
}

func brokenDelegationStore(t *testing.T) (*SQLDelegationStore, *gorm.DB) {
	t.Helper()
	db := openTestDB(t)
	_, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)
	delegations, err := NewSQLDelegationStore(db)
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable("approval_delegations"))
	return delegations, db
}

func brokenNotificationStore(t *testing.T) (*SQLNotificationStore, *gorm.DB) {
	t.Helper()
	db := openTestDB(t)
	_, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)
	store, err := NewSQLNotificationStore(db)
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable("notifications"))
	return store, db
}

func TestSQLStore_DatabaseErrors(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		store, _ := brokenStore(t)
		_, _, err := store.List(Filter{}, Page{})
		require.Error(t, err)
	})

	t.Run("get", func(t *testing.T) {
		store, _ := brokenStore(t)
		_, err := store.Get("id")
		require.Error(t, err)
	})
	t.Run("approve", func(t *testing.T) {
		store, _ := brokenStore(t)
		_, err := store.Approve("id")
		require.Error(t, err)
	})

	t.Run("reject", func(t *testing.T) {
		store, _ := brokenStore(t)
		_, err := store.Reject("id", "reason")
		require.Error(t, err)
	})

	t.Run("create", func(t *testing.T) {
		store, _ := brokenStore(t)
		_, err := store.Create(&Approval{ID: "a1"})
		require.Error(t, err)
	})

	t.Run("update", func(t *testing.T) {
		store, _ := brokenStore(t)
		_, err := store.Update(&Approval{ID: "a1"})
		require.Error(t, err)
	})
}

func TestSQLStore_SaveErrorOnApproveAndReject(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLStore(db)
	require.NoError(t, err)

	created, err := store.Create(&Approval{ID: "save-1", State: "pending"})
	require.NoError(t, err)

	// Break the table only after the record exists so First() succeeds but
	// Save() fails.
	require.NoError(t, db.Migrator().DropTable("approvals"))

	_, err = store.Approve(created.ID)
	require.Error(t, err)
}

func TestSQLStore_FilterQuery(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLStore(db)
	require.NoError(t, err)

	for _, a := range []*Approval{
		{ID: "f1", State: "pending", FunctionID: "fn-a", GameID: "g1", Env: "dev", Actor: "u1", Mode: "invoke"},
		{ID: "f2", State: "approved", FunctionID: "fn-b", GameID: "g2", Env: "prod", Actor: "u2", Mode: "task"},
	} {
		_, err := store.Create(a)
		require.NoError(t, err)
	}

	items, total, err := store.List(Filter{
		State:      "pending",
		FunctionID: "fn-a",
		GameID:     "g1",
		Env:        "dev",
		Actor:      "u1",
		Mode:       "invoke",
	}, Page{})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, items, 1)
	assert.Equal(t, "f1", items[0].ID)
}

func TestNewSQLiteStore_InvalidPath(t *testing.T) {
	_, err := NewSQLiteStore(t.TempDir() + "/missing-dir/deep/db.sqlite")
	require.Error(t, err)
}

func TestSQLWorkflowStore_DatabaseErrors(t *testing.T) {
	t.Run("create definition", func(t *testing.T) {
		store, _ := brokenWorkflowStore(t)
		_, err := store.CreateDefinition(&WorkflowDefinition{ID: "w1", Name: "n", Steps: []ApprovalStep{{ID: "s1"}}})
		require.Error(t, err)
	})

	t.Run("update definition", func(t *testing.T) {
		store, _ := brokenWorkflowStore(t)
		_, err := store.UpdateDefinition(&WorkflowDefinition{ID: "w1", Name: "n", Steps: []ApprovalStep{{ID: "s1"}}})
		require.Error(t, err)
	})

	t.Run("get definition", func(t *testing.T) {
		store, _ := brokenWorkflowStore(t)
		_, err := store.GetDefinition("w1")
		require.Error(t, err)
	})

	t.Run("list definitions", func(t *testing.T) {
		store, _ := brokenWorkflowStore(t)
		_, err := store.ListDefinitions(true)
		require.Error(t, err)
	})

	t.Run("delete definition", func(t *testing.T) {
		store, _ := brokenWorkflowStore(t)
		require.Error(t, store.DeleteDefinition("w1"))
	})

	t.Run("create instance", func(t *testing.T) {
		store, _ := brokenWorkflowStore(t)
		_, err := store.CreateInstance(&WorkflowInstance{ID: "i1", ApprovalID: "a1", Initiator: "u"})
		require.Error(t, err)
	})

	t.Run("get instance", func(t *testing.T) {
		store, _ := brokenWorkflowStore(t)
		_, err := store.GetInstance("i1")
		require.Error(t, err)
	})

	t.Run("get instance by approval", func(t *testing.T) {
		store, _ := brokenWorkflowStore(t)
		_, err := store.GetInstanceByApprovalID("a1")
		require.Error(t, err)
	})

	t.Run("update instance", func(t *testing.T) {
		store, _ := brokenWorkflowStore(t)
		_, err := store.UpdateInstance(&WorkflowInstance{ID: "i1"})
		require.Error(t, err)
	})

	t.Run("list instances", func(t *testing.T) {
		store, _ := brokenWorkflowStore(t)
		_, _, err := store.ListInstances(WorkflowInstanceFilter{State: "pending", DefinitionID: "w", Initiator: "u", ApprovalID: "a"}, Page{})
		require.Error(t, err)
	})

	t.Run("add step approval", func(t *testing.T) {
		store, _ := brokenWorkflowStore(t)
		err := store.AddStepApproval("i1", &StepApproval{StepID: "s1"})
		require.Error(t, err)
	})

	t.Run("get step approvals", func(t *testing.T) {
		store, _ := brokenWorkflowStore(t)
		_, err := store.GetStepApprovals("i1")
		require.Error(t, err)
	})
}

func TestSQLWorkflowStore_ListInstancesFiltering(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)

	def := &WorkflowDefinition{ID: "wf-list", Name: "list", Version: "1", Steps: []ApprovalStep{{ID: "s1"}}}
	_, err = store.CreateDefinition(def)
	require.NoError(t, err)

	for _, inst := range []*WorkflowInstance{
		{ID: "i-pending", DefinitionID: "wf-list", State: WorkflowStatePending, ApprovalID: "a-p", Initiator: "alice"},
		{ID: "i-approved", DefinitionID: "wf-list", State: WorkflowStateApproved, ApprovalID: "a-a", Initiator: "bob"},
	} {
		_, err := store.CreateInstance(inst)
		require.NoError(t, err)
	}

	items, total, err := store.ListInstances(WorkflowInstanceFilter{State: WorkflowStatePending}, Page{})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, items, 1)
	assert.Equal(t, "i-pending", items[0].ID)

	items, total, err = store.ListInstances(WorkflowInstanceFilter{Initiator: "bob"}, Page{Page: 1, Size: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, "i-approved", items[0].ID)

	items, _, err = store.ListInstances(WorkflowInstanceFilter{ApprovalID: "a-p"}, Page{})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "i-pending", items[0].ID)
}

func TestSQLDelegationStore_DatabaseErrors(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		store, _ := brokenDelegationStore(t)
		_, err := store.Create(&Delegation{ID: "d1"})
		require.Error(t, err)
	})

	t.Run("get", func(t *testing.T) {
		store, _ := brokenDelegationStore(t)
		_, err := store.Get("d1")
		require.Error(t, err)
	})

	t.Run("update", func(t *testing.T) {
		store, _ := brokenDelegationStore(t)
		_, err := store.Update(&Delegation{ID: "d1"})
		require.Error(t, err)
	})

	t.Run("delete", func(t *testing.T) {
		store, _ := brokenDelegationStore(t)
		require.Error(t, store.Delete("d1"))
	})

	t.Run("list", func(t *testing.T) {
		store, _ := brokenDelegationStore(t)
		_, _, err := store.List(DelegationFilter{Delegator: "a", Delegate: "b", Scope: ScopeAll, State: DelegationStateActive}, Page{})
		require.Error(t, err)
	})

	t.Run("active for user", func(t *testing.T) {
		store, _ := brokenDelegationStore(t)
		_, err := store.GetActiveDelegationsForUser("u")
		require.Error(t, err)
	})

	t.Run("active by user", func(t *testing.T) {
		store, _ := brokenDelegationStore(t)
		_, err := store.GetActiveDelegationsByUser("u")
		require.Error(t, err)
	})

	t.Run("increment usage", func(t *testing.T) {
		store, _ := brokenDelegationStore(t)
		require.Error(t, store.IncrementUsage("d1"))
	})
}

func TestSQLNotificationStore_DatabaseErrors(t *testing.T) {
	t.Run("record", func(t *testing.T) {
		store, _ := brokenNotificationStore(t)
		require.Error(t, store.RecordNotification("u", ChannelEmail, NotificationEvent{}))
	})

	t.Run("count", func(t *testing.T) {
		store, _ := brokenNotificationStore(t)
		_, err := store.GetNotificationCount("u", ChannelEmail, time.Hour)
		require.Error(t, err)
	})

	t.Run("list", func(t *testing.T) {
		store, _ := brokenNotificationStore(t)
		_, err := store.GetNotifications("u", 0)
		require.Error(t, err)
	})

	t.Run("mark as read", func(t *testing.T) {
		store, _ := brokenNotificationStore(t)
		require.Error(t, store.MarkAsRead("1"))
	})
}

func TestWorkflowModelConversion_Errors(t *testing.T) {
	t.Run("ToDefinition bad steps JSON", func(t *testing.T) {
		_, err := (&WorkflowDefinitionModel{StepsJSON: []byte("not-json")}).ToDefinition()
		require.Error(t, err)
	})

	t.Run("FromDefinition marshal error", func(t *testing.T) {
		_, err := FromDefinition(&WorkflowDefinition{
			Steps: []ApprovalStep{{
				Conditions: []ConditionGroup{{Conditions: []Condition{{Field: "f", Value: make(chan int)}}}},
			}},
		})
		require.Error(t, err)
	})

	t.Run("ToInstance bad context JSON", func(t *testing.T) {
		_, err := (&WorkflowInstanceModel{ContextJSON: []byte("not-json")}).ToInstance()
		require.Error(t, err)
	})

	t.Run("ToInstance bad history JSON", func(t *testing.T) {
		_, err := (&WorkflowInstanceModel{HistoryJSON: []byte("not-json")}).ToInstance()
		require.Error(t, err)
	})

	t.Run("FromInstance marshal error", func(t *testing.T) {
		_, err := FromInstance(&WorkflowInstance{
			Context: map[string]interface{}{"bad": make(chan int)},
		})
		require.Error(t, err)
	})

	t.Run("FromDelegation constraint marshal error", func(t *testing.T) {
		_, err := FromDelegation(&Delegation{
			Constraints: []DelegationConstraint{{Type: "t", Value: make(chan int)}},
		})
		require.Error(t, err)
	})

	t.Run("ToDelegation bad permissions JSON", func(t *testing.T) {
		_, err := (&DelegationModel{Permissions: []byte("not-json")}).ToDelegation()
		require.Error(t, err)
	})

	t.Run("ToDelegation bad constraints JSON", func(t *testing.T) {
		_, err := (&DelegationModel{Permissions: []byte("[]"), Constraints: []byte("not-json")}).ToDelegation()
		require.Error(t, err)
	})
}

func TestWorkflowModelConversion_RoundTrip(t *testing.T) {
	def := &WorkflowDefinition{
		ID:      "wf-rt",
		Name:    "round trip",
		Version: "v2",
		Active:  true,
		Steps:   []ApprovalStep{{ID: "s1", Name: "first", Type: StepTypeAny, Approvers: []string{"a"}}},
	}
	model, err := FromDefinition(def)
	require.NoError(t, err)
	back, err := model.ToDefinition()
	require.NoError(t, err)
	assert.Equal(t, def.ID, back.ID)
	require.Len(t, back.Steps, 1)
	assert.Equal(t, "s1", back.Steps[0].ID)

	completed := time.Now()
	inst := &WorkflowInstance{
		ID:           "wf-inst",
		DefinitionID: "wf-rt",
		State:        WorkflowStateApproved,
		Context:      map[string]interface{}{"k": "v"},
		ApprovalID:   "ap-1",
		Initiator:    "initiator",
		CompletedAt:  &completed,
		History:      []WorkflowHistoryEntry{{Action: "done"}},
	}
	instModel, err := FromInstance(inst)
	require.NoError(t, err)
	instBack, err := instModel.ToInstance()
	require.NoError(t, err)
	assert.Equal(t, inst.ID, instBack.ID)
	assert.Equal(t, inst.Context["k"], instBack.Context["k"])
	require.Len(t, instBack.History, 1)

	endAt := time.Now()
	delegation := &Delegation{
		ID:          "del-rt",
		Delegator:   "boss",
		Delegate:    "helper",
		Scope:       ScopeFunction,
		ScopeValue:  "fn-1",
		Permissions: []DelegationPermission{PermApprove, PermReject},
		State:       DelegationStateActive,
		EndAt:       &endAt,
	}
	delModel, err := FromDelegation(delegation)
	require.NoError(t, err)
	delBack, err := delModel.ToDelegation()
	require.NoError(t, err)
	assert.Equal(t, delegation.ID, delBack.ID)
	assert.Equal(t, delegation.Permissions, delBack.Permissions)
	require.NotNil(t, delBack.EndAt)
}

func TestSQLNotificationStore_DefaultLimit(t *testing.T) {
	db := openTestDB(t)
	_, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)
	store, err := NewSQLNotificationStore(db)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		require.NoError(t, store.RecordNotification("u-limit", ChannelInApp, NotificationEvent{Type: "t"}))
	}

	records, err := store.GetNotifications("u-limit", 0)
	require.NoError(t, err)
	assert.Len(t, records, 3)

	count, err := store.GetNotificationCount("u-limit", ChannelInApp, time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	require.NoError(t, store.MarkAsRead(records[0].ID))
}
