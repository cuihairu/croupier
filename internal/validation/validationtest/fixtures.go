// Package validationtest holds the shared schema-validation fixture table
// (SUI-016). Every server entry that validates an invoke payload — console
// execution and the direct function invoke — asserts the same verdict for the
// same schema and payload, so a divergence between the entries shows up as a
// failing fixture instead of a silent behavior drift.
package validationtest

import "encoding/json"

// Fixture is one payload-vs-schema case shared by the entries.
type Fixture struct {
	// Name identifies the case in test output.
	Name string
	// Schema is the declared input contract; nil means the entry sees no
	// schema at all (the missing-schema policy, not a validation case).
	Schema map[string]any
	// Payload is the request body under test.
	Payload string
	// WantPass: the payload satisfies the contract (or no contract exists).
	WantPass bool
	// WantDefect: the schema does not compile or the payload is not valid
	// JSON. Entries must reject it instead of treating the contract as
	// absent — this is the missing-vs-broken distinction of SUI-016.
	WantDefect bool
	// WantField names a field the verdict must surface (required path or the
	// offending unknown property). Empty skips the assertion.
	WantField string
}

// RawSchema renders the fixture schema as the JSON text entries receive it
// (console bindings carry the schema as a JSON string).
func (f Fixture) RawSchema(t TestingT) string {
	t.Helper()
	if f.Schema == nil {
		return ""
	}
	raw, err := json.Marshal(f.Schema)
	if err != nil {
		t.Fatalf("fixture %s: marshal schema: %v", f.Name, err)
	}
	return string(raw)
}

// TestingT is the minimal testing surface the fixture helpers need, so
// packages can pass their own *testing.T without a dependency cycle.
type TestingT interface {
	Helper()
	Fatalf(format string, args ...any)
}

// SchemaValidationFixtures is the shared comparison set: required handling,
// type checks, the explicit and omitted additionalProperties behavior, valid
// dictionary data, and the defect cases an entry must distinguish from a
// missing schema.
func SchemaValidationFixtures() []Fixture {
	return []Fixture{
		{
			Name:     "valid_minimal_object",
			Schema:   objectSchema(property("playerId", stringType()), nil),
			Payload:  `{"playerId":"p-1"}`,
			WantPass: true,
		},
		{
			Name:      "missing_required_fails",
			Schema:    objectSchema(property("playerId", stringType()), required("playerId")),
			Payload:   `{}`,
			WantField: "playerId",
		},
		{
			Name:      "wrong_type_fails",
			Schema:    objectSchema(property("amount", integerType()), nil),
			Payload:   `{"amount":"1"}`,
			WantField: "amount",
		},
		{
			Name: "explicit_additional_properties_false_rejects_unknown",
			Schema: withAdditionalProperties(
				objectSchema(property("playerId", stringType()), nil), false),
			Payload:   `{"playerId":"p-1","extra":"context"}`,
			WantField: "extra",
		},
		{
			Name: "explicit_additional_properties_true_accepts_unknown",
			Schema: withAdditionalProperties(
				objectSchema(property("playerId", stringType()), nil), true),
			Payload:  `{"playerId":"p-1","extra":"context"}`,
			WantPass: true,
		},
		{
			// SUI-016 pin: omitted additionalProperties follows standard JSON
			// Schema semantics on every entry — unknown fields pass.
			Name:     "omitted_additional_properties_accepts_unknown",
			Schema:   objectSchema(property("playerId", stringType()), nil),
			Payload:  `{"playerId":"p-1","extra":"context"}`,
			WantPass: true,
		},
		{
			// SUI-016 pin: an object without properties is a free-form
			// dictionary and accepts any keys.
			Name:     "dictionary_without_properties_accepts_any_keys",
			Schema:   map[string]any{"type": "object"},
			Payload:  `{"anything":{"nested":[1,2],"k":"v"},"n":7}`,
			WantPass: true,
		},
		{
			// SUI-016 pin: additionalProperties as a schema validates the
			// unknown values against it.
			Name: "dictionary_typed_values_valid",
			Schema: withAdditionalPropertiesSchema(
				map[string]any{"type": "object"}, stringType()),
			Payload:  `{"a":"x","b":"y"}`,
			WantPass: true,
		},
		{
			Name: "dictionary_typed_values_invalid",
			Schema: withAdditionalPropertiesSchema(
				map[string]any{"type": "object"}, stringType()),
			Payload:   `{"a":1}`,
			WantField: "a",
		},
		{
			Name:      "nested_required_fails_with_path",
			Schema:    objectSchema(property("profile", nestedProfile()), required("profile")),
			Payload:   `{"profile":{}}`,
			WantField: "profile.level",
		},
		{
			Name: "enum_violation_fails",
			Schema: objectSchema(property("status", map[string]any{
				"type": "string",
				"enum": []any{"active", "banned"},
			}), nil),
			Payload:   `{"status":"deleted"}`,
			WantField: "status",
		},
		{
			// Missing schema is pass-through, never an error: there is no
			// contract to enforce. Entries short-circuit before the engine.
			Name:     "missing_schema_passes",
			Schema:   nil,
			Payload:  `{"any":"payload"}`,
			WantPass: true,
		},
		{
			// SUI-016 distinction: a declared schema that cannot compile is a
			// defect — rejected, not treated as absent.
			Name:       "schema_compile_failure_rejected",
			Schema:     map[string]any{"type": 42},
			Payload:    `{}`,
			WantDefect: true,
		},
		{
			// SUI-016 distinction: a payload that is not valid JSON is a
			// defect — rejected on both entries.
			Name:       "payload_invalid_json_rejected",
			Schema:     objectSchema(property("playerId", stringType()), nil),
			Payload:    `{broken`,
			WantDefect: true,
		},
	}
}

func objectSchema(properties map[string]any, requiredNames []string) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(requiredNames) > 0 {
		list := make([]any, len(requiredNames))
		for i, name := range requiredNames {
			list[i] = name
		}
		schema["required"] = list
	}
	return schema
}

func required(names ...string) []string {
	return names
}

func property(name string, schema map[string]any) map[string]any {
	return map[string]any{name: schema}
}

func stringType() map[string]any {
	return map[string]any{"type": "string"}
}

func integerType() map[string]any {
	return map[string]any{"type": "integer"}
}

func nestedProfile() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"level": map[string]any{"type": "string"},
		},
		"required": []any{"level"},
	}
}

func withAdditionalProperties(schema map[string]any, value bool) map[string]any {
	schema["additionalProperties"] = value
	return schema
}

func withAdditionalPropertiesSchema(schema map[string]any, value map[string]any) map[string]any {
	schema["additionalProperties"] = value
	return schema
}
