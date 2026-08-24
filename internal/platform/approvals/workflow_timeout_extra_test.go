package approvals

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowEngine_timeoutEscalate_Branches(t *testing.T) {
	approvalStore := NewMemStore()
	_, err := approvalStore.Create(&Approval{ID: "approval-te", State: "pending"})
	require.NoError(t, err)

	newInstance := func(def *WorkflowDefinition) *WorkflowInstance {
		return &WorkflowInstance{
			ID:           "wf-te",
			DefinitionID: "wf-def",
			Definition:   def,
			State:        WorkflowStatePending,
			ApprovalID:   "approval-te",
			Initiator:    "initiator",
			CurrentStep:  1,
		}
	}

	t.Run("escalate to step with timeout", func(t *testing.T) {
		store := NewMockWorkflowStore()
		notifier := NewMockNotifier()
		engine := NewWorkflowEngine(store, approvalStore, notifier)

		def := &WorkflowDefinition{
			ID:     "wf-def",
			Name:   "escalation",
			Active: true,
			Steps: []ApprovalStep{
				{ID: "step-2", Name: "Upper", Type: StepTypeAny, Approvers: []string{"manager"}},
				{ID: "step-1", Name: "Lower", Type: StepTypeAny, Approvers: []string{"user"}, EscalateTo: "step-2"},
			},
		}
		instance := newInstance(def)
		store.instances[instance.ID] = instance

		updated, err := engine.timeoutEscalate(context.Background(), instance, &def.Steps[1])
		require.NoError(t, err)
		assert.Equal(t, 0, updated.CurrentStep)
		require.Len(t, updated.History, 1)
		assert.Equal(t, "escalated", updated.History[0].Action)
	})

	t.Run("escalate to missing step rejects", func(t *testing.T) {
		store := NewMockWorkflowStore()
		engine := NewWorkflowEngine(store, approvalStore, nil)

		def := &WorkflowDefinition{
			ID:     "wf-def",
			Name:   "broken",
			Active: true,
			Steps: []ApprovalStep{
				{ID: "only", Name: "Only", Type: StepTypeAny, Approvers: []string{"user"}},
			},
		}
		instance := newInstance(def)
		store.instances[instance.ID] = instance

		current := &ApprovalStep{ID: "only", EscalateTo: "ghost-step"}
		updated, err := engine.timeoutEscalate(context.Background(), instance, current)
		require.NoError(t, err)
		assert.Equal(t, WorkflowStateExpired, updated.State)
		require.NotNil(t, updated.CompletedAt)
	})

	t.Run("nil definition rejects", func(t *testing.T) {
		store := NewMockWorkflowStore()
		engine := NewWorkflowEngine(store, approvalStore, nil)

		instance := newInstance(nil)
		store.instances[instance.ID] = instance

		updated, err := engine.timeoutEscalate(context.Background(), instance, &ApprovalStep{ID: "only", EscalateTo: "anywhere"})
		require.NoError(t, err)
		assert.Equal(t, WorkflowStateExpired, updated.State)
	})

	t.Run("empty escalate target rejects", func(t *testing.T) {
		store := NewMockWorkflowStore()
		engine := NewWorkflowEngine(store, approvalStore, nil)

		instance := newInstance(&WorkflowDefinition{Steps: []ApprovalStep{{ID: "only"}}})
		store.instances[instance.ID] = instance

		updated, err := engine.timeoutEscalate(context.Background(), instance, &ApprovalStep{ID: "only"})
		require.NoError(t, err)
		assert.Equal(t, WorkflowStateExpired, updated.State)
	})

	t.Run("escalated step without timeout clears nothing", func(t *testing.T) {
		store := NewMockWorkflowStore()
		engine := NewWorkflowEngine(store, approvalStore, nil)

		def := &WorkflowDefinition{
			ID:     "wf-def",
			Name:   "no timeout",
			Active: true,
			Steps: []ApprovalStep{
				{ID: "target", Name: "Target", Type: StepTypeAny, Approvers: []string{"manager"}},
				{ID: "origin", Name: "Origin", Type: StepTypeAny, EscalateTo: "target"},
			},
		}
		instance := newInstance(def)
		expired := time.Now().Add(-time.Minute)
		instance.ExpiresAt = &expired
		store.instances[instance.ID] = instance

		updated, err := engine.timeoutEscalate(context.Background(), instance, &def.Steps[1])
		require.NoError(t, err)
		assert.Equal(t, 0, updated.CurrentStep)
		require.NotNil(t, updated.ExpiresAt)
	})
}

func TestSQLDelegationStore_GetActiveDelegationsByUser(t *testing.T) {
	db := openTestDB(t)
	_, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)
	store, err := NewSQLDelegationStore(db)
	require.NoError(t, err)

	_, err = store.Create(&Delegation{
		ID:        "del-by-1",
		Delegator: "boss",
		Delegate:  "delegate-1",
		Scope:     ScopeAll,
		State:     DelegationStateActive,
	})
	require.NoError(t, err)
	_, err = store.Create(&Delegation{
		ID:        "del-by-2",
		Delegator: "boss",
		Delegate:  "delegate-2",
		Scope:     ScopeAll,
		State:     DelegationStateRevoked,
	})
	require.NoError(t, err)

	delegations, err := store.GetActiveDelegationsByUser("boss")
	require.NoError(t, err)
	require.Len(t, delegations, 1)
	assert.Equal(t, "del-by-1", delegations[0].ID)
}

func TestSQLNotificationStore_RecordNotificationMarshalError(t *testing.T) {
	db := openTestDB(t)
	_, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)
	store, err := NewSQLNotificationStore(db)
	require.NoError(t, err)

	err = store.RecordNotification("u", ChannelEmail, NotificationEvent{
		Data: map[string]interface{}{"bad": make(chan int)},
	})
	require.Error(t, err)
}

func TestEncodeMetadataJSON(t *testing.T) {
	assert.Nil(t, encodeMetadataJSON(nil))
	assert.Nil(t, encodeMetadataJSON(map[string]string{}))
	assert.JSONEq(t, `{"k":"v"}`, string(encodeMetadataJSON(map[string]string{"k": "v"})))
}
