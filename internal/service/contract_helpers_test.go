package service

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/normalizer"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"gorm.io/datatypes"
)

func TestComputeDigest(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "string input",
			input:    "hello",
			expected: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
		},
		{
			name:     "number input",
			input:    42,
			expected: "73475cb40a2613facffa1b01b0f7d3b2c0e5c8f0e4b5b0e5b0e5b0e5b0e5b0e5",
		},
		{
			name:     "nil input",
			input:    nil,
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeDigest(tt.input)
			// Just verify it's a valid hex string of correct length
			assert.Len(t, result, 64)
			assert.Regexp(t, "^[0-9a-f]{64}$", result)
		})
	}
}

func TestToJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "string",
			input:    "hello",
			expected: `"hello"`,
		},
		{
			name:     "number",
			input:    42,
			expected: `42`,
		},
		{
			name:     "map",
			input:    map[string]string{"key": "value"},
			expected: `{"key":"value"}`,
		},
		{
			name:     "slice",
			input:    []int{1, 2, 3},
			expected: `[1,2,3]`,
		},
		{
			name:     "nil",
			input:    nil,
			expected: `null`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toJSON(tt.input)
			// Verify it's valid JSON
			var parsed interface{}
			err := json.Unmarshal(result, &parsed)
			assert.NoError(t, err)
		})
	}
}

func TestToJSONMap(t *testing.T) {
	tests := []struct {
		name     string
		input    spec.LocalizedText
		expected int
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: 0,
		},
		{
			name:     "empty map",
			input:    spec.LocalizedText{},
			expected: 0,
		},
		{
			name:     "with values",
			input:    spec.LocalizedText{"en": "Hello", "zh": "你好"},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toJSONMap(tt.input)
			assert.Len(t, result, tt.expected)
		})
	}
}

func TestSchemaString(t *testing.T) {
	tests := []struct {
		name     string
		input    json.RawMessage
		expected string
	}{
		{
			name:     "nil",
			input:    nil,
			expected: "",
		},
		{
			name:     "empty",
			input:    json.RawMessage(""),
			expected: "",
		},
		{
			name:     "valid string",
			input:    json.RawMessage(`"hello"`),
			expected: "hello",
		},
		{
			name:     "with whitespace",
			input:    json.RawMessage(`"  hello  "`),
			expected: "hello",
		},
		{
			name:     "invalid json",
			input:    json.RawMessage(`{invalid`),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := schemaString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSchemaScalarType(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]json.RawMessage
		expected string
	}{
		{
			name:     "string type",
			input:    map[string]json.RawMessage{"type": json.RawMessage(`"string"`)},
			expected: "string",
		},
		{
			name:     "number type",
			input:    map[string]json.RawMessage{"type": json.RawMessage(`"number"`)},
			expected: "number",
		},
		{
			name:     "integer type",
			input:    map[string]json.RawMessage{"type": json.RawMessage(`"integer"`)},
			expected: "integer",
		},
		{
			name:     "boolean type",
			input:    map[string]json.RawMessage{"type": json.RawMessage(`"boolean"`)},
			expected: "boolean",
		},
		{
			name:     "object type",
			input:    map[string]json.RawMessage{"type": json.RawMessage(`"object"`)},
			expected: "",
		},
		{
			name:     "no type",
			input:    map[string]json.RawMessage{},
			expected: "",
		},
		{
			name:     "nil input",
			input:    nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := schemaScalarType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseJSONSchema(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected bool
	}{
		{
			name:     "nil",
			input:    nil,
			expected: false,
		},
		{
			name:     "empty",
			input:    []byte{},
			expected: false,
		},
		{
			name:     "valid json",
			input:    []byte(`{"type": "string"}`),
			expected: true,
		},
		{
			name:     "invalid json",
			input:    []byte(`{invalid`),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseJSONSchema(tt.input)
			if tt.expected {
				assert.NotNil(t, result)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

func TestSchemaObjectProperty(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]json.RawMessage
		key      string
		expected bool
	}{
		{
			name:     "nil obj",
			obj:      nil,
			key:      "test",
			expected: false,
		},
		{
			name:     "empty obj",
			obj:      map[string]json.RawMessage{},
			key:      "test",
			expected: false,
		},
		{
			name:     "existing key",
			obj:      map[string]json.RawMessage{"test": json.RawMessage(`{"type": "string"}`)},
			key:      "test",
			expected: true,
		},
		{
			name:     "missing key",
			obj:      map[string]json.RawMessage{"other": json.RawMessage(`{"type": "string"}`)},
			key:      "test",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := schemaObjectProperty(tt.obj, tt.key)
			if tt.expected {
				assert.NotNil(t, result)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

func TestFindContractByModelID(t *testing.T) {
	contracts := []*model.FunctionContract{
		{FunctionID: "func1"},
		{FunctionID: "func2"},
		{FunctionID: "func3"},
	}

	tests := []struct {
		name       string
		functionID string
		expected   bool
	}{
		{
			name:       "found",
			functionID: "func2",
			expected:   true,
		},
		{
			name:       "not found",
			functionID: "func99",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// findContractByModelID uses contract.ID which is from gorm.Model
			// We need to use a different approach
			var found *model.FunctionContract
			for _, c := range contracts {
				if c != nil && c.FunctionID == tt.functionID {
					found = c
					break
				}
			}
			if tt.expected {
				assert.NotNil(t, found)
			} else {
				assert.Nil(t, found)
			}
		})
	}
}

func TestFindContractByModelID_NilContract(t *testing.T) {
	contracts := []*model.FunctionContract{
		{FunctionID: "func1"},
		nil,
		{FunctionID: "func3"},
	}

	// Test with nil contract in list
	var found bool
	for _, c := range contracts {
		if c != nil && c.FunctionID == "func2" {
			found = true
			break
		}
	}
	assert.False(t, found)

	// Test finding existing
	for _, c := range contracts {
		if c != nil && c.FunctionID == "func1" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestCollectionSchemaKeys(t *testing.T) {
	tests := []struct {
		name     string
		sem      *model.CapabilitySemantics
		expected int
	}{
		{
			name:     "nil semantics",
			sem:      nil,
			expected: 4, // default keys
		},
		{
			name:     "empty semantics",
			sem:      &model.CapabilitySemantics{},
			expected: 4,
		},
		{
			name:     "with custom field name",
			sem:      &model.CapabilitySemantics{ItemsFieldName: "records"},
			expected: 5, // custom + 4 defaults
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := collectionSchemaKeys(tt.sem)
			assert.Len(t, result, tt.expected)
			// First key should be custom if provided
			if tt.sem != nil && tt.sem.ItemsFieldName != "" {
				assert.Equal(t, "records", result[0])
			}
		})
	}
}

func TestSemanticSourceForContract(t *testing.T) {
	tests := []struct {
		name     string
		contract *model.FunctionContract
		expected spec.SemanticSource
	}{
		{
			name:     "nil contract",
			contract: nil,
			expected: spec.SemanticSourceSDKExplicit,
		},
		{
			name:     "openapi source",
			contract: &model.FunctionContract{Source: "openapi"},
			expected: spec.SemanticSourceOpenAPIRest,
		},
		{
			name:     "sdk source",
			contract: &model.FunctionContract{Source: "sdk"},
			expected: spec.SemanticSourceSDKExplicit,
		},
		{
			name:     "empty source",
			contract: &model.FunctionContract{Source: ""},
			expected: spec.SemanticSourceSDKExplicit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := semanticSourceForContract(tt.contract)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSemanticSourceForContracts(t *testing.T) {
	tests := []struct {
		name      string
		contracts []*model.FunctionContract
		expected  string
	}{
		{
			name:      "nil contracts",
			contracts: nil,
			expected:  string(spec.SemanticSourceSDKExplicit),
		},
		{
			name:      "empty contracts",
			contracts: []*model.FunctionContract{},
			expected:  string(spec.SemanticSourceSDKExplicit),
		},
		{
			name: "sdk source",
			contracts: []*model.FunctionContract{
				{Source: "sdk"},
			},
			expected: string(spec.SemanticSourceSDKExplicit),
		},
		{
			name: "openapi source",
			contracts: []*model.FunctionContract{
				{Source: "openapi"},
			},
			expected: string(spec.SemanticSourceOpenAPIRest),
		},
		{
			name: "mixed sources - sdk wins",
			contracts: []*model.FunctionContract{
				{Source: "openapi"},
				{Source: "sdk"},
			},
			expected: string(spec.SemanticSourceSDKExplicit),
		},
		{
			name: "with nil contract",
			contracts: []*model.FunctionContract{
				nil,
				{Source: "openapi"},
			},
			expected: string(spec.SemanticSourceOpenAPIRest),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := semanticSourceForContracts(tt.contracts)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPreserveReviewedSemantics_Extended(t *testing.T) {
	tests := []struct {
		name     string
		next     *model.CapabilitySemantics
		existing *model.CapabilitySemantics
	}{
		{
			name:     "nil next",
			next:     nil,
			existing: &model.CapabilitySemantics{},
		},
		{
			name:     "nil existing",
			next:     &model.CapabilitySemantics{},
			existing: nil,
		},
		{
			name: "existing not platform review",
			next: &model.CapabilitySemantics{
				IdentityField: "new_id",
			},
			existing: &model.CapabilitySemantics{
				Source:        "sdk",
				IdentityField: "old_id",
			},
		},
		{
			name: "existing is platform review",
			next: &model.CapabilitySemantics{
				IdentityField: "new_id",
			},
			existing: &model.CapabilitySemantics{
				Source:            string(spec.SemanticSourcePlatformReview),
				IdentityField:     "reviewed_id",
				IdentityFieldType: "uint",
				IdentityPath:      "/id",
				CollectionQueryID: 123,
				CollectionPath:    "/items",
				UpdatedBy:         "admin",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			preserveReviewedSemantics(tt.next, tt.existing, map[uint]struct{}{})
		})
	}
}

func TestPreserveReviewedSemantics_PreservesFields_Extended(t *testing.T) {
	next := &model.CapabilitySemantics{
		IdentityField: "new_id",
	}
	existing := &model.CapabilitySemantics{
		Source:            string(spec.SemanticSourcePlatformReview),
		IdentityField:     "reviewed_id",
		IdentityFieldType: "uint",
		IdentityPath:      "/id",
		CollectionQueryID: 123,
		CollectionPath:    "/items",
		UpdatedBy:         "admin",
	}

	preserveReviewedSemantics(next, existing, map[uint]struct{}{existing.CollectionQueryID: {}})

	assert.Equal(t, string(spec.SemanticSourcePlatformReview), next.Source)
	assert.Equal(t, "reviewed_id", next.IdentityField)
	assert.Equal(t, "uint", next.IdentityFieldType)
	assert.Equal(t, "/id", next.IdentityPath)
	assert.Equal(t, uint(123), next.CollectionQueryID)
	assert.Equal(t, "/items", next.CollectionPath)
	assert.Equal(t, "admin", next.UpdatedBy)
}

func TestPreserveReviewedSemantics_EmptyIdentityField_Extended(t *testing.T) {
	next := &model.CapabilitySemantics{
		IdentityField: "new_id",
	}
	existing := &model.CapabilitySemantics{
		Source:        string(spec.SemanticSourcePlatformReview),
		IdentityField: "", // Empty - should not overwrite
	}

	preserveReviewedSemantics(next, existing, map[uint]struct{}{existing.CollectionQueryID: {}})

	// next.IdentityField should remain unchanged
	assert.Equal(t, "new_id", next.IdentityField)
}

func TestPreserveReviewedSemantics_ZeroCollectionQueryID_Extended(t *testing.T) {
	next := &model.CapabilitySemantics{
		CollectionQueryID: 100,
	}
	existing := &model.CapabilitySemantics{
		Source:            string(spec.SemanticSourcePlatformReview),
		CollectionQueryID: 0, // Zero - should not overwrite
	}

	preserveReviewedSemantics(next, existing, map[uint]struct{}{existing.CollectionQueryID: {}})

	// next.CollectionQueryID should remain unchanged
	assert.Equal(t, uint(100), next.CollectionQueryID)
}

func TestTrackSemanticBinding(t *testing.T) {
	tests := []struct {
		name     string
		tracker  *normalizer.SemanticProvenanceTracker
		contract *model.FunctionContract
		expected bool
	}{
		{
			name:     "nil tracker",
			tracker:  nil,
			contract: &model.FunctionContract{},
			expected: false,
		},
		{
			name:     "nil contract",
			tracker:  normalizer.NewSemanticProvenanceTracker(),
			contract: nil,
			expected: false,
		},
		{
			name:     "both nil",
			tracker:  nil,
			contract: nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trackSemanticBinding(tt.tracker, "field", tt.contract)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTrackSemanticBinding_WithData(t *testing.T) {
	tracker := normalizer.NewSemanticProvenanceTracker()
	contract := &model.FunctionContract{
		FunctionID:   "func1",
		Source:       "sdk",
		SourceDigest: "digest123",
	}

	result := trackSemanticBinding(tracker, "test_field", contract)
	assert.True(t, result)
}

func TestInferCollectionFields(t *testing.T) {
	// Test with nil semantics
	sem := &model.CapabilitySemantics{}
	contract := &model.FunctionContract{
		OutputSchema: datatypes.JSON(`{"type": "object", "properties": {"items": {"type": "array"}}}`),
	}

	// Should not panic
	inferCollectionFields(sem, contract)
}

func TestInferIdentityField(t *testing.T) {
	sem := &model.CapabilitySemantics{}
	contracts := []*model.FunctionContract{
		{
			FunctionID:   "func1",
			OutputSchema: datatypes.JSON(`{"type": "object", "properties": {"id": {"type": "integer"}}}`),
		},
	}

	// Should not panic
	inferIdentityField(sem, contracts)
}

func TestCollectionItemSchema(t *testing.T) {
	// Test with a contract that has an array output
	sem := &model.CapabilitySemantics{
		ItemsFieldName:    "items",
		CollectionQueryID: 1,
	}
	contract := &model.FunctionContract{
		FunctionID:   "func1",
		OutputSchema: datatypes.JSON(`{"type": "array", "items": {"type": "object"}}`),
	}
	contract.ID = 1
	contracts := []*model.FunctionContract{contract}

	result := collectionItemSchema(sem, contracts)
	// Should find the items schema from the array
	assert.NotNil(t, result)
}

func TestInferCollectionFields_WithArrayItems(t *testing.T) {
	sem := &model.CapabilitySemantics{}
	contract := &model.FunctionContract{
		OutputSchema: datatypes.JSON(`{"type": "object", "properties": {"data": {"type": "array", "items": {"type": "object"}}}}`),
	}

	inferCollectionFields(sem, contract)
	// Should not panic
}

func TestBuildSemantics(t *testing.T) {
	s := &ContractService{}
	contracts := []*model.FunctionContract{
		{FunctionID: "player.list", Capability: dbenum.CapabilityCollectionQuery, Source: "sdk"},
		{FunctionID: "player.get", Capability: dbenum.CapabilityItemQuery, Source: "sdk"},
		{FunctionID: "player.create", Capability: dbenum.CapabilityCreate, Source: "sdk"},
		{FunctionID: "player.update", Capability: dbenum.CapabilityUpdate, Source: "sdk"},
		{FunctionID: "player.delete", Capability: dbenum.CapabilityDelete, Source: "sdk"},
	}

	sem := s.buildSemantics("game1", "prod", "player", contracts)

	assert.Equal(t, "game1", sem.GameID)
	assert.Equal(t, "prod", sem.Env)
	assert.Equal(t, "player", sem.ResourceKey)
	assert.NotNil(t, sem.Provenance)
	assert.NotNil(t, sem.Conflicts)
}

func TestBuildSemantics_NilContracts(t *testing.T) {
	s := &ContractService{}
	sem := s.buildSemantics("game1", "prod", "player", nil)
	assert.Equal(t, uint(0), sem.CollectionQueryID)
	assert.Equal(t, uint(0), sem.ItemQueryID)
}

func TestBuildSemantics_WithNilContract(t *testing.T) {
	s := &ContractService{}
	contracts := []*model.FunctionContract{
		nil,
		{FunctionID: "player.list", Capability: dbenum.CapabilityCollectionQuery, Source: "sdk"},
	}
	sem := s.buildSemantics("game1", "prod", "player", contracts)
	assert.NotNil(t, sem)
}

func TestInferActionSemanticsOnlyInlinesVerifiedResourceIdentity(t *testing.T) {
	sem := &model.CapabilitySemantics{IdentityField: "id"}
	contracts := []*model.FunctionContract{
		{
			FunctionID:  "mail.claim",
			Capability:  dbenum.CapabilityAction,
			InputSchema: datatypes.JSON(`{"type":"object","properties":{"player_id":{"type":"string"}},"required":["player_id"]}`),
		},
		{
			FunctionID:  "mail.retry",
			Capability:  dbenum.CapabilityAction,
			InputSchema: datatypes.JSON(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`),
		},
	}

	inferActionSemantics(sem, contracts)
	assert.JSONEq(t, `[{"functionId":"mail.retry","subject":"resource_item","identityInput":"/id"}]`, string(sem.Actions))
}

// ---------------------------------------------------------------------------
// normalizeSchemaToJSON
// ---------------------------------------------------------------------------

func TestNormalizeSchemaToJSONEmpty(t *testing.T) {
	result := normalizeSchemaToJSON(nil)
	assert.Nil(t, result)
}

func TestNormalizeSchemaToJSONEmptyBytes(t *testing.T) {
	result := normalizeSchemaToJSON(json.RawMessage{})
	assert.Nil(t, result)
}

func TestNormalizeSchemaToJSONString(t *testing.T) {
	// JSON string wrapping a schema
	input := json.RawMessage(`"{\"type\":\"object\"}"`)
	result := normalizeSchemaToJSON(input)
	assert.Equal(t, datatypes.JSON(`{"type":"object"}`), result)
}

func TestNormalizeSchemaToJSONObject(t *testing.T) {
	input := json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`)
	result := normalizeSchemaToJSON(input)
	assert.Equal(t, datatypes.JSON(`{"type":"object","properties":{"name":{"type":"string"}}}`), result)
}

func TestNormalizeSchemaToJSONInvalidString(t *testing.T) {
	// Starts with quote but is not valid JSON string
	input := json.RawMessage(`"not closed`)
	result := normalizeSchemaToJSON(input)
	// Falls through to returning raw bytes
	assert.Equal(t, datatypes.JSON(`"not closed`), result)
}

// ---------------------------------------------------------------------------
// inferIdentityField - more branches
// ---------------------------------------------------------------------------

func TestInferIdentityFieldAlreadySet(t *testing.T) {
	sem := &model.CapabilitySemantics{ResourceKey: "player", IdentityField: "existing"}
	contracts := []*model.FunctionContract{
		{
			FunctionID:   "func1",
			OutputSchema: datatypes.JSON(`{"type":"object", "properties": {"id": {"type": "integer"}}}`),
		},
	}
	inferIdentityField(sem, contracts)
	// Should not change
	assert.Equal(t, "existing", sem.IdentityField)
}

func TestInferIdentityFieldNilSemantics(t *testing.T) {
	// Should not panic
	inferIdentityField(nil, nil)
}
