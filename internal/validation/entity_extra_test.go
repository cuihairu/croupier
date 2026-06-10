package validation

import (
	"testing"
)

func TestValidateEntityDefinition_IDNotString(t *testing.T) {
	tests := []struct {
		name   string
		entity map[string]any
	}{
		{
			name: "id is number",
			entity: map[string]any{
				"id":     123,
				"type":   "entity",
				"schema": map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
		{
			name: "id is boolean",
			entity: map[string]any{
				"id":     true,
				"type":   "entity",
				"schema": map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
		{
			name: "id is array",
			entity: map[string]any{
				"id":     []any{"a", "b"},
				"type":   "entity",
				"schema": map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateEntityDefinition(tt.entity)
			if len(errors) == 0 {
				t.Error("Expected error for non-string id")
			}
		})
	}
}

func TestValidateEntityDefinition_IDWhitespaceOnly(t *testing.T) {
	entity := map[string]any{
		"id":     "   ",
		"type":   "entity",
		"schema": map[string]any{"type": "object", "properties": map[string]any{}},
	}

	errors := ValidateEntityDefinition(entity)
	if len(errors) == 0 {
		t.Error("Expected error for whitespace-only id")
	}
}

func TestValidateEntityDefinition_SchemaNotMap(t *testing.T) {
	entity := map[string]any{
		"id":     "test",
		"type":   "entity",
		"schema": "not_a_map",
	}

	errors := ValidateEntityDefinition(entity)
	if len(errors) == 0 {
		t.Error("Expected error for non-map schema")
	}

	found := false
	for _, err := range errors {
		if err == "schema must be a JSON object" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'schema must be a JSON object' error")
	}
}

func TestValidateEntityDefinition_OperationsNotMap(t *testing.T) {
	entity := map[string]any{
		"id":         "test",
		"type":       "entity",
		"schema":     map[string]any{"type": "object", "properties": map[string]any{}},
		"operations": "not_a_map",
	}

	errors := ValidateEntityDefinition(entity)
	if len(errors) == 0 {
		t.Error("Expected error for non-map operations")
	}

	found := false
	for _, err := range errors {
		if err == "operations must be a JSON object" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'operations must be a JSON object' error")
	}
}

func TestValidateEntityDefinition_UINotMap(t *testing.T) {
	entity := map[string]any{
		"id":     "test",
		"type":   "entity",
		"schema": map[string]any{"type": "object", "properties": map[string]any{}},
		"ui":     "not_a_map",
	}

	errors := ValidateEntityDefinition(entity)
	if len(errors) == 0 {
		t.Error("Expected error for non-map ui")
	}

	found := false
	for _, err := range errors {
		if err == "ui must be a JSON object" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'ui must be a JSON object' error")
	}
}

func TestValidateJSONSchema_PropertiesNotMap(t *testing.T) {
	schema := map[string]any{
		"type":       "object",
		"properties": "not_a_map",
	}

	errors := validateJSONSchema(schema)
	if len(errors) == 0 {
		t.Error("Expected error for non-map properties")
	}

	found := false
	for _, err := range errors {
		if err == "schema.properties must be a JSON object" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'schema.properties must be a JSON object' error")
	}
}

func TestValidateJSONSchema_RequiredNotArray(t *testing.T) {
	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
		"required":   "not_an_array",
	}

	errors := validateJSONSchema(schema)
	if len(errors) == 0 {
		t.Error("Expected error for non-array required")
	}

	found := false
	for _, err := range errors {
		if err == "schema.required must be an array" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'schema.required must be an array' error")
	}
}

func TestValidateSchemaProperties_TypeNotString(t *testing.T) {
	props := map[string]any{
		"field": map[string]any{
			"type": 123, // Should be string
		},
	}

	errors := validateSchemaProperties(props)
	if len(errors) == 0 {
		t.Error("Expected error for non-string type")
	}

	found := false
	for _, err := range errors {
		if err == "property 'field' type must be a string" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'property 'field' type must be a string' error")
	}
}

func TestValidateSchemaProperties_EnumNotArray(t *testing.T) {
	props := map[string]any{
		"status": map[string]any{
			"type": "string",
			"enum": "not_an_array",
		},
	}

	errors := validateSchemaProperties(props)
	if len(errors) == 0 {
		t.Error("Expected error for non-array enum")
	}

	found := false
	for _, err := range errors {
		if err == "property 'status' enum must be an array" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'property 'status' enum must be an array' error")
	}
}

func TestValidateSchemaProperties_FormatNotString(t *testing.T) {
	props := map[string]any{
		"email": map[string]any{
			"type":   "string",
			"format": 123, // Should be string
		},
	}

	errors := validateSchemaProperties(props)
	if len(errors) == 0 {
		t.Error("Expected error for non-string format")
	}

	found := false
	for _, err := range errors {
		if err == "property 'email' format must be a string" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'property 'email' format must be a string' error")
	}
}

func TestValidateSchemaProperties_MinLengthNotNumber(t *testing.T) {
	props := map[string]any{
		"name": map[string]any{
			"type":      "string",
			"minLength": "not_a_number",
		},
	}

	errors := validateSchemaProperties(props)
	if len(errors) == 0 {
		t.Error("Expected error for non-number minLength")
	}
}

func TestValidateSchemaProperties_MaxLengthNotNumber(t *testing.T) {
	props := map[string]any{
		"name": map[string]any{
			"type":      "string",
			"maxLength": "not_a_number",
		},
	}

	errors := validateSchemaProperties(props)
	if len(errors) == 0 {
		t.Error("Expected error for non-number maxLength")
	}
}

func TestValidateSchemaProperties_MaxLengthNegative(t *testing.T) {
	props := map[string]any{
		"name": map[string]any{
			"type":      "string",
			"maxLength": float64(-1),
		},
	}

	errors := validateSchemaProperties(props)
	if len(errors) == 0 {
		t.Error("Expected error for negative maxLength")
	}
}

func TestValidateSchemaProperties_MinimumNotNumber(t *testing.T) {
	props := map[string]any{
		"price": map[string]any{
			"type":    "number",
			"minimum": "not_a_number",
		},
	}

	errors := validateSchemaProperties(props)
	if len(errors) == 0 {
		t.Error("Expected error for non-number minimum")
	}
}

func TestValidateSchemaProperties_MaximumNotNumber(t *testing.T) {
	props := map[string]any{
		"price": map[string]any{
			"type":    "number",
			"maximum": "not_a_number",
		},
	}

	errors := validateSchemaProperties(props)
	if len(errors) == 0 {
		t.Error("Expected error for non-number maximum")
	}
}

func TestValidateSchemaProperties_PropertyNotMap(t *testing.T) {
	props := map[string]any{
		"field": "not_a_map",
	}

	errors := validateSchemaProperties(props)
	if len(errors) == 0 {
		t.Error("Expected error for non-map property definition")
	}

	found := false
	for _, err := range errors {
		if err == "property 'field' definition must be a JSON object" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'property 'field' definition must be a JSON object' error")
	}
}

func TestValidateUIConfig_TitleTemplateNotString(t *testing.T) {
	ui := map[string]any{
		"title_template": 123,
	}

	errors := validateUIConfig(ui)
	if len(errors) == 0 {
		t.Error("Expected error for non-string title_template")
	}
}

func TestValidateUIConfig_IconFieldNotString(t *testing.T) {
	ui := map[string]any{
		"icon_field": 123,
	}

	errors := validateUIConfig(ui)
	if len(errors) == 0 {
		t.Error("Expected error for non-string icon_field")
	}
}

func TestValidateUIConfig_StatusFieldNotString(t *testing.T) {
	ui := map[string]any{
		"status_field": 123,
	}

	errors := validateUIConfig(ui)
	if len(errors) == 0 {
		t.Error("Expected error for non-string status_field")
	}
}

func TestValidateEntityDefinition_AllBranches(t *testing.T) {
	// Test entity with all valid fields
	entity := map[string]any{
		"id":   "test-entity",
		"type": "entity",
		"schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":      "string",
					"minLength": float64(1),
					"maxLength": float64(100),
					"format":    "email",
					"enum":      []any{"a", "b"},
				},
				"age": map[string]any{
					"type":    "integer",
					"minimum": float64(0),
					"maximum": float64(150),
				},
			},
			"required": []any{"name"},
		},
		"operations": map[string]any{
			"create": "func_create",
			"read":   []any{"func_read"},
		},
		"ui": map[string]any{
			"display_field":  "name",
			"title_template": "User: {name}",
			"icon_field":     "avatar",
			"status_field":   "status",
		},
	}

	errors := ValidateEntityDefinition(entity)
	if len(errors) > 0 {
		t.Errorf("Valid entity should have no errors, got: %v", errors)
	}
}
