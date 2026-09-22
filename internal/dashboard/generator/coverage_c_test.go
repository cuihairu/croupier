package generator

import "testing"

// localizedTitleFallback：humanize 无从下手（trim 后为空的 key）时 label
// 回落为原始 key 本身——任意输入都必须产出非空 LocalizedText 结构，
// 第三 locale 照常追加同值。
func TestCoverageC_LocalizedTitleFallback_EmptyLabelFallsBackToKey(t *testing.T) {
	// " .-_" trim 后为空：HumanizeKey 返回 ""，label = key
	key := " .-_"
	text := localizedTitleFallback(key, "ja-JP")
	if text["zh-CN"] != key || text["en-US"] != key {
		t.Fatalf("空标签应回落原始 key: %+v", text)
	}
	if text["ja-JP"] != key {
		t.Fatalf("第三 locale 应追加回落值: %+v", text)
	}
	if len(text) != 3 {
		t.Fatalf("应有三个 locale key: %+v", text)
	}
}
