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
				OutputSchema: spec.JSONSchema(`{
					"type":"object",
					"properties":{"refreshed":{"type":"boolean"},"count":{"type":"integer"}}
				}`),
			},
		},
	})

	assert.Equal(t, spec.PageTypeOperation, page.Type)
	assert.Equal(t, spec.GeneratedPageQualityBasic, page.Quality)
	require.Len(t, page.Bindings, 1)
	require.NotNil(t, page.Operation)
	require.NotNil(t, page.Operation.Form)
	require.NotNil(t, page.Operation.ResultView)
	assert.Len(t, page.Operation.ResultView.Fields, 2)
	assert.NotEmpty(t, page.Operation.ResultView.SuccessMessage)
	assert.NotEmpty(t, page.Operation.ResultView.ErrorMessage)
	require.NotNil(t, page.Bindings[0].Selectors)
	assertSelectorAssignment(t, page.Bindings[0].Selectors.Input, "/scope", spec.SourceForm, "/scope")
	assertSelectorAssignment(t, page.Bindings[0].Selectors.Input, "/dryRun", spec.SourceForm, "/dryRun")
}

func TestGenerateOperationPageRequiresConfirmationForRiskOrApproval(t *testing.T) {
	tests := []struct {
		name string
		op   spec.OperationSpec
	}{
		{
			name: "high risk",
			op: spec.OperationSpec{
				FunctionID: "player.ban", Operation: "ban", Capability: spec.CapabilityAction,
				Execution: spec.FunctionExecutionSync, Risk: spec.RiskHigh, Permission: "player:ban", Enabled: true,
			},
		},
		{
			name: "approval required",
			op: spec.OperationSpec{
				FunctionID: "mail.send", Operation: "send", Capability: spec.CapabilityAction,
				Execution: spec.FunctionExecutionSync, Approval: spec.ApprovalPolicy{Required: true, PolicyKey: "two_person"}, Permission: "mail:send", Enabled: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := GenerateForOperation(tt.op, DefaultGenerateOptions())
			require.NotNil(t, page.Operation)
			require.NotNil(t, page.Operation.Confirm)
			require.Len(t, page.Bindings, 1)
			assert.Equal(t, page.Bindings[0].ID, page.Operation.Confirm.BindingID)
			assert.Equal(t, tt.op.Permission, page.Operation.Confirm.Permission)
			assert.Equal(t, string(tt.op.Risk), page.Operation.Confirm.Risk)
		})
	}
}

func TestGenerateForOperationUsesExecutionTask(t *testing.T) {
	page := GenerateForOperation(spec.OperationSpec{
		FunctionID: "reward.batch_grant",
		Operation:  "batch_grant",
		Capability: spec.CapabilityTask,
		Execution:  spec.FunctionExecutionTask,
		Enabled:    true,
	}, GenerateOptions{
		DefaultLocale: "zh-CN",
		Functions: map[string]spec.FunctionSpec{
			"reward.batch_grant": {
				ID:          "reward.batch_grant",
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

func TestGenerateTaskPageRequiresCompleteLifecycleSemantics(t *testing.T) {
	op := spec.OperationSpec{
		FunctionID: "reward.batch_grant", Operation: "batch_grant", Capability: spec.CapabilityTask,
		Execution: spec.FunctionExecutionTask, Enabled: true,
	}
	functions := map[string]spec.FunctionSpec{
		"reward.batch_grant": {ID: "reward.batch_grant", OutputSchema: spec.JSONSchema(`{"type":"object","properties":{"taskId":{"type":"string"}}}`)},
		"reward.status":      {ID: "reward.status"},
		"reward.events":      {ID: "reward.events"},
		"reward.result":      {ID: "reward.result", OutputSchema: spec.JSONSchema(`{"type":"object","properties":{"result":{"type":"object"}}}`)},
		"reward.cancel":      {ID: "reward.cancel"},
	}
	complete := spec.TaskSemantic{
		Start:  spec.FunctionRef{FunctionID: "reward.batch_grant"},
		TaskID: spec.TaskIDSemantic{ResultPath: "/taskId", ValueType: spec.JsonScalarString},
		Status: spec.TaskStatusSemantic{Function: spec.FunctionRef{FunctionID: "reward.status"}, TaskIDInput: "/taskId", StatePath: "/state"},
		Events: &spec.TaskEventsSemantic{Function: spec.FunctionRef{FunctionID: "reward.events"}, TaskIDInput: "/taskId", EventsPath: "/events"},
		Result: &spec.TaskResultSemantic{Function: spec.FunctionRef{FunctionID: "reward.result"}, TaskIDInput: "/taskId", ResultPath: "/result"},
		Cancel: &spec.TaskCommandSemantic{Function: spec.FunctionRef{FunctionID: "reward.cancel"}, TaskIDInput: "/taskId"},
	}

	page := GenerateForOperation(op, GenerateOptions{DefaultLocale: "zh-CN", Functions: functions, TaskSemantics: map[string]spec.TaskSemantic{op.FunctionID: complete}})
	assert.Equal(t, spec.GeneratedPageQualityBasic, page.Quality)
	require.NotNil(t, page.Task)
	require.NotNil(t, page.Task.TaskView)
	assert.Equal(t, "status", page.Task.TaskView.StatusBindingID)
	assert.Equal(t, "events", page.Task.TaskView.EventsBindingID)
	assert.Equal(t, "result", page.Task.TaskView.ResultBindingID)
	assert.Equal(t, "cancel", page.Task.TaskView.CancelBindingID)
	assert.Empty(t, page.Task.TaskView.RetryBindingID)
	assert.Len(t, page.Bindings, 5)

	incomplete := complete
	incomplete.Cancel = nil
	page = GenerateForOperation(op, GenerateOptions{DefaultLocale: "zh-CN", Functions: functions, TaskSemantics: map[string]spec.TaskSemantic{op.FunctionID: incomplete}})
	assert.Equal(t, spec.GeneratedPageQualityNeedsReview, page.Quality)
	assertDiagnostic(t, page.Diagnostics, "task_semantics_incomplete")
	assert.Empty(t, page.Task.TaskView.CancelBindingID)
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

func TestGenerateReportPageRequiresDatasetSemantics(t *testing.T) {
	op := spec.OperationSpec{
		FunctionID: "analytics.retention", Operation: "retention", Capability: spec.CapabilityReport,
		Execution: spec.FunctionExecutionSync, Enabled: true,
	}
	functions := map[string]spec.FunctionSpec{
		op.FunctionID: {
			ID:           op.FunctionID,
			OutputSchema: spec.JSONSchema(`{"type":"object","properties":{"rows":{"type":"array","items":{"type":"object","properties":{"day":{"type":"string","format":"date"},"retained":{"type":"integer"}}}}}}`),
		},
	}
	semantic := spec.ReportSemantic{
		Query:       spec.FunctionRef{FunctionID: op.FunctionID},
		DatasetPath: "/rows",
		Dimensions:  []string{"/day"},
		Metrics:     []string{"/retained"},
	}

	page := GenerateForOperation(op, GenerateOptions{DefaultLocale: "zh-CN", Functions: functions, ReportSemantics: map[string]spec.ReportSemantic{op.FunctionID: semantic}})
	assert.Equal(t, spec.GeneratedPageQualityReady, page.Quality)
	require.NotNil(t, page.Report)
	require.NotNil(t, page.Report.Dataset)
	assert.Len(t, page.Report.Dataset.Dimensions, 1)
	assert.Len(t, page.Report.Dataset.Metrics, 1)
	require.Len(t, page.Report.Charts, 1)
	require.NotNil(t, page.Bindings[0].Selectors)
	assert.Equal(t, "/rows", page.Bindings[0].Selectors.Output[0].Source)
	assert.Equal(t, spec.OutputShapeDataset, page.Bindings[0].Selectors.Output[0].Shape)

	semantic.DatasetPath = ""
	page = GenerateForOperation(op, GenerateOptions{DefaultLocale: "zh-CN", Functions: functions, ReportSemantics: map[string]spec.ReportSemantic{op.FunctionID: semantic}})
	assert.Equal(t, spec.GeneratedPageQualityNeedsReview, page.Quality)
	assertDiagnostic(t, page.Diagnostics, "report_dataset_missing")
	assert.Empty(t, page.Report.Charts)
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
