package spec

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONSchemaMarshalUnmarshalV9(t *testing.T) {
	out, err := json.Marshal(JSONSchema(nil))
	require.NoError(t, err)
	assert.Equal(t, "null", string(out))

	out, err = json.Marshal(JSONSchema(`{"type":"object"}`))
	require.NoError(t, err)
	assert.Equal(t, `{"type":"object"}`, string(out))

	var s JSONSchema
	require.NoError(t, json.Unmarshal([]byte(`{"a":1}`), &s))
	assert.Equal(t, `{"a":1}`, string(s))

	type v9Wrapper struct {
		Schema JSONSchema `json:"schema"`
	}
	data, err := json.Marshal(v9Wrapper{Schema: JSONSchema(`{"type":"string"}`)})
	require.NoError(t, err)
	assert.JSONEq(t, `{"schema":{"type":"string"}}`, string(data))
}

func TestCompositePageStateSchemaV9(t *testing.T) {
	assert.Empty(t, compositePageStateSchema(nil))

	comp := &CompositePageSpec{
		Sections: []CompositeSection{
			{Key: "s1", BindingID: "b1", RefreshOn: []string{"sel", "  ", "", "sel"}},
			{Key: "s2", BindingID: " b2 ", RefreshOn: []string{"b1"}},
			{Key: "s3", BindingID: "   "},
		},
	}
	out := compositePageStateSchema(comp)
	assert.Len(t, out, 3)
	assert.Contains(t, out, "sel")
	assert.Contains(t, out, "b1")
	assert.Contains(t, out, "b2")
	assert.Equal(t, JSONSchema(`{}`), out["sel"])
}

func TestSelectorContextForBindingCompositeV9(t *testing.T) {
	page := PageSpec{
		Type: PageTypeComposite,
		Composite: &CompositePageSpec{
			Sections: []CompositeSection{
				{Key: "list", BindingID: "q1", View: "table", RefreshOn: []string{"sel"}},
				{Key: "detail", BindingID: "d1", View: "fields"},
			},
		},
	}
	ctx := SelectorContextForBinding(page, PageFunctionBinding{ID: "q1"})
	assert.Equal(t, PageTypeComposite, ctx.PageType)
	assert.Empty(t, ctx.FormSchema)
	assert.Contains(t, ctx.PageState, "q1")
	assert.Contains(t, ctx.PageState, "d1")
	assert.Contains(t, ctx.PageState, "sel")
}

func TestValidatePublishableCompositePageShapeV9(t *testing.T) {
	valid := PageSpec{
		Type: PageTypeComposite,
		Composite: &CompositePageSpec{Sections: []CompositeSection{{
			Key:       "t",
			BindingID: "b",
			View:      "table",
			Span:      12,
			Table:     &CompositeTableSpec{},
		}}},
	}
	assert.Empty(t, ValidatePublishablePageShape(valid))

	empty := PageSpec{Type: PageTypeComposite, Composite: &CompositePageSpec{}}
	diags := ValidatePublishablePageShape(empty)
	require.NotEmpty(t, diags)
	assert.Equal(t, "composite_empty", diags[0].Code)
}

func TestValidatePageVariantCompositeV9(t *testing.T) {
	multi := PageSpec{Type: PageTypeComposite, Composite: &CompositePageSpec{}, Resource: &ResourcePageSpec{}}
	diags := validatePageVariant(multi)
	require.Len(t, diags, 1)
	assert.Equal(t, "page_variant_invalid", diags[0].Code)

	ok := PageSpec{Type: PageTypeComposite, Composite: &CompositePageSpec{}}
	assert.Empty(t, validatePageVariant(ok))

	mismatch := PageSpec{Type: PageTypeComposite, Resource: &ResourcePageSpec{}}
	diags = validatePageVariant(mismatch)
	require.Len(t, diags, 1)
	assert.Equal(t, "page_variant_type_mismatch", diags[0].Code)
}

func TestRequireBindingUsageBlankBindingIDV9(t *testing.T) {
	page := PageSpec{
		Type:      PageTypeOperation,
		Operation: &OperationPageSpec{},
		Bindings:  []PageFunctionBinding{{ID: "   ", Usage: BindingUsageAction}},
	}
	diags := ValidatePublishablePageShape(page)
	require.Len(t, diags, 1)
	assert.Equal(t, "page_primary_binding_missing", diags[0].Code)
}

func TestValidatePublishableNilBodiesAndBlankRefV9(t *testing.T) {
	assert.Nil(t, validatePublishableOperationPage(nil, nil))
	assert.Nil(t, validatePublishableTaskPage(nil, nil))
	assert.Nil(t, validatePublishableReportPage(nil))

	page := PageSpec{
		Type:      PageTypeOperation,
		Operation: &OperationPageSpec{Confirm: &ConfirmActionSpec{BindingID: "   "}},
		Bindings:  []PageFunctionBinding{{ID: "main", Usage: BindingUsageAction}},
	}
	diags := ValidatePublishablePageShape(page)
	assertHasDiagnosticCode(t, diags, "operation_confirm_binding_invalid")
}

func TestValidateSelectorSchemaErrorPathsV9(t *testing.T) {
	result := ValidateSelector(SelectorAST{}, JSONSchema(`{bad`), SelectorContext{})
	assert.False(t, result.Valid)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "invalid JSON Schema", result.Errors[0].Message)

	required, err := requiredPointers(JSONSchema(``))
	require.NoError(t, err)
	assert.Empty(t, required)

	_, err = requiredPointers(JSONSchema(`{bad`))
	assert.Error(t, err)

	_, err = requiredPointers(JSONSchema(`{"required":5}`))
	assert.Error(t, err)
}

func TestValidateSelectorTargetNotFoundV9(t *testing.T) {
	result := ValidateSelector(SelectorAST{Assignments: []InputAssignment{{
		Target: "/missing",
		Source: ValueSource{Kind: SourceForm, Path: "/name"},
	}}}, JSONSchema(`{"type":"object","properties":{"name":{"type":"string"}}}`), SelectorContext{
		FormSchema: JSONSchema(`{"type":"object","properties":{"name":{"type":"string"}}}`),
	})
	assert.False(t, result.Valid)
	require.NotEmpty(t, result.Errors)
	assert.Equal(t, "target field not found in schema", result.Errors[0].Message)
}

func TestBindingUsesRowSelectionSourceV9(t *testing.T) {
	b := PageFunctionBinding{ID: "a", Usage: BindingUsageAction}
	assert.False(t, bindingUsesRowSource(b))
	assert.False(t, bindingUsesSelectionSource(b))

	b.Selectors = &BindingSelectors{Input: SelectorAST{Assignments: []InputAssignment{
		{Target: "/x", Source: ValueSource{Kind: SourceForm, Path: "/x"}},
	}}}
	assert.False(t, bindingUsesRowSource(b))
	assert.False(t, bindingUsesSelectionSource(b))

	page := PageSpec{Type: PageTypeResource, Resource: &ResourcePageSpec{ListView: &ListViewSpec{}}}
	ctx := SelectorContextForBinding(page, b)
	assert.False(t, ctx.IsRowAction)
	assert.False(t, ctx.IsBatchAction)
}

func TestMarshalRawObjectInvalidValueV9(t *testing.T) {
	out := marshalRawObject(map[string]json.RawMessage{"a": json.RawMessage(`{bad`)})
	assert.Equal(t, json.RawMessage(`null`), out)
}

func TestSourceAllowedAndPathValidationV9(t *testing.T) {
	ctx := SelectorContext{
		HasDetailView: true,
		DetailSchema:  JSONSchema(`{"type":"object","properties":{"id":{"type":"string"}}}`),
	}
	assert.True(t, isSourceAllowed(SourceRow, ctx))
	assert.True(t, isSourceAllowed(SourceDetail, ctx))
	assert.True(t, validateSourcePath(SourceDetail, "/id", ctx))
	assert.False(t, validateSourcePath(SourceDetail, "/missing", ctx))
	assert.True(t, validateSourcePath(SourceLiteral, "/anything", ctx))
	assert.True(t, validateSourcePath(SourceForm, "/x", SelectorContext{}))

	result := ValidateSelector(SelectorAST{Assignments: []InputAssignment{{
		Target: "/name",
		Source: ValueSource{Kind: SourceRow, Path: "/id"},
	}}}, JSONSchema(`{"type":"object","properties":{"name":{"type":"string"}}}`), ctx)
	assert.True(t, result.Valid, result.Errors)
}

func TestValidatePageStatePathV9(t *testing.T) {
	assert.True(t, validatePageStatePath(ValueSource{Kind: SourcePageState, Key: "k"}, SelectorContext{}))

	ctx := SelectorContext{PageState: map[string]JSONSchema{
		"taskStatus": JSONSchema(`{"type":"object","properties":{"state":{"type":"string"}}}`),
	}}
	assert.False(t, validatePageStatePath(ValueSource{Kind: SourcePageState, Key: "missing"}, ctx))
	assert.True(t, validatePageStatePath(ValueSource{Kind: SourcePageState, Key: "taskStatus"}, ctx))
	assert.True(t, validatePageStatePath(ValueSource{Kind: SourcePageState, Key: "taskStatus", Path: "/state"}, ctx))
	assert.False(t, validatePageStatePath(ValueSource{Kind: SourcePageState, Key: "taskStatus", Path: "/nope"}, ctx))
}

func TestIsAssignableNonPickTransformV9(t *testing.T) {
	ctx := SelectorContext{FormSchema: JSONSchema(`{"type":"object","properties":{"n":{"type":"number"}}}`)}
	schema := JSONSchema(`{"type":"object","properties":{"count":{"type":"number"}}}`)
	assert.True(t, isAssignable(schema, "/count", ValueSource{
		Kind:      SourceForm,
		Path:      "/n",
		Transform: &TransformSpec{Type: TransformType("custom")},
	}, ctx))
}

func TestSchemaPathEdgeCasesV9(t *testing.T) {
	assert.True(t, schemaHasPath(JSONSchema(`{}`), "/a"))
	assert.False(t, schemaHasPath(JSONSchema(`{"type":"object"}`), "/a"))
	assert.False(t, schemaHasPath(JSONSchema(`{"properties":{"a":"scalar"}}`), "/a"))

	_, ok := schemaNodeAtPath(JSONSchema(`{"type":"object"}`), "/a")
	assert.False(t, ok)
	_, ok = schemaNodeAtPath(JSONSchema(`{"properties":{"a":"scalar"}}`), "/a")
	assert.False(t, ok)

	typ, ok := schemaTypeAtPath(JSONSchema(`{"type":"object","properties":{"x":{"type":["string"]}}}`), "/x")
	assert.True(t, ok)
	assert.Equal(t, "string", typ)

	assert.True(t, outputShapeMatchesSchema(OutputResultShape("bogus"),
		JSONSchema(`{"type":"object","properties":{"x":{"type":"array"}}}`), "/x"))
}

func TestSelectorStaleDiagnosticsEdgeCasesV9(t *testing.T) {
	diags := SelectorStaleDiagnostics(
		SelectorAST{Assignments: []InputAssignment{{Target: "/a", Source: ValueSource{Kind: SourceForm, Path: "/a"}}}},
		nil,
		JSONSchema(`{bad`),
		JSONSchema(`{"type":"object"}`),
		JSONSchema(``),
		JSONSchema(``),
	)
	require.NotEmpty(t, diags)
	assertDiagnosticCode(t, diags, "schema_diff_invalid_schema")

	diags = SelectorStaleDiagnostics(
		SelectorAST{},
		[]OutputAssignment{{StateKey: "out", Source: "/data", Shape: OutputShapeScalar}},
		JSONSchema(`{"type":"object","properties":{"x":{"type":"string"}}}`),
		JSONSchema(`{"type":"object","properties":{"x":{"type":"string"}}}`),
		JSONSchema(`{"type":"object","properties":{"data":{"type":"string"}}}`),
		JSONSchema(`{"type":"object","properties":{"data":{"type":"array"}}}`),
	)
	assertDiagnosticCode(t, diags, "selector_output_type_stale")
}

func TestSchemaTypeFromNodeV9(t *testing.T) {
	assert.Equal(t, "", schemaTypeFromNode(map[string]json.RawMessage{}))
	assert.Equal(t, "", schemaTypeFromNode(map[string]json.RawMessage{"other": json.RawMessage(`1`)}))
	assert.Equal(t, "string", schemaTypeFromNode(map[string]json.RawMessage{"type": json.RawMessage(`["string"]`)}))
	assert.Equal(t, "", schemaTypeFromNode(map[string]json.RawMessage{"type": json.RawMessage(`["string","null"]`)}))

	result := DiffJSONSchemaFields(
		JSONSchema(`{"type":"object","properties":{"a":"scalar","b":{"type":"string"}}}`),
		JSONSchema(`{"type":"object","properties":{"b":{"type":"string"}}}`),
	)
	assert.Empty(t, result.Diagnostics)
	assert.Empty(t, result.Changes)
}

func TestFieldRenameCandidatesEdgeCasesV9(t *testing.T) {
	result := DiffJSONSchemaFields(
		JSONSchema(`{"type":"object","properties":{"old":{}}}`),
		JSONSchema(`{"type":"object","properties":{"new":{}}}`),
	)
	assert.Empty(t, result.RenameCandidates)

	result = DiffJSONSchemaFields(
		JSONSchema(`{"type":"object","properties":{"old":{"type":"string"}}}`),
		JSONSchema(`{"type":"object","properties":{"new":{"type":"string"}},"required":["new"]}`),
	)
	assert.Empty(t, result.RenameCandidates)

	nested := DiffJSONSchemaFields(
		JSONSchema(`{"type":"object","properties":{"a":{"type":"object","properties":{"old":{"type":"string"}}}}}`),
		JSONSchema(`{"type":"object","properties":{"b":{"type":"object","properties":{"new":{"type":"string"}}}}}`),
	)
	assert.NotEmpty(t, nested.Changes)
	assertRenameCandidate(t, nested.RenameCandidates, "/a", "/b")
	nestedHasDeepRename := false
	for _, candidate := range nested.RenameCandidates {
		if candidate.OldPath == "/a/old" && candidate.NewPath == "/b/new" {
			nestedHasDeepRename = true
		}
	}
	assert.False(t, nestedHasDeepRename, "fields under different parents must not be rename candidates")
}

func TestValidateIdentityMappingRowSourceV9(t *testing.T) {
	semantics := &SemanticContext{IdentityField: "player_id"}

	good := SelectorAST{Assignments: []InputAssignment{{
		Target: "/player_id",
		Source: ValueSource{Kind: SourceRow, Path: "/player_id"},
	}}}
	result := ValidateSelectorSemantics(good, semantics, PageTypeResource, BindingUsageAction)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Warnings)

	bad := SelectorAST{Assignments: []InputAssignment{{
		Target: "/player_id",
		Source: ValueSource{Kind: SourceRow, Path: "/name"},
	}}}
	result = ValidateSelectorSemantics(bad, semantics, PageTypeResource, BindingUsageAction)
	assert.True(t, result.Valid)
	assert.NotEmpty(t, result.Warnings)
}

func TestValidateCollectionMappingV9(t *testing.T) {
	semantics := &SemanticContext{IdentityField: "player_id", PageFieldName: "page", PageSizeFieldName: "page_size"}

	mapped := SelectorAST{Assignments: []InputAssignment{{
		Target: "/page",
		Source: ValueSource{Kind: SourceForm, Path: "/page"},
	}}}
	result := ValidateSelectorSemantics(mapped, semantics, PageTypeResource, BindingUsageQuery)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Warnings)

	unmapped := SelectorAST{Assignments: []InputAssignment{{
		Target: "/keyword",
		Source: ValueSource{Kind: SourceForm, Path: "/keyword"},
	}}}
	result = ValidateSelectorSemantics(unmapped, semantics, PageTypeResource, BindingUsageQuery)
	assert.True(t, result.Valid)
	assert.NotEmpty(t, result.Warnings)
}
