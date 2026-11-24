package spec

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SUI-009 验收：归一化 OpenAPI 3.0 与 3.1 schema 方言。对照测试覆盖 3.0
// `nullable`、3.1 联合类型、缺失值与显式 null；条件 schema、元组等关键字
// 明确给出能力判定（degraded 诊断），绝不静默误渲染。前端与后端对同一批
// 样例的判定一致（web/src/utils/schemaDialect.ts 镜像，样例集锁定于支持
// 矩阵文档）。

func mustParse(t *testing.T, text string) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(text), &out))
	return out
}

func jsonOf(t *testing.T, v interface{}) string {
	t.Helper()
	encoded, err := json.Marshal(v)
	require.NoError(t, err)
	return string(encoded)
}

// --- 3.0 `nullable` 折叠 ---

func TestNormalizeDialect_NullableTrueFoldsIntoType(t *testing.T) {
	in := mustParse(t, `{"type":"string","nullable":true}`)
	out := NormalizeSchemaDialect(in, SchemaDialectOpenAPI30, nil)
	assert.Equal(t, `{"type":["string","null"]}`, jsonOf(t, out))
}

func TestNormalizeDialect_NullableFalseRemoved(t *testing.T) {
	in := mustParse(t, `{"type":"string","nullable":false}`)
	out := NormalizeSchemaDialect(in, SchemaDialectOpenAPI30, nil)
	assert.Equal(t, `{"type":"string"}`, jsonOf(t, out))
}

func TestNormalizeDialect_NullableWithoutTypeIsPermissive(t *testing.T) {
	// nullable with no type = any value or null, equivalent to no constraint.
	in := mustParse(t, `{"nullable":true,"description":"d"}`)
	out := NormalizeSchemaDialect(in, SchemaDialectOpenAPI30, nil)
	assert.Equal(t, `{"description":"d"}`, jsonOf(t, out))
}

func TestNormalizeDialect_NullableObjectType(t *testing.T) {
	in := mustParse(t, `{"type":"object","nullable":true}`)
	out := NormalizeSchemaDialect(in, SchemaDialectOpenAPI30, nil)
	assert.Equal(t, `{"type":["object","null"]}`, jsonOf(t, out))
}

// --- 3.1 联合类型 ---

func TestNormalizeDialect_31NullUnionMovesNullLast(t *testing.T) {
	// 3.1 declares null first; canonical form must move it last so the primary
	// (non-null) type is the first element.
	in := mustParse(t, `{"type":["null","string"]}`)
	out := NormalizeSchemaDialect(in, SchemaDialectOpenAPI31, nil)
	assert.Equal(t, `{"type":["string","null"]}`, jsonOf(t, out))
}

func TestNormalizeDialect_31NullUnionAlreadyCanonical(t *testing.T) {
	in := mustParse(t, `{"type":["string","null"]}`)
	out := NormalizeSchemaDialect(in, SchemaDialectOpenAPI31, nil)
	assert.Equal(t, `{"type":["string","null"]}`, jsonOf(t, out))
}

func TestNormalizeDialect_31MultiTypeUnionPreservedAndNoted(t *testing.T) {
	in := mustParse(t, `{"type":["string","integer"]}`)
	var notes []DialectNote
	out := NormalizeSchemaDialect(in, SchemaDialectOpenAPI31, &notes)
	assert.Equal(t, `{"type":["string","integer"]}`, jsonOf(t, out))
	require.Len(t, notes, 1)
	assert.Equal(t, "type", notes[0].Keyword)
	assert.Equal(t, DialectCapabilityDegraded, notes[0].Capability)
	assert.Contains(t, notes[0].Message, "not directly formable")
}

func TestNormalizeDialect_31MultiTypeWithNullMovesNullLast(t *testing.T) {
	in := mustParse(t, `{"type":["string","integer","null"]}`)
	var notes []DialectNote
	out := NormalizeSchemaDialect(in, SchemaDialectOpenAPI31, &notes)
	assert.Equal(t, `{"type":["string","integer","null"]}`, jsonOf(t, out))
	require.Len(t, notes, 1)
	assert.Equal(t, "type", notes[0].Keyword)
}

// --- 缺失值 vs 显式 null（语义由 required + type-null 共同表达） ---

func TestNormalizeDialect_NullableRequiredDistinguishesMissingFromNull(t *testing.T) {
	in := mustParse(t, `{"type":"object","properties":{"note":{"type":"string","nullable":true}},"required":["note"]}`)
	out := NormalizeSchemaDialect(in, SchemaDialectOpenAPI30, nil)
	// note is required but nullable: missing → error, explicit null → allowed.
	props := out["properties"].(map[string]interface{})
	assert.Equal(t, `{"type":["string","null"]}`, jsonOf(t, props["note"]))
	assert.Equal(t, `["note"]`, jsonOf(t, out["required"]))
}

func TestNormalizeDialect_OptionalNonNullableAllowsMissingNotNull(t *testing.T) {
	// not required and not nullable: missing → allowed, null → rejected.
	in := mustParse(t, `{"type":"object","properties":{"q":{"type":"string"}}}`)
	out := NormalizeSchemaDialect(in, SchemaDialectOpenAPI30, nil)
	props := out["properties"].(map[string]interface{})
	assert.Equal(t, `{"type":"string"}`, jsonOf(t, props["q"]))
	assert.NotContains(t, out, "required")
}

// --- 条件 schema 与元组 ---

func TestNormalizeDialect_ConditionalSchemaNoted(t *testing.T) {
	in := mustParse(t, `{"type":"object","properties":{"x":{"type":"string"},"y":{"type":"string"}},"if":{"properties":{"x":{"const":"a"}}},"then":{"required":["y"]}}`)
	var notes []DialectNote
	out := NormalizeSchemaDialect(in, SchemaDialectOpenAPI30, &notes)
	require.Len(t, notes, 2) // if + then
	keywords := []string{notes[0].Keyword, notes[1].Keyword}
	assert.ElementsMatch(t, []string{"if", "then"}, keywords)
	for _, note := range notes {
		assert.Equal(t, DialectCapabilityDegraded, note.Capability)
		assert.Contains(t, note.Message, "conditional schema")
	}
	// the subschema under if is still normalized recursively
	cond := out["if"].(map[string]interface{})
	assert.Contains(t, cond, "properties")
}

func TestNormalizeDialect_TupleItemsNoted(t *testing.T) {
	in := mustParse(t, `{"type":"array","items":[{"type":"string"},{"type":"integer"}]}`)
	var notes []DialectNote
	out := NormalizeSchemaDialect(in, SchemaDialectOpenAPI30, &notes)
	require.Len(t, notes, 1)
	assert.Equal(t, "items", notes[0].Keyword)
	assert.Equal(t, DialectCapabilityDegraded, notes[0].Capability)
	// tuple item schemas normalized recursively
	items := out["items"].([]interface{})
	assert.Equal(t, `{"type":"string"}`, jsonOf(t, items[0]))
	assert.Equal(t, `{"type":"integer"}`, jsonOf(t, items[1]))
}

func TestNormalizeDialect_PrefixItemsNoted(t *testing.T) {
	in := mustParse(t, `{"type":"array","prefixItems":[{"type":"string"}]}`)
	var notes []DialectNote
	NormalizeSchemaDialect(in, SchemaDialectOpenAPI31, &notes)
	require.Len(t, notes, 1)
	assert.Equal(t, "prefixItems", notes[0].Keyword)
	assert.Equal(t, DialectCapabilityDegraded, notes[0].Capability)
}

// --- 非 schema 值不被误归一 ---

func TestNormalizeDialect_NonSchemaValuesPassThrough(t *testing.T) {
	// default / example / enum 是任意 JSON 值，绝不能被当作 schema 归一。
	in := mustParse(t, `{"type":"object","properties":{"mode":{"type":"string","enum":["a","b"],"default":{"nullable":true},"example":{"type":"integer"}}}}`)
	out := NormalizeSchemaDialect(in, SchemaDialectOpenAPI30, nil)
	props := out["properties"].(map[string]interface{})
	mode := props["mode"].(map[string]interface{})
	// default 与 example 保持原样（不是 schema）
	assert.Equal(t, `{"nullable":true}`, jsonOf(t, mode["default"]))
	assert.Equal(t, `{"type":"integer"}`, jsonOf(t, mode["example"]))
}

func TestNormalizeDialect_NestedPropertiesRecursive(t *testing.T) {
	in := mustParse(t, `{"type":"object","properties":{"address":{"type":"object","properties":{"city":{"type":"string","nullable":true}}}}}`)
	out := NormalizeSchemaDialect(in, SchemaDialectOpenAPI30, nil)
	props := out["properties"].(map[string]interface{})
	address := props["address"].(map[string]interface{})
	addrProps := address["properties"].(map[string]interface{})
	assert.Equal(t, `{"type":["string","null"]}`, jsonOf(t, addrProps["city"]))
}

func TestNormalizeDialect_AllOfSubschemasNormalized(t *testing.T) {
	in := mustParse(t, `{"allOf":[{"type":"object","properties":{"a":{"type":"string","nullable":true}}},{"type":"object"}]}`)
	out := NormalizeSchemaDialect(in, SchemaDialectOpenAPI30, nil)
	allOf := out["allOf"].([]interface{})
	first := allOf[0].(map[string]interface{})
	props := first["properties"].(map[string]interface{})
	assert.Equal(t, `{"type":["string","null"]}`, jsonOf(t, props["a"]))
}

// --- 无方言关键字时字节稳定（SUI-004 保真） ---

func TestNormalizeDialect_NoDialectKeywordsByteStable(t *testing.T) {
	// 无方言关键字（无 nullable / type 数组 / 条件 / 元组）的 schema 归一后
	// 与原值字节一致（json.Marshal 按键排序，两侧同为 map 序列化）。
	text := `{"type":"object","properties":{"id":{"type":"string"},"count":{"type":"integer","minimum":0}},"required":["id"]}`
	in := mustParse(t, text)
	out := NormalizeSchemaDialect(in, SchemaDialectOpenAPI30, nil)
	assert.Equal(t, jsonOf(t, in), jsonOf(t, out))
}

// --- ClassifySchemaSupport（前端/后端判定一致的基础） ---

func TestClassifySchemaSupport_Samples(t *testing.T) {
	cases := []struct {
		name     string
		schema   string
		expected DialectCapability
	}{
		{"plain object", `{"type":"object","properties":{"id":{"type":"string"}}}`, DialectCapabilitySupported},
		{"nullable single type", `{"type":"string","nullable":true}`, DialectCapabilitySupported},
		{"31 null union", `{"type":["string","null"]}`, DialectCapabilitySupported},
		{"multi-type union", `{"type":["string","integer"]}`, DialectCapabilityDegraded},
		{"conditional", `{"type":"object","if":{"properties":{"x":{"const":1}}}}`, DialectCapabilityDegraded},
		{"tuple items", `{"type":"array","items":[{"type":"string"}]}`, DialectCapabilityDegraded},
		{"nested conditional", `{"type":"object","properties":{"nested":{"type":"object","if":{"properties":{"x":{"const":1}}}}}}`, DialectCapabilityDegraded},
		{"deep multi-type in properties", `{"type":"object","properties":{"v":{"type":["number","integer"]}}}`, DialectCapabilityDegraded},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifySchemaSupport(mustParse(t, tc.schema))
			assert.Equal(t, tc.expected, got)
		})
	}
}

// --- 确定性：同一 schema 多次归一，诊断顺序恒定（SUI-007 确定性延伸） ---

func TestNormalizeDialect_DiagnosticsDeterministic(t *testing.T) {
	text := `{"type":"object","properties":{"mode":{"type":["string","integer"]},"pair":{"type":"array","items":[{"type":"string"},{"type":"integer"}]},"choice":{"type":"object","if":{"properties":{"x":{"const":1}}},"then":{"required":["x"]}}}}`
	first := ""
	for i := 0; i < 30; i++ {
		in := mustParse(t, text)
		var notes []DialectNote
		NormalizeSchemaDialect(in, SchemaDialectOpenAPI31, &notes)
		diags := DialectNotesToDiagnostics(notes)
		encoded := jsonOf(t, diags)
		if first == "" {
			first = encoded
		}
		assert.Equal(t, first, encoded, "iteration %d must produce identical diagnostics", i)
	}
}

// --- DetectSchemaDialect ---

func TestDetectSchemaDialect(t *testing.T) {
	cases := []struct {
		doc      string
		expected SchemaDialect
	}{
		{`{"openapi":"3.0.3"}`, SchemaDialectOpenAPI30},
		{`{"openapi":"3.1.0"}`, SchemaDialectOpenAPI31},
		{`{"openapi":"3.2.0"}`, SchemaDialectUnknown},
		{`{"swagger":"2.0"}`, SchemaDialectUnknown},
		{`{}`, SchemaDialectUnknown},
	}
	for _, tc := range cases {
		got := DetectSchemaDialect(mustParse(t, tc.doc))
		assert.Equal(t, tc.expected, got)
	}
}
