package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/policy"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	// Migrate the FunctionPolicy model
	err = db.AutoMigrate(&model.FunctionPolicy{})
	require.NoError(t, err)

	return db
}

func setupTestHandler(t *testing.T) (*Handler, *gorm.DB) {
	db := setupTestDB(t)

	// Create a temporary default policies file
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/default-policies.yaml"
	yamlContent := []byte(`low:
  require_approval: false
  require_audit: false
  allowed_roles:
    - user
  description: "Low risk"
medium:
  require_approval: false
  require_audit: true
  allowed_roles:
    - operator
  description: "Medium risk"
high:
  require_approval: true
  approval_workflow: single_admin
  require_audit: true
  allowed_roles:
    - admin
  description: "High risk"
danger:
  require_approval: true
  approval_workflow: two_person
  require_audit: true
  allowed_roles:
    - super_admin
  description: "Danger"
`)

	err := os.WriteFile(tmpFile, yamlContent, 0644)
	require.NoError(t, err)

	manager, err := policy.NewManager(db, tmpFile)
	require.NoError(t, err)

	return NewHandler(manager), db
}

func TestHandler_GetPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupTestHandler(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	tests := []struct {
		name           string
		functionID     string
		riskLevel      string
		setupOverride  bool
		expectedStatus int
		checkBody      func(t *testing.T, body []byte)
	}{
		{
			name:           "获取默认medium风险策略",
			functionID:     "test.function",
			riskLevel:      "medium",
			setupOverride:  false,
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				err := json.Unmarshal(body, &resp)
				require.NoError(t, err)
				assert.True(t, resp["requireAudit"].(bool))
				assert.False(t, resp["requireApproval"].(bool))
			},
		},
		{
			name:           "获取默认high风险策略",
			functionID:     "test.dangerous",
			riskLevel:      "high",
			setupOverride:  false,
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				err := json.Unmarshal(body, &resp)
				require.NoError(t, err)
				assert.True(t, resp["requireApproval"].(bool))
				assert.Equal(t, "single_admin", resp["approvalWorkflow"])
			},
		},
		{
			name:           "获取覆盖策略",
			functionID:     "test.override",
			riskLevel:      "",
			setupOverride:  true,
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				err := json.Unmarshal(body, &resp)
				require.NoError(t, err)
				assert.True(t, resp["isOverride"].(bool))
			},
		},
		{
			name:           "缺少function_id",
			functionID:     "",
			riskLevel:      "",
			setupOverride:  false,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup override if needed
			if tt.setupOverride {
				override := &policy.Policy{
					RequireApproval:  true,
					ApprovalWorkflow: "two_person",
					RequireAudit:     true,
					AllowedRoles:     []string{"super_admin"},
				}
				err := handler.manager.SetOverride(context.Background(), tt.functionID, override)
				require.NoError(t, err)
			}

			router := gin.New()
			router.GET("/functions/:function_id/policy", handler.GetPolicy)

			url := "/functions/" + tt.functionID + "/policy"
			if tt.riskLevel != "" {
				url += "?risk_level=" + tt.riskLevel
			}

			req, _ := http.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkBody != nil && w.Code == http.StatusOK {
				tt.checkBody(t, w.Body.Bytes())
			}
		})
	}
}

func TestHandler_SetPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupTestHandler(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	tests := []struct {
		name           string
		functionID     string
		requestBody    interface{}
		expectedStatus int
	}{
		{
			name:       "设置策略成功",
			functionID: "test.function",
			requestBody: map[string]interface{}{
				"requireApproval":  true,
				"approvalWorkflow": "two_person",
				"requireAudit":     true,
				"allowedRoles":     []string{"admin", "super_admin"},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:       "设置简单策略",
			functionID: "test.simple",
			requestBody: map[string]interface{}{
				"requireApproval": false,
				"requireAudit":    true,
				"allowedRoles":    []string{"operator"},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "缺少function_id",
			functionID:     "",
			requestBody:    map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "无效JSON",
			functionID:     "test.function",
			requestBody:    "invalid json",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.PUT("/functions/:function_id/policy", handler.SetPolicy)

			var body []byte
			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else {
				var err error
				body, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}

			req, _ := http.NewRequest("PUT", "/functions/"+tt.functionID+"/policy", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				// Verify policy was set
				pol, err := handler.manager.GetPolicy(context.Background(), tt.functionID, "")
				require.NoError(t, err)
				assert.True(t, pol.IsOverride)
			}
		})
	}
}

func TestHandler_DeletePolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupTestHandler(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	// Setup: create an override first
	override := &policy.Policy{
		RequireApproval:  true,
		ApprovalWorkflow: "two_person",
		RequireAudit:     true,
		AllowedRoles:     []string{"super_admin"},
	}
	err := handler.manager.SetOverride(context.Background(), "test.to-delete", override)
	require.NoError(t, err)

	// Verify override exists
	pol, err := handler.manager.GetPolicy(context.Background(), "test.to-delete", "")
	require.NoError(t, err)
	assert.True(t, pol.IsOverride)

	router := gin.New()
	router.DELETE("/functions/:function_id/policy", handler.DeletePolicy)

	req, _ := http.NewRequest("DELETE", "/functions/test.to-delete/policy", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify override was deleted
	pol, err = handler.manager.GetPolicy(context.Background(), "test.to-delete", "medium")
	require.NoError(t, err)
	assert.False(t, pol.IsOverride)
}

func TestHandler_ListOverrides(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupTestHandler(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	// Setup: create some overrides
	override1 := &policy.Policy{
		RequireApproval:  true,
		ApprovalWorkflow: "single_admin",
		RequireAudit:     true,
		AllowedRoles:     []string{"admin"},
	}
	override2 := &policy.Policy{
		RequireApproval: false,
		RequireAudit:    true,
		AllowedRoles:    []string{"operator"},
	}

	err := handler.manager.SetOverride(context.Background(), "func1", override1)
	require.NoError(t, err)
	err = handler.manager.SetOverride(context.Background(), "func2", override2)
	require.NoError(t, err)

	router := gin.New()
	router.GET("/policies/overrides", handler.ListOverrides)

	req, _ := http.NewRequest("GET", "/policies/overrides", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	policies := resp["policies"].([]interface{})
	assert.GreaterOrEqual(t, len(policies), 2)
}

func TestHandler_GetDefaultPolicies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupTestHandler(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	router := gin.New()
	router.GET("/policies/defaults", handler.GetDefaultPolicies)

	req, _ := http.NewRequest("GET", "/policies/defaults", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Check all risk levels exist
	assert.Contains(t, resp, "low")
	assert.Contains(t, resp, "medium")
	assert.Contains(t, resp, "high")
	assert.Contains(t, resp, "danger")

	// Check low risk policy
	low := resp["low"].(map[string]interface{})
	assert.False(t, low["requireApproval"].(bool))
	assert.False(t, low["requireAudit"].(bool))

	// Check high risk policy
	high := resp["high"].(map[string]interface{})
	assert.True(t, high["requireApproval"].(bool))
	assert.Equal(t, "single_admin", high["approvalWorkflow"])
}

func TestHandler_ReloadConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupTestHandler(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	router := gin.New()
	router.POST("/policies/reload", handler.ReloadConfig)

	req, _ := http.NewRequest("POST", "/policies/reload", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Configuration reloaded", resp["message"])
}
