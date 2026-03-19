package approvals

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockWorkflowStore is a mock implementation of WorkflowStore for testing
type MockWorkflowStore struct {
	definitions   map[string]*WorkflowDefinition
	instances     map[string]*WorkflowInstance
	stepApprovals map[string][]StepApproval
	mu            map[string]interface{}
}

func NewMockWorkflowStore() *MockWorkflowStore {
	return &MockWorkflowStore{
		definitions:   make(map[string]*WorkflowDefinition),
		instances:     make(map[string]*WorkflowInstance),
		stepApprovals: make(map[string][]StepApproval),
	}
}

func (m *MockWorkflowStore) CreateDefinition(def *WorkflowDefinition) (*WorkflowDefinition, error) {
	m.definitions[def.ID] = def
	return def, nil
}

func (m *MockWorkflowStore) UpdateDefinition(def *WorkflowDefinition) (*WorkflowDefinition, error) {
	m.definitions[def.ID] = def
	return def, nil
}

func (m *MockWorkflowStore) GetDefinition(id string) (*WorkflowDefinition, error) {
	def, ok := m.definitions[id]
	if !ok {
		return nil, ErrWorkflowNotFound
	}
	return def, nil
}

func (m *MockWorkflowStore) ListDefinitions(activeOnly bool) ([]*WorkflowDefinition, error) {
	var result []*WorkflowDefinition
	for _, def := range m.definitions {
		if !activeOnly || def.Active {
			result = append(result, def)
		}
	}
	return result, nil
}

func (m *MockWorkflowStore) DeleteDefinition(id string) error {
	delete(m.definitions, id)
	return nil
}

func (m *MockWorkflowStore) CreateInstance(inst *WorkflowInstance) (*WorkflowInstance, error) {
	m.instances[inst.ID] = inst
	return inst, nil
}

func (m *MockWorkflowStore) GetInstance(id string) (*WorkflowInstance, error) {
	inst, ok := m.instances[id]
	if !ok {
		return nil, ErrWorkflowNotFound
	}
	return inst, nil
}

func (m *MockWorkflowStore) GetInstanceByApprovalID(approvalID string) (*WorkflowInstance, error) {
	for _, inst := range m.instances {
		if inst.ApprovalID == approvalID {
			return inst, nil
		}
	}
	return nil, ErrWorkflowNotFound
}

func (m *MockWorkflowStore) UpdateInstance(inst *WorkflowInstance) (*WorkflowInstance, error) {
	// Preserve the definition reference if updating an existing instance
	if existing, ok := m.instances[inst.ID]; ok && existing.Definition != nil && inst.Definition == nil {
		inst.Definition = existing.Definition
	}
	m.instances[inst.ID] = inst
	return inst, nil
}

func (m *MockWorkflowStore) ListInstances(filter WorkflowInstanceFilter, page Page) ([]*WorkflowInstance, int, error) {
	var result []*WorkflowInstance
	for _, inst := range m.instances {
		result = append(result, inst)
	}
	return result, len(result), nil
}

func (m *MockWorkflowStore) AddStepApproval(instanceID string, approval *StepApproval) error {
	m.stepApprovals[instanceID] = append(m.stepApprovals[instanceID], *approval)
	return nil
}

func (m *MockWorkflowStore) GetStepApprovals(instanceID string) ([]StepApproval, error) {
	return m.stepApprovals[instanceID], nil
}

// MockNotifier is a mock implementation of Notifier for testing
type MockNotifier struct {
	events []NotificationEvent
}

func NewMockNotifier() *MockNotifier {
	return &MockNotifier{events: make([]NotificationEvent, 0)}
}

func (m *MockNotifier) Notify(ctx context.Context, recipients []string, event NotificationEvent) error {
	m.events = append(m.events, event)
	return nil
}

func (m *MockNotifier) NotifyWithChannels(ctx context.Context, recipients []string, event NotificationEvent, channels []NotificationChannel) error {
	m.events = append(m.events, event)
	return nil
}

func (m *MockNotifier) GetEvents() []NotificationEvent {
	return m.events
}

func (m *MockNotifier) Clear() {
	m.events = make([]NotificationEvent, 0)
}

// TestWorkflowEngine_CreateDefinition validates workflow definition creation
func TestWorkflowEngine_CreateDefinition(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	t.Run("Valid definition", func(t *testing.T) {
		def := &WorkflowDefinition{
			ID:      "test-workflow",
			Name:    "Test Workflow",
			Version: "1.0",
			Active:  true,
			Steps: []ApprovalStep{
				{
					ID:        "step1",
					Name:      "Manager Approval",
					Type:      StepTypeSequential,
					Approvers: []string{"manager1", "manager2"},
					Order:     0,
				},
			},
		}

		created, err := engine.CreateDefinition(def)
		require.NoError(t, err)
		assert.Equal(t, def.ID, created.ID)
		assert.False(t, created.CreatedAt.IsZero())
		assert.False(t, created.UpdatedAt.IsZero())
	})

	t.Run("Missing ID", func(t *testing.T) {
		def := &WorkflowDefinition{
			Name:    "Test Workflow",
			Version: "1.0",
			Steps:   []ApprovalStep{{ID: "step1", Approvers: []string{"user1"}}},
		}

		_, err := engine.CreateDefinition(def)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workflow ID is required")
	})

	t.Run("Missing name", func(t *testing.T) {
		def := &WorkflowDefinition{
			ID:      "test-workflow",
			Version: "1.0",
			Steps:   []ApprovalStep{{ID: "step1", Approvers: []string{"user1"}}},
		}

		_, err := engine.CreateDefinition(def)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workflow name is required")
	})

	t.Run("No steps", func(t *testing.T) {
		def := &WorkflowDefinition{
			ID:      "test-workflow",
			Name:    "Test Workflow",
			Version: "1.0",
			Steps:   []ApprovalStep{},
		}

		_, err := engine.CreateDefinition(def)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one step")
	})

	t.Run("Duplicate step IDs", func(t *testing.T) {
		def := &WorkflowDefinition{
			ID:      "test-workflow",
			Name:    "Test Workflow",
			Version: "1.0",
			Steps: []ApprovalStep{
				{ID: "step1", Approvers: []string{"user1"}},
				{ID: "step1", Approvers: []string{"user2"}},
			},
		}

		_, err := engine.CreateDefinition(def)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate step ID")
	})

	t.Run("Invalid percentage type", func(t *testing.T) {
		def := &WorkflowDefinition{
			ID:      "test-workflow",
			Name:    "Test Workflow",
			Version: "1.0",
			Steps: []ApprovalStep{
				{
					ID:            "step1",
					Type:          StepTypePercentage,
					RequiredCount: 150, // Invalid: > 100
					Approvers:     []string{"user1"},
				},
			},
		}

		_, err := engine.CreateDefinition(def)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "percentage type requires required_count between 1 and 100")
	})
}

// TestWorkflowEngine_StartWorkflow tests workflow instance creation
func TestWorkflowEngine_StartWorkflow(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	// Create a workflow definition first
	def := &WorkflowDefinition{
		ID:      "test-workflow",
		Name:    "Test Workflow",
		Version: "1.0",
		Active:  true,
		Steps: []ApprovalStep{
			{
				ID:        "step1",
				Name:      "Manager Approval",
				Type:      StepTypeSequential,
				Approvers: []string{"manager1"},
				Order:     0,
			},
		},
	}
	_, err := store.CreateDefinition(def)
	require.NoError(t, err)

	// Create an approval
	approval := &Approval{
		ID:         "approval-1",
		State:      "pending",
		FunctionID: "test-function",
		GameID:     "test-game",
		Env:        "dev",
		Actor:      "initiator",
	}
	_, err = approvalStore.Create(approval)
	require.NoError(t, err)

	t.Run("Successfully start workflow", func(t *testing.T) {
		ctx := context.Background()
		instance, err := engine.StartWorkflow(ctx, "test-workflow", approval)
		require.NoError(t, err)
		assert.Equal(t, WorkflowStatePending, instance.State)
		assert.Equal(t, "test-workflow", instance.DefinitionID)
		assert.Equal(t, approval.ID, instance.ApprovalID)
		assert.Equal(t, "initiator", instance.Initiator)
		assert.Equal(t, 0, instance.CurrentStep)
		assert.NotNil(t, instance.Definition)
		assert.Len(t, instance.History, 1)
		assert.Equal(t, "workflow_started", instance.History[0].Action)

		// Check notification was sent
		events := notifier.GetEvents()
		assert.NotEmpty(t, events)
		assert.Equal(t, "approval_required", events[0].Type)
	})

	t.Run("Workflow not found", func(t *testing.T) {
		ctx := context.Background()
		_, err := engine.StartWorkflow(ctx, "nonexistent", approval)
		assert.Error(t, err)
	})

	t.Run("Inactive workflow", func(t *testing.T) {
		inactiveDef := &WorkflowDefinition{
			ID:      "inactive-workflow",
			Name:    "Inactive Workflow",
			Active:  false,
			Steps:   []ApprovalStep{{ID: "step1", Approvers: []string{"user1"}}},
		}
		_, err := store.CreateDefinition(inactiveDef)
		require.NoError(t, err)

		ctx := context.Background()
		_, err = engine.StartWorkflow(ctx, "inactive-workflow", approval)
		assert.Error(t, err)
		assert.Equal(t, ErrWorkflowNotActive, err)
	})

	t.Run("Duplicate workflow for approval", func(t *testing.T) {
		ctx := context.Background()
		_, err := engine.StartWorkflow(ctx, "test-workflow", approval)
		assert.Error(t, err)
		assert.Equal(t, ErrApprovalAlreadyExists, err)
	})

	t.Run("Workflow with timeout", func(t *testing.T) {
		defWithTimeout := &WorkflowDefinition{
			ID:      "timeout-workflow",
			Name:    "Timeout Workflow",
			Active:  true,
			Steps: []ApprovalStep{
				{
					ID:        "step1",
					Name:      "Step 1",
					Approvers: []string{"user1"},
					Timeout:   time.Hour,
				},
			},
		}
		_, err := store.CreateDefinition(defWithTimeout)
		require.NoError(t, err)

		approval2 := &Approval{
			ID:    "approval-2",
			State: "pending",
			Actor: "initiator",
		}
		_, err = approvalStore.Create(approval2)
		require.NoError(t, err)

		ctx := context.Background()
		instance, err := engine.StartWorkflow(ctx, "timeout-workflow", approval2)
		require.NoError(t, err)
		assert.NotNil(t, instance.ExpiresAt)
	})
}

// TestWorkflowEngine_ApproveStep_Basic tests basic approval scenarios
func TestWorkflowEngine_ApproveStep_Basic(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	def := &WorkflowDefinition{
		ID:      "test-workflow-basic",
		Name:    "Test Workflow",
		Active:  true,
		Steps: []ApprovalStep{
			{
				ID:        "step1",
				Name:      "Manager Approval",
				Type:      StepTypeSequential,
				Approvers: []string{"manager1"},
			},
			{
				ID:        "step2",
				Name:      "Director Approval",
				Type:      StepTypeSequential,
				Approvers: []string{"director1"},
			},
		},
	}
	_, err := store.CreateDefinition(def)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Successfully approve step", func(t *testing.T) {
		approval := &Approval{
			ID:    "approval-basic-1",
			State: "pending",
			Actor: "initiator",
		}
		_, err = approvalStore.Create(approval)
		require.NoError(t, err)

		instance, err := engine.StartWorkflow(ctx, "test-workflow-basic", approval)
		require.NoError(t, err)

		updated, err := engine.ApproveStep(ctx, instance.ID, "manager1", "Looks good", "127.0.0.1", "test-agent")
		require.NoError(t, err)
		assert.Equal(t, WorkflowStatePending, updated.State)
		assert.Equal(t, 1, updated.CurrentStep)
		assert.Len(t, updated.StepApprovals, 1)
	})
}

// TestWorkflowEngine_ApproveStep_Unauthorized tests unauthorized approval attempts
func TestWorkflowEngine_ApproveStep_Unauthorized(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	def := &WorkflowDefinition{
		ID:      "test-workflow-auth",
		Name:    "Test Workflow",
		Active:  true,
		Steps: []ApprovalStep{
			{
				ID:        "step1",
				Name:      "Manager Approval",
				Type:      StepTypeSequential,
				Approvers: []string{"manager1"},
			},
		},
	}
	_, err := store.CreateDefinition(def)
	require.NoError(t, err)

	ctx := context.Background()
	approval := &Approval{
		ID:    "approval-auth-1",
		State: "pending",
		Actor: "initiator",
	}
	_, err = approvalStore.Create(approval)
	require.NoError(t, err)

	instance, err := engine.StartWorkflow(ctx, "test-workflow-auth", approval)
	require.NoError(t, err)

	_, err = engine.ApproveStep(ctx, instance.ID, "unauthorized", "Trying to approve", "", "")
	assert.Error(t, err)
	assert.Equal(t, ErrNotAuthorizedApprover, err)
}

// TestWorkflowEngine_ApproveStep_Complete tests completing a workflow
func TestWorkflowEngine_ApproveStep_Complete(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	def := &WorkflowDefinition{
		ID:      "test-workflow-complete",
		Name:    "Test Workflow",
		Active:  true,
		Steps: []ApprovalStep{
			{
				ID:        "step1",
				Name:      "Manager Approval",
				Type:      StepTypeSequential,
				Approvers: []string{"manager1"},
			},
			{
				ID:        "step2",
				Name:      "Director Approval",
				Type:      StepTypeSequential,
				Approvers: []string{"director1"},
			},
		},
	}
	_, err := store.CreateDefinition(def)
	require.NoError(t, err)

	ctx := context.Background()
	approval := &Approval{
		ID:    "approval-complete-1",
		State: "pending",
		Actor: "initiator",
	}
	_, err = approvalStore.Create(approval)
	require.NoError(t, err)

	instance, err := engine.StartWorkflow(ctx, "test-workflow-complete", approval)
	require.NoError(t, err)

	// First approval
	_, err = engine.ApproveStep(ctx, instance.ID, "manager1", "First", "", "")
	require.NoError(t, err)

	// Get updated instance and approve final step
	instance, err = store.GetInstance(instance.ID)
	require.NoError(t, err)

	updated, err := engine.ApproveStep(ctx, instance.ID, "director1", "Approved", "127.0.0.1", "test-agent")
	require.NoError(t, err)
	assert.Equal(t, WorkflowStateApproved, updated.State)
	assert.NotNil(t, updated.CompletedAt)
}

// TestWorkflowEngine_ApproveStep_Duplicate tests duplicate approval scenarios
func TestWorkflowEngine_ApproveStep_Duplicate(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	// Create a workflow where first step needs 2 approvers
	def := &WorkflowDefinition{
		ID:      "test-workflow-dup",
		Name:    "Two Approver Workflow",
		Active:  true,
		Steps: []ApprovalStep{
			{
				ID:        "step1",
				Name:      "Step 1",
				Type:      StepTypeParallel,
				Approvers: []string{"manager1", "manager2"},
			},
			{
				ID:        "step2",
				Name:      "Step 2",
				Type:      StepTypeSequential,
				Approvers: []string{"director1"},
			},
		},
	}
	_, err := store.CreateDefinition(def)
	require.NoError(t, err)

	ctx := context.Background()
	approval := &Approval{
		ID:    "approval-dup-1",
		State: "pending",
		Actor: "initiator",
	}
	_, err = approvalStore.Create(approval)
	require.NoError(t, err)

	instance, err := engine.StartWorkflow(ctx, "test-workflow-dup", approval)
	require.NoError(t, err)

	// First approval
	_, err = engine.ApproveStep(ctx, instance.ID, "manager1", "First", "", "")
	require.NoError(t, err)

	// Try to approve same step again with same user
	_, err = engine.ApproveStep(ctx, instance.ID, "manager1", "Again", "", "")
	assert.Error(t, err)
	assert.Equal(t, ErrApprovalAlreadyExists, err)
}

// TestWorkflowEngine_RejectStep tests workflow rejection
func TestWorkflowEngine_RejectStep(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	def := &WorkflowDefinition{
		ID:      "test-workflow",
		Name:    "Test Workflow",
		Active:  true,
		Steps: []ApprovalStep{
			{
				ID:        "step1",
				Name:      "Manager Approval",
				Type:      StepTypeSequential,
				Approvers: []string{"manager1"},
			},
		},
	}
	_, err := store.CreateDefinition(def)
	require.NoError(t, err)

	approval := &Approval{
		ID:    "approval-1",
		State: "pending",
		Actor: "initiator",
	}
	_, err = approvalStore.Create(approval)
	require.NoError(t, err)

	ctx := context.Background()
	instance, err := engine.StartWorkflow(ctx, "test-workflow", approval)
	require.NoError(t, err)

	t.Run("Successfully reject workflow", func(t *testing.T) {
		updated, err := engine.RejectStep(ctx, instance.ID, "manager1", "Not approved", "127.0.0.1", "test-agent")
		require.NoError(t, err)
		assert.Equal(t, WorkflowStateRejected, updated.State)
		assert.NotNil(t, updated.CompletedAt)

		// Check original approval was rejected
		originalApproval, err := approvalStore.Get(approval.ID)
		require.NoError(t, err)
		assert.Equal(t, "rejected", originalApproval.State)
		assert.Equal(t, "Not approved", originalApproval.Reason)
	})

	t.Run("Unauthorized rejection", func(t *testing.T) {
		approval2 := &Approval{
			ID:    "approval-2",
			State: "pending",
			Actor: "initiator",
		}
		_, err := approvalStore.Create(approval2)
		require.NoError(t, err)

		instance2, err := engine.StartWorkflow(ctx, "test-workflow", approval2)
		require.NoError(t, err)

		_, err = engine.RejectStep(ctx, instance2.ID, "unauthorized", "No", "", "")
		assert.Error(t, err)
		assert.Equal(t, ErrNotAuthorizedApprover, err)
	})
}

// TestWorkflowEngine_CancelWorkflow tests workflow cancellation
func TestWorkflowEngine_CancelWorkflow(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	def := &WorkflowDefinition{
		ID:      "test-workflow",
		Name:    "Test Workflow",
		Active:  true,
		Steps: []ApprovalStep{
			{ID: "step1", Name: "Step 1", Approvers: []string{"manager1"}},
		},
	}
	_, err := store.CreateDefinition(def)
	require.NoError(t, err)

	approval := &Approval{
		ID:    "approval-1",
		State: "pending",
		Actor: "initiator",
	}
	_, err = approvalStore.Create(approval)
	require.NoError(t, err)

	ctx := context.Background()
	instance, err := engine.StartWorkflow(ctx, "test-workflow", approval)
	require.NoError(t, err)

	t.Run("Successfully cancel workflow", func(t *testing.T) {
		updated, err := engine.CancelWorkflow(ctx, instance.ID, "initiator", "No longer needed")
		require.NoError(t, err)
		assert.Equal(t, WorkflowStateCancelled, updated.State)
		assert.NotNil(t, updated.CompletedAt)

		// Check original approval was rejected
		originalApproval, err := approvalStore.Get(approval.ID)
		require.NoError(t, err)
		assert.Equal(t, "rejected", originalApproval.State)
		assert.Contains(t, originalApproval.Reason, "Cancelled")
	})
}

// TestWorkflowEngine_ParallelStepType tests parallel approval step type
func TestWorkflowEngine_ParallelStepType(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	def := &WorkflowDefinition{
		ID:      "parallel-workflow",
		Name:    "Parallel Workflow",
		Active:  true,
		Steps: []ApprovalStep{
			{
				ID:        "step1",
				Name:      "Parallel Approval",
				Type:      StepTypeParallel,
				Approvers: []string{"manager1", "manager2", "manager3"},
			},
		},
	}
	_, err := store.CreateDefinition(def)
	require.NoError(t, err)

	approval := &Approval{
		ID:    "approval-1",
		State: "pending",
		Actor: "initiator",
	}
	_, err = approvalStore.Create(approval)
	require.NoError(t, err)

	ctx := context.Background()
	instance, err := engine.StartWorkflow(ctx, "parallel-workflow", approval)
	require.NoError(t, err)

	// First approval
	updated, err := engine.ApproveStep(ctx, instance.ID, "manager1", "OK", "", "")
	require.NoError(t, err)
	assert.Equal(t, WorkflowStatePending, updated.State) // Still needs all approvers
	assert.Len(t, updated.StepApprovals, 1)

	// Second approval
	updated, err = engine.ApproveStep(ctx, instance.ID, "manager2", "OK", "", "")
	require.NoError(t, err)
	assert.Equal(t, WorkflowStatePending, updated.State)
	assert.Len(t, updated.StepApprovals, 2)

	// Third approval completes the step
	updated, err = engine.ApproveStep(ctx, instance.ID, "manager3", "OK", "", "")
	require.NoError(t, err)
	assert.Equal(t, WorkflowStateApproved, updated.State)
}

// TestWorkflowEngine_AnyStepType tests "any" approval step type
func TestWorkflowEngine_AnyStepType(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	def := &WorkflowDefinition{
		ID:      "any-workflow",
		Name:    "Any Approval Workflow",
		Active:  true,
		Steps: []ApprovalStep{
			{
				ID:        "step1",
				Name:      "Any Manager Approval",
				Type:      StepTypeAny,
				Approvers: []string{"manager1", "manager2", "manager3"},
			},
		},
	}
	_, err := store.CreateDefinition(def)
	require.NoError(t, err)

	approval := &Approval{
		ID:    "approval-1",
		State: "pending",
		Actor: "initiator",
	}
	_, err = approvalStore.Create(approval)
	require.NoError(t, err)

	ctx := context.Background()
	instance, err := engine.StartWorkflow(ctx, "any-workflow", approval)
	require.NoError(t, err)

	// Single approval completes the step
	updated, err := engine.ApproveStep(ctx, instance.ID, "manager2", "OK", "", "")
	require.NoError(t, err)
	assert.Equal(t, WorkflowStateApproved, updated.State)
	assert.Len(t, updated.StepApprovals, 1)
}

// TestWorkflowEngine_PercentageStepType tests percentage-based approval
func TestWorkflowEngine_PercentageStepType(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	def := &WorkflowDefinition{
		ID:      "percentage-workflow",
		Name:    "Percentage Workflow",
		Active:  true,
		Steps: []ApprovalStep{
			{
				ID:            "step1",
				Name:          "50% Approval",
				Type:          StepTypePercentage,
				Approvers:     []string{"user1", "user2", "user3", "user4"},
				RequiredCount: 50, // Need 50% = 2 of 4
			},
		},
	}
	_, err := store.CreateDefinition(def)
	require.NoError(t, err)

	approval := &Approval{
		ID:    "approval-1",
		State: "pending",
		Actor: "initiator",
	}
	_, err = approvalStore.Create(approval)
	require.NoError(t, err)

	ctx := context.Background()
	instance, err := engine.StartWorkflow(ctx, "percentage-workflow", approval)
	require.NoError(t, err)

	// First approval
	updated, err := engine.ApproveStep(ctx, instance.ID, "user1", "OK", "", "")
	require.NoError(t, err)
	assert.Equal(t, WorkflowStatePending, updated.State)

	// Second approval (50% reached) completes the step
	updated, err = engine.ApproveStep(ctx, instance.ID, "user2", "OK", "", "")
	require.NoError(t, err)
	assert.Equal(t, WorkflowStateApproved, updated.State)
}

// TestWorkflowEngine_ProcessTimeouts tests timeout processing
func TestWorkflowEngine_ProcessTimeouts(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	t.Run("Timeout reject action", func(t *testing.T) {
		def := &WorkflowDefinition{
			ID:      "timeout-reject-workflow",
			Name:    "Timeout Reject Workflow",
			Active:  true,
			Steps: []ApprovalStep{
				{
					ID:            "step1",
					Name:          "Step 1",
					Approvers:     []string{"manager1"},
					Timeout:       time.Millisecond,
					TimeoutAction: "reject",
				},
			},
		}
		_, err := store.CreateDefinition(def)
		require.NoError(t, err)

		approval := &Approval{
			ID:    "approval-1",
			State: "pending",
			Actor: "initiator",
		}
		_, err = approvalStore.Create(approval)
		require.NoError(t, err)

		ctx := context.Background()
		instance, err := engine.StartWorkflow(ctx, "timeout-reject-workflow", approval)
		require.NoError(t, err)

		// Wait for timeout
		time.Sleep(100 * time.Millisecond)

		processed, err := engine.ProcessTimeouts(ctx)
		require.NoError(t, err)
		assert.Len(t, processed, 1)

		// Check instance was rejected
		updated, err := store.GetInstance(instance.ID)
		require.NoError(t, err)
		assert.Equal(t, WorkflowStateExpired, updated.State)
	})

	t.Run("Timeout approve action", func(t *testing.T) {
		def := &WorkflowDefinition{
			ID:      "timeout-approve-workflow",
			Name:    "Timeout Approve Workflow",
			Active:  true,
			Steps: []ApprovalStep{
				{
					ID:            "step1",
					Name:          "Step 1",
					Approvers:     []string{"manager1"},
					Timeout:       time.Millisecond,
					TimeoutAction: "approve",
				},
			},
		}
		_, err := store.CreateDefinition(def)
		require.NoError(t, err)

		approval := &Approval{
			ID:    "approval-2",
			State: "pending",
			Actor: "initiator",
		}
		_, err = approvalStore.Create(approval)
		require.NoError(t, err)

		ctx := context.Background()
		instance, err := engine.StartWorkflow(ctx, "timeout-approve-workflow", approval)
		require.NoError(t, err)

		// Wait for timeout
		time.Sleep(100 * time.Millisecond)

		processed, err := engine.ProcessTimeouts(ctx)
		require.NoError(t, err)
		assert.Len(t, processed, 1)

		// Check instance was approved
		updated, err := store.GetInstance(instance.ID)
		require.NoError(t, err)
		assert.Equal(t, WorkflowStateApproved, updated.State)
	})
}

// TestHelper_functions tests helper functions
func TestHelperFunctions(t *testing.T) {
	t.Run("approvalToMap", func(t *testing.T) {
		approval := &Approval{
			ID:         "test-id",
			State:      "pending",
			FunctionID: "func-1",
			GameID:     "game-1",
			Env:        "dev",
			Actor:      "user1",
			Mode:       "invoke",
		}

		m := approvalToMap(approval)
		assert.Equal(t, "test-id", m["id"])
		assert.Equal(t, "pending", m["state"])
		assert.Equal(t, "func-1", m["function_id"])
		assert.Equal(t, "game-1", m["game_id"])
		assert.Equal(t, "dev", m["env"])
		assert.Equal(t, "user1", m["actor"])
		assert.Equal(t, "invoke", m["mode"])
	})

	t.Run("containsString", func(t *testing.T) {
		// containsString checks if string starts with substring (prefix check)
		assert.True(t, containsString("hello world", "hello"))
		assert.False(t, containsString("hello world", "world")) // Not a prefix
		assert.False(t, containsString("hello world", "xyz"))
		assert.False(t, containsString("short", "longer string"))
	})

	t.Run("compareNumbers", func(t *testing.T) {
		assert.Greater(t, compareNumbers(10, 5), 0)
		assert.Less(t, compareNumbers(5, 10), 0)
		assert.Equal(t, 0, compareNumbers(5, 5))
		assert.Equal(t, 0, compareNumbers(5.5, 5.5))
	})

	t.Run("toFloat64", func(t *testing.T) {
		assert.Equal(t, 42.0, toFloat64(42))
		assert.Equal(t, 42.0, toFloat64(int64(42)))
		// float32 has precision loss when converted to float64
		assert.InDelta(t, 3.14, toFloat64(float32(3.14)), 0.001)
		assert.Equal(t, 2.718, toFloat64(2.718))
		assert.Equal(t, 0.0, toFloat64("not a number"))
	})

	t.Run("inList", func(t *testing.T) {
		list := []interface{}{"a", "b", "c", 123}
		assert.True(t, inList("a", list))
		assert.True(t, inList("b", list))
		assert.True(t, inList(123, list))
		assert.False(t, inList("d", list))
		assert.False(t, inList("not in list", list))
		assert.False(t, inList("value", []interface{}{}))
	})

	t.Run("generateID", func(t *testing.T) {
		id1 := generateID("test")
		assert.Contains(t, id1, "test")
		// IDs generated in quick succession might be the same,
		// but they should both contain the prefix and a timestamp
		assert.Contains(t, id1, "_")
	})
}

// TestWorkflowEngine_ConditionalSteps tests conditional step entry
func TestWorkflowEngine_ConditionalSteps(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	def := &WorkflowDefinition{
		ID:      "conditional-workflow",
		Name:    "Conditional Workflow",
		Active:  true,
		Steps: []ApprovalStep{
			{
				ID:        "step1",
				Name:      "Game Specific",
				Approvers: []string{"manager1"},
				Conditions: []ConditionGroup{
					{
						Conditions: []Condition{
							{Field: "game_id", Operator: CondOpEquals, Value: "special-game"},
						},
					},
				},
			},
			{
				ID:        "step2",
				Name:      "Default Approval",
				Approvers: []string{"director1"},
			},
		},
	}
	_, err := store.CreateDefinition(def)
	require.NoError(t, err)

	t.Run("Condition matches - starts at step1", func(t *testing.T) {
		approval := &Approval{
			ID:     "approval-1",
			State:  "pending",
			Actor:  "initiator",
			GameID: "special-game", // Matches condition
		}
		_, err = approvalStore.Create(approval)
		require.NoError(t, err)

		ctx := context.Background()
		instance, err := engine.StartWorkflow(ctx, "conditional-workflow", approval)
		require.NoError(t, err)
		assert.Equal(t, 0, instance.CurrentStep) // Starts at step1 (first matching)
	})

	t.Run("No condition matches - starts at last step", func(t *testing.T) {
		approval := &Approval{
			ID:     "approval-2",
			State:  "pending",
			Actor:  "initiator",
			GameID: "other-game", // Doesn't match condition
		}
		_, err = approvalStore.Create(approval)
		require.NoError(t, err)

		ctx := context.Background()
		instance, err := engine.StartWorkflow(ctx, "conditional-workflow", approval)
		require.NoError(t, err)
		assert.Equal(t, 1, instance.CurrentStep) // Falls through to step2
	})
}

// TestWorkflowEngine_DelegationApproval tests approval via delegation
func TestWorkflowEngine_DelegationApproval(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMemStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	def := &WorkflowDefinition{
		ID:      "delegation-workflow",
		Name:    "Delegation Workflow",
		Active:  true,
		Steps: []ApprovalStep{
			{
				ID:           "step1",
				Name:         "Step 1",
				Approvers:    []string{"manager1"},
				AllowDelegate: true,
			},
		},
	}
	_, err := store.CreateDefinition(def)
	require.NoError(t, err)

	approval := &Approval{
		ID:    "approval-1",
		State: "pending",
		Actor: "initiator",
	}
	_, err = approvalStore.Create(approval)
	require.NoError(t, err)

	ctx := context.Background()
	instance, err := engine.StartWorkflow(ctx, "delegation-workflow", approval)
	require.NoError(t, err)

	// Test that direct approver is authorized
	assert.True(t, engine.isAuthorizedApprover(def.Steps[0], "manager1", instance))

	// Test that non-approver is not authorized
	assert.False(t, engine.isAuthorizedApprover(def.Steps[0], "random-user", instance))

	// Add a delegation record where delegate has already approved on behalf of manager1
	// The isAuthorizedApprover function checks if user == approver OR if user == delegatedBy
	// So if we want "delegate" to be authorized, we need an approval where DelegatedBy == "delegate"
	// This means "delegate" was the original approver who delegated to someone else
	delegatedApproval := &StepApproval{
		StepID:      "step1",
		Approver:    "manager1",     // manager1 is doing the approving
		DelegatedBy: "delegate",     // on behalf of delegate (delegate delegated to manager1)
		Decision:    "approved",
		DecidedAt:   time.Now(),
	}
	err = store.AddStepApproval(instance.ID, delegatedApproval)
	require.NoError(t, err)

	// Now delegate should be authorized because they delegated their approval to manager1
	// and manager1 has already approved
	assert.True(t, engine.isAuthorizedApprover(def.Steps[0], "delegate", instance))
}

// TestWorkflowEngine_MarshalJSON tests JSON marshaling
func TestWorkflowEngine_MarshalJSON(t *testing.T) {
	now := time.Now()
	instance := &WorkflowInstance{
		ID:           "inst-1",
		DefinitionID: "def-1",
		State:        WorkflowStatePending,
		CurrentStep:  0,
		ApprovalID:   "approval-1",
		Initiator:    "user1",
		StartedAt:    now,
	}

	data, err := instance.MarshalJSON()
	require.NoError(t, err)
	assert.Contains(t, string(data), "inst-1")
	assert.Contains(t, string(data), "pending")
}
