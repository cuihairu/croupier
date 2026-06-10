package role

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRoleHandlerTestExtra(t *testing.T) (*Handler, *gorm.DB) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = model.AutoMigrate(db)
	require.NoError(t, err)

	// Create test admin and permissions
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)
	permissionModel := model.NewPermissionModel(db)

	admin := &model.Admin{
		Username: "testadmin",
		Nickname: "Test Admin",
		Status:   1,
	}
	err = adminModel.Create(nil, admin, "password123")
	require.NoError(t, err)

	role := &model.Role{Name: "admin"}
	err = roleModel.Create(nil, role)
	require.NoError(t, err)

	err = adminModel.AssignRole(nil, admin.ID, role.ID)
	require.NoError(t, err)

	err = roleModel.ReplacePermissions(nil, role.ID, []string{
		"admin:all", "roles:manage", "role:write",
	})
	require.NoError(t, err)

	permissions := []*model.Permission{
		{ID: "admin:all", Name: "Admin All", Resource: "admin", Action: "all", Category: "admin"},
		{ID: "roles:manage", Name: "Roles Manage", Resource: "roles", Action: "manage", Category: "role"},
		{ID: "role:write", Name: "Role Write", Resource: "role", Action: "write", Category: "role"},
		{ID: "game:read", Name: "Game Read", Resource: "game", Action: "read", Category: "game"},
	}
	for _, perm := range permissions {
		err = db.Create(perm).Error
		require.NoError(t, err)
	}

	svcCtx := &svc.ServiceContext{
		DB:              db,
		AdminModel:      adminModel,
		RoleModel:       roleModel,
		PermissionModel: permissionModel,
		Cache:           cache.NewNullCache(),
		CacheHelper:     cache.NewCacheHelper(cache.NewNullCache()),
	}

	service := NewService(svcCtx)
	handler := NewHandler(service)

	return handler, db
}

func addAuthMiddlewareExtra(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		adminModel := model.NewAdminModel(db)
		admin, _ := adminModel.FindByUsername(c.Request.Context(), "testadmin")
		ctx := context.WithValue(c.Request.Context(), "username", "testadmin")
		ctx = context.WithValue(ctx, "adminID", admin.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func TestHandler_RoleDetail_NotFound_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)

	router := gin.New()
	router.Use(addAuthMiddlewareExtra(db))
	router.GET("/roles/:id", handler.RoleDetail)

	req := httptest.NewRequest("GET", "/roles/99999", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleDetail_InvalidID_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)

	router := gin.New()
	router.Use(addAuthMiddlewareExtra(db))
	router.GET("/roles/:id", handler.RoleDetail)

	req := httptest.NewRequest("GET", "/roles/invalid", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleDetail_Success_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)
	roleModel := model.NewRoleModel(db)

	// Create a role
	role := &model.Role{
		Name:        "testrole",
		Description: "Test Role",
		Category:    "test",
	}
	err := roleModel.Create(nil, role)
	require.NoError(t, err)

	router := gin.New()
	router.Use(addAuthMiddlewareExtra(db))
	router.GET("/roles/:id", handler.RoleDetail)

	req := httptest.NewRequest("GET", "/roles/"+strconv.FormatUint(uint64(role.ID), 10), nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result map[string]interface{}
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, "testrole", result["name"])
}

func TestHandler_RoleDelete_Success_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)
	roleModel := model.NewRoleModel(db)

	// Create a role to delete
	role := &model.Role{Name: "todelete"}
	err := roleModel.Create(nil, role)
	require.NoError(t, err)

	router := gin.New()
	router.Use(addAuthMiddlewareExtra(db))
	router.DELETE("/roles/:id", handler.RoleDelete)

	req := httptest.NewRequest("DELETE", "/roles/"+strconv.FormatUint(uint64(role.ID), 10), nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleDelete_NotFound_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)

	router := gin.New()
	router.Use(addAuthMiddlewareExtra(db))
	router.DELETE("/roles/:id", handler.RoleDelete)

	req := httptest.NewRequest("DELETE", "/roles/99999", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleDelete_InvalidID_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)

	router := gin.New()
	router.Use(addAuthMiddlewareExtra(db))
	router.DELETE("/roles/:id", handler.RoleDelete)

	req := httptest.NewRequest("DELETE", "/roles/invalid", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleUpdate_Success_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)
	roleModel := model.NewRoleModel(db)

	// Create a role to update
	role := &model.Role{Name: "toupdate"}
	err := roleModel.Create(nil, role)
	require.NoError(t, err)

	router := gin.New()
	router.Use(addAuthMiddlewareExtra(db))
	router.PUT("/roles/:id", handler.RoleUpdate)

	body := map[string]interface{}{
		"name":        "updated",
		"description": "Updated role",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/roles/"+strconv.FormatUint(uint64(role.ID), 10), bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result map[string]interface{}
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, "updated", result["name"])
}

func TestHandler_RoleUpdate_NotFound_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)

	router := gin.New()
	router.Use(addAuthMiddlewareExtra(db))
	router.PUT("/roles/:id", handler.RoleUpdate)

	body := map[string]interface{}{
		"name": "updated",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/roles/99999", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleUpdate_InvalidID_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)

	router := gin.New()
	router.Use(addAuthMiddlewareExtra(db))
	router.PUT("/roles/:id", handler.RoleUpdate)

	body := map[string]interface{}{
		"name": "updated",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/roles/invalid", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleUpdate_WithPermissions_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)
	roleModel := model.NewRoleModel(db)

	// Create a role to update
	role := &model.Role{Name: "toupdateperms"}
	err := roleModel.Create(nil, role)
	require.NoError(t, err)

	router := gin.New()
	router.Use(addAuthMiddlewareExtra(db))
	router.PUT("/roles/:id", handler.RoleUpdate)

	body := map[string]interface{}{
		"name":        "updated",
		"permissions": []string{"game:read"},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/roles/"+strconv.FormatUint(uint64(role.ID), 10), bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleUpdate_EmptyBody_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)
	roleModel := model.NewRoleModel(db)

	// Create a role to update
	role := &model.Role{Name: "toupdateempty"}
	err := roleModel.Create(nil, role)
	require.NoError(t, err)

	router := gin.New()
	router.Use(addAuthMiddlewareExtra(db))
	router.PUT("/roles/:id", handler.RoleUpdate)

	req := httptest.NewRequest("PUT", "/roles/"+strconv.FormatUint(uint64(role.ID), 10), bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Empty body should still succeed (no fields to update)
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_RolesList_Success_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)

	router := gin.New()
	router.Use(addAuthMiddlewareExtra(db))
	router.GET("/roles", handler.RolesList)

	req := httptest.NewRequest("GET", "/roles?page=1&pageSize=10", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_RolesList_WithCategory_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)

	router := gin.New()
	router.Use(addAuthMiddlewareExtra(db))
	router.GET("/roles", handler.RolesList)

	req := httptest.NewRequest("GET", "/roles?page=1&pageSize=10&category=system", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_RolesList_WithSearch_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)

	router := gin.New()
	router.Use(addAuthMiddlewareExtra(db))
	router.GET("/roles", handler.RolesList)

	req := httptest.NewRequest("GET", "/roles?page=1&pageSize=10&search=admin", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_RolesList_DefaultPagination_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)

	router := gin.New()
	router.Use(addAuthMiddlewareExtra(db))
	router.GET("/roles", handler.RolesList)

	req := httptest.NewRequest("GET", "/roles", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleCreate_Success_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)

	router := gin.New()
	router.Use(addAuthMiddlewareExtra(db))
	router.POST("/roles", handler.RoleCreate)

	body := map[string]interface{}{
		"name":        "newrole",
		"description": "New Role",
		"category":    "custom",
		"permissions": []string{"game:read"},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/roles", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, "newrole", result["name"])
}

func TestHandler_RoleCreate_EmptyName_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)

	router := gin.New()
	router.Use(addAuthMiddlewareExtra(db))
	router.POST("/roles", handler.RoleCreate)

	body := map[string]interface{}{
		"name": "",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/roles", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleCreate_InvalidJSON_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupRoleHandlerTestExtra(t)

	router := gin.New()
	router.POST("/roles", handler.RoleCreate)

	req := httptest.NewRequest("POST", "/roles", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_EnsurePermissionIDs_NilRoleModel(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = model.AutoMigrate(db)
	require.NoError(t, err)

	// Create test admin with permissions
	adminModel := model.NewAdminModel(db)
	admin := &model.Admin{
		Username: "testadmin",
		Nickname: "Test Admin",
		Status:   1,
	}
	err = adminModel.Create(nil, admin, "password123")
	require.NoError(t, err)

	roleModel := model.NewRoleModel(db)
	role := &model.Role{Name: "admin"}
	err = roleModel.Create(nil, role)
	require.NoError(t, err)

	err = adminModel.AssignRole(nil, admin.ID, role.ID)
	require.NoError(t, err)

	err = roleModel.ReplacePermissions(nil, role.ID, []string{"admin:all"})
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		DB:              db,
		AdminModel:      adminModel,
		RoleModel:       nil, // Nil role model
		PermissionModel: model.NewPermissionModel(db),
		Cache:           cache.NewNullCache(),
		CacheHelper:     cache.NewCacheHelper(cache.NewNullCache()),
	}

	service := NewService(svcCtx)
	handler := NewHandler(service)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(addAuthMiddlewareExtra(db))
	router.POST("/roles", handler.RoleCreate)

	body := map[string]interface{}{
		"name":        "newrole",
		"permissions": []string{"perm1"},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/roles", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleDetail_PermissionDenied(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)

	// Create a user without permissions
	adminModel := model.NewAdminModel(db)
	admin := &model.Admin{
		Username: "noperm",
		Nickname: "No Perm",
		Status:   1,
	}
	err := adminModel.Create(nil, admin, "password123")
	require.NoError(t, err)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), "username", "noperm")
		ctx = context.WithValue(ctx, "adminID", admin.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.GET("/roles/:id", handler.RoleDetail)

	req := httptest.NewRequest("GET", "/roles/1", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleDelete_PermissionDenied_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)

	// Create a user without permissions
	adminModel := model.NewAdminModel(db)
	admin := &model.Admin{
		Username: "noperm",
		Nickname: "No Perm",
		Status:   1,
	}
	err := adminModel.Create(nil, admin, "password123")
	require.NoError(t, err)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), "username", "noperm")
		ctx = context.WithValue(ctx, "adminID", admin.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.DELETE("/roles/:id", handler.RoleDelete)

	req := httptest.NewRequest("DELETE", "/roles/1", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleUpdate_PermissionDenied_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)

	// Create a user without permissions
	adminModel := model.NewAdminModel(db)
	admin := &model.Admin{
		Username: "noperm",
		Nickname: "No Perm",
		Status:   1,
	}
	err := adminModel.Create(nil, admin, "password123")
	require.NoError(t, err)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), "username", "noperm")
		ctx = context.WithValue(ctx, "adminID", admin.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.PUT("/roles/:id", handler.RoleUpdate)

	body := map[string]interface{}{
		"name": "updated",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/roles/1", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleCreate_PermissionDenied_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)

	// Create a user without permissions
	adminModel := model.NewAdminModel(db)
	admin := &model.Admin{
		Username: "noperm",
		Nickname: "No Perm",
		Status:   1,
	}
	err := adminModel.Create(nil, admin, "password123")
	require.NoError(t, err)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), "username", "noperm")
		ctx = context.WithValue(ctx, "adminID", admin.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.POST("/roles", handler.RoleCreate)

	body := map[string]interface{}{
		"name": "newrole",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/roles", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_RolesList_PermissionDenied_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)

	// Create a user without permissions
	adminModel := model.NewAdminModel(db)
	admin := &model.Admin{
		Username: "noperm",
		Nickname: "No Perm",
		Status:   1,
	}
	err := adminModel.Create(nil, admin, "password123")
	require.NoError(t, err)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), "username", "noperm")
		ctx = context.WithValue(ctx, "adminID", admin.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.GET("/roles", handler.RolesList)

	req := httptest.NewRequest("GET", "/roles", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleUpdate_ZeroID_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)

	router := gin.New()
	router.Use(addAuthMiddlewareExtra(db))
	router.PUT("/roles/:id", handler.RoleUpdate)

	body := map[string]interface{}{
		"name": "updated",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/roles/0", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleDelete_ZeroID_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)

	router := gin.New()
	router.Use(addAuthMiddlewareExtra(db))
	router.DELETE("/roles/:id", handler.RoleDelete)

	req := httptest.NewRequest("DELETE", "/roles/0", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleDetail_ZeroID_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTestExtra(t)

	router := gin.New()
	router.Use(addAuthMiddlewareExtra(db))
	router.GET("/roles/:id", handler.RoleDetail)

	req := httptest.NewRequest("GET", "/roles/0", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}
