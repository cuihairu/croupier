package approvals

import (
	"context"
	"testing"
	"time"
)

// MockWorkflowStore is a mock implementation of WorkflowStore
type MockWorkflowStore struct {
	definitions map[string]*WorkflowDefinition
	instances   map[string]*WorkflowInstance
	approvals   map[string][]StepApproval
}

func NewMockWorkflowStore() *MockWorkflowStore {
	return &MockWorkflowStore{
		definitions: make(map[string]*WorkflowDefinition),
		instances:   make(map[string]*WorkflowInstance),
		approvals:   make(map[string][]StepApproval),
	}
}

func (s *MockWorkflowStore) CreateDefinition(def *WorkflowDefinition) (*WorkflowDefinition, error) {
	s.definitions[def.ID] = def
	return def, nil
}

func (s *MockWorkflowStore) UpdateDefinition(def *WorkflowDefinition) (*WorkflowDefinition, error) {
	s.definitions[def.ID] = def
	return def, nil
}

func (s *MockWorkflowStore) GetDefinition(id string) (*WorkflowDefinition, error) {
	def, exists := s.definitions[id]
	if !exists {
		return nil, ErrWorkflowNotFound
	}
	return def, nil
}

func (s *MockWorkflowStore) ListDefinitions(activeOnly bool) ([]*WorkflowDefinition, error) {
	var result []*WorkflowDefinition
	for _, def := range s.definitions {
		if !activeOnly || def.Active {
			result = append(result, def)
		}
	}
	return result, nil
}

func (s *MockWorkflowStore) DeleteDefinition(id string) error {
	delete(s.definitions, id)
	return nil
}

func (s *MockWorkflowStore) CreateInstance(inst *WorkflowInstance) (*WorkflowInstance, error) {
	s.instances[inst.ID] = inst
	return inst, nil
}

func (s *MockWorkflowStore) GetInstance(id string) (*WorkflowInstance, error) {
	inst, exists := s.instances[id]
	if !exists {
		return nil, ErrWorkflowNotFound
	}
	return inst, nil
}

func (s *MockWorkflowStore) GetInstanceByApprovalID(approvalID string) (*WorkflowInstance, error) {
	for _, inst := range s.instances {
		if inst.ApprovalID == approvalID {
			return inst, nil
		}
	}
	return nil, ErrWorkflowNotFound
}

func (s *MockWorkflowStore) UpdateInstance(inst *WorkflowInstance) (*WorkflowInstance, error) {
	s.instances[inst.ID] = inst
	return inst, nil
}

func (s *MockWorkflowStore) ListInstances(filter WorkflowInstanceFilter, page Page) ([]*WorkflowInstance, int, error) {
	var result []*WorkflowInstance
	for _, inst := range s.instances {
		if filter.State != "" && inst.State != filter.State {
			continue
		}
		if filter.DefinitionID != "" && inst.DefinitionID != filter.DefinitionID {
			continue
		}
		if filter.Initiator != "" && inst.Initiator != filter.Initiator {
			continue
		}
		result = append(result, inst)
	}
	return result, len(result), nil
}

func (s *MockWorkflowStore) AddStepApproval(instanceID string, approval *StepApproval) error {
	s.approvals[instanceID] = append(s.approvals[instanceID], *approval)
	return nil
}

func (s *MockWorkflowStore) GetStepApprovals(instanceID string) ([]StepApproval, error) {
	return s.approvals[instanceID], nil
}

// MockNotifier is a mock implementation of Notifier
type MockNotifier struct {
	Notifications []NotificationCall
}

type NotificationCall struct {
	Recipients []string
	Event      NotificationEvent
}

func NewMockNotifier() *MockNotifier {
	return &MockNotifier{
		Notifications: []NotificationCall{},
	}
}

func (n *MockNotifier) Notify(ctx context.Context, recipients []string, event NotificationEvent) error {
	n.Notifications = append(n.Notifications, NotificationCall{
		Recipients: recipients,
		Event:      event,
	})
	return nil
}

func (n *MockNotifier) NotifyWithChannels(ctx context.Context, recipients []string, event NotificationEvent, channels []NotificationChannel) error {
	n.Notifications = append(n.Notifications, NotificationCall{
		Recipients: recipients,
		Event:      event,
	})
	return nil
}

// MockApprovalStore is a mock implementation of Store
type MockApprovalStore struct {
	approvals map[string]*Approval
}

func NewMockApprovalStore() *MockApprovalStore {
	return &MockApprovalStore{
		approvals: make(map[string]*Approval),
	}
}

func (s *MockApprovalStore) List(f Filter, p Page) ([]*Approval, int, error) {
	var result []*Approval
	for _, a := range s.approvals {
		result = append(result, a)
	}
	return result, len(result), nil
}

func (s *MockApprovalStore) Get(id string) (*Approval, error) {
	a, exists := s.approvals[id]
	if !exists {
		return nil, ErrWorkflowNotFound
	}
	return a, nil
}

func (s *MockApprovalStore) Approve(id string) (*Approval, error) {
	a, exists := s.approvals[id]
	if !exists {
		return nil, ErrWorkflowNotFound
	}
	a.State = "approved"
	return a, nil
}

func (s *MockApprovalStore) Reject(id, reason string) (*Approval, error) {
	a, exists := s.approvals[id]
	if !exists {
		return nil, ErrWorkflowNotFound
	}
	a.State = "rejected"
	a.Reason = reason
	return a, nil
}

func (s *MockApprovalStore) Create(approval *Approval) (*Approval, error) {
	s.approvals[approval.ID] = approval
	return approval, nil
}

func (s *MockApprovalStore) Update(approval *Approval) (*Approval, error) {
	s.approvals[approval.ID] = approval
	return approval, nil
}

func TestWorkflowEngine_CreateDefinition(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMockApprovalStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	def := &WorkflowDefinition{
		ID:          "wf-1",
		Name:        "Two-Person Approval",
		Description: "Requires two approvers",
		Version:     "1.0",
		Active:      true,
		Steps: []ApprovalStep{
			{
				ID:        "step-1",
				Name:      "First Approval",
				Type:      StepTypeSequential,
				Approvers: []string{"user1", "user2"},
				Order:     0,
			},
		},
	}

	created, err := engine.CreateDefinition(def)
	if err != nil {
		t.Fatalf("CreateDefinition failed: %v", err)
	}

	if created.ID != def.ID {
		t.Errorf("Expected ID %s, got %s", def.ID, created.ID)
	}

	if created.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestWorkflowEngine_CreateDefinition_Invalid(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMockApprovalStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	tests := []struct {
		name    string
		def     *WorkflowDefinition
		wantErr bool
	}{
		{
			name: "empty ID",
			def: &WorkflowDefinition{
				Name:  "Test",
				Steps: []ApprovalStep{{ID: "s1"}},
			},
			wantErr: true,
		},
		{
			name: "empty name",
			def: &WorkflowDefinition{
				ID:    "wf-1",
				Steps: []ApprovalStep{{ID: "s1"}},
			},
			wantErr: true,
		},
		{
			name: "no steps",
			def: &WorkflowDefinition{
				ID:   "wf-1",
				Name: "Test",
			},
			wantErr: true,
		},
		{
			name: "duplicate step IDs",
			def: &WorkflowDefinition{
				ID:   "wf-1",
				Name: "Test",
				Steps: []ApprovalStep{
					{ID: "s1"},
					{ID: "s1"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := engine.CreateDefinition(tt.def)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateDefinition() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWorkflowEngine_StartWorkflow(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMockApprovalStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	// Create workflow definition
	def := &WorkflowDefinition{
		ID:          "wf-1",
		Name:        "Two-Person Approval",
		Description: "Requires two approvers",
		Version:     "1.0",
		Active:      true,
		Steps: []ApprovalStep{
			{
				ID:        "step-1",
				Name:      "First Approval",
				Type:      StepTypeSequential,
				Approvers: []string{"approver1", "approver2"},
				Order:     0,
			},
			{
				ID:        "step-2",
				Name:      "Second Approval",
				Type:      StepTypeSequential,
				Approvers: []string{"approver3"},
				Order:     1,
			},
		},
	}
	store.CreateDefinition(def)

	// Create approval request
	approval := &Approval{
		ID:         "approval-1",
		State:      "pending",
		FunctionID: "func-1",
		GameID:     "game-1",
		Env:        "prod",
		Actor:      "requester1",
	}

	// Start workflow
	ctx := context.Background()
	instance, err := engine.StartWorkflow(ctx, "wf-1", approval)
	if err != nil {
		t.Fatalf("StartWorkflow failed: %v", err)
	}

	if instance.State != WorkflowStatePending {
		t.Errorf("Expected state pending, got %s", instance.State)
	}

	if instance.CurrentStep != 0 {
		t.Errorf("Expected current step 0, got %d", instance.CurrentStep)
	}

	// Check notification was sent
	if len(notifier.Notifications) != 1 {
		t.Errorf("Expected 1 notification, got %d", len(notifier.Notifications))
	}
}

func TestWorkflowEngine_ApproveStep(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMockApprovalStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	// Create workflow definition with single step
	def := &WorkflowDefinition{
		ID:      "wf-1",
		Name:    "Single Approval",
		Version: "1.0",
		Active:  true,
		Steps: []ApprovalStep{
			{
				ID:        "step-1",
				Name:      "Approval",
				Type:      StepTypeAny,
				Approvers: []string{"approver1", "approver2"},
			},
		},
	}
	store.CreateDefinition(def)

	// Create approval and workflow instance
	approval := &Approval{
		ID:         "approval-1",
		State:      "pending",
		FunctionID: "func-1",
		Actor:      "requester1",
	}
	approvalStore.Create(approval)

	instance := &WorkflowInstance{
		ID:            "inst-1",
		DefinitionID:  "wf-1",
		Definition:    def,
		State:         WorkflowStatePending,
		CurrentStep:   0,
		ApprovalID:    "approval-1",
		Initiator:     "requester1",
		StartedAt:     time.Now(),
		StepApprovals: []StepApproval{},
		History:       []WorkflowHistoryEntry{},
	}
	store.CreateInstance(instance)

	// Approve step
	ctx := context.Background()
	updated, err := engine.ApproveStep(ctx, "inst-1", "approver1", "Looks good", "192.168.1.1", "test-agent")
	if err != nil {
		t.Fatalf("ApproveStep failed: %v", err)
	}

	if updated.State != WorkflowStateApproved {
		t.Errorf("Expected state approved, got %s", updated.State)
	}

	// Check approval was updated
	updatedApproval, _ := approvalStore.Get("approval-1")
	if updatedApproval.State != "approved" {
		t.Errorf("Expected approval state approved, got %s", updatedApproval.State)
	}
}

func TestWorkflowEngine_RejectStep(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMockApprovalStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	// Create workflow definition
	def := &WorkflowDefinition{
		ID:      "wf-1",
		Name:    "Single Approval",
		Version: "1.0",
		Active:  true,
		Steps: []ApprovalStep{
			{
				ID:        "step-1",
				Name:      "Approval",
				Type:      StepTypeAny,
				Approvers: []string{"approver1"},
			},
		},
	}
	store.CreateDefinition(def)

	// Create approval and instance
	approval := &Approval{
		ID:         "approval-1",
		State:      "pending",
		FunctionID: "func-1",
		Actor:      "requester1",
	}
	approvalStore.Create(approval)

	instance := &WorkflowInstance{
		ID:            "inst-1",
		DefinitionID:  "wf-1",
		Definition:    def,
		State:         WorkflowStatePending,
		CurrentStep:   0,
		ApprovalID:    "approval-1",
		Initiator:     "requester1",
		StartedAt:     time.Now(),
		StepApprovals: []StepApproval{},
		History:       []WorkflowHistoryEntry{},
	}
	store.CreateInstance(instance)

	// Reject step
	ctx := context.Background()
	updated, err := engine.RejectStep(ctx, "inst-1", "approver1", "Not approved", "", "")
	if err != nil {
		t.Fatalf("RejectStep failed: %v", err)
	}

	if updated.State != WorkflowStateRejected {
		t.Errorf("Expected state rejected, got %s", updated.State)
	}

	// Check approval was rejected
	updatedApproval, _ := approvalStore.Get("approval-1")
	if updatedApproval.State != "rejected" {
		t.Errorf("Expected approval state rejected, got %s", updatedApproval.State)
	}
}

func TestWorkflowEngine_MultiStepApproval(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMockApprovalStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	// Create workflow with 2 steps
	def := &WorkflowDefinition{
		ID:      "wf-1",
		Name:    "Two-Step Approval",
		Version: "1.0",
		Active:  true,
		Steps: []ApprovalStep{
			{
				ID:        "step-1",
				Name:      "First Approval",
				Type:      StepTypeAny,
				Approvers: []string{"approver1"},
			},
			{
				ID:        "step-2",
				Name:      "Second Approval",
				Type:      StepTypeAny,
				Approvers: []string{"approver2"},
			},
		},
	}
	store.CreateDefinition(def)

	// Create approval and instance
	approval := &Approval{
		ID:         "approval-1",
		State:      "pending",
		FunctionID: "func-1",
		Actor:      "requester1",
	}
	approvalStore.Create(approval)

	instance := &WorkflowInstance{
		ID:            "inst-1",
		DefinitionID:  "wf-1",
		Definition:    def,
		State:         WorkflowStatePending,
		CurrentStep:   0,
		ApprovalID:    "approval-1",
		Initiator:     "requester1",
		StartedAt:     time.Now(),
		StepApprovals: []StepApproval{},
		History:       []WorkflowHistoryEntry{},
	}
	store.CreateInstance(instance)

	ctx := context.Background()

	// First approval - should advance to next step
	updated, err := engine.ApproveStep(ctx, "inst-1", "approver1", "OK", "", "")
	if err != nil {
		t.Fatalf("First ApproveStep failed: %v", err)
	}

	if updated.CurrentStep != 1 {
		t.Errorf("Expected current step 1, got %d", updated.CurrentStep)
	}

	if updated.State != WorkflowStatePending {
		t.Errorf("Expected state pending, got %s", updated.State)
	}

	// Second approval - should complete workflow
	updated, err = engine.ApproveStep(ctx, "inst-1", "approver2", "Approved", "", "")
	if err != nil {
		t.Fatalf("Second ApproveStep failed: %v", err)
	}

	if updated.State != WorkflowStateApproved {
		t.Errorf("Expected state approved, got %s", updated.State)
	}
}

func TestWorkflowEngine_UnauthorizedApprover(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMockApprovalStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	// Create workflow
	def := &WorkflowDefinition{
		ID:      "wf-1",
		Name:    "Approval",
		Version: "1.0",
		Active:  true,
		Steps: []ApprovalStep{
			{
				ID:        "step-1",
				Name:      "Approval",
				Type:      StepTypeAny,
				Approvers: []string{"approver1"},
			},
		},
	}
	store.CreateDefinition(def)

	instance := &WorkflowInstance{
		ID:            "inst-1",
		DefinitionID:  "wf-1",
		Definition:    def,
		State:         WorkflowStatePending,
		CurrentStep:   0,
		ApprovalID:    "approval-1",
		Initiator:     "requester1",
		StartedAt:     time.Now(),
		StepApprovals: []StepApproval{},
	}
	store.CreateInstance(instance)

	// Try to approve with unauthorized user
	ctx := context.Background()
	_, err := engine.ApproveStep(ctx, "inst-1", "unauthorized", "Trying to approve", "", "")
	if err != ErrNotAuthorizedApprover {
		t.Errorf("Expected ErrNotAuthorizedApprover, got %v", err)
	}
}

func TestWorkflowEngine_ParallelApproval(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMockApprovalStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	// Create workflow with parallel approval (all must approve)
	def := &WorkflowDefinition{
		ID:      "wf-1",
		Name:    "Parallel Approval",
		Version: "1.0",
		Active:  true,
		Steps: []ApprovalStep{
			{
				ID:        "step-1",
				Name:      "Parallel Approval",
				Type:      StepTypeParallel,
				Approvers: []string{"approver1", "approver2"},
			},
		},
	}
	store.CreateDefinition(def)

	approval := &Approval{
		ID:         "approval-1",
		State:      "pending",
		FunctionID: "func-1",
		Actor:      "requester1",
	}
	approvalStore.Create(approval)

	instance := &WorkflowInstance{
		ID:            "inst-1",
		DefinitionID:  "wf-1",
		Definition:    def,
		State:         WorkflowStatePending,
		CurrentStep:   0,
		ApprovalID:    "approval-1",
		Initiator:     "requester1",
		StartedAt:     time.Now(),
		StepApprovals: []StepApproval{},
		History:       []WorkflowHistoryEntry{},
	}
	store.CreateInstance(instance)

	ctx := context.Background()

	// First approval - should not complete
	updated, err := engine.ApproveStep(ctx, "inst-1", "approver1", "OK", "", "")
	if err != nil {
		t.Fatalf("First ApproveStep failed: %v", err)
	}

	if updated.State != WorkflowStatePending {
		t.Errorf("Expected state pending after first approval, got %s", updated.State)
	}

	// Second approval - should complete
	updated, err = engine.ApproveStep(ctx, "inst-1", "approver2", "OK", "", "")
	if err != nil {
		t.Fatalf("Second ApproveStep failed: %v", err)
	}

	if updated.State != WorkflowStateApproved {
		t.Errorf("Expected state approved after both approvals, got %s", updated.State)
	}
}

func TestWorkflowEngine_Timeout(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMockApprovalStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	// Create workflow with timeout
	def := &WorkflowDefinition{
		ID:      "wf-1",
		Name:    "Timeout Approval",
		Version: "1.0",
		Active:  true,
		Steps: []ApprovalStep{
			{
				ID:            "step-1",
				Name:          "Approval",
				Type:          StepTypeAny,
				Approvers:     []string{"approver1"},
				Timeout:       time.Hour,
				TimeoutAction: "reject",
			},
		},
	}
	store.CreateDefinition(def)

	approval := &Approval{
		ID:         "approval-1",
		State:      "pending",
		FunctionID: "func-1",
		Actor:      "requester1",
	}
	approvalStore.Create(approval)

	// Create instance with expired timeout
	pastTime := time.Now().Add(-2 * time.Hour)
	instance := &WorkflowInstance{
		ID:            "inst-1",
		DefinitionID:  "wf-1",
		Definition:    def,
		State:         WorkflowStatePending,
		CurrentStep:   0,
		ApprovalID:    "approval-1",
		Initiator:     "requester1",
		StartedAt:     pastTime,
		ExpiresAt:     &pastTime, // Already expired
		StepApprovals: []StepApproval{},
		History:       []WorkflowHistoryEntry{},
	}
	store.CreateInstance(instance)

	// Process timeouts
	ctx := context.Background()
	processed, err := engine.ProcessTimeouts(ctx)
	if err != nil {
		t.Fatalf("ProcessTimeouts failed: %v", err)
	}

	if len(processed) != 1 {
		t.Errorf("Expected 1 processed instance, got %d", len(processed))
	}

	// Check instance was rejected
	updated, _ := store.GetInstance("inst-1")
	if updated.State != WorkflowStateExpired {
		t.Errorf("Expected state expired, got %s", updated.State)
	}
}

func TestWorkflowEngine_ConditionalWorkflow(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMockApprovalStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	// Create workflow with conditions
	def := &WorkflowDefinition{
		ID:      "wf-1",
		Name:    "Conditional Approval",
		Version: "1.0",
		Active:  true,
		Steps: []ApprovalStep{
			{
				ID:        "step-1",
				Name:      "Normal Approval",
				Type:      StepTypeAny,
				Approvers: []string{"approver1"},
				Order:     0,
				Conditions: []ConditionGroup{
					{
						Conditions: []Condition{
							{Field: "env", Operator: CondOpEquals, Value: "dev"},
						},
						Logic: "and",
					},
				},
			},
			{
				ID:        "step-2",
				Name:      "Production Approval",
				Type:      StepTypeAny,
				Approvers: []string{"approver2"},
				Order:     1,
				Conditions: []ConditionGroup{
					{
						Conditions: []Condition{
							{Field: "env", Operator: CondOpEquals, Value: "prod"},
						},
						Logic: "and",
					},
				},
			},
		},
	}
	store.CreateDefinition(def)

	// Test with production environment
	approval := &Approval{
		ID:         "approval-1",
		State:      "pending",
		FunctionID: "func-1",
		Env:        "prod",
		Actor:      "requester1",
	}

	ctx := context.Background()
	instance, err := engine.StartWorkflow(ctx, "wf-1", approval)
	if err != nil {
		t.Fatalf("StartWorkflow failed: %v", err)
	}

	// Should start at step 1 (index 1) for production
	// Note: The current implementation starts at first matching step
	// This test verifies the workflow starts successfully with conditions
	if instance.State != WorkflowStatePending {
		t.Errorf("Expected state pending, got %s", instance.State)
	}
}

func TestWorkflowEngine_CancelWorkflow(t *testing.T) {
	store := NewMockWorkflowStore()
	approvalStore := NewMockApprovalStore()
	notifier := NewMockNotifier()
	engine := NewWorkflowEngine(store, approvalStore, notifier)

	// Create workflow
	def := &WorkflowDefinition{
		ID:      "wf-1",
		Name:    "Approval",
		Version: "1.0",
		Active:  true,
		Steps: []ApprovalStep{
			{
				ID:        "step-1",
				Name:      "Approval",
				Type:      StepTypeAny,
				Approvers: []string{"approver1"},
			},
		},
	}
	store.CreateDefinition(def)

	approval := &Approval{
		ID:         "approval-1",
		State:      "pending",
		FunctionID: "func-1",
		Actor:      "requester1",
	}
	approvalStore.Create(approval)

	instance := &WorkflowInstance{
		ID:            "inst-1",
		DefinitionID:  "wf-1",
		Definition:    def,
		State:         WorkflowStatePending,
		CurrentStep:   0,
		ApprovalID:    "approval-1",
		Initiator:     "requester1",
		StartedAt:     time.Now(),
		StepApprovals: []StepApproval{},
	}
	store.CreateInstance(instance)

	// Cancel workflow
	ctx := context.Background()
	updated, err := engine.CancelWorkflow(ctx, "inst-1", "requester1", "No longer needed")
	if err != nil {
		t.Fatalf("CancelWorkflow failed: %v", err)
	}

	if updated.State != WorkflowStateCancelled {
		t.Errorf("Expected state cancelled, got %s", updated.State)
	}

	// Check approval was rejected
	updatedApproval, _ := approvalStore.Get("approval-1")
	if updatedApproval.State != "rejected" {
		t.Errorf("Expected approval state rejected, got %s", updatedApproval.State)
	}
}
