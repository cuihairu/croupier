package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// ValidateJSON validates a JSON payload `data` against a minimal subset of JSON Schema contained in `schema`.
// Supported subset:
// - type: object
// - properties: string|number|integer|boolean|object (object validated shallowly)
// - required: [..]
// Returns first error encountered for simplicity.
func ValidateJSON(schema map[string]any, data []byte) error {
	// parse data
	var v any
	if len(data) == 0 {
		v = map[string]any{}
	} else if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	// only object supported at top-level
	m, ok := v.(map[string]any)
	if !ok {
		return errors.New("payload must be a JSON object")
	}

	// read schema
	st, _ := schema["type"].(string)
	if st != "object" && st != "" {
		return fmt.Errorf("schema.type %q not supported", st)
	}
	props, _ := schema["properties"].(map[string]any)
	reqArr, _ := schema["required"].([]any)

	// required
	for _, r := range reqArr {
		key, _ := r.(string)
		if key == "" {
			continue
		}
		if _, ok := m[key]; !ok {
			return fmt.Errorf("missing required field: %s", key)
		}
	}

	// types
	for k, raw := range props {
		pm, _ := raw.(map[string]any)
		t, _ := pm["type"].(string)
		if t == "" {
			continue
		}
		val, exists := m[k]
		if !exists {
			continue
		}
		if err := checkType(t, val); err != nil {
			return fmt.Errorf("field %s: %w", k, err)
		}
	}
	return nil
}

func checkType(t string, v any) error {
	switch t {
	case "string":
		if _, ok := v.(string); ok {
			return nil
		}
		return typeErr("string", v)
	case "boolean":
		if _, ok := v.(bool); ok {
			return nil
		}
		return typeErr("boolean", v)
	case "number":
		switch v.(type) {
		case float64:
			return nil
		case json.Number:
			if _, err := strconv.ParseFloat(string(v.(json.Number)), 64); err == nil {
				return nil
			}
		}
		return typeErr("number", v)
	case "integer":
		switch val := v.(type) {
		case float64:
			if val == float64(int64(val)) {
				return nil
			}
		case json.Number:
			if _, err := strconv.ParseInt(string(val), 10, 64); err == nil {
				return nil
			}
		}
		return typeErr("integer", v)
	case "object":
		if _, ok := v.(map[string]any); ok {
			return nil
		}
		return typeErr("object", v)
	default:
		// treat unknown types as pass-through
		return nil
	}
}

func typeErr(expect string, v any) error {
	return fmt.Errorf("want %s, got %T", expect, v)
}
