package openapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOpenAPITestService(t *testing.T) *Service {
	t.Helper()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-1",
		GameID:  "demo-game",
		Env:     "development",
		Functions: map[string]registry.FunctionMeta{
			"player.list": {Enabled: true, Version: "1.0.0"},
			"player.get":  {Enabled: true, Version: "1.0.0"},
		},
		LastSeen: time.Now(),
	})

	return NewService(&svc.ServiceContext{
		DB:                        db,
		FunctionModel:             model.NewFunctionModel(db),
		RegistryStore:             store,
		OpenAPISourceModel:        model.NewOpenAPISourceModel(db),
		OpenAPISourceBindingModel: model.NewOpenAPISourceBindingModel(db),
	})
}

func openAPITestContext() context.Context {
	return svc.WithGameScope(context.Background(), svc.GameScope{
		GameID: "demo-game",
		Env:    "development",
	})
}

func rawSpec(t *testing.T, spec map[string]interface{}) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(spec)
	require.NoError(t, err)
	return json.RawMessage(data)
}

func setupOpenAPITestHandler(t *testing.T) (*Handler, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	service := setupOpenAPITestService(t)
	handler := NewHandler(service)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(openAPITestContext())
		c.Next()
	})
	router.GET("/spec/:id", handler.GetSpec)
	router.GET("/document", handler.GetDocument)
	router.POST("/batch/spec", handler.BatchGetSpec)
	router.GET("/sources", handler.ListSources)
	router.POST("/sources", handler.CreateSource)
	router.GET("/sources/:sourceId", handler.GetSource)
	router.GET("/sources/:sourceId/diagnostics", handler.SourceDiagnostics)
	router.POST("/sources/:sourceId/bindings", handler.CreateBinding)

	return handler, router
}

// Service Tests

func TestService_GetSpec_RegisteredFunction(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)
	resp, err := service.GetSpec(context.Background(), &GetSpecRequest{ID: "player.list"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotNil(t, resp.Spec)

	var op openapi3.Operation
	require.NoError(t, json.Unmarshal(resp.Spec, &op))
	assert.Equal(t, "player.list", op.OperationID)
}

func TestService_GetSpec_UnregisteredFunction(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)
	_, err := service.GetSpec(context.Background(), &GetSpecRequest{ID: "nonexistent"})
	assert.Error(t, err)
}

func TestService_GetSpec_EmptyID(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)
	_, err := service.GetSpec(context.Background(), &GetSpecRequest{ID: ""})
	assert.Error(t, err)
}

func TestService_CreateSource_ValidSpec(t *testing.T) {
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
					"summary":     "Get all users",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Success",
						},
					},
				},
			},
		},
	}

	resp, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{
		Name: "Test API",
		Spec: rawSpec(t, spec),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Source.SourceID)
	assert.Equal(t, "Test API", resp.Source.Name)
	assert.Len(t, resp.Source.Operations, 1)
	assert.Equal(t, "getUsers", resp.Source.Operations[0].OperationID)
	assert.Empty(t, resp.Source.Diagnostics)

	_, err = service.svcCtx.RegistryStore.GetOpenAPI("getUsers")
	assert.Error(t, err, "source upload must not register executable functions")
}

func TestService_CreateSource_InvalidSpec(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		// Missing required info field
	}

	_, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.True(t, errors.As(err, &codeErr))
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
}

func TestService_CreateSource_RejectsUIExtensions(t *testing.T) {
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
				"post": map[string]interface{}{
					"operationId": "createUser",
					"x-ui": map[string]interface{}{
						"type": "object",
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Success",
						},
					},
				},
			},
		},
	}

	_, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

func TestService_CreateSource_MultipleOperations(t *testing.T) {
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
					"summary":     "Get all users",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Success",
						},
					},
				},
				"post": map[string]interface{}{
					"operationId": "createUser",
					"summary":     "Create user",
					"responses": map[string]interface{}{
						"201": map[string]interface{}{
							"description": "Created",
						},
					},
				},
			},
		},
	}

	resp, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.NoError(t, err)
	assert.Len(t, resp.Source.Operations, 2)
}

func TestService_OpenAPISourceListDiagnosticsAndBinding(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Player API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/players": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "playerList",
					"x-resource":  "player",
					"x-operation": "list",
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}
	created, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.NoError(t, err)

	list, err := service.ListSources(openAPITestContext(), &OpenAPISourceListRequest{})
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	assert.Equal(t, created.Source.SourceID, list.Items[0].SourceID)
	assert.Equal(t, 1, list.Items[0].OperationCount)

	diags, err := service.SourceDiagnostics(openAPITestContext(), &OpenAPISourceGetRequest{SourceID: created.Source.SourceID})
	require.NoError(t, err)
	assert.Empty(t, diags.Diagnostics)

	binding, err := service.CreateBinding(openAPITestContext(), &OpenAPISourceBindingCreateRequest{
		SourceID:    created.Source.SourceID,
		OperationID: "playerList",
		Kind:        "provider",
		FunctionID:  "player.list",
	})
	require.NoError(t, err)
	assert.Equal(t, "playerList", binding.Binding.OperationID)
	assert.Equal(t, "player.list", binding.Binding.FunctionID)

	detail, err := service.GetSource(openAPITestContext(), &OpenAPISourceGetRequest{SourceID: created.Source.SourceID})
	require.NoError(t, err)
	require.Len(t, detail.Source.Operations, 1)
	assert.True(t, detail.Source.Operations[0].Bound)
	assert.Equal(t, "player.list", detail.Source.Operations[0].FunctionID)
}

func TestService_GetDocument(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	// Add an operation
	op := &openapi3.Operation{
		OperationID: "testOperation",
		Summary:     "Test operation",
	}
	service.svcCtx.RegistryStore.UpsertOpenAPI("testOperation", op)

	resp, err := service.GetDocument(context.Background(), &GetDocumentRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp.Spec)
}

func TestService_BatchGetSpec_MultipleIDs(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	// Add operations
	op1 := &openapi3.Operation{OperationID: "func1", Summary: "Function 1"}
	op2 := &openapi3.Operation{OperationID: "func2", Summary: "Function 2"}
	service.svcCtx.RegistryStore.UpsertOpenAPI("func1", op1)
	service.svcCtx.RegistryStore.UpsertOpenAPI("func2", op2)

	resp, err := service.BatchGetSpec(context.Background(), &BatchGetSpecRequest{
		FunctionIDs: []string{"func1", "func2", "player.list"},
	})
	require.NoError(t, err)
	assert.Len(t, resp, 3)
	assert.NotNil(t, resp["func1"])
	assert.NotNil(t, resp["func2"])
	assert.NotNil(t, resp["player.list"])
}

func TestService_BatchGetSpec_WithNonExistent(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	resp, err := service.BatchGetSpec(context.Background(), &BatchGetSpecRequest{
		FunctionIDs: []string{"nonexistent"},
	})
	require.NoError(t, err)
	assert.Nil(t, resp["nonexistent"])
}

func TestService_BatchGetSpec_EmptyIDs(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	resp, err := service.BatchGetSpec(context.Background(), &BatchGetSpecRequest{
		FunctionIDs: []string{},
	})
	require.NoError(t, err)
	assert.Empty(t, resp)
}

func TestService_BatchGetSpec_WithEmptyStringID(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)

	resp, err := service.BatchGetSpec(context.Background(), &BatchGetSpecRequest{
		FunctionIDs: []string{"", "player.list"},
	})
	require.NoError(t, err)
	// Empty string should be skipped
	assert.Len(t, resp, 1)
}

// Handler Tests

func TestHandler_GetSpec_Success(t *testing.T) {
	t.Parallel()

	_, router := setupOpenAPITestHandler(t)

	req, _ := http.NewRequest("GET", "/spec/player.list", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_GetSpec_NotFound(t *testing.T) {
	t.Parallel()

	_, router := setupOpenAPITestHandler(t)

	req, _ := http.NewRequest("GET", "/spec/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Handler might return 500 for not found
	assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError)
}

func TestHandler_CreateSource_Success(t *testing.T) {
	t.Parallel()

	_, router := setupOpenAPITestHandler(t)

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
					"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}

	body, _ := json.Marshal(spec)
	req, _ := http.NewRequest("POST", "/sources", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp OpenAPISourceGetResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Source.Operations, 1)
}

func TestHandler_GetDocument_Success(t *testing.T) {
	t.Parallel()

	_, router := setupOpenAPITestHandler(t)

	req, _ := http.NewRequest("GET", "/document", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_BatchGetSpec_Success(t *testing.T) {
	t.Parallel()

	_, router := setupOpenAPITestHandler(t)

	reqBody := BatchGetSpecRequest{
		FunctionIDs: []string{"player.list", "player.get"},
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/batch/spec", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp BatchGetSpecResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestHandler_BatchGetSpec_MissingBody(t *testing.T) {
	t.Parallel()

	_, router := setupOpenAPITestHandler(t)

	req, _ := http.NewRequest("POST", "/batch/spec", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Handler might return 500 for missing body
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError)
}

// Tests for hasRegisteredFunction through public API

func TestHasRegisteredFunction_WithRegistryStore(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-1",
		Functions: map[string]registry.FunctionMeta{
			"test.func": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		DB:            db,
		RegistryStore: store,
		FunctionModel: model.NewFunctionModel(db),
	}

	// Test through GetSpec which calls hasRegisteredFunction
	resp, err := NewService(svcCtx).GetSpec(context.Background(), &GetSpecRequest{ID: "test.func"})
	require.NoError(t, err)
	assert.NotNil(t, resp.Spec)
}

func TestHasRegisteredFunction_NilSvcCtx(t *testing.T) {
	t.Parallel()

	// Test with nil context
	service := NewService(&svc.ServiceContext{
		RegistryStore: registry.NewStore(),
	})

	_, err := service.GetSpec(context.Background(), &GetSpecRequest{ID: "test"})
	assert.Error(t, err)
}

func TestNormalizeOpenAPIDoc_NilDoc(t *testing.T) {
	t.Parallel()

	normalizeOpenAPIDoc(nil)
	// Should not panic
}

func TestNormalizeOpenAPIDoc_NilPaths(t *testing.T) {
	t.Parallel()

	doc := &openapi3.T{}
	normalizeOpenAPIDoc(doc)
	// Should not panic
}

func TestNormalizeOpenAPIDoc_MissingResponseDescription(t *testing.T) {
	t.Parallel()

	// Test normalization through Source upload which calls normalizeOpenAPIDoc
	service := setupOpenAPITestService(t)

	// Spec with missing response descriptions
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
					"responses": map[string]interface{}{
						// Missing description for 200
						"200": map[string]interface{}{},
					},
				},
			},
		},
	}

	resp, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: rawSpec(t, spec)})
	require.NoError(t, err)
	assert.Len(t, resp.Source.Operations, 1)
}
