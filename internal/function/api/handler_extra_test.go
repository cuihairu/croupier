package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	. "github.com/stretchr/testify/assert"
)

func TestHandler_ListFunctions_WithAllFilters(t *testing.T) {
	router, service := setupTestRouter()

	// Register test functions
	service.Register(testCtx(), &functionv1.FunctionMetadata{
		Id:       "player.get",
		Resource: "player",
		Tags:     []string{"read"},
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
	})

	// Test with all filters
	req := httptest.NewRequest("GET", "/api/v1/metadata/functions?resource=player&tag=read&risk=low&mode=query&page=1&pageSize=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusOK, w.Code)
	var resp ListFunctionsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	Nil(t, err)
	True(t, len(resp.Functions) >= 1)
}

func TestHandler_ListFunctions_EmptyResult(t *testing.T) {
	router, _ := setupTestRouter()

	req := httptest.NewRequest("GET", "/api/v1/metadata/functions?resource=nonexistent&page=1&pageSize=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusOK, w.Code)
	var resp ListFunctionsResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	Equal(t, 0, len(resp.Functions))
}

func TestHandler_GetFunction_InvalidID(t *testing.T) {
	router, _ := setupTestRouter()

	// Test with empty ID (should still work as it's a valid path)
	req := httptest.NewRequest("GET", "/api/v1/metadata/functions/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 404 or redirect
	True(t, w.Code == http.StatusNotFound || w.Code == http.StatusMovedPermanently)
}

func TestHandler_RegisterFunction_InvalidJSON(t *testing.T) {
	router, _ := setupTestRouter()

	req := httptest.NewRequest("POST", "/api/v1/metadata/functions", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_RegisterFunction_EmptyBody(t *testing.T) {
	router, _ := setupTestRouter()

	req := httptest.NewRequest("POST", "/api/v1/metadata/functions", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_UpdateFunction_InvalidJSON(t *testing.T) {
	router, service := setupTestRouter()

	service.Register(testCtx(), &functionv1.FunctionMetadata{
		Id:   "test.func",
		Name: "Test",
	})

	req := httptest.NewRequest("PUT", "/api/v1/metadata/functions/test.func", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_UpdateFunction_PartialUpdate(t *testing.T) {
	router, service := setupTestRouter()

	service.Register(testCtx(), &functionv1.FunctionMetadata{
		Id:          "partial.update",
		Name:        "Original Name",
		Description: "Original Description",
		Security:    &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		Behavior:    &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_COMMAND},
	})

	// Update only name
	reqBody := `{"name": "Updated Name"}`
	req := httptest.NewRequest("PUT", "/api/v1/metadata/functions/partial.update", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusOK, w.Code)
	var resp UpdateFunctionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	Equal(t, "Updated Name", resp.Function.Name)
	Equal(t, "Original Description", resp.Function.Description)
}

func TestHandler_UpdateFunction_WithExtensions(t *testing.T) {
	router, service := setupTestRouter()

	service.Register(testCtx(), &functionv1.FunctionMetadata{
		Id:   "ext.update",
		Name: "Test",
	})

	reqBody := `{"extensions": {"x-custom": "value", "x-another": "test"}}`
	req := httptest.NewRequest("PUT", "/api/v1/metadata/functions/ext.update", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusOK, w.Code)
	var resp UpdateFunctionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	Equal(t, "value", resp.Function.Extensions["x-custom"])
	Equal(t, "test", resp.Function.Extensions["x-another"])
}

func TestHandler_DeleteFunction_InvalidID(t *testing.T) {
	router, _ := setupTestRouter()

	req := httptest.NewRequest("DELETE", "/api/v1/metadata/functions/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 404 or redirect
	True(t, w.Code == http.StatusNotFound || w.Code == http.StatusMovedPermanently)
}

func TestHandler_UnknownMetadataRouteNotFound(t *testing.T) {
	router, _ := setupTestRouter()

	req := httptest.NewRequest("POST", "/api/v1/metadata/functions/removed-route", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_GetResources_Empty(t *testing.T) {
	router, _ := setupTestRouter()

	req := httptest.NewRequest("GET", "/api/v1/metadata/functions/resources", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusOK, w.Code)
	var resp map[string][]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	NotNil(t, resp["resources"])
}

func TestHandler_GetTags_Empty(t *testing.T) {
	router, _ := setupTestRouter()

	req := httptest.NewRequest("GET", "/api/v1/metadata/functions/tags", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusOK, w.Code)
	var resp map[string][]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	NotNil(t, resp["tags"])
}

func TestHandler_ListFunctions_WithModeFilter(t *testing.T) {
	router, service := setupTestRouter()

	service.Register(testCtx(), &functionv1.FunctionMetadata{
		Id:       "query.func",
		Resource: "test",
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
	})
	service.Register(testCtx(), &functionv1.FunctionMetadata{
		Id:       "command.func",
		Resource: "test",
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_COMMAND},
	})

	req := httptest.NewRequest("GET", "/api/v1/metadata/functions?mode=query&page=1&pageSize=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusOK, w.Code)
	var resp ListFunctionsResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	// Should only return query functions
	for _, f := range resp.Functions {
		Equal(t, "query", f.Behavior.Mode)
	}
}

func TestHandler_RegisterFunction_WithAllFields(t *testing.T) {
	router, _ := setupTestRouter()

	reqBody := RegisterFunctionRequest{
		ID:           "full.func",
		Version:      "1.0.0",
		Resource:     "test",
		Tags:         []string{"tag1", "tag2"},
		Name:         "Full Function",
		Description:  "A function with all fields",
		InputSchema:  `{"type":"object"}`,
		OutputSchema: `{"type":"object"}`,
		Security: &FunctionSecurity{
			RiskLevel:        "high",
			Permission:       "test.invoke",
			RequiresApproval: true,
			AuditLog:         true,
		},
		Behavior: &FunctionBehavior{
			Mode:       "command",
			Idempotent: true,
			TimeoutMs:  60000,
		},
		Extensions: map[string]string{
			"x-custom": "value",
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/metadata/functions", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusCreated, w.Code)
	var resp RegisterFunctionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	Equal(t, "full.func", resp.Function.ID)
	Equal(t, "Full Function", resp.Function.Name)
	Equal(t, "high", resp.Function.Security.RiskLevel)
	Equal(t, "command", resp.Function.Behavior.Mode)
	Equal(t, "value", resp.Function.Extensions["x-custom"])
}
