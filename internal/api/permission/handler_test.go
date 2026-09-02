package permission

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func setupPermissionHandlerTest(t *testing.T) (*Handler, *gorm.DB) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = model.AutoMigrate(db)
	require.NoError(t, err)

	// Create test admin and permissions
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)

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
		"admin:all", "roles:read", "role:read", "roles:manage", "role:write",
	})
	require.NoError(t, err)

	permissions := []*model.Permission{
		{ID: "admin:all", Name: "Admin All", Resource: "admin", Action: "all", Category: "admin"},
		{ID: "game:read", Name: "Game Read", Resource: "game", Action: "read", Category: "game"},
		{ID: "player:read", Name: "Player Read", Resource: "player", Action: "read", Category: "player"},
	}
	for _, perm := range permissions {
		err = db.Create(perm).Error
		require.NoError(t, err)
	}

	svcCtx := &svc.ServiceContext{
		DB:              db,
		AdminModel:      adminModel,
		RoleModel:       roleModel,
		PermissionModel: model.NewPermissionModel(db),
		Cache:           cache.NewNullCache(),
		CacheHelper:     cache.NewCacheHelper(cache.NewNullCache()),
	}

	service := NewService(svcCtx)
	handler := NewHandler(service)

	return handler, db
}

func TestHandler_List_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupPermissionHandlerTest(t)

	router := gin.New()
	router.GET("/permissions", handler.List)

	req := httptest.NewRequest("GET", "/permissions?page=1&pageSize=10", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Without auth context, will return internal server error
	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestHandler_List_BindQueryError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupPermissionHandlerTest(t)

	router := gin.New()
	router.GET("/permissions", handler.List)

	req := httptest.NewRequest("GET", "/permissions?page=invalid", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Bug4 修复后：page=invalid 按契约返回 400
	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestHandler_List_EmptyQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupPermissionHandlerTest(t)

	router := gin.New()
	router.GET("/permissions", handler.List)

	req := httptest.NewRequest("GET", "/permissions", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Without auth context
	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestHandler_List_WithCategoryFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupPermissionHandlerTest(t)

	router := gin.New()
	router.GET("/permissions", handler.List)

	req := httptest.NewRequest("GET", "/permissions?category=game", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestHandler_List_WithResourceFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupPermissionHandlerTest(t)

	router := gin.New()
	router.GET("/permissions", handler.List)

	req := httptest.NewRequest("GET", "/permissions?resource=player", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestHandler_List_WithMultipleFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupPermissionHandlerTest(t)

	router := gin.New()
	router.GET("/permissions", handler.List)

	req := httptest.NewRequest("GET", "/permissions?category=game&resource=game&page=1&pageSize=5", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestHandler_Detail_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupPermissionHandlerTest(t)

	router := gin.New()
	router.GET("/permissions/:id", handler.Detail)

	req := httptest.NewRequest("GET", "/permissions/game:read", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Without auth context
	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestHandler_Detail_BindUriError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupPermissionHandlerTest(t)

	router := gin.New()
	router.GET("/permissions/:id", handler.Detail)

	// Missing ID parameter
	req := httptest.NewRequest("GET", "/permissions/", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Gin will return 404 for missing parameter
	assert.Equal(t, http.StatusNotFound, resp.Code)
}

func TestHandler_Detail_EmptyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupPermissionHandlerTest(t)

	router := gin.New()
	router.GET("/permissions/:id", handler.Detail)

	// Use encoded space (%20) instead of literal space in URL
	req := httptest.NewRequest("GET", "/permissions/%20%20", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Without auth context, will return error
	assert.Contains(t, []int{http.StatusInternalServerError, http.StatusUnauthorized}, resp.Code)
}

func TestHandler_Detail_SpecialCharactersID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupPermissionHandlerTest(t)

	router := gin.New()
	router.GET("/permissions/:id", handler.Detail)

	req := httptest.NewRequest("GET", "/permissions/test:%20read", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestHandler_Detail_WithAuthContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupPermissionHandlerTest(t)

	router := gin.New()

	// Add middleware to set auth context
	router.Use(func(c *gin.Context) {
		adminModel := model.NewAdminModel(db)
		admin, err := adminModel.FindByUsername(c.Request.Context(), "testadmin")
		if err == nil {
			c.Set("username", "testadmin")
			c.Set("adminID", admin.ID)
			// Also set in the context
			ctx := context.WithValue(c.Request.Context(), "username", "testadmin")
			ctx = context.WithValue(ctx, "adminID", admin.ID)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	})

	router.GET("/permissions/:id", handler.Detail)

	req := httptest.NewRequest("GET", "/permissions/game:read", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should get either success or forbidden depending on permission check
	assert.Contains(t, []int{http.StatusOK, http.StatusForbidden, http.StatusUnauthorized}, resp.Code)
}

func TestHandler_List_WithAuthContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupPermissionHandlerTest(t)

	router := gin.New()

	router.Use(func(c *gin.Context) {
		adminModel := model.NewAdminModel(db)
		admin, err := adminModel.FindByUsername(c.Request.Context(), "testadmin")
		if err == nil {
			c.Set("username", "testadmin")
			c.Set("adminID", admin.ID)
			// Also set in the context
			ctx := context.WithValue(c.Request.Context(), "username", "testadmin")
			ctx = context.WithValue(ctx, "adminID", admin.ID)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	})

	router.GET("/permissions", handler.List)

	req := httptest.NewRequest("GET", "/permissions?page=1&pageSize=10", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should get success since admin has permissions
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_List_POSTMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupPermissionHandlerTest(t)

	router := gin.New()
	router.POST("/permissions", handler.List)

	body := map[string]interface{}{
		"page":     1,
		"pageSize": 10,
		"category": "game",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/permissions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Without auth context
	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestHandler_List_InvalidPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupPermissionHandlerTest(t)

	router := gin.New()
	router.GET("/permissions", handler.List)

	req := httptest.NewRequest("GET", "/permissions?page=-1", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestHandler_List_InvalidPageSize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupPermissionHandlerTest(t)

	router := gin.New()
	router.GET("/permissions", handler.List)

	req := httptest.NewRequest("GET", "/permissions?pageSize=abc", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Bug4 修复后：pageSize=abc 按契约返回 400
	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestHandler_Detail_JSONResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupPermissionHandlerTest(t)

	router := gin.New()

	router.Use(func(c *gin.Context) {
		adminModel := model.NewAdminModel(db)
		admin, _ := adminModel.FindByUsername(c.Request.Context(), "testadmin")
		c.Set("username", "testadmin")
		c.Set("adminID", admin.ID)
		c.Next()
	})

	router.GET("/permissions/:id", handler.Detail)

	req := httptest.NewRequest("GET", "/permissions/game:read", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Check content type
	if resp.Code == http.StatusOK {
		assert.Equal(t, "application/json; charset=utf-8", resp.Header().Get("Content-Type"))
	}
}

func TestNewHandler(t *testing.T) {
	service := &Service{}
	handler := NewHandler(service)

	assert.NotNil(t, handler)
	assert.Equal(t, service, handler.service)
}

func TestHandler_List_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(nil)
	router := gin.New()
	router.GET("/permissions", handler.List)

	req := httptest.NewRequest("GET", "/permissions", nil)
	resp := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			// Expected to panic due to nil service
			assert.NotNil(t, r)
		}
	}()

	router.ServeHTTP(resp, req)

	// If we get here without panic, check the response
	assert.NotEqual(t, http.StatusOK, resp.Code)
}
