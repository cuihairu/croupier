package dbtype

import (
	"encoding/json"
	"testing"
)

// UnmarshalJSON 直接透传给 json.RawMessage.UnmarshalJSON；在 Go 1.26 中该方法
// 对非 nil 接收器永不报错（仅 append 原始字节），因此 json.go:69-71 的错误
// 分支是防御性死代码，无法通过任何输入触发。以下测试固化该行为边界：
// 任意字节（含非法 JSON）都原样保留。
func TestJSONUnmarshalJSONPreservesArbitraryBytes(t *testing.T) {
	cases := []string{`{"a":1}`, `not json at all`, `123`, `["x"`}
	for _, raw := range cases {
		var j JSON
		if err := j.UnmarshalJSON([]byte(raw)); err != nil {
			t.Fatalf("UnmarshalJSON(%q) unexpected error: %v", raw, err)
		}
		if string(j) != raw {
			t.Fatalf("UnmarshalJSON(%q) = %q, want raw bytes preserved", raw, string(j))
		}
	}
}

func TestJSONUnmarshalViaEncodingJSONRoundTrip(t *testing.T) {
	var j JSON
	if err := json.Unmarshal([]byte(`{"ok":true}`), &j); err != nil {
		t.Fatalf("json.Unmarshal into JSON: %v", err)
	}
	if string(j) != `{"ok":true}` {
		t.Fatalf("j = %s, want raw document", j)
	}
	var out map[string]bool
	if err := json.Unmarshal(j, &out); err != nil {
		t.Fatalf("json.Unmarshal from JSON: %v", err)
	}
	if !out["ok"] {
		t.Fatal("round trip lost the document payload")
	}
}
