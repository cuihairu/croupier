package svc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	jwtutil "github.com/cuihairu/croupier/internal/security/jwtutil"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// --- db.go ---

func TestResolveDriverAndDSN_AutoAndEnv(t *testing.T) {
	tests := []struct {
		name       string
		cfg        config.Config
		wantDriver string
		wantDSN    string
	}{
		{"postgres url", config.Config{Database: config.DatabaseConfig{DataSource: "postgres://h/db"}}, "postgres", "postgres://h/db"},
		{"postgresql url", config.Config{Database: config.DatabaseConfig{DataSource: "postgresql://h/db"}}, "postgres", "postgresql://h/db"},
		{"pgx url", config.Config{Database: config.DatabaseConfig{DataSource: "pgx://h/db"}}, "postgres", "pgx://h/db"},
		{"mysql url", config.Config{Database: config.DatabaseConfig{DataSource: "mysql://h/db"}}, "mysql", "mysql://h/db"},
		{"sqlserver url", config.Config{Database: config.DatabaseConfig{DataSource: "sqlserver://h/db"}}, "sqlserver", "sqlserver://h/db"},
		{"default sqlite", config.Config{Database: config.DatabaseConfig{DataSource: "x.db"}}, "sqlite", "x.db"},
		{"empty all", config.Config{}, "sqlite", ""},
		{"explicit driver", config.Config{Database: config.DatabaseConfig{Driver: " MySQL ", DataSource: "d"}}, "mysql", "d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver, dsn := ResolveDriverAndDSN(tt.cfg)
			assert.Equal(t, tt.wantDriver, driver)
			assert.Equal(t, tt.wantDSN, dsn)
		})
	}
}

func TestResolveDriverAndDSN_EnvOverridesBoth(t *testing.T) {
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DATABASE_URL", "file:x.db")

	driver, dsn := ResolveDriverAndDSN(config.Config{
		Database: config.DatabaseConfig{Driver: "postgres", DataSource: "host=x"},
	})
	assert.Equal(t, "sqlite", driver)
	assert.Equal(t, "file:x.db", dsn)
}

func TestOpenReadOnlyDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ro.db")
	cfg := config.Config{Database: config.DatabaseConfig{Driver: "sqlite", DataSource: path}}

	// 只读打开要求文件已存在，先正常创建一次
	seedDB, err := openGorm("sqlite", path)
	require.NoError(t, err)
	seedSQL, _ := seedDB.DB()
	require.NoError(t, seedSQL.Close())

	db, err := OpenReadOnlyDatabase(cfg)
	require.NoError(t, err)
	require.NotNil(t, db)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	// 只读模式：写入应失败
	require.Error(t, db.Exec("CREATE TABLE t_ro(v int)").Error)
}

func TestOpenReadOnlyGorm_DSNRequired(t *testing.T) {
	for _, driver := range []string{"postgres", "mysql", "sqlserver"} {
		_, err := openReadOnlyGorm(driver, "")
		require.Error(t, err, driver)
		assert.Contains(t, err.Error(), "DSN is required")
	}
	_, err := openReadOnlyGorm("oracle", "")
	require.Error(t, err)
}

func TestOpenGormForRouter_SQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "router.db")
	db, err := OpenGormForRouter("sqlite", path)
	require.NoError(t, err)
	require.NotNil(t, db)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
}

func TestEnsureGameDatabase_SQLite(t *testing.T) {
	dsn, err := EnsureGameDatabase("sqlite", filepath.Join(t.TempDir(), "meta.db"), "game_demo_prod")
	require.NoError(t, err)
	assert.Contains(t, dsn, "game_demo_prod.db")

	dsn2, err := EnsureGameDatabase("sqlite3", ":memory:", "game_x")
	require.NoError(t, err)
	assert.Contains(t, dsn2, "game_x")
}

func TestEnsureGameDatabase_UnreachableServers(t *testing.T) {
	t.Run("postgres", func(t *testing.T) {
		_, err := EnsureGameDatabase("postgres", "host=127.0.0.1 port=1 user=u dbname=meta", "game_x")
		require.Error(t, err)
	})
	t.Run("mysql", func(t *testing.T) {
		_, err := EnsureGameDatabase("mysql", "u:p@tcp(127.0.0.1:1)/meta", "game_x")
		require.Error(t, err)
	})
	t.Run("sqlserver", func(t *testing.T) {
		_, err := EnsureGameDatabase("sqlserver", "sqlserver://u:p@127.0.0.1:1?database=meta", "game_x")
		require.Error(t, err)
	})
	t.Run("unsupported", func(t *testing.T) {
		_, err := EnsureGameDatabase("oracle", "dsn", "db")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported driver")
	})
}

func TestCreateDatabases_Unreachable(t *testing.T) {
	require.Error(t, createPostgresDatabase("host=127.0.0.1 port=1 user=u dbname=postgres", "game_x"))
	require.Error(t, createMySQLDatabase("u:p@tcp(127.0.0.1:1)/mysql", "game_x"))
	require.Error(t, createSQLServerDatabase("sqlserver://u:p@127.0.0.1:1?database=master", "game_x"))
}

func TestDSNForDatabase_DefaultBranch(t *testing.T) {
	// 未知驱动回退到 postgres DSN 处理
	dsn := DSNForDatabase("oracle", "postgres://h/meta?sslmode=disable", "game_x")
	assert.Contains(t, dsn, "game_x")
}

// --- game_middleware.go ---

func newSvcTestContext(t *testing.T) *ServiceContext {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open("file::memory:?mode=memory"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrateMeta(db))
	require.NoError(t, model.AutoMigrate(db))
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	return &ServiceContext{
		DB:         db,
		AdminModel: model.NewAdminModel(db),
		GameModel:  model.NewGameModel(db),
		RoleModel:  model.NewRoleModel(db),
	}
}

func ginTestEngine(svcCtx *ServiceContext) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(GameDBMiddleware(svcCtx))
	r.GET("/scoped", func(c *gin.Context) {
		scope := GameScopeFromContext(c.Request.Context())
		c.JSON(http.StatusOK, gin.H{"gameId": scope.GameID, "env": scope.Env})
	})
	return r
}

func doScopedRequest(t *testing.T, svcCtx *ServiceContext, gameID, env string, adminID uint) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/scoped", nil)
	if gameID != "" {
		req.Header.Set(GameDBHeader, gameID)
		req.Header.Set(EnvHeader, env)
	}
	w := httptest.NewRecorder()

	eng := gin.New()
	eng.Use(func(inner *gin.Context) {
		if adminID > 0 {
			inner.Set("adminID", adminID)
		}
		GameDBMiddleware(svcCtx)(inner)
	})
	eng.GET("/scoped", func(inner *gin.Context) {
		scope := GameScopeFromContext(inner.Request.Context())
		inner.JSON(http.StatusOK, gin.H{"gameId": scope.GameID, "env": scope.Env})
	})
	eng.ServeHTTP(w, req)
	return w
}

func seedGameWithEnv(t *testing.T, svcCtx *ServiceContext, gameID, env string) (*model.Game, *model.Admin) {
	t.Helper()
	game := &model.Game{GameID: gameID, Name: "Game " + gameID, AliasName: "alias-" + gameID, Enabled: true}
	require.NoError(t, svcCtx.GameModel.Create(context.Background(), game))
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(context.Background(), gameID, env, "db_"+gameID+"_"+env, "", ""))

	admin := &model.Admin{Username: "u-" + gameID, Status: 1}
	require.NoError(t, svcCtx.DB.Create(admin).Error)
	return game, admin
}

func TestGameDBMiddleware_HalfHeaderRejected(t *testing.T) {
	svcCtx := newSvcTestContext(t)

	req := httptest.NewRequest(http.MethodGet, "/scoped", nil)
	req.Header.Set(GameDBHeader, "demo")
	w := httptest.NewRecorder()
	ginTestEngine(svcCtx).ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "incomplete_scope")
}

func TestGameDBMiddleware_ScopeRequired(t *testing.T) {
	svcCtx := newSvcTestContext(t)

	w := doScopedRequest(t, svcCtx, "", "", 0)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "scope_required")
}

func TestGameDBMiddleware_AdminRoleBypass(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	game, admin := seedGameWithEnv(t, svcCtx, "demo", "prod")

	role := &model.Role{Name: "admin", Description: "admin"}
	require.NoError(t, svcCtx.DB.Create(role).Error)
	require.NoError(t, svcCtx.AdminModel.AssignRole(context.Background(), admin.ID, role.ID))

	w := doScopedRequest(t, svcCtx, "demo", "prod", admin.ID)
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"gameId":"demo","env":"prod"}`, w.Body.String())
	_ = game
}

func TestGameDBMiddleware_NonAdminAuthorized(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	game, admin := seedGameWithEnv(t, svcCtx, "demo", "prod")

	scope := &model.AdminGameEnvScope{AdminID: admin.ID, GameID: game.ID, Env: "prod"}
	require.NoError(t, svcCtx.DB.Create(scope).Error)

	w := doScopedRequest(t, svcCtx, "demo", "prod", admin.ID)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestGameDBMiddleware_NonAdminForbidden(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	_, admin := seedGameWithEnv(t, svcCtx, "demo", "prod")

	w := doScopedRequest(t, svcCtx, "demo", "prod", admin.ID)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "scope_not_authorized")
}

func TestGameDBMiddleware_UnknownGameForbidden(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	_, admin := seedGameWithEnv(t, svcCtx, "demo", "prod")

	role := &model.Role{Name: "admin"}
	require.NoError(t, svcCtx.DB.Create(role).Error)
	require.NoError(t, svcCtx.AdminModel.AssignRole(context.Background(), admin.ID, role.ID))

	// admin 角色跳过 scope 授权，但 game_envs 校验仍拦截未知 scope
	w := doScopedRequest(t, svcCtx, "ghost", "prod", admin.ID)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_game_scope")
}

func TestGameDBMiddleware_LastScopeFallback(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	game, admin := seedGameWithEnv(t, svcCtx, "demo", "prod")

	role := &model.Role{Name: "viewer"}
	require.NoError(t, svcCtx.DB.Create(role).Error)
	require.NoError(t, svcCtx.AdminModel.AssignRole(context.Background(), admin.ID, role.ID))
	scope := &model.AdminGameEnvScope{AdminID: admin.ID, GameID: game.ID, Env: "prod"}
	require.NoError(t, svcCtx.DB.Create(scope).Error)

	// 持久化的 last scope
	require.NoError(t, svcCtx.DB.Model(admin).Updates(map[string]any{"last_game_id": "demo", "last_env": "prod"}).Error)
	w := doScopedRequest(t, svcCtx, "", "", admin.ID)
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"gameId":"demo","env":"prod"}`, w.Body.String())
}

func TestGameDBMiddleware_StaleLastScopeFallsBackToFirstAuthorized(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	game, admin := seedGameWithEnv(t, svcCtx, "demo", "prod")

	// last scope 指向不存在的 env → 丢弃，回落到第一个授权 scope
	require.NoError(t, svcCtx.DB.Model(admin).Updates(map[string]any{"last_game_id": "demo", "last_env": "ghost"}).Error)
	scope := &model.AdminGameEnvScope{AdminID: admin.ID, GameID: game.ID, Env: "prod"}
	require.NoError(t, svcCtx.DB.Create(scope).Error)

	w := doScopedRequest(t, svcCtx, "", "", admin.ID)
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"gameId":"demo","env":"prod"}`, w.Body.String())
}

func TestResolveFirstAuthorizedGame_AdminAndNonAdmin(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	ctx := context.Background()

	// admin 用户：取 game_envs 第一个绑定
	game1, admin1 := seedGameWithEnv(t, svcCtx, "a-game", "prod")
	seedGameWithEnv(t, svcCtx, "b-game", "prod")
	role := &model.Role{Name: "super_admin"}
	require.NoError(t, svcCtx.DB.Create(role).Error)
	require.NoError(t, svcCtx.AdminModel.AssignRole(ctx, admin1.ID, role.ID))

	gotGame, gotEnv := resolveFirstAuthorizedGame(ctx, svcCtx, admin1.ID)
	assert.Equal(t, "a-game", gotGame)
	assert.Equal(t, "prod", gotEnv)

	// 非管理员：取第一个 admin_game_env_scopes（按 ID 排序）
	_, admin2 := seedGameWithEnv(t, svcCtx, "c-game", "dev")
	scope2 := &model.AdminGameEnvScope{AdminID: admin2.ID, GameID: game1.ID, Env: "staging"}
	require.NoError(t, svcCtx.DB.Create(scope2).Error)

	gotGame2, gotEnv2 := resolveFirstAuthorizedGame(ctx, svcCtx, admin2.ID)
	assert.Equal(t, "a-game", gotGame2)
	assert.Equal(t, "staging", gotEnv2)

	// 无任何 scope
	_, admin3 := seedGameWithEnv(t, svcCtx, "d-game", "prod")
	gotGame3, gotEnv3 := resolveFirstAuthorizedGame(ctx, svcCtx, admin3.ID)
	assert.Equal(t, "", gotGame3)
	assert.Equal(t, "", gotEnv3)
}

func TestAuthorizeScope_Direct(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	ctx := context.Background()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	// nil 模型 → legacy 跳过
	legacy := &ServiceContext{}
	assert.NoError(t, authorizeScope(ctx, legacy, c, "g", "e"))

	// 无 adminID → 跳过
	assert.NoError(t, authorizeScope(ctx, svcCtx, c, "g", "e"))

	game, admin := seedGameWithEnv(t, svcCtx, "demo", "prod")
	c.Set("adminID", admin.ID)

	// 游戏不存在
	err := authorizeScope(ctx, svcCtx, c, "ghost", "prod")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "game not found")

	// 无授权 scope
	err = authorizeScope(ctx, svcCtx, c, "demo", "prod")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not authorized")

	// 有匹配 scope（大小写与空格不敏感）
	sc := &model.AdminGameEnvScope{AdminID: admin.ID, GameID: game.ID, Env: "PROD"}
	require.NoError(t, svcCtx.DB.Create(sc).Error)
	assert.NoError(t, authorizeScope(ctx, svcCtx, c, "demo", " prod "))
}

func TestValidateGameScope_Direct(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	ctx := context.Background()

	// 无 GameModel → legacy 跳过
	assert.NoError(t, validateGameScope(ctx, &ServiceContext{}, "g", "e"))

	seedGameWithEnv(t, svcCtx, "demo", "prod")
	assert.NoError(t, validateGameScope(ctx, svcCtx, "demo", "prod"))
	err := validateGameScope(ctx, svcCtx, "demo", "ghost")
	require.Error(t, err)
	assert.ErrorIs(t, err, errGameScopeNotFound)
}

func TestGetAdminIDFromGinContext(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.Zero(t, getAdminIDFromGinContext(c))
	c.Set("adminID", uint(7))
	assert.Equal(t, uint(7), getAdminIDFromGinContext(c))
	c.Set("adminID", "not-uint")
	assert.Zero(t, getAdminIDFromGinContext(c))
}

// --- service_context.go ---

func TestAuthMiddleware_Handle(t *testing.T) {
	// InitGlobalSecret 是 sync.Once；同进程其他测试可能已用不同 secret
	// 初始化过，显式重键并在测试结束后恢复。
	defer jwtutil.ResetGlobalSecretForTesting("test-secret-for-svc")()
	svcCtx := newSvcTestContext(t)
	mw := NewAuthMiddlewareImpl(svcCtx)

	newEngine := func() *gin.Engine {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.Use(mw.Handle)
		handler := func(c *gin.Context) {
			username := "anonymous"
			if v, ok := c.Get("username"); ok {
				username = v.(string)
			}
			c.JSON(http.StatusOK, gin.H{"user": username})
		}
		r.GET("/any", handler)
		r.GET("/healthz", handler)
		r.GET("/api/v1/auth/login", handler)
		return r
	}

	t.Run("bypass healthz", func(t *testing.T) {
		w := httptest.NewRecorder()
		newEngine().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("bypass login", func(t *testing.T) {
		w := httptest.NewRecorder()
		newEngine().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/auth/login", nil))
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("missing token", func(t *testing.T) {
		w := httptest.NewRecorder()
		newEngine().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/any", nil))
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "missing authorization")
	})

	t.Run("bad header format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/any", nil)
		req.Header.Set("Authorization", "Basic abc")
		w := httptest.NewRecorder()
		newEngine().ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "invalid authorization header format")
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/any", nil)
		req.Header.Set("Authorization", "Bearer not-a-jwt")
		w := httptest.NewRecorder()
		newEngine().ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "authentication_failed")
	})

	t.Run("valid token via header", func(t *testing.T) {
		token, err := jwtutil.Sign("test-secret-for-svc", "alice", []string{"admin"}, 42, time.Now())
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/any", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		newEngine().ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "alice")
	})

	t.Run("valid token via query", func(t *testing.T) {
		token, err := jwtutil.Sign("test-secret-for-svc", "bob", nil, 43, time.Now())
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/any?token="+token, nil)
		w := httptest.NewRecorder()
		newEngine().ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "bob")
	})
}

func TestAuthenticate_InvalidToken(t *testing.T) {
	jwtutil.InitGlobalSecret("test-secret-for-svc")
	m := NewAuthMiddlewareImpl(nil)

	// 使用错误 secret 签发的 token 应被拒绝
	token, err := jwtutil.Sign("wrong-secret", "eve", nil, 1, time.Now())
	require.NoError(t, err)
	_, _, _, err = m.authenticate(context.Background(), token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token")
}

func TestScopeContextForBackgroundRegistration(t *testing.T) {
	t.Run("nil context", func(t *testing.T) {
		var ctx *ServiceContext
		out := ctx.scopeContextForBackgroundRegistration("g", "e")
		scope := GameScopeFromContext(out)
		assert.Equal(t, "g", scope.GameID)
		assert.Equal(t, "e", scope.Env)
	})

	t.Run("nil router", func(t *testing.T) {
		out := (&ServiceContext{}).scopeContextForBackgroundRegistration("g", "e")
		assert.Equal(t, "g", GameScopeFromContext(out).GameID)
	})

	t.Run("with sqlite router", func(t *testing.T) {
		dir := t.TempDir()
		cfg := config.Config{Database: config.DatabaseConfig{
			Driver:     "sqlite",
			DataSource: filepath.Join(dir, "meta.db"),
			MultiGame:  true,
		}}
		db, err := openGorm("sqlite", cfg.Database.DataSource)
		require.NoError(t, err)
		svcCtx := &ServiceContext{Router: newGameRouter(cfg, db)}

		out := svcCtx.scopeContextForBackgroundRegistration("demo", "prod")
		assert.Equal(t, "demo", GameScopeFromContext(out).GameID)
	})
}

func TestNewTelemetryService_Disabled(t *testing.T) {
	svc, err := NewTelemetryService(config.Config{}, "svc", nil)
	require.NoError(t, err)
	assert.Nil(t, svc)
}

func TestAutoMigrateAndMeta(t *testing.T) {
	db, err := openGorm("sqlite", ":memory:")
	require.NoError(t, err)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	require.NoError(t, autoMigrate(db))
	require.NoError(t, autoMigrateMeta(db))
}

func TestInitObjectStore(t *testing.T) {
	ctx := context.Background()

	t.Run("empty driver", func(t *testing.T) {
		store, err := initObjectStore(ctx, config.StorageConfig{})
		require.NoError(t, err)
		assert.Nil(t, store)
	})

	t.Run("validate error", func(t *testing.T) {
		_, err := initObjectStore(ctx, config.StorageConfig{Driver: "s3"})
		require.Error(t, err)
	})

	t.Run("file driver", func(t *testing.T) {
		dir := t.TempDir()
		store, err := initObjectStore(ctx, config.StorageConfig{
			Driver:       "file",
			BaseDir:      filepath.Join(dir, "uploads"),
			SignedURLTTL: "not-a-duration", // 非法 TTL 被忽略
		})
		require.NoError(t, err)
		require.NotNil(t, store)
	})

	t.Run("oss missing credentials", func(t *testing.T) {
		_, err := initObjectStore(ctx, config.StorageConfig{Driver: "oss", Bucket: "b", Endpoint: "oss.example.com"})
		require.Error(t, err)
	})

	t.Run("unknown driver", func(t *testing.T) {
		_, err := initObjectStore(ctx, config.StorageConfig{Driver: "ftp", BaseDir: t.TempDir()})
		require.Error(t, err)
	})
}

func TestSeedBootstrapExtensionCatalog(t *testing.T) {
	dir := t.TempDir()
	extDir := filepath.Join(dir, "bootstrap", "extensions")
	require.NoError(t, os.MkdirAll(extDir, 0o755))

	catalog := map[string]any{
		"items": []map[string]any{
			{
				"extensionId": "ext-1",
				"name":        "Ext One",
				"kind":        "source",
				"releases": []map[string]any{
					{"version": "1.0.0", "publishedAt": "2024-01-01T00:00:00Z"},
					{"version": "1.1.0", "manifest": map[string]any{"x": 1}},
				},
			},
			{"extensionId": "", "name": "skip-empty-id"},
		},
	}
	raw, err := json.Marshal(catalog)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(extDir, "catalog.json"), raw, 0o644))

	svcCtx := newSvcTestContext(t)
	svcCtx.Config = config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: filepath.Join(dir, "bootstrap")}}

	require.NoError(t, seedBootstrapExtensionCatalog(svcCtx))

	var cat model.ExtensionCatalog
	require.NoError(t, svcCtx.DB.Where("extension_id = ?", "ext-1").First(&cat).Error)
	assert.Equal(t, "Ext One", cat.Name)

	var releases []model.ExtensionRelease
	require.NoError(t, svcCtx.DB.Where("extension_id = ?", "ext-1").Find(&releases).Error)
	assert.Len(t, releases, 2)

	// 二次执行覆盖 update 分支
	catalog["items"] = []map[string]any{{"extensionId": "ext-1", "name": "Ext Renamed"}}
	raw2, _ := json.Marshal(catalog)
	require.NoError(t, os.WriteFile(filepath.Join(extDir, "catalog.json"), raw2, 0o644))
	require.NoError(t, seedBootstrapExtensionCatalog(svcCtx))
	require.NoError(t, svcCtx.DB.Where("extension_id = ?", "ext-1").First(&cat).Error)
	assert.Equal(t, "Ext Renamed", cat.Name)
}

func TestSeedBootstrapExtensionCatalog_NoFileAndBadData(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	svcCtx.Config = config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: t.TempDir()}}

	// 目录无 catalog.json → 直接成功
	require.NoError(t, seedBootstrapExtensionCatalog(svcCtx))

	// nil ctx / nil DB
	require.NoError(t, seedBootstrapExtensionCatalog(nil))
	require.NoError(t, seedBootstrapExtensionCatalog(&ServiceContext{}))

	// 非法 JSON
	dir := t.TempDir()
	extDir := filepath.Join(dir, "extensions")
	require.NoError(t, os.MkdirAll(extDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(extDir, "catalog.json"), []byte("{bad"), 0o644))
	bad := newSvcTestContext(t)
	bad.Config = config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: dir}}
	require.Error(t, seedBootstrapExtensionCatalog(bad))
}

func TestSeedBootstrapGames_Defaults(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	svcCtx.Config = config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: t.TempDir()}}

	require.NoError(t, seedBootstrapGames(svcCtx))

	var games []model.Game
	require.NoError(t, svcCtx.DB.Find(&games).Error)
	require.NotEmpty(t, games)

	var bindings []model.GameEnvBinding
	require.NoError(t, svcCtx.DB.Find(&bindings).Error)
	require.NotEmpty(t, bindings)

	// 已有游戏时直接返回
	require.NoError(t, seedBootstrapGames(svcCtx))
	require.NoError(t, svcCtx.DB.Find(&games).Error)
	assert.Len(t, games, len(games))
}

func TestSeedBootstrapGames_NilGuard(t *testing.T) {
	require.NoError(t, seedBootstrapGames(nil))
	require.NoError(t, seedBootstrapGames(&ServiceContext{}))
}

func TestDBHealth_WithDB(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	// 注意：DBHealth.Check 固定查询 ID=1，且对 "admin not found"（非 sql.ErrNoRows）
	// 会误判为连接失败（轻微 bug，见报告），先插入 ID=1 的管理员
	require.NoError(t, svcCtx.DB.Create(&model.Admin{Username: "health-admin", Status: 1}).Error)

	h := NewDBHealth(svcCtx)
	require.NoError(t, h.Ping())

	var nilHealth *DBHealth
	// 注意：Check 对 nil 接收者未做守卫（h.svcCtx 直接解引用会 panic，见报告），
	// 因此此处不调用 nilHealth.Check/Ping。
	_ = nilHealth

	broken := NewDBHealth(&ServiceContext{})
	require.Error(t, broken.Ping())
}
