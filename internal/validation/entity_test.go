package validation

import (
	"encoding/json"
	"testing"
)

// TestValidateEntityDefinition 测试实体定义验证
func TestValidateEntityDefinition(t *testing.T) {
	tests := []struct {
		name     string
		entity   map[string]any
		wantErr  bool
		errCount int
	}{
		{
			name: "有效实体",
			entity: map[string]any{
				"id":     "test-entity",
				"type":   "entity",
				"schema": map[string]any{"type": "object", "properties": map[string]any{}},
			},
			wantErr: false,
		},
		{
			name:     "缺少 id",
			entity:   map[string]any{"type": "entity", "schema": map[string]any{"type": "object", "properties": map[string]any{}}},
			wantErr:  true,
			errCount: 1,
		},
		{
			name:     "空 id",
			entity:   map[string]any{"id": "", "type": "entity", "schema": map[string]any{"type": "object", "properties": map[string]any{}}},
			wantErr:  true,
			errCount: 1,
		},
		{
			name:     "错误的类型",
			entity:   map[string]any{"id": "test", "type": "wrong", "schema": map[string]any{"type": "object", "properties": map[string]any{}}},
			wantErr:  true,
			errCount: 1,
		},
		{
			name:     "缺少 schema",
			entity:   map[string]any{"id": "test", "type": "entity"},
			wantErr:  true,
			errCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateEntityDefinition(tt.entity)
			if (len(errors) > 0) != tt.wantErr {
				t.Errorf("ValidateEntityDefinition() error = %v, wantErr %v", errors, tt.wantErr)
			}
			if tt.errCount > 0 && len(errors) != tt.errCount {
				t.Errorf("ValidateEntityDefinition() error count = %d, want %d", len(errors), tt.errCount)
			}
		})
	}
}

// TestValidateJSONSchema 测试 JSON Schema 验证
func TestValidateJSONSchema(t *testing.T) {
	tests := []struct {
		name     string
		schema   map[string]any
		wantErr  bool
		errCount int
	}{
		{
			name: "有效 schema",
			schema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
			wantErr: false,
		},
		{
			name:     "缺少 type",
			schema:   map[string]any{"properties": map[string]any{}},
			wantErr:  true,
			errCount: 1,
		},
		{
			name:     "错误的 type",
			schema:   map[string]any{"type": "array", "properties": map[string]any{}},
			wantErr:  true,
			errCount: 1,
		},
		{
			name:     "缺少 properties",
			schema:   map[string]any{"type": "object"},
			wantErr:  true,
			errCount: 1,
		},
		{
			name: "有效的 required 数组",
			schema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
				"required":   []any{"field1", "field2"},
			},
			wantErr: false,
		},
		{
			name: "无效的 required - 非字符串",
			schema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
				"required":   []any{123},
			},
			wantErr:  true,
			errCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateJSONSchema(tt.schema)
			if (len(errors) > 0) != tt.wantErr {
				t.Errorf("validateJSONSchema() error = %v, wantErr %v", errors, tt.wantErr)
			}
			if tt.errCount > 0 && len(errors) != tt.errCount {
				t.Errorf("validateJSONSchema() error count = %d, want %d", len(errors), tt.errCount)
			}
		})
	}
}

// TestValidateSchemaProperties 测试属性验证
func TestValidateSchemaProperties(t *testing.T) {
	tests := []struct {
		name     string
		props    map[string]any
		wantErr  bool
		errCount int
	}{
		{
			name:    "空属性",
			props:   map[string]any{},
			wantErr: false,
		},
		{
			name: "有效的字符串属性",
			props: map[string]any{
				"name": map[string]any{"type": "string"},
			},
			wantErr: false,
		},
		{
			name: "有效的整数属性",
			props: map[string]any{
				"age": map[string]any{"type": "integer"},
			},
			wantErr: false,
		},
		{
			name: "无效的类型",
			props: map[string]any{
				"field": map[string]any{"type": "invalid"},
			},
			wantErr:  true,
			errCount: 1,
		},
		{
			name: "缺少类型",
			props: map[string]any{
				"field": map[string]any{},
			},
			wantErr:  true,
			errCount: 1,
		},
		{
			name: "有效的 enum",
			props: map[string]any{
				"status": map[string]any{
					"type": "string",
					"enum": []any{"active", "inactive"},
				},
			},
			wantErr: false,
		},
		{
			name: "空的 enum",
			props: map[string]any{
				"status": map[string]any{
					"type": "string",
					"enum": []any{},
				},
			},
			wantErr:  true,
			errCount: 1,
		},
		{
			name: "有效的 format",
			props: map[string]any{
				"email": map[string]any{
					"type":   "string",
					"format": "email",
				},
			},
			wantErr: false,
		},
		{
			name: "无效的 format",
			props: map[string]any{
				"date": map[string]any{
					"type":   "string",
					"format": "invalid",
				},
			},
			wantErr:  true,
			errCount: 1,
		},
		{
			name: "有效的字符串约束",
			props: map[string]any{
				"name": map[string]any{
					"type":      "string",
					"minLength": float64(1),
					"maxLength": float64(100),
				},
			},
			wantErr: false,
		},
		{
			name: "负的 minLength",
			props: map[string]any{
				"name": map[string]any{
					"type":      "string",
					"minLength": float64(-1),
				},
			},
			wantErr:  true,
			errCount: 1,
		},
		{
			name: "有效的数字约束",
			props: map[string]any{
				"price": map[string]any{
					"type":    "number",
					"minimum": float64(0),
					"maximum": float64(1000),
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateSchemaProperties(tt.props)
			if (len(errors) > 0) != tt.wantErr {
				t.Errorf("validateSchemaProperties() error = %v, wantErr %v", errors, tt.wantErr)
			}
			if tt.errCount > 0 && len(errors) != tt.errCount {
				t.Errorf("validateSchemaProperties() error count = %d, want %d", len(errors), tt.errCount)
			}
		})
	}
}

// TestValidateOperations 测试操作验证
func TestValidateOperations(t *testing.T) {
	tests := []struct {
		name     string
		ops      map[string]any
		wantErr  bool
		errCount int
	}{
		{
			name:    "空操作",
			ops:     map[string]any{},
			wantErr: false,
		},
		{
			name: "有效的单个操作",
			ops: map[string]any{
				"create": "function_create_user",
			},
			wantErr: false,
		},
		{
			name: "有效的数组操作",
			ops: map[string]any{
				"create": []any{"func1", "func2"},
			},
			wantErr: false,
		},
		{
			name: "无效的操作名",
			ops: map[string]any{
				"invalid_op": "function_id",
			},
			wantErr:  true,
			errCount: 1,
		},
		{
			name: "操作值不是字符串或数组",
			ops: map[string]any{
				"create": 123,
			},
			wantErr:  true,
			errCount: 1,
		},
		{
			name: "数组中包含非字符串",
			ops: map[string]any{
				"create": []any{"func1", 123},
			},
			wantErr:  true,
			errCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateOperations(tt.ops)
			if (len(errors) > 0) != tt.wantErr {
				t.Errorf("validateOperations() error = %v, wantErr %v", errors, tt.wantErr)
			}
			if tt.errCount > 0 && len(errors) != tt.errCount {
				t.Errorf("validateOperations() error count = %d, want %d", len(errors), tt.errCount)
			}
		})
	}
}

// TestValidateUIConfig 测试 UI 配置验证
func TestValidateUIConfig(t *testing.T) {
	tests := []struct {
		name     string
		ui       map[string]any
		wantErr  bool
		errCount int
	}{
		{
			name:    "空 UI 配置",
			ui:      map[string]any{},
			wantErr: false,
		},
		{
			name: "有效的 display_field",
			ui: map[string]any{
				"display_field": "name",
			},
			wantErr: false,
		},
		{
			name: "无效的 display_field - 非字符串",
			ui: map[string]any{
				"display_field": 123,
			},
			wantErr:  true,
			errCount: 1,
		},
		{
			name: "有效的多个 UI 字段",
			ui: map[string]any{
				"display_field":  "name",
				"title_template": "{name}",
				"icon_field":     "icon",
				"status_field":   "status",
			},
			wantErr: false,
		},
		{
			name: "多个无效字段",
			ui: map[string]any{
				"display_field":  123,
				"title_template": 456,
			},
			wantErr:  true,
			errCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateUIConfig(tt.ui)
			if (len(errors) > 0) != tt.wantErr {
				t.Errorf("validateUIConfig() error = %v, wantErr %v", errors, tt.wantErr)
			}
			if tt.errCount > 0 && len(errors) != tt.errCount {
				t.Errorf("validateUIConfig() error count = %d, want %d", len(errors), tt.errCount)
			}
		})
	}
}

// TestValidateEntityDefinition_Complex 测试复杂实体定义
func TestValidateEntityDefinition_Complex(t *testing.T) {
	entity := map[string]any{
		"id":   "user",
		"type": "entity",
		"schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":      "string",
					"minLength": float64(1),
					"maxLength": float64(100),
				},
				"age": map[string]any{
					"type":    "integer",
					"minimum": float64(0),
					"maximum": float64(150),
				},
				"email": map[string]any{
					"type":   "string",
					"format": "email",
				},
				"status": map[string]any{
					"type": "string",
					"enum": []any{"active", "inactive", "pending"},
				},
			},
			"required": []any{"name", "email"},
		},
		"operations": map[string]any{
			"create": []any{"func_create", "func_validate"},
			"read":   "func_read",
			"update": "func_update",
			"delete": "func_delete",
			"list":   "func_list",
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

// TestValidateEntityDefinition_AllErrors 测试所有错误都返回
func TestValidateEntityDefinition_AllErrors(t *testing.T) {
	entity := map[string]any{
		// 缺少 id
		// type 错误
		"type":   "wrong",
		"schema": "not_a_map",
	}

	errors := ValidateEntityDefinition(entity)
	if len(errors) == 0 {
		t.Error("Invalid entity should have errors")
	}

	// 验证返回了多个错误
	if len(errors) < 2 {
		t.Errorf("Expected at least 2 errors, got %d: %v", len(errors), errors)
	}
}

// TestValidateJSON_DataValidation 测试 JSON 数据验证
func TestValidateJSON_DataValidation(t *testing.T) {
	tests := []struct {
		name    string
		schema  map[string]any
		data    []byte
		wantErr bool
	}{
		{
			name: "有效数据",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
				"required": []any{"name"},
			},
			data:    []byte(`{"name":"test"}`),
			wantErr: false,
		},
		{
			name: "空数据",
			schema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
			data:    []byte{},
			wantErr: false,
		},
		{
			name: "无效的 JSON",
			schema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
			data:    []byte(`{invalid}`),
			wantErr: true,
		},
		{
			name: "不是对象",
			schema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
			data:    []byte(`"string"`),
			wantErr: true,
		},
		{
			name: "缺少必填字段",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
				"required": []any{"name"},
			},
			data:    []byte(`{}`),
			wantErr: true,
		},
		{
			name: "错误的字段类型",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"age": map[string]any{"type": "integer"},
				},
			},
			data:    []byte(`{"age":"not_a_number"}`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJSON(tt.schema, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestCheckType 测试类型检查
func TestCheckType(t *testing.T) {
	tests := []struct {
		name    string
		typ     string
		value   any
		wantErr bool
	}{
		{"string 类型", "string", "hello", false},
		{"string 类型错误", "string", 123, true},
		{"boolean 类型 true", "boolean", true, false},
		{"boolean 类型 false", "boolean", false, false},
		{"boolean 类型错误", "boolean", "true", true},
		{"number 类型 float64", "number", float64(123.45), false},
		{"number 类型整数", "number", float64(123), false},
		{"number 类型 JSON.Number", "number", json.Number("123.45"), false},
		{"number 类型错误", "number", "123", true},
		{"integer 类型 整数", "integer", float64(42), false},
		{"integer 类型 浮点错误", "integer", float64(42.5), true},
		{"integer 类型 JSON.Number", "integer", json.Number("42"), false},
		{"object 类型", "object", map[string]any{}, false},
		{"object 类型错误", "object", "not_a_map", true},
		{"未知类型通过", "unknown_type", 123, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkType(tt.typ, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// BenchmarkValidateJSON 性能基准测试
func BenchmarkValidateJSON(b *testing.B) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":  map[string]any{"type": "string"},
			"age":   map[string]any{"type": "integer"},
			"email": map[string]any{"type": "string"},
		},
		"required": []any{"name"},
	}
	data := []byte(`{"name":"John Doe","age":30,"email":"john@example.com"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateJSON(schema, data)
	}
}

// BenchmarkValidateEntityDefinition 性能基准测试
func BenchmarkValidateEntityDefinition(b *testing.B) {
	entity := map[string]any{
		"id":   "test",
		"type": "entity",
		"schema": map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateEntityDefinition(entity)
	}
}
