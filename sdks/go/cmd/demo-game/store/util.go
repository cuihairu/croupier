package store

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"sync/atomic"
	"time"
)

// ---------------------------------------------------------------------------
// Shared helpers (ported from examples/gameops-demo util files so the game
// logic keeps one copy; the main package stays contracts + wiring only).
// ---------------------------------------------------------------------------

func intStr(v int) string { return strconv.Itoa(v) }

// atoi parses an integer id, falling back to 0 on empty/invalid input.
func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func padLeft(s string, n int, pad byte) string {
	for len(s) < n {
		s = string(pad) + s
	}
	return s
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func ternaryInt(cond bool, a, b int) int {
	if cond {
		return a
	}
	return b
}

func strPtr(v string) *string { return &v }

func int64Ptr(v int64) *int64 { return &v }

func nowMillis() int64 { return time.Now().UnixMilli() }

func millisISO(ms int64) string {
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

func timeParseRFC3339(v string) (int64, error) {
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return 0, err
	}
	return t.UnixMilli(), nil
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

// uniqueStrings removes duplicate target IDs while preserving first order.
func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, item := range in {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func optionalString(ok bool, value, fallback string) *string {
	if !ok {
		if fallback == "" {
			return nil
		}
		return &fallback
	}
	return &value
}

func multiplyPtr(p *int64, n int) *int64 {
	if p == nil {
		return nil
	}
	v := *p * int64(n)
	return &v
}

func countLabel(n int) string {
	if n == 1 {
		return "1"
	}
	return "many"
}

// ---------------------------------------------------------------------------
// Payload decoding (ported jsonutil.go). Handlers treat the wire payload as
// opaquely as the browser executors: missing fields are zero values.
// ---------------------------------------------------------------------------

func decodePayload(payload []byte) (map[string]any, error) {
	var out map[string]any
	if len(payload) == 0 {
		return map[string]any{}, nil
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, fmt.Errorf("payload must be a JSON object: %w", err)
	}
	return out, nil
}

func encodeJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return raw, nil
}

func valueString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func valueBool(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	if v, ok := m[key]; ok && v != nil {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func valueInt(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	if v, ok := m[key]; ok && v != nil {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		case json.Number:
			if i, err := n.Int64(); err == nil {
				return int(i)
			}
		}
	}
	return 0
}

func valueInt64(m map[string]any, key string) int64 {
	if m == nil {
		return 0
	}
	if v, ok := m[key]; ok && v != nil {
		switch n := v.(type) {
		case float64:
			return int64(n)
		case int:
			return int64(n)
		case int64:
			return n
		case json.Number:
			if i, err := n.Int64(); err == nil {
				return i
			}
		}
	}
	return 0
}

func toAnySlice(v any) []any {
	if v == nil {
		return nil
	}
	if out, ok := v.([]any); ok {
		return out
	}
	return nil
}

func stringSlice(in []any) []string {
	out := make([]string, 0, len(in))
	for _, item := range in {
		if item == nil {
			continue
		}
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func valueStringSlice(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	switch items := v.(type) {
	case []any:
		return stringSlice(items)
	case []string:
		out := make([]string, len(items))
		copy(out, items)
		return out
	}
	return nil
}

func strOr(v string, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// ---------------------------------------------------------------------------
// Sort semantics (ported helpers.go + reflect.go): orderBy names a record
// field, orderDirection asc/desc. Unknown keys leave the seed order
// untouched (deterministic).
// ---------------------------------------------------------------------------

func sortSliceBy[T any](m map[string]any, items []T) {
	key := valueString(m, "orderBy")
	if key == "" || len(items) < 2 {
		return
	}
	desc := valueString(m, "orderDirection") == "desc"
	sort.SliceStable(items, func(i, j int) bool {
		vi := recordFieldString(items[i], key)
		vj := recordFieldString(items[j], key)
		if vi == vj {
			return false
		}
		if desc {
			return vi > vj
		}
		return vi < vj
	})
}

// recordFieldString extracts a field (by JSON tag or Go field name) as a
// comparable string. Numbers use zero-padded integer text so numeric ordering
// is correct; dates/text compare lexically (ISO-8601 and lexicographic order
// coincide).
func recordFieldString(v any, key string) string {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return ""
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return ""
	}
	for i := 0; i < rv.NumField(); i++ {
		f := rv.Type().Field(i)
		if !f.IsExported() {
			continue
		}
		if tag := f.Tag.Get("json"); tag != "" {
			if cutJSONTag(tag) == key {
				return formatFieldValue(rv.Field(i))
			}
		}
		if f.Name == key {
			return formatFieldValue(rv.Field(i))
		}
	}
	return ""
}

func formatFieldValue(v reflect.Value) string {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%020d", v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%020d", v.Uint())
	case reflect.String:
		return v.String()
	case reflect.Pointer:
		if v.IsNil() {
			return ""
		}
		return formatFieldValue(v.Elem())
	default:
		return ""
	}
}

func cutJSONTag(tag string) string {
	for i, r := range tag {
		if r == ',' {
			return tag[:i]
		}
	}
	return tag
}

// ---------------------------------------------------------------------------
// Auto operation IDs for writes that carry no client operationId. Process
// nonce + atomic counter: unique within the process, meaningless across
// restarts (replays are keyed to client operationIds only).
// ---------------------------------------------------------------------------

var (
	autoOpCounter uint64
	processNonce  = strconv.FormatInt(time.Now().UnixNano(), 36)
)

func autoOperationID() string {
	return "auto-" + processNonce + "-" + strconv.FormatUint(atomic.AddUint64(&autoOpCounter, 1), 10)
}
