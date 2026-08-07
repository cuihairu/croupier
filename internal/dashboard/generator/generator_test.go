package generator

import (
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateForOperationCreatesBasicPage(t *testing.T) {
	page := GenerateForOperation(spec.OperationSpec{
		FunctionID: "cache.refresh",
		Operation:  "refresh",
		Enabled:    true,
	}, GenerateOptions{
		DefaultLocale: "zh-CN",
		Functions: map[string]spec.FunctionSpec{
			"cache.refresh": {
				ID: "cache.refresh",
				InputSchema: spec.JSONSchema(`{
					"type":"object",
					"properties":{
						"scope":{"type":"string"},
						"dryRun":{"type":"boolean"}
					}
				}`),
			},
		},
	})

	assert.Equal(t, spec.PageTypeOperation, page.Type)
	assert.Equal(t, spec.GeneratedPageQualityBasic, page.Quality)
	require.Len(t, page.Bindings, 1)
	require.NotNil(t, page.Operation)
	require.NotNil(t, page.Operation.Form)
	require.NotNil(t, page.Bindings[0].Selectors)
	assertSelectorAssignment(t, page.Bindings[0].Selectors.Input, "/scope", spec.SourceForm, "/scope")
	assertSelectorAssignment(t, page.Bindings[0].Selectors.Input, "/dryRun", spec.SourceForm, "/dryRun")
}

func TestGenerateForOperationUsesExecutionTask(t *testing.T) {
	page := GenerateForOperation(spec.OperationSpec{
		FunctionID: "reward.batchGrant",
		Operation:  "batchGrant",
		Capability: spec.CapabilityTask,
		Execution:  spec.FunctionExecutionTask,
		Enabled:    true,
	}, GenerateOptions{
		DefaultLocale: "zh-CN",
		Functions: map[string]spec.FunctionSpec{
			"reward.batchGrant": {
				ID:          "reward.batchGrant",
				InputSchema: spec.JSONSchema(`{"type":"object","properties":{"segment":{"type":"string"}}}`),
			},
		},
	})

	assert.Equal(t, spec.PageTypeTask, page.Type)
	assert.Equal(t, spec.GeneratedPageQualityNeedsReview, page.Quality)
	assertDiagnostic(t, page.Diagnostics, "task_semantics_missing")
	require.Len(t, page.Bindings, 1)
	assert.Equal(t, spec.BindingUsageTask, page.Bindings[0].Usage)
	assert.Equal(t, spec.PageExecutionModeTask, page.Bindings[0].Execution.Mode)
	require.NotNil(t, page.Task)
	require.NotNil(t, page.Task.TaskView)
	require.NotNil(t, page.Bindings[0].Selectors)
	assertSelectorAssignment(t, page.Bindings[0].Selectors.Input, "/segment", spec.SourceForm, "/segment")
}

func TestGenerateForOperationUsesCapabilityReport(t *testing.T) {
	page := GenerateForOperation(spec.OperationSpec{
		FunctionID:  "analytics.retention",
		ResourceKey: "analytics",
		Operation:   "retention",
		Capability:  spec.CapabilityReport,
		Execution:   spec.FunctionExecutionSync,
		Enabled:     true,
	}, DefaultGenerateOptions())

	assert.Equal(t, spec.PageTypeReport, page.Type)
	assert.Equal(t, spec.GeneratedPageQualityNeedsReview, page.Quality)
	assertDiagnostic(t, page.Diagnostics, "report_dataset_missing")
	require.Len(t, page.Bindings, 1)
	assert.Equal(t, spec.BindingUsageReport, page.Bindings[0].Usage)
	require.NotNil(t, page.Report)
	require.NotNil(t, page.Report.Dataset)
}

func TestInferPageTypeUsesOnlyCapabilitySemantics(t *testing.T) {
	assert.Equal(t, spec.PageTypeOperation, InferPageType([]spec.OperationSpec{{FunctionID: "player.list", Operation: "list"}}))
	assert.Equal(t, spec.PageTypeTask, InferPageType([]spec.OperationSpec{{Capability: spec.CapabilityTask, Execution: spec.FunctionExecutionTask}}))
	assert.Equal(t, spec.PageTypeReport, InferPageType([]spec.OperationSpec{{Capability: spec.CapabilityReport}}))
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

func assertSelectorAssignment(t *testing.T, selector spec.SelectorAST, target string, sourceKind spec.ValueSourceKind, sourcePath string) {
	t.Helper()
	for _, assignment := range selector.Assignments {
		if assignment.Target == target && assignment.Source.Kind == sourceKind && assignment.Source.Path == sourcePath {
			return
		}
	}
	t.Fatalf("selector assignment %s <- %s:%s not found in %#v", target, sourceKind, sourcePath, selector.Assignments)
}
