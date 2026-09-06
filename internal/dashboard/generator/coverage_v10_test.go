package generator

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// form_hints.go 未覆盖分支
// ---------------------------------------------------------------------------

func TestHintLocalizedNonStringNonObjectV10(t *testing.T) {
	assert.Nil(t, hintLocalized(json.RawMessage("123")))
	assert.Nil(t, hintLocalized(json.RawMessage("[1,2]")))
}

func TestHintLocalizedObjectWithoutLocaleV10(t *testing.T) {
	assert.Nil(t, hintLocalized(json.RawMessage(`{"fr":"bonjour"}`)))
}

func TestHintEnumOptionsMalformedPayloadV10(t *testing.T) {
	assert.Nil(t, hintEnumOptions(json.RawMessage(`{"a":1}`)))
}

func TestHintEnumOptionsItemShapesV10(t *testing.T) {
	// 非对象元素 / 缺 value / 缺 label 全部跳过，最终回落 nil。
	assert.Nil(t, hintEnumOptions(json.RawMessage(`[123]`)))
	assert.Nil(t, hintEnumOptions(json.RawMessage(`[{"label":{"zh-CN":"x"}}]`)))
	assert.Nil(t, hintEnumOptions(json.RawMessage(`[{"value":"a"}]`)))
	options := hintEnumOptions(json.RawMessage(`[{"value":"a","label":{"zh-CN":"选项"}}]`))
	require.Len(t, options, 1)
	assert.Equal(t, "a", options[0].Value)
}

func TestHintWidgetPropsShapesV10(t *testing.T) {
	assert.Nil(t, hintWidgetProps(json.RawMessage(`{}`)))
	assert.Nil(t, hintWidgetProps(json.RawMessage(`"oops"`)))
	props := hintWidgetProps(json.RawMessage(`{"precision":2}`))
	require.NotNil(t, props)
	assert.Len(t, props, 1)
}

func TestParseHintConditionEdgeCasesV10(t *testing.T) {
	assert.Nil(t, parseHintCondition(json.RawMessage(`"plain-string"`), 0))                // 非对象
	assert.Nil(t, parseHintCondition(json.RawMessage(`{"kind":"equals","path":"/a"}`), 0)) // 缺 value
	assert.Nil(t, parseHintCondition(json.RawMessage(`{"kind":"exists","path":"a"}`), 0))  // path 非指针
	assert.Nil(t, parseHintCondition(json.RawMessage(`{"kind":"all"}`), 0))                // 缺 conditions
	assert.Nil(t, parseHintCondition(json.RawMessage(`{"kind":"any","conditions":123}`), 0))
	assert.Nil(t, parseHintCondition(json.RawMessage(`{"kind":"all","conditions":[123]}`), 0)) // 子条件全非法
	assert.Nil(t, parseHintCondition(json.RawMessage(`{"kind":"weird"}`), 0))                  // 未知 kind
}

func TestHasEnumValuesNonArrayV10(t *testing.T) {
	assert.False(t, hasEnumValues(parseRawMapV10(t, `{"enum":123}`)))
}

func TestDefaultWidgetForSchemaTimeFormatV10(t *testing.T) {
	widget := defaultWidgetForSchema(parseRawMapV10(t, `{"type":"string","format":"time"}`))
	assert.Equal(t, spec.FormWidgetTimePicker, widget)
}

func TestRawBoolNonBoolPayloadV10(t *testing.T) {
	assert.Nil(t, rawBool(json.RawMessage("123")))
	assert.NotNil(t, rawBool(json.RawMessage("true")))
}

// ---------------------------------------------------------------------------
// generator.go 未覆盖分支
// ---------------------------------------------------------------------------

func TestBuildFormFieldsDescriptionAndWidgetPropsV10(t *testing.T) {
	schema := spec.JSONSchema(`{"type":"object","properties":{"f":{"type":"string","x-description":{"zh-CN":"备注说明"},"x-widget-props":{"precision":2}}}}`)
	fields := buildFormFields(schema, "zh-CN")
	require.Len(t, fields, 1)
	require.NotNil(t, fields[0].Description)
	assert.Equal(t, "备注说明", fields[0].Description["zh-CN"])
	require.NotNil(t, fields[0].WidgetProps)
	assert.Len(t, fields[0].WidgetProps, 1)
}

func TestOperationPageKeyUnsanitizableFunctionV10(t *testing.T) {
	// FunctionID 全为非法字符 → sanitize 后为空 → 回落 "unbound"。
	key := operationPageKey(spec.OperationSpec{FunctionID: "!!!"}, GenerateOptions{})
	assert.Contains(t, key, "unbound")
}

func TestBuildDimensionsDatetimeCoercedToDateV10(t *testing.T) {
	itemSchema := parseRawMapV10(t, `{"type":"object","properties":{"day":{"type":"string","format":"date-time"}}}`)
	dims := buildDimensionsFromPointers(itemSchema, []string{"/day"}, "zh-CN")
	require.Len(t, dims, 1)
	assert.Equal(t, "date", dims[0].DataType)
}

// ---------------------------------------------------------------------------
// resource_generator.go 未覆盖分支
// ---------------------------------------------------------------------------

func TestResourceBindingNoSelectorsV10(t *testing.T) {
	// 无输入输出 schema：selector 为空时提前返回、不带 Selectors。
	binding := resourceBinding(&model.FunctionContract{FunctionID: "demo.empty"}, "query", spec.BindingUsageQuery, nil)
	assert.Equal(t, "query", binding.ID)
	assert.Nil(t, binding.Selectors)
}

func TestBuildActionFormPresentationStripsTargetFieldV10(t *testing.T) {
	contract := &model.FunctionContract{
		FunctionID:  "demo.ban",
		InputSchema: model.JSON(`{"type":"object","properties":{"id":{"type":"string"},"reason":{"type":"string"}}}`),
	}
	semantics := &model.CapabilitySemantics{IdentityField: "id"}
	form := buildActionFormPresentation(contract, semantics, resourceActionSemantic{IdentityInput: "reason"})
	require.NotNil(t, form)
	for _, f := range form.Fields {
		assert.NotEqual(t, "reason", f.Key, "target field must be stripped from action form")
		assert.NotEqual(t, "id", f.Key, "identity field must be stripped from action form")
	}
}

func TestBuildListViewFromContractFilterableColumnV10(t *testing.T) {
	contract := &model.FunctionContract{
		FunctionID:   "demo.list",
		InputSchema:  model.JSON(`{"type":"object","properties":{"page":{"type":"integer"},"page_size":{"type":"integer"},"name":{"type":"string"}}}`),
		OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"}}}}}}`),
	}
	view := buildListViewFromContract(contract, &model.CapabilitySemantics{IdentityField: "id"})
	require.NotNil(t, view)
	byKey := map[string]spec.ColumnSpec{}
	for _, col := range view.Columns {
		byKey[col.Key] = col
	}
	assert.True(t, byKey["name"].Filterable, "filter-backed column must be filterable")
	assert.False(t, byKey["id"].Filterable, "non-filter column must not be filterable")
	assert.Equal(t, "left", byKey["id"].Fixed, "identity column must be fixed left")
}

func TestBuildInlineResourceActionSelectorReplaceFailsV10(t *testing.T) {
	semantics := &model.CapabilitySemantics{IdentityField: "id"}

	// 说明：schemaCanBind* 会 TrimSpace 目标字段，而 replaceSelectorSource 也
	// 对 IdentityInput 做同样的 TrimSpace，两侧归一后必然相等，因此
	// 573/584/606 的"替换失败"分支在 CanBind 通过的前提下不可达；这里用
	// 带空白的字段名触发 CanBind 失败路径（TrimSpace 后属性缺失）。
	_, placement, ok := buildInlineResourceActionSelector(&model.FunctionContract{
		InputSchema: model.JSON(`{"type":"object","properties":{" id ":{"type":"string"}},"required":[" id "]}`),
	}, semantics, resourceActionSemantic{Subject: "resource_item", IdentityInput: "/ id "})
	assert.False(t, ok)
	assert.Empty(t, placement)

	// identity + 附加字段分支：trim 后属性缺失。
	_, placement, ok = buildInlineResourceActionSelector(&model.FunctionContract{
		InputSchema: model.JSON(`{"type":"object","properties":{" id ":{"type":"string"},"reason":{"type":"string"}},"required":[" id ","reason"]}`),
	}, semantics, resourceActionSemantic{Subject: "resource_item", IdentityInput: "/ id "})
	assert.False(t, ok)
	assert.Empty(t, placement)

	// resource_selection 分支：trim 后属性缺失。
	_, placement, ok = buildInlineResourceActionSelector(&model.FunctionContract{
		InputSchema: model.JSON(`{"type":"object","properties":{" ids ":{"type":"array","items":{"type":"string"}}},"required":[" ids "]}`),
	}, semantics, resourceActionSemantic{Subject: "resource_selection", IdentityInput: "/ ids "})
	assert.False(t, ok)
	assert.Empty(t, placement)
}

func TestSchemaCanBindOnlyIdentityNoPropertiesV10(t *testing.T) {
	assert.False(t, schemaCanBindOnlyIdentity(spec.JSONSchema(`{"type":"object"}`), "id"))
}

func TestResolvePaginationFieldsPageSizeNotInSchemaV10(t *testing.T) {
	properties := parseRawMapV10(t, `{"page":{},"other":{}}`)
	semantics := &model.CapabilitySemantics{PageFieldName: "page", PageSizeFieldName: "size"}
	assert.False(t, resolvePaginationFields(properties, semantics))
}

// ---------------------------------------------------------------------------
// composite_generator.go 显式 InputAssignments 覆盖/追加
// ---------------------------------------------------------------------------

func TestGenerateCompositePageInputAssignmentOverridesV10(t *testing.T) {
	contract := &model.FunctionContract{
		FunctionID:  "demo.report",
		InputSchema: model.JSON(`{"type":"object","properties":{"date":{"type":"string"}},"required":["date"]}`),
	}
	page, ok := GenerateCompositePage("demo--combo", []CompositeSectionInput{
		{
			FunctionID: "demo.report",
			View:       "fields",
			InputAssignments: []spec.InputAssignment{
				{Target: "/date", Source: spec.ValueSource{Kind: spec.SourceForm, Path: "/date"}},   // 覆盖自动映射
				{Target: "/extra", Source: spec.ValueSource{Kind: spec.SourceForm, Path: "/extra"}}, // 追加
			},
		},
	}, []*model.FunctionContract{contract}, GenerateOptions{})
	require.True(t, ok)

	var binding *spec.PageFunctionBinding
	for i := range page.Bindings {
		if page.Bindings[i].ID == "demo.report" {
			binding = &page.Bindings[i]
		}
	}
	require.NotNil(t, binding, "binding for demo.report must exist")
	require.NotNil(t, binding.Selectors)
	assignments := binding.Selectors.Input.Assignments
	require.Len(t, assignments, 2)
	assert.Equal(t, "/date", assignments[0].Target)
	assert.Equal(t, spec.SourceForm, assignments[0].Source.Kind, "explicit assignment must replace page_state default")
	assert.Equal(t, "/extra", assignments[1].Target, "unknown target must be appended")
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func parseRawMapV10(t *testing.T, raw string) map[string]json.RawMessage {
	t.Helper()
	obj := parseJSONObject(json.RawMessage(raw))
	require.NotNil(t, obj)
	return obj
}
