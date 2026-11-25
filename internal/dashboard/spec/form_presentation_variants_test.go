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
