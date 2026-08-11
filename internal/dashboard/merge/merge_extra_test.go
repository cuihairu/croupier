package merge

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// bytesEqual edge cases
// ---------------------------------------------------------------------------

func TestBytesEqual(t *testing.T) {
	assert.True(t, bytesEqual(nil, nil))
	assert.True(t, bytesEqual([]byte{}, []byte{}))
	assert.True(t, bytesEqual(nil, []byte{}))
	assert.True(t, bytesEqual([]byte{}, nil))
	assert.False(t, bytesEqual(nil, []byte(`"x"`)))
	assert.False(t, bytesEqual([]byte(`"x"`), nil))
	assert.True(t, bytesEqual([]byte(`"a"`), []byte(`"a"`)))
	assert.False(t, bytesEqual([]byte(`"a"`), []byte(`"b"`)))
}

// ---------------------------------------------------------------------------
// matchPattern edge cases
// ---------------------------------------------------------------------------

func TestMatchPattern(t *testing.T) {
	// Exact match (no [])
	assert.True(t, matchPattern("title", "title"))
	assert.False(t, matchPattern("title", "description"))
	// Pattern with more than one [] => false
	assert.False(t, matchPattern("a[][]b", "a[0][b]"))
	// Valid indexed pattern
	assert.True(t, matchPattern("resource.listView.columns[].title", "resource.listView.columns[0].title"))
	assert.True(t, matchPattern("resource.listView.columns[].title", "resource.listView.columns[99].title"))
	// Non-numeric index
	assert.False(t, matchPattern("resource.listView.columns[].title", "resource.listView.columns[abc].title"))
	// Missing prefix
	assert.False(t, matchPattern("columns[].title", "resource.listView.columns[0].title"))
	// Missing suffix
	assert.False(t, matchPattern("columns[].title", "columns[0].width"))
	// Empty index part (field ends with [] before suffix)
	assert.False(t, matchPattern("columns[].title", "columns[].title"))
}

// ---------------------------------------------------------------------------
// isConflictField with patterns
// ---------------------------------------------------------------------------

func TestIsConflictFieldPatterns(t *testing.T) {
	assert.True(t, isConflictField("bindings[0].id"))
	assert.True(t, isConflictField("bindings[0].functionId"))
	assert.True(t, isConflictField("operation.form.fields[0].key"))
	assert.False(t, isConflictField("unknown.field"))
	// Non-pattern field that's in ConflictFields
	assert.True(t, isConflictField("type"))
	assert.True(t, isConflictField("resourceKey"))
	// Pattern field not in ConflictFields
	assert.False(t, isConflictField("resource.listView.columns[0].title"))
}

// ---------------------------------------------------------------------------
// isAutoMergeField with patterns
// ---------------------------------------------------------------------------

func TestIsAutoMergeFieldPatterns(t *testing.T) {
	assert.True(t, isAutoMergeField("title"))
	assert.True(t, isAutoMergeField("description"))
	assert.True(t, isAutoMergeField("icon"))
	assert.True(t, isAutoMergeField("order"))
	assert.True(t, isAutoMergeField("resource.listView.columns[0].title"))
	assert.True(t, isAutoMergeField("resource.listView.columns[0].width"))
	assert.True(t, isAutoMergeField("resource.detailView.fields[0].title"))
	assert.True(t, isAutoMergeField("operation.form.fields[0].label"))
	assert.True(t, isAutoMergeField("report.charts[0].title"))
	// Not auto-merge
	assert.False(t, isAutoMergeField("type"))
	assert.False(t, isAutoMergeField("bindings"))
}

// ---------------------------------------------------------------------------
// compareField with prefix
// ---------------------------------------------------------------------------

func TestCompareFieldWithPrefix(t *testing.T) {
	base := spec.PageSpec{
		Title:      spec.LocalizedText{"zh-CN": "原始"},
		PageKey:    "test",
		Type:       spec.PageTypeOperation,
		Navigation: &spec.NavigationSpec{Title: spec.LocalizedText{"zh-CN": "导航"}},
	}
	draft := base
	draft.Navigation = &spec.NavigationSpec{Title: spec.LocalizedText{"zh-CN": "新导航"}}
	latest := base
	latest.Navigation = &spec.NavigationSpec{Title: spec.LocalizedText{"zh-CN": "最终导航"}}

	result := ThreeWayMerge(base, draft, latest)

	found := false
	for _, item := range result.AutoMerge {
		if item.Field == "navigation.title" {
			found = true
			break
		}
	}
	assert.True(t, found, "navigation.title should be auto-merged")
}

// ---------------------------------------------------------------------------
// compareOperationFields
// ---------------------------------------------------------------------------

func TestThreeWayMerge_OperationPage(t *testing.T) {
	base := spec.PageSpec{
		PageKey: "op.test",
		Type:    spec.PageTypeOperation,
		Operation: &spec.OperationPageSpec{
			Form: &spec.FormPresentationSpec{
				JSONSchema: spec.JSONSchema(`{"type":"object","properties":{"name":{"type":"string"}}}`),
				Fields: []spec.FormFieldSpec{
					{Key: "name", Label: spec.LocalizedText{"zh-CN": "名称"}, Placeholder: spec.LocalizedText{"zh-CN": "请输入"}},
				},
			},
			Confirm: &spec.ConfirmActionSpec{
				Title:       spec.LocalizedText{"zh-CN": "确认"},
				Description: spec.LocalizedText{"zh-CN": "确认执行"},
				ConfirmText: spec.LocalizedText{"zh-CN": "确定"},
				BindingID:   "main",
			},
		},
		Bindings: []spec.PageFunctionBinding{
			{ID: "main", FunctionID: "op.func1", Usage: spec.BindingUsageAction},
		},
	}

	draft := base
	draft.Operation = &spec.OperationPageSpec{
		Form: &spec.FormPresentationSpec{
			JSONSchema: spec.JSONSchema(`{"type":"object","properties":{"name":{"type":"string"}}}`),
			Fields: []spec.FormFieldSpec{
				{Key: "name", Label: spec.LocalizedText{"zh-CN": "用户标签"}, Placeholder: spec.LocalizedText{"zh-CN": "用户占位"}},
			},
		},
		Confirm: &spec.ConfirmActionSpec{
			Title:       spec.LocalizedText{"zh-CN": "用户确认"},
			Description: spec.LocalizedText{"zh-CN": "用户描述"},
			ConfirmText: spec.LocalizedText{"zh-CN": "用户确定"},
			BindingID:   "main",
		},
	}

	latest := base
	latest.Operation = &spec.OperationPageSpec{
		Form: &spec.FormPresentationSpec{
			JSONSchema: spec.JSONSchema(`{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}}}`),
			Fields: []spec.FormFieldSpec{
				{Key: "name", Label: spec.LocalizedText{"zh-CN": "最新标签"}, Placeholder: spec.LocalizedText{"zh-CN": "最新占位"}},
			},
		},
		Confirm: &spec.ConfirmActionSpec{
			Title:       spec.LocalizedText{"zh-CN": "最新确认"},
			Description: spec.LocalizedText{"zh-CN": "最新描述"},
			ConfirmText: spec.LocalizedText{"zh-CN": "最新确定"},
			BindingID:   "main",
		},
	}

	result := ThreeWayMerge(base, draft, latest)

	// Should have auto-merge items for display fields (label, placeholder, etc.)
	assert.NotEmpty(t, result.AutoMerge, "should have auto-merge items for operation form fields")

	// Should have conflict for jsonSchema and confirm fields
	fieldNames := make(map[string]bool)
	for _, c := range result.Conflicts {
		fieldNames[c.Field] = true
	}
	assert.True(t, fieldNames["operation.form.jsonSchema"], "jsonSchema should be a conflict")
	assert.True(t, fieldNames["operation.confirm"], "confirm should be a conflict")
}

// ---------------------------------------------------------------------------
// compareTaskFields
// ---------------------------------------------------------------------------

func TestThreeWayMerge_TaskPage(t *testing.T) {
	base := spec.PageSpec{
		PageKey: "task.test",
		Type:    spec.PageTypeTask,
		Task: &spec.TaskPageSpec{
			Form: &spec.FormPresentationSpec{
				JSONSchema: spec.JSONSchema(`{"type":"object","properties":{"msg":{"type":"string"}}}`),
				Fields: []spec.FormFieldSpec{
					{Key: "msg", Label: spec.LocalizedText{"zh-CN": "消息"}, Placeholder: spec.LocalizedText{"zh-CN": "输入"}},
				},
			},
			TaskView: &spec.TaskViewSpec{
				TaskIDStateKey:  "taskId",
				StatusBindingID: "status",
				StatusStatePath: "/state",
			},
		},
		Bindings: []spec.PageFunctionBinding{
			{ID: "task", FunctionID: "task.func1", Usage: spec.BindingUsageTask},
			{ID: "status", FunctionID: "task.status1", Usage: spec.BindingUsageTaskStatus},
		},
	}

	draft := base
	draft.Task = &spec.TaskPageSpec{
		Form: &spec.FormPresentationSpec{
			JSONSchema: spec.JSONSchema(`{"type":"object","properties":{"msg":{"type":"string"}}}`),
			Fields: []spec.FormFieldSpec{
				{Key: "msg", Label: spec.LocalizedText{"zh-CN": "用户消息"}, Placeholder: spec.LocalizedText{"zh-CN": "用户输入"}},
			},
		},
		TaskView: &spec.TaskViewSpec{
			TaskIDStateKey:  "taskId",
			StatusBindingID: "status",
			StatusStatePath: "/state",
			ShowTimeline:    true, // differs from base
		},
	}

	latest := base
	latest.Task = &spec.TaskPageSpec{
		Form: &spec.FormPresentationSpec{
			JSONSchema: spec.JSONSchema(`{"type":"object","properties":{"msg":{"type":"string"},"extra":{"type":"string"}}}`),
			Fields: []spec.FormFieldSpec{
				{Key: "msg", Label: spec.LocalizedText{"zh-CN": "最新消息"}, Placeholder: spec.LocalizedText{"zh-CN": "最新输入"}},
			},
		},
		TaskView: &spec.TaskViewSpec{
			TaskIDStateKey:  "taskId",
			StatusBindingID: "status",
			StatusStatePath: "/state",
			ShowProgress:    true, // differs from base and draft
		},
	}

	result := ThreeWayMerge(base, draft, latest)

	assert.NotEmpty(t, result.AutoMerge, "should have auto-merge items for task form fields")
	fieldNames := make(map[string]bool)
	for _, c := range result.Conflicts {
		fieldNames[c.Field] = true
	}
	assert.True(t, fieldNames["task.form.jsonSchema"], "task jsonSchema should be a conflict")
	assert.True(t, fieldNames["task.taskView"], "taskView should be a conflict")
}

// ---------------------------------------------------------------------------
// compareReportFields
// ---------------------------------------------------------------------------

func TestThreeWayMerge_ReportPage(t *testing.T) {
	base := spec.PageSpec{
		PageKey: "report.test",
		Type:    spec.PageTypeReport,
		Report: &spec.ReportPageSpec{
			QueryForm: &spec.FormPresentationSpec{
				JSONSchema: spec.JSONSchema(`{"type":"object","properties":{"date":{"type":"string"}}}`),
				Fields: []spec.FormFieldSpec{
					{Key: "date", Label: spec.LocalizedText{"zh-CN": "日期"}, Placeholder: spec.LocalizedText{"zh-CN": "选择日期"}},
				},
			},
			Charts: []spec.ChartSpec{
				{Type: "bar", Title: spec.LocalizedText{"zh-CN": "原始图表"}},
			},
			Dataset: &spec.DatasetSpec{
				Dimensions: []spec.DimensionSpec{{Key: "region", Title: spec.LocalizedText{"zh-CN": "地区"}, DataType: "string"}},
				Metrics:    []spec.MetricSpec{{Key: "count", Title: spec.LocalizedText{"zh-CN": "数量"}, DataType: "number"}},
			},
		},
		Bindings: []spec.PageFunctionBinding{
			{ID: "query", FunctionID: "report.func1", Usage: spec.BindingUsageReport},
		},
	}

	draft := base
	draft.Report = &spec.ReportPageSpec{
		QueryForm: &spec.FormPresentationSpec{
			JSONSchema: spec.JSONSchema(`{"type":"object","properties":{"date":{"type":"string"}}}`),
			Fields: []spec.FormFieldSpec{
				{Key: "date", Label: spec.LocalizedText{"zh-CN": "用户日期"}, Placeholder: spec.LocalizedText{"zh-CN": "用户选择"}},
			},
		},
		Charts: []spec.ChartSpec{
			{Type: "bar", Title: spec.LocalizedText{"zh-CN": "用户图表"}},
		},
		Dataset: &spec.DatasetSpec{
			Dimensions: []spec.DimensionSpec{{Key: "region", Title: spec.LocalizedText{"zh-CN": "地区"}, DataType: "string"}, {Key: "date", Title: spec.LocalizedText{"zh-CN": "日期"}, DataType: "date"}},
			Metrics:    []spec.MetricSpec{{Key: "count", Title: spec.LocalizedText{"zh-CN": "数量"}, DataType: "number"}},
		},
	}

	latest := base
	latest.Report = &spec.ReportPageSpec{
		QueryForm: &spec.FormPresentationSpec{
			JSONSchema: spec.JSONSchema(`{"type":"object","properties":{"date":{"type":"string"},"extra":{"type":"string"}}}`),
			Fields: []spec.FormFieldSpec{
				{Key: "date", Label: spec.LocalizedText{"zh-CN": "最新日期"}, Placeholder: spec.LocalizedText{"zh-CN": "最新选择"}},
			},
		},
		Charts: []spec.ChartSpec{
			{Type: "bar", Title: spec.LocalizedText{"zh-CN": "最新图表"}},
		},
		Dataset: &spec.DatasetSpec{
			Dimensions: []spec.DimensionSpec{{Key: "region", Title: spec.LocalizedText{"zh-CN": "地区"}, DataType: "string"}},
			Metrics:    []spec.MetricSpec{{Key: "count", Title: spec.LocalizedText{"zh-CN": "数量"}, DataType: "number"}, {Key: "total", Title: spec.LocalizedText{"zh-CN": "总量"}, DataType: "number"}},
		},
	}

	result := ThreeWayMerge(base, draft, latest)

	assert.NotEmpty(t, result.AutoMerge, "should have auto-merge items for report fields")
	fieldNames := make(map[string]bool)
	for _, c := range result.Conflicts {
		fieldNames[c.Field] = true
	}
	assert.True(t, fieldNames["report.queryForm.jsonSchema"], "report jsonSchema should be a conflict")
	// report.dataset is a conflict field when values differ
	assert.True(t, fieldNames["report.dataset"], "report dataset should be a conflict")
}

// ---------------------------------------------------------------------------
// compareResourceFields - DetailView and extra ListView fields
// ---------------------------------------------------------------------------

func TestThreeWayMerge_ResourceDetailView(t *testing.T) {
	base := spec.PageSpec{
		PageKey: "res.test",
		Type:    spec.PageTypeResource,
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{
				Columns: []spec.ColumnSpec{
					{Key: "name", Title: spec.LocalizedText{"zh-CN": "名称"}, Width: 100},
				},
				DefaultSort: &spec.SortSpec{Field: "name", Order: "asc"},
				Filters:     []spec.FilterSpec{{Key: "status", Title: spec.LocalizedText{"zh-CN": "状态"}, Type: "select"}},
				Pagination:  &spec.PaginationSpec{Enabled: true, DefaultSize: 20},
			},
			DetailView: &spec.DetailViewSpec{
				Fields: []spec.DetailFieldSpec{
					{Key: "name", Title: spec.LocalizedText{"zh-CN": "名称"}, DataType: "string", Span: 6},
				},
			},
		},
		Bindings: []spec.PageFunctionBinding{
			{ID: "list", FunctionID: "res.list", Usage: spec.BindingUsageQuery},
			{ID: "detail", FunctionID: "res.detail", Usage: spec.BindingUsageDetail},
		},
	}

	draft := base
	draft.Resource = &spec.ResourcePageSpec{
		ListView: &spec.ListViewSpec{
			Columns: []spec.ColumnSpec{
				{Key: "name", Title: spec.LocalizedText{"zh-CN": "用户名称"}, Width: 150},
			},
			DefaultSort: &spec.SortSpec{Field: "createdAt", Order: "desc"},
			Filters:     []spec.FilterSpec{{Key: "status", Title: spec.LocalizedText{"zh-CN": "状态"}, Type: "select"}},
			Pagination:  &spec.PaginationSpec{Enabled: true, DefaultSize: 20},
		},
		DetailView: &spec.DetailViewSpec{
			Fields: []spec.DetailFieldSpec{
				{Key: "name", Title: spec.LocalizedText{"zh-CN": "用户名称"}, DataType: "string", Span: 12},
			},
		},
	}

	latest := base
	latest.Resource = &spec.ResourcePageSpec{
		ListView: &spec.ListViewSpec{
			Columns: []spec.ColumnSpec{
				{Key: "name", Title: spec.LocalizedText{"zh-CN": "最新名称"}, Width: 200},
			},
			DefaultSort: &spec.SortSpec{Field: "id", Order: "asc"},
			Filters:     []spec.FilterSpec{{Key: "status", Title: spec.LocalizedText{"zh-CN": "状态"}, Type: "select"}},
			Pagination:  &spec.PaginationSpec{Enabled: true, DefaultSize: 20},
		},
		DetailView: &spec.DetailViewSpec{
			Fields: []spec.DetailFieldSpec{
				{Key: "name", Title: spec.LocalizedText{"zh-CN": "最新名称"}, DataType: "string", Span: 8},
			},
		},
	}

	result := ThreeWayMerge(base, draft, latest)

	autoMergeFields := make(map[string]bool)
	for _, item := range result.AutoMerge {
		autoMergeFields[item.Field] = true
	}
	assert.True(t, autoMergeFields["resource.listView.columns[0].title"], "column title should be auto-merged")
	assert.True(t, autoMergeFields["resource.listView.columns[0].width"], "column width should be auto-merged")
	assert.True(t, autoMergeFields["resource.listView.defaultSort"], "defaultSort should be auto-merged")
	// Detail view fields - all three differ: base=6, draft=12, latest=8
	assert.True(t, autoMergeFields["resource.detailView.fields[0].title"], "detail title should be auto-merged")
	assert.True(t, autoMergeFields["resource.detailView.fields[0].span"], "detail span should be auto-merged")
}

// ---------------------------------------------------------------------------
// compareColumns - different length slices
// ---------------------------------------------------------------------------

func TestThreeWayMerge_ColumnsDifferentLengths(t *testing.T) {
	base := spec.PageSpec{
		PageKey: "res.test",
		Type:    spec.PageTypeResource,
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{
				Columns: []spec.ColumnSpec{
					{Key: "a", Title: spec.LocalizedText{"zh-CN": "A"}},
				},
			},
		},
		Bindings: []spec.PageFunctionBinding{
			{ID: "list", FunctionID: "res.list", Usage: spec.BindingUsageQuery},
		},
	}

	draft := base
	draft.Resource = &spec.ResourcePageSpec{
		ListView: &spec.ListViewSpec{
			Columns: []spec.ColumnSpec{
				{Key: "a", Title: spec.LocalizedText{"zh-CN": "A"}},
				{Key: "b", Title: spec.LocalizedText{"zh-CN": "B"}},
			},
		},
	}

	latest := base
	latest.Resource = &spec.ResourcePageSpec{
		ListView: &spec.ListViewSpec{
			Columns: []spec.ColumnSpec{
				{Key: "a", Title: spec.LocalizedText{"zh-CN": "A"}},
				{Key: "b", Title: spec.LocalizedText{"zh-CN": "B"}},
				{Key: "c", Title: spec.LocalizedText{"zh-CN": "C"}},
			},
		},
	}

	result := ThreeWayMerge(base, draft, latest)
	// Column[0] is same in all three => no change
	// Column[1] was added in both draft and latest with same value => no conflict
	// Column[2] was added only in latest (base=nil, draft=nil) => allEqual returns true for nil-nil
	// So no conflicts
	assert.False(t, result.HasConflicts, "same additions in both should not conflict")
}

// ---------------------------------------------------------------------------
// compareFormFields - different length slices
// ---------------------------------------------------------------------------

func TestThreeWayMerge_FormFieldsDifferentLengths(t *testing.T) {
	base := spec.PageSpec{
		PageKey: "op.test",
		Type:    spec.PageTypeOperation,
		Operation: &spec.OperationPageSpec{
			Form: &spec.FormPresentationSpec{
				JSONSchema: spec.JSONSchema(`{"type":"object","properties":{"a":{"type":"string"}}}`),
				Fields: []spec.FormFieldSpec{
					{Key: "a", Label: spec.LocalizedText{"zh-CN": "A"}},
				},
			},
		},
		Bindings: []spec.PageFunctionBinding{
			{ID: "main", FunctionID: "op.func1", Usage: spec.BindingUsageAction},
		},
	}

	draft := base
	draft.Operation = &spec.OperationPageSpec{
		Form: &spec.FormPresentationSpec{
			JSONSchema: spec.JSONSchema(`{"type":"object","properties":{"a":{"type":"string"}}}`),
			Fields: []spec.FormFieldSpec{
				{Key: "a", Label: spec.LocalizedText{"zh-CN": "A"}},
				{Key: "b", Label: spec.LocalizedText{"zh-CN": "B"}},
			},
		},
	}

	latest := base
	latest.Operation = &spec.OperationPageSpec{
		Form: &spec.FormPresentationSpec{
			JSONSchema: spec.JSONSchema(`{"type":"object","properties":{"a":{"type":"string"}}}`),
			Fields: []spec.FormFieldSpec{
				{Key: "a", Label: spec.LocalizedText{"zh-CN": "A"}},
				{Key: "b", Label: spec.LocalizedText{"zh-CN": "B"}},
				{Key: "c", Label: spec.LocalizedText{"zh-CN": "C"}},
			},
		},
	}

	result := ThreeWayMerge(base, draft, latest)
	assert.False(t, result.HasConflicts, "should not conflict for same additions")
}

// ---------------------------------------------------------------------------
// compareCharts - different length slices
// ---------------------------------------------------------------------------

func TestThreeWayMerge_ChartsDifferentLengths(t *testing.T) {
	base := spec.PageSpec{
		PageKey: "report.test",
		Type:    spec.PageTypeReport,
		Report: &spec.ReportPageSpec{
			Charts: []spec.ChartSpec{
				{Type: "bar", Title: spec.LocalizedText{"zh-CN": "原始"}},
			},
			Dataset: &spec.DatasetSpec{
				Dimensions: []spec.DimensionSpec{{Key: "k", Title: spec.LocalizedText{"zh-CN": "K"}, DataType: "string"}},
				Metrics:    []spec.MetricSpec{{Key: "m", Title: spec.LocalizedText{"zh-CN": "M"}, DataType: "number"}},
			},
		},
		Bindings: []spec.PageFunctionBinding{
			{ID: "q", FunctionID: "r.f1", Usage: spec.BindingUsageReport},
		},
	}

	draft := base
	draft.Report = &spec.ReportPageSpec{
		Charts: []spec.ChartSpec{
			{Type: "bar", Title: spec.LocalizedText{"zh-CN": "原始"}},
			{Type: "line", Title: spec.LocalizedText{"zh-CN": "新增图表"}},
		},
		Dataset: &spec.DatasetSpec{
			Dimensions: []spec.DimensionSpec{{Key: "k", Title: spec.LocalizedText{"zh-CN": "K"}, DataType: "string"}},
			Metrics:    []spec.MetricSpec{{Key: "m", Title: spec.LocalizedText{"zh-CN": "M"}, DataType: "number"}},
		},
	}

	latest := base
	latest.Report = &spec.ReportPageSpec{
		Charts: []spec.ChartSpec{
			{Type: "bar", Title: spec.LocalizedText{"zh-CN": "最新"}},
		},
		Dataset: &spec.DatasetSpec{
			Dimensions: []spec.DimensionSpec{{Key: "k", Title: spec.LocalizedText{"zh-CN": "K"}, DataType: "string"}},
			Metrics:    []spec.MetricSpec{{Key: "m", Title: spec.LocalizedText{"zh-CN": "M"}, DataType: "number"}},
		},
	}

	result := ThreeWayMerge(base, draft, latest)

	autoMergeFields := make(map[string]bool)
	for _, item := range result.AutoMerge {
		autoMergeFields[item.Field] = true
	}
	assert.True(t, autoMergeFields["report.charts[0].title"], "chart title should be auto-merged")
}

// ---------------------------------------------------------------------------
// compareField - draft == base (use latest)
// ---------------------------------------------------------------------------

func TestCompareField_DraftUnchangedUseLatest(t *testing.T) {
	base := spec.PageSpec{
		PageKey: "test",
		Type:    spec.PageTypeOperation,
		Title:   spec.LocalizedText{"zh-CN": "原始"},
	}
	draft := base // draft unchanged
	latest := base
	latest.Title = spec.LocalizedText{"zh-CN": "生成器新标题"}

	result := ThreeWayMerge(base, draft, latest)

	assert.Len(t, result.AutoMerge, 1)
	assert.Equal(t, "title", result.AutoMerge[0].Field)
	// Since draft == base, the merged value should use latest
	assert.Contains(t, string(result.AutoMerge[0].MergedValue), "生成器新标题")
}

// ---------------------------------------------------------------------------
// toJSON edge cases
// ---------------------------------------------------------------------------

func TestToJSON(t *testing.T) {
	result := toJSON[any](nil)
	assert.Equal(t, json.RawMessage("null"), result)

	result = toJSON("hello")
	assert.Equal(t, json.RawMessage(`"hello"`), result)

	result = toJSON(42)
	assert.Equal(t, json.RawMessage("42"), result)

	result = toJSON(true)
	assert.Equal(t, json.RawMessage("true"), result)
}

// ---------------------------------------------------------------------------
// Navigation nil branches
// ---------------------------------------------------------------------------

func TestThreeWayMerge_NavigationNil(t *testing.T) {
	base := spec.PageSpec{
		PageKey:    "test",
		Type:       spec.PageTypeOperation,
		Navigation: nil,
	}
	draft := base
	latest := base

	result := ThreeWayMerge(base, draft, latest)
	assert.False(t, result.HasConflicts)
	assert.Empty(t, result.AutoMerge)
}

// ---------------------------------------------------------------------------
// Resource/Operation/Task/Report nil branches
// ---------------------------------------------------------------------------

func TestThreeWayMerge_SpecSectionsNil(t *testing.T) {
	tests := []struct {
		name string
		page spec.PageSpec
	}{
		{"resource nil", spec.PageSpec{PageKey: "test", Type: spec.PageTypeOperation, Resource: nil}},
		{"operation nil", spec.PageSpec{PageKey: "test", Type: spec.PageTypeOperation, Operation: nil}},
		{"task nil", spec.PageSpec{PageKey: "test", Type: spec.PageTypeTask, Task: nil}},
		{"report nil", spec.PageSpec{PageKey: "test", Type: spec.PageTypeReport, Report: nil}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ThreeWayMerge(tt.page, tt.page, tt.page)
			assert.False(t, result.HasConflicts)
			assert.Empty(t, result.AutoMerge)
		})
	}
}

// ---------------------------------------------------------------------------
// Category fields
// ---------------------------------------------------------------------------

func TestThreeWayMerge_CategoryFields(t *testing.T) {
	base := spec.PageSpec{
		PageKey:  "test",
		Type:     spec.PageTypeOperation,
		Category: spec.PageCategorySpec{Key: "admin", Labels: spec.LocalizedText{"zh-CN": "管理"}, Order: 1},
	}
	draft := base
	draft.Category = spec.PageCategorySpec{Key: "user", Labels: spec.LocalizedText{"zh-CN": "用户管理"}, Order: 1}
	latest := base
	latest.Category = spec.PageCategorySpec{Key: "system", Labels: spec.LocalizedText{"zh-CN": "最新管理"}, Order: 2}

	result := ThreeWayMerge(base, draft, latest)

	autoMergeFields := make(map[string]bool)
	for _, item := range result.AutoMerge {
		autoMergeFields[item.Field] = true
	}
	assert.True(t, autoMergeFields["category.labels"], "category.labels should be auto-merged")
	assert.True(t, autoMergeFields["category.order"], "category.order should be auto-merged")

	conflictFields := make(map[string]bool)
	for _, c := range result.Conflicts {
		conflictFields[c.Field] = true
	}
	assert.True(t, conflictFields["category.key"], "category.key should be a conflict")
}

// ---------------------------------------------------------------------------
// Default conflict for completely unknown fields
// ---------------------------------------------------------------------------

func TestCompareField_DefaultConflictUnknownField(t *testing.T) {
	base := json.RawMessage(`"base"`)
	draft := json.RawMessage(`"draft"`)
	latest := json.RawMessage(`"latest"`)

	result := MergeResult{
		AutoMerge: make([]MergeItem, 0),
		Conflicts: make([]MergeConflict, 0),
	}

	compareField("custom", "unknown", base, draft, latest, &result)

	assert.Len(t, result.AutoMerge, 0)
	assert.Len(t, result.Conflicts, 1)
	assert.Equal(t, "custom.unknown", result.Conflicts[0].Field)
	assert.Contains(t, result.Conflicts[0].Reason, "field changed in both")
}

// ---------------------------------------------------------------------------
// Resource ListView with RowActions, BatchActions, ToolbarActions
// ---------------------------------------------------------------------------

func TestThreeWayMerge_ResourceActions(t *testing.T) {
	base := spec.PageSpec{
		PageKey: "res.test",
		Type:    spec.PageTypeResource,
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{
				Columns: []spec.ColumnSpec{
					{Key: "name", Title: spec.LocalizedText{"zh-CN": "名称"}},
				},
				RowActions:     []spec.ActionSpec{{Key: "edit", Title: spec.LocalizedText{"zh-CN": "编辑"}}},
				BatchActions:   []spec.ActionSpec{{Key: "batch_delete", Title: spec.LocalizedText{"zh-CN": "批量删除"}}},
				ToolbarActions: []spec.ActionSpec{{Key: "export", Title: spec.LocalizedText{"zh-CN": "导出"}}},
			},
		},
		Bindings: []spec.PageFunctionBinding{
			{ID: "list", FunctionID: "res.list", Usage: spec.BindingUsageQuery},
		},
	}

	draft := base
	draft.Resource = &spec.ResourcePageSpec{
		ListView: &spec.ListViewSpec{
			Columns: []spec.ColumnSpec{
				{Key: "name", Title: spec.LocalizedText{"zh-CN": "名称"}},
			},
			RowActions:     []spec.ActionSpec{{Key: "edit", Title: spec.LocalizedText{"zh-CN": "编辑"}}},
			BatchActions:   []spec.ActionSpec{{Key: "batch_delete", Title: spec.LocalizedText{"zh-CN": "批量删除"}}},
			ToolbarActions: []spec.ActionSpec{{Key: "export", Title: spec.LocalizedText{"zh-CN": "导出"}}},
		},
	}

	latest := base
	latest.Resource = &spec.ResourcePageSpec{
		ListView: &spec.ListViewSpec{
			Columns: []spec.ColumnSpec{
				{Key: "name", Title: spec.LocalizedText{"zh-CN": "名称"}},
			},
			RowActions:     []spec.ActionSpec{{Key: "edit", Title: spec.LocalizedText{"zh-CN": "编辑"}, Confirm: true}},
			BatchActions:   []spec.ActionSpec{{Key: "batch_delete", Title: spec.LocalizedText{"zh-CN": "批量删除"}}},
			ToolbarActions: []spec.ActionSpec{{Key: "export", Title: spec.LocalizedText{"zh-CN": "导出"}}},
		},
	}

	result := ThreeWayMerge(base, draft, latest)

	// rowActions, batchActions, toolbarActions are conflict fields
	conflictFields := make(map[string]bool)
	for _, c := range result.Conflicts {
		conflictFields[c.Field] = true
	}
	assert.True(t, conflictFields["resource.listView.rowActions"], "rowActions should be a conflict")
	// batchActions and toolbarActions are same in draft and latest
	assert.False(t, conflictFields["resource.listView.batchActions"], "batchActions same in draft/latest should not conflict")
	assert.False(t, conflictFields["resource.listView.toolbarActions"], "toolbarActions same in draft/latest should not conflict")
}

// ---------------------------------------------------------------------------
// DetailView nil in one spec
// ---------------------------------------------------------------------------

func TestThreeWayMerge_DetailViewNilInBase(t *testing.T) {
	base := spec.PageSpec{
		PageKey: "res.test",
		Type:    spec.PageTypeResource,
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{
				Columns: []spec.ColumnSpec{{Key: "name", Title: spec.LocalizedText{"zh-CN": "名称"}}},
			},
			DetailView: nil, // nil in base
		},
		Bindings: []spec.PageFunctionBinding{
			{ID: "list", FunctionID: "res.list", Usage: spec.BindingUsageQuery},
		},
	}

	draft := base
	draft.Resource = &spec.ResourcePageSpec{
		ListView: &spec.ListViewSpec{
			Columns: []spec.ColumnSpec{{Key: "name", Title: spec.LocalizedText{"zh-CN": "名称"}}},
		},
		DetailView: &spec.DetailViewSpec{
			Fields: []spec.DetailFieldSpec{{Key: "name", Title: spec.LocalizedText{"zh-CN": "名称"}}},
		},
	}

	latest := base
	latest.Resource = &spec.ResourcePageSpec{
		ListView: &spec.ListViewSpec{
			Columns: []spec.ColumnSpec{{Key: "name", Title: spec.LocalizedText{"zh-CN": "名称"}}},
		},
		DetailView: &spec.DetailViewSpec{
			Fields: []spec.DetailFieldSpec{{Key: "name", Title: spec.LocalizedText{"zh-CN": "名称"}}},
		},
	}

	// Since base.DetailView is nil, compareResourceFields won't compare detail fields
	result := ThreeWayMerge(base, draft, latest)
	assert.False(t, result.HasConflicts)
}

// ---------------------------------------------------------------------------
// Operation ResultView field (non-auto, non-conflict, default conflict)
// ---------------------------------------------------------------------------

func TestThreeWayMerge_OperationResultView(t *testing.T) {
	base := spec.PageSpec{
		PageKey: "op.test",
		Type:    spec.PageTypeOperation,
		Operation: &spec.OperationPageSpec{
			Form: &spec.FormPresentationSpec{
				JSONSchema: spec.JSONSchema(`{"type":"object","properties":{}}`),
			},
			ResultView: &spec.ResultViewSpec{
				SuccessMessage: spec.LocalizedText{"zh-CN": "成功"},
			},
		},
		Bindings: []spec.PageFunctionBinding{
			{ID: "main", FunctionID: "op.func1", Usage: spec.BindingUsageAction},
		},
	}

	draft := base
	draft.Operation = &spec.OperationPageSpec{
		Form: &spec.FormPresentationSpec{
			JSONSchema: spec.JSONSchema(`{"type":"object","properties":{}}`),
		},
		ResultView: &spec.ResultViewSpec{
			SuccessMessage: spec.LocalizedText{"zh-CN": "用户成功"},
		},
	}

	latest := base
	latest.Operation = &spec.OperationPageSpec{
		Form: &spec.FormPresentationSpec{
			JSONSchema: spec.JSONSchema(`{"type":"object","properties":{}}`),
		},
		ResultView: &spec.ResultViewSpec{
			SuccessMessage: spec.LocalizedText{"zh-CN": "最新成功"},
		},
	}

	result := ThreeWayMerge(base, draft, latest)

	// resultView is not in AutoMergeFields or ConflictFields, so it's default conflict
	fieldNames := make(map[string]bool)
	for _, c := range result.Conflicts {
		fieldNames[c.Field] = true
	}
	assert.True(t, fieldNames["operation.resultView"], "resultView should be a default conflict")
}

// ---------------------------------------------------------------------------
// Report table field (non-auto, non-conflict, default conflict)
// ---------------------------------------------------------------------------

func TestThreeWayMerge_ReportTable(t *testing.T) {
	tableCols := []spec.ColumnSpec{{Key: "name", Title: spec.LocalizedText{"zh-CN": "名称"}}}

	base := spec.PageSpec{
		PageKey: "report.test",
		Type:    spec.PageTypeReport,
		Report: &spec.ReportPageSpec{
			Dataset: &spec.DatasetSpec{
				Dimensions: []spec.DimensionSpec{{Key: "k", Title: spec.LocalizedText{"zh-CN": "K"}, DataType: "string"}},
				Metrics:    []spec.MetricSpec{{Key: "m", Title: spec.LocalizedText{"zh-CN": "M"}, DataType: "number"}},
			},
			Table: &spec.ListViewSpec{Columns: tableCols},
		},
		Bindings: []spec.PageFunctionBinding{
			{ID: "q", FunctionID: "r.f1", Usage: spec.BindingUsageReport},
		},
	}

	draft := base
	draft.Report = &spec.ReportPageSpec{
		Dataset: &spec.DatasetSpec{
			Dimensions: []spec.DimensionSpec{{Key: "k", Title: spec.LocalizedText{"zh-CN": "K"}, DataType: "string"}},
			Metrics:    []spec.MetricSpec{{Key: "m", Title: spec.LocalizedText{"zh-CN": "M"}, DataType: "number"}},
		},
		Table: &spec.ListViewSpec{Columns: tableCols},
	}

	latest := base
	latest.Report = &spec.ReportPageSpec{
		Dataset: &spec.DatasetSpec{
			Dimensions: []spec.DimensionSpec{{Key: "k", Title: spec.LocalizedText{"zh-CN": "K"}, DataType: "string"}},
			Metrics:    []spec.MetricSpec{{Key: "m", Title: spec.LocalizedText{"zh-CN": "M"}, DataType: "number"}},
		},
		Table: &spec.ListViewSpec{Columns: []spec.ColumnSpec{{Key: "id", Title: spec.LocalizedText{"zh-CN": "ID"}}}},
	}

	result := ThreeWayMerge(base, draft, latest)

	// report.table is not in AutoMergeFields or ConflictFields, so it's default conflict
	fieldNames := make(map[string]bool)
	for _, c := range result.Conflicts {
		fieldNames[c.Field] = true
	}
	assert.True(t, fieldNames["report.table"], "report.table should be a default conflict")
}
