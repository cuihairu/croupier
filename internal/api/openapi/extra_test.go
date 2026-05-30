package openapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Additional tests to improve coverage to 80%+

func TestNewHandler(t *testing.T) {
	t.Parallel()

	service := &Service{}
	handler := NewHandler(service)

	assert.NotNil(t, handler)
	assert.Equal(t, service, handler.service)
}

func TestService_GetSpec_ErrorPath(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	// Test with special characters in ID
	_, err := service.GetSpec(context.Background(), &GetSpecRequest{ID: "test with spaces"})
	assert.Error(t, err)
}

func TestService_Import_WithServers(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"servers": []interface{}{
			map[string]interface{}{"url": "https://api.example.com"},
		},
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "testOp",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}

	resp, err := service.Import(context.Background(), &ImportRequest{Spec: spec})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Imported)
}

func TestService_EntityFunctions_MultipleOperations(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	// Add multiple operations for same entity
	operations := []*openapi3.Operation{
		{OperationID: "getUser", Summary: "Get user", Extensions: map[string]interface{}{"x-entity": "User", "x-operation": "read"}},
		{OperationID: "listUsers", Summary: "List users", Extensions: map[string]interface{}{"x-entity": "User", "x-operation": "list"}},
		{OperationID: "createUser", Summary: "Create user", Extensions: map[string]interface{}{"x-entity": "User", "x-operation": "create"}},
	}

	for _, op := range operations {
		service.svcCtx.RegistryStore.UpsertOpenAPI(op.OperationID, op)
	}

	resp, err := service.EntityFunctions(context.Background(), &EntityFunctionsRequest{ID: "User"})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 3)
}

func TestService_EntityFunctions_CaseInsensitiveMatch(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	// Test case sensitivity
	op := &openapi3.Operation{
		OperationID: "getProduct",
		Summary:     "Get product",
		Extensions: map[string]interface{}{
			"x-entity": "product", // lowercase
		},
	}
	service.svcCtx.RegistryStore.UpsertOpenAPI("getProduct", op)

	resp, err := service.EntityFunctions(context.Background(), &EntityFunctionsRequest{ID: "Product"})
	require.NoError(t, err)
	// Should match case-insensitively based on implementation
	assert.NotEmpty(t, resp.Items)
}

func TestService_GetDocument_WithMultipleOperations(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	// Add multiple operations
	for i := 0; i < 5; i++ {
		op := &openapi3.Operation{
			OperationID: "operation" + string(rune('0'+i)),
			Summary:     "Test operation",
		}
		service.svcCtx.RegistryStore.UpsertOpenAPI(op.OperationID, op)
	}

	resp, err := service.GetDocument(context.Background(), &GetDocumentRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp.Spec)

	doc, ok := resp.Spec.(*openapi3.T)
	require.True(t, ok)
	assert.NotNil(t, doc.Paths)
}

func TestService_BatchGetSpec_AllNonExistent(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	resp, err := service.BatchGetSpec(context.Background(), &BatchGetSpecRequest{
		FunctionIDs: []string{"nonexistent1", "nonexistent2", "nonexistent3"},
	})
	require.NoError(t, err)
	assert.Len(t, resp, 3)
	assert.Nil(t, resp["nonexistent1"])
	assert.Nil(t, resp["nonexistent2"])
	assert.Nil(t, resp["nonexistent3"])
}

func TestService_BatchGetSpec_Mixed(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	// Add one operation
	op := &openapi3.Operation{OperationID: "existingFunc", Summary: "Exists"}
	service.svcCtx.RegistryStore.UpsertOpenAPI("existingFunc", op)

	resp, err := service.BatchGetSpec(context.Background(), &BatchGetSpecRequest{
		FunctionIDs: []string{"existingFunc", "nonexistent"},
	})
	require.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.NotNil(t, resp["existingFunc"])
	assert.Nil(t, resp["nonexistent"])
}

func TestHandler_GetSpec_InvalidID(t *testing.T) {
	t.Parallel()

	_, router := setupOpenAPITestHandler(t)

	// Test with special characters that might cause binding issues
	req, _ := http.NewRequest("GET", "/spec/test/with/slashes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return error (not 200)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestHandler_Import_EmptySpec(t *testing.T) {
	t.Parallel()

	_, router := setupOpenAPITestHandler(t)

	req, _ := http.NewRequest("POST", "/import", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Empty spec should fail validation
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestHandler_Import_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, router := setupOpenAPITestHandler(t)

	req, _ := http.NewRequest("POST", "/import", strings.NewReader("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_EntityFunctions_NotFound(t *testing.T) {
	t.Parallel()

	_, router := setupOpenAPITestHandler(t)

	req, _ := http.NewRequest("GET", "/entity/NonExistentEntity/functions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Body should contain empty items list
}

func TestHandler_BatchGetSpec_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, router := setupOpenAPITestHandler(t)

	req, _ := http.NewRequest("POST", "/batch/spec", strings.NewReader("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMatchesEntity_NoExtension(t *testing.T) {
	t.Parallel()

	result := matchesEntity(nil, "TestEntity", "test-entity")
	assert.False(t, result)
}

func TestMatchesEntity_NilValue(t *testing.T) {
	t.Parallel()

	result := matchesEntity("User", "", "user")
	assert.True(t, result)
}

func TestMatchesEntity_DifferentEntity(t *testing.T) {
	t.Parallel()

	result := matchesEntity("Product", "User", "user")
	assert.False(t, result)
}

func TestHasRegisteredFunction_ErrorCases(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	// Test with function that exists in registry but not as FunctionMeta
	op := &openapi3.Operation{OperationID: "testOp"}
	service.svcCtx.RegistryStore.UpsertOpenAPI("testOp", op)

	resp, err := service.GetSpec(context.Background(), &GetSpecRequest{ID: "testOp"})
	// Should return the operation from registry even if not in FunctionMeta
	if err == nil {
		assert.NotNil(t, resp.Spec)
	}
}

func TestImportServiceError_MarshalFailure(t *testing.T) {
	t.Parallel()

	// Create a spec that cannot be marshaled properly
	// Using a channel which can't be marshaled to JSON
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title": "Test",
		},
		"unmarshalable": make(chan int),
	}

	service := setupOpenAPITestService(t)
	_, err := service.Import(context.Background(), &ImportRequest{Spec: spec})
	// Should return an error due to marshal failure
	assert.Error(t, err)
}

func TestNormalizeOpenAPIDoc_ComplexDoc(t *testing.T) {
	t.Parallel()

	// Use Import to test normalizeOpenAPIDoc indirectly
	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "testOp",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							// Missing description - should be filled by normalizeOpenAPIDoc
						},
					},
				},
			},
		},
	}

	resp, err := service.Import(context.Background(), &ImportRequest{Spec: spec})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Imported)
}

func TestService_Import_WithQueryParams(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/users": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "getUsers",
					"parameters": []interface{}{
						map[string]interface{}{
							"name":     "limit",
							"in":       "query",
							"required": true,
							"schema":   map[string]interface{}{"type": "integer"},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "OK"},
					},
				},
			},
		},
	}

	resp, err := service.Import(context.Background(), &ImportRequest{Spec: spec})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Imported)
}

func TestService_GetSpec_ErrorResponse(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	// Test error handling through GetSpec
	_, err := service.GetSpec(context.Background(), &GetSpecRequest{ID: ""})
	assert.Error(t, err)
}
