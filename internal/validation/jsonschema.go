package validation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ValidateJSON validates a JSON payload `data` against a minimal subset of JSON Schema contained in `schema`.
// Supported subset:
// - object/array/scalar types
// - properties/items
// - required
// - enum
// - local $defs/$ref supported by the jsonschema compiler
//
// Object payloads are strict by default: if a schema object declares properties
// but does not declare additionalProperties, unknown fields are rejected.
func ValidateJSON(schema map[string]any, data []byte) error {
	if schema == nil {
		schema = map[string]any{}
	}
	return ValidateJSONSchema(schema, data)
}

// ValidateJSONRaw validates a JSON payload against a raw JSON Schema object.
func ValidateJSONRaw(schema json.RawMessage, data []byte) error {
	if len(bytes.TrimSpace(schema)) == 0 {
		return ValidateJSONSchema(map[string]any{}, data)
	}
	var parsed any
	if err := json.Unmarshal(schema, &parsed); err != nil {
		return fmt.Errorf("invalid JSON Schema: %w", err)
	}
	return ValidateJSONSchema(parsed, data)
}

// ValidateJSONSchema validates a payload against a JSON Schema value.
func ValidateJSONSchema(schema any, data []byte) error {
	payload, err := decodeJSONPayload(data)
	if err != nil {
		return err
	}

	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)
	if err := compiler.AddResource("schema.json", strictSchemaObjects(schema)); err != nil {
		return fmt.Errorf("invalid JSON Schema: %w", err)
	}
	compiled, err := compiler.Compile("schema.json")
	if err != nil {
		return fmt.Errorf("invalid JSON Schema: %w", err)
	}
	if err := compiled.Validate(payload); err != nil {
		return fmt.Errorf("payload does not match schema: %w", err)
	}
	return nil
}

func decodeJSONPayload(data []byte) (any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var payload any
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("invalid JSON: multiple JSON values")
	}
	return payload, nil
}

func strictSchemaObjects(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed)+1)
		for key, item := range typed {
			out[key] = strictSchemaObjects(item)
		}
		if schemaDeclaresObjectProperties(out) {
			if _, ok := out["additionalProperties"]; !ok {
				out["additionalProperties"] = false
			}
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = strictSchemaObjects(item)
		}
		return out
	default:
		return value
	}
}

func schemaDeclaresObjectProperties(schema map[string]any) bool {
	if _, ok := schema["properties"].(map[string]any); !ok {
		return false
	}
	schemaType, ok := schema["type"]
	if !ok {
		return true
	}
	switch typed := schemaType.(type) {
	case string:
		return typed == "object"
	case []any:
		for _, item := range typed {
			if item == "object" {
				return true
			}
		}
	}
	return false
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
