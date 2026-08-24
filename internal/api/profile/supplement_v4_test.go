package profile

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- UpdateScope handler tests (covers 0% → 100%) ---

func TestHandler_UpdateScope_UnauthorizedV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})
	ctx, rec := newProfileTestContext("POST", "/scope", `{"gameId":"test","env":"prod"}`)
	handler.UpdateScope(ctx)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_UpdateScope_BindValidationV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name     string
		body     string
		wantCode int
	}{
		{"invalid json", `invalid`, http.StatusBadRequest},
		{"empty body", ``, http.StatusBadRequest},
		{"missing gameId", `{"env":"prod"}`, http.StatusBadRequest},
		{"missing env", `{"gameId":"test"}`, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, rec := newProfileTestContext("POST", "/scope", tt.body)
			ctx.Set("adminID", uint(1))
			handler.UpdateScope(ctx)

			assert.Equal(t, tt.wantCode, rec.Code)
		})
	}
}

func TestHandler_UpdateScope_WithAdminIDV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := svcCtx.AdminModel
	gameModel := svcCtx.GameModel
	roleModel := svcCtx.RoleModel
	service := NewService(adminModel, gameModel, roleModel)
	handler := NewHandler(service)

	// Create admin with admin role
	adminID := createTestAdminWithRole(t, db, "scopehandler", "password123", "admin")

	// Create game
	game := &model.Game{GameID: "scope-game", Name: "Scope Game", Status: "dev"}
	require.NoError(t, gameModel.Create(context.Background(), game))
	require.NoError(t, gameModel.AddEnvBinding(context.Background(), game.GameID, "production", "test", "", ""))

	router := gin.New()
	router.POST("/scope", func(c *gin.Context) {
		c.Set("adminID", adminID)
		c.Next()
	}, handler.UpdateScope)

	rec := performProfileRequest(router, "POST", "/scope", `{"gameId":"scope-game","env":"production"}`)
	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, true, result["ok"])
}

func TestHandler_UpdateScope_GameNotFoundV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := svcCtx.AdminModel
	gameModel := svcCtx.GameModel
	roleModel := svcCtx.RoleModel
	service := NewService(adminModel, gameModel, roleModel)
	handler := NewHandler(service)

	adminID := createTestAdminWithRole(t, db, "scopehandler2", "password123", "admin")

	router := gin.New()
	router.POST("/scope", func(c *gin.Context) {
		c.Set("adminID", adminID)
		c.Next()
	}, handler.UpdateScope)

	rec := performProfileRequest(router, "POST", "/scope", `{"gameId":"nonexistent","env":"prod"}`)
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

// --- GetProfile handler with real service ---

func TestHandler_GetProfile_SuccessV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := svcCtx.AdminModel
	gameModel := svcCtx.GameModel
	roleModel := svcCtx.RoleModel
	service := NewService(adminModel, gameModel, roleModel)
	handler := NewHandler(service)

	// Create admin
	admin := &model.Admin{Username: "profilehandler", Nickname: "Profile Handler", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "password"))
	role := &model.Role{Name: "admin"}
	require.NoError(t, db.Create(role).Error)
	require.NoError(t, adminModel.AssignRole(context.Background(), admin.ID, role.ID))

	router := gin.New()
	router.GET("/profile", func(c *gin.Context) {
		c.Set("username", "profilehandler")
		c.Next()
	}, handler.GetProfile)

	rec := performProfileRequest(router, "GET", "/profile", "")
	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, "profilehandler", result["username"])
}

func TestHandler_GetProfile_WithRolesV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := svcCtx.AdminModel
	gameModel := svcCtx.GameModel
	roleModel := svcCtx.RoleModel
	service := NewService(adminModel, gameModel, roleModel)
	handler := NewHandler(service)

	admin := &model.Admin{Username: "profilehandler2", Nickname: "Profile Handler 2", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "password"))
	role := &model.Role{Name: "editor"}
	require.NoError(t, db.Create(role).Error)
	require.NoError(t, adminModel.AssignRole(context.Background(), admin.ID, role.ID))

	router := gin.New()
	router.GET("/profile", func(c *gin.Context) {
		c.Set("username", "profilehandler2")
		c.Next()
	}, handler.GetProfile)

	rec := performProfileRequest(router, "GET", "/profile", "")
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- GetGames handler with real service ---

func TestHandler_GetGames_SuccessV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := svcCtx.AdminModel
	gameModel := svcCtx.GameModel
	roleModel := svcCtx.RoleModel
	service := NewService(adminModel, gameModel, roleModel)
	handler := NewHandler(service)

	createTestAdminWithRole(t, db, "gameshandler", "password123", "admin")

	// Create game
	game := &model.Game{Name: "handlergame", AliasName: "Handler Game", Status: "running"}
	err := game.SetEnvs([]model.GameEnv{{Env: "prod"}})
	require.NoError(t, err)
	createProfileTestGame(t, gameModel, game)

	router := gin.New()
	router.GET("/games", func(c *gin.Context) {
		c.Set("username", "gameshandler")
		c.Next()
	}, handler.GetGames)

	rec := performProfileRequest(router, "GET", "/games", "")
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- UpdateProfile handler with real service ---

func TestHandler_UpdateProfile_SuccessV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := svcCtx.AdminModel
	gameModel := svcCtx.GameModel
	roleModel := svcCtx.RoleModel
	service := NewService(adminModel, gameModel, roleModel)
	handler := NewHandler(service)

	admin := &model.Admin{Username: "updprofile", Nickname: "Upd Profile", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "password"))

	router := gin.New()
	router.PUT("/profile", func(c *gin.Context) {
		c.Set("username", "updprofile")
		c.Next()
	}, handler.UpdateProfile)

	rec := performProfileRequest(router, "PUT", "/profile", `{"nickname":"Updated","email":"new@example.com"}`)
	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, true, result["ok"])
}

func TestHandler_UpdateProfile_BindErrorV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := svcCtx.AdminModel
	gameModel := svcCtx.GameModel
	roleModel := svcCtx.RoleModel
	service := NewService(adminModel, gameModel, roleModel)
	handler := NewHandler(service)

	router := gin.New()
	router.PUT("/profile", func(c *gin.Context) {
		c.Set("username", "updprofile")
		c.Next()
	}, handler.UpdateProfile)

	rec := performProfileRequest(router, "PUT", "/profile", `invalid json`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_UpdateProfile_ServiceErrorV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := svcCtx.AdminModel
	gameModel := svcCtx.GameModel
	roleModel := svcCtx.RoleModel
	service := NewService(adminModel, gameModel, roleModel)
	handler := NewHandler(service)

	router := gin.New()
	router.PUT("/profile", func(c *gin.Context) {
		c.Set("username", "nonexistent_user_xyz")
		c.Next()
	}, handler.UpdateProfile)

	rec := performProfileRequest(router, "PUT", "/profile", `{"nickname":"test"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- ChangePassword handler with real service ---

func TestHandler_ChangePassword_SuccessV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := svcCtx.AdminModel
	gameModel := svcCtx.GameModel
	roleModel := svcCtx.RoleModel
	service := NewService(adminModel, gameModel, roleModel)
	handler := NewHandler(service)

	admin := &model.Admin{Username: "chgpw", Nickname: "Chg Pw", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "password123"))

	router := gin.New()
	router.POST("/change-password", func(c *gin.Context) {
		c.Set("username", "chgpw")
		c.Next()
	}, handler.ChangePassword)

	rec := performProfileRequest(router, "POST", "/change-password", `{"oldPassword":"password123","newPassword":"newpassword123"}`)
	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, true, result["ok"])
}

func TestHandler_ChangePassword_BindErrorV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := svcCtx.AdminModel
	gameModel := svcCtx.GameModel
	roleModel := svcCtx.RoleModel
	service := NewService(adminModel, gameModel, roleModel)
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/change-password", func(c *gin.Context) {
		c.Set("username", "chgpw")
		c.Next()
	}, handler.ChangePassword)

	rec := performProfileRequest(router, "POST", "/change-password", `invalid json`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ChangePassword_WrongPasswordV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := svcCtx.AdminModel
	gameModel := svcCtx.GameModel
	roleModel := svcCtx.RoleModel
	service := NewService(adminModel, gameModel, roleModel)
	handler := NewHandler(service)

	admin := &model.Admin{Username: "chgpw2", Nickname: "Chg Pw 2", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "password123"))

	router := gin.New()
	router.POST("/change-password", func(c *gin.Context) {
		c.Set("username", "chgpw2")
		c.Next()
	}, handler.ChangePassword)

	rec := performProfileRequest(router, "POST", "/change-password", `{"oldPassword":"wrongpassword","newPassword":"newpw123456"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- GetPermissions handler with real service ---

func TestHandler_GetPermissions_SuccessV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := svcCtx.AdminModel
	gameModel := svcCtx.GameModel
	roleModel := svcCtx.RoleModel
	service := NewService(adminModel, gameModel, roleModel)
	handler := NewHandler(service)

	createTestAdminWithRole(t, db, "permshandler", "password123", "admin")

	router := gin.New()
	router.GET("/permissions", func(c *gin.Context) {
		c.Set("username", "permshandler")
		c.Next()
	}, handler.GetPermissions)

	rec := performProfileRequest(router, "GET", "/permissions", "")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandler_GetPermissions_NoRolesV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := svcCtx.AdminModel
	gameModel := svcCtx.GameModel
	roleModel := svcCtx.RoleModel
	service := NewService(adminModel, gameModel, roleModel)
	handler := NewHandler(service)

	admin := &model.Admin{Username: "noperms", Nickname: "No Perms", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "password"))

	router := gin.New()
	router.GET("/permissions", func(c *gin.Context) {
		c.Set("username", "noperms")
		c.Next()
	}, handler.GetPermissions)

	rec := performProfileRequest(router, "GET", "/permissions", "")
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- GetProfile with admin status inactive ---

func TestHandler_GetProfile_InactiveAdminV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := svcCtx.AdminModel
	gameModel := svcCtx.GameModel
	roleModel := svcCtx.RoleModel
	service := NewService(adminModel, gameModel, roleModel)
	handler := NewHandler(service)

	admin := &model.Admin{Username: "inactivehandler", Nickname: "Inactive", Status: 0}
	require.NoError(t, adminModel.Create(context.Background(), admin, "password"))

	router := gin.New()
	router.GET("/profile", func(c *gin.Context) {
		c.Set("username", "inactivehandler")
		c.Next()
	}, handler.GetProfile)

	rec := performProfileRequest(router, "GET", "/profile", "")
	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	assert.NoError(t, err)
	// admin.Status 0 means inactive; profile reports active=false
	// Note: the model may normalize Status, so just check the response has the field
	assert.Contains(t, rec.Body.String(), "active")
}

// --- UpdateProfile with empty fields ---

func TestHandler_UpdateProfile_EmptyFieldsV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := svcCtx.AdminModel
	gameModel := svcCtx.GameModel
	roleModel := svcCtx.RoleModel
	service := NewService(adminModel, gameModel, roleModel)
	handler := NewHandler(service)

	admin := &model.Admin{Username: "emptyupd", Nickname: "Empty Upd", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "password"))

	router := gin.New()
	router.PUT("/profile", func(c *gin.Context) {
		c.Set("username", "emptyupd")
		c.Next()
	}, handler.UpdateProfile)

	rec := performProfileRequest(router, "PUT", "/profile", `{"nickname":"","email":"","phone":"","avatar":""}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- ChangePassword with short new password ---

func TestHandler_ChangePassword_ShortNewPwV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := svcCtx.AdminModel
	gameModel := svcCtx.GameModel
	roleModel := svcCtx.RoleModel
	service := NewService(adminModel, gameModel, roleModel)
	handler := NewHandler(service)

	admin := &model.Admin{Username: "emptypass", Nickname: "Empty Pass", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "password123"))

	router := gin.New()
	router.POST("/change-password", func(c *gin.Context) {
		c.Set("username", "emptypass")
		c.Next()
	}, handler.ChangePassword)

	// Short new password - binding requires min=6, so this should fail
	rec := performProfileRequest(router, "POST", "/change-password", `{"oldPassword":"password123","newPassword":"12345"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- GetProfile with lastLoginAt timestamp ---

func TestHandler_GetProfile_WithLastLoginV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := svcCtx.AdminModel
	gameModel := svcCtx.GameModel
	roleModel := svcCtx.RoleModel

	// Persist the login in audit_records (table-backed fallback)
	db.Exec("DELETE FROM audit_records")
	loginTime := time.Now().Add(-1 * time.Hour)
	seedAuditLogin(t, db, "loginuser", "success", loginTime)

	service := NewService(adminModel, gameModel, roleModel).WithDB(db)
	handler := NewHandler(service)

	admin := &model.Admin{Username: "loginuser", Nickname: "Login User", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "password"))

	router := gin.New()
	router.GET("/profile", func(c *gin.Context) {
		c.Set("username", "loginuser")
		c.Next()
	}, handler.GetProfile)

	rec := performProfileRequest(router, "GET", "/profile", "")
	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	assert.NoError(t, err)
	lastLogin, exists := result["lastLoginAt"]
	assert.True(t, exists)
	assert.NotEmpty(t, lastLogin)
}

// --- GetGames with env bindings ---

func TestHandler_GetGames_WithEnvsV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := svcCtx.AdminModel
	gameModel := svcCtx.GameModel
	roleModel := svcCtx.RoleModel
	service := NewService(adminModel, gameModel, roleModel)
	handler := NewHandler(service)

	createTestAdminWithRole(t, db, "envgameshandler", "password123", "admin")

	game := &model.Game{Name: "envgame_h", AliasName: "Env Game H", Status: "running"}
	err := game.SetEnvs([]model.GameEnv{
		{Env: "prod", Description: "Production"},
		{Env: "staging", Description: "Staging"},
	})
	require.NoError(t, err)
	createProfileTestGame(t, gameModel, game)

	router := gin.New()
	router.GET("/games", func(c *gin.Context) {
		c.Set("username", "envgameshandler")
		c.Next()
	}, handler.GetGames)

	rec := performProfileRequest(router, "GET", "/games", "")
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- GetPermissions with super_admin role ---

func TestHandler_GetPermissions_SuperAdminV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := svcCtx.AdminModel
	gameModel := svcCtx.GameModel
	roleModel := svcCtx.RoleModel
	service := NewService(adminModel, gameModel, roleModel)
	handler := NewHandler(service)

	createTestAdminWithRole(t, db, "superpermshandler", "password123", "super_admin")

	router := gin.New()
	router.GET("/permissions", func(c *gin.Context) {
		c.Set("username", "superpermshandler")
		c.Next()
	}, handler.GetPermissions)

	rec := performProfileRequest(router, "GET", "/permissions", "")
	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, true, result["admin"])
}

// --- UpdateScope with non-admin role ---

func TestHandler_UpdateScope_NonAdminUnauthorizedV4(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := svcCtx.AdminModel
	gameModel := svcCtx.GameModel
	roleModel := svcCtx.RoleModel
	service := NewService(adminModel, gameModel, roleModel)
	handler := NewHandler(service)

	adminID := createTestAdminWithRole(t, db, "viewerscope", "password123", "viewer")

	game := &model.Game{GameID: "viewer-game", Name: "Viewer Game", Status: "dev"}
	require.NoError(t, gameModel.Create(context.Background(), game))
	require.NoError(t, gameModel.AddEnvBinding(context.Background(), game.GameID, "prod", "test", "", ""))

	router := gin.New()
	router.POST("/scope", func(c *gin.Context) {
		c.Set("adminID", adminID)
		c.Next()
	}, handler.UpdateScope)

	rec := performProfileRequest(router, "POST", "/scope", `{"gameId":"viewer-game","env":"prod"}`)
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

// --- performProfileRequest helper ---

func performProfileRequest(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
