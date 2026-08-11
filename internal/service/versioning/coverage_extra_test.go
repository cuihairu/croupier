package versioning

import (
	"context"
	"encoding/json"
	"testing"

	dashboardmerge "github.com/cuihairu/croupier/internal/dashboard/merge"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// parseIndexedMergeField
// ---------------------------------------------------------------------------

func TestParseIndexedMergeField(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		prefix    string
		wantOK    bool
		wantIndex int
		wantLeaf  string
	}{
		{"matching", "resource.listView.columns[0].title", "resource.listView.columns", true, 0, "title"},
		{"index 2", "resource.listView.columns[2].width", "resource.listView.columns", true, 2, "width"},
		{"no match", "other.field[0].title", "resource.listView.columns", false, 0, ""},
		{"no closing bracket", "resource.listView.columns[0.title", "resource.listView.columns", true, -1, ""},
		{"no dot after bracket", "resource.listView.columns[0]title", "resource.listView.columns", true, -1, ""},
		{"invalid index", "resource.listView.columns[abc].title", "resource.listView.columns", true, -1, ""},
		{"empty field", "", "resource.listView.columns", false, 0, ""},
		{"operation form", "operation.form.fields[1].label", "operation.form.fields", true, 1, "label"},
		{"task form", "task.form.fields[0].placeholder", "task.form.fields", true, 0, "placeholder"},
		{"report query", "report.queryForm.fields[3].order", "report.queryForm.fields", true, 3, "order"},
		{"report charts", "report.charts[0].title", "report.charts", true, 0, "title"},
		{"detail view", "resource.detailView.fields[0].title", "resource.detailView.fields", true, 0, "title"},
		{"trailing bracket", "resource.listView.columns[", "resource.listView.columns", true, -1, ""},
		{"trailing bracket dot", "resource.listView.columns[].", "resource.listView.columns", true, -1, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, index, leaf := parseIndexedMergeField(tt.field, tt.prefix)
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.Equal(t, tt.wantIndex, index)
				assert.Equal(t, tt.wantLeaf, leaf)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// diffSummary
// ---------------------------------------------------------------------------

func TestDiffSummaryV2(t *testing.T) {
	assert.Equal(t, "found no changes", diffSummary(0, 0, 0))
	assert.Contains(t, diffSummary(3, 1, 2), "3 changes")
}

// ---------------------------------------------------------------------------
// manualMergeMessage
// ---------------------------------------------------------------------------

func TestManualMergeMessageV2(t *testing.T) {
	assert.Equal(t, "accepted latest proposal snapshot", manualMergeMessage(0, 0))
	assert.Contains(t, manualMergeMessage(0, 2), "resolved 2 conflicts")
	assert.Contains(t, manualMergeMessage(1, 1), "auto-merged 1")
}

// ---------------------------------------------------------------------------
// mergePreviewMessage
// ---------------------------------------------------------------------------

func TestMergePreviewMessageV2(t *testing.T) {
	assert.Equal(t, "no contract changes require merge", mergePreviewMessage(0, 0, false))
	assert.Equal(t, "latest proposal snapshot can be accepted without page content changes", mergePreviewMessage(0, 0, true))
	assert.Contains(t, mergePreviewMessage(1, 0, false), "1 safe changes")
	assert.Contains(t, mergePreviewMessage(1, 1, false), "1 conflicts")
}

// ---------------------------------------------------------------------------
// cloneRawJSON
// ---------------------------------------------------------------------------

func TestCloneRawJSONV2(t *testing.T) {
	assert.Nil(t, cloneRawJSON(nil))
	assert.Nil(t, cloneRawJSON([]byte{}))
	original := json.RawMessage(`{"key":"value"}`)
	cloned := cloneRawJSON(original)
	assert.JSONEq(t, string(original), string(cloned))
	original[1] = 'x'
	assert.NotEqual(t, string(original), string(cloned))
}

// ---------------------------------------------------------------------------
// cloneJSONSchema
// ---------------------------------------------------------------------------

func TestCloneJSONSchemaV2(t *testing.T) {
	assert.Nil(t, cloneJSONSchema(nil))
	assert.Nil(t, cloneJSONSchema([]byte{}))
	schema := json.RawMessage(`{"type":"object"}`)
	cloned := cloneJSONSchema(schema)
	assert.Equal(t, spec.JSONSchema(schema), cloned)
}

// ---------------------------------------------------------------------------
// decodeRawJSONField
// ---------------------------------------------------------------------------

func TestDecodeRawJSONFieldV2(t *testing.T) {
	var s string
	err := decodeRawJSONField(json.RawMessage(`"hello"`), "test", &s)
	assert.NoError(t, err)
	assert.Equal(t, "hello", s)

	err = decodeRawJSONField(nil, "test", &s)
	assert.NoError(t, err)

	err = decodeRawJSONField(json.RawMessage(`not json`), "test", &s)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// mustPageSpecFromModel
// ---------------------------------------------------------------------------

func TestMustPageSpecFromModelV2(t *testing.T) {
	page := &model.PageSpec{
		SpecJSON: `{"pageKey":"test","type":"resource"}`,
	}
	result := mustPageSpecFromModel(page)
	assert.Equal(t, "test", result.PageKey)
	assert.Equal(t, spec.PageTypeResource, result.Type)

	invalidPage := &model.PageSpec{SpecJSON: `invalid`}
	result2 := mustPageSpecFromModel(invalidPage)
	assert.Equal(t, spec.PageSpec{}, result2)

	emptyPage := &model.PageSpec{}
	result3 := mustPageSpecFromModel(emptyPage)
	assert.Equal(t, spec.PageSpec{}, result3)
}

// ---------------------------------------------------------------------------
// ensure* helpers
// ---------------------------------------------------------------------------

func TestEnsureResourcePageV2(t *testing.T) {
	page := &spec.PageSpec{}
	resource := ensureResourcePage(page)
	assert.NotNil(t, resource)
	assert.NotNil(t, page.Resource)
	// ResourcePageSpec doesn't have Title, test with ListView
	resource.ListView = &spec.ListViewSpec{IdentityKey: "id"}
	resource2 := ensureResourcePage(page)
	assert.Equal(t, "id", resource2.ListView.IdentityKey)
}

func TestEnsureResourceListViewV2(t *testing.T) {
	page := &spec.PageSpec{}
	lv := ensureResourceListView(page)
	assert.NotNil(t, lv)
	assert.NotNil(t, page.Resource.ListView)
}

func TestEnsureResourceDetailViewV2(t *testing.T) {
	page := &spec.PageSpec{}
	dv := ensureResourceDetailView(page)
	assert.NotNil(t, dv)
	assert.NotNil(t, page.Resource.DetailView)
}

func TestEnsureOperationPageV2(t *testing.T) {
	page := &spec.PageSpec{}
	op := ensureOperationPage(page)
	assert.NotNil(t, op)
	assert.NotNil(t, page.Operation)
}

func TestEnsureOperationFormV2(t *testing.T) {
	page := &spec.PageSpec{}
	form := ensureOperationForm(page)
	assert.NotNil(t, form)
	assert.NotNil(t, page.Operation.Form)
}

func TestEnsureTaskPageV2(t *testing.T) {
	page := &spec.PageSpec{}
	task := ensureTaskPage(page)
	assert.NotNil(t, task)
	assert.NotNil(t, page.Task)
}

func TestEnsureTaskFormV2(t *testing.T) {
	page := &spec.PageSpec{}
	form := ensureTaskForm(page)
	assert.NotNil(t, form)
	assert.NotNil(t, page.Task.Form)
}

func TestEnsureReportPageV2(t *testing.T) {
	page := &spec.PageSpec{}
	report := ensureReportPage(page)
	assert.NotNil(t, report)
	assert.NotNil(t, page.Report)
}

func TestEnsureReportQueryFormV2(t *testing.T) {
	page := &spec.PageSpec{}
	form := ensureReportQueryForm(page)
	assert.NotNil(t, form)
	assert.NotNil(t, page.Report.QueryForm)
}

// ---------------------------------------------------------------------------
// applyAutoMergeItem - various fields
// ---------------------------------------------------------------------------

func TestApplyAutoMergeItemTitleV2(t *testing.T) {
	page := spec.PageSpec{}
	item := dashboardmerge.MergeItem{
		Field:       "title",
		MergedValue: json.RawMessage(`{"zh-CN":"新标题"}`),
	}
	err := applyAutoMergeItem(&page, item)
	require.NoError(t, err)
	assert.Equal(t, "新标题", page.Title["zh-CN"])
}

func TestApplyAutoMergeItemDescriptionV2(t *testing.T) {
	page := spec.PageSpec{}
	item := dashboardmerge.MergeItem{
		Field:       "description",
		MergedValue: json.RawMessage(`{"zh-CN":"新描述"}`),
	}
	err := applyAutoMergeItem(&page, item)
	require.NoError(t, err)
	assert.Equal(t, "新描述", page.Description["zh-CN"])
}

func TestApplyAutoMergeItemIconV2(t *testing.T) {
	page := spec.PageSpec{}
	item := dashboardmerge.MergeItem{
		Field:       "icon",
		MergedValue: json.RawMessage(`"icon-name"`),
	}
	err := applyAutoMergeItem(&page, item)
	require.NoError(t, err)
	assert.Equal(t, "icon-name", page.Icon)
}

func TestApplyAutoMergeItemOrderV2(t *testing.T) {
	page := spec.PageSpec{}
	item := dashboardmerge.MergeItem{
		Field:       "order",
		MergedValue: json.RawMessage(`10`),
	}
	err := applyAutoMergeItem(&page, item)
	require.NoError(t, err)
	assert.Equal(t, 10, page.Order)
}

func TestApplyAutoMergeItemCategoryLabelsV2(t *testing.T) {
	page := spec.PageSpec{}
	item := dashboardmerge.MergeItem{
		Field:       "category.labels",
		MergedValue: json.RawMessage(`{"zh-CN":"玩家"}`),
	}
	err := applyAutoMergeItem(&page, item)
	require.NoError(t, err)
	assert.Equal(t, "玩家", page.Category.Labels["zh-CN"])
}

func TestApplyAutoMergeItemCategoryOrderV2(t *testing.T) {
	page := spec.PageSpec{}
	item := dashboardmerge.MergeItem{
		Field:       "category.order",
		MergedValue: json.RawMessage(`5`),
	}
	err := applyAutoMergeItem(&page, item)
	require.NoError(t, err)
	assert.Equal(t, 5, page.Category.Order)
}

func TestApplyAutoMergeItemNavigationTitleV2(t *testing.T) {
	page := spec.PageSpec{}
	item := dashboardmerge.MergeItem{
		Field:       "navigation.title",
		MergedValue: json.RawMessage(`{"zh-CN":"导航"}`),
	}
	err := applyAutoMergeItem(&page, item)
	require.NoError(t, err)
	assert.NotNil(t, page.Navigation)
	assert.Equal(t, "导航", page.Navigation.Title["zh-CN"])
}

func TestApplyAutoMergeItemNavigationBreadcrumbV2(t *testing.T) {
	page := spec.PageSpec{}
	item := dashboardmerge.MergeItem{
		Field:       "navigation.breadcrumb",
		MergedValue: json.RawMessage(`[{"title":{"zh-CN":"首页"},"path":"/"}]`),
	}
	err := applyAutoMergeItem(&page, item)
	require.NoError(t, err)
	assert.NotNil(t, page.Navigation)
	assert.Len(t, page.Navigation.Breadcrumb, 1)
}

func TestApplyAutoMergeItemUnsupportedFieldV2(t *testing.T) {
	page := spec.PageSpec{}
	item := dashboardmerge.MergeItem{
		Field:       "unsupported.field",
		MergedValue: json.RawMessage(`"value"`),
	}
	err := applyAutoMergeItem(&page, item)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// applyConflictField - various fields
// ---------------------------------------------------------------------------

func TestApplyConflictFieldTypeV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "type", json.RawMessage(`"operation"`))
	require.NoError(t, err)
	assert.Equal(t, spec.PageTypeOperation, page.Type)
}

func TestApplyConflictFieldResourceKeyV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "resourceKey", json.RawMessage(`"player"`))
	require.NoError(t, err)
	assert.Equal(t, "player", page.ResourceKey)
}

func TestApplyConflictFieldCategoryKeyV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "category.key", json.RawMessage(`"player"`))
	require.NoError(t, err)
	assert.Equal(t, "player", page.Category.Key)
}

func TestApplyConflictFieldBindingsV2(t *testing.T) {
	page := spec.PageSpec{}
	bindings := []spec.PageFunctionBinding{{ID: "run", FunctionID: "player.ban"}}
	raw, _ := json.Marshal(bindings)
	err := applyConflictField(&page, "bindings", raw)
	require.NoError(t, err)
	assert.Len(t, page.Bindings, 1)
}

func TestApplyConflictFieldUnsupportedV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "unsupported.field", json.RawMessage(`"value"`))
	assert.Error(t, err)
}

func TestApplyConflictFieldListViewIdentityKeyV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "resource.listView.identityKey", json.RawMessage(`"player_id"`))
	require.NoError(t, err)
	assert.Equal(t, "player_id", page.Resource.ListView.IdentityKey)
}

func TestApplyConflictFieldListViewRowSchemaV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "resource.listView.rowSchema", json.RawMessage(`{"type":"object"}`))
	require.NoError(t, err)
	assert.NotNil(t, page.Resource.ListView.RowSchema)
}

func TestApplyConflictFieldListViewDefaultSortV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "resource.listView.defaultSort", json.RawMessage(`{"field":"name","order":"asc"}`))
	require.NoError(t, err)
	assert.NotNil(t, page.Resource.ListView.DefaultSort)
}

func TestApplyConflictFieldListViewFiltersV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "resource.listView.filters", json.RawMessage(`[{"key":"status"}]`))
	require.NoError(t, err)
}

func TestApplyConflictFieldListViewPaginationV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "resource.listView.pagination", json.RawMessage(`{"pageSize":20}`))
	require.NoError(t, err)
	assert.NotNil(t, page.Resource.ListView.Pagination)
}

func TestApplyConflictFieldListViewRowActionsV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "resource.listView.rowActions", json.RawMessage(`[{"id":"edit"}]`))
	require.NoError(t, err)
}

func TestApplyConflictFieldListViewBatchActionsV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "resource.listView.batchActions", json.RawMessage(`[{"id":"delete"}]`))
	require.NoError(t, err)
}

func TestApplyConflictFieldListViewToolbarActionsV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "resource.listView.toolbarActions", json.RawMessage(`[{"id":"refresh"}]`))
	require.NoError(t, err)
}

func TestApplyConflictFieldDetailViewActionsV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "resource.detailView.actions", json.RawMessage(`[{"id":"edit"}]`))
	require.NoError(t, err)
}

func TestApplyConflictFieldResourceActionsV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "resource.actions", json.RawMessage(`[{"id":"create"}]`))
	require.NoError(t, err)
}

func TestApplyConflictFieldResourceCreateFormV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "resource.createForm", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.NotNil(t, page.Resource.CreateForm)
}

func TestApplyConflictFieldResourceUpdateFormV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "resource.updateForm", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.NotNil(t, page.Resource.UpdateForm)
}

func TestApplyConflictFieldResourceDeleteActionV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "resource.deleteAction", json.RawMessage(`{"title":{},"bindingID":"run"}`))
	require.NoError(t, err)
	assert.NotNil(t, page.Resource.DeleteAction)
}

func TestApplyConflictFieldOperationFormJsonSchemaV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "operation.form.jsonSchema", json.RawMessage(`{"type":"object"}`))
	require.NoError(t, err)
	assert.NotNil(t, page.Operation.Form)
}

func TestApplyConflictFieldOperationConfirmV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "operation.confirm", json.RawMessage(`{"title":{},"bindingID":"run"}`))
	require.NoError(t, err)
	assert.NotNil(t, page.Operation.Confirm)
}

func TestApplyConflictFieldOperationResultViewV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "operation.resultView", json.RawMessage(`{"title":{}}`))
	require.NoError(t, err)
	assert.NotNil(t, page.Operation.ResultView)
}

func TestApplyConflictFieldTaskFormJsonSchemaV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "task.form.jsonSchema", json.RawMessage(`{"type":"object"}`))
	require.NoError(t, err)
	assert.NotNil(t, page.Task.Form)
}

func TestApplyConflictFieldTaskTaskViewV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "task.taskView", json.RawMessage(`{"title":{}}`))
	require.NoError(t, err)
	assert.NotNil(t, page.Task.TaskView)
}

func TestApplyConflictFieldTaskResultViewV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "task.resultView", json.RawMessage(`{"title":{}}`))
	require.NoError(t, err)
	assert.NotNil(t, page.Task.ResultView)
}

func TestApplyConflictFieldReportQueryFormJsonSchemaV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "report.queryForm.jsonSchema", json.RawMessage(`{"type":"object"}`))
	require.NoError(t, err)
	assert.NotNil(t, page.Report.QueryForm)
}

func TestApplyConflictFieldReportDatasetV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "report.dataset", json.RawMessage(`{"columns":[]}`))
	require.NoError(t, err)
	assert.NotNil(t, page.Report.Dataset)
}

func TestApplyConflictFieldReportTableV2(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "report.table", json.RawMessage(`{"columns":[]}`))
	require.NoError(t, err)
	assert.NotNil(t, page.Report.Table)
}

// ---------------------------------------------------------------------------
// applyFormFieldAutoMergeItem
// ---------------------------------------------------------------------------

func TestApplyFormFieldAutoMergeItemV2(t *testing.T) {
	tests := []struct {
		name string
		leaf string
		val  string
	}{
		{"label", "label", `{"zh-CN":"标签"}`},
		{"placeholder", "placeholder", `{"zh-CN":"请输入"}`},
		{"description", "description", `{"zh-CN":"说明"}`},
		{"order", "order", "5"},
		{"widget", "widget", `"input"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := &spec.FormFieldSpec{}
			item := dashboardmerge.MergeItem{
				Field:       "operation.form.fields[0]." + tt.leaf,
				MergedValue: json.RawMessage(tt.val),
			}
			err := applyFormFieldAutoMergeItem(field, tt.leaf, item)
			require.NoError(t, err)
		})
	}

	t.Run("unsupported", func(t *testing.T) {
		field := &spec.FormFieldSpec{}
		item := dashboardmerge.MergeItem{
			Field:       "operation.form.fields[0].unsupported",
			MergedValue: json.RawMessage(`"value"`),
		}
		err := applyFormFieldAutoMergeItem(field, "unsupported", item)
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// applyFormFieldConflictValue
// ---------------------------------------------------------------------------

func TestApplyFormFieldConflictValueV2(t *testing.T) {
	tests := []struct {
		name string
		leaf string
		val  string
	}{
		{"key", "key", `"playerId"`},
		{"visibleWhen", "visibleWhen", `{"type":"exists","field":"status"}`},
		{"required", "required", "true"},
		{"defaultValue", "defaultValue", `"default"`},
		{"disabled", "disabled", "false"},
		{"widgetProps", "widgetProps", `{"min":1}`},
		{"validationRules", "validationRules", `[{"type":"required"}]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := &spec.FormFieldSpec{}
			err := applyFormFieldConflictValue(field, tt.leaf, json.RawMessage(tt.val), "operation.form.fields[0]."+tt.leaf)
			require.NoError(t, err)
		})
	}

	t.Run("unsupported", func(t *testing.T) {
		field := &spec.FormFieldSpec{}
		err := applyFormFieldConflictValue(field, "unsupported", json.RawMessage(`"value"`), "operation.form.fields[0].unsupported")
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// applyConflictField - indexed fields
// ---------------------------------------------------------------------------

func TestApplyConflictFieldIndexedOperationFormFieldsV2(t *testing.T) {
	page := spec.PageSpec{
		Operation: &spec.OperationPageSpec{
			Form: &spec.FormPresentationSpec{
				Fields: []spec.FormFieldSpec{{Key: "playerId"}},
			},
		},
	}
	err := applyConflictField(&page, "operation.form.fields[0].key", json.RawMessage(`"newPlayerId"`))
	require.NoError(t, err)
	assert.Equal(t, "newPlayerId", page.Operation.Form.Fields[0].Key)
}

func TestApplyConflictFieldIndexedOperationFormFieldsOutOfRangeV2(t *testing.T) {
	page := spec.PageSpec{
		Operation: &spec.OperationPageSpec{
			Form: &spec.FormPresentationSpec{Fields: []spec.FormFieldSpec{}},
		},
	}
	err := applyConflictField(&page, "operation.form.fields[0].key", json.RawMessage(`"test"`))
	assert.Error(t, err)
}

func TestApplyConflictFieldIndexedTaskFormFieldsV2(t *testing.T) {
	page := spec.PageSpec{
		Task: &spec.TaskPageSpec{
			Form: &spec.FormPresentationSpec{
				Fields: []spec.FormFieldSpec{{Key: "playerId"}},
			},
		},
	}
	err := applyConflictField(&page, "task.form.fields[0].key", json.RawMessage(`"newKey"`))
	require.NoError(t, err)
	assert.Equal(t, "newKey", page.Task.Form.Fields[0].Key)
}

func TestApplyConflictFieldIndexedReportQueryFormFieldsV2(t *testing.T) {
	page := spec.PageSpec{
		Report: &spec.ReportPageSpec{
			QueryForm: &spec.FormPresentationSpec{
				Fields: []spec.FormFieldSpec{{Key: "startDate"}},
			},
		},
	}
	err := applyConflictField(&page, "report.queryForm.fields[0].key", json.RawMessage(`"endDate"`))
	require.NoError(t, err)
	assert.Equal(t, "endDate", page.Report.QueryForm.Fields[0].Key)
}

// ---------------------------------------------------------------------------
// Handler - NewHandler
// ---------------------------------------------------------------------------

func TestNewHandlerV2(t *testing.T) {
	service := NewService(setupTestDB(t))
	handler := NewHandler(service)
	assert.NotNil(t, handler)
}

// ---------------------------------------------------------------------------
// pageSpecFromProposalModel
// ---------------------------------------------------------------------------

func TestPageSpecFromProposalModelNilV2(t *testing.T) {
	_, err := pageSpecFromProposalModel(nil)
	assert.Error(t, err)
}

func TestPageSpecFromProposalModelEmptySpecV2(t *testing.T) {
	_, err := pageSpecFromProposalModel(&model.PageProposal{})
	assert.Error(t, err)
}

func TestPageSpecFromProposalModelValidV2(t *testing.T) {
	pageSpec := spec.PageSpec{PageKey: "test", Type: spec.PageTypeResource}
	raw, err := json.Marshal(pageSpec)
	require.NoError(t, err)
	proposal := &model.PageProposal{PageSpec: raw}
	result, err := pageSpecFromProposalModel(proposal)
	require.NoError(t, err)
	assert.Equal(t, "test", result.PageKey)
}

// ---------------------------------------------------------------------------
// proposalForPage
// ---------------------------------------------------------------------------

func TestProposalForPageNilPageV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	_, err := svc.proposalForPage(context.Background(), "game", "env", nil)
	assert.Error(t, err)
}

func TestProposalForPageWithBaseProposalKeyV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	page := &model.PageSpec{BaseProposalKey: "test-key"}
	_, err := svc.proposalForPage(context.Background(), "game", "env", page)
	assert.Error(t, err)
}

func TestProposalForPageWithoutBaseProposalKeyV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	page := &model.PageSpec{PageKey: "test-page"}
	_, err := svc.proposalForPage(context.Background(), "game", "env", page)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// GetChangeChain - error paths
// ---------------------------------------------------------------------------

func TestGetChangeChainEmptyPageKeyV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	_, err := svc.GetChangeChain(context.Background(), &GetChangeChainRequest{
		GameID:  "demo-game",
		Env:     "development",
		PageKey: "",
	})
	assert.Error(t, err)
}

func TestGetChangeChainPageNotFoundV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	_, err := svc.GetChangeChain(context.Background(), &GetChangeChainRequest{
		GameID:  "demo-game",
		Env:     "development",
		PageKey: "nonexistent",
	})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Diff - error paths
// ---------------------------------------------------------------------------

func TestDiffEmptyPageKeyV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	_, err := svc.Diff(context.Background(), &DiffRequest{
		GameID:  "demo-game",
		Env:     "development",
		PageKey: "",
	})
	assert.Error(t, err)
}

func TestDiffPageNotFoundV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	_, err := svc.Diff(context.Background(), &DiffRequest{
		GameID:  "demo-game",
		Env:     "development",
		PageKey: "nonexistent",
	})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Merge - error paths
// ---------------------------------------------------------------------------

func TestMergeUnknownStrategyV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	_, err := svc.Merge(context.Background(), &MergeRequest{
		GameID:   "demo-game",
		Env:      "development",
		PageKey:  "test",
		Strategy: "unknown",
	})
	assert.Error(t, err)
}

func TestMergeAcceptForbiddenV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	_, err := svc.Merge(context.Background(), &MergeRequest{
		GameID:   "demo-game",
		Env:      "development",
		PageKey:  "test",
		Strategy: MergeStrategyAccept,
	})
	assert.Error(t, err)
}

func TestMergeAutoNoDraftRevisionV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	_, err := svc.Merge(context.Background(), &MergeRequest{
		GameID:                "demo-game",
		Env:                   "development",
		PageKey:               "resource--player",
		ExpectedDraftRevision: 0,
		Strategy:              MergeStrategyAuto,
	})
	assert.Error(t, err)
}

func TestMergeEmptyPageKeyV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	_, err := svc.Merge(context.Background(), &MergeRequest{
		GameID:   "demo-game",
		Env:      "development",
		PageKey:  "",
		Strategy: MergeStrategyAuto,
	})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// RollbackDraft - error paths
// ---------------------------------------------------------------------------

func TestRollbackDraftInvalidVersionV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	_, err := svc.RollbackDraft(context.Background(), &RollbackRequest{
		GameID:  "demo-game",
		Env:     "development",
		PageKey: "test",
		Version: 0,
	})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// RollbackPublish - error paths
// ---------------------------------------------------------------------------

func TestRollbackPublishInvalidVersionV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	_, err := svc.RollbackPublish(context.Background(), &RollbackRequest{
		GameID:  "demo-game",
		Env:     "development",
		PageKey: "test",
		Version: 0,
	})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// RegenerateProposal - error paths
// ---------------------------------------------------------------------------

func TestRegenerateProposalPageNotFoundV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	_, err := svc.RegenerateProposal(context.Background(), &RegenerateProposalRequest{
		GameID:  "demo-game",
		Env:     "development",
		PageKey: "nonexistent",
	})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Republish - error paths
// ---------------------------------------------------------------------------

func TestRepublishPageNotFoundV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	_, err := svc.Republish(context.Background(), &RepublishRequest{
		GameID:  "demo-game",
		Env:     "development",
		PageKey: "nonexistent",
	})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// validateManualConflictResolutions
// ---------------------------------------------------------------------------

func TestValidateManualConflictResolutionsNoConflictsV2(t *testing.T) {
	resolutions, err := validateManualConflictResolutions(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, resolutions)
}

func TestValidateManualConflictResolutionsExtraResolutionsV2(t *testing.T) {
	_, err := validateManualConflictResolutions(nil, []ConflictResolution{
		{Path: "extra"},
	})
	assert.Error(t, err)
}

func TestValidateManualConflictResolutionsDuplicatePathV2(t *testing.T) {
	conflicts := []dashboardmerge.MergeConflict{{Field: "title"}}
	_, err := validateManualConflictResolutions(conflicts, []ConflictResolution{
		{Path: "title"},
		{Path: "title"},
	})
	assert.Error(t, err)
}

func TestValidateManualConflictResolutionsEmptyPathV2(t *testing.T) {
	conflicts := []dashboardmerge.MergeConflict{{Field: "title"}}
	_, err := validateManualConflictResolutions(conflicts, []ConflictResolution{
		{Path: ""},
	})
	assert.Error(t, err)
}

func TestValidateManualConflictResolutionsMissingV2(t *testing.T) {
	conflicts := []dashboardmerge.MergeConflict{
		{Field: "title"},
		{Field: "description"},
	}
	_, err := validateManualConflictResolutions(conflicts, []ConflictResolution{
		{Path: "title", AcceptNew: true},
	})
	assert.Error(t, err)
}

func TestValidateManualConflictResolutionsExtraFieldV2(t *testing.T) {
	conflicts := []dashboardmerge.MergeConflict{
		{Field: "title"},
	}
	_, err := validateManualConflictResolutions(conflicts, []ConflictResolution{
		{Path: "title", AcceptNew: true},
		{Path: "extra"},
	})
	assert.Error(t, err)
}

func TestValidateManualConflictResolutionsValidV2(t *testing.T) {
	conflicts := []dashboardmerge.MergeConflict{
		{Field: "title"},
	}
	resolutions, err := validateManualConflictResolutions(conflicts, []ConflictResolution{
		{Path: "title", AcceptNew: true},
	})
	require.NoError(t, err)
	assert.Len(t, resolutions, 1)
}

// ---------------------------------------------------------------------------
// resolveConflictRawValue
// ---------------------------------------------------------------------------

func TestResolveConflictRawValueCustomValueV2(t *testing.T) {
	conflict := dashboardmerge.MergeConflict{
		LatestValue: json.RawMessage(`"new"`),
		DraftValue:  json.RawMessage(`"old"`),
	}
	resolution := ConflictResolution{
		AcceptNew: false,
		Value:     json.RawMessage(`"custom"`),
	}
	result := resolveConflictRawValue(conflict, resolution)
	assert.Equal(t, `"custom"`, string(result))
}

func TestResolveConflictRawValueAcceptNewV2(t *testing.T) {
	conflict := dashboardmerge.MergeConflict{
		LatestValue: json.RawMessage(`"new"`),
		DraftValue:  json.RawMessage(`"old"`),
	}
	resolution := ConflictResolution{AcceptNew: true}
	result := resolveConflictRawValue(conflict, resolution)
	assert.Equal(t, `"new"`, string(result))
}

func TestResolveConflictRawValueKeepOldV2(t *testing.T) {
	conflict := dashboardmerge.MergeConflict{
		LatestValue: json.RawMessage(`"new"`),
		DraftValue:  json.RawMessage(`"old"`),
	}
	resolution := ConflictResolution{AcceptNew: false}
	result := resolveConflictRawValue(conflict, resolution)
	assert.Equal(t, `"old"`, string(result))
}
