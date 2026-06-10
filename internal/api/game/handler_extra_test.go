package game

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
	"github.com/cuihairu/croupier/internal/service/permission"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupGameHandlerTest(t *testing.T) (*Handler, *gorm.DB) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = model.AutoMigrate(db)
	require.NoError(t, err)

	// Create test admin and permissions
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)
	permissionModel := model.NewPermissionModel(db)
	gameModel := model.NewGameModel(db)

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
		"admin:all", "games:manage",
	})
	require.NoError(t, err)

	permissions := []*model.Permission{
		{ID: "admin:all", Name: "Admin All", Resource: "admin", Action: "all", Category: "admin"},
		{ID: "games:manage", Name: "Games Manage", Resource: "games", Action: "manage", Category: "game"},
		{ID: "games:read", Name: "Games Read", Resource: "games", Action: "read", Category: "game"},
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

func addGameAuthMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		adminModel := model.NewAdminModel(db)
		admin, _ := adminModel.FindByUsername(c.Request.Context(), "testadmin")
		ctx := context.WithValue(c.Request.Context(), "username", "testadmin")
		ctx = context.WithValue(ctx, "adminID", admin.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func TestHandler_List_Success_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	// Create a game first
	gameModel := model.NewGameModel(db)
	game := &model.Game{
		Name:      "testgame",
		AliasName: "Test Game",
		Status:    "dev",
	}
	err := gameModel.Create(nil, game)
	require.NoError(t, err)

	router := gin.New()
	router.Use(addGameAuthMiddleware(db))
	router.GET("/games", handler.List)

	req := httptest.NewRequest("GET", "/games?page=1&pageSize=10", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Create_Success_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	router := gin.New()
	router.Use(addGameAuthMiddleware(db))
	router.POST("/games", handler.Create)

	body := map[string]interface{}{
		"name":      "newgame",
		"aliasName": "New Game",
		"color":     "#8c8c8c",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/games", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Detail_Success_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	// Create a game first
	gameModel := model.NewGameModel(db)
	game := &model.Game{
		Name:      "detailgame",
		AliasName: "Detail Game",
		Status:    "dev",
	}
	err := gameModel.Create(nil, game)
	require.NoError(t, err)

	router := gin.New()
	router.Use(addGameAuthMiddleware(db))
	router.GET("/games/:id", handler.Detail)

	req := httptest.NewRequest("GET", "/games/"+strconv.FormatUint(uint64(game.ID), 10), nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Detail_NotFound_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	router := gin.New()
	router.Use(addGameAuthMiddleware(db))
	router.GET("/games/:id", handler.Detail)

	req := httptest.NewRequest("GET", "/games/99999", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_Detail_InvalidID_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	router := gin.New()
	router.Use(addGameAuthMiddleware(db))
	router.GET("/games/:id", handler.Detail)

	req := httptest.NewRequest("GET", "/games/invalid", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_Update_Success_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	// Create a game first
	gameModel := model.NewGameModel(db)
	game := &model.Game{
		Name:      "updatgame",
		AliasName: "Update Game",
		Status:    "dev",
	}
	err := gameModel.Create(nil, game)
	require.NoError(t, err)

	router := gin.New()
	router.Use(addGameAuthMiddleware(db))
	router.PUT("/games/:id", handler.Update)

	body := map[string]interface{}{
		"aliasName": "Updated Game",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/games/"+strconv.FormatUint(uint64(game.ID), 10), bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Update_NotFound_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	router := gin.New()
	router.Use(addGameAuthMiddleware(db))
	router.PUT("/games/:id", handler.Update)

	body := map[string]interface{}{
		"aliasName": "Updated Game",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/games/99999", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_Delete_Success_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	// Create a game first
	gameModel := model.NewGameModel(db)
	game := &model.Game{
		Name:      "deletegame",
		AliasName: "Delete Game",
		Status:    "dev",
	}
	err := gameModel.Create(nil, game)
	require.NoError(t, err)

	router := gin.New()
	router.Use(addGameAuthMiddleware(db))
	router.DELETE("/games/:id", handler.Delete)

	req := httptest.NewRequest("DELETE", "/games/"+strconv.FormatUint(uint64(game.ID), 10), nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Delete_NotFound_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	router := gin.New()
	router.Use(addGameAuthMiddleware(db))
	router.DELETE("/games/:id", handler.Delete)

	req := httptest.NewRequest("DELETE", "/games/99999", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// GORM soft delete doesn't error on non-existent records
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_EnvsList_Success_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	// Create a game with envs
	gameModel := model.NewGameModel(db)
	game := &model.Game{
		Name:      "envgame",
		AliasName: "Env Game",
		Status:    "dev",
	}
	err := gameModel.Create(nil, game)
	require.NoError(t, err)

	// Add envs via SetEnvs
	envs := []model.GameEnv{
		{Env: "prod", Description: "Production"},
	}
	err = game.SetEnvs(envs)
	require.NoError(t, err)
	err = gameModel.Update(nil, game.ID, map[string]interface{}{"envs": game.Envs})
	require.NoError(t, err)

	router := gin.New()
	router.Use(addGameAuthMiddleware(db))
	router.GET("/games/:id/envs", handler.EnvsList)

	req := httptest.NewRequest("GET", "/games/"+strconv.FormatUint(uint64(game.ID), 10)+"/envs", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_EnvAdd_Success_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	// Create a game first
	gameModel := model.NewGameModel(db)
	game := &model.Game{
		Name:      "envaddgame",
		AliasName: "Env Add Game",
		Status:    "dev",
	}
	err := gameModel.Create(nil, game)
	require.NoError(t, err)

	router := gin.New()
	router.Use(addGameAuthMiddleware(db))
	router.POST("/games/:id/envs", handler.EnvAdd)

	body := map[string]interface{}{
		"name": "staging",
		"type": "Staging Environment",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/games/"+strconv.FormatUint(uint64(game.ID), 10)+"/envs", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_EnvUpdate_Success_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	// Create a game with env
	gameModel := model.NewGameModel(db)
	game := &model.Game{
		Name:      "envupdategame",
		AliasName: "Env Update Game",
		Status:    "dev",
	}
	err := gameModel.Create(nil, game)
	require.NoError(t, err)

	// Add envs via SetEnvs
	envs := []model.GameEnv{
		{Env: "prod", Description: "Production"},
	}
	err = game.SetEnvs(envs)
	require.NoError(t, err)
	err = gameModel.Update(nil, game.ID, map[string]interface{}{"envs": game.Envs})
	require.NoError(t, err)

	router := gin.New()
	router.Use(addGameAuthMiddleware(db))
	router.PUT("/games/:id/envs/:envId", handler.EnvUpdate)

	body := map[string]interface{}{
		"name": "Production Updated",
		"type": "Production Environment",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/games/"+strconv.FormatUint(uint64(game.ID), 10)+"/envs/prod", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_EnvDelete_Success_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	// Create a game with env
	gameModel := model.NewGameModel(db)
	game := &model.Game{
		Name:      "envdeletegame",
		AliasName: "Env Delete Game",
		Status:    "dev",
	}
	err := gameModel.Create(nil, game)
	require.NoError(t, err)

	// Add envs via SetEnvs
	envs := []model.GameEnv{
		{Env: "staging", Description: "Staging"},
	}
	err = game.SetEnvs(envs)
	require.NoError(t, err)
	err = gameModel.Update(nil, game.ID, map[string]interface{}{"envs": game.Envs})
	require.NoError(t, err)

	router := gin.New()
	router.Use(addGameAuthMiddleware(db))
	router.DELETE("/games/:id/envs/:envId", handler.EnvDelete)

	req := httptest.NewRequest("DELETE", "/games/"+strconv.FormatUint(uint64(game.ID), 10)+"/envs/staging", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_EnvDelete_NotFound_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	// Create a game
	gameModel := model.NewGameModel(db)
	game := &model.Game{
		Name:      "envdeletegame2",
		AliasName: "Env Delete Game 2",
		Status:    "dev",
	}
	err := gameModel.Create(nil, game)
	require.NoError(t, err)

	router := gin.New()
	router.Use(addGameAuthMiddleware(db))
	router.DELETE("/games/:id/envs/:envId", handler.EnvDelete)

	req := httptest.NewRequest("DELETE", "/games/"+strconv.FormatUint(uint64(game.ID), 10)+"/envs/nonexistent", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_EnvsList_NotFound_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	router := gin.New()
	router.Use(addGameAuthMiddleware(db))
	router.GET("/games/:id/envs", handler.EnvsList)

	req := httptest.NewRequest("GET", "/games/99999/envs", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_EnvAdd_Duplicate_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	// Create a game with env
	gameModel := model.NewGameModel(db)
	game := &model.Game{
		Name:      "envdupgame",
		AliasName: "Env Dup Game",
		Status:    "dev",
	}
	err := gameModel.Create(nil, game)
	require.NoError(t, err)

	// Add envs via SetEnvs
	envs := []model.GameEnv{
		{Env: "prod", Description: "Production"},
	}
	err = game.SetEnvs(envs)
	require.NoError(t, err)
	err = gameModel.Update(nil, game.ID, map[string]interface{}{"envs": game.Envs})
	require.NoError(t, err)

	router := gin.New()
	router.Use(addGameAuthMiddleware(db))
	router.POST("/games/:id/envs", handler.EnvAdd)

	body := map[string]interface{}{
		"name": "prod",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/games/"+strconv.FormatUint(uint64(game.ID), 10)+"/envs", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_Create_DuplicateName_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	// Create a game first
	gameModel := model.NewGameModel(db)
	game := &model.Game{
		Name:      "dupgame",
		AliasName: "Dup Game",
		Status:    "dev",
	}
	err := gameModel.Create(nil, game)
	require.NoError(t, err)

	router := gin.New()
	router.Use(addGameAuthMiddleware(db))
	router.POST("/games", handler.Create)

	body := map[string]interface{}{
		"name":      "dupgame",
		"aliasName": "Duplicate Game",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/games", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_Create_InvalidJSON_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	router := gin.New()
	router.Use(addGameAuthMiddleware(db))
	router.POST("/games", handler.Create)

	req := httptest.NewRequest("POST", "/games", bytes.NewBufferString("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_Update_InvalidJSON_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	gameModel := model.NewGameModel(db)
	game := &model.Game{Name: "invjsongame", AliasName: "InvJSON", Status: "dev"}
	err := gameModel.Create(nil, game)
	require.NoError(t, err)

	router := gin.New()
	router.Use(addGameAuthMiddleware(db))
	router.PUT("/games/:id", handler.Update)

	req := httptest.NewRequest("PUT", "/games/"+strconv.FormatUint(uint64(game.ID), 10), bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_EnvAdd_InvalidJSON_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	gameModel := model.NewGameModel(db)
	game := &model.Game{Name: "envaddinvjson", AliasName: "EnvAddInvJSON", Status: "dev"}
	err := gameModel.Create(nil, game)
	require.NoError(t, err)

	router := gin.New()
	router.Use(addGameAuthMiddleware(db))
	router.POST("/games/:id/envs", handler.EnvAdd)

	req := httptest.NewRequest("POST", "/games/"+strconv.FormatUint(uint64(game.ID), 10)+"/envs", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_EnvUpdate_InvalidJSON_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	gameModel := model.NewGameModel(db)
	game := &model.Game{Name: "envupinvjson", AliasName: "EnvUpInvJSON", Status: "dev"}
	err := gameModel.Create(nil, game)
	require.NoError(t, err)

	envs := []model.GameEnv{{Env: "prod", Description: "Production"}}
	err = game.SetEnvs(envs)
	require.NoError(t, err)
	err = gameModel.Update(nil, game.ID, map[string]interface{}{"envs": game.Envs})
	require.NoError(t, err)

	router := gin.New()
	router.Use(addGameAuthMiddleware(db))
	router.PUT("/games/:id/envs/:envId", handler.EnvUpdate)

	req := httptest.NewRequest("PUT", "/games/"+strconv.FormatUint(uint64(game.ID), 10)+"/envs/prod", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_List_PermissionDenied_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupGameHandlerTest(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), "username", "nopermuser")
		ctx = context.WithValue(ctx, "adminID", uint(99999))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.GET("/games", handler.List)

	req := httptest.NewRequest("GET", "/games?page=1&pageSize=10", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_Create_PermissionDenied_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupGameHandlerTest(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), "username", "nopermuser")
		ctx = context.WithValue(ctx, "adminID", uint(99999))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.POST("/games", handler.Create)

	body := map[string]interface{}{"name": "testgame"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/games", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_Detail_PermissionDenied_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupGameHandlerTest(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), "username", "nopermuser")
		ctx = context.WithValue(ctx, "adminID", uint(99999))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.GET("/games/:id", handler.Detail)

	req := httptest.NewRequest("GET", "/games/1", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_Update_PermissionDenied_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupGameHandlerTest(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), "username", "nopermuser")
		ctx = context.WithValue(ctx, "adminID", uint(99999))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.PUT("/games/:id", handler.Update)

	body := map[string]interface{}{"aliasName": "X"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/games/1", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_Delete_PermissionDenied_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupGameHandlerTest(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), "username", "nopermuser")
		ctx = context.WithValue(ctx, "adminID", uint(99999))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.DELETE("/games/:id", handler.Delete)

	req := httptest.NewRequest("DELETE", "/games/1", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_EnvsList_PermissionDenied_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupGameHandlerTest(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), "username", "nopermuser")
		ctx = context.WithValue(ctx, "adminID", uint(99999))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.GET("/games/:id/envs", handler.EnvsList)

	req := httptest.NewRequest("GET", "/games/1/envs", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_EnvAdd_PermissionDenied_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupGameHandlerTest(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), "username", "nopermuser")
		ctx = context.WithValue(ctx, "adminID", uint(99999))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.POST("/games/:id/envs", handler.EnvAdd)

	body := map[string]interface{}{"name": "prod"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/games/1/envs", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_EnvUpdate_PermissionDenied_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupGameHandlerTest(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), "username", "nopermuser")
		ctx = context.WithValue(ctx, "adminID", uint(99999))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.PUT("/games/:id/envs/:envId", handler.EnvUpdate)

	body := map[string]interface{}{"name": "x"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/games/1/envs/prod", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_EnvDelete_PermissionDenied_Extra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupGameHandlerTest(t)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), "username", "nopermuser")
		ctx = context.WithValue(ctx, "adminID", uint(99999))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.DELETE("/games/:id/envs/:envId", handler.EnvDelete)

	req := httptest.NewRequest("DELETE", "/games/1/envs/prod", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}
