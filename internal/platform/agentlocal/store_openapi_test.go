package agentlocal

import (
	"encoding/json"
	"testing"

	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// TestFunctionMetaOpenAPIFields tests that FunctionMeta properly stores OpenAPI fields
func TestFunctionMetaOpenAPIFields(t *testing.T) {
	store := NewLocalStore()

	// Create a function descriptor with OpenAPI 3.0.3 fields
	funcs := []*sdkv1.LocalFunctionDescriptor{
		{
			Id:           "player.ban",
			Version:      "1.0.0",
			Tags:         []string{"player", "moderation"},
			Summary:      "Ban a player",
			Description:  "Permanently ban a player from the game",
			OperationId:  "banPlayer",
			Deprecated:   false,
			InputSchema:  `{"type":"object","properties":{"playerId":{"type":"string"},"reason":{"type":"string"}}}`,
			OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
		},
	}

	// Register the function
	store.Register("service-1", "service-1", "localhost:18780", "1.0.0", funcs, nil)

	// Retrieve metadata
	meta := store.FunctionMetadata()

	if len(meta) != 1 {
		t.Fatalf("expected 1 function metadata, got %d", len(meta))
	}

	functionMeta := meta["player.ban"]
	if functionMeta == nil {
		t.Fatal("function metadata for 'player.ban' not found")
	}

	// Verify basic fields
	if functionMeta.ID != "player.ban" {
		t.Errorf("expected ID 'player.ban', got '%s'", functionMeta.ID)
	}

	if functionMeta.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", functionMeta.Version)
	}

	if functionMeta.Summary != "Ban a player" {
		t.Errorf("expected summary 'Ban a player', got '%s'", functionMeta.Summary)
	}

	// Verify OpenAPI schema fields
	if functionMeta.InputSchema == "" {
		t.Error("input schema should not be empty")
	}

	if functionMeta.OutputSchema == "" {
		t.Error("output schema should not be empty")
	}

	// Verify tags
	if len(functionMeta.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(functionMeta.Tags))
	}

	if functionMeta.OpenAPIOperation == "" {
		t.Fatal("OpenAPIOperation should not be empty")
	}
	var operation map[string]interface{}
	if err := json.Unmarshal([]byte(functionMeta.OpenAPIOperation), &operation); err != nil {
		t.Fatalf("OpenAPIOperation should be valid JSON: %v", err)
	}
	if operation["operationId"] != "banPlayer" {
		t.Fatalf("unexpected operationId in OpenAPIOperation: %v", operation["operationId"])
	}
}

func TestFunctionMetaOpenAPIOperationInvalidSchemaFallback(t *testing.T) {
	store := NewLocalStore()
	funcs := []*sdkv1.LocalFunctionDescriptor{
		{
			Id:          "bad.schema.func",
			Version:     "1.0.0",
			Summary:     "Bad Schema",
			InputSchema: `{"type":"object","properties":{"x":{"type":"string"}`,
		},
	}
	store.Register("service-1", "service-1", "localhost:18780", "1.0.0", funcs, nil)
	meta := store.FunctionMetadata()
	if meta["bad.schema.func"] == nil {
		t.Fatal("function metadata not found")
	}
	if meta["bad.schema.func"].OpenAPIOperation != "" {
		t.Fatal("OpenAPIOperation should remain empty when schema is invalid")
	}
}

// TestFunctionMetadataImmutability tests that FunctionMetadata returns immutable copies
func TestFunctionMetadataImmutability(t *testing.T) {
	store := NewLocalStore()

	funcs := []*sdkv1.LocalFunctionDescriptor{
		{
			Id:          "test.func",
			Version:     "1.0.0",
			Tags:        []string{"tag1"},
			InputSchema: `{"type":"object"}`,
		},
	}

	store.Register("service-1", "service-1", "localhost:18780", "1.0.0", funcs, nil)

	// Get metadata
	meta1 := store.FunctionMetadata()

	// Modify the returned metadata
	meta1["test.func"].Tags = append(meta1["test.func"].Tags, "tag2")
	meta1["test.func"].InputSchema = `{"modified":true}`

	// Get metadata again
	meta2 := store.FunctionMetadata()

	// Verify that the original metadata was not modified
	if len(meta2["test.func"].Tags) != 1 {
		t.Errorf("expected 1 tag in original metadata, got %d", len(meta2["test.func"].Tags))
	}

	if meta2["test.func"].InputSchema != `{"type":"object"}` {
		t.Errorf("original input schema was modified: %s", meta2["test.func"].InputSchema)
	}
}

// TestMultipleFunctionsMetadata tests metadata storage for multiple functions
func TestMultipleFunctionsMetadata(t *testing.T) {
	store := NewLocalStore()

	funcs := []*sdkv1.LocalFunctionDescriptor{
		{
			Id:           "func1",
			Version:      "1.0.0",
			Summary:      "Function 1",
			InputSchema:  `{"type":"string"}`,
			OutputSchema: `{"type":"number"}`,
		},
		{
			Id:           "func2",
			Version:      "2.0.0",
			Summary:      "Function 2",
			InputSchema:  `{"type":"array"}`,
			OutputSchema: `{"type":"boolean"}`,
		},
	}

	store.Register("service-1", "service-1", "localhost:18780", "1.0.0", funcs, nil)

	meta := store.FunctionMetadata()

	if len(meta) != 2 {
		t.Fatalf("expected 2 function metadata, got %d", len(meta))
	}

	// Verify each function
	testCases := []struct {
		id              string
		expectedSummary string
		expectedInput   string
		expectedOutput  string
	}{
		{"func1", "Function 1", `{"type":"string"}`, `{"type":"number"}`},
		{"func2", "Function 2", `{"type":"array"}`, `{"type":"boolean"}`},
	}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			fnMeta := meta[tc.id]
			if fnMeta == nil {
				t.Fatalf("function %s not found in metadata", tc.id)
			}

			if fnMeta.Summary != tc.expectedSummary {
				t.Errorf("expected summary '%s', got '%s'", tc.expectedSummary, fnMeta.Summary)
			}

			if fnMeta.InputSchema != tc.expectedInput {
				t.Errorf("expected input schema '%s', got '%s'", tc.expectedInput, fnMeta.InputSchema)
			}

			if fnMeta.OutputSchema != tc.expectedOutput {
				t.Errorf("expected output schema '%s', got '%s'", tc.expectedOutput, fnMeta.OutputSchema)
			}
		})
	}
}

// TestFunctionMetadataUpdate tests that metadata is updated on re-registration
func TestFunctionMetadataUpdate(t *testing.T) {
	store := NewLocalStore()

	// Initial registration
	funcs1 := []*sdkv1.LocalFunctionDescriptor{
		{
			Id:          "test.func",
			Version:     "1.0.0",
			Summary:     "Old summary",
			InputSchema: `{"version":1}`,
		},
	}

	store.Register("service-1", "service-1", "localhost:18780", "1.0.0", funcs1, nil)

	// Update with new version
	funcs2 := []*sdkv1.LocalFunctionDescriptor{
		{
			Id:          "test.func",
			Version:     "2.0.0",
			Summary:     "New summary",
			InputSchema: `{"version":2}`,
		},
	}

	store.Register("service-1", "service-1", "localhost:18780", "2.0.0", funcs2, nil)

	// Verify metadata was updated
	meta := store.FunctionMetadata()

	fnMeta := meta["test.func"]
	if fnMeta == nil {
		t.Fatal("function metadata not found")
	}

	if fnMeta.Version != "2.0.0" {
		t.Errorf("expected version '2.0.0', got '%s'", fnMeta.Version)
	}

	if fnMeta.Summary != "New summary" {
		t.Errorf("expected summary 'New summary', got '%s'", fnMeta.Summary)
	}

	if fnMeta.InputSchema != `{"version":2}` {
		t.Errorf("expected input schema '{\"version\":2}', got '%s'", fnMeta.InputSchema)
	}
}
