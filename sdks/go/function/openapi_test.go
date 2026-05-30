package function

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
	"github.com/getkin/kin-openapi/openapi3"
)

// mockClient is a mock implementation of croupier.Client for testing.
type mockClient struct {
	registeredFunctions map[string]croupier.FunctionDescriptor
}

func newMockClient() *mockClient {
	return &mockClient{
		registeredFunctions: make(map[string]croupier.FunctionDescriptor),
	}
}

func (m *mockClient) RegisterFunction(desc croupier.FunctionDescriptor, handler croupier.FunctionHandler) error {
	m.registeredFunctions[desc.ID] = desc
	return nil
}

func (m *mockClient) Connect(ctx context.Context) error {
	return nil
}

func (m *mockClient) Serve(ctx context.Context) error {
	return nil
}

func (m *mockClient) Stop() error {
	return nil
}

func (m *mockClient) Close() error {
	return nil
}

func TestRegistry_RegisterFromOpenAPI(t *testing.T) {
	// Simple OpenAPI spec
	spec := []byte(`{
		"openapi": "3.0.3",
		"info": {
			"title": "Test API",
			"version": "1.0.0"
		},
		"paths": {
			"/players/ban": {
				"post": {
					"operationId": "player.ban",
					"summary": "Ban Player",
					"tags": ["player", "moderation"],
					"description": "Ban a player from the game",
					"x-category": "player",
					"x-risk": "high",
					"x-permission": "player.ban.invoke",
					"requestBody": {
						"required": true,
						"content": {
							"application/json": {
								"schema": {
									"type": "object",
									"properties": {
										"playerId": {"type": "string"},
										"reason": {"type": "string"}
									}
								}
							}
						}
					},
					"responses": {
						"200": {
							"description": "Success",
							"content": {
								"application/json": {
									"schema": {
										"type": "object",
										"properties": {
											"success": {"type": "boolean"}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}`)

	client := newMockClient()
	registry := NewRegistryWithLogger(client, &NoOpLogger{})

	// Handler function
	handlerFunc := func(operationID string) Handler {
		return func(ctx context.Context, input []byte) ([]byte, error) {
			return []byte(`{"success":true}`), nil
		}
	}

	options := &ImportOptions{
		ContinueOnError:  false,
		DefaultTimeoutMs: 30000,
	}

	err := registry.RegisterFromOpenAPI(spec, options, handlerFunc)
	if err != nil {
		t.Fatalf("RegisterFromOpenAPI failed: %v", err)
	}

	// Verify the function was registered
	metadata, exists := registry.GetMetadata("player.ban")
	if !exists {
		t.Fatal("Function 'player.ban' was not registered")
	}

	if metadata.Name != "Ban Player" {
		t.Errorf("Expected name 'Ban Player', got '%s'", metadata.Name)
	}

	if metadata.Category != "player" {
		t.Errorf("Expected category 'player', got '%s'", metadata.Category)
	}

	if metadata.Risk.Level != RiskHigh {
		t.Errorf("Expected risk level RiskHigh, got %v", metadata.Risk.Level)
	}
}

func TestRegistry_RegisterFromOpenAPIWithHandlers(t *testing.T) {
	spec := []byte(`{
		"openapi": "3.0.3",
		"info": {
			"title": "Test API",
			"version": "1.0.0"
		},
		"paths": {
			"/test": {
				"get": {
					"operationId": "test.function",
					"summary": "Test Function",
					"responses": {
						"200": {
							"description": "Success"
						}
					}
				}
			}
		}
	}`)

	client := newMockClient()
	registry := NewRegistryWithLogger(client, &NoOpLogger{})

	handlers := map[string]Handler{
		"test.function": func(ctx context.Context, input []byte) ([]byte, error) {
			return []byte(`{}`), nil
		},
	}

	err := registry.RegisterFromOpenAPIWithHandlers(spec, nil, handlers)
	if err != nil {
		t.Fatalf("RegisterFromOpenAPIWithHandlers failed: %v", err)
	}

	_, exists := registry.GetMetadata("test.function")
	if !exists {
		t.Fatal("Function 'test.function' was not registered")
	}
}

func TestRegistry_RegisterFromOpenAPI_WithPrefix(t *testing.T) {
	spec := []byte(`{
		"openapi": "3.0.3",
		"info": {
			"title": "Test API",
			"version": "1.0.0"
		},
		"paths": {
			"/test": {
				"get": {
					"operationId": "test.function",
					"tags": ["tag1", "tag2"],
					"x-category": "mycategory",
					"responses": {
						"200": {
							"description": "Success"
						}
					}
				}
			}
		}
	}`)

	client := newMockClient()
	registry := NewRegistryWithLogger(client, &NoOpLogger{})

	options := &ImportOptions{
		CategoryPrefix: "prefix",
		TagPrefix:      "pre:",
	}

	handlerFunc := func(operationID string) Handler {
		return func(ctx context.Context, input []byte) ([]byte, error) {
			return []byte(`{}`), nil
		}
	}

	err := registry.RegisterFromOpenAPI(spec, options, handlerFunc)
	if err != nil {
		t.Fatalf("RegisterFromOpenAPI failed: %v", err)
	}

	metadata, _ := registry.GetMetadata("test.function")

	if metadata.Category != "prefix.mycategory" {
		t.Errorf("Expected category 'prefix.mycategory', got '%s'", metadata.Category)
	}

	if len(metadata.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(metadata.Tags))
	}

	if len(metadata.Tags) > 0 && metadata.Tags[0] != "pre:tag1" {
		t.Errorf("Expected tag 'pre:tag1', got '%s'", metadata.Tags[0])
	}
}

func TestParseRiskLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected RiskLevel
	}{
		{"low", RiskLow},
		{"safe", RiskLow},
		{"medium", RiskMedium},
		{"moderate", RiskMedium},
		{"high", RiskHigh},
		{"danger", RiskDanger},
		{"critical", RiskDanger},
		{"unknown", RiskMedium},
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

func TestToTitleCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"player_ban", "Player Ban"},
		{"get_player_info", "Get Player Info"},
		{"", ""},
		{"single", "Single"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toTitleCase(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestDeriveOperationID(t *testing.T) {
	op := &openapi3.Operation{OperationID: "my.function"}

	// Test with operationID set
	result := deriveOperationID(op, "/some/path")
	if result != "my.function" {
		t.Errorf("Expected 'my.function', got '%s'", result)
	}

	// Test with path only
	op2 := &openapi3.Operation{}
	result2 := deriveOperationID(op2, "/api/players/{id}")
	if result2 != "api.players.{id}" {
		t.Errorf("Expected 'api.players.{id}', got '%s'", result2)
	}
}
