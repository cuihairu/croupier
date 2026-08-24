package generator

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Task binding builders
// ---------------------------------------------------------------------------

func extraTaskFunctions() map[string]spec.FunctionSpec {
	fn := func(id string) spec.FunctionSpec {
		return spec.FunctionSpec{
			ID:           id,
			InputSchema:  spec.JSONSchema(`{"type":"object","properties":{"taskId":{"type":"string"}},"required":["taskId"]}`),
			OutputSchema: spec.JSONSchema(`{"type":"object","properties":{"state":{"type":"string"}}}`),
		}
	}
	return map[string]spec.FunctionSpec{
		"task.start":  fn("task.start"),
		"task.status": fn("task.status"),
		"task.events": fn("task.events"),
		"task.result": fn("task.result"),
		"task.cancel": fn("task.cancel"),
	}
}

func TestExtraBuildTaskStatusBinding(t *testing.T) {
	fns := extraTaskFunctions()
	stateKey := "banTaskId"

	t.Run("missing function", func(t *testing.T) {
		_, ok, diag := buildTaskStatusBinding(spec.TaskSemantic{
			Status: spec.TaskStatusSemantic{Function: spec.FunctionRef{FunctionID: "ghost"}},
		}, fns, stateKey)
		assert.False(t, ok)
		assert.Equal(t, "task_status_function_missing", diag.Code)
	})

	t.Run("invalid semantic inputs", func(t *testing.T) {
		_, ok, diag := buildTaskStatusBinding(spec.TaskSemantic{
			Status: spec.TaskStatusSemantic{Function: spec.FunctionRef{FunctionID: "task.status"}},
		}, fns, stateKey)
		assert.False(t, ok)
		assert.Equal(t, "task_status_semantic_invalid", diag.Code)
	})

	t.Run("success builds page-state selector", func(t *testing.T) {
		binding, ok, diag := buildTaskStatusBinding(spec.TaskSemantic{
			Status: spec.TaskStatusSemantic{
				Function:    spec.FunctionRef{FunctionID: "task.status"},
				TaskIDInput: "/taskId",
				StatePath:   "/state",
			},
		}, fns, stateKey)
		require.True(t, ok)
		assert.Empty(t, diag.Code)
		assert.Equal(t, "status", binding.ID)
		require.NotNil(t, binding.Selectors)
		require.Len(t, binding.Selectors.Input.Assignments, 1)
		assert.Equal(t, spec.SourcePageState, binding.Selectors.Input.Assignments[0].Source.Kind)
		assert.Equal(t, stateKey, binding.Selectors.Input.Assignments[0].Source.Key)
	})
}

func TestExtraBuildTaskEventsBinding(t *testing.T) {
	fns := extraTaskFunctions()

	t.Run("nil events", func(t *testing.T) {
		_, ok, diag := buildTaskEventsBinding(spec.TaskSemantic{}, fns, "k")
		assert.False(t, ok)
		assert.Empty(t, diag.Code)
	})

	t.Run("missing function", func(t *testing.T) {
		_, ok, diag := buildTaskEventsBinding(spec.TaskSemantic{
			Events: &spec.TaskEventsSemantic{Function: spec.FunctionRef{FunctionID: "ghost"}},
		}, fns, "k")
		assert.False(t, ok)
		assert.Equal(t, "task_events_function_missing", diag.Code)
	})

	t.Run("invalid semantic", func(t *testing.T) {
		_, ok, diag := buildTaskEventsBinding(spec.TaskSemantic{
			Events: &spec.TaskEventsSemantic{Function: spec.FunctionRef{FunctionID: "task.events"}},
		}, fns, "k")
		assert.False(t, ok)
		assert.Equal(t, "task_events_semantic_invalid", diag.Code)
	})

	t.Run("success", func(t *testing.T) {
		binding, ok, _ := buildTaskEventsBinding(spec.TaskSemantic{
			Events: &spec.TaskEventsSemantic{
				Function:    spec.FunctionRef{FunctionID: "task.events"},
				TaskIDInput: "/taskId",
				EventsPath:  "/events",
			},
		}, fns, "k")
		require.True(t, ok)
		assert.Equal(t, "events", binding.ID)
		require.Len(t, binding.Selectors.Output, 1)
		assert.Equal(t, "/events", binding.Selectors.Output[0].Source)
	})
}

func TestExtraBuildTaskResultBinding(t *testing.T) {
	fns := map[string]spec.FunctionSpec{
		"task.result": {
			ID: "task.result",
			OutputSchema: spec.JSONSchema(
				`{"type":"object","properties":{"data":{"type":"array"},"summary":{"type":"object"}}}`),
		},
	}

	t.Run("nil result", func(t *testing.T) {
		_, ok, diag := buildTaskResultBinding(spec.TaskSemantic{}, fns, "k")
		assert.False(t, ok)
		assert.Empty(t, diag.Code)
	})

	t.Run("missing function and invalid semantic", func(t *testing.T) {
		_, ok, diag := buildTaskResultBinding(spec.TaskSemantic{
			Result: &spec.TaskResultSemantic{Function: spec.FunctionRef{FunctionID: "ghost"}},
		}, fns, "k")
		assert.False(t, ok)
		assert.Equal(t, "task_result_function_missing", diag.Code)

		_, ok, diag = buildTaskResultBinding(spec.TaskSemantic{
			Result: &spec.TaskResultSemantic{Function: spec.FunctionRef{FunctionID: "task.result"}},
		}, fns, "k")
		assert.False(t, ok)
		assert.Equal(t, "task_result_semantic_invalid", diag.Code)
	})

	t.Run("success derives shapes from pointer", func(t *testing.T) {
		binding, ok, _ := buildTaskResultBinding(spec.TaskSemantic{
			Result: &spec.TaskResultSemantic{
				Function:    spec.FunctionRef{FunctionID: "task.result"},
				TaskIDInput: "/taskId",
				ResultPath:  "/data",
			},
		}, fns, "k")
		require.True(t, ok)
		assert.Equal(t, spec.OutputShapeCollection, binding.Selectors.Output[0].Shape)

		binding, _, _ = buildTaskResultBinding(spec.TaskSemantic{
			Result: &spec.TaskResultSemantic{
				Function:    spec.FunctionRef{FunctionID: "task.result"},
				TaskIDInput: "/taskId",
				ResultPath:  "/summary",
			},
		}, fns, "k")
		assert.Equal(t, spec.OutputShapeObject, binding.Selectors.Output[0].Shape)

		binding, _, _ = buildTaskResultBinding(spec.TaskSemantic{
			Result: &spec.TaskResultSemantic{
				Function:    spec.FunctionRef{FunctionID: "task.result"},
				TaskIDInput: "/taskId",
				ResultPath:  "/unknown",
			},
		}, fns, "k")
		assert.Equal(t, spec.OutputShapeScalar, binding.Selectors.Output[0].Shape)
	})
}

func TestExtraBuildTaskCancelBinding(t *testing.T) {
	fns := extraTaskFunctions()

	t.Run("nil cancel", func(t *testing.T) {
		_, ok, diag := buildTaskCancelBinding(spec.TaskSemantic{}, fns, "k")
		assert.False(t, ok)
		assert.Empty(t, diag.Code)
	})

	t.Run("missing function and invalid input", func(t *testing.T) {
		_, ok, diag := buildTaskCancelBinding(spec.TaskSemantic{
			Cancel: &spec.TaskCommandSemantic{Function: spec.FunctionRef{FunctionID: "ghost"}},
		}, fns, "k")
		assert.False(t, ok)
		assert.Equal(t, "task_cancel_function_missing", diag.Code)

		_, ok, diag = buildTaskCancelBinding(spec.TaskSemantic{
			Cancel: &spec.TaskCommandSemantic{Function: spec.FunctionRef{FunctionID: "task.cancel"}},
		}, fns, "k")
		assert.False(t, ok)
		assert.Equal(t, "task_cancel_semantic_invalid", diag.Code)
	})

	t.Run("success", func(t *testing.T) {
		binding, ok, _ := buildTaskCancelBinding(spec.TaskSemantic{
			Cancel: &spec.TaskCommandSemantic{
				Function:    spec.FunctionRef{FunctionID: "task.cancel"},
				TaskIDInput: "/taskId",
			},
		}, fns, "k")
		require.True(t, ok)
		assert.Equal(t, "cancel", binding.ID)
		assert.Equal(t, spec.BindingUsageTaskCancel, binding.Usage)
	})
}

// ---------------------------------------------------------------------------
// Pagination resolution / filters
// ---------------------------------------------------------------------------

func TestExtraResolvePaginationFields(t *testing.T) {
	props := func(raw string) map[string]json.RawMessage {
		out := map[string]json.RawMessage{}
		require.NoError(t, json.Unmarshal([]byte(raw), &out))
		return out
	}

	properties := props(`{"pageNo":{"type":"integer"},"limit":{"type":"integer"},"q":{"type":"string"}}`)

	semantics := &model.CapabilitySemantics{PageFieldName: "page", PageSizeFieldName: "page_size"}
	assert.True(t, resolvePaginationFields(properties, semantics))
	assert.Equal(t, "pageNo", semantics.PageFieldName)
	assert.Equal(t, "limit", semantics.PageSizeFieldName)

	assert.False(t, resolvePaginationFields(properties, nil))

	missing := props(`{"q":{"type":"string"}}`)
	assert.False(t, resolvePaginationFields(missing, &model.CapabilitySemantics{}))

	partial := props(`{"page":{"type":"integer"}}`)
	assert.False(t, resolvePaginationFields(partial, &model.CapabilitySemantics{PageFieldName: "page"}))
}

func TestExtraBuildFiltersFromContract(t *testing.T) {
	assert.Nil(t, buildFiltersFromContract(nil, nil))
	assert.Nil(t, buildFiltersFromContract(&model.FunctionContract{}, nil))
	assert.Nil(t, buildFiltersFromContract(&model.FunctionContract{
		InputSchema: []byte(`{"type":"object"}`),
	}, nil))

	contract := &model.FunctionContract{
		InputSchema: []byte(`{
			"type":"object",
			"properties":{
				"page":{"type":"integer"},
				"pageSize":{"type":"integer"},
				"status":{"type":"string","enum":["on","off"]},
				"createdAt":{"type":"string","format":"date"}
			}
		}`),
	}
	semantics := &model.CapabilitySemantics{
		PageFieldName:     "page",
		PageSizeFieldName: "pageSize",
	}
	filters := buildFiltersFromContract(contract, semantics)
	require.Len(t, filters, 2)

	byKey := map[string]spec.FilterSpec{}
	for _, f := range filters {
		byKey[f.Key] = f
	}
	assert.Equal(t, "date", byKey["createdAt"].Type)
	assert.Equal(t, "select", byKey["status"].Type)
	require.Len(t, byKey["status"].Options, 2)
	assert.Equal(t, "on", byKey["status"].Options[0].Value)
}

func TestExtraPaginationFromContract(t *testing.T) {
	contract := &model.FunctionContract{
		InputSchema: []byte(`{"type":"object","properties":{"page":{"type":"integer"},"pageSize":{"type":"integer"}}}`),
	}
	assert.Nil(t, paginationFromContract(nil, nil))
	assert.Nil(t, paginationFromContract(&model.FunctionContract{}, nil))
	assert.Nil(t, paginationFromContract(&model.FunctionContract{}, &model.CapabilitySemantics{}))
	assert.Nil(t, paginationFromContract(&model.FunctionContract{InputSchema: contract.InputSchema}, &model.CapabilitySemantics{}))

	got := paginationFromContract(contract, &model.CapabilitySemantics{PageFieldName: "page", PageSizeFieldName: "pageSize"})
	require.NotNil(t, got)
	assert.True(t, got.Enabled)
}

// ---------------------------------------------------------------------------
// Semantics assessment / view validation
// ---------------------------------------------------------------------------

func TestExtraAssessResourceSemantics(t *testing.T) {
	diags := assessResourceSemantics(nil)
	require.Len(t, diags, 1)
	assert.Equal(t, "resource_semantics_missing", diags[0].Code)

	diags = assessResourceSemantics(&model.CapabilitySemantics{})
	codes := map[string]int{}
	for _, d := range diags {
		codes[d.Code]++
	}
	assert.Equal(t, 1, codes["identity_missing"])
	assert.Equal(t, 1, codes["collection_query_missing"])

	diags = assessResourceSemantics(&model.CapabilitySemantics{
		IdentityField:     "id",
		CollectionQueryID: 3,
		Conflicts:         datatypes.JSON(`[{`),
	})
	require.Len(t, diags, 1)
	assert.Equal(t, "semantic_conflicts_invalid", diags[0].Code)

	diags = assessResourceSemantics(&model.CapabilitySemantics{
		IdentityField:     "id",
		CollectionQueryID: 3,
		Conflicts:         datatypes.JSON(`[{"field":"risk","resolution":""}]`),
	})
	require.Len(t, diags, 1)
	assert.Equal(t, "semantic_conflict_unresolved", diags[0].Code)

	assert.Nil(t, assessResourceSemantics(&model.CapabilitySemantics{
		IdentityField:     "id",
		CollectionQueryID: 3,
	}))
}

func TestExtraValidateGeneratedResourceViews(t *testing.T) {
	assert.Nil(t, validateGeneratedResourceViews(nil, nil, nil))

	listView := &spec.ListViewSpec{Columns: []spec.ColumnSpec{{Key: "name"}}}
	diags := validateGeneratedResourceViews(listView, nil,
		&model.CapabilitySemantics{IdentityField: "player_id"})
	require.Len(t, diags, 1)
	assert.Equal(t, "resource_identity_column_missing", diags[0].Code)

	ok := validateGeneratedResourceViews(&spec.ListViewSpec{Columns: []spec.ColumnSpec{{Key: " player_id "}}}, nil,
		&model.CapabilitySemantics{IdentityField: "player_id"})
	assert.Nil(t, ok)
}

// ---------------------------------------------------------------------------
// Contract lookups / inline actions
// ---------------------------------------------------------------------------

func TestExtraFindContracts(t *testing.T) {
	query := &model.FunctionContract{Model: gorm.Model{ID: 1}, FunctionID: "player.list", Capability: "collection_query"}
	item := &model.FunctionContract{Model: gorm.Model{ID: 2}, FunctionID: "player.get", Capability: "item_query"}
	action := &model.FunctionContract{Model: gorm.Model{ID: 3}, FunctionID: "player.ban", Capability: "action"}

	assert.Nil(t, findResourceContract([]*model.FunctionContract{query}, 0, spec.CapabilityCollectionQuery))
	assert.Nil(t, findResourceContract([]*model.FunctionContract{query}, 9, spec.CapabilityCollectionQuery))
	assert.Nil(t, findResourceContract([]*model.FunctionContract{action}, 3, spec.CapabilityCollectionQuery))
	assert.Same(t, query, findResourceContract([]*model.FunctionContract{query}, 1, spec.CapabilityCollectionQuery))

	assert.Nil(t, findContractByID([]*model.FunctionContract{query}, 5))
	assert.Nil(t, findContractByFunctionID([]*model.FunctionContract{query}, " nope "))
	assert.Nil(t, findContractByFunctionID([]*model.FunctionContract{nil}, "x"))
	assert.Same(t, item, findContractByFunctionID([]*model.FunctionContract{query, item}, "player.get"))
}

func TestExtraParseResourceActionSemantics(t *testing.T) {
	assert.Nil(t, parseResourceActionSemantics(nil))
	assert.Nil(t, parseResourceActionSemantics(&model.CapabilitySemantics{}))
	assert.Nil(t, parseResourceActionSemantics(&model.CapabilitySemantics{
		Actions: datatypes.JSON(`{broken`),
	}))

	actions := parseResourceActionSemantics(&model.CapabilitySemantics{
		Actions: datatypes.JSON(`[
			{"functionId":" a.fn ","subject":" resource_item ","identityInput":" /id "},
			{"functionId":"","subject":"none"},
			{"functionId":"b.fn","subject":""},
			{"functionId":"c.fn","subject":"none","identityInput":"/ignored"}
		]`),
	})
	require.Len(t, actions, 2)
	assert.Equal(t, "a.fn", actions[0].FunctionID)
	assert.Equal(t, "/id", actions[0].IdentityInput)
	assert.Equal(t, "c.fn", actions[1].FunctionID)
	assert.Empty(t, actions[1].IdentityInput)
}

func TestExtraTopLevelPointerTokenAndReplaceSource(t *testing.T) {
	assert.Equal(t, "", topLevelPointerToken("player_id"))
	assert.Equal(t, "", topLevelPointerToken("/a/b"))
	assert.Equal(t, "", topLevelPointerToken("/"))
	assert.Equal(t, "a/b", topLevelPointerToken("/a~1b"))
	assert.Equal(t, "a~b", topLevelPointerToken("/a~0b"))

	assert.False(t, replaceSelectorSource(nil, "/x", spec.ValueSource{}))

	selector := spec.DefaultSelector(spec.JSONSchema(`{"type":"object","properties":{"playerId":{"type":"string"}}}`))
	assert.True(t, replaceSelectorSource(&selector, "/playerId", spec.ValueSource{Kind: spec.SourceRow}))
	assert.False(t, replaceSelectorSource(&selector, "/ghost", spec.ValueSource{}))
}

func TestExtraInlineActionTitleAndType(t *testing.T) {
	base := model.FunctionContract{FunctionID: "player.ban", OperationKey: "player.ban"}

	title := inlineActionTitle(&base, "zh-CN")
	assert.NotEmpty(t, title["zh-CN"])

	withSummary := base
	withSummary.Summary = map[string]interface{}{"zh-CN": "封禁玩家"}
	title = inlineActionTitle(&withSummary, "zh-CN")
	assert.Equal(t, "封禁玩家", title["zh-CN"])

	lowRisk := base
	assert.Equal(t, "default", inlineActionType(&lowRisk))
	highRisk := base
	highRisk.Risk = string(spec.RiskHigh)
	assert.Equal(t, "danger", inlineActionType(&highRisk))
}

// ---------------------------------------------------------------------------
// Schema bind-ability checks
// ---------------------------------------------------------------------------

func TestExtraSchemaCanBindChecks(t *testing.T) {
	singleRequired := spec.JSONSchema(`{
		"type":"object",
		"required":["player_id"],
		"properties":{"player_id":{"type":"string"}}
	}`)
	identityPlusFields := spec.JSONSchema(`{
		"type":"object",
		"required":["player_id","reason"],
		"properties":{"player_id":{"type":"string"},"reason":{"type":"string"}}
	}`)
	arrayField := spec.JSONSchema(`{
		"type":"object",
		"required":["ids"],
		"properties":{"ids":{"type":"array"}}
	}`)
	wrongArrayField := spec.JSONSchema(`{
		"type":"object",
		"required":["ids"],
		"properties":{"ids":{"type":"string"},"other":{"type":"array"}}
	}`)
	noRequired := spec.JSONSchema(`{"type":"object","properties":{"a":{"type":"string"}}}`)
	notObject := spec.JSONSchema(`[]`)

	t.Run("single required field", func(t *testing.T) {
		assert.False(t, schemaCanBindBySingleRequiredField(singleRequired, ""))
		assert.False(t, schemaCanBindBySingleRequiredField(notObject, "player_id"))
		assert.False(t, schemaCanBindBySingleRequiredField(noRequired, "missing"))
		assert.True(t, schemaCanBindBySingleRequiredField(singleRequired, "player_id"))
	})

	t.Run("identity plus fields", func(t *testing.T) {
		assert.False(t, schemaCanBindIdentityPlusFields(identityPlusFields, ""))
		assert.False(t, schemaCanBindIdentityPlusFields(notObject, "player_id"))
		assert.False(t, schemaCanBindIdentityPlusFields(noRequired, "missing"))
		assert.False(t, schemaCanBindIdentityPlusFields(singleRequired, "reason"))
		assert.True(t, schemaCanBindIdentityPlusFields(identityPlusFields, "player_id"))
	})

	t.Run("only identity", func(t *testing.T) {
		assert.False(t, schemaCanBindOnlyIdentity(singleRequired, ""))
		assert.False(t, schemaCanBindOnlyIdentity(notObject, "player_id"))
		assert.False(t, schemaCanBindOnlyIdentity(noRequired, "missing"))
		assert.False(t, schemaCanBindOnlyIdentity(identityPlusFields, "player_id"))
		assert.True(t, schemaCanBindOnlyIdentity(singleRequired, "player_id"))
	})

	t.Run("single required array field", func(t *testing.T) {
		assert.False(t, schemaCanBindBySingleRequiredArrayField(arrayField, ""))
		assert.False(t, schemaCanBindBySingleRequiredArrayField(notObject, "ids"))
		assert.False(t, schemaCanBindBySingleRequiredArrayField(noRequired, "missing"))
		assert.False(t, schemaCanBindBySingleRequiredArrayField(wrongArrayField, "ids"))
		assert.True(t, schemaCanBindBySingleRequiredArrayField(arrayField, "ids"))
	})

	t.Run("no required fields", func(t *testing.T) {
		assert.True(t, schemaHasNoRequiredFields(noRequired))
		assert.False(t, schemaHasNoRequiredFields(singleRequired))
	})
}

// ---------------------------------------------------------------------------
// removeTopLevelSchemaField / buildActionFormPresentation
// ---------------------------------------------------------------------------

func TestExtraRemoveTopLevelSchemaField(t *testing.T) {
	schema := spec.JSONSchema(`{
		"type":"object",
		"required":["player_id","reason"],
		"properties":{"player_id":{"type":"string"},"reason":{"type":"string"}}
	}`)

	next := removeTopLevelSchemaField(schema, "player_id")
	var rawProps struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	require.NoError(t, json.Unmarshal([]byte(next), &rawProps))
	assert.NotContains(t, rawProps.Properties, "player_id")
	assert.Contains(t, rawProps.Properties, "reason")

	onlyIdentity := removeTopLevelSchemaField(spec.JSONSchema(`{
		"type":"object",
		"required":["player_id"],
		"properties":{"player_id":{"type":"string"}}
	}`), "player_id")
	assert.NotContains(t, string(onlyIdentity), "required")

	assert.Equal(t, schema, removeTopLevelSchemaField(schema, ""))
	invalid := spec.JSONSchema(`not-object`)
	assert.Equal(t, invalid, removeTopLevelSchemaField(invalid, "x"))
	assert.Equal(t, spec.JSONSchema(nil), removeTopLevelSchemaField(spec.JSONSchema(nil), "x"))
}

func TestExtraBuildActionFormPresentation(t *testing.T) {
	contract := &model.FunctionContract{
		InputSchema: []byte(`{
			"type":"object",
			"required":["player_id","reason"],
			"properties":{"player_id":{"type":"string"},"reason":{"type":"string"}}
		}`),
	}

	// No input schema → nil form.
	assert.Nil(t, buildActionFormPresentation(&model.FunctionContract{}, nil,
		resourceActionSemantic{Subject: "resource_item"}))

	semantics := &model.CapabilitySemantics{IdentityField: "player_id"}
	form := buildActionFormPresentation(contract, semantics,
		resourceActionSemantic{Subject: "resource_item", IdentityInput: "/player_id"})
	require.NotNil(t, form)
	for _, field := range form.Fields {
		assert.NotEqual(t, "player_id", field.Key, "identity should be stripped from row-form fields")
	}
}

// ---------------------------------------------------------------------------
// Detail view helpers
// ---------------------------------------------------------------------------

func TestExtraDetailHelpers(t *testing.T) {
	collection := &model.FunctionContract{
		OutputSchema: []byte(`{
			"type":"object",
			"properties":{
				"items":{"type":"array","items":{"type":"object","properties":{"player_id":{"type":"string"}}}}
			}
		}`),
	}
	itemQuery := &model.FunctionContract{
		InputSchema: []byte(`{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}`),
		OutputSchema: []byte(`{
			"type":"object",
			"properties":{"id":{"type":"string"},"name":{"type":"string"}}
		}`),
	}
	semantics := &model.CapabilitySemantics{IdentityField: "id"}

	assert.False(t, canUseItemQueryAsDetailSource(nil, semantics))
	assert.False(t, canUseItemQueryAsDetailSource(itemQuery, nil))
	assert.False(t, canUseItemQueryAsDetailSource(&model.FunctionContract{}, semantics))
	assert.False(t, canUseItemQueryAsDetailSource(itemQuery, &model.CapabilitySemantics{}))
	assert.False(t, canUseItemQueryAsDetailSource(collection, semantics))
	assert.True(t, canUseItemQueryAsDetailSource(itemQuery, semantics))

	detail := buildDetailViewFromContracts(collection, itemQuery, semantics)
	require.NotNil(t, detail)
	foundName := false
	for _, field := range detail.Fields {
		if field.Key == "name" {
			foundName = true
		}
	}
	assert.True(t, foundName, "item query output should drive detail fields")

	// Without an item query the collection items schema drives the fields.
	fallback := buildDetailViewFromContracts(collection, nil, semantics)
	if fallback != nil {
		for _, field := range fallback.Fields {
			assert.NotEqual(t, "name", field.Key)
		}
	}

	binding, ok := buildDetailBinding(itemQuery, semantics)
	assert.True(t, ok)
	assert.Equal(t, "detail", binding.ID)

	binding, ok = buildDetailBinding(collection, semantics)
	assert.False(t, ok)
	assert.Empty(t, binding.ID)
}

func TestExtraFindCollectionItemsSchema(t *testing.T) {
	semantics := &model.CapabilitySemantics{ItemsFieldName: "rows"}

	assert.Nil(t, findCollectionItemsSchema(spec.JSONSchema(``), semantics))
	assert.Nil(t, findCollectionItemsSchema(spec.JSONSchema(`not-json`), semantics))
	assert.Nil(t, findCollectionItemsSchema(spec.JSONSchema(`{"type":"object"}`), semantics))

	named := findCollectionItemsSchema(spec.JSONSchema(`{
		"type":"object",
		"properties":{"rows":{"type":"array","items":{"type":"object"}}}
	}`), semantics)
	assert.NotNil(t, named)

	rootItems := findCollectionItemsSchema(spec.JSONSchema(`{"type":"array","items":{"type":"object"}}`), semantics)
	assert.NotNil(t, rootItems)
}

// ---------------------------------------------------------------------------
// Schema subset diagnostics
// ---------------------------------------------------------------------------

func TestExtraCollectUnsupportedSchemaFeatures(t *testing.T) {
	reasons := map[string]struct{}{}
	collectUnsupportedSchemaFeatures(json.RawMessage(`{"type":"object","oneOf":[{"type":"string"}],"$ref":"https://x/y"}`), reasons)
	assert.Contains(t, reasons, "oneOf")
	assert.Contains(t, reasons, "remote_$ref")

	reasons = map[string]struct{}{}
	collectUnsupportedSchemaFeatures(json.RawMessage(`[[{"anyOf":{}}]]`), reasons)
	assert.Contains(t, reasons, "anyOf")

	reasons = map[string]struct{}{}
	collectUnsupportedSchemaFeatures(json.RawMessage(`{bad`), reasons)
	assert.Contains(t, reasons, "invalid_json")

	reasons = map[string]struct{}{}
	collectUnsupportedSchemaFeatures(nil, reasons)
	assert.Empty(t, reasons)
}

func TestExtraSchemaSubsetDiagnostics(t *testing.T) {
	assert.Nil(t, schemaSubsetDiagnostics("fn", "input", nil))

	diag := schemaSubsetDiagnostics("fn", "input", spec.JSONSchema(`{"discriminator":1}`))
	require.Len(t, diag, 1)
	assert.Contains(t, diag[0].Message, "JSON Schema uses unsupported generation features")
}
