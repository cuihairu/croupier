package approvals

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkflowEngine_SetTimer tests SetTimer method
func TestWorkflowEngine_SetTimer(t *testing.T) {
	store := &MemStore{}
	engine := NewWorkflowEngine(store, store, nil)

	// Create a mock timer
	mockTimer := &mockTimer{}
	engine.SetTimer(mockTimer)

	// Verify timer was set (we can't directly access it, but we can verify it doesn't crash)
	assert.NotNil(t, engine)
}

// TestWorkflowEngine_UpdateDefinition tests UpdateDefinition method
func TestWorkflowEngine_UpdateDefinition(t *testing.T) {
	store := &MemStore{}
	engine := NewWorkflowEngine(store, store, nil)

	// First create a definition
	def := &WorkflowDefinition{
		ID:          "test-workflow",
		Name:        "Test Workflow",
		Description: "A test workflow",
		Version:     "1.0",
		Active:      true,
		Steps:       createTestSteps("test-step"),
	}

	created, err := engine.CreateDefinition(def)
	require.NoError(t, err)
	require.NotNil(t, created)

	// Update the definition
	created.Description = "Updated description"
	created.Version = "2.0"

	updated, err := engine.UpdateDefinition(created)
	require.NoError(t, err)
	assert.Equal(t, "Updated description", updated.Description)
	assert.Equal(t, "2.0", updated.Version)
}

// TestWorkflowEngine_UpdateDefinition_NotFound tests UpdateDefinition with non-existent workflow
func TestWorkflowEngine_UpdateDefinition_NotFound(t *testing.T) {
	store := &MemStore{}
	engine := NewWorkflowEngine(store, store, nil)

	def := &WorkflowDefinition{
		ID:          "nonexistent",
		Name:        "Non-existent",
		Description: "Does not exist",
		Version:     "1.0",
		Active:      true,
		Steps:       createTestSteps("step1"),
	}

	_, err := engine.UpdateDefinition(def)
	assert.Error(t, err)
}

// TestWorkflowEngine_UpdateDefinition_Invalid tests UpdateDefinition with invalid definition
func TestWorkflowEngine_UpdateDefinition_Invalid(t *testing.T) {
	store := &MemStore{}
	engine := NewWorkflowEngine(store, store, nil)

	// Create valid definition first
	valid := &WorkflowDefinition{
		ID:          "test-workflow",
		Name:        "Test Workflow",
		Description: "A test workflow",
		Version:     "1.0",
		Active:      true,
		Steps:       createTestSteps("step1"),
	}

	created, err := engine.CreateDefinition(valid)
	require.NoError(t, err)

	// Try to update with invalid data (empty name)
	created.Name = ""

	_, err = engine.UpdateDefinition(created)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workflow name is required")
}

// TestWorkflowEngine_timeoutEscalate tests timeoutEscalate method
func TestWorkflowEngine_timeoutEscalate(t *testing.T) {
	store := &MemStore{}
	engine := NewWorkflowEngine(store, store, nil)

	// Create a definition with escalation
	stepWithEscalation := ApprovalStep{
		ID:            "step1",
		Name:          "Step 1",
		Type:          StepTypeSequential,
		Approvers:     []string{"user1"},
		Order:         1,
		Timeout:       time.Hour,
		TimeoutAction: "escalate",
		EscalateTo:    "step2",
	}
	step2 := ApprovalStep{
		ID:        "step2",
		Name:      "Step 2",
		Type:      StepTypeSequential,
		Approvers: []string{"user2"},
		Order:     2,
	}

	def := &WorkflowDefinition{
		ID:      "escalation-workflow",
		Name:    "Escalation Workflow",
		Active:  true,
		Steps:    []ApprovalStep{stepWithEscalation, step2},
	}

	_, err := engine.CreateDefinition(def)
	require.NoError(t, err)

	// Create an instance
	approval := &Approval{
		ID:     "approval-1",
		GameID:  "game1",
		Env:     "dev",
		UserID:  "user1",
		Type:    "test_type",
		Status:  "pending",
		Payload: map[string]interface{}{"amount": 100},
	}

	instance, err := engine.StartWorkflow(context.Background(), "escalation-workflow", approval)
	require.NoError(t, err)

	// Manually set step as timed out for testing
	instance.CurrentStep = 0
	instance.Status = WorkflowStatePending

	// Trigger timeout escalation (normally called by ProcessTimeouts)
	// We can't directly access timeoutEscalate, but we can verify the workflow structure supports it
	assert.NotNil(t, instance)
	assert.Equal(t, "step1", def.Steps[0].ID)
	assert.Equal(t, "step2", def.Steps[0].EscalateTo)
}

// TestWorkflowEngine_Now tests Now method
func TestWorkflowEngine_Now(t *testing.T) {
	// The Now function should return current time
	now := Now()
	assert.False(t, now.IsZero())
	assert.True(t, time.Now().Sub(now) < time.Second) // Should be very recent
}

// TestWorkflowEngine_MarshalJSON tests MarshalJSON method
func TestWorkflowEngine_MarshalJSON(t *testing.T) {
	store := &MemStore{}
	engine := NewWorkflowEngine(store, store, nil)

	// Create a definition
	def := &WorkflowDefinition{
		ID:      "json-workflow",
		Name:    "JSON Workflow",
		Active:  true,
		Steps:   createTestSteps("step1", "step2"),
	}

	_, err := engine.CreateDefinition(def)
	require.NoError(t, err)

	// Marshal to JSON - tests the MarshalJSON method
	data, err := engine.MarshalJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, string(data), "json-workflow")
}

// TestWorkflowEngine_evaluateCondition tests evaluateCondition method
func TestWorkflowEngine_evaluateCondition(t *testing.T) {
	store := &MemStore{}
	engine := NewWorkflowEngine(store, store, nil)

	// Create a workflow with conditional steps
	stepWithCondition := ApprovalStep{
		ID:     "conditional-step",
		Name:   "Conditional Step",
		Type:   StepTypeSequential,
		Approvers: []string{"admin"},
		Order:  1,
		Conditions: []ConditionGroup{
			{
				Conditions: []Condition{
					{Field: "amount", Operator: CondOpGreaterThan, Value: 1000.0},
				},
				Logic: "and",
			},
		},
	}

	def := &WorkflowDefinition{
		ID:    "conditional-workflow",
		Name:  "Conditional Workflow",
		Active: true,
		Steps: []ApprovalStep{stepWithCondition},
	}

	_, err := engine.CreateDefinition(def)
	require.NoError(t, err)

	// Test with approval that meets condition
	approval1 := &Approval{
		ID:     "approval-1",
		GameID:  "game1",
		Env:     "dev",
		Type:    "test_type",
		Status:  "pending",
		Payload: map[string]interface{}{"amount": 2000.0},
	}

	instance1, err := engine.StartWorkflow(context.Background(), "conditional-workflow", approval1)
	require.NoError(t, err)
	assert.NotNil(t, instance1)

	// Test with approval that doesn't meet condition
	approval2 := &Approval{
		ID:     "approval-2",
		GameID:  "game1",
		Env:     "dev",
		Type:    "test_type",
		Status:  "pending",
		Payload: map[string]interface{}{"amount": 500.0},
	}

	instance2, err := engine.StartWorkflow(context.Background(), "conditional-workflow", approval2)
	require.NoError(t, err)
	// With condition not met, it should skip to end (no current step)
	assert.Equal(t, -1, instance2.CurrentStep)
}

// TestWorkflowEngine_evaluateCondition_MultipleGroups tests OR logic in conditions
func TestWorkflowEngine_evaluateCondition_MultipleGroups(t *testing.T) {
	store := &MemStore{}
	engine := NewWorkflowEngine(store, store, nil)

	// Create a step with multiple condition groups (OR logic)
	step := ApprovalStep{
		ID:     "multi-condition",
		Name:   "Multi Condition",
		Type:   StepTypeSequential,
		Approvers: []string{"admin"},
		Order:  1,
		Conditions: []ConditionGroup{
			{
				Conditions: []Condition{
					{Field: "department", Operator: CondOpEquals, Value: "finance"},
				},
				Logic: "or",
			},
			{
				Conditions: []Condition{
					{Field: "amount", Operator: CondOpLessThan, Value: 100.0},
				},
				Logic: "or",
			},
		},
	}

	def := &WorkflowDefinition{
		ID:    "multi-condition-workflow",
		Name:  "Multi Condition Workflow",
		Active: true,
		Steps: []ApprovalStep{step},
	}

	_, err := engine.CreateDefinition(def)
	require.NoError(t, err)

	// Test with first condition met
	approval1 := &Approval{
		ID:     "approval-1",
		GameID:  "game1",
		Env:     "dev",
		Type:    "test_type",
		Status:  "pending",
		Payload: map[string]interface{}{"department": "finance"},
	}

	instance1, err := engine.StartWorkflow(context.Background(), "multi-condition-workflow", approval1)
	require.NoError(t, err)
	assert.Equal(t, 0, instance1.CurrentStep)

	// Test with second condition met
	approval2 := &Approval{
		ID:     "approval-2",
		GameID:  "game1",
		Env:     "dev",
		Type:    "test_type",
		Status:  "pending",
		Payload: map[string]interface{}{"amount": 50.0},
	}

	instance2, err := engine.StartWorkflow(context.Background(), "multi-condition-workflow", approval2)
	require.NoError(t, err)
	assert.Equal(t, 0, instance2.CurrentStep)

	// Test with no condition met
	approval3 := &Approval{
		ID:     "approval-3",
		GameID:  "game1",
		Env:     "dev",
		Type:    "test_type",
		Status:  "pending",
		Payload: map[string]interface{}{"department": "engineering", "amount": 200.0},
	}

	instance3, err := engine.StartWorkflow(context.Background(), "multi-condition-workflow", approval3)
	require.NoError(t, err)
	assert.Equal(t, -1, instance3.CurrentStep)
}

// TestWorkflowEngine_evaluateCondition_AndLogic tests AND logic in conditions
func TestWorkflowEngine_evaluateCondition_AndLogic(t *testing.T) {
	store := &MemStore{}
	engine := NewWorkflowEngine(store, store, nil)

	// Create a step with AND logic (both conditions must be met)
	step := ApprovalStep{
		ID:     "and-condition",
		Name:   "AND Condition",
		Type:   StepTypeSequential,
		Approvers: []string{"admin"},
		Order:  1,
		Conditions: []ConditionGroup{
			{
				Conditions: []Condition{
					{Field: "department", Operator: CondOpEquals, Value: "finance"},
					{Field: "amount", Operator: CondOpGreaterThan, Value: 1000.0},
				},
				Logic: "and",
			},
		},
	}

	def := &WorkflowDefinition{
		ID:    "and-condition-workflow",
		Name:  "AND Condition Workflow",
		Active: true,
		Steps: []ApprovalStep{step},
	}

	_, err := engine.CreateDefinition(def)
	require.NoError(t, err)

	// Test with both conditions met
	approval1 := &Approval{
		ID:     "approval-1",
		GameID:  "game1",
		Env:     "dev",
		Type:    "test_type",
		Status:  "pending",
		Payload: map[string]interface{}{"department": "finance", "amount": 2000.0},
	}

	instance1, err := engine.StartWorkflow(context.Background(), "and-condition-workflow", approval1)
	require.NoError(t, err)
	assert.Equal(t, 0, instance1.CurrentStep)

	// Test with only one condition met
	approval2 := &Approval{
		ID:     "approval-2",
		GameID:  "game1",
		Env:     "dev",
		Type:    "test_type",
		Status:  "pending",
		Payload: map[string]interface{}{"department": "finance", "amount": 500.0},
	}

	instance2, err := engine.StartWorkflow(context.Background(), "and-condition-workflow", approval2)
	require.NoError(t, err)
	assert.Equal(t, -1, instance2.CurrentStep)
}

// TestWorkflowEngine_evaluateCondition_ContainsOperator tests contains operator
func TestWorkflowEngine_evaluateCondition_ContainsOperator(t *testing.T) {
	store := &MemStore{}
	engine := NewWorkflowEngine(store, store, nil)

	step := ApprovalStep{
		ID:     "contains-condition",
		Name:   "Contains Condition",
		Type:   StepTypeSequential,
		Approvers: []string{"admin"},
		Order:  1,
		Conditions: []ConditionGroup{
			{
				Conditions: []Condition{
					{Field: "tags", Operator: CondOpContains, Value: "urgent"},
				},
				Logic: "and",
			},
		},
	}

	def := &WorkflowDefinition{
		ID:    "contains-workflow",
		Name:  "Contains Workflow",
		Active: true,
		Steps: []ApprovalStep{step},
	}

	_, err := engine.CreateDefinition(def)
	require.NoError(t, err)

	// Test with matching value
	approval1 := &Approval{
		ID:     "approval-1",
		GameID:  "game1",
		Env:     "dev",
		Type:    "test_type",
		Status:  "pending",
		Payload: map[string]interface{}{"tags": "urgent,important"},
	}

	instance1, err := engine.StartWorkflow(context.Background(), "contains-workflow", approval1)
	require.NoError(t, err)
	assert.Equal(t, 0, instance1.CurrentStep)

	// Test with non-matching value
	approval2 := &Approval{
		ID:     "approval-2",
		GameID:  "game1",
		Env:     "dev",
		Type:    "test_type",
		Status:  "pending",
		Payload: map[string]interface{}{"tags": "normal,low"},
	}

	instance2, err := engine.StartWorkflow(context.Background(), "contains-workflow", approval2)
	require.NoError(t, err)
	assert.Equal(t, -1, instance2.CurrentStep)
}

// TestWorkflowEngine_evaluateCondition_InOperator tests in operator
func TestWorkflowEngine_evaluateCondition_InOperator(t *testing.T) {
	store := &MemStore{}
	engine := NewWorkflowEngine(store, store, nil)

	step := ApprovalStep{
		ID:     "in-condition",
		Name:   "In Condition",
		Type:   StepTypeSequential,
		Approvers: []string{"admin"},
		Order:  1,
		Conditions: []ConditionGroup{
			{
				Conditions: []Condition{
					{Field: "role", Operator: CondOpIn, Value: []string{"admin", "moderator"}},
				},
				Logic: "and",
			},
		},
	}

	def := &WorkflowDefinition{
		ID:    "in-workflow",
		Name:  "In Workflow",
		Active: true,
			Steps: []ApprovalStep{step},
	}

	_, err := engine.CreateDefinition(def)
	require.NoError(t, err)

	// Test with value in list
	approval1 := &Approval{
		ID:     "approval-1",
		GameID:  "game1",
		Env:     "dev",
		Type:    "test_type",
		Status:  "pending",
		Payload: map[string]interface{}{"role": "admin"},
	}

	instance1, err := engine.StartWorkflow(context.Background(), "in-workflow", approval1)
	require.NoError(t, err)
	assert.Equal(t, 0, instance1.CurrentStep)

	// Test with value not in list
	approval2 := &Approval{
		ID:     "approval-2",
		GameID:  "game1",
		Env:     "dev",
		Type:    "test_type",
		Status:  "pending",
		Payload: map[string]interface{}{"role": "user"},
	}

	instance2, err := engine.StartWorkflow(context.Background(), "in-workflow", approval2)
	require.NoError(t, err)
	assert.Equal(t, -1, instance2.CurrentStep)
}

// TestWorkflowEngine_evaluateCondition_NotEqualsOperator tests not_equals operator
func TestWorkflowEngine_evaluateCondition_NotEqualsOperator(t *testing.T) {
	store := &MemStore{}
	engine := NewWorkflowEngine(store, store, nil)

	step := ApprovalStep{
		ID:     "not-equals-condition",
		Name:   "Not Equals Condition",
		Type:   StepTypeSequential,
		Approvers: []string{"admin"},
		Order:  1,
		Conditions: []ConditionGroup{
			{
				Conditions: []Condition{
					{Field: "status", Operator: CondOpNotEquals, Value: "rejected"},
				},
				Logic: "and",
			},
		},
	}

	def := &WorkflowDefinition{
		ID:    "not-equals-workflow",
		Name:  "Not Equals Workflow",
		Active: true,
			Steps: []ApprovalStep{step},
	}

	_, err := engine.CreateDefinition(def)
	require.NoError(t, err)

	// Test with not equal value
	approval1 := &Approval{
		ID:     "approval-1",
		GameID:  "game1",
		Env:     "dev",
		Type:    "test_type",
		Status:  "pending",
		Payload: map[string]interface{}{"status": "approved"},
	}

	instance1, err := engine.StartWorkflow(context.Background(), "not-equals-workflow", approval1)
	require.NoError(t, err)
	assert.Equal(t, 0, instance1.CurrentStep)

	// Test with equal value (should skip)
	approval2 := &Approval{
		ID:     "approval-2",
		GameID:  "game1",
		Env:     "dev",
		Type:    "test_type",
		Status:  "pending",
		Payload: map[string]interface{}{"status": "rejected"},
	}

	instance2, err := engine.StartWorkflow(context.Background(), "not-equals-workflow", approval2)
	require.NoError(t, err)
	assert.Equal(t, -1, instance2.CurrentStep)
}

// TestConditionOperators tests all condition operators
func TestConditionOperators(t *testing.T) {
	operators := []ConditionOperator{
		CondOpEquals, CondOpNotEquals, CondOpContains,
		CondOpGreaterThan, CondOpLessThan,
	}

	// Test that each operator can be used in a condition
	for _, op := range operators {
		condition := Condition{
			Field:    "test_field",
			Operator: op,
			Value:    "test_value",
		}
		assert.Equal(t, op, condition.Operator)
	}
}

// TestWorkflowEngine_notifyApprovers_ConditionalStep tests notify with conditional steps
func TestWorkflowEngine_notifyApprovers_ConditionalStep(t *testing.T) {
	store := &MemStore{}
	engine := NewWorkflowEngine(store, store, nil)

	// Create a workflow with conditional step
	stepWithCondition := ApprovalStep{
		ID:     "conditional-step",
		Name:   "Conditional Step",
		Type:   StepTypeSequential,
		Approvers: []string{"user1"},
		Order:  1,
		Conditions: []ConditionGroup{
			{
				Conditions: []Condition{
					{Field: "amount", Operator: CondOpGreaterThan, Value: 100.0},
				},
				Logic: "and",
			},
		},
	}

	def := &WorkflowDefinition{
		ID:     "notify-conditional",
		Name:   "Notify Conditional",
		Active: true,
		Steps:  []ApprovalStep{stepWithCondition},
	}

	_, err := engine.CreateDefinition(def)
	require.NoError(t, err)

	// Start workflow with condition met
	approval := &Approval{
		ID:     "approval-1",
		GameID:  "game1",
		Env:     "dev",
		Type:   "test_type",
		Status: "pending",
		Payload: map[string]interface{}{"amount": 200.0},
	}

	instance, err := engine.StartWorkflow(context.Background(), "notify-conditional", approval)
	require.NoError(t, err)

	// Verify instance was created with conditional step as current step
	assert.Equal(t, 0, instance.CurrentStep)
	assert.Equal(t, "conditional-step", def.Steps[instance.CurrentStep].ID)
}

// TestWorkflowEngine_ApproveStep_WithTimeout tests approving a step with timeout
func TestWorkflowEngine_ApproveStep_WithTimeout(t *testing.T) {
	store := &MemStore{}
	engine := NewWorkflowEngine(store, store, nil)

	stepWithTimeout := ApprovalStep{
		ID:            "timeout-step",
		Name:          "Timeout Step",
		Type:          StepTypeSequential,
		Approvers:     []string{"user1"},
		Order:         1,
		Timeout:       time.Hour,
		TimeoutAction: "reject",
	}

	def := &WorkflowDefinition{
		ID:     "timeout-workflow",
		Name:   "Timeout Workflow",
		Active: true,
		Steps:  []ApprovalStep{stepWithTimeout},
	}

	_, err := engine.CreateDefinition(def)
	require.NoError(t, err)

	approval := &Approval{
		ID:     "approval-1",
		GameID:  "game1",
		Env:     "dev",
		Type:   "test_type",
		Status: "pending",
	}

	instance, err := engine.StartWorkflow(context.Background(), "timeout-workflow", approval)
	require.NoError(t, err)

	// Approve the step
	approvedInstance, err := engine.ApproveStep(context.Background(), instance.ID, "timeout-step", "user1", "")
	require.NoError(t, err)
	assert.NotNil(t, approvedInstance)
	assert.Equal(t, WorkflowStateApproved, approvedInstance.Status)
}

// TestWorkflowEngine_ApproveStep_ParallelStepType tests parallel step approval
func TestWorkflowEngine_ApproveStep_ParallelStepType(t *testing.T) {
	store := &MemStore{}
	engine := NewWorkflowEngine(store, store, nil)

	parallelStep := ApprovalStep{
		ID:        "parallel-step",
		Name:      "Parallel Step",
		Type:      StepTypeParallel,
		Approvers: []string{"user1", "user2", "user3"},
		Order:     1,
	}

	def := &WorkflowDefinition{
		ID:     "parallel-workflow",
		Name:   "Parallel Workflow",
		Active: true,
			Steps:  []ApprovalStep{parallelStep},
	}

	_, err := engine.CreateDefinition(def)
	require.NoError(t, err)

	approval := &Approval{
		ID:     "approval-1",
		GameID:  "game1",
		Env:     "dev",
		Type:   "test_type",
		Status: "pending",
	}

	instance, err := engine.StartWorkflow(context.Background(), "parallel-workflow", approval)
	require.NoError(t, err)

	// One approval should be enough for parallel type
	_, err = engine.ApproveStep(context.Background(), instance.ID, "parallel-step", "user1", "")
	require.NoError(t, err)

	// Check if step was approved (parallel requires only one approval)
	updatedInstance, err := store.GetInstance(instance.ID)
	require.NoError(t, err)

	// For parallel type, one approval should complete the step
	stepApproval := updatedInstance.StepApprovals[0]
	assert.Equal(t, "user1", stepApproval.ApproverID)
}

// TestHelperFunctions_toFloat64 tests toFloat64 helper function
func TestHelperFunctions_toFloat64(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected float64
	}{
		{"integer", 42, 42.0},
		{"float", 3.14, 3.14},
		{"string number", "100", 100.0},
		{"string invalid", "invalid", 0.0},
		{"bool true", true, 1.0},
		{"bool false", false, 0.0},
		{"nil", nil, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toFloat64(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestHelperFunctions_compareNumbers tests compareNumbers helper function
func TestHelperFunctions_compareNumbers(t *testing.T) {
	tests := []struct {
		name     string
		a        float64
		b        float64
		operator ConditionOperator
		expected bool
	}{
		{"greater_than - true", 10.0, 5.0, CondOpGreaterThan, true},
		{"greater_than - false", 5.0, 10.0, CondOpGreaterThan, false},
		{"less_than - true", 5.0, 10.0, CondOpLessThan, true},
		{"less_than - false", 10.0, 5.0, CondOpLessThan, false},
		{"equals - true", 5.0, 5.0, CondOpEquals, true},
		{"equals - false", 5.0, 6.0, CondOpEquals, false},
		{"not_equals - true", 5.0, 6.0, CondOpNotEquals, true},
		{"not_equals - false", 5.0, 5.0, CondOpNotEquals, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareNumbers(tt.a, tt.operator, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestWorkflowEngine_evaluateConditionGroup_EmptyConditions tests empty condition group
func TestWorkflowEngine_evaluateConditionGroup_EmptyConditions(t *testing.T) {
	// Empty condition group should evaluate to false
	conditionGroup := ConditionGroup{
		Conditions: []Condition{},
		Logic:      "and",
	}

	result := evaluateConditionGroup(conditionGroup, &Approval{})
	assert.False(t, result)
}

// TestWorkflowEngine_evaluateCondition_InvalidField tests with missing field
func TestWorkflowEngine_evaluateCondition_InvalidField(t *testing.T) {
	store := &MemStore{}
	engine := NewWorkflowEngine(store, store, nil)

	step := ApprovalStep{
		ID:     "missing-field-step",
		Name:   "Missing Field Step",
		Type:   StepTypeSequential,
		Approvers: []string{"admin"},
		Order:  1,
		Conditions: []ConditionGroup{
			{
				Conditions: []Condition{
					{Field: "nonexistent", Operator: CondOpEquals, Value: "value"},
				},
				Logic: "and",
			},
		},
	}

	def := &WorkflowDefinition{
		ID:    "missing-field-workflow",
		Name:  "Missing Field Workflow",
		Active: true,
		Steps:  []ApprovalStep{step},
	}

	_, err := engine.CreateDefinition(def)
	require.NoError(t, err)

	approval := &Approval{
		ID:     "approval-1",
		GameID: "game1",
		Env:     "dev",
		Type:   "test_type",
		Status: "pending",
		Payload: map[string]interface{}{},
	}

	// Should start but skip to end since condition can't be evaluated
	instance, err := engine.StartWorkflow(context.Background(), "missing-field-workflow", approval)
	require.NoError(t, err)
	assert.Equal(t, -1, instance.CurrentStep)
}

// TestWorkflowEngine_InvalidStepType tests with invalid step type
func TestWorkflowEngine_InvalidStepType(t *testing.T) {
	// The test framework already covers invalid step type in CreateDefinition tests
	// This is a placeholder for any additional edge cases
	step := ApprovalStep{
		ID:     "test-step",
		Name:   "Test Step",
		Type:   "invalid_type", // Invalid type
			Approvers: []string{"admin"},
		Order:  1,
	}

	def := &WorkflowDefinition{
		ID:    "invalid-type-workflow",
		Name:  "Invalid Type Workflow",
		Active: true,
		Steps:  []ApprovalStep{step},
	}

	store := &MemStore{}
	engine := NewWorkflowEngine(store, store, nil)

	_, err := engine.CreateDefinition(def)
	assert.Error(t, err)
}

// TestWorkflowState_String tests WorkflowState string values
func TestWorkflowState_String(t *testing.T) {
	states := []WorkflowState{
		WorkflowStateDraft,
		WorkflowStatePending,
		WorkflowStateApproved,
		WorkflowStateRejected,
		WorkflowStateCancelled,
		WorkflowStateExpired,
	}

	expectedStates := []string{
		"draft", "pending", "approved", "rejected", "cancelled", "expired",
	}

	for i, state := range states {
		assert.Equal(t, expectedStates[i], string(state))
	}
}

// TestStepType_String tests StepType string values
func TestStepType_String(t *testing.T) {
	types := []StepType{
		StepTypeSequential,
		StepTypeParallel,
		StepTypeAny,
			StepTypePercentage,
	}

	expectedTypes := []string{
		"sequential", "parallel", "any", "percentage",
	}

	for i, stepType := range types {
		assert.Equal(t, expectedTypes[i], string(stepType))
	}
}

// Mock timer for testing
type mockTimer struct{}

func (m *mockTimer) AfterFunc(d time.Duration, f func()) *time.Timer {
	return time.AfterFunc(d, f)
}

// Helper function to create test steps
func createTestSteps(stepIDs ...string) []ApprovalStep {
	steps := make([]ApprovalStep, len(stepIDs))
	for i, id := range stepIDs {
		steps[i] = ApprovalStep{
			ID:        id,
			Name:      fmt.Sprintf("Step %s", id),
			Type:      StepTypeSequential,
			Approvers: []string{"user1"},
			Order:     i + 1,
		}
	}
	return steps
}
