package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/service/permission"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func assertAdminStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("expected status %d, got %d body=%s", want, rec.Code, rec.Body.String())
	}
}

func assertAdminRejected(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code == http.StatusOK {
		t.Fatalf("expected request to be rejected, got 200 body=%s", rec.Body.String())
	}
}

func newAdminTestContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	return ctx, rec
}

func TestHandler_List_BindValidation(t *testing.T) {

	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name     string
		query    string
		wantCode int
	}{
		{
			name:     "default parameters",
			query:    "",
			wantCode: 0, // Will fail at service layer
		},
		{
			name:     "with page",
			query:    "?page=2",
			wantCode: 0,
		},
		{
			name:     "with pageSize",
			query:    "?pageSize=10",
			wantCode: 0,
		},
		{
			name:     "with search",
			query:    "?search=admin",
			wantCode: 0,
		},
		{
			name:     "with role filter",
			query:    "?role=admin",
			wantCode: 0,
		},
		{
			name:     "with status filter",
			query:    "?status=1",
			wantCode: 0,
		},
		{
			name:     "all filters combined",
			query:    "?page=1&pageSize=10&search=test&role=admin&status=1",
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx, rec := newAdminTestContext("GET", "/admins"+tt.query, "")
			handler.List(ctx)

			if tt.wantCode == 0 {
				if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusUnauthorized && rec.Code != http.StatusOK {
					t.Errorf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
				}
			}
		})
	}
}

func TestHandler_Create_BindValidation(t *testing.T) {

	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name     string
		body     string
		wantCode int
	}{
		{
			name:     "valid create request",
			body:     `{"username":"newadmin","password":"MyPass123","nickname":"New Admin","email":"new@example.com"}`,
			wantCode: 0, // Will fail at service layer
		},
		{
			name:     "missing username",
			body:     `{"password":"MyPass123","nickname":"New Admin"}`,
			wantCode: 400,
		},
		{
			name:     "missing password",
			body:     `{"username":"newadmin","nickname":"New Admin"}`,
			wantCode: 400,
		},
		{
			name:     "empty json",
			body:     `{}`,
			wantCode: 400,
		},
		{
			name:     "invalid json",
			body:     `invalid`,
			wantCode: 400,
		},
		{
			name:     "with roles",
			body:     `{"username":"newadmin","password":"MyPass123","roles":["admin","editor"]}`,
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx, rec := newAdminTestContext("POST", "/admins", tt.body)
			handler.Create(ctx)

			if tt.wantCode != 0 {
				assertAdminRejected(t, rec)
			}
		})
	}
}

func TestHandler_Get_BindValidation(t *testing.T) {

	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name     string
		uri      string
		wantCode int
	}{
		{
			name:     "valid numeric id",
			uri:      "/admins/123",
			wantCode: 0, // Will fail at service layer
		},
		{
			name:     "empty id",
			uri:      "/admins/",
			wantCode: 400,
		},
		{
			name:     "non-numeric id",
			uri:      "/admins/abc",
			wantCode: 400,
		},
		{
			name:     "zero id",
			uri:      "/admins/0",
			wantCode: 400,
		},
		{
			name:     "negative id",
			uri:      "/admins/-1",
			wantCode: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx, rec := newAdminTestContext("GET", tt.uri, "")
			ctx.Params = gin.Params{gin.Param{Key: "id", Value: strings.TrimPrefix(tt.uri, "/admins/")}}
			handler.Get(ctx)

			if tt.wantCode != 0 {
				assertAdminRejected(t, rec)
			}
		})
	}
}

func TestHandler_Update_BindValidation(t *testing.T) {

	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name     string
		uri      string
		body     string
		wantCode int
	}{
		{
			name:     "valid update request",
			uri:      "/admins/123",
			body:     `{"nickname":"Updated Name","email":"updated@example.com","status":1}`,
			wantCode: 0, // Will fail at service layer
		},
		{
			name:     "empty roles array",
			uri:      "/admins/123",
			body:     `{"roles":[]}`,
			wantCode: 0,
		},
		{
			name:     "with roles",
			uri:      "/admins/123",
			body:     `{"roles":["admin","viewer"]}`,
			wantCode: 0,
		},
		{
			name:     "invalid id in uri",
			uri:      "/admins/abc",
			body:     `{"nickname":"Updated"}`,
			wantCode: 400,
		},
		{
			name:     "invalid json",
			uri:      "/admins/123",
			body:     `invalid`,
			wantCode: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx, rec := newAdminTestContext("PUT", tt.uri, tt.body)
			ctx.Params = gin.Params{gin.Param{Key: "id", Value: strings.TrimPrefix(tt.uri, "/admins/")}}
			handler.Update(ctx)

			if tt.wantCode != 0 {
				assertAdminRejected(t, rec)
			}
		})
	}
}

func TestHandler_Delete_BindValidation(t *testing.T) {

	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name     string
		uri      string
		wantCode int
	}{
		{
			name:     "valid numeric id",
			uri:      "/admins/123",
			wantCode: 0, // Will fail at service layer
		},
		{
			name:     "empty id",
			uri:      "/admins/",
			wantCode: 400,
		},
		{
			name:     "non-numeric id",
			uri:      "/admins/abc",
			wantCode: 400,
		},
		{
			name:     "zero id",
			uri:      "/admins/0",
			wantCode: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx, rec := newAdminTestContext("DELETE", tt.uri, "")
			ctx.Params = gin.Params{gin.Param{Key: "id", Value: strings.TrimPrefix(tt.uri, "/admins/")}}
			handler.Delete(ctx)

			if tt.wantCode != 0 {
				assertAdminRejected(t, rec)
			}
		})
	}
}

func TestHandler_PasswordReset_BindValidation(t *testing.T) {

	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name     string
		uri      string
		body     string
		wantCode int
	}{
		{
			name:     "valid password reset",
			uri:      "/admins/123/password-reset",
			body:     `{"newPassword":"newPassword123"}`,
			wantCode: 0, // Will fail at service layer
		},
		{
			name:     "missing new password",
			uri:      "/admins/123/password-reset",
			body:     `{}`,
			wantCode: 400,
		},
		{
			name:     "invalid id",
			uri:      "/admins/abc/password-reset",
			body:     `{"newPassword":"newPassword123"}`,
			wantCode: 400,
		},
		{
			name:     "invalid json",
			uri:      "/admins/123/password-reset",
			body:     `invalid`,
			wantCode: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx, rec := newAdminTestContext("POST", tt.uri, tt.body)
			ctx.Params = gin.Params{gin.Param{Key: "id", Value: strings.TrimPrefix(strings.TrimSuffix(tt.uri, "/password-reset"), "/admins/")}}
			handler.PasswordReset(ctx)

			if tt.wantCode != 0 {
				assertAdminRejected(t, rec)
			}
		})
	}
}

func TestHandler_GetGames_BindValidation(t *testing.T) {

	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name     string
		uri      string
		wantCode int
	}{
		{
			name:     "valid id",
			uri:      "/admins/123/games",
			wantCode: 0, // Will fail at service layer
		},
		{
			name:     "empty id",
			uri:      "/admins//games",
			wantCode: 400,
		},
		{
			name:     "non-numeric id",
			uri:      "/admins/abc/games",
			wantCode: 400,
		},
		{
			name:     "zero id",
			uri:      "/admins/0/games",
			wantCode: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx, rec := newAdminTestContext("GET", tt.uri, "")
			ctx.Params = gin.Params{gin.Param{Key: "id", Value: strings.TrimSuffix(strings.TrimPrefix(tt.uri, "/admins/"), "/games")}}
			handler.GetGames(ctx)

			if tt.wantCode != 0 {
				assertAdminRejected(t, rec)
			}
		})
	}
}

func TestHandler_UpdateGames_BindValidation(t *testing.T) {

	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name     string
		uri      string
		body     string
		wantCode int
	}{
		{
			name:     "valid update",
			uri:      "/admins/123/games",
			body:     `{"games":[{"gameId":"test","envs":["prod","dev"]}]}`,
			wantCode: 0, // Will fail at service layer
		},
		{
			name:     "empty games array",
			uri:      "/admins/123/games",
			body:     `{"games":[]}`,
			wantCode: 0,
		},
		{
			name:     "invalid id",
			uri:      "/admins/abc/games",
			body:     `{"games":[]}`,
			wantCode: 400,
		},
		{
			name:     "invalid json",
			uri:      "/admins/123/games",
			body:     `invalid`,
			wantCode: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx, rec := newAdminTestContext("PUT", tt.uri, tt.body)
			ctx.Params = gin.Params{gin.Param{Key: "id", Value: strings.TrimSuffix(strings.TrimPrefix(tt.uri, "/admins/"), "/games")}}
			handler.UpdateGames(ctx)

			if tt.wantCode != 0 {
				assertAdminRejected(t, rec)
			}
		})
	}
}

// Handler integration tests with proper router setup

func setupAdminHandlerTest(t *testing.T) (*Handler, *gorm.DB) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = model.AutoMigrate(db)
	require.NoError(t, err)

	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)
	permissionModel := model.NewPermissionModel(db)
	gameModel := model.NewGameModel(db)

	// Create admin
	admin := &model.Admin{Username: "testadmin", Nickname: "Test Admin", Status: 1}
	err = adminModel.Create(nil, admin, "password123")
	require.NoError(t, err)

	role := &model.Role{Name: "admin"}
	err = roleModel.Create(nil, role)
	require.NoError(t, err)

	err = adminModel.AssignRole(nil, admin.ID, role.ID)
	require.NoError(t, err)

	err = roleModel.ReplacePermissions(nil, role.ID, []string{"admin:all", "user:read", "user:write"})
	require.NoError(t, err)

	permissions := []*model.Permission{
		{ID: "admin:all", Name: "Admin All", Resource: "admin", Action: "all", Category: "admin"},
		{ID: "user:read", Name: "User Read", Resource: "user", Action: "read", Category: "user"},
		{ID: "user:write", Name: "User Write", Resource: "user", Action: "write", Category: "user"},
	}
	for _, perm := range permissions {
		err = db.Create(perm).Error
		require.NoError(t, err)
	}

	permSvc := permission.NewPermissionService(db)
	nullCache := cache.NewNullCache()
	cacheHelper := cache.NewCacheHelper(nullCache)

	svcCtx := &svc.ServiceContext{
		DB:                db,
		AdminModel:        adminModel,
		RoleModel:         roleModel,
		PermissionModel:   permissionModel,
		GameModel:         gameModel,
		PermissionService: permSvc,
		Cache:             nullCache,
		CacheHelper:       cacheHelper,
	}

	service := NewService(svcCtx)
	handler := NewHandler(service)

	return handler, db
}

func addAdminAuthMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		adminModel := model.NewAdminModel(db)
		admin, _ := adminModel.FindByUsername(c.Request.Context(), "testadmin")
		ctx := context.WithValue(c.Request.Context(), "username", "testadmin")
		ctx = context.WithValue(ctx, "adminID", admin.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func TestHandler_List_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAdminHandlerTest(t)

	router := gin.New()
	router.Use(addAdminAuthMiddleware(db))
	router.GET("/admins", handler.List)

	req := httptest.NewRequest("GET", "/admins?page=1&pageSize=10", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Get_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAdminHandlerTest(t)

	adminModel := model.NewAdminModel(db)
	admin, err := adminModel.FindByUsername(nil, "testadmin")
	require.NoError(t, err)

	router := gin.New()
	router.Use(addAdminAuthMiddleware(db))
	router.GET("/admins/:id", handler.Get)

	req := httptest.NewRequest("GET", "/admins/"+strconv.FormatUint(uint64(admin.ID), 10), nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Create_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAdminHandlerTest(t)

	router := gin.New()
	router.Use(addAdminAuthMiddleware(db))
	router.POST("/admins", handler.Create)

	body := `{"username":"newadmin","password":"MyPass123","nickname":"New Admin"}`
	req := httptest.NewRequest("POST", "/admins", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Delete_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAdminHandlerTest(t)

	// Create an admin to delete
	adminModel := model.NewAdminModel(db)
	admin := &model.Admin{Username: "todelete", Nickname: "To Delete", Status: 1}
	err := adminModel.Create(nil, admin, "password123")
	require.NoError(t, err)

	router := gin.New()
	router.Use(addAdminAuthMiddleware(db))
	router.DELETE("/admins/:id", handler.Delete)

	req := httptest.NewRequest("DELETE", "/admins/"+strconv.FormatUint(uint64(admin.ID), 10), nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_PasswordReset_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAdminHandlerTest(t)

	adminModel := model.NewAdminModel(db)
	admin, err := adminModel.FindByUsername(nil, "testadmin")
	require.NoError(t, err)

	router := gin.New()
	router.Use(addAdminAuthMiddleware(db))
	router.POST("/admins/:id/password-reset", handler.PasswordReset)

	body := `{"newPassword":"newPassword456"}`
	req := httptest.NewRequest("POST", "/admins/"+strconv.FormatUint(uint64(admin.ID), 10)+"/password-reset", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_GetGames_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAdminHandlerTest(t)

	adminModel := model.NewAdminModel(db)
	admin, err := adminModel.FindByUsername(nil, "testadmin")
	require.NoError(t, err)

	router := gin.New()
	router.Use(addAdminAuthMiddleware(db))
	router.GET("/admins/:id/games", handler.GetGames)

	req := httptest.NewRequest("GET", "/admins/"+strconv.FormatUint(uint64(admin.ID), 10)+"/games", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_UpdateGames_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAdminHandlerTest(t)

	adminModel := model.NewAdminModel(db)
	admin, err := adminModel.FindByUsername(nil, "testadmin")
	require.NoError(t, err)

	router := gin.New()
	router.Use(addAdminAuthMiddleware(db))
	router.PUT("/admins/:id/games", handler.UpdateGames)

	body := `{"games":[]}`
	req := httptest.NewRequest("PUT", "/admins/"+strconv.FormatUint(uint64(admin.ID), 10)+"/games", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Update_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAdminHandlerTest(t)

	adminModel := model.NewAdminModel(db)
	admin, err := adminModel.FindByUsername(nil, "testadmin")
	require.NoError(t, err)

	router := gin.New()
	router.Use(addAdminAuthMiddleware(db))
	router.PUT("/admins/:id", handler.Update)

	body := `{"nickname":"Updated Nickname"}`
	req := httptest.NewRequest("PUT", "/admins/"+strconv.FormatUint(uint64(admin.ID), 10), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

// Service error path tests via router

func TestHandler_Get_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAdminHandlerTest(t)

	router := gin.New()
	router.Use(addAdminAuthMiddleware(db))
	router.GET("/admins/:id", handler.Get)

	req := httptest.NewRequest("GET", "/admins/99999", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assertAdminRejected(t, resp)
}

func TestHandler_Update_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAdminHandlerTest(t)

	router := gin.New()
	router.Use(addAdminAuthMiddleware(db))
	router.PUT("/admins/:id", handler.Update)

	body := `{"nickname":"Updated"}`
	req := httptest.NewRequest("PUT", "/admins/99999", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assertAdminRejected(t, resp)
}

func TestHandler_Delete_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAdminHandlerTest(t)

	router := gin.New()
	router.Use(addAdminAuthMiddleware(db))
	router.DELETE("/admins/:id", handler.Delete)

	req := httptest.NewRequest("DELETE", "/admins/99999", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assertAdminRejected(t, resp)
}

func TestHandler_PasswordReset_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAdminHandlerTest(t)

	router := gin.New()
	router.Use(addAdminAuthMiddleware(db))
	router.POST("/admins/:id/password-reset", handler.PasswordReset)

	body := `{"newPassword":"newPassword456"}`
	req := httptest.NewRequest("POST", "/admins/99999/password-reset", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assertAdminRejected(t, resp)
}

func TestHandler_Create_InvalidJSON_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAdminHandlerTest(t)

	router := gin.New()
	router.Use(addAdminAuthMiddleware(db))
	router.POST("/admins", handler.Create)

	req := httptest.NewRequest("POST", "/admins", bytes.NewBufferString("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assertAdminRejected(t, resp)
}

func TestHandler_Update_InvalidJSON_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAdminHandlerTest(t)

	adminModel := model.NewAdminModel(db)
	admin, err := adminModel.FindByUsername(nil, "testadmin")
	require.NoError(t, err)

	router := gin.New()
	router.Use(addAdminAuthMiddleware(db))
	router.PUT("/admins/:id", handler.Update)

	req := httptest.NewRequest("PUT", "/admins/"+strconv.FormatUint(uint64(admin.ID), 10), bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assertAdminRejected(t, resp)
}

func TestHandler_PasswordReset_InvalidJSON_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAdminHandlerTest(t)

	adminModel := model.NewAdminModel(db)
	admin, err := adminModel.FindByUsername(nil, "testadmin")
	require.NoError(t, err)

	router := gin.New()
	router.Use(addAdminAuthMiddleware(db))
	router.POST("/admins/:id/password-reset", handler.PasswordReset)

	req := httptest.NewRequest("POST", "/admins/"+strconv.FormatUint(uint64(admin.ID), 10)+"/password-reset", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assertAdminRejected(t, resp)
}

func TestHandler_UpdateGames_InvalidJSON_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAdminHandlerTest(t)

	adminModel := model.NewAdminModel(db)
	admin, err := adminModel.FindByUsername(nil, "testadmin")
	require.NoError(t, err)

	router := gin.New()
	router.Use(addAdminAuthMiddleware(db))
	router.PUT("/admins/:id/games", handler.UpdateGames)

	req := httptest.NewRequest("PUT", "/admins/"+strconv.FormatUint(uint64(admin.ID), 10)+"/games", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assertAdminRejected(t, resp)
}

func TestHandler_Create_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupAdminHandlerTest(t)

	router := gin.New()
	router.Use(addAdminAuthMiddleware(db))
	router.POST("/admins", handler.Create)

	// Duplicate username
	body := `{"username":"testadmin","password":"MyPass123","nickname":"Dup"}`
	req := httptest.NewRequest("POST", "/admins", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assertAdminRejected(t, resp)
}
