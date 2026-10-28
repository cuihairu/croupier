package main

import "sort"

// Collection of small domain helpers shared by executors.

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
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

func ternaryInt(cond bool, a, b int) int {
	if cond {
		return a
	}
	return b
}

func mustPayloadMap(payload []byte) map[string]any {
	m, err := decodePayload(payload)
	if err != nil {
		return map[string]any{}
	}
	return m
}

func countLabel(n int) string {
	if n == 1 {
		return "1"
	}
	return "many"
}

// sortSliceBy implements the E2 list sort semantics: orderBy names a record
// field, orderDirection asc/desc. Unknown keys leave the seed order untouched
// (deterministic). It mutates items in place.
func sortSliceBy[T any](m map[string]any, items []T) {
	key := valueString(m, "orderBy")
	if key == "" || len(items) == 0 {
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
