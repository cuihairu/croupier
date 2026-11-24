package openapi

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/platform/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SUI-008 验收：契约提取把无法内联的 schema 引用显式标记待审，取代旧
// 「缺失/循环引用静默坍缩为匿名对象、外部引用保留但无信号」。嵌套本地引用
// 照常内联（无诊断）；循环引用在深度上限截断并记 cyclic；缺失引用坍缩为
// {"type":"object"} 并记 missing；外部引用原样保留 {"$ref":...} 并记
// external。同一引用按 (ref, reason) 去重。

func refTestRequest(bodySchema string) *RequestContract {
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

func refIssueOf(t *testing.T, projection InputProjection) spec.Diagnostic {
	t.Helper()
	require.Len(t, projection.Diagnostics, 1, "expected exactly one diagnostic")
	return projection.Diagnostics[0]
}

func refIssueReason(t *testing.T, projection InputProjection) string {
	t.Helper()
	diag := refIssueOf(t, projection)
	require.Equal(t, spec.DiagSchemaReferenceUnresolved, diag.Code)
	require.Equal(t, spec.SeverityWarning, diag.Severity)
	require.Equal(t, "op", diag.FunctionID)
	prefix := "schema reference " + diag.Field + " could not be inlined ("
	require.True(t, strings.HasPrefix(diag.Message, prefix), diag.Message)
	inner := diag.Message[len(prefix):]
	return inner[:strings.Index(inner, ")")]
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return string(encoded)
}

func TestSchemaRef_NestedLocalRefsResolve(t *testing.T) {
	root := map[string]any{
		"components": map[string]any{
			"schemas": map[string]any{
				"Profile": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"avatar": map[string]any{"$ref": "#/components/schemas/Avatar"},
					},
				},
				"Avatar": map[string]any{"type": "string", "format": "uri"},
			},
		},
	}
	request := refTestRequest(`{"type":"object","properties":{"profile":{"$ref":"#/components/schemas/Profile"}}}`)
	projection := ProjectInput("op", request, root)
	require.Empty(t, projection.Diagnostics)
	assert.False(t, projection.Grouped)
	var schema map[string]any
	require.NoError(t, json.Unmarshal([]byte(projection.Schema), &schema))
	props := schema["properties"].(map[string]any)
	require.JSONEq(t, `{"type":"object","properties":{"avatar":{"type":"string","format":"uri"}}}`,
		mustJSON(t, props["profile"]))
}

func TestSchemaRef_CyclicRefRecordsDiagnostic(t *testing.T) {
	schemas := map[string]any{
		"Profile": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"owner": map[string]any{"$ref": "#/components/schemas/Owner"},
			},
		},
		"Owner": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"profile": map[string]any{"$ref": "#/components/schemas/Profile"},
			},
		},
	}
	root := map[string]any{"components": map[string]any{"schemas": schemas}}
	request := refTestRequest(`{"type":"object","properties":{"profile":{"$ref":"#/components/schemas/Profile"}}}`)
	projection := ProjectInput("op", request, root)
	assert.Equal(t, refReasonCyclic, refIssueReason(t, projection))
	assert.Equal(t, "#/components/schemas/Profile", refIssueOf(t, projection).Field)
	// The projection still yields a valid standalone schema (deterministic
	// truncation), just marked for review.
	var schema map[string]any
	require.NoError(t, json.Unmarshal([]byte(projection.Schema), &schema))
}

func TestSchemaRef_MissingRefRecordsDiagnostic(t *testing.T) {
	root := map[string]any{"components": map[string]any{"schemas": map[string]any{}}}
	request := refTestRequest(`{"type":"object","properties":{"ghost":{"$ref":"#/components/schemas/DoesNotExist"}}}`)
	projection := ProjectInput("op", request, root)
	assert.Equal(t, refReasonMissing, refIssueReason(t, projection))
	var schema map[string]any
	require.NoError(t, json.Unmarshal([]byte(projection.Schema), &schema))
	props := schema["properties"].(map[string]any)
	// The dangling reference collapses to an anonymous object, now marked.
	require.JSONEq(t, `{"type":"object"}`, mustJSON(t, props["ghost"]))
}

func TestSchemaRef_ExternalRefRecordsDiagnostic(t *testing.T) {
	request := refTestRequest(`{"type":"object","properties":{"remote":{"$ref":"other.yaml#/components/schemas/Thing"}}}`)
	projection := ProjectInput("op", request, map[string]any{})
	assert.Equal(t, refReasonExternal, refIssueReason(t, projection))
	var schema map[string]any
	require.NoError(t, json.Unmarshal([]byte(projection.Schema), &schema))
	props := schema["properties"].(map[string]any)
	// External references are preserved verbatim, not dropped.
	require.JSONEq(t, `{"$ref":"other.yaml#/components/schemas/Thing"}`, mustJSON(t, props["remote"]))
}

func TestSchemaRef_SameRefDedupedPerContract(t *testing.T) {
	root := map[string]any{"components": map[string]any{"schemas": map[string]any{}}}
	request := refTestRequest(`{"type":"object","properties":{"a":{"$ref":"#/components/schemas/Nope"},"b":{"$ref":"#/components/schemas/Nope"}}}`)
	projection := ProjectInput("op", request, root)
	require.Len(t, projection.Diagnostics, 1)
	assert.Equal(t, "#/components/schemas/Nope", projection.Diagnostics[0].Field)
}

func TestSchemaRef_ParameterRefCollected(t *testing.T) {
	root := map[string]any{"components": map[string]any{"schemas": map[string]any{}}}
	request := &RequestContract{
		Parameters: []RequestParameter{{
			In: "query", Name: "ghost", Required: false,
			Schema: json.RawMessage(`{"$ref":"#/components/schemas/Nope"}`),
			Origin: RequestOrigin{Pointer: "#/paths/~1things/get/parameters/0"},
		}},
	}
	projection := ProjectInput("op", request, root)
	require.Len(t, projection.Diagnostics, 1)
	assert.Equal(t, refReasonMissing, refIssueReason(t, projection))
}

// The provider's map fallback path (kin refuses dangling references) records
// the same diagnostics through GetMethodDetails.
func TestSchemaRef_FallbackDiscoveryRecordsDiagnostic(t *testing.T) {
	specYAML := `openapi: 3.0.3
info:
  title: dangling
  version: "1"
paths:
  /things:
    post:
      operationId: createThing
      requestBody:
        content:
          application/json:
            schema: {$ref: '#/components/schemas/Nope'}
      responses:
        "204":
          description: created
`
	dir := t.TempDir()
	path := filepath.Join(dir, "dangling.yaml")
	require.NoError(t, os.WriteFile(path, []byte(specYAML), 0o644))

	p := NewProvider()
	require.NoError(t, p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config:  map[string]interface{}{"openapiSpec": path},
	}))
	details := p.GetMethodDetails()
	method, ok := details["createThing"]
	require.True(t, ok)
	require.NotEmpty(t, method.Diagnostics)
	var found bool
	for _, diagnostic := range method.Diagnostics {
		if diagnostic.Code == spec.DiagSchemaReferenceUnresolved {
			assert.Equal(t, "#/components/schemas/Nope", diagnostic.Field)
			assert.Contains(t, diagnostic.Message, refReasonMissing)
			assert.Equal(t, "createThing", diagnostic.FunctionID)
			found = true
		}
	}
	require.True(t, found, "fallback discovery must record the unresolved-reference diagnostic")
}
