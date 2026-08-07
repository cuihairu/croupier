package normalizer

import (
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

func TestNormalizeCarriesCapabilityAndExecution(t *testing.T) {
	result := Normalize(DescriptorInput{
		ID:                "reward.batch_grant",
		Version:           "1.0.0",
		Resource:          "reward",
		Operation:         "batch_grant",
		Capability:        "task",
		InputSchema:       `{"type":"object"}`,
		Execution:         "task",
		ApprovalRequired:  true,
		ApprovalPolicyKey: "two_person",
		Enabled:           true,
	})

	if result.Function.Capability != spec.CapabilityTask {
		t.Fatalf("expected function capability task, got %q", result.Function.Capability)
	}
	if result.Function.Execution != spec.FunctionExecutionTask {
		t.Fatalf("expected function execution task, got %q", result.Function.Execution)
	}
	if !result.Function.Approval.Required || result.Function.Approval.PolicyKey != "two_person" {
		t.Fatalf("expected approval policy to be carried, got %#v", result.Function.Approval)
	}
	if result.Operation == nil {
		t.Fatal("expected operation")
	}
	if result.Operation.Capability != spec.CapabilityTask {
		t.Fatalf("expected operation capability task, got %q", result.Operation.Capability)
	}
	if result.Operation.Execution != spec.FunctionExecutionTask {
		t.Fatalf("expected operation execution task, got %q", result.Operation.Execution)
	}
	if !result.Operation.Approval.Required || result.Operation.Approval.PolicyKey != "two_person" {
		t.Fatalf("expected operation approval policy to be carried, got %#v", result.Operation.Approval)
	}
}

func TestNormalizeDefaultsExecutionFromCapability(t *testing.T) {
	result := Normalize(DescriptorInput{
		ID:          "reward.batch_grant",
		Capability:  "task",
		InputSchema: `{"type":"object"}`,
		Enabled:     true,
	})

	if result.Function.Execution != spec.FunctionExecutionTask {
		t.Fatalf("expected task execution default for task capability, got %q", result.Function.Execution)
	}
}

func TestNormalizeRejectsInvalidCapabilityAndExecution(t *testing.T) {
	result := Normalize(DescriptorInput{
		ID:          "player.ban",
		Capability:  "row_button",
		Execution:   "modal",
		InputSchema: `{"type":"object"}`,
		Enabled:     true,
	})

	assertDiagnostic(t, result.Diagnostics, "capability_invalid")
	assertDiagnostic(t, result.Diagnostics, "execution_invalid")
}

func TestNormalizeRejectsUnstableKeys(t *testing.T) {
	result := Normalize(DescriptorInput{
		ID:          "Reward.BatchGrant",
		Resource:    "reward",
		Operation:   "batchGrant",
		Capability:  "task",
		InputSchema: `{"type":"object"}`,
		Enabled:     true,
	})

	assertDiagnostic(t, result.Diagnostics, "function_id_invalid")
	assertDiagnostic(t, result.Diagnostics, "operation_key_invalid")
}

func assertDiagnostic(t *testing.T, diagnostics []spec.Diagnostic, code string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return
		}
	}
	t.Fatalf("diagnostic %s not found in %#v", code, diagnostics)
}
