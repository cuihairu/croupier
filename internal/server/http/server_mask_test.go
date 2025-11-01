package httpserver

import (
    "testing"
    desc "github.com/cuihairu/croupier/internal/function/descriptor"
)

func TestMaskSnapshot(t *testing.T) {
    s := &Server{descIndex: map[string]*desc.Descriptor{}}
    s.descIndex["f"] = &desc.Descriptor{UI: map[string]any{"sensitive": []any{"password", "token"}}}
    payload := map[string]any{"username": "alice", "password": "secret", "nested": map[string]any{"token": "abc"}}
    out := s.maskSnapshot("f", payload)
    // Expect password and nested.token masked
    if !contains(out, "\"password\":\"***\"") || !contains(out, "\"token\":\"***\"") {
        t.Fatalf("masking failed: %s", out)
    }
    if !contains(out, "\"username\":\"alice\"") {
        t.Fatalf("non-sensitive should remain: %s", out)
    }
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (func() bool { return (stringIndex(s, sub) >= 0) })() }

// simple index to avoid importing strings for tiny test
func stringIndex(s, sub string) int {
    n, m := len(s), len(sub)
    if m == 0 { return 0 }
    for i := 0; i+m <= n; i++ {
        if s[i:i+m] == sub { return i }
    }
    return -1
}

