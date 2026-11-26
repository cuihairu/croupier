package platform

import (
	"bytes"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	dispatch "github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Split from service_test.go; test bodies moved verbatim.

// TestHandler_ListPlatforms_ServiceContextWithExtensions tests handler with Extensions in context
func TestHandler_ListPlatforms_ServiceContextWithExtensions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test_platform.method1": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}

	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms", handler.ListPlatforms)

	req := httptest.NewRequest("GET", "/platforms", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "test_platform")
}

// TestHandler_ListMethods_WithExtensionsInContext tests ListMethods with Extensions

// TestHandler_ListMethods_WithExtensionsInContext tests ListMethods with Extensions
func TestHandler_ListMethods_WithExtensionsInContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.my_platform.method1": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}

	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms/my_platform/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "method1")
}

// TestCall_WithDispatcherTestsEmptyResultPath tests the path where dispatcher returns empty result

// TestHandler_ListPlatforms_ResponseStructure verifies the handler response structure
func TestHandler_ListPlatforms_ResponseStructure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test_platform.method1": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{RegistryStore: store}
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms", handler.ListPlatforms)

	req := httptest.NewRequest("GET", "/platforms", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()

	// Verify response contains expected fields
	assert.Contains(t, body, "test_platform")
	assert.Contains(t, body, `"source":"extension"`)
}

// TestHandler_ListMethods_ResponseStructure verifies the handler response structure

// TestHandler_ListMethods_ResponseStructure verifies the handler response structure
func TestHandler_ListMethods_ResponseStructure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.my_platform.method1": {Enabled: true},
			"external.my_platform.method2": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{RegistryStore: store}
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms/my_platform/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()

	// Verify response contains expected fields
	assert.Contains(t, body, "method1")
	assert.Contains(t, body, "method2")
	assert.Contains(t, body, `"source":"extension"`)
}

// TestHandler_Call_ResponseStructure verifies the handler call response structure

// TestHandler_Call_ResponseStructure verifies the handler call response structure
func TestHandler_Call_ResponseStructure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test_platform","method":"test_method"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
	body := resp.Body.String()

	assert.Contains(t, body, "not available")
}

// TestHandler_Call_MissingPlatformField tests missing platform in request

// TestHandler_Call_MissingPlatformField tests missing platform in request
func TestHandler_Call_MissingPlatformField2(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	// Missing platform field
	reqBody := `{"method":"test_method"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "platform")
}

// TestHandler_Call_MissingMethodField tests missing method in request

// TestHandler_Call_MissingMethodField tests missing method in request
func TestHandler_Call_MissingMethodField2(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	// Missing method field
	reqBody := `{"platform":"test_platform"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "method")
}

// TestHandler_ListMethods_EmptyPlatformParam tests with empty platform parameter

// TestHandler_ListMethods_EmptyPlatformParam tests with empty platform parameter
func TestHandler_ListMethods_EmptyPlatformParam2(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms//methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "platform")
}

// TestHandler_ListMethods_NonexistentPlatform tests with platform that doesn't exist

// TestHandler_ListMethods_NonexistentPlatform tests with platform that doesn't exist
func TestHandler_ListMethods_NonexistentPlatform2(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms/nonexistent_platform/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNotFound, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "not found")
}

// TestHandler_Call_WithEmptyRequestData tests with empty request data

// TestHandler_Call_WithEmptyRequestData tests with empty request data
func TestHandler_Call_WithEmptyRequestData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test","method":"method","request":""}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "not available")
}

// TestHandler_Call_WithValidRequestData tests with valid request data

// TestHandler_Call_WithValidRequestData tests with valid request data
func TestHandler_Call_WithValidRequestData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test","method":"method","request":"{\"key\":\"value\"}"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "not available")
}

// TestHandler_Call_AllFields tests with all fields present

// TestHandler_Call_AllFields tests with all fields present
func TestHandler_Call_AllFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test_platform","method":"test_method","request":"{\"data\":\"value\"}"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
	assert.Contains(t, resp.Body.String(), "not available")
}

// TestCall_RequestFieldNotEmpty tests request field conversion (line 43-44)

// TestHandler_ListMethods_ResponseFields verifies response fields
func TestHandler_ListMethods_ResponseFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test_platform.method1": {Enabled: true},
			"external.test_platform.method2": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{RegistryStore: store}
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms/test_platform/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()

	// Verify response contains all expected fields
	assert.Contains(t, body, `"methods"`)
	assert.Contains(t, body, `"source":"extension"`)
	assert.Contains(t, body, "method1")
	assert.Contains(t, body, "method2")
}

// TestCall_WithVariousRequestData tests request data conversion

// TestHandler_Call_WithAllFieldsMissing tests with all fields missing
func TestHandler_Call_WithAllFieldsMissing3(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{}` // Empty JSON object
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "platform")
}

// TestHandler_Call_SuccessResponseFields verifies success response fields

// TestHandler_Call_SuccessResponseFields verifies success response fields
func TestHandler_Call_SuccessResponseFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := reg.NewStore()
	dispatcher := dispatch.NewDispatcher(store)

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Dispatcher:    dispatcher,
	}

	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test","method":"method"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "internal_error")
}

// TestHandler_ListPlatforms_EmptyList verifies empty platform list response

// TestHandler_ListPlatforms_EmptyList verifies empty platform list response
func TestHandler_ListPlatforms_EmptyList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Empty registry - no platforms
	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms", handler.ListPlatforms)

	req := httptest.NewRequest("GET", "/platforms", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "{")

	// Should return 200 with success message
}

// TestHandler_ListPlatforms_WithPlatforms verifies platforms in response

// TestHandler_ListPlatforms_WithPlatforms verifies platforms in response
func TestHandler_ListPlatforms_WithPlatforms2(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test_platform.method1": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{RegistryStore: store}
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms", handler.ListPlatforms)

	req := httptest.NewRequest("GET", "/platforms", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()

	// Should return platform info
	assert.Contains(t, body, `"name":"test_platform"`)
	assert.Contains(t, body, `"enabled":true`)
	assert.Contains(t, body, `"methods":["method1"]`)
	assert.Contains(t, body, `"source":"extension"`)
}

// TestDiscoverExternalPlatforms_WithInstallationService exercises Installation path

// TestHandler_List_AliasMethod tests the List alias method
func TestHandler_List_AliasMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}

	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms", handler.List)

	req, _ := http.NewRequest("GET", "/platforms", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

// TestHandler_Methods_AliasMethod tests the Methods alias method

// TestHandler_Methods_AliasMethod tests the Methods alias method
func TestHandler_Methods_AliasMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.test.method1": {Enabled: true},
			"external.test.method2": {Enabled: true},
		},
	})

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: extensioninstallation.NewService(nil, nil, nil),
		},
	}

	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.Methods)

	req, _ := http.NewRequest("GET", "/platforms/test/methods", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

// TestWriteListPlatformsResponse_NilResponse tests the nil response path
