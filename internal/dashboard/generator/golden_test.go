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
	"gorm.io/gorm"
)

// TestGenerateOperationPageGolden tests that the generator produces stable output
// for the same input. This ensures that repeated generation with the same input
// produces byte-identical results.
func TestGenerateOperationPageGolden(t *testing.T) {
	tests := []struct {
		name     string
		op       spec.OperationSpec
		opts     GenerateOptions
		wantType spec.PageType
	}{
		{
			name: "sync operation",
			op: spec.OperationSpec{
				FunctionID:  "mail.send",
				ResourceKey: "mail",
				Operation:   "send",
				Capability:  spec.CapabilityAction,
				Execution:   spec.FunctionExecutionSync,
				Enabled:     true,
			},
			opts:     DefaultGenerateOptions(),
			wantType: spec.PageTypeOperation,
		},
		{
			name: "task operation",
			op: spec.OperationSpec{
				FunctionID:  "reward.batch_grant",
				ResourceKey: "reward",
				Operation:   "batch_grant",
				Capability:  spec.CapabilityTask,
				Execution:   spec.FunctionExecutionTask,
				Enabled:     true,
			},
			opts:     DefaultGenerateOptions(),
			wantType: spec.PageTypeTask,
		},
		{
			name: "report operation",
			op: spec.OperationSpec{
				FunctionID:  "analytics.retention",
				ResourceKey: "analytics",
				Operation:   "retention",
				Capability:  spec.CapabilityReport,
				Execution:   spec.FunctionExecutionSync,
				Enabled:     true,
			},
			opts:     DefaultGenerateOptions(),
			wantType: spec.PageTypeReport,
		},
		{
			name: "high risk operation",
			op: spec.OperationSpec{
				FunctionID:  "player.ban",
				ResourceKey: "player",
				Operation:   "ban",
				Capability:  spec.CapabilityAction,
				Execution:   spec.FunctionExecutionSync,
				Risk:        spec.RiskHigh,
				Enabled:     true,
			},
			opts:     DefaultGenerateOptions(),
			wantType: spec.PageTypeOperation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate twice to verify stability
			result1 := GenerateForOperation(tt.op, tt.opts)
			result2 := GenerateForOperation(tt.op, tt.opts)

			// Verify type
			assert.Equal(t, tt.wantType, result1.Type)

			// Verify stability - byte-identical JSON
			json1, err := json.Marshal(result1)
			require.NoError(t, err)

			json2, err := json.Marshal(result2)
			require.NoError(t, err)

			assert.Equal(t, string(json1), string(json2),
				"generator should produce identical output for same input")

			// Verify required fields
			assert.NotEmpty(t, result1.PageKey)
			assert.NotEmpty(t, result1.Title)
			assert.NotEmpty(t, result1.Bindings)
			assert.NotNil(t, pageShape(result1.PageSpec))
		})
	}
}

// TestGenerateResourcePageGolden tests resource page generation stability.
func TestGenerateResourcePageGolden(t *testing.T) {
	collection := &model.FunctionContract{
		Model:        gormModelWithID(101),
		FunctionID:   "player.list",
		ResourceKey:  "player",
		Capability:   dbenum.CapabilityCollectionQuery,
		Execution:    string(spec.FunctionExecutionSync),
		Enabled:      true,
		InputSchema:  model.JSON(`{"type":"object","properties":{"page":{"type":"integer"},"page_size":{"type":"integer"}}}`),
		OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"}}}},"total":{"type":"integer"}}}`),
	}
	semantics := &model.CapabilitySemantics{
		ResourceKey:       "player",
		CollectionQueryID: collection.ID,
		IdentityField:     "id",
		ItemsFieldName:    "items",
		TotalFieldName:    "total",
		PageFieldName:     "page",
		PageSizeFieldName: "page_size",
	}

	first, ok := GenerateResourcePageProposal(semantics, []*model.FunctionContract{collection}, DefaultGenerateOptions())
	require.True(t, ok)
	second, ok := GenerateResourcePageProposal(semantics, []*model.FunctionContract{collection}, DefaultGenerateOptions())
	require.True(t, ok)

	firstJSON, err := json.Marshal(first)
	require.NoError(t, err)
	secondJSON, err := json.Marshal(second)
	require.NoError(t, err)
	assert.Equal(t, string(firstJSON), string(secondJSON))
	assert.Equal(t, spec.PageTypeResource, first.Type)
	assert.Equal(t, spec.GeneratedPageQualityReady, first.Quality)
	require.NotNil(t, first.Resource)
	require.NotNil(t, first.Resource.ListView)
	assert.Equal(t, "id", first.Resource.ListView.IdentityKey)
	assert.Len(t, first.Bindings, 1)
	assert.Equal(t, spec.BindingUsageQuery, first.Bindings[0].Usage)
	assert.Nil(t, first.Resource.CreateForm)
	assert.Nil(t, first.Resource.UpdateForm)
	assert.Nil(t, first.Resource.DeleteAction)
}

func TestGenerateResourcePageCRUDGovernance(t *testing.T) {
	collection := &model.FunctionContract{Model: gormModelWithID(201), FunctionID: "player.list", ResourceKey: "player", Capability: dbenum.CapabilityCollectionQuery, Enabled: true,
		OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"}}}}}}`)}
	create := &model.FunctionContract{Model: gormModelWithID(202), FunctionID: "player.create", ResourceKey: "player", Capability: dbenum.CapabilityCreate, Enabled: true,
		InputSchema: model.JSON(`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`)}
	update := &model.FunctionContract{Model: gormModelWithID(203), FunctionID: "player.update", ResourceKey: "player", Capability: dbenum.CapabilityUpdate, Enabled: true,
		InputSchema: model.JSON(`{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"}},"required":["id","name"]}`)}
	deleteContract := &model.FunctionContract{Model: gormModelWithID(204), FunctionID: "player.delete", ResourceKey: "player", Capability: dbenum.CapabilityDelete, Enabled: true, Risk: dbenum.RiskDanger, Permission: "player:delete",
		Approval: datatypes.JSONMap{"required": true}, InputSchema: model.JSON(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`)}
	semantics := &model.CapabilitySemantics{ResourceKey: "player", CollectionQueryID: collection.ID, CreateID: create.ID, UpdateID: update.ID, DeleteID: deleteContract.ID, IdentityField: "id", ItemsFieldName: "items"}

	generated, ok := GenerateResourcePageProposal(semantics, []*model.FunctionContract{collection, create, update, deleteContract}, DefaultGenerateOptions())
	require.True(t, ok)
	require.NotNil(t, generated.Resource)
	require.NotNil(t, generated.Resource.CreateForm)
	require.NotNil(t, generated.Resource.UpdateForm)
	assert.NotContains(t, string(generated.Resource.UpdateForm.JSONSchema), `"id"`)
	require.NotNil(t, generated.Resource.DeleteAction)
	assert.Equal(t, "danger", generated.Resource.DeleteAction.Risk)
	assert.Equal(t, "player:delete", generated.Resource.DeleteAction.Permission)
	for _, binding := range generated.Bindings {
		if binding.ID == "update" {
			assertSelectorAssignment(t, binding.Selectors.Input, "/id", spec.SourceRow, "/id")
		}
		if binding.ID == "delete" {
			assert.True(t, binding.Execution.RequireConfirm)
			assertSelectorAssignment(t, binding.Selectors.Input, "/id", spec.SourceRow, "/id")
		}
	}
}

func TestGenerateResourcePageDetailSources(t *testing.T) {
	collection := &model.FunctionContract{Model: gormModelWithID(301), FunctionID: "player.list", ResourceKey: "player", Capability: dbenum.CapabilityCollectionQuery, Enabled: true,
		OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"}}}}}}`)}
	item := &model.FunctionContract{Model: gormModelWithID(302), FunctionID: "player.get", ResourceKey: "player", Capability: dbenum.CapabilityItemQuery, Enabled: true,
		InputSchema:  model.JSON(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`),
		OutputSchema: model.JSON(`{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"},"level":{"type":"integer"}}}`)}
	semantics := &model.CapabilitySemantics{ResourceKey: "player", CollectionQueryID: collection.ID, ItemQueryID: item.ID, IdentityField: "id", ItemsFieldName: "items"}

	generated, ok := GenerateResourcePageProposal(semantics, []*model.FunctionContract{collection, item}, DefaultGenerateOptions())
	require.True(t, ok)
	require.NotNil(t, generated.Resource)
	require.NotNil(t, generated.Resource.DetailView)
	assert.Len(t, generated.Resource.DetailView.Fields, 3)
	assert.Contains(t, bindingIDs(generated.Bindings), "detail")

	semantics.ItemQueryID = 0
	generated, ok = GenerateResourcePageProposal(semantics, []*model.FunctionContract{collection}, DefaultGenerateOptions())
	require.True(t, ok)
	require.NotNil(t, generated.Resource.DetailView)
	assert.Len(t, generated.Resource.DetailView.Fields, 2)
	assert.NotContains(t, bindingIDs(generated.Bindings), "detail")
}

func TestGenerateResourcePageInlineActions(t *testing.T) {
	collection := &model.FunctionContract{
		Model: gormModelWithID(401), FunctionID: "player.list", ResourceKey: "player", Capability: dbenum.CapabilityCollectionQuery, Enabled: true,
		OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"}}}}}}`),
	}
	itemAction := &model.FunctionContract{
		Model: gormModelWithID(402), FunctionID: "player.ban", ResourceKey: "player", OperationKey: "ban", Capability: dbenum.CapabilityAction, Enabled: true,
		InputSchema: model.JSON(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`),
	}
	selectionAction := &model.FunctionContract{
		Model: gormModelWithID(403), FunctionID: "player.notice", ResourceKey: "player", OperationKey: "notice", Capability: dbenum.CapabilityAction, Enabled: true,
		InputSchema: model.JSON(`{"type":"object","properties":{"ids":{"type":"array","items":{"type":"string"}}},"required":["ids"]}`),
	}
	toolbarAction := &model.FunctionContract{
		Model: gormModelWithID(404), FunctionID: "player.refresh", ResourceKey: "player", OperationKey: "refresh", Capability: dbenum.CapabilityAction, Enabled: true,
		InputSchema: model.JSON(`{"type":"object","properties":{"force":{"type":"boolean"}}}`),
	}
	semantics := &model.CapabilitySemantics{
		ResourceKey: "player", CollectionQueryID: collection.ID, IdentityField: "id", ItemsFieldName: "items",
		Actions: model.JSON(`[
			{"functionId":"player.ban","subject":"resource_item","identityInput":"/id"},
			{"functionId":"player.notice","subject":"resource_selection","identityInput":"/ids"},
			{"functionId":"player.refresh","subject":"none"}
		]`),
	}

	generated, ok := GenerateResourcePageProposal(semantics, []*model.FunctionContract{collection, itemAction, selectionAction, toolbarAction}, DefaultGenerateOptions())
	require.True(t, ok)
	require.NotNil(t, generated.Resource)
	require.Len(t, generated.Resource.ListView.RowActions, 1)
	require.Len(t, generated.Resource.ListView.BatchActions, 1)
	require.Len(t, generated.Resource.ListView.ToolbarActions, 1)
	assert.Equal(t, "action.ban", generated.Resource.ListView.RowActions[0].BindingID)
	assert.Equal(t, "action.notice", generated.Resource.ListView.BatchActions[0].BindingID)
	assert.Equal(t, "action.refresh", generated.Resource.ListView.ToolbarActions[0].BindingID)

	bindings := make(map[string]spec.PageFunctionBinding, len(generated.Bindings))
	for _, binding := range generated.Bindings {
		bindings[binding.ID] = binding
	}
	require.NotNil(t, bindings["action.ban"].Selectors)
	assert.Equal(t, spec.SourceRow, bindings["action.ban"].Selectors.Input.Assignments[0].Source.Kind)
	assert.Equal(t, "/id", bindings["action.ban"].Selectors.Input.Assignments[0].Source.Path)
	require.NotNil(t, bindings["action.notice"].Selectors)
	selectionSource := bindings["action.notice"].Selectors.Input.Assignments[0].Source
	assert.Equal(t, spec.SourceSelection, selectionSource.Kind)
	require.NotNil(t, selectionSource.Transform)
	assert.Equal(t, spec.TransformPick, selectionSource.Transform.Type)
}

func TestGenerateResourcePageUnsafeActionStaysStandalone(t *testing.T) {
	collection := &model.FunctionContract{
		Model: gormModelWithID(411), FunctionID: "player.list", ResourceKey: "player", Capability: dbenum.CapabilityCollectionQuery, Enabled: true,
		OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"}}}}}}`),
	}
	unsafeAction := &model.FunctionContract{
		Model: gormModelWithID(412), FunctionID: "player.ban", ResourceKey: "player", OperationKey: "ban", Capability: dbenum.CapabilityAction, Enabled: true,
		InputSchema: model.JSON(`{"type":"object","properties":{"id":{"type":"string"},"reason":{"type":"string"}},"required":["id","reason"]}`),
	}
	semantics := &model.CapabilitySemantics{
		ResourceKey: "player", CollectionQueryID: collection.ID, IdentityField: "id", ItemsFieldName: "items",
		Actions: model.JSON(`[{"functionId":"player.ban","subject":"resource_item","identityInput":"/id"}]`),
	}

	generated, ok := GenerateResourcePageProposal(semantics, []*model.FunctionContract{collection, unsafeAction}, DefaultGenerateOptions())
	require.True(t, ok)
	// id(必填 identity) + reason(附加字段) 现以内联表单行操作呈现：
	// identity 由 row 注入，reason 弹 SchemaFormRenderer 收集。
	require.Len(t, generated.Resource.ListView.RowActions, 1)
	action := generated.Resource.ListView.RowActions[0]
	assert.Equal(t, "action.ban", action.BindingID)
	require.NotNil(t, action.Form, "identity+fields action must carry a form")
	// 表单剥离了 identity 字段，只留 reason
	assert.NotContains(t, string(action.Form.JSONSchema), "\"id\"")
	assert.Contains(t, string(action.Form.JSONSchema), "\"reason\"")
	// selector 中 id 映射为 row 源
	binding := findBindingByID(generated.Bindings, "action.ban")
	require.NotNil(t, binding)
	for _, assignment := range binding.Selectors.Input.Assignments {
		if assignment.Target == "/id" {
			assert.Equal(t, spec.SourceRow, assignment.Source.Kind)
		}
	}
}

func findBindingByID(bindings []spec.PageFunctionBinding, id string) *spec.PageFunctionBinding {
	for i := range bindings {
		if bindings[i].ID == id {
			return &bindings[i]
		}
	}
	return nil
}

func bindingIDs(bindings []spec.PageFunctionBinding) []string {
	ids := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		ids = append(ids, binding.ID)
	}
	return ids
}

func gormModelWithID(id uint) gorm.Model {
	return gorm.Model{ID: id}
}

// TestGeneratePageKeyStability tests that page keys are deterministic.
func TestGeneratePageKeyStability(t *testing.T) {
	tests := []struct {
		name string
		op   spec.OperationSpec
		opts GenerateOptions
		want string
	}{
		{
			name: "with resource and operation",
			op: spec.OperationSpec{
				FunctionID:  "player.ban",
				ResourceKey: "player",
				Operation:   "ban",
			},
			opts: GenerateOptions{DefaultLocale: "zh-CN"},
			want: "operation--player.ban",
		},
		{
			name: "with only function id",
			op: spec.OperationSpec{
				FunctionID: "mail.send",
			},
			opts: GenerateOptions{DefaultLocale: "zh-CN"},
			want: "operation--mail.send",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateForOperation(tt.op, tt.opts)
			assert.Equal(t, tt.want, result.PageKey)
		})
	}
}

// TestQualityAssessmentGolden tests quality assessment stability.
func TestQualityAssessmentGolden(t *testing.T) {
	tests := []struct {
		name    string
		op      spec.OperationSpec
		wantQ   spec.GeneratedPageQuality
		wantErr bool
	}{
		{
			name: "complete operation - basic quality",
			op: spec.OperationSpec{
				FunctionID:  "player.ban",
				ResourceKey: "player",
				Operation:   "ban",
				Capability:  spec.CapabilityAction,
				Execution:   spec.FunctionExecutionSync,
				Risk:        spec.RiskHigh,
				Permission:  "player.ban.invoke",
				Enabled:     true,
			},
			wantQ: spec.GeneratedPageQualityBasic,
		},
		{
			name: "task without semantics - needs_review",
			op: spec.OperationSpec{
				FunctionID:  "reward.batch_grant",
				ResourceKey: "reward",
				Operation:   "batch_grant",
				Capability:  spec.CapabilityTask,
				Execution:   spec.FunctionExecutionTask,
				Enabled:     true,
			},
			wantQ: spec.GeneratedPageQualityNeedsReview,
		},
		{
			name: "report without semantics - needs_review",
			op: spec.OperationSpec{
				FunctionID:  "analytics.retention",
				ResourceKey: "analytics",
				Operation:   "retention",
				Capability:  spec.CapabilityReport,
				Enabled:     true,
			},
			wantQ: spec.GeneratedPageQualityNeedsReview,
		},
		{
			name: "disabled function - needs review with error diagnostic",
			op: spec.OperationSpec{
				FunctionID: "player.ban",
				Enabled:    false,
			},
			wantQ:   spec.GeneratedPageQualityNeedsReview,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateForOperation(tt.op, DefaultGenerateOptions())
			assert.Equal(t, tt.wantQ, result.Quality)
			if tt.wantErr {
				assertDiagnostic(t, result.Diagnostics, "function_disabled")
			}
		})
	}
}

// TestBindingStability tests that bindings are deterministic.
func TestBindingStability(t *testing.T) {
	op := spec.OperationSpec{
		FunctionID:  "player.ban",
		ResourceKey: "player",
		Operation:   "ban",
		Capability:  spec.CapabilityAction,
		Execution:   spec.FunctionExecutionSync,
		Enabled:     true,
	}

	// Generate multiple times
	for i := 0; i < 10; i++ {
		result := GenerateForOperation(op, DefaultGenerateOptions())

		// Verify binding structure
		require.Len(t, result.Bindings, 1)
		binding := result.Bindings[0]

		assert.NotEmpty(t, binding.ID)
		assert.Equal(t, "player.ban", binding.FunctionID)
		assert.Equal(t, spec.BindingUsageAction, binding.Usage)
		assert.Equal(t, spec.PageExecutionModeSync, binding.Execution.Mode)
	}
}

// TestPageSpecStability tests that generated PageSpec output is deterministic.
func TestPageSpecStability(t *testing.T) {
	op := spec.OperationSpec{
		FunctionID:  "mail.send",
		ResourceKey: "mail",
		Operation:   "send",
		Capability:  spec.CapabilityAction,
		Execution:   spec.FunctionExecutionSync,
		Enabled:     true,
	}

	// Generate multiple times
	var specs []string
	for i := 0; i < 10; i++ {
		result := GenerateForOperation(op, DefaultGenerateOptions())
		raw, err := json.Marshal(result.PageSpec)
		require.NoError(t, err)
		specs = append(specs, string(raw))
	}

	for i := 1; i < len(specs); i++ {
		assert.Equal(t, specs[0], specs[i],
			"PageSpec should be stable across multiple generations")
	}
}

func pageShape(page spec.PageSpec) interface{} {
	switch page.Type {
	case spec.PageTypeResource:
		return page.Resource
	case spec.PageTypeOperation:
		return page.Operation
	case spec.PageTypeTask:
		return page.Task
	case spec.PageTypeReport:
		return page.Report
	default:
		return nil
	}
}
