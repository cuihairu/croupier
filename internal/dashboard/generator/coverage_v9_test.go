package generator

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// parseRawMapV9 parses a JSON object into a raw-message map for tests that
// feed schema fragments directly into helpers.
func parseRawMapV9(t *testing.T, raw string) map[string]json.RawMessage {
	t.Helper()
	obj := parseJSONObject(json.RawMessage(raw))
	require.NotNil(t, obj)
	return obj
}

// ---------------------------------------------------------------------------
// TermDictionary.Lookup edge cases
// ---------------------------------------------------------------------------

func TestTermDictionaryLookupEmptyPartsV9(t *testing.T) {
	terms := TermDictionary{"resource/player": spec.LocalizedText{"zh-CN": "玩家"}}

	text, ok := terms.Lookup("resource", "")
	assert.False(t, ok)
	assert.Nil(t, text)

	text, ok = terms.Lookup("", "player")
	assert.False(t, ok)
	assert.Nil(t, text)

	text, ok = terms.Lookup("RESOURCE", "PLAYER")
	assert.True(t, ok)
	assert.Equal(t, "玩家", text["zh-CN"])

	var nilTerms TermDictionary
	text, ok = nilTerms.Lookup("resource", "player")
	assert.False(t, ok)
	assert.Nil(t, text)
}

// ---------------------------------------------------------------------------
// Small pure helpers
// ---------------------------------------------------------------------------

func TestNormalizeOptionsEmptyLocaleV9(t *testing.T) {
	opts := normalizeOptions(GenerateOptions{})
	assert.Equal(t, "zh-CN", opts.DefaultLocale)
	assert.NotNil(t, opts.Functions)
	assert.NotNil(t, opts.TaskSemantics)
	assert.NotNil(t, opts.ReportSemantics)
}

func TestOperationPageKeyEmptyFunctionV9(t *testing.T) {
	// sanitizeBindingID("") falls back to "binding", so an operation without
	// a function id derives its key from that placeholder.
	assert.Equal(t, "operation--binding", operationPageKey(spec.OperationSpec{}, GenerateOptions{}))
}

func TestAssessBaseCandidateMissingFunctionIDV9(t *testing.T) {
	diags := assessBaseCandidate(spec.OperationSpec{})
	require.Len(t, diags, 3)
	assert.Equal(t, "function_id_missing", diags[0].Code)
	assert.Equal(t, "function_disabled", diags[1].Code)
	assert.Equal(t, "operation_missing", diags[2].Code)
}

func TestApplySelectorsNilBindingV9(t *testing.T) {
	assert.NotPanics(t, func() {
		applySelectors(nil, spec.FunctionSpec{
			InputSchema: spec.JSONSchema(`{"type":"object","properties":{"a":{"type":"string"}}}`),
		})
	})
}

func TestFlushWordV9(t *testing.T) {
	var words []string
	var b strings.Builder

	// Nothing buffered: flush is a no-op.
	flushWord(&words, &b)
	assert.Empty(t, words)

	b.WriteString("player")
	flushWord(&words, &b)
	require.Len(t, words, 1)
	assert.Equal(t, "player", words[0])
	assert.Zero(t, b.Len())

	b.WriteString("ban")
	flushWord(&words, &b)
	require.Len(t, words, 2)
	assert.Equal(t, "ban", words[1])
}

func TestRawStringNonStringV9(t *testing.T) {
	assert.Empty(t, rawString([]byte(`123`)))
	assert.Empty(t, rawString([]byte(`{"a":1}`)))
	assert.Equal(t, "x", rawString([]byte(`"x"`)))
}

// ---------------------------------------------------------------------------
// GenerateTaskPageForOperation: events/result/cancel diagnostics
// ---------------------------------------------------------------------------

func TestGenerateTaskPageCompanionDiagnosticsV9(t *testing.T) {
	taskFn := func(id string) spec.FunctionSpec {
		return spec.FunctionSpec{
			ID:           id,
			InputSchema:  spec.JSONSchema(`{"type":"object","properties":{"taskId":{"type":"string"}},"required":["taskId"]}`),
			OutputSchema: spec.JSONSchema(`{"type":"object","properties":{"state":{"type":"string"}}}`),
		}
	}
	opts := GenerateOptions{
		DefaultLocale: "zh-CN",
		Functions: map[string]spec.FunctionSpec{
			"task.start":  taskFn("task.start"),
			"task.status": taskFn("task.status"),
		},
		TaskSemantics: map[string]spec.TaskSemantic{
			"task.start": {
				Start:  spec.FunctionRef{FunctionID: "task.start"},
				TaskID: spec.TaskIDSemantic{ResultPath: "/taskId", ValueType: spec.JsonScalarString},
				Status: spec.TaskStatusSemantic{
					Function:    spec.FunctionRef{FunctionID: "task.status"},
					TaskIDInput: "/taskId",
					StatePath:   "/state",
				},
				Events: &spec.TaskEventsSemantic{Function: spec.FunctionRef{FunctionID: "ghost.events"}},
				Result: &spec.TaskResultSemantic{Function: spec.FunctionRef{FunctionID: "ghost.result"}},
				Cancel: &spec.TaskCommandSemantic{Function: spec.FunctionRef{FunctionID: "ghost.cancel"}},
			},
		},
	}

	page := GenerateTaskPageForOperation(spec.OperationSpec{
		FunctionID: "task.start", Operation: "start", Enabled: true,
		Execution: spec.FunctionExecutionTask,
	}, opts)

	codes := map[string]bool{}
	for _, diag := range page.Diagnostics {
		codes[diag.Code] = true
	}
	assert.True(t, codes["task_events_function_missing"])
	assert.True(t, codes["task_result_function_missing"])
	assert.True(t, codes["task_cancel_function_missing"])
	assert.True(t, codes["task_semantics_incomplete"])
	require.NotNil(t, page.Task)
	assert.Equal(t, "status", page.Task.TaskView.StatusBindingID)
	assert.Empty(t, page.Task.TaskView.EventsBindingID)
}

// ---------------------------------------------------------------------------
// GenerateOperationPageForOperation: resource term title
// ---------------------------------------------------------------------------

func TestGenerateOperationPageResourceTermTitleV9(t *testing.T) {
	opts := GenerateOptions{
		DefaultLocale: "zh-CN",
		Functions: map[string]spec.FunctionSpec{
			"player.ban": {ID: "player.ban"},
		},
		Terms: TermDictionary{
			"resource/player.ban": spec.LocalizedText{"zh-CN": "封禁玩家"},
		},
	}
	page := GenerateOperationPageForOperation(spec.OperationSpec{
		FunctionID: "player.ban", Operation: "", Enabled: true,
	}, opts)
	assert.Equal(t, "封禁玩家", page.Title["zh-CN"])
}

// ---------------------------------------------------------------------------
// buildFormFields: enum selects
// ---------------------------------------------------------------------------

func TestBuildFormFieldsEnumSelectV9(t *testing.T) {
	fields := buildFormFields(spec.JSONSchema(`{
		"type":"object",
		"properties":{
			"status":{"type":"string","enum":{"active":"banned"}},
			"note":{"type":"string","title":"备注"}
		}
	}`), "zh-CN")
	require.Len(t, fields, 1)
	assert.Equal(t, "status", fields[0].Key)
	assert.Equal(t, spec.FormWidgetSelect, fields[0].Widget)
}

// ---------------------------------------------------------------------------
// Dataset helpers
// ---------------------------------------------------------------------------

func TestBuildDatasetSpecNoPropertiesV9(t *testing.T) {
	assert.Nil(t, buildDatasetSpec(spec.JSONSchema(`{"type":"array","items":{"type":"object"}}`), "zh-CN"))
}

func TestBuildDimensionsFromPointersSkipsUnknownV9(t *testing.T) {
	itemSchema := parseRawMapV9(t, `{"type":"object","properties":{"date":{"type":"string"}}}`)
	dims := buildDimensionsFromPointers(itemSchema, []string{"/date", "/ghost"}, "zh-CN")
	require.Len(t, dims, 1)
	assert.Equal(t, "date", dims[0].Key)
}

func TestDatasetItemSchemaAtPointerWalkStopsV9(t *testing.T) {
	schema := spec.JSONSchema(`{"type":"object","properties":{"a":{"type":"string"},"rows":{"type":"array","items":{"type":"object"}}}}`)
	assert.Nil(t, datasetItemSchemaAtPointer(schema, "/a/b"))
	assert.Nil(t, datasetItemSchemaAtPointer(schema, "/ghost"))
	require.NotNil(t, datasetItemSchemaAtPointer(schema, "/rows"))
}

// ---------------------------------------------------------------------------
// Composite generator
// ---------------------------------------------------------------------------

func TestGenerateCompositePageEmptyInputsV9(t *testing.T) {
	_, ok := GenerateCompositePage("", nil, nil, GenerateOptions{})
	assert.False(t, ok)

	contract := &model.FunctionContract{FunctionID: "player.list"}
	_, ok = GenerateCompositePage("", []CompositeSectionInput{{FunctionID: "player.list"}}, []*model.FunctionContract{contract}, GenerateOptions{})
	assert.False(t, ok)

	_, ok = GenerateCompositePage("composite--x", nil, []*model.FunctionContract{contract}, GenerateOptions{})
	assert.False(t, ok)
}

func TestGenerateCompositePageDefaultsAndToolbarV9(t *testing.T) {
	contracts := []*model.FunctionContract{
		{
			FunctionID:   "player.list",
			Capability:   dbenum.CapabilityCollectionQuery,
			InputSchema:  model.JSON(`{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`),
			OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"}}}}}}`),
		},
		{
			FunctionID:  "player.get",
			Capability:  dbenum.CapabilityItemQuery,
			InputSchema: model.JSON(`{"type":"object","properties":{"id":{"type":"string"}}}`),
		},
		{
			FunctionID:  "player.export",
			Capability:  dbenum.CapabilityAction,
			Risk:        dbenum.RiskDanger,
			InputSchema: model.JSON(`{"type":"object","properties":{"fmt":{"type":"string"}}}`),
		},
	}
	inputs := []CompositeSectionInput{
		{
			FunctionID: "player.list",
			Toolbar: []CompositeToolbarActionInput{
				{Label: "刷新", TargetSection: "player.list", Danger: true},
			},
		},
		{FunctionID: "player.get", Key: "detail", Title: "  "},
		{FunctionID: "player.export", View: "form"},
		{FunctionID: "ghost.fn"},
	}

	page, ok := GenerateCompositePage("composite--demo", inputs, contracts, GenerateOptions{})
	require.True(t, ok)
	require.NotNil(t, page.Composite)
	require.Len(t, page.Composite.Sections, 3)
	require.Len(t, page.Bindings, 3)

	// Section 0: collection_query defaults to table view with toolbar.
	first := page.Composite.Sections[0]
	assert.Equal(t, "table", first.View)
	require.NotNil(t, first.Toolbar)
	require.Len(t, first.Toolbar.Actions, 1)
	assert.Equal(t, "刷新", first.Toolbar.Actions[0].Label["zh-CN"])
	assert.Equal(t, spec.BindingUsageQuery, page.Bindings[0].Usage)

	// Section 1: item_query defaults to fields view; blank title falls back
	// to the function id.
	second := page.Composite.Sections[1]
	assert.Equal(t, "fields", second.View)
	assert.Equal(t, "player.get", second.Title["zh-CN"])

	// Section 2: explicit form view builds a form; danger risk requires
	// confirmation and usage stays action.
	third := page.Composite.Sections[2]
	assert.Equal(t, "form", third.View)
	assert.NotNil(t, third.Form)
	assert.True(t, page.Bindings[2].Execution.RequireConfirm)
	assert.Equal(t, spec.BindingUsageAction, page.Bindings[2].Usage)

	// The unknown function produces a function_missing diagnostic.
	found := false
	for _, diag := range page.Diagnostics {
		if diag.Code == "function_missing" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestDefaultCompositeViewBranchesV9(t *testing.T) {
	assert.Equal(t, "table", defaultCompositeView(&model.FunctionContract{Capability: dbenum.CapabilityCollectionQuery}))
	assert.Equal(t, "fields", defaultCompositeView(&model.FunctionContract{Capability: dbenum.CapabilityItemQuery}))
	assert.Equal(t, "form", defaultCompositeView(&model.FunctionContract{Capability: dbenum.CapabilityAction}))
	assert.Equal(t, "form", defaultCompositeView(&model.FunctionContract{}))
}

func TestCompositeUsageBranchesV9(t *testing.T) {
	assert.Equal(t, spec.BindingUsageQuery, compositeUsage("table"))
	assert.Equal(t, spec.BindingUsageQuery, compositeUsage("fields"))
	assert.Equal(t, spec.BindingUsageAction, compositeUsage("form"))
	assert.Equal(t, spec.BindingUsageAction, compositeUsage("actions"))
	assert.Equal(t, spec.BindingUsageAction, compositeUsage("toolbar"))
	assert.Equal(t, spec.BindingUsageAction, compositeUsage("anything"))
}

func TestRequiredPropertiesInvalidSchemaV9(t *testing.T) {
	assert.Empty(t, requiredProperties(spec.JSONSchema(`not-json`)))
	assert.Empty(t, requiredProperties(spec.JSONSchema(`{"required":"uid"}`)))
	assert.Empty(t, requiredProperties(spec.JSONSchema(`{"required":[1,2]}`)))
	requirements := requiredProperties(spec.JSONSchema(`{"required":["uid",true,"name"]}`))
	assert.True(t, requirements["uid"])
	assert.True(t, requirements["name"])
	assert.Len(t, requirements, 2)
}

func TestSortedRequiredSortsV9(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, sortedRequired(map[string]bool{"c": true, "a": true, "b": true}))
	assert.Empty(t, sortedRequired(map[string]bool{}))
}

func TestFirstNonEmptyStrAllBlankV9(t *testing.T) {
	assert.Empty(t, firstNonEmptyStr())
	assert.Empty(t, firstNonEmptyStr("", "   ", "\t"))
	assert.Equal(t, "x", firstNonEmptyStr("", " x "))
}

// ---------------------------------------------------------------------------
// GenerateResourcePageProposal guards
// ---------------------------------------------------------------------------

func TestGenerateResourcePageProposalGuardsV9(t *testing.T) {
	opts := DefaultGenerateOptions()

	_, ok := GenerateResourcePageProposal(nil, nil, opts)
	assert.False(t, ok)

	_, ok = GenerateResourcePageProposal(&model.CapabilitySemantics{ResourceKey: "  "}, nil, opts)
	assert.False(t, ok)

	_, ok = GenerateResourcePageProposal(&model.CapabilitySemantics{
		ResourceKey: "player", CollectionQueryID: 0, IdentityField: "id",
	}, nil, opts)
	assert.False(t, ok)

	_, ok = GenerateResourcePageProposal(&model.CapabilitySemantics{
		ResourceKey: "player", CollectionQueryID: 7, IdentityField: "",
	}, nil, opts)
	assert.False(t, ok)

	// Collection contract not present under the referenced id.
	_, ok = GenerateResourcePageProposal(&model.CapabilitySemantics{
		ResourceKey: "player", CollectionQueryID: 7, IdentityField: "id",
	}, []*model.FunctionContract{
		{Model: gorm.Model{ID: 7}, FunctionID: "player.list", Capability: dbenum.CapabilityItemQuery},
	}, opts)
	assert.False(t, ok)
}

func TestGenerateResourcePageProposalNilContractSkippedV9(t *testing.T) {
	page, ok := GenerateResourcePageProposal(&model.CapabilitySemantics{
		ResourceKey: "player", CollectionQueryID: 1, IdentityField: "id",
	}, []*model.FunctionContract{
		{
			Model:        gorm.Model{ID: 1},
			FunctionID:   "player.list",
			Capability:   dbenum.CapabilityCollectionQuery,
			InputSchema:  model.JSON(`{"type":"object","properties":{"page":{"type":"integer"},"pageSize":{"type":"integer"}}}`),
			OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"}}}},"total":{"type":"integer"}}}`),
		},
		nil, // nil contract must be skipped by the diagnostics loop
	}, DefaultGenerateOptions())
	require.True(t, ok)
	assert.Equal(t, "resource--player", page.PageKey)
}

// ---------------------------------------------------------------------------
// resourceOutputAssignments total-append path
// ---------------------------------------------------------------------------

func TestResourceOutputAssignmentsAppendsCustomTotalV9(t *testing.T) {
	contract := &model.FunctionContract{
		OutputSchema: model.JSON(`{"type":"object","properties":{
			"rows":{"type":"array","items":{"type":"object"}},
			"totalCount":{"type":"integer"}
		}}`),
	}
	semantics := &model.CapabilitySemantics{
		ItemsFieldName: "rows",
		TotalFieldName: "totalCount",
	}
	assignments := resourceOutputAssignments(contract, spec.BindingUsageQuery, semantics)
	require.Len(t, assignments, 2)
	assert.Equal(t, "items", assignments[0].StateKey)
	assert.Equal(t, "/rows", assignments[0].Source)
	assert.Equal(t, "total", assignments[1].StateKey)
	assert.Equal(t, "/totalCount", assignments[1].Source)
}

// ---------------------------------------------------------------------------
// Selector tweaks: nil/empty semantics guards
// ---------------------------------------------------------------------------

func TestApplyCollectionQuerySelectorNilSemanticsV9(t *testing.T) {
	selector := spec.SelectorAST{Assignments: []spec.InputAssignment{{Target: "/page"}}}
	unchanged := applyCollectionQuerySelector(selector, nil)
	require.Len(t, unchanged.Assignments, 1)
	assert.Empty(t, unchanged.Assignments[0].Source.Kind)
}

func TestApplyIdentityRowSelectorEmptyIdentityV9(t *testing.T) {
	selector := spec.SelectorAST{Assignments: []spec.InputAssignment{{Target: "/id"}}}
	unchanged := applyIdentityRowSelector(selector, &model.CapabilitySemantics{})
	require.Len(t, unchanged.Assignments, 1)
	assert.Empty(t, unchanged.Assignments[0].Source.Kind)
}

func TestBuildUpdateFormFromContractEmptyIdentityV9(t *testing.T) {
	contract := &model.FunctionContract{
		InputSchema: model.JSON(`{"type":"object","required":["uid"],"properties":{"uid":{"type":"string"}}}`),
	}
	form := buildUpdateFormFromContract(contract, &model.CapabilitySemantics{})
	require.NotNil(t, form)
	assert.Contains(t, string(form.JSONSchema), "uid")
}

// ---------------------------------------------------------------------------
// schemaCanBind* guards
// ---------------------------------------------------------------------------

func TestSchemaCanBindIdentityPlusFieldsV9(t *testing.T) {
	assert.False(t, schemaCanBindIdentityPlusFields(spec.JSONSchema(`{"type":"object"}`), "uid"))
	assert.False(t, schemaCanBindIdentityPlusFields(spec.JSONSchema(`{"type":"object","properties":{"uid":{"type":"string"}}}`), "uid"),
		"identity present but not required must be rejected")
	assert.False(t, schemaCanBindIdentityPlusFields(spec.JSONSchema(`{"type":"object","required":["uid"],"properties":{"uid":{"type":"string"}}}`), "ghost"))
	assert.True(t, schemaCanBindIdentityPlusFields(spec.JSONSchema(`{"type":"object","required":["uid"],"properties":{"uid":{"type":"string"},"reason":{"type":"string"}}}`), "uid"))
}

func TestSchemaCanBindOnlyIdentityInvalidSchemaV9(t *testing.T) {
	assert.False(t, schemaCanBindOnlyIdentity(spec.JSONSchema(`not-json`), "uid"))
}

func TestRequiredFieldNamesInvalidPayloadV9(t *testing.T) {
	assert.Nil(t, requiredFieldNames(parseRawMapV9(t, `{"required":"uid"}`)))
}

func TestSchemaCanBindBySingleRequiredFieldNoPropertiesV9(t *testing.T) {
	assert.False(t, schemaCanBindBySingleRequiredField(spec.JSONSchema(`{"type":"object"}`), "uid"))
}

func TestSchemaCanBindBySingleRequiredArrayFieldV9(t *testing.T) {
	assert.False(t, schemaCanBindBySingleRequiredArrayField(spec.JSONSchema(`{"type":"object"}`), "ids"))
	assert.False(t, schemaCanBindBySingleRequiredArrayField(spec.JSONSchema(`{"type":"object","required":["ids","extra"],"properties":{"ids":{"type":"array"},"extra":{"type":"string"}}}`), "ids"),
		"another required field must be rejected")
}

// ---------------------------------------------------------------------------
// Inline resource actions
// ---------------------------------------------------------------------------

func TestBuildInlineResourceActionNilInputsV9(t *testing.T) {
	_, _, _, ok, diag := buildInlineResourceAction(nil, nil, resourceActionSemantic{}, "zh-CN")
	assert.False(t, ok)
	assert.Empty(t, diag.Code)
}

func TestBuildInlineResourceActionsUninlinableActionV9(t *testing.T) {
	semantics := &model.CapabilitySemantics{
		IdentityField: "uid",
		Actions: model.JSON(`[
			{"functionId":"fn.odd","subject":"resource_item","identityInput":"/ghost"}
		]`),
	}
	contracts := []*model.FunctionContract{
		{
			FunctionID:  "fn.odd",
			Capability:  dbenum.CapabilityAction,
			InputSchema: model.JSON(`{"type":"object","required":["uid"],"properties":{"uid":{"type":"string"}}}`),
		},
	}
	rowActions, batchActions, toolbarActions, bindings, diags := buildInlineResourceActions(semantics, contracts, "zh-CN")
	assert.Empty(t, rowActions)
	assert.Empty(t, batchActions)
	assert.Empty(t, toolbarActions)
	assert.Empty(t, bindings)
	require.Len(t, diags, 1)
	assert.Equal(t, "resource_action_requires_operation_page", diags[0].Code)
}

func TestBuildInlineResourceActionSelectorFailuresV9(t *testing.T) {
	semantics := &model.CapabilitySemantics{IdentityField: "uid"}
	nonArray := &model.FunctionContract{
		InputSchema: model.JSON(`{"type":"object","required":["uid"],"properties":{"uid":{"type":"string"}}}`),
	}
	// resource_selection over a non-array field cannot bind.
	_, _, ok := buildInlineResourceActionSelector(nonArray, semantics,
		resourceActionSemantic{Subject: "resource_selection", IdentityInput: "/uid"})
	assert.False(t, ok)

	// "none" subject requires an input schema without required fields.
	withRequired := &model.FunctionContract{
		InputSchema: model.JSON(`{"type":"object","required":["uid"],"properties":{"uid":{"type":"string"}}}`),
	}
	_, _, ok = buildInlineResourceActionSelector(withRequired, semantics,
		resourceActionSemantic{Subject: "none"})
	assert.False(t, ok)
}

func TestResourceOperationRequiresConfirmationV9(t *testing.T) {
	assert.False(t, resourceOperationRequiresConfirmation(nil))
	assert.True(t, resourceOperationRequiresConfirmation(&model.FunctionContract{Risk: dbenum.RiskHigh}))
	assert.True(t, resourceOperationRequiresConfirmation(&model.FunctionContract{Risk: dbenum.RiskDanger}))
	assert.False(t, resourceOperationRequiresConfirmation(&model.FunctionContract{Risk: dbenum.RiskSafe}))
}

// ---------------------------------------------------------------------------
// List view / filters / pagination
// ---------------------------------------------------------------------------

func TestBuildListViewFromContractFallbackColumnTitleV9(t *testing.T) {
	// "-" humanizes to the empty string, so the raw key is used as title.
	contract := &model.FunctionContract{
		InputSchema:  model.JSON(`{"type":"object","properties":{"page":{"type":"integer"},"pageSize":{"type":"integer"},"status":{"type":"string"}}}`),
		OutputSchema: model.JSON(`{"type":"object","properties":{"rows":{"type":"array","items":{"type":"object","properties":{"-":{"type":"string"},"uid":{"type":"string"}}}}}}`),
	}
	semantics := &model.CapabilitySemantics{
		IdentityField:     "uid",
		ItemsFieldName:    "rows",
		PageFieldName:     "page",
		PageSizeFieldName: "pageSize",
	}
	list := buildListViewFromContract(contract, semantics)
	require.NotNil(t, list)
	found := false
	for _, col := range list.Columns {
		if col.Key == "-" {
			found = true
			assert.Equal(t, "-", col.Title["zh-CN"])
		}
		if col.Key == "status" {
			assert.True(t, col.Filterable, "filter key shared with a column marks it filterable")
		}
	}
	assert.True(t, found)
}

func TestBuildFiltersFromContractFallbackTitleV9(t *testing.T) {
	contract := &model.FunctionContract{
		InputSchema: model.JSON(`{"type":"object","properties":{"-":{"type":"string"}}}`),
	}
	filters := buildFiltersFromContract(contract, &model.CapabilitySemantics{})
	require.Len(t, filters, 1)
	assert.Equal(t, "-", filters[0].Key)
	assert.Equal(t, "-", filters[0].Title["zh-CN"])
}

func TestPaginationFromContractGuardsV9(t *testing.T) {
	semantics := &model.CapabilitySemantics{PageFieldName: "page", PageSizeFieldName: "pageSize"}
	assert.Nil(t, paginationFromContract(nil, semantics))
	assert.Nil(t, paginationFromContract(&model.FunctionContract{}, semantics))
	assert.Nil(t, paginationFromContract(&model.FunctionContract{
		InputSchema: model.JSON(`{"type":"object"}`),
	}, semantics))
	assert.Nil(t, paginationFromContract(&model.FunctionContract{
		InputSchema: model.JSON(`{"type":"object","properties":{"pageSize":{"type":"integer"}}}`),
	}, semantics))
	assert.Nil(t, paginationFromContract(&model.FunctionContract{
		InputSchema: model.JSON(`{"type":"object","properties":{"page":{"type":"integer"}}}`),
	}, semantics))
	require.NotNil(t, paginationFromContract(&model.FunctionContract{
		InputSchema: model.JSON(`{"type":"object","properties":{"page":{"type":"integer"},"pageSize":{"type":"integer"}}}`),
	}, semantics))
}

func TestResolvePaginationFieldsUnresolvableV9(t *testing.T) {
	properties := parseRawMapV9(t, `{"type":"object","properties":{"uid":{"type":"string"}}}`)
	assert.False(t, resolvePaginationFields(properties, &model.CapabilitySemantics{
		PageFieldName: "page", PageSizeFieldName: "pageSize",
	}))

	withPage := parseRawMapV9(t, `{"type":"object","properties":{"page":{"type":"integer"}}}`)
	assert.False(t, resolvePaginationFields(withPage, &model.CapabilitySemantics{
		PageFieldName: "page", PageSizeFieldName: "pageSize",
	}))
}

// ---------------------------------------------------------------------------
// Detail view helpers
// ---------------------------------------------------------------------------

func TestBuildDetailViewFromContractsEmptyShapesV9(t *testing.T) {
	// No collection output schema: no detail schema can be derived.
	assert.Nil(t, buildDetailViewFromContracts(&model.FunctionContract{}, nil, nil))

	itemsNoProps := &model.FunctionContract{
		OutputSchema: model.JSON(`{"type":"object","properties":{"rows":{"type":"array","items":{"type":"object"}}}}`),
	}
	assert.Nil(t, buildDetailViewFromContracts(itemsNoProps, nil, nil))
}

func TestBuildDetailViewFallbackColumnTitleV9(t *testing.T) {
	contract := &model.FunctionContract{
		OutputSchema: model.JSON(`{"type":"object","properties":{"rows":{"type":"array","items":{"type":"object","properties":{"-":{"type":"string"}}}}}}`),
	}
	detail := buildDetailViewFromContracts(contract, nil, nil)
	require.NotNil(t, detail)
	require.Len(t, detail.Fields, 1)
	assert.Equal(t, "-", detail.Fields[0].Title["zh-CN"])
}

func TestCanUseItemQueryAsDetailSourceNoPropertiesV9(t *testing.T) {
	item := &model.FunctionContract{
		InputSchema:  model.JSON(`{"type":"object","required":["uid"],"properties":{"uid":{"type":"string"}}}`),
		OutputSchema: model.JSON(`{"type":"string"}`),
	}
	assert.False(t, canUseItemQueryAsDetailSource(item, &model.CapabilitySemantics{IdentityField: "uid"}))
}
