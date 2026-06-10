package approvals

import (
	"encoding/json"
	"testing"
	"time"
)

func TestWorkflowDefinitionModel_ToDefinition(t *testing.T) {
	steps := []ApprovalStep{
		{ID: "s1", Name: "Step 1", Approvers: []string{"user1"}, RequiredCount: 1},
	}
	stepsJSON, _ := json.Marshal(steps)

	now := time.Now().Truncate(time.Second)
	model := &WorkflowDefinitionModel{
		ID:          "def1",
		Name:        "Test Workflow",
		Description: "A test workflow",
		Version:     "1.0",
		Active:      true,
		StepsJSON:   stepsJSON,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   "admin",
	}

	def, err := model.ToDefinition()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if def.ID != "def1" {
		t.Errorf("expected ID 'def1', got %q", def.ID)
	}
	if def.Name != "Test Workflow" {
		t.Errorf("expected Name 'Test Workflow', got %q", def.Name)
	}
	if len(def.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(def.Steps))
	}
	if def.Steps[0].ID != "s1" {
		t.Errorf("expected step ID 's1', got %q", def.Steps[0].ID)
	}
	if !def.Active {
		t.Error("expected Active to be true")
	}
	if def.CreatedBy != "admin" {
		t.Errorf("expected CreatedBy 'admin', got %q", def.CreatedBy)
	}
}

func TestWorkflowDefinitionModel_ToDefinition_InvalidJSON(t *testing.T) {
	model := &WorkflowDefinitionModel{
		StepsJSON: []byte("invalid json"),
	}

	_, err := model.ToDefinition()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestFromDefinition(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	def := &WorkflowDefinition{
		ID:          "def1",
		Name:        "Test",
		Description: "Desc",
		Version:     "1.0",
		Active:      true,
		Steps: []ApprovalStep{
			{ID: "s1", Name: "Step 1"},
		},
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: "admin",
	}

	model, err := FromDefinition(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if model.ID != "def1" {
		t.Errorf("expected ID 'def1', got %q", model.ID)
	}
	if model.Name != "Test" {
		t.Errorf("expected Name 'Test', got %q", model.Name)
	}

	// Verify StepsJSON is valid
	var steps []ApprovalStep
	if err := json.Unmarshal(model.StepsJSON, &steps); err != nil {
		t.Fatalf("failed to unmarshal StepsJSON: %v", err)
	}
	if len(steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(steps))
	}
}

func TestWorkflowInstanceModel_ToInstance(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	completedAt := now.Add(time.Hour)
	expiresAt := now.Add(24 * time.Hour)

	contextJSON := []byte(`{"key":"value"}`)
	historyJSON := []byte(`[{"action":"start","actor":"user1","timestamp":"2024-01-01T00:00:00Z"}]`)

	model := &WorkflowInstanceModel{
		ID:           "inst1",
		DefinitionID: "def1",
		State:        "running",
		CurrentStep:  1,
		ContextJSON:  contextJSON,
		ApprovalID:   "app1",
		Initiator:    "user1",
		StartedAt:    now,
		CompletedAt:  &completedAt,
		ExpiresAt:    &expiresAt,
		HistoryJSON:  historyJSON,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	inst, err := model.ToInstance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inst.ID != "inst1" {
		t.Errorf("expected ID 'inst1', got %q", inst.ID)
	}
	if inst.State != WorkflowState("running") {
		t.Errorf("expected state 'running', got %q", inst.State)
	}
	if inst.CurrentStep != 1 {
		t.Errorf("expected CurrentStep 1, got %d", inst.CurrentStep)
	}
	if inst.Context["key"] != "value" {
		t.Errorf("expected context key=value, got %v", inst.Context)
	}
	if len(inst.History) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(inst.History))
	}
	if inst.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
	if inst.ExpiresAt == nil {
		t.Error("expected ExpiresAt to be set")
	}
}

func TestWorkflowInstanceModel_ToInstance_NilJSON(t *testing.T) {
	model := &WorkflowInstanceModel{
		ID:           "inst1",
		DefinitionID: "def1",
		State:        "pending",
		CurrentStep:  0,
		ContextJSON:  nil,
		HistoryJSON:  nil,
	}

	inst, err := model.ToInstance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inst.Context != nil {
		t.Errorf("expected nil context, got %v", inst.Context)
	}
	if inst.History != nil {
		t.Errorf("expected nil history, got %v", inst.History)
	}
}

func TestWorkflowInstanceModel_ToInstance_InvalidJSON(t *testing.T) {
	model := &WorkflowInstanceModel{
		ContextJSON: []byte("invalid"),
	}

	_, err := model.ToInstance()
	if err == nil {
		t.Error("expected error for invalid context JSON")
	}

	model2 := &WorkflowInstanceModel{
		HistoryJSON: []byte("invalid"),
	}

	_, err = model2.ToInstance()
	if err == nil {
		t.Error("expected error for invalid history JSON")
	}
}

func TestFromInstance(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	completedAt := now.Add(time.Hour)

	inst := &WorkflowInstance{
		ID:           "inst1",
		DefinitionID: "def1",
		State:        "completed",
		CurrentStep:  3,
		Context:      map[string]interface{}{"key": "value"},
		ApprovalID:   "app1",
		Initiator:    "user1",
		StartedAt:    now,
		CompletedAt:  &completedAt,
		History: []WorkflowHistoryEntry{
			{Action: "start", Actor: "user1"},
		},
	}

	model, err := FromInstance(inst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if model.ID != "inst1" {
		t.Errorf("expected ID 'inst1', got %q", model.ID)
	}
	if model.State != "completed" {
		t.Errorf("expected state 'completed', got %q", model.State)
	}

	// Verify JSON fields
	var ctx map[string]interface{}
	json.Unmarshal(model.ContextJSON, &ctx)
	if ctx["key"] != "value" {
		t.Errorf("expected context key=value, got %v", ctx)
	}

	var history []WorkflowHistoryEntry
	json.Unmarshal(model.HistoryJSON, &history)
	if len(history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history))
	}
}

func TestFromInstance_NilMaps(t *testing.T) {
	inst := &WorkflowInstance{
		ID:      "inst1",
		Context: nil,
		History: nil,
	}

	model, err := FromInstance(inst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if model.ContextJSON != nil {
		t.Errorf("expected nil ContextJSON, got %v", model.ContextJSON)
	}
	if model.HistoryJSON != nil {
		t.Errorf("expected nil HistoryJSON, got %v", model.HistoryJSON)
	}
}

func TestStepApprovalModel_ToStepApproval(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	model := &StepApprovalModel{
		StepID:      "s1",
		Approver:    "user1",
		DelegatedBy: "user2",
		Decision:    "approved",
		Comment:     "LGTM",
		DecidedAt:   now,
		IPAddress:   "127.0.0.1",
		UserAgent:   "test-agent",
	}

	approval := model.ToStepApproval()

	if approval.StepID != "s1" {
		t.Errorf("expected StepID 's1', got %q", approval.StepID)
	}
	if approval.Approver != "user1" {
		t.Errorf("expected Approver 'user1', got %q", approval.Approver)
	}
	if approval.DelegatedBy != "user2" {
		t.Errorf("expected DelegatedBy 'user2', got %q", approval.DelegatedBy)
	}
	if approval.Decision != "approved" {
		t.Errorf("expected Decision 'approved', got %q", approval.Decision)
	}
	if approval.Comment != "LGTM" {
		t.Errorf("expected Comment 'LGTM', got %q", approval.Comment)
	}
	if approval.IPAddress != "127.0.0.1" {
		t.Errorf("expected IPAddress '127.0.0.1', got %q", approval.IPAddress)
	}
}

func TestDelegationModel_ToDelegation(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	endAt := now.Add(24 * time.Hour)

	permissions := []DelegationPermission{
		PermApprove,
	}
	permissionsJSON, _ := json.Marshal(permissions)

	constraints := []DelegationConstraint{
		{Type: "time_restriction", Value: map[string]interface{}{"start_hour": 9}, Enforced: true},
	}
	constraintsJSON, _ := json.Marshal(constraints)

	model := &DelegationModel{
		ID:          "del1",
		Delegator:   "user1",
		Delegate:    "user2",
		Scope:       "global",
		ScopeValue:  "*",
		Permissions: permissionsJSON,
		State:       "active",
		Reason:      "vacation",
		StartAt:     now,
		EndAt:       &endAt,
		MaxUsages:   10,
		UsageCount:  3,
		Constraints: constraintsJSON,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	delegation, err := model.ToDelegation()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if delegation.ID != "del1" {
		t.Errorf("expected ID 'del1', got %q", delegation.ID)
	}
	if delegation.Delegator != "user1" {
		t.Errorf("expected Delegator 'user1', got %q", delegation.Delegator)
	}
	if delegation.Delegate != "user2" {
		t.Errorf("expected Delegate 'user2', got %q", delegation.Delegate)
	}
	if delegation.Scope != DelegationScope("global") {
		t.Errorf("expected scope 'global', got %q", delegation.Scope)
	}
	if len(delegation.Permissions) != 1 {
		t.Errorf("expected 1 permission, got %d", len(delegation.Permissions))
	}
	if len(delegation.Constraints) != 1 {
		t.Errorf("expected 1 constraint, got %d", len(delegation.Constraints))
	}
	if delegation.MaxUsages != 10 {
		t.Errorf("expected MaxUsages 10, got %d", delegation.MaxUsages)
	}
	if delegation.UsageCount != 3 {
		t.Errorf("expected UsageCount 3, got %d", delegation.UsageCount)
	}
}

func TestDelegationModel_ToDelegation_NilConstraints(t *testing.T) {
	permissionsJSON, _ := json.Marshal([]DelegationPermission{})

	model := &DelegationModel{
		ID:          "del1",
		Permissions: permissionsJSON,
		Constraints: nil,
	}

	delegation, err := model.ToDelegation()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if delegation.Constraints != nil {
		t.Errorf("expected nil constraints, got %v", delegation.Constraints)
	}
}

func TestDelegationModel_ToDelegation_InvalidJSON(t *testing.T) {
	model := &DelegationModel{
		Permissions: []byte("invalid"),
	}

	_, err := model.ToDelegation()
	if err == nil {
		t.Error("expected error for invalid permissions JSON")
	}

	model2 := &DelegationModel{
		Permissions: []byte("[]"),
		Constraints: []byte("invalid"),
	}

	_, err = model2.ToDelegation()
	if err == nil {
		t.Error("expected error for invalid constraints JSON")
	}
}

func TestFromDelegation(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	endAt := now.Add(24 * time.Hour)

	delegation := &Delegation{
		ID:         "del1",
		Delegator:  "user1",
		Delegate:   "user2",
		Scope:      DelegationScope("global"),
		ScopeValue: "*",
		Permissions: []DelegationPermission{
			PermApprove,
		},
		State:     DelegationState("active"),
		Reason:    "vacation",
		StartAt:   now,
		EndAt:     &endAt,
		MaxUsages: 10,
		UsageCount: 3,
		Constraints: []DelegationConstraint{
			{Type: "time_restriction"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	model, err := FromDelegation(delegation)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if model.ID != "del1" {
		t.Errorf("expected ID 'del1', got %q", model.ID)
	}
	if model.Delegator != "user1" {
		t.Errorf("expected Delegator 'user1', got %q", model.Delegator)
	}

	// Verify JSON fields
	var perms []DelegationPermission
	json.Unmarshal(model.Permissions, &perms)
	if len(perms) != 1 {
		t.Errorf("expected 1 permission, got %d", len(perms))
	}

	var constraints []DelegationConstraint
	json.Unmarshal(model.Constraints, &constraints)
	if len(constraints) != 1 {
		t.Errorf("expected 1 constraint, got %d", len(constraints))
	}
}

func TestFromDelegation_NilConstraints(t *testing.T) {
	delegation := &Delegation{
		ID:          "del1",
		Permissions: []DelegationPermission{PermApprove},
		Constraints: nil,
	}

	model, err := FromDelegation(delegation)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if model.Constraints != nil {
		t.Errorf("expected nil Constraints, got %v", model.Constraints)
	}
}

func TestApprovalModel_ToApproval(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	model := &ApprovalModel{
		ID:              "app1",
		State:           "pending",
		FunctionID:      "player.ban",
		GameID:          "game1",
		Env:             "production",
		Actor:           "admin",
		Mode:            "invoke",
		IdempotencyKey:  "key1",
		Route:           "/api/invoke",
		TargetServiceID: "svc1",
		HashKey:         "hash1",
		Payload:         []byte(`{"player_id":"123"}`),
		Reason:          "cheating",
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	approval := model.ToApproval()

	if approval.ID != "app1" {
		t.Errorf("expected ID 'app1', got %q", approval.ID)
	}
	if approval.State != "pending" {
		t.Errorf("expected State 'pending', got %q", approval.State)
	}
	if approval.FunctionID != "player.ban" {
		t.Errorf("expected FunctionID 'player.ban', got %q", approval.FunctionID)
	}
	if approval.GameID != "game1" {
		t.Errorf("expected GameID 'game1', got %q", approval.GameID)
	}
	if approval.Actor != "admin" {
		t.Errorf("expected Actor 'admin', got %q", approval.Actor)
	}
	if approval.Reason != "cheating" {
		t.Errorf("expected Reason 'cheating', got %q", approval.Reason)
	}
}

func TestApprovalModel_ToApproval_Nil(t *testing.T) {
	var model *ApprovalModel
	approval := model.ToApproval()
	if approval != nil {
		t.Error("expected nil approval for nil model")
	}
}

func TestFromApproval(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	approval := &Approval{
		ID:              "app1",
		State:           "approved",
		FunctionID:      "player.ban",
		GameID:          "game1",
		Env:             "production",
		Actor:           "admin",
		Mode:            "invoke",
		IdempotencyKey:  "key1",
		Route:           "/api/invoke",
		TargetServiceID: "svc1",
		HashKey:         "hash1",
		Payload:         []byte(`{}`),
		Reason:          "test",
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	model := FromApproval(approval)

	if model.ID != "app1" {
		t.Errorf("expected ID 'app1', got %q", model.ID)
	}
	if model.State != "approved" {
		t.Errorf("expected State 'approved', got %q", model.State)
	}
}

func TestFromApproval_Nil(t *testing.T) {
	model := FromApproval(nil)
	if model != nil {
		t.Error("expected nil model for nil approval")
	}
}

func TestModelTableNames(t *testing.T) {
	tests := []struct {
		name     string
		model    interface{ TableName() string }
		expected string
	}{
		{"ApprovalModel", ApprovalModel{}, "approvals"},
		{"WorkflowDefinitionModel", WorkflowDefinitionModel{}, "workflow_definitions"},
		{"WorkflowInstanceModel", WorkflowInstanceModel{}, "workflow_instances"},
		{"StepApprovalModel", StepApprovalModel{}, "workflow_step_approvals"},
		{"DelegationModel", DelegationModel{}, "approval_delegations"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.model.TableName(); got != tt.expected {
				t.Errorf("TableName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestTemplateRenderer(t *testing.T) {
	renderer := NewTemplateRenderer()

	template := &NotificationTemplate{
		ID:      "approval_request",
		Type:    "approval",
		Subject: "Approval Request: {{function_name}}",
		Body:    "{{actor}} requests approval for {{function_name}} in {{game_id}}",
	}
	renderer.AddTemplate(template)

	variables := map[string]string{
		"function_name": "player.ban",
		"actor":         "admin",
		"game_id":       "game1",
	}

	event, err := renderer.Render("approval_request", variables)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.Type != "approval" {
		t.Errorf("expected Type 'approval', got %q", event.Type)
	}
	if event.Title != "Approval Request: player.ban" {
		t.Errorf("expected Title 'Approval Request: player.ban', got %q", event.Title)
	}
	expectedBody := "admin requests approval for player.ban in game1"
	if event.Message != expectedBody {
		t.Errorf("expected Message %q, got %q", expectedBody, event.Message)
	}
}

func TestTemplateRenderer_NotFound(t *testing.T) {
	renderer := NewTemplateRenderer()

	_, err := renderer.Render("nonexistent", nil)
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}

func TestTemplateRenderer_NoVariables(t *testing.T) {
	renderer := NewTemplateRenderer()
	renderer.AddTemplate(&NotificationTemplate{
		ID:      "simple",
		Type:    "info",
		Subject: "No variables",
		Body:    "Static content",
	})

	event, err := renderer.Render("simple", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.Title != "No variables" {
		t.Errorf("expected Title 'No variables', got %q", event.Title)
	}
	if event.Message != "Static content" {
		t.Errorf("expected Message 'Static content', got %q", event.Message)
	}
}

func TestTemplateRenderer_MultipleTemplates(t *testing.T) {
	renderer := NewTemplateRenderer()

	renderer.AddTemplate(&NotificationTemplate{
		ID:      "t1",
		Type:    "type1",
		Subject: "Template 1",
		Body:    "Body 1",
	})
	renderer.AddTemplate(&NotificationTemplate{
		ID:      "t2",
		Type:    "type2",
		Subject: "Template 2",
		Body:    "Body 2",
	})

	event1, _ := renderer.Render("t1", nil)
	event2, _ := renderer.Render("t2", nil)

	if event1.Type != "type1" {
		t.Errorf("expected type1, got %q", event1.Type)
	}
	if event2.Type != "type2" {
		t.Errorf("expected type2, got %q", event2.Type)
	}
}

func TestReplaceAll(t *testing.T) {
	tests := []struct {
		s, old, new, expected string
	}{
		{"hello world", "world", "Go", "hello Go"},
		{"aaa", "a", "b", "bbb"},
		{"no match", "xyz", "abc", "no match"},
		{"", "a", "b", ""},
		{"{{name}}", "{{name}}", "John", "John"},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			result := replaceAll(tt.s, tt.old, tt.new)
			if result != tt.expected {
				t.Errorf("replaceAll(%q, %q, %q) = %q, want %q", tt.s, tt.old, tt.new, result, tt.expected)
			}
		})
	}
}

func TestFindSubstring(t *testing.T) {
	tests := []struct {
		s, substr string
		expected  int
	}{
		{"hello", "ll", 2},
		{"hello", "hello", 0},
		{"hello", "xyz", -1},
		{"hello", "helloo", -1},
		{"", "a", -1},
		{"a", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			result := findSubstring(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("findSubstring(%q, %q) = %d, want %d", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}
