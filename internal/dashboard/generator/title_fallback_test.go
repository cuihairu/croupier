package generator

import (
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// localizedTitleFallback：humanize 后双写 zh-CN/en-US；请求第三 locale 时
// 追加该 key；空/默认 locale 不追加。
func TestLocalizedTitleFallback(t *testing.T) {
	// 返回值即 LocalizedText 契约
	var text spec.LocalizedText = localizedTitleFallback("player_query", "")
	if text["zh-CN"] == "" || text["zh-CN"] != text["en-US"] {
		t.Fatalf("zh-CN/en-US 应双写同一 humanize 标签: %+v", text)
	}
	if len(text) != 2 {
		t.Fatalf("空 locale 只应有 zh-CN/en-US: %+v", text)
	}

	// 默认 locale 不追加第三 key
	en := localizedTitleFallback("player_query", "en-US")
	if len(en) != 2 || en["en-US"] == "" {
		t.Fatalf("en-US 请求不应追加第三 key: %+v", en)
	}

	// 第三 locale 追加同标签
	ja := localizedTitleFallback("player.query", "ja-JP")
	if ja["ja-JP"] == "" || ja["ja-JP"] != ja["zh-CN"] {
		t.Fatalf("ja-JP 应追加同标签: %+v", ja)
	}
	if len(ja) != 3 {
		t.Fatalf("ja-JP 请求应有三个 key: %+v", ja)
	}

	// humanize 无从下手时回落原 key：任意输入都产出非空标签
	raw := localizedTitleFallback("xyz", "zh-CN")
	if raw["zh-CN"] == "" {
		t.Fatalf("任意 key 都应产出非空标签: %+v", raw)
	}
}
