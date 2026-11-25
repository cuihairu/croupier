package dbtype

import (
	"encoding/json"
	"testing"
)

// UnmarshalJSON passes straight through to json.RawMessage.UnmarshalJSON; in
// Go 1.26 that method never errors on a non-nil receiver (it only appends the
// raw bytes), so the error branch at json.go:69-71 is defensive dead code that
// no input can trigger. The following test pins this behavioral boundary:
// arbitrary bytes (including invalid JSON) are preserved as-is.
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
