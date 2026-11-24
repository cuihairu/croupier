package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SUI-010 验收：readOnly/writeOnly 字段方向。请求侧剔除 readOnly 字段、
// 响应侧剔除 writeOnly 字段，两侧 required 集同步收缩；嵌套字段、引用内联
// 字段与普通字段同规则。请求与响应 fixture 均经驱动层对拍验证
// （internal/drivers/openapi/schema_direction_test.go）。契约级过滤（参数 + body
// + 诊断）在新树由驱动层的 RequestContract 归一承担，测试同置该包。

// --- 普通字段（顶层 properties） ---

func TestDirectionFilter_RequestDropsReadOnlyTopLevel(t *testing.T) {
	in := mustParse(t, `{"type":"object","properties":{"id":{"type":"string","readOnly":true},"name":{"type":"string"}},"required":["id","name"]}`)
	var notes []DirectionNote
	out := FilterSchemaDirection(in, SchemaDirectionRequest, &notes)
	require.NotNil(t, out)
	props := out["properties"].(map[string]interface{})
	assert.NotContains(t, props, "id")
	assert.Contains(t, props, "name")
	// required 同步收缩：必填只读字段不得令请求不可发送。
	assert.Equal(t, `["name"]`, jsonOf(t, out["required"]))
	require.Len(t, notes, 1)
	assert.Equal(t, "/properties/id", notes[0].Path)
	assert.Equal(t, "readOnly", notes[0].Keyword)
	assert.Contains(t, notes[0].Message, "request")
}

func TestDirectionFilter_ResponseDropsWriteOnlyTopLevel(t *testing.T) {
	in := mustParse(t, `{"type":"object","properties":{"secret":{"type":"string","writeOnly":true},"id":{"type":"string"}},"required":["secret","id"]}`)
	var notes []DirectionNote
	out := FilterSchemaDirection(in, SchemaDirectionResponse, &notes)
	require.NotNil(t, out)
	props := out["properties"].(map[string]interface{})
	assert.NotContains(t, props, "secret")
	assert.Contains(t, props, "id")
	assert.Equal(t, `["id"]`, jsonOf(t, out["required"]))
	require.Len(t, notes, 1)
	assert.Equal(t, "/properties/secret", notes[0].Path)
	assert.Equal(t, "writeOnly", notes[0].Keyword)
	assert.Contains(t, notes[0].Message, "response")
}

// --- 嵌套字段 ---

func TestDirectionFilter_RequestDropsNestedReadOnly(t *testing.T) {
	in := mustParse(t, `{"type":"object","properties":{"user":{"type":"object","properties":{"id":{"type":"string","readOnly":true},"email":{"type":"string"}},"required":["id","email"]},"note":{"type":"string"}},"required":["user"]}`)
	var notes []DirectionNote
	out := FilterSchemaDirection(in, SchemaDirectionRequest, &notes)
	props := out["properties"].(map[string]interface{})
	user := props["user"].(map[string]interface{})
	userProps := user["properties"].(map[string]interface{})
	assert.NotContains(t, userProps, "id")
	assert.Contains(t, userProps, "email")
	assert.Equal(t, `["email"]`, jsonOf(t, user["required"]), "nested required set shrinks")
	// 外层 required 不受嵌套剔除影响。
	assert.Equal(t, `["user"]`, jsonOf(t, out["required"]))
	require.Len(t, notes, 1)
	assert.Equal(t, "/properties/user/properties/id", notes[0].Path)
}

// --- 引用内联后的字段（$ref 已被 SUI-008 展开，过滤器只见独立 schema） ---

func TestDirectionFilter_RequestDropsInlinedRefReadOnly(t *testing.T) {
	// kin/map 前端内联 $ref 后传入的形态：User.id(readOnly) 已展开进 body。
	in := mustParse(t, `{"type":"object","properties":{"user":{"type":"object","properties":{"id":{"type":"string","readOnly":true},"v":{"type":"integer"}},"required":["id","v"]}},"required":["user"]}`)
	var notes []DirectionNote
	out := FilterSchemaDirection(in, SchemaDirectionRequest, &notes)
	props := out["properties"].(map[string]interface{})
	user := props["user"].(map[string]interface{})
	userProps := user["properties"].(map[string]interface{})
	assert.NotContains(t, userProps, "id")
	assert.Contains(t, userProps, "v")
	require.Len(t, notes, 1)
	assert.Equal(t, "/properties/user/properties/id", notes[0].Path)
}

// --- 数组元素与 map 值 schema ---

func TestDirectionFilter_RequestDropsArrayItemReadOnly(t *testing.T) {
	in := mustParse(t, `{"type":"array","items":{"type":"object","properties":{"id":{"type":"string","readOnly":true},"v":{"type":"integer"}},"required":["id","v"]}}`)
	var notes []DirectionNote
	out := FilterSchemaDirection(in, SchemaDirectionRequest, &notes)
	items := out["items"].(map[string]interface{})
	itemsProps := items["properties"].(map[string]interface{})
	assert.NotContains(t, itemsProps, "id")
	assert.Equal(t, `["v"]`, jsonOf(t, items["required"]))
	require.Len(t, notes, 1)
	assert.Equal(t, "/items/properties/id", notes[0].Path)
}

func TestDirectionFilter_FlaggedItemsAndAdditionalPropertiesRemoved(t *testing.T) {
	// items / additionalProperties 自身被标记 → 该槽位整体剔除（元素/附加值
	// 属另一侧，本侧无数组/映射可发）。
	in := mustParse(t, `{"type":"object","properties":{"list":{"type":"array","items":{"type":"string","readOnly":true}},"m":{"type":"object","additionalProperties":{"type":"string","readOnly":true}}}}`)
	var notes []DirectionNote
	out := FilterSchemaDirection(in, SchemaDirectionRequest, &notes)
	props := out["properties"].(map[string]interface{})
	list := props["list"].(map[string]interface{})
	assert.NotContains(t, list, "items")
	m := props["m"].(map[string]interface{})
	assert.NotContains(t, m, "additionalProperties")
	require.Len(t, notes, 2)
}

// --- 组合器成员 ---

func TestDirectionFilter_RequestDropsAllOfMemberField(t *testing.T) {
	in := mustParse(t, `{"allOf":[{"type":"object","properties":{"id":{"type":"string","readOnly":true}}},{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}]}`)
	var notes []DirectionNote
	out := FilterSchemaDirection(in, SchemaDirectionRequest, &notes)
	allOf := out["allOf"].([]interface{})
	require.Len(t, allOf, 2)
	first := allOf[0].(map[string]interface{})
	firstProps := first["properties"].(map[string]interface{})
	assert.NotContains(t, firstProps, "id")
	second := allOf[1].(map[string]interface{})
	assert.Contains(t, second["properties"], "name")
	require.Len(t, notes, 1)
	assert.Equal(t, "/allOf/0/properties/id", notes[0].Path)
}

func TestDirectionFilter_FlaggedCombinatorMemberRemoved(t *testing.T) {
	// anyOf 成员整体被标记 → 成员从数组移除（其余成员保留原位）。
	in := mustParse(t, `{"anyOf":[{"type":"string"},{"type":"object","readOnly":true},{"type":"integer"}]}`)
	var notes []DirectionNote
	out := FilterSchemaDirection(in, SchemaDirectionRequest, &notes)
	anyOf := out["anyOf"].([]interface{})
	require.Len(t, anyOf, 2)
	assert.Equal(t, `{"type":"string"}`, jsonOf(t, anyOf[0]))
	assert.Equal(t, `{"type":"integer"}`, jsonOf(t, anyOf[1]))
	require.Len(t, notes, 1)
	assert.Equal(t, "/anyOf/1", notes[0].Path)
}

// --- 条件 schema 与元组：递归但永不移除成员（SUI-009 已判 degraded） ---

func TestDirectionFilter_ConditionalMembersRecursedNeverDropped(t *testing.T) {
	in := mustParse(t, `{"type":"object","properties":{"x":{"type":"string","readOnly":true}},"if":{"type":"object","readOnly":true,"properties":{"y":{"type":"string"}}},"then":{"properties":{"z":{"type":"string","readOnly":true}}}}`)
	var notes []DirectionNote
	out := FilterSchemaDirection(in, SchemaDirectionRequest, &notes)
	// if 分支自身被标记 → 保留（移除会改变条件语义），但内部字段仍被过滤。
	cond := out["if"].(map[string]interface{})
	assert.Contains(t, cond, "readOnly", "flagged conditional branch is kept")
	assert.Contains(t, cond, "properties")
	thenProps := out["then"].(map[string]interface{})["properties"].(map[string]interface{})
	assert.NotContains(t, thenProps, "z", "fields inside branches are still filtered")
	// 只有 then 内部的 z 与顶层 x 产生 note；if 分支本体不产生 note。
	paths := []string{}
	for _, note := range notes {
		paths = append(paths, note.Path)
	}
	assert.ElementsMatch(t, []string{"/properties/x", "/then/properties/z"}, paths)
}

func TestDirectionFilter_TupleMembersRecursedNeverDropped(t *testing.T) {
	// draft-07 元组（items 数组）：成员 schema 位置不变，内部字段仍过滤。
	in := mustParse(t, `{"type":"array","items":[{"type":"object","properties":{"id":{"type":"string","readOnly":true}}},{"type":"object","readOnly":true}]}`)
	var notes []DirectionNote
	out := FilterSchemaDirection(in, SchemaDirectionRequest, &notes)
	items := out["items"].([]interface{})
	require.Len(t, items, 2, "tuple members are never removed")
	firstProps := items[0].(map[string]interface{})["properties"].(map[string]interface{})
	assert.NotContains(t, firstProps, "id")
	second := items[1].(map[string]interface{})
	assert.Contains(t, second, "readOnly", "flagged tuple member is kept")
}

// --- 方向匹配的标记存活 ---

func TestDirectionFilter_MatchingFlagSurvives(t *testing.T) {
	// 请求侧保留 writeOnly 标记（响应侧保留 readOnly），方向信息仍可读。
	in := mustParse(t, `{"type":"object","properties":{"secret":{"type":"string","writeOnly":true},"id":{"type":"string","readOnly":true}}}`)
	var notes []DirectionNote
	out := FilterSchemaDirection(in, SchemaDirectionRequest, &notes)
	props := out["properties"].(map[string]interface{})
	assert.Equal(t, true, props["secret"].(map[string]interface{})["writeOnly"])
	assert.NotContains(t, props, "id")
	require.Len(t, notes, 1)
}

// --- 根级标记 → 整份 schema 属另一侧 ---

func TestDirectionFilter_FlaggedRootDroppedEntirely(t *testing.T) {
	in := mustParse(t, `{"type":"object","readOnly":true,"properties":{"id":{"type":"string"}}}`)
	var notes []DirectionNote
	out := FilterSchemaDirection(in, SchemaDirectionRequest, &notes)
	assert.Nil(t, out)
	require.Len(t, notes, 1)
	assert.Equal(t, "", notes[0].Path)
}

// --- $defs 不动（SUI-008 内联后是未用素材，非渲染 schema） ---

func TestDirectionFilter_DefsUntouched(t *testing.T) {
	in := mustParse(t, `{"type":"object","properties":{"id":{"type":"string"}},"$defs":{"Meta":{"type":"object","properties":{"id":{"type":"string","readOnly":true}}}}}`)
	var notes []DirectionNote
	out := FilterSchemaDirection(in, SchemaDirectionRequest, &notes)
	defs := out["$defs"].(map[string]interface{})
	metaProps := defs["Meta"].(map[string]interface{})["properties"].(map[string]interface{})
	assert.Contains(t, metaProps, "id", "$defs entries must not be filtered")
	assert.Empty(t, notes)
}

// --- required 收缩边界 ---

func TestDirectionFilter_EmptyRequiredRemoved(t *testing.T) {
	// required 收缩为空 → 键删除（空 required 是无效 schema）。
	in := mustParse(t, `{"type":"object","properties":{"id":{"type":"string","readOnly":true}},"required":["id"]}`)
	out := FilterSchemaDirection(in, SchemaDirectionRequest, nil)
	assert.NotContains(t, out, "required")
	assert.Contains(t, out, "properties")
}

func TestDirectionFilter_RequiredUntouchedWhenNothingDropped(t *testing.T) {
	// 无剔除 → required 原样（含既有同名缺失键也不动，字节稳定）。
	in := mustParse(t, `{"type":"object","properties":{"a":{"type":"string"}},"required":["a","ghost"]}`)
	var notes []DirectionNote
	out := FilterSchemaDirection(in, SchemaDirectionRequest, &notes)
	assert.Equal(t, `["a","ghost"]`, jsonOf(t, out["required"]))
	assert.Empty(t, notes)
}

// --- 无方向关键字字节稳定（SUI-004 保真延伸） ---

func TestDirectionFilter_NoFlagsByteStable(t *testing.T) {
	text := `{"type":"object","properties":{"id":{"type":"string"},"count":{"type":"integer","minimum":0}},"required":["id"]}`
	in := mustParse(t, text)
	out := FilterSchemaDirection(in, SchemaDirectionRequest, nil)
	assert.Equal(t, jsonOf(t, in), jsonOf(t, out))
}

func TestDirectionFilter_NilSchema(t *testing.T) {
	assert.Nil(t, FilterSchemaDirection(nil, SchemaDirectionRequest, nil))
}

// --- 输入不被突变 ---

func TestDirectionFilter_DoesNotMutateInput(t *testing.T) {
	in := mustParse(t, `{"type":"object","properties":{"user":{"type":"object","properties":{"id":{"type":"string","readOnly":true}},"required":["id"]}},"required":["user"]}`)
	before := jsonOf(t, in)
	FilterSchemaDirection(in, SchemaDirectionRequest, nil)
	assert.Equal(t, before, jsonOf(t, in), "input schema must not be mutated")
}

// --- 诊断映射与确定性 ---

func TestDirectionNoteDiagnosticCode_Mapping(t *testing.T) {
	assert.Equal(t, DiagSchemaReadonlyDropped, DirectionNoteDiagnosticCode("readOnly"))
	assert.Equal(t, DiagSchemaWriteonlyDropped, DirectionNoteDiagnosticCode("writeOnly"))
}

func TestDirectionNotesToDiagnostics_SortedAndShaped(t *testing.T) {
	notes := []DirectionNote{
		{Path: "/properties/z", Keyword: "readOnly", Message: "m2"},
		{Path: "/properties/a", Keyword: "writeOnly", Message: "m1"},
		{Path: "/properties/a", Keyword: "readOnly", Message: "m3"},
	}
	diags := DirectionNotesToDiagnostics(notes)
	require.Len(t, diags, 3)
	// 排序：(path, keyword) —— a/readOnly、a/writeOnly、z/readOnly。
	assert.Equal(t, DiagSchemaReadonlyDropped, diags[0].Code)
	assert.Equal(t, "/properties/a", diags[0].Field)
	assert.Equal(t, DiagSchemaWriteonlyDropped, diags[1].Code)
	assert.Equal(t, "/properties/a", diags[1].Field)
	assert.Equal(t, DiagSchemaReadonlyDropped, diags[2].Code)
	assert.Equal(t, "/properties/z", diags[2].Field)
	for _, diag := range diags {
		assert.Equal(t, SeverityWarning, diag.Severity)
	}
}

func TestDirectionFilter_NotesDeterministicAcrossIterations(t *testing.T) {
	// properties 遍历序不定，同一文档 30 次过滤的诊断序列必须恒同（SUI-007
	// 确定性延伸到方向诊断）。
	text := `{"type":"object","properties":{"b":{"type":"string","readOnly":true},"a":{"type":"object","properties":{"x":{"type":"string","readOnly":true},"y":{"type":"string","readOnly":true}},"required":["x","y"]},"c":{"type":"string","writeOnly":true}},"required":["a","b"]}`
	first := ""
	for i := 0; i < 30; i++ {
		var notes []DirectionNote
		FilterSchemaDirection(mustParse(t, text), SchemaDirectionRequest, &notes)
		diags := DirectionNotesToDiagnostics(notes)
		encoded := jsonOf(t, diags)
		if first == "" {
			first = encoded
		}
		assert.Equal(t, first, encoded, "iteration %d must produce identical diagnostics", i)
	}
}
