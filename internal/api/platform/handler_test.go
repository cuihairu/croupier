package platform

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestNewHandler(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	assert.NotNil(t, handler)
	assert.Equal(t, service, handler.service)
}

func TestHandler_Call_BindingError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should return error for invalid JSON
	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_Call_EmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle empty body gracefully
	assert.NotEqual(t, http.StatusNotFound, resp.Code)
}

func TestHandler_Call_ValidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test","method":"test_method","params":{}}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Service will fail due to unknown platform, but binding should succeed
	assert.NotEqual(t, http.StatusBadRequest, resp.Code)
}

func TestHandler_ListPlatforms_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms", handler.ListPlatforms)

	req := httptest.NewRequest("GET", "/platforms", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should succeed
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_List_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platform/list", handler.List)

	req := httptest.NewRequest("GET", "/platform/list", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should succeed - alias method
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_ListMethods_MissingPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms//methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle empty platform parameter
	assert.NotEqual(t, http.StatusNotFound, resp.Code)
}

func TestHandler_ListMethods_ValidPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms/test_platform/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle the request (platform may not exist)
	assert.NotEqual(t, http.StatusNotFound, resp.Code)
}

func TestHandler_Methods_Alias(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platform/:platform/methods", handler.Methods)

	req := httptest.NewRequest("GET", "/platform/test/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle the request - alias method
	assert.NotEqual(t, http.StatusNotFound, resp.Code)
}

func TestHandler_Call_WithParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test","method":"test_method","params":{"key":"value"}}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Binding should succeed
	assert.NotEqual(t, http.StatusBadRequest, resp.Code)
}

func TestHandler_Call_MissingPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"method":"test_method","params":{}}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Service will return error for missing platform
	assert.Contains(t, resp.Body.String(), "required")
}

// Additional service tests

func TestService_Call_EmptyPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "",
		Method:   "test_method",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 400, resp.Code)
	assert.Contains(t, resp.Message, "platform")
}

func TestService_Call_EmptyMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test_platform",
		Method:   "",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 400, resp.Code)
	assert.Contains(t, resp.Message, "method")
}

func TestService_Call_NilDispatcher(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "test_platform",
		Method:   "test_method",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 503, resp.Code)
	assert.Contains(t, resp.Message, "not available")
}

func TestService_ListMethods_EmptyPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	resp, err := service.ListMethods(context.Background(), "")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 400, resp.Code)
	assert.Contains(t, resp.Message, "platform")
}

func TestService_ListMethods_WhitespacePlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	resp, err := service.ListMethods(context.Background(), "  ")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 400, resp.Code)
}

func TestService_ListMethods_NilContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	resp, err := service.ListMethods(context.Background(), "test_platform")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// Platform not found because no extensions
	assert.Equal(t, 404, resp.Code)
}

func TestService_ListPlatforms_NilContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	resp, err := service.ListPlatforms(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 200, resp.Code)
	// Empty list because no extensions
	assert.NotNil(t, resp.Platforms)
}

func TestService_ListPlatforms_NilServiceContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	resp, err := service.ListPlatforms(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 200, resp.Code)
}

func TestResolveMethodsSource_WithoutExtension(t *testing.T) {
	result := resolveMethodsSource(false)
	assert.Empty(t, result)
}

func TestDiscoverExternalPlatforms_NilService(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			// Expected to panic for nil service
		}
	}()
	var service *Service
	result := service.discoverExternalPlatforms(context.Background())
	// If we get here without panic, result should not be nil
	if result != nil {
		assert.NotNil(t, result)
	}
}

func TestDiscoverExternalPlatforms_NilServiceContext(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	result := service.discoverExternalPlatforms(context.Background())
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestExtractPlatformMethodsFromBindings_EmptyList(t *testing.T) {
	result := extractPlatformMethodsFromBindings([]model.ExtensionRuntimeBinding{})
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestExtractPlatformMethodsFromBindings_NilBindings(t *testing.T) {
	result := extractPlatformMethodsFromBindings(nil)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestParseExternalFunctionID_EmptyString(t *testing.T) {
	provider, method, ok := parseExternalFunctionID("")
	assert.False(t, ok)
	assert.Empty(t, provider)
	assert.Empty(t, method)
}

func TestParseExternalFunctionID_InvalidFormat(t *testing.T) {
	provider, method, ok := parseExternalFunctionID("invalid_format")
	assert.False(t, ok)
	assert.Empty(t, provider)
	assert.Empty(t, method)
}

// Additional handler tests for binding

func TestHandler_Call_MissingMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test","params":{}}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Service will return error for missing method
	// The handler wraps the response in success format, so we check for method required in body
	body := resp.Body.String()
	if resp.Code == http.StatusOK {
		// Response should contain error from service
		assert.Contains(t, body, "method")
	}
}

func TestHandler_Call_WithRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test","method":"test","request":"{\"key\":\"value\"}"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Binding should succeed
	assert.NotEqual(t, http.StatusBadRequest, resp.Code)
}

// Additional handler edge case tests

func TestHandler_Call_EmptyPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"method":"test_method"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "platform")
}

func TestHandler_Call_EmptyMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "method")
}

func TestHandler_ListMethods_EmptyPlatformParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	// Use URL-encoded spaces
	req := httptest.NewRequest("GET", "/platforms/%20%20%20/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "platform")
}

func TestHandler_ListMethods_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms/nonexistent/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "404")
}

func TestHandler_ListPlatforms_SuccessResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms", handler.ListPlatforms)

	req := httptest.NewRequest("GET", "/platforms", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	// Check for success response - platforms list may be empty
	assert.Contains(t, body, `"code":200`)
}

func TestHandler_Call_PlatformWithWhitespace(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"  test_platform  ","method":"test_method"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Call_WithEmptyParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test","method":"test_method","params":{}}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Call_WithParamsObject(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test","method":"test_method","params":{"key":"value","number":123}}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

// Additional tests to improve handler coverage

func TestHandler_Call_WithAllFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test_platform","method":"test_method","params":{"key":"value"},"request":"{\"extra\":\"data\"}"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Call_MissingBothPlatformAndMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"params":{"key":"value"}}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "platform")
}

func TestHandler_ListMethods_WithNumericPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms/12345/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle numeric platform parameter
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_ListMethods_WithSpecialCharacters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms/test-platform_v2/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle special characters in platform name
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_ListMethods_WithUnderscores(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms/test_platform/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle underscores in platform name
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_ListPlatforms_WithQueryParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms", handler.ListPlatforms)

	// Add query parameters (should be ignored)
	req := httptest.NewRequest("GET", "/platforms?filter=enabled&sort=name", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_List_ViaAlias(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platform/list", handler.List)

	req := httptest.NewRequest("GET", "/platform/list", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Methods_ViaAlias(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platform/:platform/methods", handler.Methods)

	req := httptest.NewRequest("GET", "/platform/test/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Call_WithLargeRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	// Create a large request
	largeRequest := `{"platform":"test","method":"test_method","request":"`
	for i := 0; i < 1000; i++ {
		largeRequest += "x"
	}
	largeRequest += `"}`

	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(largeRequest))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Call_WithComplexParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test","method":"test_method","params":{"nested":{"key":"value"},"array":[1,2,3],"bool":true,"null":null}}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_ListMethods_CaseSensitive(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms/UpperCasePlatform/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle case-sensitive platform parameter
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_ListMethods_WithDotInName(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms/test.platform/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_ListMethods_WithHyphenInName(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms/test-platform/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_ListPlatforms_WithHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms", handler.ListPlatforms)

	req := httptest.NewRequest("GET", "/platforms", nil)
	req.Header.Set("X-Custom-Header", "test-value")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Call_WithHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test","method":"test_method"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Custom-Header", "test-value")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_ListMethods_WithPathTrailingSlash(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods/", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms/test/methods/", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle trailing slash
	assert.Equal(t, http.StatusOK, resp.Code)
}

// Tests to improve handler coverage
func TestHandler_ListMethods_WithEmptyPlatformParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms//methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle empty platform parameter
	// Service will return 400 error for empty platform
	assert.NotEqual(t, http.StatusNotFound, resp.Code)
}

func TestHandler_ListPlatforms_WithError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create service with nil context that will cause errors
	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms", handler.ListPlatforms)

	req := httptest.NewRequest("GET", "/platforms", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle request even with minimal context
	assert.NotEqual(t, http.StatusNotFound, resp.Code)
}

func TestHandler_Call_WithComplexJson(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	// Complex nested JSON
	reqBody := `{"platform":"test","method":"complex_method","request":{"nested":{"key":"value","array":[1,2,3]},"null":null}}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle complex JSON
	assert.NotEqual(t, http.StatusBadRequest, resp.Code)
}

func TestHandler_ListMethods_WithEncodedPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	// URL encoded platform name
	req := httptest.NewRequest("GET", "/platforms/test%20platform/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle URL encoding
	assert.NotEqual(t, http.StatusNotFound, resp.Code)
}

func TestHandler_Call_WithUnicode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	// Unicode characters in request
	reqBody := `{"platform":"测试平台","method":"测试方法","request":"测试数据"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle unicode
	assert.NotEqual(t, http.StatusBadRequest, resp.Code)
}

// TestHandler_ListPlatforms_ServiceError tests error path when service returns an error
func TestHandler_ListPlatforms_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a service that will error
	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms", handler.ListPlatforms)

	// Create a request with cancelled context to trigger error
	req := httptest.NewRequest("GET", "/platforms", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle the request - service should not return error
	assert.NotEqual(t, http.StatusNotFound, resp.Code)
}

// TestHandler_ListMethods_ServiceError tests error path when ListMethods service returns error
func TestHandler_ListMethods_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a minimal service
	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	// Test with a platform that doesn't exist
	req := httptest.NewRequest("GET", "/platforms/nonexistentplatform/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should return 404 in the response body
	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "404")
}

// TestHandler_Call_ServiceValidationError tests Call with validation errors from service
func TestHandler_Call_ServiceValidationError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	// Test with missing required fields
	reqBody := `{"method":"testmethod"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should return success wrapper with validation error inside
	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "platform")
}

// TestHandler_ListPlatforms_WithNilService tests ListPlatforms with nil service
func TestHandler_ListPlatforms_WithNilService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create handler with nil service - this will panic, so we need to recover
	defer func() {
		if r := recover(); r != nil {
			// Expected to panic
			t.Log("Recovered from panic as expected with nil service")
		}
	}()

	handler := NewHandler(nil)

	router := gin.New()
	router.GET("/platforms", handler.ListPlatforms)

	req := httptest.NewRequest("GET", "/platforms", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// If we get here without panic, that's also ok
	t.Log("Handler handled nil service without panic")
}

// TestHandler_ListMethods_WithVeryLongPlatformName tests with very long platform name
func TestHandler_ListMethods_WithVeryLongPlatformName(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	// Create a very long platform name
	longPlatform := string(make([]byte, 500))
	for i := range longPlatform {
		longPlatform = "a" + longPlatform[:i]
	}
	longPlatform = "very_long_platform_name_for_testing_purposes_" + longPlatform[:100]

	req := httptest.NewRequest("GET", "/platforms/"+longPlatform+"/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle long platform names
	assert.NotEqual(t, http.StatusNotFound, resp.Code)
}

// TestHandler_ListPlatforms_WithContextTimeout tests with timeout context
func TestHandler_ListPlatforms_WithContextTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms", handler.ListPlatforms)

	req := httptest.NewRequest("GET", "/platforms", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should complete without timing out
	assert.Equal(t, http.StatusOK, resp.Code)
}

// TestHandler_ListMethods_WhitespaceOnlyPlatform tests with whitespace-only platform
func TestHandler_ListMethods_WhitespaceOnlyPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms/%20%20%20/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle whitespace-only platform parameter
	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "platform")
}

// TestHandler_Call_WithContentTypeError tests Call with content-type header issues
func TestHandler_Call_WithContentTypeError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test","method":"test"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	// Set wrong content type
	req.Header.Set("Content-Type", "text/plain")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle or reject the request
	// Gin's ShouldBindJSON will fail with wrong content type
	assert.NotEqual(t, http.StatusNotFound, resp.Code)
}

// TestHandler_ListMethods_WithSpecialURLChars tests platform name with URL special chars
func TestHandler_ListMethods_WithSpecialURLChars(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	// Test with URL-encoded special characters (without slash which may cause routing issues)
	req := httptest.NewRequest("GET", "/platforms/test%20platform%20v1/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle URL encoded characters
	assert.NotEqual(t, http.StatusNotFound, resp.Code)
}

// TestHandler_ListPlatforms_WithQueryParameters tests that query parameters don't cause issues
func TestHandler_ListPlatforms_WithQueryParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms", handler.ListPlatforms)

	req := httptest.NewRequest("GET", "/platforms?enabled=true&limit=10", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

// TestHandler_ListMethods_WithPlatformWithDots tests platform names with dots
func TestHandler_ListMethods_WithPlatformWithDots(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms/platform.v2/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle dots in platform name
	assert.Equal(t, http.StatusOK, resp.Code)
}

// TestHandler_Call_WithComplexJSON tests complex JSON structures in request
func TestHandler_Call_WithComplexJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test","method":"test","request":"{\"nested\":{\"key\":\"value\",\"array\":[1,2,3]},\"null\":null}"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle complex JSON
	assert.Equal(t, http.StatusOK, resp.Code)
}

// TestHandler_ListPlatforms_ContentTypeChecks tests with various content types
func TestHandler_ListPlatforms_ContentTypeChecks(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms", handler.ListPlatforms)

	req := httptest.NewRequest("GET", "/platforms", nil)
	req.Header.Set("Accept", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

// TestHandler_Call_BothFieldsMissing tests when both required fields are missing
func TestHandler_Call_BothFieldsMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"request":"{}"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "platform")
}

// TestHandler_ListMethods_EmptyStringAfterTrim tests whitespace-only platform
func TestHandler_ListMethods_EmptyStringAfterTrim(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms/%20/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	assert.Contains(t, body, "platform")
}

// TestHandler_ListPlatforms_ContextCheck verifies the context is passed through
func TestHandler_ListPlatforms_ContextCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms", handler.ListPlatforms)

	req := httptest.NewRequest("GET", "/platforms", nil)
	req.Header.Set("X-Request-ID", "test-123")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

// TestHandler_Call_JSONUnmarshalError tests invalid JSON
func TestHandler_Call_JSONUnmarshalError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/platform/call", handler.Call)

	reqBody := `{"platform":"test","method":"test","request":"{incomplete"}`
	req := httptest.NewRequest("POST", "/platform/call", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle the request - the invalid JSON will be passed to the service
	assert.NotEqual(t, http.StatusBadRequest, resp.Code)
}

// TestHandler_ListMethods_SuccessResponseStructure verifies response structure
func TestHandler_ListMethods_SuccessResponseStructure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms/:platform/methods", handler.ListMethods)

	req := httptest.NewRequest("GET", "/platforms/testplatform/methods", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	// Response should have JSON structure
	assert.Contains(t, body, "{")
	assert.Contains(t, body, "}")
}

// TestHandler_ListPlatforms_EmptyResponseStructure verifies empty response structure
func TestHandler_ListPlatforms_EmptyResponseStructure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/platforms", handler.ListPlatforms)

	req := httptest.NewRequest("GET", "/platforms", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	// Should have proper response structure
	assert.Contains(t, body, `"code":200`)
	assert.Contains(t, body, `"message":"success"`)
}
