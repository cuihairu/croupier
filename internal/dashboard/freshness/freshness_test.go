package freshness

import (
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

func TestEvaluateBindingIncludesSelectorStaleDiagnostics(t *testing.T) {
	diags := EvaluateBinding(
		spec.PageFunctionBinding{
			ID:         "query",
			FunctionID: "player.query",
			Selectors: &spec.BindingSelectors{
				Input: spec.SelectorAST{Assignments: []spec.InputAssignment{{
					Target: "/keyword",
					Source: spec.ValueSource{Kind: spec.SourceForm, Path: "/keyword"},
				}}},
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
				InputSchema:  spec.JSONSchema(`{"type":"object","properties":{"serverId":{"type":"string"}},"required":["serverId"]}`),
				OutputSchema: spec.JSONSchema(`{"type":"object","properties":{"rows":{"type":"array"}}}`),
			},
		},
	)

	assertBindingDiagnostic(t, diags, "binding_input_schema_stale", spec.BindingFreshnessInputSchemaStale)
	assertBindingDiagnostic(t, diags, "binding_output_schema_stale", spec.BindingFreshnessOutputSchemaStale)
	assertBindingDiagnostic(t, diags, "selector_target_stale", spec.BindingFreshnessInputSchemaStale)
	assertBindingDiagnostic(t, diags, "selector_required_stale", spec.BindingFreshnessInputSchemaStale)
	assertBindingDiagnostic(t, diags, "selector_output_source_stale", spec.BindingFreshnessOutputSchemaStale)
}

func assertBindingDiagnostic(t *testing.T, diags []spec.BindingFreshnessDiagnostic, code string, status spec.BindingFreshnessStatus) {
	t.Helper()
	for _, diag := range diags {
		if diag.Diagnostic.Code == code && diag.Status == status {
			return
		}
	}
	t.Fatalf("diagnostic %s/%s not found in %#v", code, status, diags)
}
