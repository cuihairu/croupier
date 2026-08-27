package versioning

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	dashboardmerge "github.com/cuihairu/croupier/internal/dashboard/merge"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
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

// ---------------------------------------------------------------------------
// applyIndexedAutoMergeItem
// ---------------------------------------------------------------------------

func TestApplyIndexedAutoMergeItem_ColumnTitle(t *testing.T) {
	page := &spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{
				Columns: []spec.ColumnSpec{
					{Title: spec.LocalizedText{"zh-CN": "旧"}},
				},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.listView.columns[0].title",
		MergedValue: json.RawMessage(`{"zh-CN":"新"}`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, "新", page.Resource.ListView.Columns[0].Title["zh-CN"])
}

func TestApplyIndexedAutoMergeItem_ColumnWidth(t *testing.T) {
	page := &spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{
				Columns: []spec.ColumnSpec{{Width: 100}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.listView.columns[0].width",
		MergedValue: json.RawMessage(`200`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, 200, page.Resource.ListView.Columns[0].Width)
}

func TestApplyIndexedAutoMergeItem_ColumnVisible(t *testing.T) {
	page := &spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{
				Columns: []spec.ColumnSpec{{Visible: true}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.listView.columns[0].visible",
		MergedValue: json.RawMessage(`false`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, false, page.Resource.ListView.Columns[0].Visible)
}

func TestApplyIndexedAutoMergeItem_ColumnSortable(t *testing.T) {
	page := &spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{
				Columns: []spec.ColumnSpec{{Sortable: false}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.listView.columns[0].sortable",
		MergedValue: json.RawMessage(`true`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, true, page.Resource.ListView.Columns[0].Sortable)
}

func TestApplyIndexedAutoMergeItem_ColumnFilterable(t *testing.T) {
	page := &spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{
				Columns: []spec.ColumnSpec{{Filterable: false}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.listView.columns[0].filterable",
		MergedValue: json.RawMessage(`true`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, true, page.Resource.ListView.Columns[0].Filterable)
}

func TestApplyIndexedAutoMergeItem_ColumnUnsupportedLeaf(t *testing.T) {
	page := &spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{
				Columns: []spec.ColumnSpec{{}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.listView.columns[0].unknownField",
		MergedValue: json.RawMessage(`"x"`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.True(t, ok)
	assert.Error(t, err)
}

func TestApplyIndexedAutoMergeItem_ColumnOutOfRange(t *testing.T) {
	page := &spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{
				Columns: []spec.ColumnSpec{},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.listView.columns[5].title",
		MergedValue: json.RawMessage(`"x"`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.True(t, ok)
	assert.Error(t, err)
}

func TestApplyIndexedAutoMergeItem_ColumnNilResource(t *testing.T) {
	page := &spec.PageSpec{}
	item := dashboardmerge.MergeItem{
		Field:       "resource.listView.columns[0].title",
		MergedValue: json.RawMessage(`"x"`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.True(t, ok)
	assert.Error(t, err)
}

func TestApplyIndexedAutoMergeItem_DetailViewTitle(t *testing.T) {
	page := &spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			DetailView: &spec.DetailViewSpec{
				Fields: []spec.DetailFieldSpec{
					{Title: spec.LocalizedText{"zh-CN": "旧"}},
				},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.detailView.fields[0].title",
		MergedValue: json.RawMessage(`{"zh-CN":"新"}`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.True(t, ok)
	assert.NoError(t, err)
}

func TestApplyIndexedAutoMergeItem_DetailViewSpan(t *testing.T) {
	page := &spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			DetailView: &spec.DetailViewSpec{
				Fields: []spec.DetailFieldSpec{{Span: 6}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.detailView.fields[0].span",
		MergedValue: json.RawMessage(`12`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, 12, page.Resource.DetailView.Fields[0].Span)
}

func TestApplyIndexedAutoMergeItem_DetailViewUnsupportedLeaf(t *testing.T) {
	page := &spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			DetailView: &spec.DetailViewSpec{
				Fields: []spec.DetailFieldSpec{{}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.detailView.fields[0].unknownLeaf",
		MergedValue: json.RawMessage(`"x"`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.True(t, ok)
	assert.Error(t, err)
}

func TestApplyIndexedAutoMergeItem_DetailViewOutOfRange(t *testing.T) {
	page := &spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			DetailView: &spec.DetailViewSpec{
				Fields: []spec.DetailFieldSpec{},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.detailView.fields[5].title",
		MergedValue: json.RawMessage(`"x"`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.True(t, ok)
	assert.Error(t, err)
}

func TestApplyIndexedAutoMergeItem_OperationFormFields(t *testing.T) {
	page := &spec.PageSpec{
		Operation: &spec.OperationPageSpec{
			Form: &spec.FormPresentationSpec{
				Fields: []spec.FormFieldSpec{
					{Label: spec.LocalizedText{"zh-CN": "旧"}},
				},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "operation.form.fields[0].label",
		MergedValue: json.RawMessage(`{"zh-CN":"新"}`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.True(t, ok)
	assert.NoError(t, err)
}

func TestApplyIndexedAutoMergeItem_OperationFormOutOfRange(t *testing.T) {
	page := &spec.PageSpec{
		Operation: &spec.OperationPageSpec{
			Form: &spec.FormPresentationSpec{
				Fields: []spec.FormFieldSpec{},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "operation.form.fields[5].label",
		MergedValue: json.RawMessage(`"x"`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.True(t, ok)
	assert.Error(t, err)
}

func TestApplyIndexedAutoMergeItem_TaskFormFields(t *testing.T) {
	page := &spec.PageSpec{
		Task: &spec.TaskPageSpec{
			Form: &spec.FormPresentationSpec{
				Fields: []spec.FormFieldSpec{
					{Label: spec.LocalizedText{"zh-CN": "旧"}},
				},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "task.form.fields[0].label",
		MergedValue: json.RawMessage(`{"zh-CN":"新"}`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.True(t, ok)
	assert.NoError(t, err)
}

func TestApplyIndexedAutoMergeItem_TaskFormOutOfRange(t *testing.T) {
	page := &spec.PageSpec{
		Task: &spec.TaskPageSpec{
			Form: &spec.FormPresentationSpec{
				Fields: []spec.FormFieldSpec{},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "task.form.fields[5].label",
		MergedValue: json.RawMessage(`"x"`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.True(t, ok)
	assert.Error(t, err)
}

func TestApplyIndexedAutoMergeItem_ReportQueryFormFields(t *testing.T) {
	page := &spec.PageSpec{
		Report: &spec.ReportPageSpec{
			QueryForm: &spec.FormPresentationSpec{
				Fields: []spec.FormFieldSpec{
					{Label: spec.LocalizedText{"zh-CN": "旧"}},
				},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "report.queryForm.fields[0].label",
		MergedValue: json.RawMessage(`{"zh-CN":"新"}`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.True(t, ok)
	assert.NoError(t, err)
}

func TestApplyIndexedAutoMergeItem_ReportQueryFormOutOfRange(t *testing.T) {
	page := &spec.PageSpec{
		Report: &spec.ReportPageSpec{
			QueryForm: &spec.FormPresentationSpec{
				Fields: []spec.FormFieldSpec{},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "report.queryForm.fields[5].label",
		MergedValue: json.RawMessage(`"x"`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.True(t, ok)
	assert.Error(t, err)
}

func TestApplyIndexedAutoMergeItem_ReportChartsTitle(t *testing.T) {
	page := &spec.PageSpec{
		Report: &spec.ReportPageSpec{
			Charts: []spec.ChartSpec{
				{Title: spec.LocalizedText{"zh-CN": "旧"}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "report.charts[0].title",
		MergedValue: json.RawMessage(`{"zh-CN":"新"}`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.True(t, ok)
	assert.NoError(t, err)
}

func TestApplyIndexedAutoMergeItem_ReportChartsUnsupportedLeaf(t *testing.T) {
	page := &spec.PageSpec{
		Report: &spec.ReportPageSpec{
			Charts: []spec.ChartSpec{{}},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "report.charts[0].unknownLeaf",
		MergedValue: json.RawMessage(`"x"`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.True(t, ok)
	assert.Error(t, err)
}

func TestApplyIndexedAutoMergeItem_ReportChartsOutOfRange(t *testing.T) {
	page := &spec.PageSpec{
		Report: &spec.ReportPageSpec{
			Charts: []spec.ChartSpec{},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "report.charts[5].title",
		MergedValue: json.RawMessage(`"x"`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.True(t, ok)
	assert.Error(t, err)
}

func TestApplyIndexedAutoMergeItem_UnmatchedField(t *testing.T) {
	page := &spec.PageSpec{}
	item := dashboardmerge.MergeItem{
		Field:       "unknown.path.field",
		MergedValue: json.RawMessage(`"x"`),
	}
	ok, err := applyIndexedAutoMergeItem(page, item)
	assert.False(t, ok)
	assert.NoError(t, err)
}

func testBoolPtr(v bool) *bool { return &v }

// ---------------------------------------------------------------------------
// decodeMergeValue
// ---------------------------------------------------------------------------

func TestDecodeMergeValue_Empty(t *testing.T) {
	var s string
	item := dashboardmerge.MergeItem{Field: "test"}
	assert.NoError(t, decodeMergeValue(item, &s))
}

func TestDecodeMergeValue_Null(t *testing.T) {
	var s string
	item := dashboardmerge.MergeItem{Field: "test", MergedValue: json.RawMessage(`null`)}
	assert.NoError(t, decodeMergeValue(item, &s))
}

func TestDecodeMergeValue_String(t *testing.T) {
	var s string
	item := dashboardmerge.MergeItem{Field: "test", MergedValue: json.RawMessage(`"hello"`)}
	assert.NoError(t, decodeMergeValue(item, &s))
	assert.Equal(t, "hello", s)
}

func TestDecodeMergeValue_Int(t *testing.T) {
	var n int
	item := dashboardmerge.MergeItem{Field: "test", MergedValue: json.RawMessage(`42`)}
	assert.NoError(t, decodeMergeValue(item, &n))
	assert.Equal(t, 42, n)
}

func TestDecodeMergeValue_Bool(t *testing.T) {
	var b bool
	item := dashboardmerge.MergeItem{Field: "test", MergedValue: json.RawMessage(`true`)}
	assert.NoError(t, decodeMergeValue(item, &b))
	assert.True(t, b)
}

func TestDecodeMergeValue_InvalidJSON(t *testing.T) {
	var s string
	item := dashboardmerge.MergeItem{Field: "test.field", MergedValue: json.RawMessage(`{invalid`)}
	err := decodeMergeValue(item, &s)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test.field")
}

func TestDecodeMergeValue_Map(t *testing.T) {
	var m map[string]string
	item := dashboardmerge.MergeItem{Field: "test", MergedValue: json.RawMessage(`{"key":"value"}`)}
	assert.NoError(t, decodeMergeValue(item, &m))
	assert.Equal(t, "value", m["key"])
}

// ---------------------------------------------------------------------------
// applyConflictResolutions
// ---------------------------------------------------------------------------

func TestApplyConflictResolutions_Empty(t *testing.T) {
	page := spec.PageSpec{PageKey: "test"}
	result, err := applyConflictResolutions(page, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "test", result.PageKey)
}

func TestApplyConflictResolutions_MissingResolution(t *testing.T) {
	page := spec.PageSpec{}
	conflicts := []dashboardmerge.MergeConflict{
		{Field: "title"},
	}
	_, err := applyConflictResolutions(page, conflicts, map[string]ConflictResolution{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "title")
}

func TestApplyConflictResolutions_AcceptNew(t *testing.T) {
	page := spec.PageSpec{}
	conflicts := []dashboardmerge.MergeConflict{
		{Field: "type", LatestValue: json.RawMessage(`"operation"`), DraftValue: json.RawMessage(`"resource"`)},
	}
	resolutions := map[string]ConflictResolution{
		"type": {AcceptNew: true},
	}
	result, err := applyConflictResolutions(page, conflicts, resolutions)
	require.NoError(t, err)
	assert.Equal(t, spec.PageTypeOperation, result.Type)
}

func TestApplyConflictResolutions_KeepOld(t *testing.T) {
	page := spec.PageSpec{}
	conflicts := []dashboardmerge.MergeConflict{
		{Field: "type", LatestValue: json.RawMessage(`"operation"`), DraftValue: json.RawMessage(`"resource"`)},
	}
	resolutions := map[string]ConflictResolution{
		"type": {AcceptNew: false},
	}
	result, err := applyConflictResolutions(page, conflicts, resolutions)
	require.NoError(t, err)
	assert.Equal(t, spec.PageTypeResource, result.Type)
}

func TestApplyConflictResolutions_CustomValue(t *testing.T) {
	page := spec.PageSpec{}
	conflicts := []dashboardmerge.MergeConflict{
		{Field: "resourceKey", LatestValue: json.RawMessage(`"item"`), DraftValue: json.RawMessage(`"player"`)},
	}
	resolutions := map[string]ConflictResolution{
		"resourceKey": {Value: json.RawMessage(`"guild"`)},
	}
	result, err := applyConflictResolutions(page, conflicts, resolutions)
	require.NoError(t, err)
	assert.Equal(t, "guild", result.ResourceKey)
}

func TestApplyConflictResolutions_InvalidField(t *testing.T) {
	page := spec.PageSpec{}
	conflicts := []dashboardmerge.MergeConflict{
		{Field: "unsupported.field"},
	}
	resolutions := map[string]ConflictResolution{
		"unsupported.field": {AcceptNew: true},
	}
	_, err := applyConflictResolutions(page, conflicts, resolutions)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// applyAutoMergeItems
// ---------------------------------------------------------------------------

func TestApplyAutoMergeItems_Empty(t *testing.T) {
	page := spec.PageSpec{PageKey: "test"}
	result, err := applyAutoMergeItems(page, nil)
	require.NoError(t, err)
	assert.Equal(t, "test", result.PageKey)
}

func TestApplyAutoMergeItems_WithItems(t *testing.T) {
	page := spec.PageSpec{}
	items := []dashboardmerge.MergeItem{
		{Field: "title", MergedValue: json.RawMessage(`{"zh-CN":"标题"}`)},
		{Field: "icon", MergedValue: json.RawMessage(`"icon-name"`)},
	}
	result, err := applyAutoMergeItems(page, items)
	require.NoError(t, err)
	assert.Equal(t, "标题", result.Title["zh-CN"])
	assert.Equal(t, "icon-name", result.Icon)
}

func TestApplyAutoMergeItems_Error(t *testing.T) {
	page := spec.PageSpec{}
	items := []dashboardmerge.MergeItem{
		{Field: "unsupported.field", MergedValue: json.RawMessage(`"value"`)},
	}
	_, err := applyAutoMergeItems(page, items)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// applyFormFieldConflictValue
// ---------------------------------------------------------------------------

func TestApplyFormFieldConflictValue_ValidationRules(t *testing.T) {
	field := &spec.FormFieldSpec{}
	raw := json.RawMessage(`[{"type":"required"}]`)
	err := applyFormFieldConflictValue(field, "validationRules", raw, "operation.form.fields[0].validationRules")
	require.NoError(t, err)
	assert.NotNil(t, field.ValidationRules)
}

func TestApplyFormFieldConflictValue_UnsupportedLeaf(t *testing.T) {
	field := &spec.FormFieldSpec{}
	raw := json.RawMessage(`"value"`)
	err := applyFormFieldConflictValue(field, "unsupportedLeaf", raw, "operation.form.fields[0].unsupportedLeaf")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// applyIndexedConflictField
// ---------------------------------------------------------------------------

func TestApplyIndexedConflictField_OperationFormFieldKey(t *testing.T) {
	page := &spec.PageSpec{
		Operation: &spec.OperationPageSpec{
			Form: &spec.FormPresentationSpec{
				Fields: []spec.FormFieldSpec{
					{Key: "old"},
				},
			},
		},
	}
	ok, err := applyIndexedConflictField(page, "operation.form.fields[0].key", json.RawMessage(`"new"`))
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, "new", page.Operation.Form.Fields[0].Key)
}

func TestApplyIndexedConflictField_OperationFormFieldOutOfRange(t *testing.T) {
	page := &spec.PageSpec{}
	_, err := applyIndexedConflictField(page, "operation.form.fields[5].key", json.RawMessage(`"new"`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestApplyIndexedConflictField_UnmatchedField(t *testing.T) {
	page := &spec.PageSpec{}
	ok, err := applyIndexedConflictField(page, "unmatched.field", json.RawMessage(`"value"`))
	assert.False(t, ok)
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// draftRevisionConflict
// ---------------------------------------------------------------------------

func TestDraftRevisionConflict(t *testing.T) {
	err := draftRevisionConflict(1, 2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "page draft revision conflict")
}

// ---------------------------------------------------------------------------
// pageSpecFromProposalSnapshot
// ---------------------------------------------------------------------------

func TestPageSpecFromProposalSnapshotValid(t *testing.T) {
	pageSpec := spec.PageSpec{PageKey: "test", Type: spec.PageTypeResource}
	proposal := model.PageProposal{PageSpec: model.JSON(mustMarshal(t, pageSpec))}
	raw := mustMarshal(t, proposal)
	result, err := pageSpecFromProposalSnapshot(raw)
	require.NoError(t, err)
	assert.Equal(t, "test", result.PageKey)
}

func TestPageSpecFromProposalSnapshotInvalidJSON(t *testing.T) {
	_, err := pageSpecFromProposalSnapshot(json.RawMessage(`invalid`))
	assert.Error(t, err)
}

func TestPageSpecFromProposalSnapshotEmptyPageSpec(t *testing.T) {
	proposal := model.PageProposal{}
	raw := mustMarshal(t, proposal)
	result, err := pageSpecFromProposalSnapshot(raw)
	// When PageSpec is null in JSON, it becomes empty after unmarshal
	// The function will return an empty PageSpec without error
	require.NoError(t, err)
	assert.Empty(t, result.PageKey)
}

// ---------------------------------------------------------------------------
// pageSpecFromModel
// ---------------------------------------------------------------------------

func TestPageSpecFromModelValid(t *testing.T) {
	pageSpec := spec.PageSpec{PageKey: "test", Type: spec.PageTypeResource}
	page := &model.PageSpec{
		PageKey:  "test",
		Type:     "resource",
		SpecJSON: string(mustMarshal(t, pageSpec)),
	}
	result, err := pageSpecFromModel(page)
	require.NoError(t, err)
	assert.Equal(t, "test", result.PageKey)
}

func TestPageSpecFromModelNil(t *testing.T) {
	_, err := pageSpecFromModel(nil)
	assert.Error(t, err)
}

func TestPageSpecFromModelEmptySpecJSON(t *testing.T) {
	page := &model.PageSpec{SpecJSON: ""}
	_, err := pageSpecFromModel(page)
	assert.Error(t, err)
}

func TestPageSpecFromModelInvalidJSON(t *testing.T) {
	page := &model.PageSpec{SpecJSON: "invalid"}
	_, err := pageSpecFromModel(page)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// normalizePageSpec
// ---------------------------------------------------------------------------

func TestNormalizePageSpecWhitespace(t *testing.T) {
	page := spec.PageSpec{
		PageKey:     "  test  ",
		ResourceKey: "  player  ",
		Icon:        "  icon  ",
		Category: spec.PageCategorySpec{
			Key:    "  cat  ",
			Labels: map[string]string{"zh-CN": "  玩家  "},
		},
		Bindings: []spec.PageFunctionBinding{
			{ID: "  run  ", FunctionID: "  player.ban  "},
		},
	}
	result := normalizePageSpec(page)
	assert.Equal(t, "test", result.PageKey)
	assert.Equal(t, "player", result.ResourceKey)
	assert.Equal(t, "icon", result.Icon)
	assert.Equal(t, "cat", result.Category.Key)
	assert.Equal(t, "玩家", result.Category.Labels["zh-CN"])
	assert.Equal(t, "run", result.Bindings[0].ID)
	assert.Equal(t, "player.ban", result.Bindings[0].FunctionID)
}

// ---------------------------------------------------------------------------
// samePageSpec
// ---------------------------------------------------------------------------

func TestSamePageSpecEqual(t *testing.T) {
	left := spec.PageSpec{PageKey: "test", Type: spec.PageTypeResource}
	right := spec.PageSpec{PageKey: "test", Type: spec.PageTypeResource}
	assert.True(t, samePageSpec(left, right))
}

func TestSamePageSpecDifferent(t *testing.T) {
	left := spec.PageSpec{PageKey: "test", Type: spec.PageTypeResource}
	right := spec.PageSpec{PageKey: "test", Type: spec.PageTypeOperation}
	assert.False(t, samePageSpec(left, right))
}

func TestSamePageSpecBothEmpty(t *testing.T) {
	left := spec.PageSpec{}
	right := spec.PageSpec{}
	assert.True(t, samePageSpec(left, right))
}

// ---------------------------------------------------------------------------
// applyPageSpecToModel
// ---------------------------------------------------------------------------

func TestApplyPageSpecToModel(t *testing.T) {
	page := &model.PageSpec{}
	pageSpec := spec.PageSpec{
		PageKey:     "test",
		Type:        spec.PageTypeResource,
		ResourceKey: "player",
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: map[string]string{"zh-CN": "玩家"},
		},
		Order: 10,
		Icon:  "icon-name",
		Title: map[string]string{"zh-CN": "测试"},
	}
	err := applyPageSpecToModel(page, pageSpec)
	require.NoError(t, err)
	assert.Equal(t, "test", page.PageKey)
	assert.Equal(t, "resource", page.Type)
	assert.Equal(t, "player", page.ResourceKey)
	assert.Equal(t, 10, page.Order)
	assert.Equal(t, "icon-name", page.Icon)
}

// ---------------------------------------------------------------------------
// marshalPageSpec
// ---------------------------------------------------------------------------

func TestMarshalPageSpec(t *testing.T) {
	page := spec.PageSpec{
		PageKey: "test",
		Type:    spec.PageTypeResource,
	}
	result, err := marshalPageSpec(page)
	require.NoError(t, err)
	assert.Contains(t, result, "test")
	assert.Contains(t, result, "resource")
}

// ---------------------------------------------------------------------------
// pageSpecFromProposalModel more tests
// ---------------------------------------------------------------------------

func TestPageSpecFromProposalModelInvalidJSON(t *testing.T) {
	proposal := &model.PageProposal{PageSpec: model.JSON(`invalid`)}
	_, err := pageSpecFromProposalModel(proposal)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// decodeActionListField
// ---------------------------------------------------------------------------

func TestDecodeActionListFieldValid(t *testing.T) {
	raw := json.RawMessage(`[{"key":"edit","title":{"zh-CN":"编辑"}}]`)
	var actions []spec.ActionSpec
	err := decodeActionListField(raw, "test", &actions)
	require.NoError(t, err)
	assert.Len(t, actions, 1)
	assert.Equal(t, "edit", actions[0].Key)
}

func TestDecodeActionListFieldEmpty(t *testing.T) {
	var actions []spec.ActionSpec
	err := decodeActionListField(nil, "test", &actions)
	assert.NoError(t, err)
}

func TestDecodeActionListFieldInvalidJSON(t *testing.T) {
	var actions []spec.ActionSpec
	err := decodeActionListField(json.RawMessage(`invalid`), "test.field", &actions)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test.field")
}

// ---------------------------------------------------------------------------
// applyConflictField more cases
// ---------------------------------------------------------------------------

func TestApplyConflictFieldListViewSortSpec(t *testing.T) {
	page := spec.PageSpec{}
	sortSpec := &spec.SortSpec{Field: "name", Order: "asc"}
	raw, _ := json.Marshal(sortSpec)
	err := applyConflictField(&page, "resource.listView.defaultSort", raw)
	require.NoError(t, err)
	assert.NotNil(t, page.Resource.ListView.DefaultSort)
}

func TestApplyConflictFieldListViewFilters(t *testing.T) {
	page := spec.PageSpec{}
	filters := []spec.FilterSpec{{Key: "status"}}
	raw, _ := json.Marshal(filters)
	err := applyConflictField(&page, "resource.listView.filters", raw)
	require.NoError(t, err)
	assert.Len(t, page.Resource.ListView.Filters, 1)
}

func TestApplyConflictFieldListViewPagination(t *testing.T) {
	page := spec.PageSpec{}
	pagination := &spec.PaginationSpec{Enabled: true, DefaultSize: 20}
	raw, _ := json.Marshal(pagination)
	err := applyConflictField(&page, "resource.listView.pagination", raw)
	require.NoError(t, err)
	assert.NotNil(t, page.Resource.ListView.Pagination)
}

// ---------------------------------------------------------------------------
// helper mustMarshal
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// normalizeLocalizedText
// ---------------------------------------------------------------------------

func mustMarshal(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return raw
}

// ---------------------------------------------------------------------------
// getScope
// ---------------------------------------------------------------------------

func TestGetScopeFromContext(t *testing.T) {
	tests := []struct {
		name     string
		gameID   string
		env      string
		wantGame string
		wantEnv  string
	}{
		{"normal scope", "game1", "prod", "game1", "prod"},
		{"empty scope", "", "", "", ""},
		{"game only", "game1", "", "game1", ""},
		{"env only", "", "dev", "", "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/", nil)
			ctx := svc.WithGameScope(c.Request.Context(), svc.GameScope{GameID: tt.gameID, Env: tt.env})
			c.Request = c.Request.WithContext(ctx)
			gotGame, gotEnv := getScope(c)
			assert.Equal(t, tt.wantGame, gotGame)
			assert.Equal(t, tt.wantEnv, gotEnv)
		})
	}
}

func TestGetScopeEmptyContext(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	gotGame, gotEnv := getScope(c)
	assert.Equal(t, "", gotGame)
	assert.Equal(t, "", gotEnv)
}

// ---------------------------------------------------------------------------
// buildBindingContracts
// ---------------------------------------------------------------------------

func TestBuildBindingContractsEmptyBindingsV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	contracts, err := svc.buildBindingContracts(context.Background(), "game1", "prod", []spec.PageFunctionBinding{})
	require.NoError(t, err)
	assert.Empty(t, contracts)
}

func TestBuildBindingContractsEmptyFunctionIDV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	_, err := svc.buildBindingContracts(context.Background(), "game1", "prod", []spec.PageFunctionBinding{
		{ID: "b1", FunctionID: ""},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "functionId is required")
}

func TestBuildBindingContractsFunctionNotFoundV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	_, err := svc.buildBindingContracts(context.Background(), "game1", "prod", []spec.PageFunctionBinding{
		{ID: "b1", FunctionID: "nonexistent"},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestBuildBindingContractsDisabledFunctionV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	// Create a disabled contract
	contract := &model.FunctionContract{
		GameID:     "game1",
		Env:        "prod",
		FunctionID: "fn1",
		Enabled:    false,
		Version:    "1.0.0",
		Risk:       dbenum.RiskWarning,
	}
	db.Create(contract)
	_, err := svc.buildBindingContracts(context.Background(), "game1", "prod", []spec.PageFunctionBinding{
		{ID: "b1", FunctionID: "fn1"},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestBuildBindingContractsSuccessV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	// Create an enabled contract
	contract := &model.FunctionContract{
		GameID:     "game1",
		Env:        "prod",
		FunctionID: "fn1",
		Enabled:    true,
		Version:    "1.0.0",
		Risk:       dbenum.RiskWarning,
		Permission: "player:list",
	}
	db.Create(contract)
	contracts, err := svc.buildBindingContracts(context.Background(), "game1", "prod", []spec.PageFunctionBinding{
		{ID: "b1", FunctionID: "fn1", Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}},
	})
	require.NoError(t, err)
	assert.Len(t, contracts, 1)
	assert.Equal(t, "b1", contracts[0].BindingID)
	assert.Equal(t, "fn1", contracts[0].FunctionID)
	assert.Equal(t, "1.0.0", contracts[0].FunctionVersion)
}

// ---------------------------------------------------------------------------
// bindingContractChanges
// ---------------------------------------------------------------------------

func TestBindingContractChangesNoPublishedV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	pageSpec := spec.PageSpec{
		Bindings: []spec.PageFunctionBinding{
			{ID: "b1", FunctionID: "fn1"},
		},
	}
	changes := svc.bindingContractChanges(context.Background(), "game1", "prod", "page1", pageSpec)
	// No published version, no contract -> function removed
	assert.Len(t, changes, 1)
	assert.Equal(t, "removed", changes[0].ChangeType)
}

func TestBindingContractChangesWithPublishedV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	// Create a published page spec with a frozen contract
	// Use empty input/output schemas so digest matches
	contract := &model.FunctionContract{
		GameID:     "game1",
		Env:        "prod",
		FunctionID: "fn1",
		Enabled:    true,
		Version:    "1.0.0",
		Risk:       dbenum.RiskWarning,
	}
	db.Create(contract)
	// Compute the expected digests from the contract's nil schemas
	expectedInputDigest := digestRaw(contract.InputSchema)
	expectedOutputDigest := digestRaw(contract.OutputSchema)
	bindingContracts := []spec.BindingContractSnapshot{
		{
			BindingID:          "b1",
			FunctionID:         "fn1",
			FunctionVersion:    "1.0.0",
			InputSchemaDigest:  expectedInputDigest,
			OutputSchemaDigest: expectedOutputDigest,
			Risk:               "warning",
			Permission:         "",
		},
	}
	contractsJSON, _ := json.Marshal(bindingContracts)
	db.Create(&model.PublishedPageSpec{
		GameID:               "game1",
		Env:                  "prod",
		PageKey:              "page1",
		Version:              1,
		SpecJSON:             "{}",
		BindingContractsJSON: string(contractsJSON),
		Active:               true,
	})
	pageSpec := spec.PageSpec{
		Bindings: []spec.PageFunctionBinding{
			{ID: "b1", FunctionID: "fn1"},
		},
	}
	changes := svc.bindingContractChanges(context.Background(), "game1", "prod", "page1", pageSpec)
	// No changes since contract hasn't changed
	assert.Empty(t, changes)
}

// ---------------------------------------------------------------------------
// draftBindingContractChanges
// ---------------------------------------------------------------------------

func TestDraftBindingContractChangesEmptyBindingsV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	pageSpec := spec.PageSpec{}
	changes := svc.draftBindingContractChanges(context.Background(), "game1", "prod", pageSpec)
	assert.Empty(t, changes)
}

func TestDraftBindingContractChangesEmptyFunctionIDV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	pageSpec := spec.PageSpec{
		Bindings: []spec.PageFunctionBinding{
			{ID: "b1", FunctionID: ""},
		},
	}
	changes := svc.draftBindingContractChanges(context.Background(), "game1", "prod", pageSpec)
	assert.Empty(t, changes)
}

func TestDraftBindingContractChangesFunctionNotFoundV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	pageSpec := spec.PageSpec{
		Bindings: []spec.PageFunctionBinding{
			{ID: "b1", FunctionID: "nonexistent"},
		},
	}
	changes := svc.draftBindingContractChanges(context.Background(), "game1", "prod", pageSpec)
	assert.Len(t, changes, 1)
	assert.Equal(t, "removed", changes[0].ChangeType)
}

func TestDraftBindingContractChangesFunctionExistsV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	contract := &model.FunctionContract{
		GameID:     "game1",
		Env:        "prod",
		FunctionID: "fn1",
		Enabled:    true,
		Version:    "1.0.0",
		Risk:       dbenum.RiskWarning,
	}
	db.Create(contract)
	pageSpec := spec.PageSpec{
		Bindings: []spec.PageFunctionBinding{
			{ID: "b1", FunctionID: "fn1"},
		},
	}
	changes := svc.draftBindingContractChanges(context.Background(), "game1", "prod", pageSpec)
	assert.Empty(t, changes)
}

// ---------------------------------------------------------------------------
// mainContractForStandalonePage
// ---------------------------------------------------------------------------

func TestMainContractForStandalonePageResourceTypeV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	pageSpec := spec.PageSpec{Type: spec.PageTypeResource}
	_, err := svc.mainContractForStandalonePage(context.Background(), "game1", "prod", pageSpec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resource pages")
}

func TestMainContractForStandalonePageNoActionBindingV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	pageSpec := spec.PageSpec{
		Type: spec.PageTypeOperation,
		Bindings: []spec.PageFunctionBinding{
			{ID: "b1", FunctionID: "fn1", Usage: "list"},
		},
	}
	_, err := svc.mainContractForStandalonePage(context.Background(), "game1", "prod", pageSpec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no main executable binding")
}

func TestMainContractForStandalonePageContractNotFoundV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	pageSpec := spec.PageSpec{
		Type: spec.PageTypeOperation,
		Bindings: []spec.PageFunctionBinding{
			{ID: "b1", FunctionID: "nonexistent", Usage: spec.BindingUsageAction},
		},
	}
	_, err := svc.mainContractForStandalonePage(context.Background(), "game1", "prod", pageSpec)
	assert.Error(t, err)
}

func TestMainContractForStandalonePageSuccessV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	contract := &model.FunctionContract{
		GameID:     "game1",
		Env:        "prod",
		FunctionID: "fn1",
		Enabled:    true,
		Version:    "1.0.0",
		Risk:       dbenum.RiskWarning,
	}
	db.Create(contract)
	pageSpec := spec.PageSpec{
		Type: spec.PageTypeOperation,
		Bindings: []spec.PageFunctionBinding{
			{ID: "b1", FunctionID: "fn1", Usage: spec.BindingUsageAction},
		},
	}
	result, err := svc.mainContractForStandalonePage(context.Background(), "game1", "prod", pageSpec)
	require.NoError(t, err)
	assert.Equal(t, "fn1", result.FunctionID)
}

// ---------------------------------------------------------------------------
// functionSpecsByID
// ---------------------------------------------------------------------------

func TestFunctionSpecsByIDNoBindingsV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	pageSpec := spec.PageSpec{}
	specs, err := svc.functionSpecsByID(context.Background(), "game1", "prod", pageSpec)
	require.NoError(t, err)
	assert.Empty(t, specs)
}

func TestFunctionSpecsByIDWithContractV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	contract := &model.FunctionContract{
		GameID:     "game1",
		Env:        "prod",
		FunctionID: "fn1",
		Enabled:    true,
		Version:    "1.0.0",
		Risk:       dbenum.RiskWarning,
	}
	db.Create(contract)
	pageSpec := spec.PageSpec{
		Bindings: []spec.PageFunctionBinding{
			{ID: "b1", FunctionID: "fn1"},
		},
	}
	specs, err := svc.functionSpecsByID(context.Background(), "game1", "prod", pageSpec)
	require.NoError(t, err)
	assert.NotEmpty(t, specs)
}

// ---------------------------------------------------------------------------
// contractsForPage
// ---------------------------------------------------------------------------

func TestContractsForPageEmptyBindingsV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	pageSpec := spec.PageSpec{}
	contracts, err := svc.contractsForPage(context.Background(), "game1", "prod", pageSpec)
	require.NoError(t, err)
	assert.Empty(t, contracts)
}

func TestContractsForPageEmptyFunctionIDV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	pageSpec := spec.PageSpec{
		Bindings: []spec.PageFunctionBinding{
			{ID: "b1", FunctionID: ""},
		},
	}
	contracts, err := svc.contractsForPage(context.Background(), "game1", "prod", pageSpec)
	require.NoError(t, err)
	assert.Empty(t, contracts)
}

func TestContractsForPageDuplicateFunctionIDV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	contract := &model.FunctionContract{
		GameID:     "game1",
		Env:        "prod",
		FunctionID: "fn1",
		Enabled:    true,
		Version:    "1.0.0",
		Risk:       dbenum.RiskWarning,
	}
	db.Create(contract)
	pageSpec := spec.PageSpec{
		Bindings: []spec.PageFunctionBinding{
			{ID: "b1", FunctionID: "fn1"},
			{ID: "b2", FunctionID: "fn1"},
		},
	}
	contracts, err := svc.contractsForPage(context.Background(), "game1", "prod", pageSpec)
	require.NoError(t, err)
	assert.Len(t, contracts, 1)
}

func TestContractsForPageNotFoundV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	pageSpec := spec.PageSpec{
		Bindings: []spec.PageFunctionBinding{
			{ID: "b1", FunctionID: "nonexistent"},
		},
	}
	contracts, err := svc.contractsForPage(context.Background(), "game1", "prod", pageSpec)
	require.NoError(t, err)
	assert.Empty(t, contracts)
}

// ---------------------------------------------------------------------------
// applyAutoMergeItem - resource.listView.defaultSort branch
// ---------------------------------------------------------------------------

func TestApplyAutoMergeItemListViewDefaultSortV2(t *testing.T) {
	page := spec.PageSpec{
		Resource: &spec.ResourcePageSpec{},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.listView.defaultSort",
		MergedValue: json.RawMessage(`{"field":"name","order":"asc"}`),
	}
	err := applyAutoMergeItem(&page, item)
	require.NoError(t, err)
	require.NotNil(t, page.Resource.ListView)
	require.NotNil(t, page.Resource.ListView.DefaultSort)
	assert.Equal(t, "name", page.Resource.ListView.DefaultSort.Field)
}

func TestApplyAutoMergeItemListViewDefaultSortInvalidV2(t *testing.T) {
	page := spec.PageSpec{
		Resource: &spec.ResourcePageSpec{},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.listView.defaultSort",
		MergedValue: json.RawMessage(`{invalid json`),
	}
	err := applyAutoMergeItem(&page, item)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// applyIndexedAutoMergeItem - indexed merge fields
// ---------------------------------------------------------------------------

func TestApplyIndexedAutoMergeColumnTitleV2(t *testing.T) {
	page := spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{
				Columns: []spec.ColumnSpec{{Key: "name"}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.listView.columns[0].title",
		MergedValue: json.RawMessage(`{"zh":"名称"}`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	require.NoError(t, err)
	assert.Equal(t, spec.LocalizedText{"zh": "名称"}, page.Resource.ListView.Columns[0].Title)
}

func TestApplyIndexedAutoMergeColumnWidthV2(t *testing.T) {
	page := spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{
				Columns: []spec.ColumnSpec{{Key: "name"}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.listView.columns[0].width",
		MergedValue: json.RawMessage(`200`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	require.NoError(t, err)
	assert.Equal(t, 200, page.Resource.ListView.Columns[0].Width)
}

func TestApplyIndexedAutoMergeColumnVisibleV2(t *testing.T) {
	page := spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{
				Columns: []spec.ColumnSpec{{Key: "name"}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.listView.columns[0].visible",
		MergedValue: json.RawMessage(`true`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	require.NoError(t, err)
	assert.True(t, page.Resource.ListView.Columns[0].Visible)
}

func TestApplyIndexedAutoMergeColumnSortableV2(t *testing.T) {
	page := spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{
				Columns: []spec.ColumnSpec{{Key: "name"}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.listView.columns[0].sortable",
		MergedValue: json.RawMessage(`true`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	require.NoError(t, err)
	assert.True(t, page.Resource.ListView.Columns[0].Sortable)
}

func TestApplyIndexedAutoMergeColumnFilterableV2(t *testing.T) {
	page := spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{
				Columns: []spec.ColumnSpec{{Key: "name"}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.listView.columns[0].filterable",
		MergedValue: json.RawMessage(`true`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	require.NoError(t, err)
	assert.True(t, page.Resource.ListView.Columns[0].Filterable)
}

func TestApplyIndexedAutoMergeColumnUnsupportedLeafV2(t *testing.T) {
	page := spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{
				Columns: []spec.ColumnSpec{{Key: "name"}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.listView.columns[0].unsupported",
		MergedValue: json.RawMessage(`"value"`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported column auto-merge field")
}

func TestApplyIndexedAutoMergeColumnOutOfRangeV2(t *testing.T) {
	page := spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{
				Columns: []spec.ColumnSpec{{Key: "name"}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.listView.columns[5].title",
		MergedValue: json.RawMessage(`{"zh":"x"}`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestApplyIndexedAutoMergeColumnNilResourceV2(t *testing.T) {
	page := spec.PageSpec{}
	item := dashboardmerge.MergeItem{
		Field:       "resource.listView.columns[0].title",
		MergedValue: json.RawMessage(`{"zh":"x"}`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

// DetailView fields
func TestApplyIndexedAutoMergeDetailFieldTitleV2(t *testing.T) {
	page := spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			DetailView: &spec.DetailViewSpec{
				Fields: []spec.DetailFieldSpec{{Key: "name"}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.detailView.fields[0].title",
		MergedValue: json.RawMessage(`{"zh":"名称"}`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	require.NoError(t, err)
	assert.Equal(t, spec.LocalizedText{"zh": "名称"}, page.Resource.DetailView.Fields[0].Title)
}

func TestApplyIndexedAutoMergeDetailFieldSpanV2(t *testing.T) {
	page := spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			DetailView: &spec.DetailViewSpec{
				Fields: []spec.DetailFieldSpec{{Key: "name"}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.detailView.fields[0].span",
		MergedValue: json.RawMessage(`12`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	require.NoError(t, err)
	assert.Equal(t, 12, page.Resource.DetailView.Fields[0].Span)
}

func TestApplyIndexedAutoMergeDetailFieldVisibleV2(t *testing.T) {
	page := spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			DetailView: &spec.DetailViewSpec{
				Fields: []spec.DetailFieldSpec{{Key: "name"}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.detailView.fields[0].visible",
		MergedValue: json.RawMessage(`true`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	require.NoError(t, err)
	assert.True(t, page.Resource.DetailView.Fields[0].Visible)
}

func TestApplyIndexedAutoMergeDetailFieldUnsupportedLeafV2(t *testing.T) {
	page := spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			DetailView: &spec.DetailViewSpec{
				Fields: []spec.DetailFieldSpec{{Key: "name"}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.detailView.fields[0].unsupported",
		MergedValue: json.RawMessage(`"value"`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported detail auto-merge field")
}

func TestApplyIndexedAutoMergeDetailFieldOutOfRangeV2(t *testing.T) {
	page := spec.PageSpec{
		Resource: &spec.ResourcePageSpec{
			DetailView: &spec.DetailViewSpec{
				Fields: []spec.DetailFieldSpec{{Key: "name"}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "resource.detailView.fields[5].title",
		MergedValue: json.RawMessage(`{"zh":"x"}`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

// Operation form fields
func TestApplyIndexedAutoMergeOperationFormFieldLabelV2(t *testing.T) {
	page := spec.PageSpec{
		Operation: &spec.OperationPageSpec{
			Form: &spec.FormPresentationSpec{
				Fields: []spec.FormFieldSpec{{Key: "name"}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "operation.form.fields[0].label",
		MergedValue: json.RawMessage(`{"zh":"标签"}`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	require.NoError(t, err)
	assert.Equal(t, spec.LocalizedText{"zh": "标签"}, page.Operation.Form.Fields[0].Label)
}

func TestApplyIndexedAutoMergeOperationFormFieldOutOfRangeV2(t *testing.T) {
	page := spec.PageSpec{
		Operation: &spec.OperationPageSpec{
			Form: &spec.FormPresentationSpec{
				Fields: []spec.FormFieldSpec{{Key: "name"}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "operation.form.fields[5].label",
		MergedValue: json.RawMessage(`{"zh":"x"}`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

// Task form fields
func TestApplyIndexedAutoMergeTaskFormFieldV2(t *testing.T) {
	page := spec.PageSpec{
		Task: &spec.TaskPageSpec{
			Form: &spec.FormPresentationSpec{
				Fields: []spec.FormFieldSpec{{Key: "param"}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "task.form.fields[0].placeholder",
		MergedValue: json.RawMessage(`{"zh":"占位"}`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	require.NoError(t, err)
	assert.Equal(t, spec.LocalizedText{"zh": "占位"}, page.Task.Form.Fields[0].Placeholder)
}

func TestApplyIndexedAutoMergeTaskFormFieldOutOfRangeV2(t *testing.T) {
	page := spec.PageSpec{
		Task: &spec.TaskPageSpec{
			Form: &spec.FormPresentationSpec{
				Fields: []spec.FormFieldSpec{{Key: "param"}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "task.form.fields[5].label",
		MergedValue: json.RawMessage(`{"zh":"x"}`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

// Report query form fields
func TestApplyIndexedAutoMergeReportQueryFormFieldV2(t *testing.T) {
	page := spec.PageSpec{
		Report: &spec.ReportPageSpec{
			QueryForm: &spec.FormPresentationSpec{
				Fields: []spec.FormFieldSpec{{Key: "date"}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "report.queryForm.fields[0].description",
		MergedValue: json.RawMessage(`{"zh":"描述"}`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	require.NoError(t, err)
	assert.Equal(t, spec.LocalizedText{"zh": "描述"}, page.Report.QueryForm.Fields[0].Description)
}

func TestApplyIndexedAutoMergeReportQueryFormFieldOutOfRangeV2(t *testing.T) {
	page := spec.PageSpec{
		Report: &spec.ReportPageSpec{
			QueryForm: &spec.FormPresentationSpec{
				Fields: []spec.FormFieldSpec{{Key: "date"}},
			},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "report.queryForm.fields[5].label",
		MergedValue: json.RawMessage(`{"zh":"x"}`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

// Report charts
func TestApplyIndexedAutoMergeReportChartTitleV2(t *testing.T) {
	page := spec.PageSpec{
		Report: &spec.ReportPageSpec{
			Charts: []spec.ChartSpec{{Type: "line"}},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "report.charts[0].title",
		MergedValue: json.RawMessage(`{"zh":"图表"}`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	require.NoError(t, err)
	assert.Equal(t, spec.LocalizedText{"zh": "图表"}, page.Report.Charts[0].Title)
}

func TestApplyIndexedAutoMergeReportChartUnsupportedLeafV2(t *testing.T) {
	page := spec.PageSpec{
		Report: &spec.ReportPageSpec{
			Charts: []spec.ChartSpec{{Type: "line"}},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "report.charts[0].unsupported",
		MergedValue: json.RawMessage(`"value"`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported chart auto-merge field")
}

func TestApplyIndexedAutoMergeReportChartOutOfRangeV2(t *testing.T) {
	page := spec.PageSpec{
		Report: &spec.ReportPageSpec{
			Charts: []spec.ChartSpec{{Type: "line"}},
		},
	}
	item := dashboardmerge.MergeItem{
		Field:       "report.charts[5].title",
		MergedValue: json.RawMessage(`{"zh":"x"}`),
	}
	handled, err := applyIndexedAutoMergeItem(&page, item)
	assert.True(t, handled)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

// parseIndexedMergeField edge cases
func TestParseIndexedMergeFieldInvalidFormatV2(t *testing.T) {
	ok, index, leaf := parseIndexedMergeField("resource.listView.columns[].title", "resource.listView.columns")
	assert.True(t, ok)
	assert.Equal(t, -1, index)
	assert.Equal(t, "", leaf)
}

func TestParseIndexedMergeFieldNonNumericIndexV2(t *testing.T) {
	ok, index, leaf := parseIndexedMergeField("resource.listView.columns[abc].title", "resource.listView.columns")
	assert.True(t, ok)
	assert.Equal(t, -1, index)
	assert.Equal(t, "", leaf)
}

func TestParseIndexedMergeFieldNoDotAfterBracketV2(t *testing.T) {
	ok, index, leaf := parseIndexedMergeField("resource.listView.columns[0]", "resource.listView.columns")
	assert.True(t, ok)
	assert.Equal(t, -1, index)
	assert.Equal(t, "", leaf)
}

func TestParseIndexedMergeFieldNotAPrefixV2(t *testing.T) {
	ok, index, leaf := parseIndexedMergeField("other.field[0].title", "resource.listView.columns")
	assert.False(t, ok)
	assert.Equal(t, 0, index)
	assert.Equal(t, "", leaf)
}
