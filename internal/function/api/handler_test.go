// Package api provides tests for the function metadata HTTP handlers.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/function/registry"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"github.com/gin-gonic/gin"
	. "github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestRouter() (*gin.Engine, *Service) {
	store := registry.NewStore()
	service := NewService(store)
	handler := NewHandler(service)

	router := gin.New()
	functions := router.Group("/api/v1/metadata/functions")
	{
		functions.GET("", handler.ListFunctions)
		functions.POST("", handler.RegisterFunction)
		functions.GET("/:id", handler.GetFunction)
		functions.PUT("/:id", handler.UpdateFunction)
		functions.DELETE("/:id", handler.DeleteFunction)
		functions.GET("/categories", handler.GetCategories)
		functions.GET("/tags", handler.GetTags)
	}

	return router, service
}

func TestHandler_ListFunctions(t *testing.T) {
	router, service := setupTestRouter()

	// Register test functions
	service.Register(testCtx(), &functionv1.FunctionMetadata{
		Id:       "player.get",
		Category: "player",
		Tags:     []string{"read"},
		Name:     "Get Player",
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
	})

	// Test list all
	req := httptest.NewRequest("GET", "/api/v1/metadata/functions?page=1&pageSize=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusOK, w.Code)
	var resp ListFunctionsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	Nil(t, err)
	True(t, len(resp.Functions) >= 1)
}

func TestHandler_ListFunctionsWithFilters(t *testing.T) {
	router, service := setupTestRouter()

	// Register test functions with different categories
	service.Register(testCtx(), &functionv1.FunctionMetadata{
		Id:       "player.get",
		Category: "player",
		Tags:     []string{"read"},
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
	})
	service.Register(testCtx(), &functionv1.FunctionMetadata{
		Id:       "game.create",
		Category: "game",
		Tags:     []string{"write"},
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_HIGH},
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_COMMAND},
	})

	// Test filter by category
	req := httptest.NewRequest("GET", "/api/v1/metadata/functions?category=player&page=1&pageSize=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusOK, w.Code)
	var resp ListFunctionsResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	Equal(t, 1, len(resp.Functions))
	Equal(t, "player", resp.Functions[0].Category)
}

func TestHandler_GetFunction(t *testing.T) {
	router, service := setupTestRouter()

	// Register test function
	service.Register(testCtx(), &functionv1.FunctionMetadata{
		Id:       "player.ban",
		Category: "player",
		Name:     "Ban Player",
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_HIGH},
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_COMMAND},
	})

	// Test get existing function
	req := httptest.NewRequest("GET", "/api/v1/metadata/functions/player.ban", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusOK, w.Code)
	var resp GetFunctionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	Equal(t, "player.ban", resp.Function.ID)
	Equal(t, "Ban Player", resp.Function.Name)
}

func TestHandler_GetFunctionNotFound(t *testing.T) {
	router, _ := setupTestRouter()

	// Test get non-existing function
	req := httptest.NewRequest("GET", "/api/v1/metadata/functions/not.found", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_RegisterFunction(t *testing.T) {
	router, _ := setupTestRouter()

	reqBody := RegisterFunctionRequest{
		ID:       "player.kick",
		Version:  "1.0.0",
		Category: "player",
		Tags:     []string{"moderation"},
		Name:     "Kick Player",
		Security: &FunctionSecurity{RiskLevel: "high"},
		Behavior: &FunctionBehavior{Mode: "command"},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/metadata/functions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusCreated, w.Code)
	var resp RegisterFunctionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	Equal(t, "player.kick", resp.Function.ID)
	Equal(t, "Kick Player", resp.Function.Name)
}

func TestHandler_DeleteFunction(t *testing.T) {
	router, service := setupTestRouter()

	// Register test function
	service.Register(testCtx(), &functionv1.FunctionMetadata{
		Id:       "player.warn",
		Category: "player",
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_COMMAND},
	})

	// Delete function
	req := httptest.NewRequest("DELETE", "/api/v1/metadata/functions/player.warn", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusNoContent, w.Code)
}

func TestHandler_DeleteFunctionNotFound(t *testing.T) {
	router, _ := setupTestRouter()

	// Delete non-existing function
	req := httptest.NewRequest("DELETE", "/api/v1/metadata/functions/not.found", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_GetCategories(t *testing.T) {
	router, service := setupTestRouter()

	// Register test functions
	service.Register(testCtx(), &functionv1.FunctionMetadata{
		Id:       "player.get",
		Category: "player",
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
	})
	service.Register(testCtx(), &functionv1.FunctionMetadata{
		Id:       "game.create",
		Category: "game",
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_HIGH},
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_COMMAND},
	})

	// Get categories
	req := httptest.NewRequest("GET", "/api/v1/metadata/functions/categories", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusOK, w.Code)
	var resp map[string][]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	Contains(t, resp["categories"], "player")
	Contains(t, resp["categories"], "game")
}

func TestHandler_GetTags(t *testing.T) {
	router, service := setupTestRouter()

	// Register test functions
	service.Register(testCtx(), &functionv1.FunctionMetadata{
		Id:       "player.get",
		Category: "player",
		Tags:     []string{"read", "player"},
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
	})

	// Get tags
	req := httptest.NewRequest("GET", "/api/v1/metadata/functions/tags", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusOK, w.Code)
	var resp map[string][]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	Contains(t, resp["tags"], "read")
	Contains(t, resp["tags"], "player")
}

func TestHandler_RegisterFunctionValidation(t *testing.T) {
	router, _ := setupTestRouter()

	tests := []struct {
		name       string
		reqBody    RegisterFunctionRequest
		expectCode int
	}{
		{
			name: "missing ID",
			reqBody: RegisterFunctionRequest{
				Name: "Test",
			},
			expectCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.reqBody)
			req := httptest.NewRequest("POST", "/api/v1/metadata/functions", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Equal(t, tt.expectCode, w.Code)
		})
	}
}

// Helper function for test context
func testCtx() context.Context {
	return context.Background()
}
