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
