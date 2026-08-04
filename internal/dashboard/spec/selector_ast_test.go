package spec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type selectorVector struct {
	Name              string             `json:"name"`
	InputSchema       json.RawMessage    `json:"inputSchema,omitempty"`
	OutputSchema      json.RawMessage    `json:"outputSchema,omitempty"`
	Context           selectorVectorCtx  `json:"context,omitempty"`
	Selector          SelectorAST        `json:"selector,omitempty"`
	OutputAssignments []OutputAssignment `json:"outputAssignments,omitempty"`
	WantValid         *bool              `json:"wantValid,omitempty"`
	WantOutputValid   *bool              `json:"wantOutputValid,omitempty"`
	WantErrorCode     string             `json:"wantErrorCode,omitempty"`
}

type selectorVectorCtx struct {
	FormSchema   json.RawMessage            `json:"formSchema,omitempty"`
	RowSchema    json.RawMessage            `json:"rowSchema,omitempty"`
	DetailSchema json.RawMessage            `json:"detailSchema,omitempty"`
	PageState    map[string]json.RawMessage `json:"pageState,omitempty"`
}

func TestSelectorSharedVectors(t *testing.T) {
	for _, vector := range loadSelectorVectors(t) {
		t.Run(vector.Name, func(t *testing.T) {
			if vector.WantValid != nil {
				result := ValidateSelector(vector.Selector, JSONSchema(vector.InputSchema), SelectorContext{
					FormSchema:   JSONSchema(vector.Context.FormSchema),
					RowSchema:    JSONSchema(vector.Context.RowSchema),
					DetailSchema: JSONSchema(vector.Context.DetailSchema),
					PageState:    selectorVectorPageState(vector.Context.PageState),
				})
				assert.Equal(t, *vector.WantValid, result.Valid)
				if vector.WantErrorCode != "" {
					require.NotEmpty(t, result.Errors)
					assert.Equal(t, vector.WantErrorCode, result.Errors[0].Code)
				}
			}
			if vector.WantOutputValid != nil {
				result := ValidateOutputAssignments(vector.OutputAssignments, JSONSchema(vector.OutputSchema))
				assert.Equal(t, *vector.WantOutputValid, result.Valid)
			}
		})
	}
}

func selectorVectorPageState(input map[string]json.RawMessage) map[string]JSONSchema {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]JSONSchema, len(input))
	for key, value := range input {
		out[key] = JSONSchema(value)
	}
	return out
}

func TestDefaultSelectorUsesJSONPointer(t *testing.T) {
	selector := DefaultSelector(JSONSchema(`{
		"type":"object",
		"properties":{
			"name":{"type":"string"},
			"meta/key":{"type":"string"},
			"tilde~field":{"type":"string"}
		}
	}`))

	require.Len(t, selector.Assignments, 3)
	assert.Equal(t, "/meta~1key", selector.Assignments[0].Target)
	assert.Equal(t, SourceForm, selector.Assignments[0].Source.Kind)
	assert.Equal(t, "/meta~1key", selector.Assignments[0].Source.Path)
	assert.Equal(t, "/name", selector.Assignments[1].Target)
	assert.Equal(t, "/tilde~0field", selector.Assignments[2].Target)
}

func TestValidateSelectorRejectsDotPath(t *testing.T) {
	result := ValidateSelector(SelectorAST{Assignments: []InputAssignment{{
		Target: "keyword",
		Source: ValueSource{Kind: SourceForm, Path: "keyword"},
	}}}, JSONSchema(`{"type":"object","properties":{"keyword":{"type":"string"}}}`), SelectorContext{
		FormSchema: JSONSchema(`{"type":"object","properties":{"keyword":{"type":"string"}}}`),
	})

	require.False(t, result.Valid)
	assert.Equal(t, ErrCodeInvalidPath, result.Errors[0].Code)
}

func TestValidateSelectorAcceptsJSONPointerAndRequired(t *testing.T) {
	result := ValidateSelector(SelectorAST{Assignments: []InputAssignment{{
		Target: "/keyword",
		Source: ValueSource{Kind: SourceForm, Path: "/keyword"},
	}}}, JSONSchema(`{
		"type":"object",
		"properties":{"keyword":{"type":"string"}},
		"required":["keyword"]
	}`), SelectorContext{
		FormSchema: JSONSchema(`{"type":"object","properties":{"keyword":{"type":"string"}}}`),
	})

	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestValidateSelectorRequiresPageStateKey(t *testing.T) {
	result := ValidateSelector(SelectorAST{Assignments: []InputAssignment{{
		Target: "/playerId",
		Source: ValueSource{Kind: SourcePageState, Path: "/id"},
	}}}, JSONSchema(`{
		"type":"object",
		"properties":{"playerId":{"type":"string"}},
		"required":["playerId"]
	}`), SelectorContext{})

	require.False(t, result.Valid)
	assert.Equal(t, ErrCodeMissingRequired, result.Errors[0].Code)
}

func TestValidateSelectorRejectsTypeMismatch(t *testing.T) {
	result := ValidateSelector(SelectorAST{Assignments: []InputAssignment{{
		Target: "/count",
		Source: ValueSource{Kind: SourceForm, Path: "/name"},
	}}}, JSONSchema(`{
		"type":"object",
		"properties":{"count":{"type":"number"}},
		"required":["count"]
	}`), SelectorContext{
		FormSchema: JSONSchema(`{"type":"object","properties":{"name":{"type":"string"}}}`),
	})

	require.False(t, result.Valid)
	assert.Equal(t, ErrCodeTypeMismatch, result.Errors[0].Code)
}

func TestValidateOutputAssignments(t *testing.T) {
	result := ValidateOutputAssignments([]OutputAssignment{{
		StateKey: "players",
		Source:   "/data/items",
		Shape:    OutputShapeCollection,
	}}, JSONSchema(`{
		"type":"object",
		"properties":{
			"data":{
				"type":"object",
				"properties":{"items":{"type":"array"}}
			}
		}
	}`))

	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestDiffJSONSchemaFieldsReportsChangesAndRenameCandidate(t *testing.T) {
	result := DiffJSONSchemaFields(
		JSONSchema(`{
			"type":"object",
			"properties":{
				"keyword":{"type":"string"},
				"count":{"type":"number"},
				"oldName":{"type":"string"},
				"profile":{
					"type":"object",
					"properties":{"level":{"type":"number"}},
					"required":["level"]
				}
			},
			"required":["keyword"]
		}`),
		JSONSchema(`{
			"type":"object",
			"properties":{
				"keyword":{"type":"string"},
				"count":{"type":"string"},
				"newName":{"type":"string"},
				"profile":{
					"type":"object",
					"properties":{"level":{"type":"number"}}
				},
				"serverId":{"type":"string"}
			},
			"required":["keyword","serverId"]
		}`),
	)

	assertSchemaChange(t, result.Changes, "/count", SchemaFieldTypeChanged)
	assertSchemaChange(t, result.Changes, "/oldName", SchemaFieldRemoved)
	assertSchemaChange(t, result.Changes, "/newName", SchemaFieldAdded)
	assertSchemaChange(t, result.Changes, "/serverId", SchemaFieldAdded)
	assertSchemaChange(t, result.Changes, "/profile/level", SchemaFieldRequiredChanged)
	require.NotEmpty(t, result.RenameCandidates)
	assertRenameCandidate(t, result.RenameCandidates, "/oldName", "/newName")
}

func TestSelectorStaleDiagnosticsReportsBrokenSelectors(t *testing.T) {
	diags := SelectorStaleDiagnostics(
		SelectorAST{Assignments: []InputAssignment{
			{Target: "/keyword", Source: ValueSource{Kind: SourceForm, Path: "/keyword"}},
			{Target: "/count", Source: ValueSource{Kind: SourceForm, Path: "/count"}},
			{Target: "/oldName", Source: ValueSource{Kind: SourceForm, Path: "/oldName"}},
		}},
		[]OutputAssignment{{StateKey: "players", Source: "/items", Shape: OutputShapeCollection}},
		JSONSchema(`{
			"type":"object",
			"properties":{
				"keyword":{"type":"string"},
				"count":{"type":"number"},
				"oldName":{"type":"string"}
			},
			"required":["keyword"]
		}`),
		JSONSchema(`{
			"type":"object",
			"properties":{
				"keyword":{"type":"string"},
				"count":{"type":"string"},
				"newName":{"type":"string"},
				"serverId":{"type":"string"}
			},
			"required":["keyword","serverId"]
		}`),
		JSONSchema(`{"type":"object","properties":{"items":{"type":"array"}}}`),
		JSONSchema(`{"type":"object","properties":{"rows":{"type":"array"}}}`),
	)

	assertDiagnosticCode(t, diags, "selector_target_type_stale")
	assertDiagnosticCode(t, diags, "selector_target_stale")
	assertDiagnosticCode(t, diags, "selector_required_stale")
	assertDiagnosticCode(t, diags, "selector_output_source_stale")
	assertDiagnosticCode(t, diags, "selector_field_rename_candidate")
}

func loadSelectorVectors(t *testing.T) []selectorVector {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)

	path := filepath.Join(filepath.Dir(file), "../../../testdata/dashboard_selector_vectors.json")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	var vectors []selectorVector
	require.NoError(t, json.Unmarshal(raw, &vectors))
	return vectors
}

func assertSchemaChange(t *testing.T, changes []SchemaFieldChange, path string, changeType SchemaFieldChangeType) {
	t.Helper()
	for _, change := range changes {
		if change.Path == path && change.ChangeType == changeType {
			return
		}
	}
	t.Fatalf("schema change %s at %s not found in %#v", changeType, path, changes)
}

func assertDiagnosticCode(t *testing.T, diags []Diagnostic, code string) {
	t.Helper()
	for _, diag := range diags {
		if diag.Code == code {
			return
		}
	}
	t.Fatalf("diagnostic %s not found in %#v", code, diags)
}

func assertRenameCandidate(t *testing.T, candidates []FieldRenameCandidate, oldPath string, newPath string) {
	t.Helper()
	for _, candidate := range candidates {
		if candidate.OldPath == oldPath && candidate.NewPath == newPath {
			return
		}
	}
	t.Fatalf("rename candidate %s -> %s not found in %#v", oldPath, newPath, candidates)
}
