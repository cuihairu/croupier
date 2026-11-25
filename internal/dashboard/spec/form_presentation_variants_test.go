package spec

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SUI-032：命名展示变体（SUI-031 协议）随 FormPresentationSpec 的 wire
// 往返与旧版数据兼容。草稿/发布快照的存储层走同一条 typed 序列化链
//（SaveDraft 请求 → spec.PageSpec → SpecJSON blob；发布 pageSpecFromModel
// → pageJSONMarshal → published_page_specs.SpecJSON），本文件的
// marshal/unmarshal 往返即存储往返的契约核心；服务级端到端
//（SaveDraft → GetDraft → Publish → 快照回读 → 重启重查）见
// internal/api/page/form_variants_storage_test.go。

func testFormVariants() []FormVariantSpec {
	return []FormVariantSpec{
		{
			Key:   "standard",
			Title: LocalizedText{"zh-CN": "标准", "en-US": "Standard"},
			Fields: []FormVariantFieldSpec{
				{Key: "keyword", Width: 12},
			},
		},
		{
			Key:    "compact",
			Title:  LocalizedText{"zh-CN": "紧凑", "en-US": "Compact"},
			Layout: FormLayoutGrid,
			Fields: []FormVariantFieldSpec{
				{
					Key:         "keyword",
					Widget:      FormWidgetInput,
					Width:       6,
					Order:       1,
					Label:       LocalizedText{"zh-CN": "关键词"},
					Description: LocalizedText{"zh-CN": "按昵称或 ID 搜索"},
					Placeholder: LocalizedText{"zh-CN": "输入关键词"},
					Role:        "identity",
					Unit:        LocalizedText{"zh-CN": "个"},
					PerTarget:   true,
					EnumOptions: []EnumOption{
						{Value: "a", Label: LocalizedText{"zh-CN": "甲"}, Color: "blue"},
					},
					RemoteOptions: &RemoteOptionsSpec{FunctionID: "player.list", LabelPath: "/items/*/name"},
					WidgetProps:   map[string]json.RawMessage{"allowClear": json.RawMessage(`true`)},
				},
				{Key: "/address/city", Width: 6},
			},
			Groups:       []FormGroupSpec{{Key: "basic", Fields: []string{"keyword"}}},
			SubmitButton: &FormButtonSpec{Text: LocalizedText{"zh-CN": "执行"}, Type: "primary"},
			CancelButton: &FormButtonSpec{Text: LocalizedText{"zh-CN": "取消"}},
		},
	}
}

func TestFormPresentationVariantsRoundtrip(t *testing.T) {
	in := &FormPresentationSpec{
		JSONSchema:     JSONSchema(`{"type":"object","properties":{"keyword":{"type":"string"}}}`),
		Layout:         FormLayoutVertical,
		Variants:       testFormVariants(),
		DefaultVariant: "standard",
	}

	raw, err := json.Marshal(in)
	require.NoError(t, err)

	var out FormPresentationSpec
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, in, &out, "typed roundtrip must preserve variants and defaultVariant verbatim")
}

func TestFormPresentationVariantsLegacyShapeUnchanged(t *testing.T) {
	// 旧版本快照/草稿（SUI-031 之前）：不带 variants/defaultVariant。
	// 反序列化保持零值，再序列化不得引入新键（旧版本回读逐字稳定）。
	legacy := `{"jsonSchema":{"type":"object"},"layout":"vertical","fields":[{"key":"a","width":6}]}`

	var out FormPresentationSpec
	require.NoError(t, json.Unmarshal([]byte(legacy), &out))
	assert.Empty(t, out.Variants)
	assert.Empty(t, out.DefaultVariant)

	raw, err := json.Marshal(&out)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), `"variants"`)
	assert.NotContains(t, string(raw), `"defaultVariant"`)
}

func TestFormVariantFieldSpecUnknownKeyAudit(t *testing.T) {
	// 调用相关键（required/visible）出现在变体字段 wire JSON 上：
	// 类型面没有这些字段，encoding/json 默认会静默丢弃——UnmarshalJSON
	// 必须把它们记入 UnknownKeys 供 form_variant_illegal_override 定位。
	wire := `{"key":"reason","widget":"TextArea","required":true,"visible":false,"validationRules":[{"type":"max"}]}`

	var out FormVariantFieldSpec
	require.NoError(t, json.Unmarshal([]byte(wire), &out))
	assert.Equal(t, "reason", out.Key)
	assert.Equal(t, FormWidgetTextArea, out.Widget)
	assert.Equal(t, []string{"required", "validationRules", "visible"}, out.UnknownKeys,
		"call-affecting keys must be audited in deterministic (sorted) order")

	// 白名单键不进 UnknownKeys
	clean := FormVariantFieldSpec{Key: "reason", Widget: FormWidgetTextArea, Width: 6}
	assert.Empty(t, clean.UnknownKeys)
	var parsed FormVariantFieldSpec
	require.NoError(t, json.Unmarshal([]byte(`{"key":"reason","widget":"TextArea","width":6}`), &parsed))
	assert.Empty(t, parsed.UnknownKeys)
}

func TestFormVariantFieldSpecUnknownKeysNotSerialized(t *testing.T) {
	field := FormVariantFieldSpec{
		Key:         "reason",
		Widget:      FormWidgetTextArea,
		UnknownKeys: []string{"required", "visible"},
	}
	raw, err := json.Marshal(&field)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "required")
	assert.NotContains(t, string(raw), "visible")
	assert.Contains(t, string(raw), `"key":"reason"`)

	// 含未知键的完整表单经类型编解码后诊断面可用、序列化面稳定
	form := &FormPresentationSpec{
		JSONSchema: JSONSchema(`{"type":"object","properties":{"reason":{"type":"string"}}}`),
		Variants: []FormVariantSpec{
			{Key: "compact", Fields: []FormVariantFieldSpec{
				{Key: "reason", Widget: FormWidgetTextArea, UnknownKeys: []string{"required"}},
			}},
		},
	}
	assert.Contains(t, variantCodes(t, form), "form_variant_illegal_override")
}

func TestFormVariantFieldSpecUnknownKeysViaFormDecode(t *testing.T) {
	// 从完整 FormPresentationSpec wire JSON 解码（服务层真实路径），
	// 嵌套变体字段的未知键同样被审计。
	wire := `{
		"jsonSchema": {"type":"object","properties":{"reason":{"type":"string"}}},
		"variants": [
			{"key": "compact", "fields": [{"key": "reason", "widget": "textArea", "defaultValue": "x"}]}
		]
	}`
	var out FormPresentationSpec
	require.NoError(t, json.Unmarshal([]byte(wire), &out))
	require.Len(t, out.Variants, 1)
	require.Len(t, out.Variants[0].Fields, 1)
	assert.Equal(t, []string{"defaultValue"}, out.Variants[0].Fields[0].UnknownKeys)
}
