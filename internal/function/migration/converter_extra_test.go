package migration

import (
	"testing"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
)

func TestLocalFunctionDescriptorToMetadata_WithOperationID(t *testing.T) {
	desc := LocalFunctionDescriptor{
		ID:          "player.ban",
		OperationID: "playerBan",
	}

	metadata := LocalFunctionDescriptorToMetadata(desc)
	if metadata == nil {
		t.Fatal("expected non-nil metadata")
	}

	if metadata.Extensions["x-operation-id"] != "playerBan" {
		t.Errorf("x-operation-id = %v, want playerBan", metadata.Extensions["x-operation-id"])
	}
}

func TestLocalFunctionDescriptorToMetadata_WithDeprecated(t *testing.T) {
	desc := LocalFunctionDescriptor{
		ID:         "player.ban",
		Deprecated: true,
	}

	metadata := LocalFunctionDescriptorToMetadata(desc)
	if metadata == nil {
		t.Fatal("expected non-nil metadata")
	}

	if metadata.Extensions["x-deprecated"] != "true" {
		t.Errorf("x-deprecated = %v, want true", metadata.Extensions["x-deprecated"])
	}
}

func TestLocalFunctionDescriptorToMetadata_WithEntity(t *testing.T) {
	desc := LocalFunctionDescriptor{
		ID:     "player.ban",
		Entity: "Player",
	}

	metadata := LocalFunctionDescriptorToMetadata(desc)
	if metadata == nil {
		t.Fatal("expected non-nil metadata")
	}

	if metadata.Extensions["x-entity"] != "Player" {
		t.Errorf("x-entity = %v, want Player", metadata.Extensions["x-entity"])
	}
}

func TestLocalFunctionDescriptorToMetadata_WithOperation(t *testing.T) {
	desc := LocalFunctionDescriptor{
		ID:        "player.ban",
		Operation: "delete",
	}

	metadata := LocalFunctionDescriptorToMetadata(desc)
	if metadata == nil {
		t.Fatal("expected non-nil metadata")
	}

	if metadata.Extensions["x-operation"] != "delete" {
		t.Errorf("x-operation = %v, want delete", metadata.Extensions["x-operation"])
	}
}

func TestLocalFunctionDescriptorToMetadata_DangerRisk(t *testing.T) {
	desc := LocalFunctionDescriptor{
		ID:   "player.ban",
		Risk: "danger",
	}

	metadata := LocalFunctionDescriptorToMetadata(desc)
	if metadata == nil {
		t.Fatal("expected non-nil metadata")
	}

	if !metadata.Security.MaskSensitiveData {
		t.Error("Expected MaskSensitiveData to be true for danger risk")
	}
}

func TestLocalFunctionDescriptorToMetadata_HighRisk(t *testing.T) {
	desc := LocalFunctionDescriptor{
		ID:   "player.ban",
		Risk: "high",
	}

	metadata := LocalFunctionDescriptorToMetadata(desc)
	if metadata == nil {
		t.Fatal("expected non-nil metadata")
	}

	if !metadata.Security.MaskSensitiveData {
		t.Error("Expected MaskSensitiveData to be true for high risk")
	}
}

func TestLocalFunctionDescriptorToMetadata_LowRisk(t *testing.T) {
	desc := LocalFunctionDescriptor{
		ID:   "player.get",
		Risk: "low",
	}

	metadata := LocalFunctionDescriptorToMetadata(desc)
	if metadata == nil {
		t.Fatal("expected non-nil metadata")
	}

	if metadata.Security.MaskSensitiveData {
		t.Error("Expected MaskSensitiveData to be false for low risk")
	}
}

func TestFunctionDescriptorToMetadata_EmptyID(t *testing.T) {
	desc := FunctionDescriptor{
		ID: "",
	}

	metadata := FunctionDescriptorToMetadata(desc)
	if metadata != nil {
		t.Error("expected nil metadata for empty ID")
	}
}

func TestFunctionDescriptorToMetadata_WithEntity(t *testing.T) {
	desc := FunctionDescriptor{
		ID:     "player.get",
		Entity: "Player",
	}

	metadata := FunctionDescriptorToMetadata(desc)
	if metadata == nil {
		t.Fatal("expected non-nil metadata")
	}

	found := false
	for _, tag := range metadata.Tags {
		if tag == "Player" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Entity 'Player' not found in tags")
	}

	if metadata.Extensions["x-entity"] != "Player" {
		t.Errorf("x-entity = %v, want Player", metadata.Extensions["x-entity"])
	}
}

func TestFunctionDescriptorToMetadata_WithOperation(t *testing.T) {
	desc := FunctionDescriptor{
		ID:        "player.get",
		Operation: "read",
	}

	metadata := FunctionDescriptorToMetadata(desc)
	if metadata == nil {
		t.Fatal("expected non-nil metadata")
	}

	found := false
	for _, tag := range metadata.Tags {
		if tag == "read" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Operation 'read' not found in tags")
	}

	if metadata.Extensions["x-operation"] != "read" {
		t.Errorf("x-operation = %v, want read", metadata.Extensions["x-operation"])
	}
}

func TestFunctionDescriptorToMetadata_Disabled(t *testing.T) {
	desc := FunctionDescriptor{
		ID:      "player.get",
		Enabled: false,
	}

	metadata := FunctionDescriptorToMetadata(desc)
	if metadata == nil {
		t.Fatal("expected non-nil metadata")
	}

	if metadata.Extensions["x-enabled"] != "false" {
		t.Errorf("x-enabled = %v, want false", metadata.Extensions["x-enabled"])
	}
}

func TestFunctionDescriptorToMetadata_Enabled(t *testing.T) {
	desc := FunctionDescriptor{
		ID:      "player.get",
		Enabled: true,
	}

	metadata := FunctionDescriptorToMetadata(desc)
	if metadata == nil {
		t.Fatal("expected non-nil metadata")
	}

	if _, exists := metadata.Extensions["x-enabled"]; exists {
		t.Error("x-enabled should not be set for enabled functions")
	}
}

func TestFunctionDescriptorToMetadata_DangerRisk(t *testing.T) {
	desc := FunctionDescriptor{
		ID:   "player.ban",
		Risk: "danger",
	}

	metadata := FunctionDescriptorToMetadata(desc)
	if metadata == nil {
		t.Fatal("expected non-nil metadata")
	}

	if !metadata.Security.MaskSensitiveData {
		t.Error("Expected MaskSensitiveData to be true for danger risk")
	}
}

func TestMetadataToLocalFunctionDescriptor_NilMetadata(t *testing.T) {
	desc := MetadataToLocalFunctionDescriptor(nil)
	if desc.ID != "" {
		t.Errorf("Expected empty ID for nil metadata, got: %s", desc.ID)
	}
}

func TestMetadataToLocalFunctionDescriptor_WithExtensions(t *testing.T) {
	metadata := &functionv1.FunctionMetadata{
		Id:       "player.ban",
		Version:  "1.0.0",
		Category: "player",
		Name:     "Ban Player",
		Extensions: map[string]string{
			"x-operation-id": "playerBan",
			"x-deprecated":   "true",
			"x-entity":       "Player",
			"x-operation":    "delete",
		},
		Security: &functionv1.FunctionSecurity{
			RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_HIGH,
		},
	}

	desc := MetadataToLocalFunctionDescriptor(metadata)

	if desc.OperationID != "playerBan" {
		t.Errorf("OperationID = %v, want playerBan", desc.OperationID)
	}
	if !desc.Deprecated {
		t.Error("Expected Deprecated to be true")
	}
	if desc.Entity != "Player" {
		t.Errorf("Entity = %v, want Player", desc.Entity)
	}
	if desc.Operation != "delete" {
		t.Errorf("Operation = %v, want delete", desc.Operation)
	}
	if desc.Risk != "high" {
		t.Errorf("Risk = %v, want high", desc.Risk)
	}
}

func TestMetadataToLocalFunctionDescriptor_NonTrueDeprecated(t *testing.T) {
	metadata := &functionv1.FunctionMetadata{
		Id: "player.ban",
		Extensions: map[string]string{
			"x-deprecated": "false",
		},
	}

	desc := MetadataToLocalFunctionDescriptor(metadata)

	if desc.Deprecated {
		t.Error("Expected Deprecated to be false")
	}
}

func TestMetadataToFunctionDescriptor_NilMetadata(t *testing.T) {
	desc := MetadataToFunctionDescriptor(nil)
	if desc.ID != "" {
		t.Errorf("Expected empty ID for nil metadata, got: %s", desc.ID)
	}
}

func TestMetadataToFunctionDescriptor_WithExtensions(t *testing.T) {
	metadata := &functionv1.FunctionMetadata{
		Id:       "player.get",
		Version:  "1.0.0",
		Category: "player",
		Extensions: map[string]string{
			"x-entity":    "Player",
			"x-operation": "read",
		},
		Security: &functionv1.FunctionSecurity{
			RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW,
		},
	}

	desc := MetadataToFunctionDescriptor(metadata)

	if desc.Entity != "Player" {
		t.Errorf("Entity = %v, want Player", desc.Entity)
	}
	if desc.Operation != "read" {
		t.Errorf("Operation = %v, want read", desc.Operation)
	}
	if desc.Risk != "low" {
		t.Errorf("Risk = %v, want low", desc.Risk)
	}
}

func TestMetadataToFunctionDescriptor_Disabled(t *testing.T) {
	metadata := &functionv1.FunctionMetadata{
		Id: "player.get",
		Extensions: map[string]string{
			"x-enabled": "false",
		},
	}

	desc := MetadataToFunctionDescriptor(metadata)

	if desc.Enabled {
		t.Error("Expected Enabled to be false")
	}
}

func TestInferModeFromOperation_Read(t *testing.T) {
	got := inferModeFromOperation("read")
	if got != functionv1.FunctionBehavior_MODE_QUERY {
		t.Errorf("inferModeFromOperation('read') = %v, want QUERY", got)
	}
}

func TestInferModeFromOperation_Query(t *testing.T) {
	got := inferModeFromOperation("query")
	if got != functionv1.FunctionBehavior_MODE_QUERY {
		t.Errorf("inferModeFromOperation('query') = %v, want QUERY", got)
	}
}

func TestInferModeFromOperation_List(t *testing.T) {
	got := inferModeFromOperation("list")
	if got != functionv1.FunctionBehavior_MODE_QUERY {
		t.Errorf("inferModeFromOperation('list') = %v, want QUERY", got)
	}
}

func TestInferModeFromOperation_Create(t *testing.T) {
	got := inferModeFromOperation("create")
	if got != functionv1.FunctionBehavior_MODE_COMMAND {
		t.Errorf("inferModeFromOperation('create') = %v, want COMMAND", got)
	}
}

func TestInferModeFromOperation_Empty(t *testing.T) {
	got := inferModeFromOperation("")
	if got != functionv1.FunctionBehavior_MODE_COMMAND {
		t.Errorf("inferModeFromOperation('') = %v, want COMMAND", got)
	}
}

func TestInferIdempotencyFromOperation_Update(t *testing.T) {
	got := inferIdempotencyFromOperation("update")
	if !got {
		t.Error("Expected 'update' to be idempotent")
	}
}

func TestInferIdempotencyFromOperation_Delete(t *testing.T) {
	got := inferIdempotencyFromOperation("delete")
	if !got {
		t.Error("Expected 'delete' to be idempotent")
	}
}

func TestInferIdempotencyFromOperation_Read(t *testing.T) {
	got := inferIdempotencyFromOperation("read")
	if !got {
		t.Error("Expected 'read' to be idempotent")
	}
}

func TestInferIdempotencyFromOperation_Create(t *testing.T) {
	got := inferIdempotencyFromOperation("create")
	if got {
		t.Error("Expected 'create' to NOT be idempotent")
	}
}

func TestInferIdempotencyFromOperation_Empty(t *testing.T) {
	got := inferIdempotencyFromOperation("")
	if got {
		t.Error("Expected empty operation to NOT be idempotent")
	}
}

func TestNormalizeRiskLevel_Default(t *testing.T) {
	got := normalizeRiskLevel(functionv1.FunctionSecurity_RiskLevel(999))
	if got != "medium" {
		t.Errorf("normalizeRiskLevel(unknown) = %v, want medium", got)
	}
}

func TestInferRiskLevel_Info(t *testing.T) {
	got := inferRiskLevel("info")
	if got != functionv1.FunctionSecurity_RISK_LEVEL_LOW {
		t.Errorf("inferRiskLevel('info') = %v, want LOW", got)
	}
}

func TestInferRiskLevel_Moderate(t *testing.T) {
	got := inferRiskLevel("moderate")
	if got != functionv1.FunctionSecurity_RISK_LEVEL_MEDIUM {
		t.Errorf("inferRiskLevel('moderate') = %v, want MEDIUM", got)
	}
}

func TestInferIdempotency_Mute(t *testing.T) {
	got := inferIdempotency("player.mute")
	if !got {
		t.Error("Expected 'mute' to be idempotent")
	}
}

func TestInferIdempotency_Enable(t *testing.T) {
	got := inferIdempotency("player.enable")
	if !got {
		t.Error("Expected 'enable' to be idempotent")
	}
}

func TestInferIdempotency_Disable(t *testing.T) {
	got := inferIdempotency("player.disable")
	if !got {
		t.Error("Expected 'disable' to be idempotent")
	}
}

func TestInferIdempotency_Get(t *testing.T) {
	got := inferIdempotency("player.get")
	if !got {
		t.Error("Expected 'get' to be idempotent")
	}
}

func TestInferModeFromID_Fetch(t *testing.T) {
	got := inferModeFromID("player.fetch")
	if got != functionv1.FunctionBehavior_MODE_QUERY {
		t.Errorf("inferModeFromID('player.fetch') = %v, want QUERY", got)
	}
}

func TestInferModeFromID_Info(t *testing.T) {
	got := inferModeFromID("player.info")
	if got != functionv1.FunctionBehavior_MODE_QUERY {
		t.Errorf("inferModeFromID('player.info') = %v, want QUERY", got)
	}
}

func TestInferModeFromID_Check(t *testing.T) {
	got := inferModeFromID("player.check")
	if got != functionv1.FunctionBehavior_MODE_QUERY {
		t.Errorf("inferModeFromID('player.check') = %v, want QUERY", got)
	}
}
