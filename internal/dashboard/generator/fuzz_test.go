package generator

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// FuzzRequiredProperties is the nightly fuzz entry point (CR-FIX-25): it feeds
// arbitrary bytes to requiredProperties and asserts that invalid JSON always
// yields an empty result, that valid input never panics, and that every
// returned key comes from the input bytes (panic/out-of-bounds defects are
// exposed directly by the fuzzer).
func FuzzRequiredProperties(f *testing.F) {
	f.Add([]byte(`{"required":["a","b"]}`))
	f.Add([]byte(`{"required":["uid",true,"name"]}`))
	f.Add([]byte(`{"required":"nope"}`))
	f.Add([]byte(`{"required":null}`))
	f.Add([]byte(`not json`))
	f.Add([]byte(`{}`))

	f.Fuzz(func(t *testing.T, schema []byte) {
		out := requiredProperties(spec.JSONSchema(schema))
		if !json.Valid(schema) {
			if len(out) != 0 {
				t.Fatalf("invalid JSON schema must yield empty result, got %v", out)
			}
			return
		}
		for k := range out {
			// encoding/json's standard coercion for invalid UTF-8 bytes: they surface
			// as U+FFFD in the decoded value with no matching sequence in the
			// raw bytes, so keys containing the replacement rune are exempt from
			// the presence check (testdata counterexample 189a00228b478025
			// pins this behavior).
			if strings.ContainsRune(k, utf8.RuneError) {
				continue
			}
			if !bytes.Contains(schema, []byte(k)) {
				t.Fatalf("returned key %q does not occur in schema bytes", k)
			}
		}
	})
}
