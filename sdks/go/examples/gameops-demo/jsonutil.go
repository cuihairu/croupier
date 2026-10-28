package main

import (
	"encoding/json"
	"fmt"
)

// decodePayload converts the wire payload into a plain map. Handlers treat the
// input as opaquely as the browser executors: missing fields are zero values.
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

// encodeJSON serialises a handler result.
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

func strOr(v string, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func decodeInto(raw []byte, out *map[string]any) error {
	return json.Unmarshal(raw, out)
}
