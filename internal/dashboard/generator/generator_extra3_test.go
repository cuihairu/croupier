package generator

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func extraCollectionContract() *model.FunctionContract {
	return &model.FunctionContract{
		FunctionID: "player.list",
		Capability: dbenum.CapabilityCollectionQuery,
		InputSchema: model.JSON(`{
			"type":"object",
			"properties":{
				"page":{"type":"integer"},
				"pageSize":{"type":"integer"},
				"status":{"type":"string"}
			}
		}`),
		OutputSchema: model.JSON(`{
			"type":"object",
			"properties":{
				"items":{"type":"array","items":{"type":"object","properties":{
					"player_id":{"type":"string"},"level":{"type":"integer"}
				}}},
				"total":{"type":"integer"}
			}
		}`),
	}
}

func TestExtraResourceOutputAssignments(t *testing.T) {
	semantics := &model.CapabilitySemantics{
		PageFieldName:     "page",
		PageSizeFieldName: "pageSize",
		ItemsFieldName:    "items",
		TotalFieldName:    "total",
	}

	assert.Nil(t, resourceOutputAssignments(nil, spec.BindingUsageQuery, semantics))
	assert.Nil(t, resourceOutputAssignments(&model.FunctionContract{}, spec.BindingUsageQuery, semantics))

	detail := resourceOutputAssignments(extraCollectionContract(), spec.BindingUsageDetail, semantics)
	require.Len(t, detail, 1)
	assert.Equal(t, "detail", detail[0].StateKey)

	result := resourceOutputAssignments(extraCollectionContract(), spec.BindingUsageAction, semantics)
	require.Len(t, result, 1)
	assert.Equal(t, "result", result[0].StateKey)

	query := resourceOutputAssignments(extraCollectionContract(), spec.BindingUsageQuery, semantics)
	require.Len(t, query, 2)
	assert.Equal(t, "/items", query[0].Source)
	assert.Equal(t, "/total", query[1].Source)

	// Total field configured but missing from schema keeps the derived source.
	noTotal := &model.CapabilitySemantics{ItemsFieldName: "items", TotalFieldName: "missing"}
	query = resourceOutputAssignments(extraCollectionContract(), spec.BindingUsageQuery, noTotal)
	require.Len(t, query, 2)
	assert.Equal(t, "/total", query[1].Source)

	// Output without any array yields nothing for query usage.
	bare := &model.FunctionContract{OutputSchema: model.JSON(`{"type":"object","properties":{}}`)}
	assert.Nil(t, resourceOutputAssignments(bare, spec.BindingUsageQuery, nil))
}

func TestExtraApplySelectors(t *testing.T) {
	schema := spec.JSONSchema(`{"type":"object","properties":{"page":{"type":"integer"},"pageSize":{"type":"integer"},"id":{"type":"string"}}}`)

	rowApplied := applyIdentityRowSelector(spec.DefaultSelector(schema), &model.CapabilitySemantics{IdentityField: "id"})
	for _, assignment := range rowApplied.Assignments {
		if assignment.Target == "/id" {
			assert.Equal(t, spec.SourceRow, assignment.Source.Kind)
		}
	}
	unchanged := applyIdentityRowSelector(spec.DefaultSelector(schema), nil)
	for _, assignment := range unchanged.Assignments {
		if assignment.Target == "/id" {
			assert.Equal(t, spec.SourceForm, assignment.Source.Kind)
		}
	}

	pageApplied := applyCollectionQuerySelector(spec.DefaultSelector(schema),
		&model.CapabilitySemantics{PageFieldName: "page", PageSizeFieldName: "pageSize"})
	seenPage := false
	seenSize := false
	for _, assignment := range pageApplied.Assignments {
		if assignment.Target == "/page" {
			assert.Equal(t, "/current", assignment.Source.Path)
			seenPage = true
		}
		if assignment.Target == "/pageSize" {
			assert.Equal(t, "/pageSize", assignment.Source.Path)
			seenSize = true
		}
	}
	assert.True(t, seenPage && seenSize)
}

func TestExtraBuildListViewFromContract(t *testing.T) {
	semantics := &model.CapabilitySemantics{
		IdentityField:     "player_id",
		PageFieldName:     "page",
		PageSizeFieldName: "pageSize",
		ItemsFieldName:    "items",
		TotalFieldName:    "total",
	}
	list := buildListViewFromContract(extraCollectionContract(), semantics)
	require.NotNil(t, list)
	assert.Equal(t, "player_id", list.IdentityKey)
	require.Len(t, list.Columns, 2)
	byKey := map[string]spec.ColumnSpec{}
	for _, col := range list.Columns {
		byKey[col.Key] = col
	}
	assert.Equal(t, "left", byKey["player_id"].Fixed, "identity column should be fixed")
	assert.NotNil(t, list.Pagination)
	require.Len(t, list.Filters, 1)
	assert.Equal(t, "status", list.Filters[0].Key)

	// Fallbacks return an empty default list view.
	assert.NotNil(t, buildListViewFromContract(nil, semantics))
	assert.NotNil(t, buildListViewFromContract(&model.FunctionContract{}, semantics))
	noItems := &model.FunctionContract{
		InputSchema:  []byte(`{"type":"object","properties":{}}`),
		OutputSchema: []byte(`{"type":"object","properties":{}}`),
	}
	assert.Empty(t, buildListViewFromContract(noItems, semantics).Columns)
}

// TestColumnFieldSemanticsInference：D5 输出侧 x-* hints 机械派生列呈现。
// identity 坍缩固定左列+复制、x-enum-options 坍缩枚举标签、x-field-unit 提供
// 单位后缀；无 hints 列零变化；非法 role 静默忽略。
func TestColumnFieldSemanticsInference(t *testing.T) {
	semantics := &model.CapabilitySemantics{
		IdentityField:     "",
		PageFieldName:     "page",
		PageSizeFieldName: "pageSize",
		ItemsFieldName:    "items",
		TotalFieldName:    "total",
	}
	contract := &model.FunctionContract{
		FunctionID: "player.list",
		Capability: dbenum.CapabilityCollectionQuery,
		OutputSchema: model.JSON(`{
			"type":"object",
			"properties":{
				"items":{"type":"array","items":{"type":"object","properties":{
					"player_id":{"type":"string","x-field-role":"identity"},
					"status":{"type":"string","x-field-role":"status","x-enum-options":[
						{"value":"active","label":"在线"},{"value":"offline","label":{"en-US":"Offline"}}]},
					"coins":{"type":"number","x-field-role":"amount","x-field-unit":"金币"},
					"expire_at":{"type":"string","format":"date-time","x-field-role":"expiry"},
					"plain":{"type":"string"},
					"bad_role":{"type":"string","x-field-role":"not-a-role"}
				}}}
			}
		}`),
	}
	list := buildListViewFromContract(contract, semantics)
	require.NotNil(t, list)
	byKey := map[string]spec.ColumnSpec{}
	for _, col := range list.Columns {
		byKey[col.Key] = col
	}

	// identity → Fixed left + Render copy
	assert.Equal(t, "left", byKey["player_id"].Fixed)
	assert.Equal(t, "copy", byKey["player_id"].Render)

	// x-enum-options → Enum + Render tag
	status := byKey["status"]
	assert.Equal(t, "tag", status.Render)
	require.Len(t, status.Enum, 2)
	assert.Equal(t, "在线", status.Enum[0].Label["zh-CN"])
	assert.Equal(t, "Offline", status.Enum[1].Label["en-US"])

	// x-field-unit → Unit 双语后缀
	coins := byKey["coins"]
	assert.Equal(t, "金币", coins.Unit["zh-CN"])
	assert.Equal(t, "金币", coins.Unit["en-US"])

	// 无 hints 列零变化
	plain := byKey["plain"]
	assert.Empty(t, plain.Fixed)
	assert.Empty(t, plain.Render)
	assert.Nil(t, plain.Unit)
	assert.Nil(t, plain.Enum)

	// 非法 role 静默忽略（无 Fixed/Render 派生）
	bad := byKey["bad_role"]
	assert.Empty(t, bad.Fixed)
	assert.Empty(t, bad.Render)
	assert.Nil(t, bad.Unit)
}

func TestExtraBuildInlineResourceActionSelector(t *testing.T) {
	singleField := &model.FunctionContract{
		FunctionID: "player.ban",
		InputSchema: model.JSON(`{
			"type":"object","required":["player_id"],
			"properties":{"player_id":{"type":"string"}}
		}`),
	}
	identityPlus := &model.FunctionContract{
		FunctionID: "player.warn",
		InputSchema: model.JSON(`{
			"type":"object","required":["player_id","reason"],
			"properties":{"player_id":{"type":"string"},"reason":{"type":"string"}}
		}`)}
	arrayInput := &model.FunctionContract{
		FunctionID: "player.bulkBan",
		InputSchema: model.JSON(`{
			"type":"object","required":["ids"],
			"properties":{"ids":{"type":"array"}}
		}`)}
	noRequired := &model.FunctionContract{
		FunctionID:   "player.refresh",
		InputSchema:  model.JSON(`{"type":"object","properties":{}}`),
		OutputSchema: model.JSON(`{"type":"object"}`),
	}
	semantics := &model.CapabilitySemantics{IdentityField: "player_id"}

	t.Run("resource_item single required → row", func(t *testing.T) {
		selector, placement, ok := buildInlineResourceActionSelector(singleField, semantics,
			resourceActionSemantic{Subject: "resource_item", IdentityInput: "/player_id"})
		require.True(t, ok)
		assert.Equal(t, "row", placement)
		assert.NotEmpty(t, selector.Assignments)
	})

	t.Run("resource_item identity plus fields → row-form", func(t *testing.T) {
		selector, placement, ok := buildInlineResourceActionSelector(identityPlus, semantics,
			resourceActionSemantic{Subject: "resource_item", IdentityInput: "/player_id"})
		require.True(t, ok)
		assert.Equal(t, "row-form", placement)
		assert.NotEmpty(t, selector.Assignments)
	})

	t.Run("resource_item failures", func(t *testing.T) {
		_, _, ok := buildInlineResourceActionSelector(singleField, semantics,
			resourceActionSemantic{Subject: "resource_item"})
		assert.False(t, ok)

		_, _, ok = buildInlineResourceActionSelector(identityPlus, semantics,
			resourceActionSemantic{Subject: "resource_item", IdentityInput: "/ghost"})
		assert.False(t, ok)
	})

	t.Run("resource_selection batch", func(t *testing.T) {
		selector, placement, ok := buildInlineResourceActionSelector(arrayInput, semantics,
			resourceActionSemantic{Subject: "resource_selection", IdentityInput: "/ids"})
		require.True(t, ok)
		assert.Equal(t, "batch", placement)
		assert.Equal(t, spec.SourceSelection, selector.Assignments[0].Source.Kind)

		_, _, ok = buildInlineResourceActionSelector(arrayInput, &model.CapabilitySemantics{},
			resourceActionSemantic{Subject: "resource_selection", IdentityInput: "/ids"})
		assert.False(t, ok)
	})

	t.Run("none toolbar and unknown subject", func(t *testing.T) {
		_, placement, ok := buildInlineResourceActionSelector(noRequired, semantics,
			resourceActionSemantic{Subject: "none"})
		require.True(t, ok)
		assert.Equal(t, "toolbar", placement)

		_, _, ok = buildInlineResourceActionSelector(singleField, semantics,
			resourceActionSemantic{Subject: "teleport"})
		assert.False(t, ok)
	})
}

func TestExtraBuildInlineResourceActions(t *testing.T) {
	semantics := &model.CapabilitySemantics{
		IdentityField: "player_id",
		Actions: model.JSON(`[
			{"functionId":"player.ban","subject":"resource_item","identityInput":"/player_id"},
			{"functionId":"player.missing","subject":"none"},
			{"functionId":"player.warn","subject":"resource_item","identityInput":"/player_id"},
			{"functionId":"player.bulkBan","subject":"resource_selection","identityInput":"/ids"},
			{"functionId":"player.refresh","subject":"none"}
		]`),
	}
	contracts := []*model.FunctionContract{
		{
			FunctionID: "player.ban",
			Risk:       dbenum.RiskHigh,
			InputSchema: model.JSON(`{
				"type":"object","required":["player_id"],
				"properties":{"player_id":{"type":"string"}}
			}`),
		},
		{
			FunctionID: "player.warn",
			Summary:    map[string]interface{}{"zh-CN": "警告玩家"},
			InputSchema: model.JSON(`{
				"type":"object","required":["player_id","reason"],
				"properties":{"player_id":{"type":"string"},"reason":{"type":"string"}}
			}`),
		},
		{
			FunctionID: "player.bulkBan",
			InputSchema: model.JSON(`{
				"type":"object","required":["ids"],
				"properties":{"ids":{"type":"array"}}
			}`),
		},
		{
			FunctionID:   "player.refresh",
			InputSchema:  model.JSON(`{"type":"object","properties":{}}`),
			OutputSchema: model.JSON(`{"type":"object","properties":{"ok":{"type":"boolean"}}}`),
		},
	}

	row, batch, toolbar, bindings, diags := buildInlineResourceActions(semantics, contracts, "zh-CN")

	codes := map[string]int{}
	for _, d := range diags {
		codes[d.Code]++
	}
	assert.Equal(t, 1, codes["resource_action_contract_missing"])

	assert.Len(t, row, 2)
	assert.Len(t, batch, 1)
	assert.Len(t, toolbar, 1)
	assert.Len(t, bindings, 4)

	// The high-risk action must render as danger with confirmation.
	var banAction *spec.ActionSpec
	for i := range row {
		if row[i].BindingID == inlineActionBindingID(contracts[0]) {
			banAction = &row[i]
		}
	}
	require.NotNil(t, banAction)
	assert.Equal(t, "danger", banAction.Type)
	assert.True(t, banAction.Confirm)
	assert.Nil(t, banAction.Form)

	// The identity-plus-field action carries a form presentation.
	var warnAction *spec.ActionSpec
	for i := range row {
		if row[i].BindingID == inlineActionBindingID(contracts[1]) {
			warnAction = &row[i]
		}
	}
	require.NotNil(t, warnAction)
	assert.NotNil(t, warnAction.Form)

	// Empty semantics produce no actions at all.
	row2, batch2, toolbar2, bindings2, diags2 :=
		buildInlineResourceActions(&model.CapabilitySemantics{}, contracts, "zh-CN")
	assert.Nil(t, row2)
	assert.Nil(t, batch2)
	assert.Nil(t, toolbar2)
	assert.Nil(t, bindings2)
	assert.Nil(t, diags2)
}

func TestExtraBuildTaskStartBindingAndGenerateTaskPage(t *testing.T) {
	op := spec.OperationSpec{
		FunctionID:  "task.start",
		ResourceKey: "task",
		Operation:   "start",
		Capability:  spec.CapabilityTask,
	}
	taskSemantic := spec.TaskSemantic{
		Start: spec.FunctionRef{FunctionID: "task.start"},
		TaskID: spec.TaskIDSemantic{
			ResultPath: "/taskId",
		},
		Status: spec.TaskStatusSemantic{
			Function:    spec.FunctionRef{FunctionID: "task.status"},
			TaskIDInput: "/taskId",
			StatePath:   "/state",
		},
		Events: &spec.TaskEventsSemantic{
			Function:    spec.FunctionRef{FunctionID: "task.events"},
			TaskIDInput: "/taskId",
			EventsPath:  "/events",
		},
		Result: &spec.TaskResultSemantic{
			Function:    spec.FunctionRef{FunctionID: "task.result"},
			TaskIDInput: "/taskId",
			ResultPath:  "/result",
		},
		Cancel: &spec.TaskCommandSemantic{
			Function:    spec.FunctionRef{FunctionID: "task.cancel"},
			TaskIDInput: "/taskId",
		},
	}
	opts := DefaultGenerateOptions()
	opts.Functions = extraTaskFunctions()
	opts.TaskSemantics = map[string]spec.TaskSemantic{"task.start": taskSemantic}

	generated := GenerateTaskPageForOperation(op, opts)
	require.NotNil(t, generated.Task)
	require.NotNil(t, generated.Task.TaskView)

	stateKeys := map[string]bool{}
	for _, binding := range generated.Bindings {
		if binding.Selectors == nil {
			continue
		}
		for _, out := range binding.Selectors.Output {
			stateKeys[out.StateKey] = true
		}
	}
	for _, want := range []string{"taskId", "taskStatus", "taskEvents", "taskResult"} {
		assert.True(t, stateKeys[want], "expected state key %s in %v", want, stateKeys)
	}

	// Without TaskID result path the start binding has no selectors output.
	opNoPath := op
	opts.TaskSemantics = map[string]spec.TaskSemantic{"task.start": {
		Start:  spec.FunctionRef{FunctionID: "task.start"},
		Status: spec.TaskStatusSemantic{Function: spec.FunctionRef{FunctionID: "task.status"}},
	}}
	generated = GenerateTaskPageForOperation(opNoPath, opts)
	require.NotNil(t, generated)
}

var _ = json.Marshal
