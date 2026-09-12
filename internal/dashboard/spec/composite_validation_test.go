package spec

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 覆盖 validatePublishableCompositePage（0%）+ LocalizedText JSON round-trip（0%）。

func TestValidatePublishableCompositePage(t *testing.T) {
	// nil → composite_empty
	diags := validatePublishableCompositePage(nil)
	require.Len(t, diags, 1)
	assert.Equal(t, "composite_empty", diags[0].Code)

	// 空 sections → composite_empty
	diags = validatePublishableCompositePage(&CompositePageSpec{})
	assert.Len(t, diags, 1)
	assert.Equal(t, "composite_empty", diags[0].Code)

	// 合法 section → 无诊断
	valid := &CompositePageSpec{
		Sections: []CompositeSection{{
			Key:       "t1",
			BindingID: "b1",
			View:      "table",
			Span:      12,
			Table:     &CompositeTableSpec{Columns: []ColumnSpec{{Key: "id", DataType: "string"}}},
		}},
	}
	diags = validatePublishableCompositePage(valid)
	assert.Empty(t, diags)

	// 多种违规：key 空 + binding 空 + view 非法 + span 越界 + table 缺失
	bad := &CompositePageSpec{
		Sections: []CompositeSection{{
			Key:       "",
			BindingID: "",
			View:      "bogus",
			Span:      99,
		}},
	}
	diags = validatePublishableCompositePage(bad)
	assert.Len(t, diags, 4)
	codes := map[string]bool{}
	for _, d := range diags {
		codes[d.Code] = true
	}
	assert.True(t, codes["composite_section_key_missing"])
	assert.True(t, codes["composite_section_binding_missing"])
	assert.True(t, codes["composite_section_view_invalid"])
	assert.True(t, codes["composite_section_span_invalid"])
	assert.False(t, codes["composite_section_table_missing"], "bogus view 不触发 table_missing")

	// table view 但 Table nil → table_missing
	badTable := &CompositePageSpec{
		Sections: []CompositeSection{{Key: "t", BindingID: "b", View: "table"}},
	}
	diags2 := validatePublishableCompositePage(badTable)
	codes2 := map[string]bool{}
	for _, d := range diags2 {
		codes2[d.Code] = true
	}
	assert.True(t, codes2["composite_section_table_missing"])
}

// U10 区块级条件显示：validateSectionCondition 的叶子/嵌套/key 引用校验。
func TestValidatePublishableCompositePageSectionCondition(t *testing.T) {
	// 两个区块：条件引用同页另一区块 key → 合法
	valid := &CompositePageSpec{
		Sections: []CompositeSection{
			{Key: "filter", BindingID: "bf", View: "form"},
			{
				Key:       "vipTable",
				BindingID: "bt",
				View:      "table",
				Table:     &CompositeTableSpec{Columns: []ColumnSpec{{Key: "id", DataType: "string"}}},
				VisibleWhen: &ConditionSpec{
					Kind:  "equals",
					Key:   "filter",
					Path:  "/values/mode",
					Value: json.RawMessage(`"advanced"`),
				},
			},
		},
	}
	assert.Empty(t, validatePublishableCompositePage(valid))

	// exists 不需要 value；嵌套 all/any 递归校验
	nested := &CompositePageSpec{
		Sections: []CompositeSection{
			{Key: "filter", BindingID: "bf", View: "form"},
			{Key: "t2", BindingID: "bt", View: "form", VisibleWhen: &ConditionSpec{
				Kind: "all",
				Conditions: []ConditionSpec{
					{Kind: "exists", Key: "filter", Path: "/values/mode"},
					{Kind: "any", Conditions: []ConditionSpec{
						{Kind: "notEquals", Key: "filter", Path: "/values/env", Value: json.RawMessage(`"prod"`)},
					}},
				},
			}},
		},
	}
	assert.Empty(t, validatePublishableCompositePage(nested))

	// 违规矩阵：kind 非法 / equals 缺 value / path 非 Pointer / key 缺失 / key 不在页面
	bad := &CompositePageSpec{
		Sections: []CompositeSection{
			{Key: "filter", BindingID: "bf", View: "form"},
			{Key: "s1", BindingID: "b1", View: "form", VisibleWhen: &ConditionSpec{Kind: "bogus"}},
			{Key: "s2", BindingID: "b2", View: "form", VisibleWhen: &ConditionSpec{Kind: "equals", Key: "filter", Path: "/values/mode"}},
			{Key: "s3", BindingID: "b3", View: "form", VisibleWhen: &ConditionSpec{Kind: "exists", Key: "filter", Path: "values/mode"}},
			{Key: "s4", BindingID: "b4", View: "form", VisibleWhen: &ConditionSpec{Kind: "exists", Path: "/values/mode"}},
			{Key: "s5", BindingID: "b5", View: "form", VisibleWhen: &ConditionSpec{Kind: "exists", Key: "ghost", Path: "/values/mode"}},
		},
	}
	diags := validatePublishableCompositePage(bad)
	codes := map[string]bool{}
	for _, d := range diags {
		codes[d.Code] = true
	}
	assert.True(t, codes["section_condition_kind_invalid"])
	assert.True(t, codes["section_condition_value_missing"])
	assert.True(t, codes["section_condition_path_invalid"])
	assert.True(t, codes["section_condition_key_missing"])
	assert.True(t, codes["section_condition_key_invalid"])

	// all 空子条件 → children_missing
	emptyAll := &CompositePageSpec{
		Sections: []CompositeSection{{Key: "s", BindingID: "b", View: "form", VisibleWhen: &ConditionSpec{Kind: "all"}}},
	}
	diags2 := validatePublishableCompositePage(emptyAll)
	require.NotEmpty(t, diags2)
	assert.Equal(t, "section_condition_children_missing", diags2[0].Code)
}

func TestLocalizedTextJSONRoundTrip(t *testing.T) {
	lt := LocalizedText{"zh-CN": "你好", "en-US": "hello"}

	data, err := json.Marshal(lt)
	require.NoError(t, err)

	var back LocalizedText
	require.NoError(t, json.Unmarshal(data, &back))
	assert.Equal(t, lt, back)

	// 从 map[string]string 转换
	m := map[string]string{"zh-CN": "值"}
	lt2 := LocalizedText(m)
	assert.Equal(t, "值", lt2["zh-CN"])
}
