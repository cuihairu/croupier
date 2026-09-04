package spec

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── form_presentation.go ─────────────────────────────────────────

func TestDefaultFormPresentation(t *testing.T) {
	schema := JSONSchema(`{"type":"object","properties":{"name":{"type":"string"}}}`)
	fp := DefaultFormPresentation(schema)
	require.NotNil(t, fp)
	assert.Equal(t, FormLayoutVertical, fp.Layout)
	assert.Equal(t, "提交", fp.SubmitButton.Text["zh-CN"])
	assert.Equal(t, "Submit", fp.SubmitButton.Text["en-US"])
	assert.Equal(t, "primary", fp.SubmitButton.Type)
	assert.Equal(t, "取消", fp.CancelButton.Text["zh-CN"])
	assert.Equal(t, "Cancel", fp.CancelButton.Text["en-US"])
	// Schema should be stored as-is
	assert.Equal(t, schema, JSONSchema(fp.JSONSchema))
}

// ── types.go validators ──────────────────────────────────────────

func TestIsValidCapabilityKind(t *testing.T) {
	tests := []struct {
		cap  CapabilityKind
		want bool
	}{
		{CapabilityCollectionQuery, true},
		{CapabilityItemQuery, true},
		{CapabilityCreate, true},
		{CapabilityUpdate, true},
		{CapabilityDelete, true},
		{CapabilityAction, true},
		{CapabilityTask, true},
		{CapabilityReport, true},
		{"unknown", false},
		{"", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, IsValidCapabilityKind(tt.cap), "capability=%s", tt.cap)
	}
}

func TestIsValidFunctionExecution(t *testing.T) {
	assert.True(t, IsValidFunctionExecution(FunctionExecutionSync))
	assert.True(t, IsValidFunctionExecution(FunctionExecutionTask))
	assert.False(t, IsValidFunctionExecution("unknown"))
	assert.False(t, IsValidFunctionExecution(""))
}

func TestIsValidJsonScalarType(t *testing.T) {
	assert.True(t, IsValidJsonScalarType(JsonScalarString))
	assert.True(t, IsValidJsonScalarType(JsonScalarNumber))
	assert.True(t, IsValidJsonScalarType(JsonScalarInteger))
	assert.True(t, IsValidJsonScalarType(JsonScalarBoolean))
	assert.False(t, IsValidJsonScalarType("object"))
	assert.False(t, IsValidJsonScalarType(""))
}

// ── page_publish_validation.go ───────────────────────────────────

func TestValidatePublishablePageShape_PageTypeInvalid(t *testing.T) {
	page := PageSpec{
		Type: PageType("invalid"),
		// Need exactly one body to pass variant check, but wrong type
		Resource: &ResourcePageSpec{},
	}
	diags := ValidatePublishablePageShape(page)
	require.NotEmpty(t, diags)
	assert.Equal(t, "page_type_invalid", diags[0].Code)
}

func TestValidatePublishableResourcePage_NilResource(t *testing.T) {
	page := PageSpec{Type: PageTypeResource}
	diags := ValidatePublishablePageShape(page)
	require.NotEmpty(t, diags)
	// Should fail because page_variant_invalid (no body)
	assert.Equal(t, "page_variant_invalid", diags[0].Code)
}

func TestValidatePublishableResourcePage_IdentityKeyMissing(t *testing.T) {
	page := PageSpec{
		Type: PageTypeResource,
		Resource: &ResourcePageSpec{
			ListView: &ListViewSpec{
				Columns: []ColumnSpec{{Key: "name"}},
			},
		},
		Bindings: []PageFunctionBinding{{ID: "query", Usage: BindingUsageQuery}},
	}
	diags := ValidatePublishablePageShape(page)
	require.NotEmpty(t, diags)
	assert.Equal(t, "resource_identity_key_missing", diags[0].Code)
}

func TestValidatePublishableResourcePage_IdentityKeyInvalid(t *testing.T) {
	page := PageSpec{
		Type: PageTypeResource,
		Resource: &ResourcePageSpec{
			ListView: &ListViewSpec{
				IdentityKey: "nonexistent",
				Columns:     []ColumnSpec{{Key: "name"}},
			},
		},
		Bindings: []PageFunctionBinding{{ID: "query", Usage: BindingUsageQuery}},
	}
	diags := ValidatePublishablePageShape(page)
	require.NotEmpty(t, diags)
	assert.Equal(t, "resource_identity_key_invalid", diags[0].Code)
}

func TestValidatePublishableResourcePage_Valid(t *testing.T) {
	page := PageSpec{
		Type: PageTypeResource,
		Resource: &ResourcePageSpec{
			ListView: &ListViewSpec{
				IdentityKey: "id",
				Columns:     []ColumnSpec{{Key: "id"}, {Key: "name"}},
			},
		},
		Bindings: []PageFunctionBinding{{ID: "query", Usage: BindingUsageQuery}},
	}
	diags := ValidatePublishablePageShape(page)
	assert.Empty(t, diags)
}

func TestValidatePublishableResourcePage_NilListView(t *testing.T) {
	page := PageSpec{
		Type:     PageTypeResource,
		Resource: &ResourcePageSpec{},
		Bindings: []PageFunctionBinding{{ID: "query", Usage: BindingUsageQuery}},
	}
	diags := ValidatePublishablePageShape(page)
	// validatePublishableResourcePage returns nil when ListView is nil
	assert.Empty(t, diags)
}

func TestValidatePublishableTaskPage_NilTask(t *testing.T) {
	page := PageSpec{
		Type:     PageTypeTask,
		Task:     nil,
		Bindings: []PageFunctionBinding{{ID: "task", Usage: BindingUsageTask}},
	}
	diags := ValidatePublishablePageShape(page)
	// nil task body → page_variant_invalid (no matching body)
	require.NotEmpty(t, diags)
	assert.Equal(t, "page_variant_invalid", diags[0].Code)
}

func TestValidatePublishableTaskPage_FormMissing(t *testing.T) {
	page := PageSpec{
		Type: PageTypeTask,
		Task: &TaskPageSpec{
			TaskView: &TaskViewSpec{
				TaskIDStateKey:  "taskId",
				StatusBindingID: "status",
				StatusStatePath: "/status",
			},
		},
		Bindings: []PageFunctionBinding{
			{ID: "task", Usage: BindingUsageTask},
			{ID: "status", Usage: BindingUsageTaskStatus},
		},
	}
	diags := ValidatePublishablePageShape(page)
	assertHasDiagnosticCode(t, diags, "task_form_missing")
}

func TestValidatePublishableTaskPage_TaskViewMissing(t *testing.T) {
	page := PageSpec{
		Type: PageTypeTask,
		Task: &TaskPageSpec{
			Form: &FormPresentationSpec{},
		},
		Bindings: []PageFunctionBinding{{ID: "task", Usage: BindingUsageTask}},
	}
	diags := ValidatePublishablePageShape(page)
	assertHasDiagnosticCode(t, diags, "task_view_missing")
}

func TestValidatePublishableTaskPage_MissingFields(t *testing.T) {
	page := PageSpec{
		Type: PageTypeTask,
		Task: &TaskPageSpec{
			Form:     &FormPresentationSpec{},
			TaskView: &TaskViewSpec{},
		},
		Bindings: []PageFunctionBinding{{ID: "task", Usage: BindingUsageTask}},
	}
	diags := ValidatePublishablePageShape(page)
	// Should have: taskIdStateKey missing, statusBinding missing, statusStatePath missing
	assertHasDiagnosticCode(t, diags, "task_id_state_key_missing")
	assertHasDiagnosticCode(t, diags, "task_status_binding_missing")
	assertHasDiagnosticCode(t, diags, "task_status_state_path_missing")
}

func TestValidatePublishableTaskPage_InvalidStatusBinding(t *testing.T) {
	page := PageSpec{
		Type: PageTypeTask,
		Task: &TaskPageSpec{
			Form: &FormPresentationSpec{},
			TaskView: &TaskViewSpec{
				TaskIDStateKey:  "taskId",
				StatusBindingID: "wrong",
				StatusStatePath: "/state",
			},
		},
		Bindings: []PageFunctionBinding{
			{ID: "task", Usage: BindingUsageTask},
			{ID: "status", Usage: BindingUsageTaskStatus},
		},
	}
	diags := ValidatePublishablePageShape(page)
	assertHasDiagnosticCode(t, diags, "task_status_binding_invalid")
}

func TestValidatePublishableTaskPage_InvalidStatusStatePath(t *testing.T) {
	page := PageSpec{
		Type: PageTypeTask,
		Task: &TaskPageSpec{
			Form: &FormPresentationSpec{},
			TaskView: &TaskViewSpec{
				TaskIDStateKey:  "taskId",
				StatusBindingID: "status",
				StatusStatePath: "not-a-pointer",
			},
		},
		Bindings: []PageFunctionBinding{
			{ID: "task", Usage: BindingUsageTask},
			{ID: "status", Usage: BindingUsageTaskStatus},
		},
	}
	diags := ValidatePublishablePageShape(page)
	assertHasDiagnosticCode(t, diags, "task_status_state_path_invalid")
}

func TestValidatePublishableTaskPage_EventsBindingMissing(t *testing.T) {
	page := PageSpec{
		Type: PageTypeTask,
		Task: &TaskPageSpec{
			Form: &FormPresentationSpec{},
			TaskView: &TaskViewSpec{
				TaskIDStateKey:  "taskId",
				StatusBindingID: "status",
				StatusStatePath: "/state",
				ShowEvents:      true,
			},
		},
		Bindings: []PageFunctionBinding{
			{ID: "task", Usage: BindingUsageTask},
			{ID: "status", Usage: BindingUsageTaskStatus},
		},
	}
	diags := ValidatePublishablePageShape(page)
	assertHasDiagnosticCode(t, diags, "task_events_binding_missing")
}

func TestValidatePublishableTaskPage_InvalidEventsBinding(t *testing.T) {
	page := PageSpec{
		Type: PageTypeTask,
		Task: &TaskPageSpec{
			Form: &FormPresentationSpec{},
			TaskView: &TaskViewSpec{
				TaskIDStateKey:  "taskId",
				StatusBindingID: "status",
				StatusStatePath: "/state",
				EventsBindingID: "wrong",
			},
		},
		Bindings: []PageFunctionBinding{
			{ID: "task", Usage: BindingUsageTask},
			{ID: "status", Usage: BindingUsageTaskStatus},
			{ID: "events", Usage: BindingUsageTaskEvents},
		},
	}
	diags := ValidatePublishablePageShape(page)
	assertHasDiagnosticCode(t, diags, "task_events_binding_invalid")
}

func TestValidatePublishableTaskPage_CancelableMissing(t *testing.T) {
	page := PageSpec{
		Type: PageTypeTask,
		Task: &TaskPageSpec{
			Form: &FormPresentationSpec{},
			TaskView: &TaskViewSpec{
				TaskIDStateKey:  "taskId",
				StatusBindingID: "status",
				StatusStatePath: "/state",
				Cancelable:      true,
			},
		},
		Bindings: []PageFunctionBinding{
			{ID: "task", Usage: BindingUsageTask},
			{ID: "status", Usage: BindingUsageTaskStatus},
		},
	}
	diags := ValidatePublishablePageShape(page)
	assertHasDiagnosticCode(t, diags, "task_cancel_binding_missing")
}

func TestValidatePublishableTaskPage_InvalidCancelBinding(t *testing.T) {
	page := PageSpec{
		Type: PageTypeTask,
		Task: &TaskPageSpec{
			Form: &FormPresentationSpec{},
			TaskView: &TaskViewSpec{
				TaskIDStateKey:  "taskId",
				StatusBindingID: "status",
				StatusStatePath: "/state",
				CancelBindingID: "wrong",
			},
		},
		Bindings: []PageFunctionBinding{
			{ID: "task", Usage: BindingUsageTask},
			{ID: "status", Usage: BindingUsageTaskStatus},
			{ID: "cancel", Usage: BindingUsageTaskCancel},
		},
	}
	diags := ValidatePublishablePageShape(page)
	assertHasDiagnosticCode(t, diags, "task_cancel_binding_invalid")
}

func TestValidatePublishableTaskPage_InvalidResultBinding(t *testing.T) {
	page := PageSpec{
		Type: PageTypeTask,
		Task: &TaskPageSpec{
			Form: &FormPresentationSpec{},
			TaskView: &TaskViewSpec{
				TaskIDStateKey:  "taskId",
				StatusBindingID: "status",
				StatusStatePath: "/state",
				ResultBindingID: "wrong",
			},
		},
		Bindings: []PageFunctionBinding{
			{ID: "task", Usage: BindingUsageTask},
			{ID: "status", Usage: BindingUsageTaskStatus},
			{ID: "result", Usage: BindingUsageTaskResult},
		},
	}
	diags := ValidatePublishablePageShape(page)
	assertHasDiagnosticCode(t, diags, "task_result_binding_invalid")
}

func TestValidatePublishableTaskPage_RetryUnavailable(t *testing.T) {
	page := PageSpec{
		Type: PageTypeTask,
		Task: &TaskPageSpec{
			Form: &FormPresentationSpec{},
			TaskView: &TaskViewSpec{
				TaskIDStateKey:  "taskId",
				StatusBindingID: "status",
				StatusStatePath: "/state",
				Retryable:       true,
			},
		},
		Bindings: []PageFunctionBinding{
			{ID: "task", Usage: BindingUsageTask},
			{ID: "status", Usage: BindingUsageTaskStatus},
		},
	}
	diags := ValidatePublishablePageShape(page)
	assertHasDiagnosticCode(t, diags, "task_retry_unavailable")
}

func TestValidatePublishableReportPage_NilReport(t *testing.T) {
	page := PageSpec{
		Type:     PageTypeReport,
		Report:   nil,
		Bindings: []PageFunctionBinding{{ID: "report", Usage: BindingUsageReport}},
	}
	diags := ValidatePublishablePageShape(page)
	// nil report body → page_variant_invalid (no matching body)
	require.NotEmpty(t, diags)
	assert.Equal(t, "page_variant_invalid", diags[0].Code)
}

func TestValidatePublishableReportPage_QueryFormMissing(t *testing.T) {
	page := PageSpec{
		Type: PageTypeReport,
		Report: &ReportPageSpec{
			Dataset: &DatasetSpec{
				Dimensions: []DimensionSpec{{Key: "k", Title: LocalizedText{"zh-CN": "维度"}, DataType: "string"}},
				Metrics:    []MetricSpec{{Key: "m", Title: LocalizedText{"zh-CN": "指标"}, DataType: "number"}},
			},
		},
		Bindings: []PageFunctionBinding{{ID: "report", Usage: BindingUsageReport}},
	}
	diags := ValidatePublishablePageShape(page)
	assertHasDiagnosticCode(t, diags, "report_query_form_missing")
}

func TestValidatePublishableReportPage_DatasetMissing(t *testing.T) {
	page := PageSpec{
		Type: PageTypeReport,
		Report: &ReportPageSpec{
			QueryForm: &FormPresentationSpec{},
		},
		Bindings: []PageFunctionBinding{{ID: "report", Usage: BindingUsageReport}},
	}
	diags := ValidatePublishablePageShape(page)
	assertHasDiagnosticCode(t, diags, "report_dataset_missing")
}

func TestValidatePublishableReportPage_DimensionsMetricsMissing(t *testing.T) {
	page := PageSpec{
		Type: PageTypeReport,
		Report: &ReportPageSpec{
			QueryForm: &FormPresentationSpec{},
			Dataset:   &DatasetSpec{},
		},
		Bindings: []PageFunctionBinding{{ID: "report", Usage: BindingUsageReport}},
	}
	diags := ValidatePublishablePageShape(page)
	assertHasDiagnosticCode(t, diags, "report_dimensions_missing")
	assertHasDiagnosticCode(t, diags, "report_metrics_missing")
}

func TestValidatePublishableReportPage_InvalidDimensionKey(t *testing.T) {
	page := PageSpec{
		Type: PageTypeReport,
		Report: &ReportPageSpec{
			QueryForm: &FormPresentationSpec{},
			Dataset: &DatasetSpec{
				Dimensions: []DimensionSpec{{Key: "", Title: LocalizedText{"zh-CN": "维度"}, DataType: "string"}},
				Metrics:    []MetricSpec{{Key: "m", Title: LocalizedText{"zh-CN": "指标"}, DataType: "number"}},
			},
		},
		Bindings: []PageFunctionBinding{{ID: "report", Usage: BindingUsageReport}},
	}
	diags := ValidatePublishablePageShape(page)
	assertHasDiagnosticCode(t, diags, "report_dimension_key_missing")
}

func TestValidatePublishableReportPage_DimensionTitleMissing(t *testing.T) {
	page := PageSpec{
		Type: PageTypeReport,
		Report: &ReportPageSpec{
			QueryForm: &FormPresentationSpec{},
			Dataset: &DatasetSpec{
				Dimensions: []DimensionSpec{{Key: "k", DataType: "string"}},
				Metrics:    []MetricSpec{{Key: "m", Title: LocalizedText{"zh-CN": "指标"}, DataType: "number"}},
			},
		},
		Bindings: []PageFunctionBinding{{ID: "report", Usage: BindingUsageReport}},
	}
	diags := ValidatePublishablePageShape(page)
	assertHasDiagnosticCode(t, diags, "report_dimension_title_missing")
}

func TestValidatePublishableReportPage_InvalidDimensionType(t *testing.T) {
	page := PageSpec{
		Type: PageTypeReport,
		Report: &ReportPageSpec{
			QueryForm: &FormPresentationSpec{},
			Dataset: &DatasetSpec{
				Dimensions: []DimensionSpec{{Key: "k", Title: LocalizedText{"zh-CN": "维度"}, DataType: "boolean"}},
				Metrics:    []MetricSpec{{Key: "m", Title: LocalizedText{"zh-CN": "指标"}, DataType: "number"}},
			},
		},
		Bindings: []PageFunctionBinding{{ID: "report", Usage: BindingUsageReport}},
	}
	diags := ValidatePublishablePageShape(page)
	assertHasDiagnosticCode(t, diags, "report_dimension_type_invalid")
}

func TestValidatePublishableReportPage_MetricKeyMissing(t *testing.T) {
	page := PageSpec{
		Type: PageTypeReport,
		Report: &ReportPageSpec{
			QueryForm: &FormPresentationSpec{},
			Dataset: &DatasetSpec{
				Dimensions: []DimensionSpec{{Key: "k", Title: LocalizedText{"zh-CN": "维度"}, DataType: "string"}},
				Metrics:    []MetricSpec{{Key: "", Title: LocalizedText{"zh-CN": "指标"}, DataType: "number"}},
			},
		},
		Bindings: []PageFunctionBinding{{ID: "report", Usage: BindingUsageReport}},
	}
	diags := ValidatePublishablePageShape(page)
	assertHasDiagnosticCode(t, diags, "report_metric_key_missing")
}

func TestValidatePublishableReportPage_MetricTitleMissing(t *testing.T) {
	page := PageSpec{
		Type: PageTypeReport,
		Report: &ReportPageSpec{
			QueryForm: &FormPresentationSpec{},
			Dataset: &DatasetSpec{
				Dimensions: []DimensionSpec{{Key: "k", Title: LocalizedText{"zh-CN": "维度"}, DataType: "string"}},
				Metrics:    []MetricSpec{{Key: "m", DataType: "number"}},
			},
		},
		Bindings: []PageFunctionBinding{{ID: "report", Usage: BindingUsageReport}},
	}
	diags := ValidatePublishablePageShape(page)
	assertHasDiagnosticCode(t, diags, "report_metric_title_missing")
}

func TestValidatePublishableReportPage_MetricTypeInvalid(t *testing.T) {
	page := PageSpec{
		Type: PageTypeReport,
		Report: &ReportPageSpec{
			QueryForm: &FormPresentationSpec{},
			Dataset: &DatasetSpec{
				Dimensions: []DimensionSpec{{Key: "k", Title: LocalizedText{"zh-CN": "维度"}, DataType: "string"}},
				Metrics:    []MetricSpec{{Key: "m", Title: LocalizedText{"zh-CN": "指标"}, DataType: "string"}},
			},
		},
		Bindings: []PageFunctionBinding{{ID: "report", Usage: BindingUsageReport}},
	}
	diags := ValidatePublishablePageShape(page)
	assertHasDiagnosticCode(t, diags, "report_metric_type_invalid")
}

func TestHasDefaultLocale(t *testing.T) {
	assert.True(t, hasDefaultLocale(LocalizedText{"zh-CN": "测试"}))
	assert.False(t, hasDefaultLocale(LocalizedText{"en": "test"}))
	assert.False(t, hasDefaultLocale(LocalizedText{"zh-CN": "  "})) // whitespace only
	assert.False(t, hasDefaultLocale(nil))
}

func TestIsReportDimensionType(t *testing.T) {
	assert.True(t, isReportDimensionType("string"))
	assert.True(t, isReportDimensionType("number"))
	assert.True(t, isReportDimensionType("date"))
	assert.False(t, isReportDimensionType("boolean"))
	assert.False(t, isReportDimensionType(""))
}

func TestValidateRequiredOutputAssignments_ResourceQuery(t *testing.T) {
	binding := PageFunctionBinding{
		ID:    "query",
		Usage: BindingUsageQuery,
		Selectors: &BindingSelectors{
			Output: []OutputAssignment{{StateKey: "items", Shape: OutputShapeCollection, Source: "/data"}},
		},
	}
	page := PageSpec{Type: PageTypeResource}
	diags := ValidateRequiredOutputAssignments(binding, page)
	assert.Empty(t, diags)
}

func TestValidateRequiredOutputAssignments_ResourceQuery_Missing(t *testing.T) {
	binding := PageFunctionBinding{
		ID:        "query",
		Usage:     BindingUsageQuery,
		Selectors: &BindingSelectors{Output: []OutputAssignment{}},
	}
	page := PageSpec{Type: PageTypeResource}
	diags := ValidateRequiredOutputAssignments(binding, page)
	assert.NotEmpty(t, diags)
}

func TestValidateRequiredOutputAssignments_ResourceQuery_NilSelectors(t *testing.T) {
	binding := PageFunctionBinding{ID: "query", Usage: BindingUsageQuery}
	page := PageSpec{Type: PageTypeResource}
	diags := ValidateRequiredOutputAssignments(binding, page)
	assert.NotEmpty(t, diags)
}

func TestValidateRequiredOutputAssignments_DetailQuery(t *testing.T) {
	binding := PageFunctionBinding{
		ID:    "detail",
		Usage: BindingUsageDetail,
		Selectors: &BindingSelectors{
			Output: []OutputAssignment{{StateKey: "detail", Shape: OutputShapeObject, Source: "/item"}},
		},
	}
	page := PageSpec{Type: PageTypeResource}
	diags := ValidateRequiredOutputAssignments(binding, page)
	assert.Empty(t, diags)
}

func TestValidateRequiredOutputAssignments_ReportQuery(t *testing.T) {
	binding := PageFunctionBinding{
		ID:    "report",
		Usage: BindingUsageReport,
		Selectors: &BindingSelectors{
			Output: []OutputAssignment{{StateKey: "dataset", Shape: OutputShapeDataset, Source: "/data"}},
		},
	}
	page := PageSpec{Type: PageTypeReport}
	diags := ValidateRequiredOutputAssignments(binding, page)
	assert.Empty(t, diags)
}

func TestValidateRequiredOutputAssignments_TaskStatus(t *testing.T) {
	binding := PageFunctionBinding{
		ID:    "status",
		Usage: BindingUsageTaskStatus,
		Selectors: &BindingSelectors{
			Output: []OutputAssignment{{StateKey: "taskStatus", Shape: OutputShapeObject, Source: "/status"}},
		},
	}
	page := PageSpec{Type: PageTypeTask}
	diags := ValidateRequiredOutputAssignments(binding, page)
	assert.Empty(t, diags)
}

func TestValidateRequiredOutputAssignments_TaskEvents(t *testing.T) {
	binding := PageFunctionBinding{
		ID:    "events",
		Usage: BindingUsageTaskEvents,
		Selectors: &BindingSelectors{
			Output: []OutputAssignment{{StateKey: "taskEvents", Shape: OutputShapeCollection, Source: "/events"}},
		},
	}
	page := PageSpec{Type: PageTypeTask}
	diags := ValidateRequiredOutputAssignments(binding, page)
	assert.Empty(t, diags)
}

func TestValidateRequiredOutputAssignments_TaskResult(t *testing.T) {
	binding := PageFunctionBinding{
		ID:    "result",
		Usage: BindingUsageTaskResult,
		Selectors: &BindingSelectors{
			Output: []OutputAssignment{{StateKey: "taskResult", Shape: OutputShapeObject, Source: "/result"}},
		},
	}
	page := PageSpec{Type: PageTypeTask}
	diags := ValidateRequiredOutputAssignments(binding, page)
	assert.Empty(t, diags)
}

func TestValidateRequiredOutputAssignments_UnknownUsage(t *testing.T) {
	binding := PageFunctionBinding{
		ID:    "other",
		Usage: BindingUsageAction,
	}
	page := PageSpec{Type: PageTypeResource}
	diags := ValidateRequiredOutputAssignments(binding, page)
	assert.Empty(t, diags)
}

// ── selector_ast.go ──────────────────────────────────────────────

func TestSelectorContextForBinding_ResourcePage(t *testing.T) {
	page := PageSpec{
		Type: PageTypeResource,
		Resource: &ResourcePageSpec{
			ListView: &ListViewSpec{
				RowSchema: JSONSchema(`{"type":"object","properties":{"id":{"type":"string"}}}`),
				Filters:   []FilterSpec{{Key: "keyword", Type: "text"}},
			},
			DetailView: &DetailViewSpec{
				Fields: []DetailFieldSpec{{Key: "id"}},
			},
		},
	}
	binding := PageFunctionBinding{
		ID:    "query",
		Usage: BindingUsageQuery,
	}
	ctx := SelectorContextForBinding(page, binding)
	assert.True(t, ctx.HasListView)
	assert.True(t, ctx.HasDetailView)
	assert.NotEmpty(t, ctx.RowSchema)
}

func TestSelectorContextForBinding_TaskPage(t *testing.T) {
	page := PageSpec{
		Type: PageTypeTask,
		Task: &TaskPageSpec{
			TaskView: &TaskViewSpec{TaskIDStateKey: "myTaskId"},
		},
	}
	binding := PageFunctionBinding{ID: "task", Usage: BindingUsageTask}
	ctx := SelectorContextForBinding(page, binding)
	assert.NotNil(t, ctx.PageState)
	assert.Contains(t, ctx.PageState, "myTaskId")
}

func TestSelectorContextForBinding_TaskPage_DefaultTaskID(t *testing.T) {
	page := PageSpec{
		Type: PageTypeTask,
		Task: &TaskPageSpec{
			TaskView: &TaskViewSpec{TaskIDStateKey: ""},
		},
	}
	binding := PageFunctionBinding{ID: "task", Usage: BindingUsageTask}
	ctx := SelectorContextForBinding(page, binding)
	assert.Contains(t, ctx.PageState, "taskId")
}

func TestSelectorContextForBinding_ActionBinding_RowAndBatch(t *testing.T) {
	page := PageSpec{
		Type: PageTypeResource,
		Resource: &ResourcePageSpec{
			ListView: &ListViewSpec{},
		},
	}
	binding := PageFunctionBinding{
		ID:    "action",
		Usage: BindingUsageAction,
		Selectors: &BindingSelectors{
			Input: SelectorAST{
				Assignments: []InputAssignment{
					{Target: "/id", Source: ValueSource{Kind: SourceRow, Path: "/id"}},
					{Target: "/ids", Source: ValueSource{Kind: SourceSelection, Path: "/id", Transform: &TransformSpec{Type: TransformPick}}},
				},
			},
		},
	}
	ctx := SelectorContextForBinding(page, binding)
	assert.True(t, ctx.IsRowAction)
	assert.True(t, ctx.IsBatchAction)
}

func TestFormSchemaForBinding_OperationPage(t *testing.T) {
	page := PageSpec{
		Type: PageTypeOperation,
		Operation: &OperationPageSpec{
			Form: &FormPresentationSpec{JSONSchema: JSONSchema(`{"type":"object"}`)},
		},
	}
	binding := PageFunctionBinding{ID: "op", Usage: BindingUsageAction}
	schema := FormSchemaForBinding(page, binding)
	assert.Equal(t, JSONSchema(`{"type":"object"}`), schema)
}

func TestFormSchemaForBinding_TaskPage(t *testing.T) {
	page := PageSpec{
		Type: PageTypeTask,
		Task: &TaskPageSpec{
			Form: &FormPresentationSpec{JSONSchema: JSONSchema(`{"type":"object"}`)},
		},
	}
	binding := PageFunctionBinding{ID: "task", Usage: BindingUsageTask}
	schema := FormSchemaForBinding(page, binding)
	assert.NotEmpty(t, schema)
}

func TestFormSchemaForBinding_ReportPage(t *testing.T) {
	page := PageSpec{
		Type: PageTypeReport,
		Report: &ReportPageSpec{
			QueryForm: &FormPresentationSpec{JSONSchema: JSONSchema(`{"type":"object"}`)},
		},
	}
	binding := PageFunctionBinding{ID: "report", Usage: BindingUsageReport}
	schema := FormSchemaForBinding(page, binding)
	assert.NotEmpty(t, schema)
}

func TestFormSchemaForBinding_ResourceCreate(t *testing.T) {
	page := PageSpec{
		Type: PageTypeResource,
		Resource: &ResourcePageSpec{
			CreateForm: &FormPresentationSpec{JSONSchema: JSONSchema(`{"type":"object"}`)},
		},
	}
	binding := PageFunctionBinding{ID: "create"}
	schema := FormSchemaForBinding(page, binding)
	assert.NotEmpty(t, schema)
}

func TestFormSchemaForBinding_ResourceUpdate(t *testing.T) {
	page := PageSpec{
		Type: PageTypeResource,
		Resource: &ResourcePageSpec{
			UpdateForm: &FormPresentationSpec{JSONSchema: JSONSchema(`{"type":"object"}`)},
		},
	}
	binding := PageFunctionBinding{ID: "update"}
	schema := FormSchemaForBinding(page, binding)
	assert.NotEmpty(t, schema)
}

func TestFormSchemaForBinding_QueryWithFilters(t *testing.T) {
	page := PageSpec{
		Type: PageTypeResource,
		Resource: &ResourcePageSpec{
			ListView: &ListViewSpec{
				Filters: []FilterSpec{
					{Key: "name", Type: "text"},
					{Key: "age", Type: "number"},
				},
				Pagination: &PaginationSpec{Enabled: true},
			},
		},
	}
	binding := PageFunctionBinding{ID: "list", Usage: BindingUsageQuery}
	schema := FormSchemaForBinding(page, binding)
	assert.NotEmpty(t, schema)
}

func TestFormSchemaForBinding_Default(t *testing.T) {
	page := PageSpec{Type: PageTypeResource}
	binding := PageFunctionBinding{ID: "other", Usage: BindingUsageAction}
	schema := FormSchemaForBinding(page, binding)
	assert.Nil(t, schema)
}

func TestListQuerySchema_NilResource(t *testing.T) {
	schema := listQuerySchema(nil)
	assert.Nil(t, schema)
}

func TestListQuerySchema_NilListView(t *testing.T) {
	schema := listQuerySchema(&ResourcePageSpec{})
	assert.Nil(t, schema)
}

func TestListQuerySchema_EmptyFilters(t *testing.T) {
	schema := listQuerySchema(&ResourcePageSpec{
		ListView: &ListViewSpec{},
	})
	assert.Nil(t, schema)
}

func TestSchemaForFilter(t *testing.T) {
	assert.Equal(t, `{"type":"number"}`, string(schemaForFilter(FilterSpec{Type: "number"})))
	assert.Equal(t, `{"type":"string"}`, string(schemaForFilter(FilterSpec{Type: "date"})))
	assert.Equal(t, `{"type":"string"}`, string(schemaForFilter(FilterSpec{Type: "daterange"})))
	assert.Equal(t, `{"type":"string"}`, string(schemaForFilter(FilterSpec{Type: "text"})))
}

func TestValidateInputSource_Literal(t *testing.T) {
	result := ValidateSelector(SelectorAST{Assignments: []InputAssignment{{
		Target: "/x",
		Source: ValueSource{Kind: SourceLiteral, Value: json.RawMessage(`"hello"`)},
	}}}, JSONSchema(`{"type":"object","properties":{"x":{"type":"string"}},"required":["x"]}`), SelectorContext{})
	assert.True(t, result.Valid)
}

func TestValidateInputSource_PageState_MissingKey(t *testing.T) {
	result := ValidateSelector(SelectorAST{Assignments: []InputAssignment{{
		Target: "/x",
		Source: ValueSource{Kind: SourcePageState, Path: "/val"},
	}}}, JSONSchema(`{"type":"object","properties":{"x":{"type":"string"}},"required":["x"]}`), SelectorContext{})
	assert.False(t, result.Valid)
	assert.Equal(t, ErrCodeMissingRequired, result.Errors[0].Code)
}

func TestValidateInputSource_PageState_InvalidPath(t *testing.T) {
	result := ValidateSelector(SelectorAST{Assignments: []InputAssignment{{
		Target: "/x",
		Source: ValueSource{Kind: SourcePageState, Key: "taskStatus", Path: "not-pointer"},
	}}}, JSONSchema(`{"type":"object","properties":{"x":{"type":"string"}}}`), SelectorContext{
		PageState: map[string]JSONSchema{"taskStatus": JSONSchema(`{"type":"object"}`)},
	})
	assert.False(t, result.Valid)
	assert.Equal(t, ErrCodeInvalidPath, result.Errors[0].Code)
}

func TestValidateInputSource_PageState_PathNotExist(t *testing.T) {
	result := ValidateSelector(SelectorAST{Assignments: []InputAssignment{{
		Target: "/x",
		Source: ValueSource{Kind: SourcePageState, Key: "taskStatus", Path: "/missing"},
	}}}, JSONSchema(`{"type":"object","properties":{"x":{"type":"string"}}}`), SelectorContext{
		PageState: map[string]JSONSchema{"taskStatus": JSONSchema(`{"type":"object","properties":{"status":{"type":"string"}}}`)},
	})
	assert.False(t, result.Valid)
}

func TestValidateInputSource_Form_EmptyPath(t *testing.T) {
	result := ValidateSelector(SelectorAST{Assignments: []InputAssignment{{
		Target: "/x",
		Source: ValueSource{Kind: SourceForm, Path: ""},
	}}}, JSONSchema(`{"type":"object","properties":{"x":{"type":"string"}}}`), SelectorContext{
		FormSchema: JSONSchema(`{"type":"object","properties":{"name":{"type":"string"}}}`),
	})
	assert.False(t, result.Valid)
	assert.Equal(t, ErrCodeMissingRequired, result.Errors[0].Code)
}

func TestValidateInputSource_Form_InvalidPath(t *testing.T) {
	result := ValidateSelector(SelectorAST{Assignments: []InputAssignment{{
		Target: "/x",
		Source: ValueSource{Kind: SourceForm, Path: "not-pointer"},
	}}}, JSONSchema(`{"type":"object","properties":{"x":{"type":"string"}}}`), SelectorContext{
		FormSchema: JSONSchema(`{"type":"object","properties":{"name":{"type":"string"}}}`),
	})
	assert.False(t, result.Valid)
	assert.Equal(t, ErrCodeInvalidPath, result.Errors[0].Code)
}

func TestValidateInputSource_Form_PathNotExist(t *testing.T) {
	result := ValidateSelector(SelectorAST{Assignments: []InputAssignment{{
		Target: "/x",
		Source: ValueSource{Kind: SourceForm, Path: "/missing"},
	}}}, JSONSchema(`{"type":"object","properties":{"x":{"type":"string"}}}`), SelectorContext{
		FormSchema: JSONSchema(`{"type":"object","properties":{"name":{"type":"string"}}}`),
	})
	assert.False(t, result.Valid)
}

func TestValidateOutputAssignments_EmptyStateKey(t *testing.T) {
	result := ValidateOutputAssignments([]OutputAssignment{{
		StateKey: "",
		Source:   "/data",
		Shape:    OutputShapeScalar,
	}}, JSONSchema(`{"type":"object","properties":{"data":{"type":"string"}}}`))
	assert.False(t, result.Valid)
}

func TestValidateOutputAssignments_InvalidSourcePointer(t *testing.T) {
	result := ValidateOutputAssignments([]OutputAssignment{{
		StateKey: "out",
		Source:   "not-pointer",
		Shape:    OutputShapeScalar,
	}}, JSONSchema(`{"type":"object"}`))
	assert.False(t, result.Valid)
}

func TestValidateOutputAssignments_SourceNotFound(t *testing.T) {
	result := ValidateOutputAssignments([]OutputAssignment{{
		StateKey: "out",
		Source:   "/missing",
		Shape:    OutputShapeScalar,
	}}, JSONSchema(`{"type":"object","properties":{"data":{"type":"string"}}}`))
	assert.False(t, result.Valid)
}

func TestValidateOutputAssignments_InvalidShape(t *testing.T) {
	result := ValidateOutputAssignments([]OutputAssignment{{
		StateKey: "out",
		Source:   "/data",
		Shape:    OutputResultShape("invalid"),
	}}, JSONSchema(`{"type":"object","properties":{"data":{"type":"string"}}}`))
	assert.False(t, result.Valid)
}

func TestValidateOutputAssignments_ShapeMismatch(t *testing.T) {
	result := ValidateOutputAssignments([]OutputAssignment{{
		StateKey: "out",
		Source:   "/data",
		Shape:    OutputShapeCollection,
	}}, JSONSchema(`{"type":"object","properties":{"data":{"type":"string"}}}`))
	assert.False(t, result.Valid)
}

func TestValidateOutputAssignments_EmptySchema(t *testing.T) {
	result := ValidateOutputAssignments([]OutputAssignment{{
		StateKey: "out",
		Source:   "/data",
		Shape:    OutputShapeScalar,
	}}, JSONSchema(``))
	assert.True(t, result.Valid)
}

func TestOutputShapeMatchesSchema_Scalar(t *testing.T) {
	schema := JSONSchema(`{"type":"object","properties":{"x":{"type":"string"}}}`)
	assert.True(t, outputShapeMatchesSchema(OutputShapeScalar, schema, "/x"))
	assert.False(t, outputShapeMatchesSchema(OutputShapeCollection, schema, "/x"))
	assert.False(t, outputShapeMatchesSchema(OutputShapeObject, schema, "/x"))
}

func TestOutputShapeMatchesSchema_Array(t *testing.T) {
	schema := JSONSchema(`{"type":"object","properties":{"x":{"type":"array"}}}`)
	assert.True(t, outputShapeMatchesSchema(OutputShapeCollection, schema, "/x"))
	assert.True(t, outputShapeMatchesSchema(OutputShapeDataset, schema, "/x"))
	assert.False(t, outputShapeMatchesSchema(OutputShapeObject, schema, "/x"))
}

func TestOutputShapeMatchesSchema_Object(t *testing.T) {
	schema := JSONSchema(`{"type":"object","properties":{"x":{"type":"object"}}}`)
	assert.True(t, outputShapeMatchesSchema(OutputShapeObject, schema, "/x"))
	assert.True(t, outputShapeMatchesSchema(OutputShapeTask, schema, "/x"))
	assert.False(t, outputShapeMatchesSchema(OutputShapeCollection, schema, "/x"))
}

func TestOutputShapeMatchesSchema_EmptySchema(t *testing.T) {
	assert.True(t, outputShapeMatchesSchema(OutputShapeScalar, JSONSchema(""), "/x"))
}

func TestOutputShapeMatchesSchema_NoType(t *testing.T) {
	schema := JSONSchema(`{"type":"object","properties":{"x":{}}}`)
	assert.True(t, outputShapeMatchesSchema(OutputShapeCollection, schema, "/x"))
}

func TestIsAssignable_LiteralSource(t *testing.T) {
	ctx := SelectorContext{}
	assert.True(t, isAssignable(JSONSchema(`{"type":"object"}`), "/x", ValueSource{Kind: SourceLiteral}, ctx))
}

func TestIsAssignable_NoTargetType(t *testing.T) {
	ctx := SelectorContext{FormSchema: JSONSchema(`{"type":"object","properties":{"x":{"type":"string"}}}`)}
	assert.True(t, isAssignable(JSONSchema(`{"type":"object"}`), "/x", ValueSource{Kind: SourceForm, Path: "/x"}, ctx))
}

func TestIsAssignable_PickTransform(t *testing.T) {
	ctx := SelectorContext{RowSchema: JSONSchema(`{"type":"object","properties":{"id":{"type":"string"}}}`)}
	// Pick transform requires array target
	schema := JSONSchema(`{"type":"object","properties":{"ids":{"type":"array"}}}`)
	assert.True(t, isAssignable(schema, "/ids", ValueSource{
		Kind:      SourceSelection,
		Path:      "/id",
		Transform: &TransformSpec{Type: TransformPick},
	}, ctx))
	// String target should fail
	schema2 := JSONSchema(`{"type":"object","properties":{"name":{"type":"string"}}}`)
	assert.False(t, isAssignable(schema2, "/name", ValueSource{
		Kind:      SourceSelection,
		Path:      "/id",
		Transform: &TransformSpec{Type: TransformPick},
	}, ctx))
}

func TestIsAssignable_OtherTransform(t *testing.T) {
	ctx := SelectorContext{}
	assert.True(t, isAssignable(JSONSchema(`{"type":"object"}`), "/x", ValueSource{
		Kind:      SourceForm,
		Transform: &TransformSpec{Type: TransformType("custom")},
	}, ctx))
}

func TestSourceSchemaAndPath(t *testing.T) {
	ctx := SelectorContext{
		FormSchema:   JSONSchema(`{"type":"object"}`),
		RowSchema:    JSONSchema(`{"type":"object"}`),
		DetailSchema: JSONSchema(`{"type":"object"}`),
		PageState:    map[string]JSONSchema{"task": JSONSchema(`{"type":"object"}`)},
	}
	_, _, ok := sourceSchemaAndPath(ValueSource{Kind: SourceForm, Path: "/x"}, ctx)
	assert.True(t, ok)
	_, _, ok = sourceSchemaAndPath(ValueSource{Kind: SourceRow, Path: "/x"}, ctx)
	assert.True(t, ok)
	_, _, ok = sourceSchemaAndPath(ValueSource{Kind: SourceSelection, Path: "/x"}, ctx)
	assert.True(t, ok)
	_, _, ok = sourceSchemaAndPath(ValueSource{Kind: SourceDetail, Path: "/x"}, ctx)
	assert.True(t, ok)
	_, _, ok = sourceSchemaAndPath(ValueSource{Kind: SourcePageState, Key: "task", Path: "/x"}, ctx)
	assert.True(t, ok)
	_, _, ok = sourceSchemaAndPath(ValueSource{Kind: "unknown", Path: "/x"}, ctx)
	assert.False(t, ok)
}

func TestSchemaHasPath_EmptyPath(t *testing.T) {
	assert.True(t, schemaHasPath(JSONSchema(`{"type":"object"}`), ""))
}

func TestSchemaHasPath_InvalidPath(t *testing.T) {
	assert.False(t, schemaHasPath(JSONSchema(`{"type":"object"}`), "not-pointer"))
}

func TestSchemaHasPath_InvalidJSON(t *testing.T) {
	assert.True(t, schemaHasPath(JSONSchema(`{invalid`), "/x"))
}

func TestSchemaTypeAtPath(t *testing.T) {
	schema := JSONSchema(`{"type":"object","properties":{"x":{"type":"number"}}}`)
	typ, ok := schemaTypeAtPath(schema, "/x")
	assert.True(t, ok)
	assert.Equal(t, "number", typ)
}

func TestSchemaTypeAtPath_NoType(t *testing.T) {
	schema := JSONSchema(`{"type":"object","properties":{"x":{}}}`)
	typ, ok := schemaTypeAtPath(schema, "/x")
	assert.True(t, ok)
	assert.Equal(t, "", typ)
}

func TestSchemaTypeAtPath_ArrayType(t *testing.T) {
	schema := JSONSchema(`{"type":"object","properties":{"x":{"type":["string","null"]}}}`)
	typ, ok := schemaTypeAtPath(schema, "/x")
	assert.True(t, ok)
	// array of types with length != 1 returns ""
	assert.Equal(t, "", typ)
}

func TestSchemaNodeAtPath_InvalidPath(t *testing.T) {
	_, ok := schemaNodeAtPath(JSONSchema(`{"type":"object"}`), "not-pointer")
	assert.False(t, ok)
}

func TestSchemaNodeAtPath_InvalidJSON(t *testing.T) {
	_, ok := schemaNodeAtPath(JSONSchema(`{bad`), "/x")
	assert.False(t, ok)
}

func TestJsonSchemaTypeAssignable(t *testing.T) {
	assert.True(t, jsonSchemaTypeAssignable("string", "string"))
	assert.True(t, jsonSchemaTypeAssignable("number", "integer"))
	assert.False(t, jsonSchemaTypeAssignable("string", "number"))
}

func TestIsJSONPointer(t *testing.T) {
	assert.True(t, isJSONPointer(""))
	assert.True(t, isJSONPointer("/x"))
	assert.False(t, isJSONPointer("x"))
}

func TestJsonPointerTokens(t *testing.T) {
	tokens := jsonPointerTokens("/a/b")
	assert.Equal(t, []string{"a", "b"}, tokens)
	tokens = jsonPointerTokens("")
	assert.Nil(t, tokens)
	tokens = jsonPointerTokens("/a~1b/c~0d")
	assert.Equal(t, []string{"a/b", "c~d"}, tokens)
}

func TestEscapeJSONPointerToken(t *testing.T) {
	assert.Equal(t, "a~1b", escapeJSONPointerToken("a/b"))
	assert.Equal(t, "a~0b", escapeJSONPointerToken("a~b"))
}

func TestRawObject(t *testing.T) {
	_, ok := rawObject(nil)
	assert.False(t, ok)
	_, ok = rawObject(json.RawMessage(`"string"`))
	assert.False(t, ok)
	obj, ok := rawObject(json.RawMessage(`{"a":1}`))
	assert.True(t, ok)
	assert.Contains(t, obj, "a")
}

func TestSortStrings(t *testing.T) {
	s := []string{"c", "a", "b"}
	sortStrings(s)
	assert.Equal(t, []string{"a", "b", "c"}, s)
}

func TestParentPointer(t *testing.T) {
	assert.Equal(t, "", parentPointer(""))
	assert.Equal(t, "", parentPointer("/a"))
	assert.Equal(t, "/a", parentPointer("/a/b"))
}

func TestDiffJSONSchemaFields_InvalidSchema(t *testing.T) {
	result := DiffJSONSchemaFields(JSONSchema(`{bad`), JSONSchema(`{bad`))
	assert.NotEmpty(t, result.Diagnostics)
	assert.Equal(t, "schema_diff_invalid_schema", result.Diagnostics[0].Code)
}

func TestDiffJSONSchemaFields_EmptySchemas(t *testing.T) {
	result := DiffJSONSchemaFields(JSONSchema(``), JSONSchema(``))
	assert.Empty(t, result.Changes)
}

func TestDefaultSelector_InvalidSchema(t *testing.T) {
	selector := DefaultSelector(JSONSchema(`{bad`))
	assert.Empty(t, selector.Assignments)
}

func TestDefaultSelector_NoProperties(t *testing.T) {
	selector := DefaultSelector(JSONSchema(`{"type":"object"}`))
	assert.Empty(t, selector.Assignments)
}

// helpers
func assertHasDiagnosticCode(t *testing.T, diags []Diagnostic, code string) {
	t.Helper()
	for _, d := range diags {
		if d.Code == code {
			return
		}
	}
	t.Errorf("expected diagnostic code %q in %v", code, diags)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
