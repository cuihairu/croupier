package openapi

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SUI-011 acceptance: the request contract is classified into the tier the
// generator can reach, with the field path of every construct that lowered it.
// Each construct family gets its own case; a deterministic order is asserted
// because the diagnostics are stored on the contract and replayed into the
// proposal.

func compatibilityRequest(bodySchema string) *RequestContract {
	return &RequestContract{
		Body: &RequestBody{
			Required: true,
			Content: map[string]RequestMedia{
				"application/json": {Schema: json.RawMessage(bodySchema)},
			},
			Origin: RequestOrigin{Pointer: "#/paths/~1things/post/requestBody"},
		},
	}
}

func compatibilityRoot(t *testing.T) map[string]any {
	t.Helper()
	var root map[string]any
	require.NoError(t, json.Unmarshal([]byte(`{
		"openapi": "3.1.0",
		"components": {
			"schemas": {
				"Thing": {"type": "object", "properties": {"id": {"type": "string"}}}
			}
		}
	}`), &root))
	return root
}

func TestAssessRequestCompatibility_Direct(t *testing.T) {
	got := AssessRequestCompatibility("listThings", compatibilityRequest(`{"type":"object","properties":{"limit":{"type":"integer"}}}`), compatibilityRoot(t))
	assert.Equal(t, spec.RequestCompatibilityDirect, got.Status)
	assert.Empty(t, got.Diagnostics)
}

func TestAssessRequestCompatibility_NilRequest(t *testing.T) {
	got := AssessRequestCompatibility("listThings", nil, compatibilityRoot(t))
	assert.Equal(t, spec.RequestCompatibilityDirect, got.Status)
	assert.Empty(t, got.Diagnostics)
}

func TestAssessRequestCompatibility_CompositeSchemaIsDegraded(t *testing.T) {
	cases := map[string]map[string]any{
		"allOf": {"type": "object", "allOf": []any{map[string]any{"$ref": "#/components/schemas/Thing"}}},
		"oneOf": {"type": "object", "oneOf": []any{map[string]any{"type": "object"}, map[string]any{"type": "object"}}},
		"anyOf": {"type": "object", "anyOf": []any{map[string]any{"type": "object"}, map[string]any{"type": "object"}}},
	}
	for keyword, schema := range cases {
		raw, err := json.Marshal(schema)
		require.NoError(t, err)
		got := AssessRequestCompatibility("createThing", compatibilityRequest(string(raw)), compatibilityRoot(t))
		assert.Equal(t, spec.RequestCompatibilityDegraded, got.Status, keyword)
		require.NotEmpty(t, got.Diagnostics, keyword)
		assert.Contains(t, got.Diagnostics[0].Field, keyword, keyword)
		assert.Equal(t, spec.DiagSchemaCompatibilityDegraded, got.Diagnostics[0].Code, keyword)
	}
}

func TestAssessRequestCompatibility_DiscriminatorIsManual(t *testing.T) {
	raw := []byte(`{"type":"object","oneOf":[{"type":"object"},{"type":"object"}],"discriminator":{"propertyName":"kind"}}`)
	got := AssessRequestCompatibility("createThing", compatibilityRequest(string(raw)), compatibilityRoot(t))
	assert.Equal(t, spec.RequestCompatibilityManual, got.Status)
	var found bool
	for _, diagnostic := range got.Diagnostics {
		if diagnostic.Code == spec.DiagSchemaCompatibilityManual && diagnostic.Field == "$.body.content.application~1json/schema/discriminator" {
			found = true
		}
	}
	assert.True(t, found, "expected the discriminator path, got %+v", got.Diagnostics)
}

func TestAssessRequestCompatibility_DanglingReferenceIsManual(t *testing.T) {
	raw := []byte(`{"type":"object","properties":{"thing":{"$ref":"#/components/schemas/Missing"}}}`)
	got := AssessRequestCompatibility("createThing", compatibilityRequest(string(raw)), compatibilityRoot(t))
	assert.Equal(t, spec.RequestCompatibilityManual, got.Status)
	require.NotEmpty(t, got.Diagnostics)
	assert.Contains(t, got.Diagnostics[0].Message, "#/components/schemas/Missing")
	assert.Equal(t, spec.DiagSchemaCompatibilityManual, got.Diagnostics[0].Code)
}

func TestAssessRequestCompatibility_RemoteReferenceIsManual(t *testing.T) {
	raw := []byte(`{"type":"object","properties":{"thing":{"$ref":"https://example.com/schemas/Thing"}}}`)
	got := AssessRequestCompatibility("createThing", compatibilityRequest(string(raw)), compatibilityRoot(t))
	assert.Equal(t, spec.RequestCompatibilityManual, got.Status)
	var found bool
	for _, diagnostic := range got.Diagnostics {
		if diagnostic.Code == spec.DiagSchemaCompatibilityManual && diagnostic.Field == "https://example.com/schemas/Thing" {
			found = true
		}
	}
	assert.True(t, found, "expected the unresolved reference finding, got %+v", got.Diagnostics)
}

func TestAssessRequestCompatibility_RootArrayBodyIsDegraded(t *testing.T) {
	got := AssessRequestCompatibility("bulkCreate", compatibilityRequest(`{"type":"array","items":{"type":"object"}}`), compatibilityRoot(t))
	assert.Equal(t, spec.RequestCompatibilityDegraded, got.Status)
	require.NotEmpty(t, got.Diagnostics)
	assert.Contains(t, got.Diagnostics[0].Message, "root array")
}

func TestAssessRequestCompatibility_BareValueBodyIsDegraded(t *testing.T) {
	got := AssessRequestCompatibility("echo", compatibilityRequest(`{"type":"string"}`), compatibilityRoot(t))
	assert.Equal(t, spec.RequestCompatibilityDegraded, got.Status)
	require.NotEmpty(t, got.Diagnostics)
	assert.Contains(t, got.Diagnostics[0].Message, "bare string value")
}

// SUI-012: form-urlencoded is a media type the executor serializes, so a body
// that declares only that type is sendable rather than unsendable. It stays
// out of the directly executable tier because the serializer is chosen by the
// executor config, which the generated page cannot express.
func TestAssessRequestCompatibility_FormURLBodyMediaIsDegraded(t *testing.T) {
	request := &RequestContract{Body: &RequestBody{Content: map[string]RequestMedia{
		"application/x-www-form-urlencoded": {Schema: json.RawMessage(`{"type":"object"}`)},
	}}}
	got := AssessRequestCompatibility("login", request, compatibilityRoot(t))
	assert.Equal(t, spec.RequestCompatibilityDegraded, got.Status)
	var media, noJSON bool
	for _, diagnostic := range got.Diagnostics {
		if diagnostic.Field == "$.body.content.application~1x-www-form-urlencoded" {
			media = true
		}
		if diagnostic.Field == "$.body.content" {
			noJSON = true
		}
	}
	assert.True(t, media, "expected the media finding, got %+v", got.Diagnostics)
	assert.True(t, noJSON, "expected the missing-json finding, got %+v", got.Diagnostics)
}

func TestAssessRequestCompatibility_MultipartOnlyBodyIsManual(t *testing.T) {
	request := &RequestContract{Body: &RequestBody{Content: map[string]RequestMedia{
		"multipart/form-data": {Schema: json.RawMessage(`{"type":"object"}`)},
	}}}
	got := AssessRequestCompatibility("upload", request, compatibilityRoot(t))
	assert.Equal(t, spec.RequestCompatibilityManual, got.Status)
	var unsendable bool
	for _, diagnostic := range got.Diagnostics {
		if diagnostic.Field == "$.body.content.multipart~1form-data" {
			unsendable = true
		}
	}
	assert.True(t, unsendable, "expected the unsendable media finding, got %+v", got.Diagnostics)
}

func TestAssessRequestCompatibility_JSONBesideMultipartIsManual(t *testing.T) {
	request := &RequestContract{Body: &RequestBody{Content: map[string]RequestMedia{
		"application/json":    {Schema: json.RawMessage(`{"type":"object"}`)},
		"multipart/form-data": {Schema: json.RawMessage(`{"type":"object"}`)},
	}}}
	got := AssessRequestCompatibility("upload", request, compatibilityRoot(t))
	assert.Equal(t, spec.RequestCompatibilityManual, got.Status)
	var multipart bool
	for _, diagnostic := range got.Diagnostics {
		if diagnostic.Field == "$.body.content.multipart~1form-data" {
			multipart = true
		}
	}
	assert.True(t, multipart, "expected the multipart path, got %+v", got.Diagnostics)
}

func TestAssessRequestCompatibility_ParameterContentIsDegraded(t *testing.T) {
	request := &RequestContract{Parameters: []RequestParameter{{
		In:      "query",
		Name:    "filter",
		Content: map[string]RequestMedia{"application/xml": {Schema: json.RawMessage(`{"type":"object"}`)}},
	}}}
	got := AssessRequestCompatibility("listThings", request, compatibilityRoot(t))
	assert.Equal(t, spec.RequestCompatibilityDegraded, got.Status)
	require.NotEmpty(t, got.Diagnostics)
	assert.Contains(t, got.Diagnostics[0].Field, "parameters[0]/content")
}

func TestAssessRequestCompatibility_ManualWinsOverDegraded(t *testing.T) {
	raw := []byte(`{"type":"object","allOf":[],"discriminator":{"propertyName":"kind"}}`)
	got := AssessRequestCompatibility("createThing", compatibilityRequest(string(raw)), compatibilityRoot(t))
	assert.Equal(t, spec.RequestCompatibilityManual, got.Status)
}

func TestAssessRequestCompatibility_IsDeterministic(t *testing.T) {
	raw := []byte(`{"type":"object","oneOf":[{"type":"object","allOf":[]},{"type":"object"}],"discriminator":{"propertyName":"kind"}}`)
	first := AssessRequestCompatibility("createThing", compatibilityRequest(string(raw)), compatibilityRoot(t))
	second := AssessRequestCompatibility("createThing", compatibilityRequest(string(raw)), compatibilityRoot(t))
	assert.Equal(t, first, second)
}
