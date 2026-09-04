package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// form_hints.go：x-ui-* hints 派生 + 类型缺省控件（与前端 schemaHints.ts 对齐）

func TestBuildFormFieldsHints(t *testing.T) {
	fields := buildFormFields(spec.JSONSchema(`{
		"type":"object",
		"properties":{
			"playerId":{"type":"string","x-widget":"Select","x-label":"玩家ID",
				"x-placeholder":"选择玩家","x-width":6,"x-order":2},
			"reason":{"type":"string","maxLength":200,"x-order":1,
				"x-visible-when":{"kind":"equals","path":"/channel","value":"mail"}},
			"channel":{"type":"string","x-widget":"Select","x-order":0,
				"x-enum-options":[{"value":"mail","label":"邮件"},{"value":"sms","label":{"en":"SMS"}}]},
			"deadline":{"type":"string","format":"date-time"},
			"tags":{"type":"array","items":{"type":"string","enum":["a","b"]}},
			"locked":{"type":"boolean","x-disabled":true},
			"payload":{"type":"string","x-widget":"NoSuchWidget"}
		}
	}`), "zh-CN")

	byKey := map[string]spec.FormFieldSpec{}
	for _, f := range fields {
		byKey[f.Key] = f
	}

	channel := byKey["channel"]
	// 无 x-label → humanize 兜底；选项标签在 EnumOptions
	require.NotNil(t, channel.Label)
	assert.Equal(t, "Channel", channel.Label["zh-CN"])
	assert.Equal(t, spec.FormWidgetSelect, channel.Widget)
	require.Len(t, channel.EnumOptions, 2)
	assert.Equal(t, "邮件", channel.EnumOptions[0].Label["zh-CN"])
	assert.Equal(t, "SMS", channel.EnumOptions[1].Label["en-US"])

	reason := byKey["reason"]
	require.NotNil(t, reason.VisibleWhen)
	assert.Equal(t, "equals", reason.VisibleWhen.Kind)
	assert.Equal(t, "/channel", reason.VisibleWhen.Path)
	assert.Equal(t, spec.FormWidgetTextArea, reason.Widget)

	locked := byKey["locked"]
	require.NotNil(t, locked.Disabled)
	assert.True(t, *locked.Disabled)
	assert.Equal(t, spec.FormWidgetSwitch, locked.Widget)

	deadline := byKey["deadline"]
	assert.Equal(t, spec.FormWidgetDatePicker, deadline.Widget)

	tags := byKey["tags"]
	assert.Equal(t, spec.FormWidgetMultiSelect, tags.Widget)

	playerId := byKey["playerId"]
	assert.Equal(t, spec.FormWidgetSelect, playerId.Widget)
	assert.Equal(t, "玩家ID", playerId.Label["zh-CN"])
	assert.Equal(t, "选择玩家", playerId.Placeholder["zh-CN"])
	assert.Equal(t, 6, playerId.Width)
	assert.Equal(t, 2, playerId.Order)

	// 非法 x-widget 静默忽略：payload 是 string → 退回 Input 缺省（无 widget）
	payload := byKey["payload"]
	assert.Empty(t, payload.Widget)
}

func TestBuildFormFieldsHintOrder(t *testing.T) {
	fields := buildFormFields(spec.JSONSchema(`{
		"type":"object",
		"properties":{
			"alpha":{"type":"string"},
			"beta":{"type":"string","x-order":1},
			"gamma":{"type":"string","x-order":5}
		}
	}`), "zh-CN")
	require.Len(t, fields, 3)
	assert.Equal(t, "beta", fields[0].Key, "x-order=1 最先")
	assert.Equal(t, "gamma", fields[1].Key, "x-order=5 其次")
	assert.Equal(t, "alpha", fields[2].Key, "未声明 x-order 排最后")
}

func TestHintLocalizedShortKeys(t *testing.T) {
	require.Nil(t, hintLocalized(nil))
	// 遗留短 key 归一为 canonical BCP47
	got := hintLocalized([]byte(`{"zh":"label","en":"label-en"}`))
	require.NotNil(t, got)
	assert.Equal(t, "label", got["zh-CN"])
	assert.Equal(t, "label-en", got["en-US"])
	// 空串
	require.Nil(t, hintLocalized([]byte(`""`)))
}

func TestHintConditionDepthAndValidation(t *testing.T) {
	// path 不以 / 开头 → 忽略
	assert.Nil(t, hintCondition([]byte(`{"kind":"equals","path":"foo","value":1}`)))
	// all 嵌套
	got := hintCondition([]byte(`{"kind":"all","conditions":[
		{"kind":"exists","path":"/a"},
		{"kind":"notEquals","path":"/b","value":"x"}
	]}`))
	require.NotNil(t, got)
	assert.Equal(t, "all", got.Kind)
	require.Len(t, got.Conditions, 2)
}
