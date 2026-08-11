package freshness

import (
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- EvaluatePublishedBindings ---

func TestEvaluatePublishedBindings_EmptyInputs(t *testing.T) {
	diags := EvaluatePublishedBindings(nil, nil, nil)
	assert.Empty(t, diags)
}

func TestEvaluatePublishedBindings_ContractMissing(t *testing.T) {
	bindings := []spec.PageFunctionBinding{
		{
			ID:         "binding-1",
			FunctionID: "player.query",
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		},
	}

	diags := EvaluatePublishedBindings(bindings, nil, nil)
	require.Len(t, diags, 1)
	assert.Equal(t, spec.BindingFreshnessContractMissing, diags[0].Status)
	assert.Equal(t, "binding_contract_missing", diags[0].Diagnostic.Code)
}

func TestEvaluatePublishedBindings_FunctionMissing(t *testing.T) {
	bindings := []spec.PageFunctionBinding{
		{
			ID:         "binding-1",
			FunctionID: "player.nonexistent",
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		},
	}
	contracts := []spec.BindingContractSnapshot{
		{
			BindingID:          "binding-1",
			FunctionID:         "player.nonexistent",
			FunctionVersion:    "1.0.0",
			InputSchemaDigest:  "abc",
			OutputSchemaDigest: "def",
			ExecutionMode:      spec.PageExecutionModeSync,
		},
	}

	diags := EvaluatePublishedBindings(bindings, contracts, map[string]spec.FunctionSpec{})
	require.Len(t, diags, 1)
	assert.Equal(t, spec.BindingFreshnessFunctionMissing, diags[0].Status)
	assert.Equal(t, "binding_function_missing", diags[0].Diagnostic.Code)
}

func TestEvaluatePublishedBindings_AllFresh(t *testing.T) {
	inputSchema := spec.JSONSchema(`{"type":"object","properties":{"id":{"type":"string"}}}`)
	outputSchema := spec.JSONSchema(`{"type":"object","properties":{"result":{"type":"string"}}}`)

	bindings := []spec.PageFunctionBinding{
		{
			ID:         "binding-1",
			FunctionID: "player.query",
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		},
	}
	contracts := []spec.BindingContractSnapshot{
		{
			BindingID:          "binding-1",
			FunctionID:         "player.query",
			FunctionVersion:    "1.0.0",
			InputSchemaDigest:  computeDigest(t, inputSchema),
			OutputSchemaDigest: computeDigest(t, outputSchema),
			ExecutionMode:      spec.PageExecutionModeSync,
		},
	}
	functions := map[string]spec.FunctionSpec{
		"player.query": {
			ID:           "player.query",
			Version:      "1.0.0",
			Enabled:      true,
			InputSchema:  inputSchema,
			OutputSchema: outputSchema,
		},
	}

	diags := EvaluatePublishedBindings(bindings, contracts, functions)
	assert.Empty(t, diags)
}

func TestEvaluatePublishedBindings_MultipleBindings(t *testing.T) {
	inputSchema := spec.JSONSchema(`{"type":"object","properties":{"id":{"type":"string"}}}`)
	outputSchema := spec.JSONSchema(`{"type":"object","properties":{"result":{"type":"string"}}}`)

	bindings := []spec.PageFunctionBinding{
		{ID: "b1", FunctionID: "f1", Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}},
		{ID: "b2", FunctionID: "f1", Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}},
	}
	contracts := []spec.BindingContractSnapshot{
		{BindingID: "b1", FunctionID: "f1", FunctionVersion: "1.0.0",
			InputSchemaDigest: computeDigest(t, inputSchema), OutputSchemaDigest: computeDigest(t, outputSchema),
			ExecutionMode: spec.PageExecutionModeSync},
		{BindingID: "b2", FunctionID: "f1", FunctionVersion: "1.0.0",
			InputSchemaDigest: computeDigest(t, inputSchema), OutputSchemaDigest: computeDigest(t, outputSchema),
			ExecutionMode: spec.PageExecutionModeSync},
	}
	functions := map[string]spec.FunctionSpec{
		"f1": {ID: "f1", Version: "1.0.0", InputSchema: inputSchema, OutputSchema: outputSchema},
	}

	diags := EvaluatePublishedBindings(bindings, contracts, functions)
	assert.Empty(t, diags)
}

// --- EvaluateBinding: all stale branches ---

func TestEvaluateBinding_FunctionVersionStale(t *testing.T) {
	inputSchema := spec.JSONSchema(`{"type":"object"}`)
	outputSchema := spec.JSONSchema(`{"type":"object"}`)

	diags := EvaluateBinding(
		spec.PageFunctionBinding{
			ID:         "b1",
			FunctionID: "f1",
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		},
		spec.BindingContractSnapshot{
			BindingID:          "b1",
			FunctionID:         "f1",
			FunctionVersion:    "1.0.0",
			InputSchemaDigest:  computeDigest(t, inputSchema),
			OutputSchemaDigest: computeDigest(t, outputSchema),
			ExecutionMode:      spec.PageExecutionModeSync,
		},
		map[string]spec.FunctionSpec{
			"f1": {ID: "f1", Version: "2.0.0", InputSchema: inputSchema, OutputSchema: outputSchema},
		},
	)

	found := false
	for _, d := range diags {
		if d.Diagnostic.Code == "binding_function_version_stale" {
			found = true
			assert.Equal(t, spec.BindingFreshnessFunctionVersionStale, d.Status)
		}
	}
	assert.True(t, found, "expected binding_function_version_stale diagnostic")
}

func TestEvaluateBinding_InputSchemaStale(t *testing.T) {
	oldSchema := spec.JSONSchema(`{"type":"object","properties":{"old":{"type":"string"}}}`)
	newSchema := spec.JSONSchema(`{"type":"object","properties":{"new":{"type":"string"}}}`)
	outputSchema := spec.JSONSchema(`{"type":"object"}`)

	diags := EvaluateBinding(
		spec.PageFunctionBinding{
			ID:         "b1",
			FunctionID: "f1",
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		},
		spec.BindingContractSnapshot{
			BindingID:          "b1",
			FunctionID:         "f1",
			FunctionVersion:    "1.0.0",
			InputSchemaDigest:  computeDigest(t, oldSchema),
			OutputSchemaDigest: computeDigest(t, outputSchema),
			ExecutionMode:      spec.PageExecutionModeSync,
		},
		map[string]spec.FunctionSpec{
			"f1": {ID: "f1", Version: "1.0.0", InputSchema: newSchema, OutputSchema: outputSchema},
		},
	)

	found := false
	for _, d := range diags {
		if d.Diagnostic.Code == "binding_input_schema_stale" {
			found = true
			assert.Equal(t, spec.BindingFreshnessInputSchemaStale, d.Status)
		}
	}
	assert.True(t, found, "expected binding_input_schema_stale diagnostic")
}

func TestEvaluateBinding_OutputSchemaStale_OnlyOutput(t *testing.T) {
	inputSchema := spec.JSONSchema(`{"type":"object"}`)
	oldOutput := spec.JSONSchema(`{"type":"object","properties":{"old":{"type":"string"}}}`)
	newOutput := spec.JSONSchema(`{"type":"object","properties":{"new":{"type":"string"}}}`)

	diags := EvaluateBinding(
		spec.PageFunctionBinding{
			ID:         "b1",
			FunctionID: "f1",
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		},
		spec.BindingContractSnapshot{
			BindingID:          "b1",
			FunctionID:         "f1",
			FunctionVersion:    "1.0.0",
			InputSchemaDigest:  computeDigest(t, inputSchema),
			OutputSchemaDigest: computeDigest(t, oldOutput),
			ExecutionMode:      spec.PageExecutionModeSync,
		},
		map[string]spec.FunctionSpec{
			"f1": {ID: "f1", Version: "1.0.0", InputSchema: inputSchema, OutputSchema: newOutput},
		},
	)

	found := false
	for _, d := range diags {
		if d.Diagnostic.Code == "binding_output_schema_stale" {
			found = true
			assert.Equal(t, spec.BindingFreshnessOutputSchemaStale, d.Status)
		}
	}
	assert.True(t, found, "expected binding_output_schema_stale diagnostic")
}

func TestEvaluateBinding_GovernanceStale_RiskChanged(t *testing.T) {
	inputSchema := spec.JSONSchema(`{"type":"object"}`)
	outputSchema := spec.JSONSchema(`{"type":"object"}`)

	diags := EvaluateBinding(
		spec.PageFunctionBinding{
			ID:         "b1",
			FunctionID: "f1",
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		},
		spec.BindingContractSnapshot{
			BindingID:          "b1",
			FunctionID:         "f1",
			FunctionVersion:    "1.0.0",
			InputSchemaDigest:  computeDigest(t, inputSchema),
			OutputSchemaDigest: computeDigest(t, outputSchema),
			ExecutionMode:      spec.PageExecutionModeSync,
			Risk:               spec.RiskSafe,
		},
		map[string]spec.FunctionSpec{
			"f1": {ID: "f1", Version: "1.0.0", InputSchema: inputSchema, OutputSchema: outputSchema, Risk: spec.RiskDanger},
		},
	)

	found := false
	for _, d := range diags {
		if d.Diagnostic.Code == "binding_governance_stale" {
			found = true
			assert.Equal(t, spec.BindingFreshnessGovernanceStale, d.Status)
		}
	}
	assert.True(t, found, "expected binding_governance_stale diagnostic")
}

func TestEvaluateBinding_ApprovalStale(t *testing.T) {
	inputSchema := spec.JSONSchema(`{"type":"object"}`)
	outputSchema := spec.JSONSchema(`{"type":"object"}`)

	diags := EvaluateBinding(
		spec.PageFunctionBinding{
			ID:         "b1",
			FunctionID: "f1",
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		},
		spec.BindingContractSnapshot{
			BindingID:          "b1",
			FunctionID:         "f1",
			FunctionVersion:    "1.0.0",
			InputSchemaDigest:  computeDigest(t, inputSchema),
			OutputSchemaDigest: computeDigest(t, outputSchema),
			ExecutionMode:      spec.PageExecutionModeSync,
			Approval: spec.ApprovalPolicy{
				Required:  false,
				PolicyKey: "old-policy",
			},
		},
		map[string]spec.FunctionSpec{
			"f1": {
				ID:           "f1",
				Version:      "1.0.0",
				InputSchema:  inputSchema,
				OutputSchema: outputSchema,
				Approval: spec.ApprovalPolicy{
					Required:  true,
					PolicyKey: "new-policy",
				},
			},
		},
	)

	found := false
	for _, d := range diags {
		if d.Diagnostic.Code == "binding_approval_stale" {
			found = true
			assert.Equal(t, spec.BindingFreshnessGovernanceStale, d.Status)
		}
	}
	assert.True(t, found, "expected binding_approval_stale diagnostic")
}

func TestEvaluateBinding_ExecutionModeStale(t *testing.T) {
	inputSchema := spec.JSONSchema(`{"type":"object"}`)
	outputSchema := spec.JSONSchema(`{"type":"object"}`)

	diags := EvaluateBinding(
		spec.PageFunctionBinding{
			ID:         "b1",
			FunctionID: "f1",
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		},
		spec.BindingContractSnapshot{
			BindingID:          "b1",
			FunctionID:         "f1",
			FunctionVersion:    "1.0.0",
			InputSchemaDigest:  computeDigest(t, inputSchema),
			OutputSchemaDigest: computeDigest(t, outputSchema),
			ExecutionMode:      spec.PageExecutionModeSync,
		},
		map[string]spec.FunctionSpec{
			"f1": {
				ID:           "f1",
				Version:      "1.0.0",
				InputSchema:  inputSchema,
				OutputSchema: outputSchema,
				Execution:    spec.FunctionExecutionTask,
			},
		},
	)

	found := false
	for _, d := range diags {
		if d.Diagnostic.Code == "binding_execution_mode_stale" {
			found = true
			assert.Equal(t, spec.BindingFreshnessExecutionModeStale, d.Status)
		}
	}
	assert.True(t, found, "expected binding_execution_mode_stale diagnostic")
}

func TestEvaluateBinding_ExecutionModeStale_BindingMismatch(t *testing.T) {
	inputSchema := spec.JSONSchema(`{"type":"object"}`)
	outputSchema := spec.JSONSchema(`{"type":"object"}`)

	diags := EvaluateBinding(
		spec.PageFunctionBinding{
			ID:         "b1",
			FunctionID: "f1",
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeTask},
		},
		spec.BindingContractSnapshot{
			BindingID:          "b1",
			FunctionID:         "f1",
			FunctionVersion:    "1.0.0",
			InputSchemaDigest:  computeDigest(t, inputSchema),
			OutputSchemaDigest: computeDigest(t, outputSchema),
			ExecutionMode:      spec.PageExecutionModeSync,
		},
		map[string]spec.FunctionSpec{
			"f1": {
				ID:           "f1",
				Version:      "1.0.0",
				InputSchema:  inputSchema,
				OutputSchema: outputSchema,
				Execution:    spec.FunctionExecutionSync,
			},
		},
	)

	found := false
	for _, d := range diags {
		if d.Diagnostic.Code == "binding_execution_mode_stale" {
			found = true
		}
	}
	assert.True(t, found, "expected binding_execution_mode_stale when binding mode differs from contract")
}

// --- executionModeForFunction ---

func TestExecutionModeForFunction_Task(t *testing.T) {
	mode := executionModeForFunction(spec.FunctionSpec{
		Execution: spec.FunctionExecutionTask,
	})
	assert.Equal(t, spec.PageExecutionModeTask, mode)
}

func TestExecutionModeForFunction_Sync(t *testing.T) {
	mode := executionModeForFunction(spec.FunctionSpec{
		Execution: spec.FunctionExecutionSync,
	})
	assert.Equal(t, spec.PageExecutionModeSync, mode)
}

func TestExecutionModeForFunction_Empty(t *testing.T) {
	mode := executionModeForFunction(spec.FunctionSpec{})
	assert.Equal(t, spec.PageExecutionModeSync, mode)
}

// --- selectorFreshnessDiagnostics ---

func TestSelectorFreshnessDiagnostics_NilSelectors(t *testing.T) {
	diags := selectorFreshnessDiagnostics("b1", "f1", spec.PageFunctionBinding{}, spec.FunctionSpec{})
	assert.Empty(t, diags)
}

func TestSelectorFreshnessDiagnostics_WithInputOutput(t *testing.T) {
	binding := spec.PageFunctionBinding{
		Selectors: &spec.BindingSelectors{
			Input: spec.SelectorAST{
				Assignments: []spec.InputAssignment{{
					Target: "/keyword",
					Source: spec.ValueSource{Kind: spec.SourceForm, Path: "/keyword"},
				}},
			},
			Output: []spec.OutputAssignment{{
				StateKey: "result",
				Source:   "/result",
				Shape:    spec.OutputShapeCollection,
			}},
		},
	}

	fn := spec.FunctionSpec{
		ID:           "f1",
		InputSchema:  spec.JSONSchema(`{"type":"object","properties":{"different":{"type":"string"}}}`),
		OutputSchema: spec.JSONSchema(`{"type":"object","properties":{"different":{"type":"string"}}}`),
	}

	diags := selectorFreshnessDiagnostics("b1", "f1", binding, fn)
	// Should produce stale diagnostics since schemas changed
	assert.NotEmpty(t, diags)
}

// --- selectorFreshnessStatus ---

func TestSelectorFreshnessStatus_Output(t *testing.T) {
	status := selectorFreshnessStatus("output.items")
	assert.Equal(t, spec.BindingFreshnessOutputSchemaStale, status)
}

func TestSelectorFreshnessStatus_Input(t *testing.T) {
	status := selectorFreshnessStatus("input.keyword")
	assert.Equal(t, spec.BindingFreshnessInputSchemaStale, status)
}

func TestSelectorFreshnessStatus_Root(t *testing.T) {
	status := selectorFreshnessStatus("root")
	assert.Equal(t, spec.BindingFreshnessInputSchemaStale, status)
}

// --- digestRaw ---

func TestDigestRaw_Empty(t *testing.T) {
	result := digestRaw(nil)
	assert.Equal(t, "", result)

	result = digestRaw([]byte{})
	assert.Equal(t, "", result)
}

func TestDigestRaw_NonEmpty(t *testing.T) {
	result := digestRaw([]byte("hello"))
	assert.NotEmpty(t, result)
	assert.Len(t, result, 64) // SHA256 hex = 64 chars
}

func TestDigestRaw_Deterministic(t *testing.T) {
	data := []byte("test data")
	h1 := digestRaw(data)
	h2 := digestRaw(data)
	assert.Equal(t, h1, h2)
}

// --- firstNonEmpty ---

func TestFirstNonEmpty_AllEmpty(t *testing.T) {
	result := firstNonEmpty("", "", "")
	assert.Equal(t, "", result)
}

func TestFirstNonEmpty_FirstNonEmpty(t *testing.T) {
	result := firstNonEmpty("first", "second", "third")
	assert.Equal(t, "first", result)
}

func TestFirstNonEmpty_SecondNonEmpty(t *testing.T) {
	result := firstNonEmpty("", "second", "third")
	assert.Equal(t, "second", result)
}

func TestFirstNonEmpty_ThirdNonEmpty(t *testing.T) {
	result := firstNonEmpty("", "", "third")
	assert.Equal(t, "third", result)
}

func TestFirstNonEmpty_WithSpaces(t *testing.T) {
	result := firstNonEmpty("  ", "value", "")
	assert.Equal(t, "value", result)
}

func TestFirstNonEmpty_SingleValue(t *testing.T) {
	result := firstNonEmpty("only")
	assert.Equal(t, "only", result)
}

func TestFirstNonEmpty_EmptyArgs(t *testing.T) {
	result := firstNonEmpty()
	assert.Equal(t, "", result)
}

// --- EvaluateBinding with selectors + input stale ---

func TestEvaluateBinding_SelectorsWithInputStale(t *testing.T) {
	inputSchema := spec.JSONSchema(`{"type":"object","properties":{"serverId":{"type":"string"}},"required":["serverId"]}`)
	outputSchema := spec.JSONSchema(`{"type":"object","properties":{"rows":{"type":"array"}}}`)

	diags := EvaluateBinding(
		spec.PageFunctionBinding{
			ID:         "query",
			FunctionID: "player.query",
			Selectors: &spec.BindingSelectors{
				Input: spec.SelectorAST{
					Assignments: []spec.InputAssignment{{
						Target: "/keyword",
						Source: spec.ValueSource{Kind: spec.SourceForm, Path: "/keyword"},
					}},
				},
				Output: []spec.OutputAssignment{{
					StateKey: "players",
					Source:   "/items",
					Shape:    spec.OutputShapeCollection,
				}},
			},
			Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		},
		spec.BindingContractSnapshot{
			BindingID:          "query",
			FunctionID:         "player.query",
			FunctionVersion:    "1.0.0",
			InputSchemaDigest:  "old-input-digest",
			OutputSchemaDigest: "old-output-digest",
			ExecutionMode:      spec.PageExecutionModeSync,
		},
		map[string]spec.FunctionSpec{
			"player.query": {
				ID:           "player.query",
				Version:      "2.0.0",
				Enabled:      true,
				InputSchema:  inputSchema,
				OutputSchema: outputSchema,
			},
		},
	)

	// Should have version stale, input schema stale, output schema stale, and selector diagnostics
	codes := make(map[string]bool)
	for _, d := range diags {
		codes[d.Diagnostic.Code] = true
	}
	assert.True(t, codes["binding_function_version_stale"])
	assert.True(t, codes["binding_input_schema_stale"])
	assert.True(t, codes["binding_output_schema_stale"])
}

// --- bindingFreshnessDiagnostic ---

func TestBindingFreshnessDiagnostic(t *testing.T) {
	diag := bindingFreshnessDiagnostic("b1", "f1", spec.BindingFreshnessGovernanceStale, "test_code", "test message", "test.field")
	assert.Equal(t, "b1", diag.BindingID)
	assert.Equal(t, "f1", diag.FunctionID)
	assert.Equal(t, spec.BindingFreshnessGovernanceStale, diag.Status)
	assert.Equal(t, "test_code", diag.Diagnostic.Code)
	assert.Equal(t, "test message", diag.Diagnostic.Message)
	assert.Equal(t, "test.field", diag.Diagnostic.Field)
	assert.Equal(t, spec.SeverityError, diag.Diagnostic.Severity)
}

// --- EvaluateBinding: FunctionID from contract ---

func TestEvaluateBinding_FunctionIDFromContract(t *testing.T) {
	inputSchema := spec.JSONSchema(`{"type":"object"}`)
	outputSchema := spec.JSONSchema(`{"type":"object"}`)

	diags := EvaluateBinding(
		spec.PageFunctionBinding{
			ID:         "b1",
			FunctionID: "", // empty function ID
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		},
		spec.BindingContractSnapshot{
			BindingID:          "b1",
			FunctionID:         "f1-from-contract",
			FunctionVersion:    "1.0.0",
			InputSchemaDigest:  computeDigest(t, inputSchema),
			OutputSchemaDigest: computeDigest(t, outputSchema),
			ExecutionMode:      spec.PageExecutionModeSync,
		},
		map[string]spec.FunctionSpec{
			"f1-from-contract": {
				ID:           "f1-from-contract",
				Version:      "1.0.0",
				InputSchema:  inputSchema,
				OutputSchema: outputSchema,
			},
		},
	)
	// Should be fresh since function ID comes from contract
	assert.Empty(t, diags)
}

// --- EvaluateBinding: all fresh (no diagnostics) ---

func TestEvaluateBinding_AllFresh(t *testing.T) {
	inputSchema := spec.JSONSchema(`{"type":"object","properties":{"id":{"type":"string"}}}`)
	outputSchema := spec.JSONSchema(`{"type":"object","properties":{"result":{"type":"string"}}}`)

	diags := EvaluateBinding(
		spec.PageFunctionBinding{
			ID:         "b1",
			FunctionID: "f1",
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		},
		spec.BindingContractSnapshot{
			BindingID:          "b1",
			FunctionID:         "f1",
			FunctionVersion:    "1.0.0",
			InputSchemaDigest:  computeDigest(t, inputSchema),
			OutputSchemaDigest: computeDigest(t, outputSchema),
			ExecutionMode:      spec.PageExecutionModeSync,
		},
		map[string]spec.FunctionSpec{
			"f1": {
				ID:           "f1",
				Version:      "1.0.0",
				InputSchema:  inputSchema,
				OutputSchema: outputSchema,
			},
		},
	)
	assert.Empty(t, diags)
}

// helper to compute a digest for test data
func computeDigest(t *testing.T, raw []byte) string {
	t.Helper()
	return digestRaw(raw)
}
