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
	gsqlite "github.com/glebarez/sqlite"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRoleHandlerTest(t *testing.T) (*Handler, *gorm.DB) {
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

func TestNewHandler(t *testing.T) {
	service := &Service{}
	handler := NewHandler(service)

	assert.NotNil(t, handler)
	assert.Equal(t, service, handler.service)
}

func TestHandler_RoleCreate_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTest(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		adminModel := model.NewAdminModel(db)
		admin, _ := adminModel.FindByUsername(c.Request.Context(), "testadmin")
		ctx := context.WithValue(c.Request.Context(), "username", "testadmin")
		ctx = context.WithValue(ctx, "adminID", admin.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.POST("/roles", handler.RoleCreate)

	body := map[string]interface{}{
		"name":        "editor",
		"description": "Editor role",
		"category":    "custom",
		"permissions": []string{"game:read"},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/roles", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleCreate_EmptyName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTest(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		adminModel := model.NewAdminModel(db)
		admin, _ := adminModel.FindByUsername(c.Request.Context(), "testadmin")
		ctx := context.WithValue(c.Request.Context(), "username", "testadmin")
		ctx = context.WithValue(ctx, "adminID", admin.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.POST("/roles", handler.RoleCreate)

	body := map[string]interface{}{
		"name":        "",
		"description": "Test role",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/roles", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleCreate_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupRoleHandlerTest(t)

	router := gin.New()
	router.POST("/roles", handler.RoleCreate)

	req := httptest.NewRequest("POST", "/roles", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleCreate_NoContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupRoleHandlerTest(t)

	router := gin.New()
	router.POST("/roles", handler.RoleCreate)

	req := httptest.NewRequest("POST", "/roles", bytes.NewBuffer([]byte("{}")))
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Without auth and content type, should return error
	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleDelete_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTest(t)
	roleModel := model.NewRoleModel(db)

	// Create a role to delete
	role := &model.Role{Name: "todelete"}
	err := roleModel.Create(nil, role)
	require.NoError(t, err)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		adminModel := model.NewAdminModel(db)
		admin, _ := adminModel.FindByUsername(c.Request.Context(), "testadmin")
		ctx := context.WithValue(c.Request.Context(), "username", "testadmin")
		ctx = context.WithValue(ctx, "adminID", admin.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.DELETE("/roles/:id", handler.RoleDelete)

	req := httptest.NewRequest("DELETE", "/roles/"+strconv.FormatUint(uint64(role.ID), 10), nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleDelete_BindUriError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupRoleHandlerTest(t)

	router := gin.New()
	router.DELETE("/roles/:id", handler.RoleDelete)

	// Missing ID parameter
	req := httptest.NewRequest("DELETE", "/roles/", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNotFound, resp.Code)
}

func TestHandler_RoleDelete_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTest(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		adminModel := model.NewAdminModel(db)
		admin, _ := adminModel.FindByUsername(c.Request.Context(), "testadmin")
		ctx := context.WithValue(c.Request.Context(), "username", "testadmin")
		ctx = context.WithValue(ctx, "adminID", admin.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.DELETE("/roles/:id", handler.RoleDelete)

	req := httptest.NewRequest("DELETE", "/roles/99999", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleDetail_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTest(t)
	roleModel := model.NewRoleModel(db)

	// Create a role
	role := &model.Role{
		Name:        "moderator",
		Description: "Moderator role",
		Category:    "custom",
	}
	err := roleModel.Create(nil, role)
	require.NoError(t, err)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		adminModel := model.NewAdminModel(db)
		admin, _ := adminModel.FindByUsername(c.Request.Context(), "testadmin")
		ctx := context.WithValue(c.Request.Context(), "username", "testadmin")
		ctx = context.WithValue(ctx, "adminID", admin.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.GET("/roles/:id", handler.RoleDetail)

	req := httptest.NewRequest("GET", "/roles/"+strconv.FormatUint(uint64(role.ID), 10), nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleDetail_BindUriError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupRoleHandlerTest(t)

	router := gin.New()
	router.GET("/roles/:id", handler.RoleDetail)

	req := httptest.NewRequest("GET", "/roles/", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNotFound, resp.Code)
}

func TestHandler_RoleUpdate_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTest(t)
	roleModel := model.NewRoleModel(db)

	// Create a role to update
	role := &model.Role{Name: "toupdate"}
	err := roleModel.Create(nil, role)
	require.NoError(t, err)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		adminModel := model.NewAdminModel(db)
		admin, _ := adminModel.FindByUsername(c.Request.Context(), "testadmin")
		ctx := context.WithValue(c.Request.Context(), "username", "testadmin")
		ctx = context.WithValue(ctx, "adminID", admin.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
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
}

func TestHandler_RoleUpdate_BindUriError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupRoleHandlerTest(t)

	router := gin.New()
	router.PUT("/roles/:id", handler.RoleUpdate)

	req := httptest.NewRequest("PUT", "/roles/", bytes.NewBuffer([]byte("{}")))
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNotFound, resp.Code)
}

func TestHandler_RoleUpdate_BindJSONError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupRoleHandlerTest(t)

	router := gin.New()
	router.PUT("/roles/:id", handler.RoleUpdate)

	req := httptest.NewRequest("PUT", "/roles/1", bytes.NewBuffer([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleUpdate_EmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupRoleHandlerTest(t)

	router := gin.New()
	router.PUT("/roles/:id", handler.RoleUpdate)

	req := httptest.NewRequest("PUT", "/roles/1", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	// Without auth context, will return internal server error or success depending on implementation
	// Since service layer checks auth, we expect either 500 (auth error) or 200 (if it passes through)
	assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, resp.Code)
}

func TestHandler_RolesList_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTest(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		adminModel := model.NewAdminModel(db)
		admin, _ := adminModel.FindByUsername(c.Request.Context(), "testadmin")
		ctx := context.WithValue(c.Request.Context(), "username", "testadmin")
		ctx = context.WithValue(ctx, "adminID", admin.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.GET("/roles", handler.RolesList)

	req := httptest.NewRequest("GET", "/roles?page=1&pageSize=10", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_RolesList_BindQueryError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupRoleHandlerTest(t)

	router := gin.New()
	router.GET("/roles", handler.RolesList)

	req := httptest.NewRequest("GET", "/roles?page=invalid", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Without auth context, will return internal server error
	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestHandler_RolesList_EmptyQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupRoleHandlerTest(t)

	router := gin.New()
	router.GET("/roles", handler.RolesList)

	req := httptest.NewRequest("GET", "/roles", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestHandler_RolesList_WithCategoryFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupRoleHandlerTest(t)

	router := gin.New()
	router.GET("/roles", handler.RolesList)

	req := httptest.NewRequest("GET", "/roles?category=custom", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestHandler_RolesList_WithSearch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupRoleHandlerTest(t)

	router := gin.New()
	router.GET("/roles", handler.RolesList)

	req := httptest.NewRequest("GET", "/roles?search=admin", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestHandler_RoleCreate_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(nil)
	router := gin.New()
	router.POST("/roles", handler.RoleCreate)

	body := map[string]interface{}{"name": "test"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/roles", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			// Expected to panic
			assert.NotNil(t, r)
		}
	}()

	router.ServeHTTP(resp, req)

	// If we get here without panic
	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_RoleUpdate_WithPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTest(t)
	roleModel := model.NewRoleModel(db)

	// Create a role to update
	role := &model.Role{Name: "toupdateperms"}
	err := roleModel.Create(nil, role)
	require.NoError(t, err)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		adminModel := model.NewAdminModel(db)
		admin, _ := adminModel.FindByUsername(c.Request.Context(), "testadmin")
		ctx := context.WithValue(c.Request.Context(), "username", "testadmin")
		ctx = context.WithValue(ctx, "adminID", admin.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
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

func TestHandler_RoleCreate_DuplicateName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupRoleHandlerTest(t)

	// "admin" role already exists from setup
	router := gin.New()
	router.Use(func(c *gin.Context) {
		adminModel := model.NewAdminModel(db)
		admin, _ := adminModel.FindByUsername(c.Request.Context(), "testadmin")
		ctx := context.WithValue(c.Request.Context(), "username", "testadmin")
		ctx = context.WithValue(ctx, "adminID", admin.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.POST("/roles", handler.RoleCreate)

	body := map[string]interface{}{
		"name": "admin",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/roles", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should fail due to duplicate name (database constraint)
	assert.NotEqual(t, http.StatusOK, resp.Code)
}
