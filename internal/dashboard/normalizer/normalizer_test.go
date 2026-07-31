package normalizer

import (
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

func TestNormalizeCarriesCapabilityAndExecution(t *testing.T) {
	result := Normalize(DescriptorInput{
		ID:          "reward.batchGrant",
		Version:     "1.0.0",
		Resource:    "reward",
		Operation:   "batchGrant",
		Capability:  "task",
		InputSchema: `{"type":"object"}`,
		Execution:   "task",
		Enabled:     true,
	})

	if result.Function.Capability != spec.CapabilityTask {
		t.Fatalf("expected function capability task, got %q", result.Function.Capability)
	}
	if result.Function.Execution != spec.FunctionExecutionTask {
		t.Fatalf("expected function execution task, got %q", result.Function.Execution)
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
}

func TestNormalizeDefaultsExecutionFromCapability(t *testing.T) {
	result := Normalize(DescriptorInput{
		ID:          "reward.batchGrant",
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

func assertDiagnostic(t *testing.T, diagnostics []spec.Diagnostic, code string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return
		}
	}
	t.Fatalf("diagnostic %s not found in %#v", code, diagnostics)
}
