package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAuthHandlerTest(t *testing.T) (*Handler, *gorm.DB) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = model.AutoMigrate(db)
	require.NoError(t, err)

	adminModel := model.NewAdminModel(db)
	permSvc := permissionservice.NewPermissionService(db)

	service := NewService(adminModel, permSvc, "test-secret-key")
	handler := NewHandler(service)

	return handler, db
}

func createTestAdmin(t *testing.T, db *gorm.DB, username, password string) {
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)

	admin := &model.Admin{
		Username: username,
		Nickname: username + " User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, password)
	require.NoError(t, err)

	role := &model.Role{Name: "admin"}
	err = roleModel.Create(context.Background(), role)
	require.NoError(t, err)

	err = adminModel.AssignRole(context.Background(), admin.ID, role.ID)
	require.NoError(t, err)
}

func TestHandler_Login_ServiceError_Unauthorized_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupAuthHandlerTest(t)

	router := gin.New()
	router.POST("/login", handler.Login)

	tests := []struct {
		name     string
		body     string
		wantCode int
	}{
		{
			name:     "wrong password",
			body:     `{"username":"testuser","password":"wrongpassword"}`,
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "nonexistent user",
			body:     `{"username":"nonexistent","password":"password123"}`,
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "empty username",
			body:     `{"username":"","password":"password123"}`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "empty password",
			body:     `{"username":"testuser","password":""}`,
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := performRequest(router, "POST", "/login", tt.body)
			assert.Equal(t, tt.wantCode, rec.Code)
		})
	}
}

func TestHandler_Login_Success_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAuthHandlerTest(t)
	createTestAdmin(t, db, "testuser", "password123")

	router := gin.New()
	router.POST("/login", handler.Login)

	rec := performRequest(router, "POST", "/login", `{"username":"testuser","password":"password123"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandler_Check_ServiceError_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAuthHandlerTest(t)
	createTestAdmin(t, db, "testuser", "password123")

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("username", "testuser")
		c.Next()
	})
	router.POST("/check", handler.Check)

	// Test with valid request - should return 200
	rec := performRequest(router, "POST", "/check", `{"resource":"game","action":"read"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandler_Check_Unauthorized_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupAuthHandlerTest(t)

	router := gin.New()
	router.POST("/check", handler.Check)

	// Test without username in context
	rec := performRequest(router, "POST", "/check", `{"resource":"game","action":"read"}`)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_BatchCheck_ServiceError_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAuthHandlerTest(t)
	createTestAdmin(t, db, "testuser", "password123")

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("username", "testuser")
		c.Next()
	})
	router.POST("/batch-check", handler.BatchCheck)

	// Test with valid request - should return 200
	rec := performRequest(router, "POST", "/batch-check", `{"checks":[{"resource":"game","action":"read"}]}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandler_BatchCheck_Unauthorized_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupAuthHandlerTest(t)

	router := gin.New()
	router.POST("/batch-check", handler.BatchCheck)

	// Test without username in context
	rec := performRequest(router, "POST", "/batch-check", `{"checks":[{"resource":"game","action":"read"}]}`)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_Logout_Success_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupAuthHandlerTest(t)

	router := gin.New()
	router.POST("/logout", handler.Logout)

	rec := performRequest(router, "POST", "/logout", `{}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandler_Check_InvalidJSON_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupAuthHandlerTest(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("username", "testuser")
		c.Next()
	})
	router.POST("/check", handler.Check)

	rec := performRequest(router, "POST", "/check", `invalid json`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_BatchCheck_InvalidJSON_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupAuthHandlerTest(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("username", "testuser")
		c.Next()
	})
	router.POST("/batch-check", handler.BatchCheck)

	rec := performRequest(router, "POST", "/batch-check", `invalid json`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Login_InvalidJSON_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupAuthHandlerTest(t)

	router := gin.New()
	router.POST("/login", handler.Login)

	rec := performRequest(router, "POST", "/login", `invalid json`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Check_ServiceError_UserNotFound_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupAuthHandlerTest(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("username", "nonexistent_user")
		c.Next()
	})
	router.POST("/check", handler.Check)

	// Test with nonexistent user - should return 500
	rec := performRequest(router, "POST", "/check", `{"resource":"game","action":"read"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_BatchCheck_ServiceError_UserNotFound_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupAuthHandlerTest(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("username", "nonexistent_user")
		c.Next()
	})
	router.POST("/batch-check", handler.BatchCheck)

	// Test with nonexistent user - should return 500
	rec := performRequest(router, "POST", "/batch-check", `{"checks":[{"resource":"game","action":"read"}]}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_Check_PermissionDenied_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAuthHandlerTest(t)
	createTestAdmin(t, db, "testuser", "password123")

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("username", "testuser")
		c.Next()
	})
	router.POST("/check", handler.Check)

	// Test with resource/action that user doesn't have permission for
	rec := performRequest(router, "POST", "/check", `{"resource":"nonexistent","action":"unknown"}`)
	assert.Equal(t, http.StatusOK, rec.Code)

	// The response should indicate permission denied
	var result map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, false, result["allowed"])
}

func TestHandler_BatchCheck_PermissionDenied_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAuthHandlerTest(t)
	createTestAdmin(t, db, "testuser", "password123")

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("username", "testuser")
		c.Next()
	})
	router.POST("/batch-check", handler.BatchCheck)

	// Test with resource/action that user doesn't have permission for
	rec := performRequest(router, "POST", "/batch-check", `{"checks":[{"resource":"nonexistent","action":"unknown"}]}`)
	assert.Equal(t, http.StatusOK, rec.Code)

	// The response should indicate permission denied
	var result map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	assert.NoError(t, err)
	results := result["results"].([]interface{})
	assert.Len(t, results, 1)
	firstResult := results[0].(map[string]interface{})
	assert.Equal(t, false, firstResult["allowed"])
}

// performRequest is a helper to perform HTTP requests
func performRequest(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
