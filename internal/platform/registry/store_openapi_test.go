package registry

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestUpsertOpenAPI(t *testing.T) {
	store := NewStore()

	// Create a test operation
	op := &openapi3.Operation{
		Summary:     "Test Function",
		Description: "A test function",
		RequestBody: &openapi3.RequestBodyRef{
			Value: &openapi3.RequestBody{
				Content: map[string]*openapi3.MediaType{
					"application/json": {
						Schema: &openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: &openapi3.Types{"object"},
							},
						},
					},
				},
			},
		},
		Responses: openapi3.NewResponses(
			openapi3.WithName("200", openapi3.NewResponse().WithDescription("Success")),
		),
	}

	// Test upsert
	err := store.UpsertOpenAPI("test.function", op)
	if err != nil {
		t.Fatalf("UpsertOpenAPI failed: %v", err)
	}

	// Test get
	retrieved, err := store.GetOpenAPI("test.function")
	if err != nil {
		t.Fatalf("GetOpenAPI failed: %v", err)
	}

	if retrieved.Summary != op.Summary {
		t.Errorf("Expected summary %s, got %s", op.Summary, retrieved.Summary)
	}
}

func TestGetOpenAPINotFound(t *testing.T) {
	store := NewStore()

	_, err := store.GetOpenAPI("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent function, got nil")
	}
}

func TestListOpenAPIOperations(t *testing.T) {
	store := NewStore()

	// Insert multiple operations
	op1 := &openapi3.Operation{
		Summary: "Function 1",
		Responses: openapi3.NewResponses(
			openapi3.WithName("200", openapi3.NewResponse()),
		),
	}
	op2 := &openapi3.Operation{
		Summary: "Function 2",
		Responses: openapi3.NewResponses(
			openapi3.WithName("200", openapi3.NewResponse()),
		),
	}

	store.UpsertOpenAPI("func1", op1)
	store.UpsertOpenAPI("func2", op2)

	// List all
	operations := store.ListOpenAPIOperations()

	if len(operations) != 2 {
		t.Errorf("Expected 2 operations, got %d", len(operations))
	}

	if _, exists := operations["func1"]; !exists {
		t.Error("func1 not found in list")
	}

	if _, exists := operations["func2"]; !exists {
		t.Error("func2 not found in list")
	}
}

func TestDeleteOpenAPI(t *testing.T) {
	store := NewStore()

	op := &openapi3.Operation{
		Summary: "To be deleted",
		Responses: openapi3.NewResponses(
			openapi3.WithName("200", openapi3.NewResponse()),
		),
	}

	store.UpsertOpenAPI("delete.me", op)

	// Delete
	err := store.DeleteOpenAPI("delete.me")
	if err != nil {
		t.Fatalf("DeleteOpenAPI failed: %v", err)
	}

	// Verify deleted
	_, err = store.GetOpenAPI("delete.me")
	if err == nil {
		t.Error("Expected error after deletion, got nil")
	}
}

func TestUpsertOpenAPIProvider(t *testing.T) {
	store := NewStore()

	caps := OpenAPIProviderCaps{
		ID:      "test-provider",
		Version: "1.0.0",
		Lang:    "go",
		SDK:     "croupier-go@v1",
		OpenAPIDoc: []byte(`{
			"openapi": "3.0.3",
			"info": {"title": "Test Provider", "version": "1.0.0"},
			"paths": {}
		}`),
	}

	err := store.UpsertOpenAPIProvider(caps)
	if err != nil {
		t.Fatalf("UpsertOpenAPIProvider failed: %v", err)
	}

	// Retrieve
	retrieved, err := store.GetOpenAPIProvider("test-provider")
	if err != nil {
		t.Fatalf("GetOpenAPIProvider failed: %v", err)
	}

	if retrieved.ID != caps.ID {
		t.Errorf("Expected ID %s, got %s", caps.ID, retrieved.ID)
	}

	if retrieved.Version != caps.Version {
		t.Errorf("Expected version %s, got %s", caps.Version, retrieved.Version)
	}
}

func TestBuildOpenAPISpec(t *testing.T) {
	store := NewStore()

	// Add some operations
	op1 := &openapi3.Operation{
		Summary: "Function 1",
		Responses: openapi3.NewResponses(
			openapi3.WithName("200", openapi3.NewResponse()),
		),
	}
	op2 := &openapi3.Operation{
		Summary: "Function 2",
		Responses: openapi3.NewResponses(
			openapi3.WithName("200", openapi3.NewResponse()),
		),
	}

	store.UpsertOpenAPI("func1", op1)
	store.UpsertOpenAPI("func2", op2)

	// Build spec
	doc, err := store.BuildOpenAPISpec()
	if err != nil {
		t.Fatalf("BuildOpenAPISpec failed: %v", err)
	}

	if doc.OpenAPI != "3.0.3" {
		t.Errorf("Expected OpenAPI version 3.0.3, got %s", doc.OpenAPI)
	}

	paths := doc.Paths.Map()
	if len(paths) != 2 {
		t.Errorf("Expected 2 paths, got %d", len(paths))
	}

	// Check path format
	if _, exists := paths["/functions/func1"]; !exists {
		t.Error("Expected path /functions/func1 not found")
	}

	if _, exists := paths["/functions/func2"]; !exists {
		t.Error("Expected path /functions/func2 not found")
	}
}

func TestOpenAPIValidation(t *testing.T) {
	store := NewStore()

	// Test empty function ID
	err := store.UpsertOpenAPI("", &openapi3.Operation{})
	if err == nil {
		t.Error("Expected error for empty function ID, got nil")
	}

	// Test nil operation
	err = store.UpsertOpenAPI("test.func", nil)
	if err == nil {
		t.Error("Expected error for nil operation, got nil")
	}

	// Test empty provider ID
	caps := OpenAPIProviderCaps{Version: "1.0.0"}
	err = store.UpsertOpenAPIProvider(caps)
	if err == nil {
		t.Error("Expected error for empty provider ID, got nil")
	}
}
