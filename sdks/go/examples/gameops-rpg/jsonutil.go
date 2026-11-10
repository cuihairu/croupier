package main

import (
	"encoding/json"
	"fmt"
)

// decodePayload converts the wire payload into a plain map. Handlers treat the
// input as opaque JSON: missing fields are zero values.
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
	return int(valueInt64(m, key))
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

func valueStringSlice(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	if items, ok := v.([]any); ok {
		return stringSlice(items)
	}
	return nil
}

// valueAttachmentSlice decodes [{itemId,count}] attachment payloads.
func valueAttachmentSlice(m map[string]any, key string) []rpgMailAttachment {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]rpgMailAttachment, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := valueString(obj, "itemId")
		count := valueInt64(obj, "count")
		if id == "" || count <= 0 {
			continue
		}
		out = append(out, rpgMailAttachment{ItemID: id, Count: count})
	}
	return out
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
