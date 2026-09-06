package generator

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
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

	issue := CreateBlockedProposalIssue("game1", "prod", "player", "player.ban", diags)

	assert.Equal(t, "game1", issue.GameID)
	assert.Equal(t, "prod", issue.Env)
	assert.Equal(t, "player", issue.ResourceKey)
	assert.Equal(t, "player.ban", issue.FunctionID)
	assert.Equal(t, "open", issue.Status)
	assert.Len(t, issue.Diagnostics, 1)
	assert.NotEmpty(t, issue.RepairHint)
	assert.Equal(t, "缺少函数 ID，请先注册函数。", issue.RepairHint["zh-CN"])
	assert.Equal(t, "Function ID is required. Please register the function first.", issue.RepairHint["en-US"])
}

func TestGenerateRepairHint(t *testing.T) {
	tests := []struct {
		name       string
		diags      []spec.Diagnostic
		expectedZh string
		expectedEn string
	}{
		{
			name:       "empty diagnostics",
			diags:      nil,
			expectedZh: "未发现问题",
			expectedEn: "No issues found",
		},
		{
			name: "function_id_missing",
			diags: []spec.Diagnostic{
				{Code: "function_id_missing"},
			},
			expectedZh: "缺少函数 ID，请先注册函数。",
			expectedEn: "Function ID is required. Please register the function first.",
		},
		{
			name: "function_disabled",
			diags: []spec.Diagnostic{
				{Code: "function_disabled"},
			},
			expectedZh: "函数已禁用，创建页面前请先启用。",
			expectedEn: "Function is disabled. Please enable it before creating a page.",
		},
		{
			name: "other error code",
			diags: []spec.Diagnostic{
				{Code: "other_error", Message: "Something went wrong"},
			},
			expectedZh: "Something went wrong",
			expectedEn: "Something went wrong",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateRepairHint(tt.diags)
			assert.Equal(t, tt.expectedZh, result["zh-CN"])
			assert.Equal(t, tt.expectedEn, result["en-US"])
			assert.Len(t, result, 2)
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
			InputSchema:  model.JSON(`{"type":"object"}`),
			OutputSchema: model.JSON(`{"type":"object"}`),
			Summary:      datatypes.JSONMap{"zh-CN": "封禁玩家"},
			Description:  datatypes.JSONMap{"zh-CN": "封禁指定玩家"},
			ResourceKey:  "player",
			OperationKey: "ban",
			Capability:   dbenum.CapabilityAction,
			Execution:    "sync",
			Risk:         dbenum.RiskHigh,
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

func TestInferDataType(t *testing.T) {
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
			name:     "boolean type",
			schema:   json.RawMessage(`{"type":"boolean"}`),
			expected: "boolean",
		},
		{
			name:     "string type",
			schema:   json.RawMessage(`{"type":"string"}`),
			expected: "string",
		},
		{
			name:     "string with date-time format",
			schema:   json.RawMessage(`{"type":"string","format":"date-time"}`),
			expected: "datetime",
		},
		{
			name:     "string with date format",
			schema:   json.RawMessage(`{"type":"string","format":"date"}`),
			expected: "datetime",
		},
		{
			name:     "unknown type defaults to string",
			schema:   json.RawMessage(`{"type":"array"}`),
			expected: "string",
		},
		{
			name:     "empty schema",
			schema:   json.RawMessage(`{}`),
			expected: "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferDataType(tt.schema)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInlineActionNeedsConfirm(t *testing.T) {
	tests := []struct {
		name     string
		contract *model.FunctionContract
		expected bool
	}{
		{
			name: "high risk needs confirm",
			contract: &model.FunctionContract{
				Risk: dbenum.RiskHigh,
			},
			expected: true,
		},
		{
			name: "danger risk needs confirm",
			contract: &model.FunctionContract{
				Risk: dbenum.RiskDanger,
			},
			expected: true,
		},
		{
			name: "safe risk no confirm",
			contract: &model.FunctionContract{
				Risk: dbenum.RiskSafe,
			},
			expected: false,
		},
		{
			name: "warning risk no confirm",
			contract: &model.FunctionContract{
				Risk: dbenum.RiskWarning,
			},
			expected: false,
		},
		{
			name: "empty risk no confirm",
			contract: &model.FunctionContract{
				Risk: dbenum.RiskSafe,
			},
			expected: false,
		},
		{
			name: "approval required needs confirm",
			contract: &model.FunctionContract{
				Approval: datatypes.JSONMap{"required": true},
				Risk:     dbenum.RiskSafe,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inlineActionNeedsConfirm(tt.contract)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResourceQuality(t *testing.T) {
	tests := []struct {
		name      string
		semantics *model.CapabilitySemantics
		diags     []spec.Diagnostic
		expected  spec.GeneratedPageQuality
	}{
		{
			name:      "nil semantics with no diags",
			semantics: nil,
			diags:     nil,
			expected:  spec.GeneratedPageQualityBasic,
		},
		{
			name:      "error diagnostic returns needs review",
			semantics: nil,
			diags: []spec.Diagnostic{
				{Code: "some_error", Severity: spec.SeverityError},
			},
			expected: spec.GeneratedPageQualityNeedsReview,
		},
		{
			name:      "unsupported schema returns needs review",
			semantics: nil,
			diags: []spec.Diagnostic{
				{Code: "json_schema_generation_subset_unsupported", Severity: spec.SeverityWarning},
			},
			expected: spec.GeneratedPageQualityNeedsReview,
		},
		{
			name:      "warning diagnostic returns basic",
			semantics: nil,
			diags: []spec.Diagnostic{
				{Code: "some_warning", Severity: spec.SeverityWarning},
			},
			expected: spec.GeneratedPageQualityBasic,
		},
		{
			name: "semantics with collection and identity returns ready",
			semantics: &model.CapabilitySemantics{
				CollectionQueryID: 1,
				IdentityField:     "id",
			},
			diags:    nil,
			expected: spec.GeneratedPageQualityReady,
		},
		{
			name: "semantics without collection returns basic",
			semantics: &model.CapabilitySemantics{
				IdentityField: "id",
			},
			diags:    nil,
			expected: spec.GeneratedPageQualityBasic,
		},
		{
			name: "semantics without identity returns basic",
			semantics: &model.CapabilitySemantics{
				CollectionQueryID: 1,
			},
			diags:    nil,
			expected: spec.GeneratedPageQualityBasic,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resourceQuality(tt.semantics, tt.diags)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPointerTokens(t *testing.T) {
	tests := []struct {
		name     string
		pointer  string
		expected []string
	}{
		{"empty pointer", "", nil},
		{"root pointer", "/", []string{""}},
		{"single token", "/name", []string{"name"}},
		{"multiple tokens", "/a/b/c", []string{"a", "b", "c"}},
		{"escaped slash", "/a~1b", []string{"a/b"}},
		{"escaped tilde", "/a~0b", []string{"a~b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pointerTokens(tt.pointer)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestKeyFromPointer(t *testing.T) {
	tests := []struct {
		name     string
		pointer  string
		expected string
	}{
		{"empty pointer returns value", "", "value"},
		{"single token", "/name", "name"},
		{"multiple tokens returns last", "/a/b/c", "c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := keyFromPointer(tt.pointer)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDataTypeFromSchema(t *testing.T) {
	tests := []struct {
		name     string
		schema   map[string]json.RawMessage
		expected string
	}{
		{"integer type", map[string]json.RawMessage{"type": json.RawMessage(`"integer"`)}, "number"},
		{"number type", map[string]json.RawMessage{"type": json.RawMessage(`"number"`)}, "number"},
		{"boolean type", map[string]json.RawMessage{"type": json.RawMessage(`"boolean"`)}, "boolean"},
		{"array type", map[string]json.RawMessage{"type": json.RawMessage(`"array"`)}, "array"},
		{"object type", map[string]json.RawMessage{"type": json.RawMessage(`"object"`)}, "object"},
		{"string type", map[string]json.RawMessage{"type": json.RawMessage(`"string"`)}, "string"},
		{"string with date-time format", map[string]json.RawMessage{"type": json.RawMessage(`"string"`), "format": json.RawMessage(`"date-time"`)}, "datetime"},
		{"string with date format", map[string]json.RawMessage{"type": json.RawMessage(`"string"`), "format": json.RawMessage(`"date"`)}, "date"},
		{"unknown type defaults to string", map[string]json.RawMessage{}, "string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dataTypeFromSchema(tt.schema)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name     string
		values   []string
		expected string
	}{
		{"all empty", []string{"", "", ""}, ""},
		{"first non-empty", []string{"a", "b", "c"}, "a"},
		{"second non-empty", []string{"", "b", "c"}, "b"},
		{"third non-empty", []string{"", "", "c"}, "c"},
		{"with whitespace", []string{"  ", "b", "c"}, "b"},
		{"single value", []string{"hello"}, "hello"},
		{"empty slice", []string{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := firstNonEmpty(tt.values...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTermDictionaryLocalization(t *testing.T) {
	terms := TermDictionary{
		"resource/inventory": spec.LocalizedText{"zh-CN": "道具", "en-US": "Item"},
		"operation/consume":  spec.LocalizedText{"zh-CN": "消耗", "en-US": "Consume"},
	}

	t.Run("category labels resolve through resource terms", func(t *testing.T) {
		page := GenerateForOperation(spec.OperationSpec{
			FunctionID: "inventory.consume",
			Operation:  "consume",
			Enabled:    true,
		}, GenerateOptions{DefaultLocale: "zh-CN", Terms: terms})
		assert.Equal(t, "inventory", page.Category.Key)
		assert.Equal(t, "道具", page.Category.Labels["zh-CN"])
		assert.Equal(t, "Item", page.Category.Labels["en-US"])
	})

	t.Run("title falls back to operation term when summary missing", func(t *testing.T) {
		page := GenerateForOperation(spec.OperationSpec{
			FunctionID: "inventory.consume",
			Operation:  "consume",
			Enabled:    true,
		}, GenerateOptions{DefaultLocale: "zh-CN", Terms: terms})
		require.NotNil(t, page.Operation)
		assert.Equal(t, "消耗", page.Title["zh-CN"])
	})

	t.Run("summary still wins over terms", func(t *testing.T) {
		page := GenerateForOperation(spec.OperationSpec{
			FunctionID: "inventory.consume",
			Operation:  "consume",
			Enabled:    true,
		}, GenerateOptions{
			DefaultLocale: "zh-CN",
			Terms:         terms,
			Functions: map[string]spec.FunctionSpec{
				"inventory.consume": {ID: "inventory.consume", Summary: spec.LocalizedText{"zh-CN": "消耗背包道具"}},
			},
		})
		assert.Equal(t, "消耗背包道具", page.Title["zh-CN"])
	})

	t.Run("missing terms keep humanize fallback", func(t *testing.T) {
		page := GenerateForOperation(spec.OperationSpec{
			FunctionID: "unknown.zone",
			Operation:  "zone",
			Enabled:    true,
		}, GenerateOptions{DefaultLocale: "zh-CN", Terms: terms})
		assert.Equal(t, "unknown", page.Category.Key)
		assert.Equal(t, "Unknown", page.Category.Labels["zh-CN"])
	})
}

func TestResourcePageProposalUsesResourceTerm(t *testing.T) {
	terms := TermDictionary{
		"resource/inventory": spec.LocalizedText{"zh-CN": "道具", "en-US": "Item"},
	}
	collection := &model.FunctionContract{
		Model:        gormModelWithID(501),
		FunctionID:   "inventory.list",
		ResourceKey:  "inventory",
		Capability:   dbenum.CapabilityCollectionQuery,
		Enabled:      true,
		OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"}},"required":["id"]}},"total":{"type":"integer"}},"required":["items","total"]}`),
	}
	semantics := &model.CapabilitySemantics{
		ResourceKey:       "inventory",
		CollectionQueryID: collection.ID,
		IdentityField:     "id",
		ItemsFieldName:    "items",
		TotalFieldName:    "total",
	}
	generated, ok := GenerateResourcePageProposal(semantics, []*model.FunctionContract{collection}, GenerateOptions{DefaultLocale: "zh-CN", Terms: terms})
	require.True(t, ok)
	assert.Equal(t, "道具", generated.Title["zh-CN"])
	assert.Equal(t, "Item", generated.Title["en-US"])
	assert.Equal(t, "道具", generated.Category.Labels["zh-CN"])
}
