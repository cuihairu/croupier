package generator

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
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

func TestHasDiagnosticCode(t *testing.T) {
	diags := []spec.Diagnostic{
		{Code: "error_1", Severity: spec.SeverityError},
		{Code: "warning_1", Severity: spec.SeverityWarning},
	}

	assert.True(t, hasDiagnosticCode(diags, "error_1"))
	assert.True(t, hasDiagnosticCode(diags, "warning_1"))
	assert.False(t, hasDiagnosticCode(diags, "not_found"))
	assert.False(t, hasDiagnosticCode(nil, "error_1"))
	assert.False(t, hasDiagnosticCode([]spec.Diagnostic{}, "error_1"))
}

func TestBindingIDForOperationWithSuffix(t *testing.T) {
	tests := []struct {
		name       string
		op         spec.OperationSpec
		suffix     string
		expectedID string
	}{
		{
			name:       "with suffix",
			op:         spec.OperationSpec{FunctionID: "player.ban", Operation: "ban"},
			suffix:     "confirm",
			expectedID: "player.ban.confirm",
		},
		{
			name:       "empty suffix defaults to main",
			op:         spec.OperationSpec{FunctionID: "player.ban", Operation: "ban"},
			suffix:     "",
			expectedID: "player.ban.main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := bindingIDForOperationWithSuffix(tt.op, tt.suffix)
			assert.Equal(t, tt.expectedID, result)
		})
	}
}

func TestSanitizeBindingID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "player.ban", "player.ban"},
		{"with spaces", "player ban", "player.ban"},
		{"with special chars", "player@ban#run", "player.ban.run"},
		{"multiple spaces", "player   ban", "player.ban"},
		{"leading/trailing spaces", "  player.ban  ", "player.ban"},
		{"empty returns binding", "", "binding"},
		{"only spaces", "   ", "binding"},
		{"with hyphens", "player-ban", "player.ban"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeBindingID(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLocalizedTitle(t *testing.T) {
	tests := []struct {
		name     string
		op       spec.OperationSpec
		pageKey  string
		locale   string
		opts     GenerateOptions
		expected spec.LocalizedText
	}{
		{
			name:     "with locale",
			op:       spec.OperationSpec{FunctionID: "player.ban"},
			pageKey:  "ops",
			locale:   "zh-CN",
			expected: spec.LocalizedText{"zh-CN": "Player Ban"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := localizedTitle(tt.op, tt.pageKey, tt.locale, tt.opts)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExecutionModeForOperation(t *testing.T) {
	tests := []struct {
		name     string
		op       spec.OperationSpec
		expected spec.PageExecutionMode
	}{
		{
			name:     "sync execution",
			op:       spec.OperationSpec{Execution: spec.FunctionExecutionSync},
			expected: spec.PageExecutionModeSync,
		},
		{
			name:     "task execution",
			op:       spec.OperationSpec{Execution: spec.FunctionExecutionTask},
			expected: spec.PageExecutionModeTask,
		},
		{
			name:     "empty execution",
			op:       spec.OperationSpec{},
			expected: spec.PageExecutionModeSync,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executionModeForOperation(tt.op)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTaskOutputShapeForPointer(t *testing.T) {
	tests := []struct {
		name     string
		schema   spec.JSONSchema
		pointer  string
		expected spec.OutputResultShape
	}{
		{
			name:     "empty pointer returns shape for schema",
			schema:   spec.JSONSchema(`{"type":"object"}`),
			pointer:  "",
			expected: spec.OutputShapeObject,
		},
		{
			name:     "invalid pointer returns scalar",
			schema:   spec.JSONSchema(`{"type":"object"}`),
			pointer:  "/nonexistent",
			expected: spec.OutputShapeScalar,
		},
		{
			name:     "array pointer",
			schema:   spec.JSONSchema(`{"type":"object","properties":{"items":{"type":"array"}}}`),
			pointer:  "/items",
			expected: spec.OutputShapeCollection,
		},
		{
			name:     "object pointer",
			schema:   spec.JSONSchema(`{"type":"object","properties":{"data":{"type":"object"}}}`),
			pointer:  "/data",
			expected: spec.OutputShapeObject,
		},
		{
			name:     "string pointer",
			schema:   spec.JSONSchema(`{"type":"object","properties":{"name":{"type":"string"}}}`),
			pointer:  "/name",
			expected: spec.OutputShapeScalar,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := taskOutputShapeForPointer(tt.schema, tt.pointer)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildDatasetSpec(t *testing.T) {
	tests := []struct {
		name     string
		schema   spec.JSONSchema
		locale   string
		expected bool // has result
	}{
		{
			name:     "empty schema returns nil",
			schema:   nil,
			locale:   "zh-CN",
			expected: false,
		},
		{
			name:     "array with number and string fields",
			schema:   spec.JSONSchema(`{"type":"array","items":{"type":"object","properties":{"count":{"type":"integer"},"name":{"type":"string"}}}}`),
			locale:   "zh-CN",
			expected: true,
		},
		{
			name:     "array with only strings returns nil",
			schema:   spec.JSONSchema(`{"type":"array","items":{"type":"object","properties":{"name":{"type":"string"}}}}`),
			locale:   "zh-CN",
			expected: false,
		},
		{
			name:     "array with only numbers returns nil",
			schema:   spec.JSONSchema(`{"type":"array","items":{"type":"object","properties":{"count":{"type":"integer"}}}}`),
			locale:   "zh-CN",
			expected: false,
		},
		{
			name:     "boolean and number fields",
			schema:   spec.JSONSchema(`{"type":"array","items":{"type":"object","properties":{"active":{"type":"boolean"},"score":{"type":"number"}}}}`),
			locale:   "zh-CN",
			expected: true,
		},
		{
			name:     "date and number fields",
			schema:   spec.JSONSchema(`{"type":"array","items":{"type":"object","properties":{"created":{"type":"string","format":"date"},"score":{"type":"number"}}}}`),
			locale:   "zh-CN",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildDatasetSpec(tt.schema, tt.locale)
			if tt.expected {
				assert.NotNil(t, result)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

func TestDatasetItemSchema(t *testing.T) {
	tests := []struct {
		name     string
		schema   spec.JSONSchema
		expected bool // has result
	}{
		{
			name:     "empty schema returns nil",
			schema:   nil,
			expected: false,
		},
		{
			name:     "array schema returns items",
			schema:   spec.JSONSchema(`{"type":"array","items":{"type":"object","properties":{"id":{"type":"integer"}}}}`),
			expected: true,
		},
		{
			name:     "object with items property",
			schema:   spec.JSONSchema(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object"}}}}`),
			expected: true,
		},
		{
			name:     "object with dataset property",
			schema:   spec.JSONSchema(`{"type":"object","properties":{"dataset":{"type":"array","items":{"type":"object"}}}}`),
			expected: true,
		},
		{
			name:     "object without array properties",
			schema:   spec.JSONSchema(`{"type":"object","properties":{"name":{"type":"string"}}}`),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := datasetItemSchema(tt.schema)
			if tt.expected {
				assert.NotNil(t, result)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

func TestExtractString(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]json.RawMessage
		key      string
		expected string
	}{
		{
			name:     "nil map",
			obj:      nil,
			key:      "name",
			expected: "",
		},
		{
			name:     "empty map",
			obj:      map[string]json.RawMessage{},
			key:      "name",
			expected: "",
		},
		{
			name:     "key exists",
			obj:      map[string]json.RawMessage{"name": json.RawMessage(`"test"`)},
			key:      "name",
			expected: "test",
		},
		{
			name:     "key with whitespace",
			obj:      map[string]json.RawMessage{"name": json.RawMessage(`"  test  "`)},
			key:      "name",
			expected: "test",
		},
		{
			name:     "key not found",
			obj:      map[string]json.RawMessage{"other": json.RawMessage(`"test"`)},
			key:      "name",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractString(tt.obj, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInferFilterType(t *testing.T) {
	tests := []struct {
		name     string
		schema   json.RawMessage
		expected string
	}{
		{
			name:     "integer type",
			schema:   json.RawMessage(`{"type":"integer"}`),
			expected: "number",
		},
		{
			name:     "number type",
			schema:   json.RawMessage(`{"type":"number"}`),
			expected: "number",
		},
		{
			name:     "string type",
			schema:   json.RawMessage(`{"type":"string"}`),
			expected: "text",
		},
		{
			name:     "string with date format",
			schema:   json.RawMessage(`{"type":"string","format":"date"}`),
			expected: "date",
		},
		{
			name:     "string with datetime format",
			schema:   json.RawMessage(`{"type":"string","format":"date-time"}`),
			expected: "date",
		},
		{
			name:     "enum field",
			schema:   json.RawMessage(`{"type":"string","enum":["a","b","c"]}`),
			expected: "select",
		},
		{
			name:     "unknown type defaults to text",
			schema:   json.RawMessage(`{"type":"boolean"}`),
			expected: "text",
		},
		{
			name:     "empty schema",
			schema:   json.RawMessage(`{}`),
			expected: "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferFilterType(tt.schema)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEnumOptionsFromSchema(t *testing.T) {
	tests := []struct {
		name     string
		schema   json.RawMessage
		expected int
	}{
		{
			name:     "no enum returns nil",
			schema:   json.RawMessage(`{"type":"string"}`),
			expected: -1, // nil
		},
		{
			name:     "with enum values",
			schema:   json.RawMessage(`{"type":"string","enum":["active","inactive"]}`),
			expected: 2,
		},
		{
			name:     "empty enum returns empty slice",
			schema:   json.RawMessage(`{"type":"string","enum":[]}`),
			expected: 0,
		},
		{
			name:     "invalid enum returns nil",
			schema:   json.RawMessage(`{"type":"string","enum":"invalid"}`),
			expected: -1, // nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := enumOptionsFromSchema(tt.schema)
			if tt.expected == -1 {
				assert.Nil(t, result)
			} else {
				assert.Len(t, result, tt.expected)
			}
		})
	}
}

func TestShouldBlockProposal(t *testing.T) {
	tests := []struct {
		name     string
		diags    []spec.Diagnostic
		expected bool
	}{
		{
			name:     "empty diagnostics",
			diags:    nil,
			expected: false,
		},
		{
			name: "warning only",
			diags: []spec.Diagnostic{
				{Code: "some_warning", Severity: spec.SeverityWarning},
			},
			expected: false,
		},
		{
			name: "error but not blocking",
			diags: []spec.Diagnostic{
				{Code: "some_error", Severity: spec.SeverityError},
			},
			expected: false,
		},
		{
			name: "function_id_missing blocks",
			diags: []spec.Diagnostic{
				{Code: "function_id_missing", Severity: spec.SeverityError},
			},
			expected: true,
		},
		{
			name: "function_disabled blocks",
			diags: []spec.Diagnostic{
				{Code: "function_disabled", Severity: spec.SeverityError},
			},
			expected: true,
		},
		{
			name: "mixed diagnostics with blocking",
			diags: []spec.Diagnostic{
				{Code: "some_warning", Severity: spec.SeverityWarning},
				{Code: "function_disabled", Severity: spec.SeverityError},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldBlockProposal(tt.diags)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateBlockedProposalIssue(t *testing.T) {
	diags := []spec.Diagnostic{
		{Code: "function_id_missing", Severity: spec.SeverityError, Message: "Function ID is required"},
	}

	issue := CreateBlockedProposalIssue("game1", "prod", "player", "player.ban", diags, "zh-CN")

	assert.Equal(t, "game1", issue.GameID)
	assert.Equal(t, "prod", issue.Env)
	assert.Equal(t, "player", issue.ResourceKey)
	assert.Equal(t, "player.ban", issue.FunctionID)
	assert.Equal(t, "open", issue.Status)
	assert.Len(t, issue.Diagnostics, 1)
	assert.NotEmpty(t, issue.RepairHint)
}

func TestGenerateRepairHint(t *testing.T) {
	tests := []struct {
		name     string
		diags    []spec.Diagnostic
		locale   string
		expected string
	}{
		{
			name:     "empty diagnostics",
			diags:    nil,
			locale:   "zh-CN",
			expected: "No issues found",
		},
		{
			name: "function_id_missing",
			diags: []spec.Diagnostic{
				{Code: "function_id_missing"},
			},
			locale:   "zh-CN",
			expected: "Function ID is required. Please register the function first.",
		},
		{
			name: "function_disabled",
			diags: []spec.Diagnostic{
				{Code: "function_disabled"},
			},
			locale:   "zh-CN",
			expected: "Function is disabled. Please enable it before creating a page.",
		},
		{
			name: "other error code",
			diags: []spec.Diagnostic{
				{Code: "other_error", Message: "Something went wrong"},
			},
			locale:   "zh-CN",
			expected: "Something went wrong",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateRepairHint(tt.diags, tt.locale)
			assert.Equal(t, tt.expected, result[tt.locale])
		})
	}
}

func TestContractToFunctionSpec(t *testing.T) {
	t.Run("nil contract returns empty spec", func(t *testing.T) {
		result := contractToFunctionSpec(nil)
		assert.Empty(t, result.ID)
	})

	t.Run("contract with all fields", func(t *testing.T) {
		contract := &model.FunctionContract{
			FunctionID:   "player.ban",
			Version:      "1.0.0",
			Enabled:      true,
			Deprecated:   false,
			InputSchema:  datatypes.JSON(`{"type":"object"}`),
			OutputSchema: datatypes.JSON(`{"type":"object"}`),
			Summary:      datatypes.JSONMap{"zh-CN": "封禁玩家"},
			Description:  datatypes.JSONMap{"zh-CN": "封禁指定玩家"},
			ResourceKey:  "player",
			OperationKey: "ban",
			Capability:   "action",
			Execution:    "sync",
			Risk:         "high",
			Permission:   "player:ban",
		}

		result := contractToFunctionSpec(contract)

		assert.Equal(t, "player.ban", result.ID)
		assert.Equal(t, "1.0.0", result.Version)
		assert.True(t, result.Enabled)
		assert.False(t, result.Deprecated)
		assert.Equal(t, "player", result.Resource)
		assert.Equal(t, "ban", result.Operation)
		assert.Equal(t, spec.CapabilityKind("action"), result.Capability)
		assert.Equal(t, spec.FunctionExecution("sync"), result.Execution)
		assert.Equal(t, spec.RiskLevel("high"), result.Risk)
		assert.Equal(t, "player:ban", result.Permission)
	})

	t.Run("contract with nil maps", func(t *testing.T) {
		contract := &model.FunctionContract{
			FunctionID: "test",
		}

		result := contractToFunctionSpec(contract)

		assert.Equal(t, "test", result.ID)
		assert.Nil(t, result.Summary)
		assert.Nil(t, result.Description)
		assert.Empty(t, result.Approval)
	})
}

func TestOutputShapeForSchema(t *testing.T) {
	tests := []struct {
		name     string
		schema   spec.JSONSchema
		expected spec.OutputResultShape
	}{
		{
			name:     "empty schema",
			schema:   nil,
			expected: spec.OutputShapeScalar,
		},
		{
			name:     "array schema",
			schema:   spec.JSONSchema(`{"type":"array"}`),
			expected: spec.OutputShapeCollection,
		},
		{
			name:     "object schema",
			schema:   spec.JSONSchema(`{"type":"object"}`),
			expected: spec.OutputShapeObject,
		},
		{
			name:     "string schema",
			schema:   spec.JSONSchema(`{"type":"string"}`),
			expected: spec.OutputShapeScalar,
		},
		{
			name:     "integer schema",
			schema:   spec.JSONSchema(`{"type":"integer"}`),
			expected: spec.OutputShapeScalar,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := outputShapeForSchema(tt.schema)
			assert.Equal(t, tt.expected, result)
		})
	}
}
