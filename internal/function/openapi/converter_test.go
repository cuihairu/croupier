package openapi

import (
	"testing"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"github.com/getkin/kin-openapi/openapi3"
)

func TestConverter_MetadataToOperation(t *testing.T) {
	converter := NewConverter()

	metadata := &functionv1.FunctionMetadata{
		Id:           "player.ban",
		Version:      "1.0.0",
		Category:     "player",
		Name:         "Ban Player",
		Description:  "Ban a player from the game",
		Tags:         []string{"moderation", "player"},
		InputSchema:  `{"type":"object","properties":{"playerId":{"type":"string"},"reason":{"type":"string"}}}`,
		OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
		Behavior: &functionv1.FunctionBehavior{
			Mode:          functionv1.FunctionBehavior_MODE_COMMAND,
			Idempotent:    false,
			TimeoutMs:     30000,
			RouteStrategy: functionv1.FunctionBehavior_ROUTE_STRATEGY_LB,
		},
		Security: &functionv1.FunctionSecurity{
			RiskLevel:        functionv1.FunctionSecurity_RISK_LEVEL_HIGH,
			Permission:       "player.ban.invoke",
			RequiresApproval: true,
			AuditLog:         true,
		},
	}

	op, err := converter.MetadataToOperation(metadata)
	if err != nil {
		t.Fatalf("MetadataToOperation failed: %v", err)
	}

	if op.OperationID != metadata.Id {
		t.Errorf("Expected OperationID %s, got %s", metadata.Id, op.OperationID)
	}

	if op.Summary != metadata.Name {
		t.Errorf("Expected Summary %s, got %s", metadata.Name, op.Summary)
	}

	// Check extensions
	if category, ok := op.Extensions["x-category"].(string); !ok || category != metadata.Category {
		t.Errorf("Expected x-category %s, got %v", metadata.Category, op.Extensions["x-category"])
	}

	if risk, ok := op.Extensions["x-risk"].(string); !ok || risk != "high" {
		t.Errorf("Expected x-risk high, got %v", op.Extensions["x-risk"])
	}

	// Check request body
	if op.RequestBody == nil {
		t.Error("Expected RequestBody to be set")
	}

	// Check response
	if op.Responses == nil {
		t.Error("Expected Responses to be set")
	}
}

func TestConverter_OperationToMetadata(t *testing.T) {
	converter := NewConverter()

	op := &openapi3.Operation{
		OperationID: "player.ban",
		Summary:     "Ban Player",
		Description: "Ban a player from the game",
		Tags:        []string{"moderation", "player"},
		Extensions:  map[string]interface{}{},
		RequestBody: &openapi3.RequestBodyRef{
			Value: &openapi3.RequestBody{
				Content: openapi3.Content{
					"application/json": &openapi3.MediaType{
						Schema: &openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: func() *openapi3.Types { t := openapi3.Types{"object"}; return &t }(),
							},
						},
					},
				},
			},
		},
		Responses: openapi3.NewResponses(),
	}
	op.Extensions["x-category"] = "player"
	op.Extensions["x-risk"] = "high"
	op.Extensions["x-permission"] = "player.ban.invoke"

	response := &openapi3.Response{
		Content: openapi3.Content{
			"application/json": &openapi3.MediaType{
				Schema: &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type: func() *openapi3.Types { t := openapi3.Types{"object"}; return &t }(),
					},
				},
			},
		},
	}
	desc := "Success response"
	response.Description = &desc
	op.Responses.Set("200", &openapi3.ResponseRef{Value: response})

	metadata, err := converter.ImportToMetadata("player.ban", op)
	if err != nil {
		t.Fatalf("ImportToMetadata failed: %v", err)
	}

	if metadata.Id != "player.ban" {
		t.Errorf("Expected ID player.ban, got %s", metadata.Id)
	}

	if metadata.Category != "player" {
		t.Errorf("Expected category player, got %s", metadata.Category)
	}

	if metadata.Behavior.Mode != functionv1.FunctionBehavior_MODE_QUERY {
		t.Errorf("Expected default mode QUERY, got %v", metadata.Behavior.Mode)
	}

	if metadata.Security.RiskLevel != functionv1.FunctionSecurity_RISK_LEVEL_HIGH {
		t.Errorf("Expected risk level RISK_HIGH, got %v", metadata.Security.RiskLevel)
	}

	if metadata.Security.Permission != "player.ban.invoke" {
		t.Errorf("Expected permission player.ban.invoke, got %s", metadata.Security.Permission)
	}
}

func TestConverter_ImportFromSpec(t *testing.T) {
	converter := NewConverter()

	spec := &openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Paths: openapi3.NewPaths(),
	}

	// Add a path with operation
	pathItem := &openapi3.PathItem{}
	op1 := &openapi3.Operation{
		OperationID: "player.ban",
		Summary:     "Ban Player",
		Tags:        []string{"player", "moderation"},
		Extensions:  map[string]interface{}{},
	}
	op1.Extensions["x-category"] = "player"
	op1.Extensions["x-risk"] = "high"

	objectType := openapi3.Types{"object"}
	op1.RequestBody = &openapi3.RequestBodyRef{
		Value: &openapi3.RequestBody{
			Content: openapi3.Content{
				"application/json": &openapi3.MediaType{
					Schema: &openapi3.SchemaRef{
						Value: &openapi3.Schema{Type: &objectType},
					},
				},
			},
		},
	}

	response := &openapi3.Response{
		Content: openapi3.Content{
			"application/json": &openapi3.MediaType{
				Schema: &openapi3.SchemaRef{
					Value: &openapi3.Schema{Type: &objectType},
				},
			},
		},
	}
	desc := "Success"
	response.Description = &desc
	responses := openapi3.NewResponses()
	responses.Set("200", &openapi3.ResponseRef{Value: response})
	op1.Responses = responses

	pathItem.Post = op1
	spec.Paths.Set("/players/ban", pathItem)

	metadatas, err := converter.ImportFromSpec(spec, nil)
	if err != nil {
		t.Fatalf("ImportFromSpec failed: %v", err)
	}

	if len(metadatas) != 1 {
		t.Fatalf("Expected 1 metadata, got %d", len(metadatas))
	}

	md := metadatas[0]
	if md.Id != "player.ban" {
		t.Errorf("Expected ID player.ban, got %s", md.Id)
	}

	if md.Category != "player" {
		t.Errorf("Expected category player, got %s", md.Category)
	}

	if md.Security.RiskLevel != functionv1.FunctionSecurity_RISK_LEVEL_HIGH {
		t.Errorf("Expected risk level RISK_HIGH, got %v", md.Security.RiskLevel)
	}
}

func TestConverter_ExportToSpec(t *testing.T) {
	converter := NewConverter()

	metadatas := []*functionv1.FunctionMetadata{
		{
			Id:          "player.ban",
			Category:    "player",
			Name:        "Ban Player",
			Description: "Ban a player",
			Tags:        []string{"moderation"},
			InputSchema: `{"type":"object"}`,
			Behavior:    &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_COMMAND},
			Security:    &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_HIGH},
		},
	}

	spec, err := converter.ExportToSpec(metadatas)
	if err != nil {
		t.Fatalf("ExportToSpec failed: %v", err)
	}

	if spec.OpenAPI != "3.0.3" {
		t.Errorf("Expected OpenAPI version 3.0.3, got %s", spec.OpenAPI)
	}

	if len(spec.Paths.Map()) != 1 {
		t.Errorf("Expected 1 path, got %d", len(spec.Paths.Map()))
	}
}

func TestSchemaMapper_OpenAPIToJSONSchema(t *testing.T) {
	mapper := NewSchemaMapper()

	objectType := openapi3.Types{"object"}
	schema := &openapi3.Schema{
		Type:        &objectType,
		Description: "Test schema",
	}

	jsonSchema, err := mapper.OpenAPIToJSONSchema(schema)
	if err != nil {
		t.Fatalf("OpenAPIToJSONSchema failed: %v", err)
	}

	if jsonSchema == "" {
		t.Error("Expected non-empty JSON Schema")
	}

	// Verify it's valid JSON
	if err := mapper.ValidateJSONSchema(jsonSchema); err != nil {
		t.Errorf("Invalid JSON Schema: %v", err)
	}
}

func TestSchemaMapper_JSONSchemaToOpenAPI(t *testing.T) {
	mapper := NewSchemaMapper()

	jsonSchema := `{"type":"object","properties":{"name":{"type":"string"}}}`

	schemaRef := mapper.JSONSchemaToOpenAPI(jsonSchema)
	if schemaRef == nil {
		t.Fatal("Expected non-nil SchemaRef")
	}

	if schemaRef.Value == nil {
		t.Fatal("Expected non-nil Schema")
	}
}

func TestSchemaMapper_GetSchemaType(t *testing.T) {
	mapper := NewSchemaMapper()

	tests := []struct {
		name     string
		schema   string
		expected string
	}{
		{
			name:     "object type",
			schema:   `{"type":"object"}`,
			expected: "object",
		},
		{
			name:     "array type",
			schema:   `{"type":"array"}`,
			expected: "array",
		},
		{
			name:     "string type",
			schema:   `{"type":"string"}`,
			expected: "string",
		},
		{
			name:     "$ref",
			schema:   `{"$ref":"#/definitions/something"}`,
			expected: "$ref",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schemaType, err := mapper.GetSchemaType(tt.schema)
			if err != nil {
				t.Fatalf("GetSchemaType failed: %v", err)
			}

			if schemaType != tt.expected {
				t.Errorf("Expected type %s, got %s", tt.expected, schemaType)
			}
		})
	}
}

func TestSchemaMapper_IsObjectSchema(t *testing.T) {
	mapper := NewSchemaMapper()

	if !mapper.IsObjectSchema(`{"type":"object"}`) {
		t.Error("Expected true for object schema")
	}

	if mapper.IsObjectSchema(`{"type":"array"}`) {
		t.Error("Expected false for array schema")
	}
}

func TestSchemaMapper_GetObjectProperties(t *testing.T) {
	mapper := NewSchemaMapper()

	props, err := mapper.GetObjectProperties(`{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}}}`)
	if err != nil {
		t.Fatalf("GetObjectProperties failed: %v", err)
	}

	if len(props) != 2 {
		t.Errorf("Expected 2 properties, got %d", len(props))
	}
}

func TestSchemaMapper_ValidateJSONSchema(t *testing.T) {
	mapper := NewSchemaMapper()

	tests := []struct {
		name      string
		schema    string
		expectErr bool
	}{
		{
			name:      "valid object schema",
			schema:    `{"type":"object"}`,
			expectErr: false,
		},
		{
			name:      "invalid JSON",
			schema:    `{invalid}`,
			expectErr: true,
		},
		{
			name:      "empty schema",
			schema:    ``,
			expectErr: true,
		},
		{
			name:      "schema with ref",
			schema:    `{"$ref":"#/definitions/test"}`,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mapper.ValidateJSONSchema(tt.schema)
			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestSchemaMapper_MergeSchemas(t *testing.T) {
	mapper := NewSchemaMapper()

	schema1 := `{"type":"object"}`
	schema2 := `{"type":"string"}`

	merged, err := mapper.MergeSchemas(schema1, schema2)
	if err != nil {
		t.Fatalf("MergeSchemas failed: %v", err)
	}

	// Verify merged schema contains allOf
	if err := mapper.ValidateJSONSchema(merged); err != nil {
		t.Errorf("Merged schema is invalid: %v", err)
	}
}

func TestParseRiskLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected functionv1.FunctionSecurity_RiskLevel
	}{
		{"low", functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		{"safe", functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		{"medium", functionv1.FunctionSecurity_RISK_LEVEL_MEDIUM},
		{"high", functionv1.FunctionSecurity_RISK_LEVEL_HIGH},
		{"danger", functionv1.FunctionSecurity_RISK_LEVEL_DANGER},
		{"critical", functionv1.FunctionSecurity_RISK_LEVEL_DANGER},
		{"unknown", functionv1.FunctionSecurity_RISK_LEVEL_MEDIUM},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseRiskLevel(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestParseRouteStrategy(t *testing.T) {
	tests := []struct {
		input    string
		expected functionv1.FunctionBehavior_RouteStrategy
	}{
		{"lb", functionv1.FunctionBehavior_ROUTE_STRATEGY_LB},
		{"load_balance", functionv1.FunctionBehavior_ROUTE_STRATEGY_LB},
		{"broadcast", functionv1.FunctionBehavior_ROUTE_STRATEGY_BROADCAST},
		{"targeted", functionv1.FunctionBehavior_ROUTE_STRATEGY_TARGETED},
		{"hash", functionv1.FunctionBehavior_ROUTE_STRATEGY_HASH},
		{"consistent_hash", functionv1.FunctionBehavior_ROUTE_STRATEGY_HASH},
		{"unknown", functionv1.FunctionBehavior_ROUTE_STRATEGY_LB},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseRouteStrategy(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestConverter_ImportFromSpecData(t *testing.T) {
	converter := NewConverter()

	t.Run("import valid spec data", func(t *testing.T) {
		specData := `{
			"openapi": "3.0.3",
			"info": {"title": "Test API", "version": "1.0.0"},
			"paths": {
				"/players": {
					"get": {
						"operationId": "player.list",
						"summary": "List players",
						"responses": {
							"200": {
								"description": "Success",
								"content": {
									"application/json": {
										"schema": {"type": "array"}
									}
								}
							}
						}
					}
				}
			}
		}`

		metadatas, err := converter.ImportFromSpecData([]byte(specData), nil)
		if err != nil {
			t.Fatalf("ImportFromSpecData failed: %v", err)
		}

		if len(metadatas) != 1 {
			t.Errorf("Expected 1 metadata, got %d", len(metadatas))
		}

		if metadatas[0].Id != "player.list" {
			t.Errorf("Expected ID player.list, got %s", metadatas[0].Id)
		}
	})

	t.Run("import invalid spec data", func(t *testing.T) {
		invalidData := []byte(`{invalid json`)

		_, err := converter.ImportFromSpecData(invalidData, nil)
		if err == nil {
			t.Error("Expected error for invalid JSON, got nil")
		}
	})

	t.Run("import with continue on error", func(t *testing.T) {
		specData := `{
			"openapi": "3.0.3",
			"info": {"title": "Test API", "version": "1.0.0"},
			"paths": {
				"/players": {
					"get": {
						"operationId": "player.list",
						"summary": "List players",
						"responses": {
							"200": {"description": "Success"}
						}
					}
				}
			}
		}`

		opts := &ImportOptions{
			ContinueOnError: true,
		}

		metadatas, err := converter.ImportFromSpecData([]byte(specData), opts)
		if err != nil {
			t.Fatalf("ImportFromSpecData with ContinueOnError failed: %v", err)
		}

		// Should return 1 metadata even with no request body
		if len(metadatas) != 1 {
			t.Errorf("Expected 1 metadata, got %d", len(metadatas))
		}
	})

	t.Run("import with options", func(t *testing.T) {
		specData := `{
			"openapi": "3.0.3",
			"info": {"title": "Test API", "version": "1.0.0"},
			"paths": {
				"/players": {
					"post": {
						"operationId": "player.create",
						"summary": "Create player",
						"requestBody": {
							"content": {
								"application/json": {
									"schema": {"type": "object"}
								}
							}
						},
						"responses": {
							"201": {"description": "Created"}
						}
					}
				}
			}
		}`

		opts := &ImportOptions{
			DefaultTimeoutMs: 45000,
		}

		metadatas, err := converter.ImportFromSpecData([]byte(specData), opts)
		if err != nil {
			t.Fatalf("ImportFromSpecData with options failed: %v", err)
		}

		if len(metadatas) != 1 {
			t.Fatalf("Expected 1 metadata, got %d", len(metadatas))
		}

		md := metadatas[0]
		if md.Behavior.TimeoutMs != 45000 {
			t.Errorf("Expected timeout 45000, got %d", md.Behavior.TimeoutMs)
		}
	})
}

func TestConverter_ImportFromSpec_NilSpec(t *testing.T) {
	converter := NewConverter()

	_, err := converter.ImportFromSpec(nil, nil)
	if err == nil {
		t.Error("Expected error for nil spec, got nil")
	}
}

func TestSchemaMapper_IsArraySchema(t *testing.T) {
	mapper := NewSchemaMapper()

	t.Run("valid array schema", func(t *testing.T) {
		if !mapper.IsArraySchema(`{"type":"array"}`) {
			t.Error("Expected true for array schema")
		}
	})

	t.Run("non-array schema", func(t *testing.T) {
		if mapper.IsArraySchema(`{"type":"object"}`) {
			t.Error("Expected false for object schema")
		}
		if mapper.IsArraySchema(`{"type":"string"}`) {
			t.Error("Expected false for string schema")
		}
	})

	t.Run("invalid schema", func(t *testing.T) {
		if mapper.IsArraySchema(`{invalid}`) {
			t.Error("Expected false for invalid schema")
		}
	})
}

func TestSchemaMapper_InferType(t *testing.T) {
	mapper := NewSchemaMapper()

	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{"string value", "hello", "string"},
		{"integer value", 42, "integer"},
		{"float value", 3.14, "number"},
		{"bool value", true, "boolean"},
		{"nil value", nil, "string"},
		{"array value", []interface{}{}, "array"},
		{"map value", map[string]interface{}{}, "object"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.inferType(tt.value)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestConverter_ExportToSpec_WithNilMetadata(t *testing.T) {
	converter := NewConverter()

	metadatas := []*functionv1.FunctionMetadata{
		{
			Id:       "player.get",
			Category: "player",
			Name:     "Get Player",
			Behavior: &functionv1.FunctionBehavior{},
			Security: &functionv1.FunctionSecurity{},
		},
		nil, // Nil metadata should be skipped
		{
			Id:       "game.create",
			Category: "game",
			Name:     "Create Game",
			Behavior: &functionv1.FunctionBehavior{},
			Security: &functionv1.FunctionSecurity{},
		},
	}

	spec, err := converter.ExportToSpec(metadatas)
	if err != nil {
		t.Fatalf("ExportToSpec failed: %v", err)
	}

	// Should have 2 paths (nil metadata skipped)
	if len(spec.Paths.Map()) != 2 {
		t.Errorf("Expected 2 paths, got %d", len(spec.Paths.Map()))
	}
}

func TestConverter_ImportToMetadata_NilOperation(t *testing.T) {
	converter := NewConverter()

	_, err := converter.ImportToMetadata("test.id", nil)
	if err == nil {
		t.Error("Expected error for nil operation, got nil")
	}
}

func TestConverter_DeriveName(t *testing.T) {
	t.Run("operation with summary", func(t *testing.T) {
		op := &openapi3.Operation{
			Summary:     "Get Player",
			OperationID: "player.get",
		}

		name := deriveName(op)
		if name != "Get Player" {
			t.Errorf("Expected 'Get Player', got '%s'", name)
		}
	})

	t.Run("operation with only operationId", func(t *testing.T) {
		op := &openapi3.Operation{
			OperationID: "player_ban",
		}

		name := deriveName(op)
		if name != "Player ban" {
			t.Errorf("Expected 'Player ban', got '%s'", name)
		}
	})

	t.Run("operation with no summary or operationId", func(t *testing.T) {
		op := &openapi3.Operation{}

		name := deriveName(op)
		if name != "Unnamed Function" {
			t.Errorf("Expected 'Unnamed Function', got '%s'", name)
		}
	})

	t.Run("nil operation", func(t *testing.T) {
		name := deriveName(nil)
		if name != "Unnamed Function" {
			t.Errorf("Expected 'Unnamed Function', got '%s'", name)
		}
	})
}

func TestConverter_DeriveFunctionID(t *testing.T) {
	t.Run("operation with operationId", func(t *testing.T) {
		op := &openapi3.Operation{
			OperationID: "player.get",
		}

		id := deriveFunctionID(op, "/api/players")
		if id != "player.get" {
			t.Errorf("Expected 'player.get', got '%s'", id)
		}
	})

	t.Run("generate from path", func(t *testing.T) {
		op := &openapi3.Operation{}

		id := deriveFunctionID(op, "/api/players/{id}")
		if id != "api.players.{id}" {
			t.Errorf("Expected 'api.players.{id}', got '%s'", id)
		}
	})

	t.Run("nil operation and empty path", func(t *testing.T) {
		id := deriveFunctionID(nil, "")
		if id != "unknown.function" {
			t.Errorf("Expected 'unknown.function', got '%s'", id)
		}
	})

	t.Run("empty path", func(t *testing.T) {
		id := deriveFunctionID(&openapi3.Operation{}, "")
		if id != "unknown.function" {
			t.Errorf("Expected 'unknown.function', got '%s'", id)
		}
	})
}

func TestSchemaMapper_GetObjectProperties_ErrorCases(t *testing.T) {
	mapper := NewSchemaMapper()

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := mapper.GetObjectProperties(`{invalid}`)
		if err == nil {
			t.Error("Expected error for invalid JSON")
		}
	})

	t.Run("non-object schema", func(t *testing.T) {
		_, err := mapper.GetObjectProperties(`{"type":"string"}`)
		if err == nil {
			t.Error("Expected error for non-object schema")
		}
	})

	t.Run("missing properties field", func(t *testing.T) {
		_, err := mapper.GetObjectProperties(`{"type":"object"}`)
		if err == nil {
			t.Error("Expected error for schema without properties")
		}
	})
}

func TestConverter_ExportToSpec_EmptyMetadatas(t *testing.T) {
	converter := NewConverter()

	spec, err := converter.ExportToSpec([]*functionv1.FunctionMetadata{})
	if err != nil {
		t.Fatalf("ExportToSpec failed: %v", err)
	}

	if spec.OpenAPI != "3.0.3" {
		t.Errorf("Expected OpenAPI 3.0.3, got %s", spec.OpenAPI)
	}

	if len(spec.Paths.Map()) != 0 {
		t.Errorf("Expected 0 paths, got %d", len(spec.Paths.Map()))
	}
}
