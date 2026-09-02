package svc

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	jwtutil "github.com/cuihairu/croupier/internal/security/jwtutil"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitObjectStore_Variants(t *testing.T) {
	ctx := context.Background()

	store, err := initObjectStore(ctx, config.StorageConfig{})
	assert.NoError(t, err)
	assert.Nil(t, store)

	fileStore, err := initObjectStore(ctx, config.StorageConfig{
		Driver:  "file",
		BaseDir: filepath.Join(t.TempDir(), "uploads"),
	})
	require.NoError(t, err)
	require.NotNil(t, fileStore)

	_, err = initObjectStore(ctx, config.StorageConfig{Driver: "unknown-driver"})
	assert.Error(t, err)

	_, err = initObjectStore(ctx, config.StorageConfig{Driver: "s3"})
	assert.Error(t, err)
}

func TestDBHealth_CheckAndPing(t *testing.T) {
	ctx := context.Background()

	health := NewDBHealth(nil)
	assert.Error(t, health.Check(ctx))

	svcCtx := setupTestServiceContext(t)

	// No admin persisted yet -> lookup fails.
	broken := NewDBHealth(svcCtx)
	assert.Error(t, broken.Check(ctx))
	assert.Error(t, broken.Ping())

	// With an admin present the health check succeeds.
	admin := &model.Admin{Username: "health", Status: 1}
	require.NoError(t, svcCtx.AdminModel.Create(ctx, admin, "password"))
	ok := NewDBHealth(svcCtx)
	assert.NoError(t, ok.Check(ctx))
	assert.NoError(t, ok.Ping())

	// A closed database pool fails again.
	sqlDB, err := svcCtx.DB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	assert.Error(t, broken.Check(ctx))
}

func TestAuthMiddleware_Authenticate(t *testing.T) {
	m := NewAuthMiddlewareImpl(&ServiceContext{})

	// Empty global secret is rejected.
	restore := jwtutil.ResetGlobalSecretForTesting("")
	defer restore()
	_, _, _, err := m.authenticate(context.Background(), "token")
	assert.Error(t, err)

	jwtutil.InitGlobalSecret("unit-test-secret")
	defer jwtutil.InitGlobalSecret("")

	token, err := jwtutil.Sign("unit-test-secret", "alice", []string{"admin"}, 7, 0, time.Now())
	require.NoError(t, err)

	username, roles, adminID, err := m.authenticate(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, "alice", username)
	assert.Equal(t, []string{"admin"}, roles)
	assert.Equal(t, uint(7), adminID)

	_, _, _, err = m.authenticate(context.Background(), "not-a-jwt")
	assert.Error(t, err)
}

func TestAuthMiddleware_ShouldBypass_Matrix(t *testing.T) {
	m := NewAuthMiddlewareImpl(&ServiceContext{})
	gin.SetMode(gin.TestMode)

	cases := []struct {
		method string
		path   string
		want   bool
	}{
		{"GET", "/healthz", true},
		{"POST", "/api/v1/auth/login", true},
		{"GET", "/api/v1/auth/login/sso", true},
		{"GET", "/api/v1/registry/functions", true},
		{"POST", "/api/v1/registry/agents", false},
		{"GET", "/api/v1/players", false},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		assert.Equal(t, tc.want, m.shouldBypass(req), "%s %s", tc.method, tc.path)
	}
}

func TestNewGameRouter_NameForGameWithPrefix(t *testing.T) {
	cfg := newSvcConfig(t, true)
	cfg.Database.GameDBPrefix = "game_"

	db := setupTestDB(t)
	router := newGameRouter(cfg, db)
	require.NotNil(t, router)
	require.NotNil(t, router.NameForGame)

	assert.Equal(t, "game_demo_prod", router.NameForGame("Demo", "PROD"))
	assert.Equal(t, "game_default_default", router.NameForGame("", ""))
	assert.Equal(t, "game_we_rd_e_v", router.NameForGame("we!rd", "e@v"))
}

func TestResolveFirstAuthorizedGame_Branches(t *testing.T) {
	ctx := context.Background()
	svcCtx := setupTestServiceContext(t)

	// Admin-role user: bindings are sorted by (game_id, env).
	admin := &model.Admin{Username: "root", Status: 1}
	require.NoError(t, svcCtx.AdminModel.Create(ctx, admin, "password"))
	role := &model.Role{Name: "admin"}
	require.NoError(t, svcCtx.RoleModel.Create(ctx, role))
	require.NoError(t, svcCtx.AdminModel.AssignRole(ctx, admin.ID, role.ID))

	require.NoError(t, svcCtx.GameModel.Create(ctx, &model.Game{Name: "Zeta", GameID: "zeta", AliasName: "zeta"}))
	require.NoError(t, svcCtx.GameModel.Create(ctx, &model.Game{Name: "Alpha", GameID: "alpha", AliasName: "alpha"}))
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(ctx, "zeta", "prod", "db_zeta_prod", "", ""))
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(ctx, "alpha", "dev", "db_alpha_dev", "", ""))

	gameID, env := resolveFirstAuthorizedGame(ctx, svcCtx, admin.ID)
	assert.Equal(t, "alpha", gameID)
	assert.Equal(t, "dev", env)

	// Non-admin user whose env scope points at a missing numeric game.
	player := &model.Admin{Username: "limited", Status: 1}
	require.NoError(t, svcCtx.AdminModel.Create(ctx, player, "password"))
	require.NoError(t, svcCtx.AdminModel.SetGameEnvScope(ctx, player.ID, 99999, "prod"))

	gameID, env = resolveFirstAuthorizedGame(ctx, svcCtx, player.ID)
	assert.Empty(t, gameID)
	assert.Empty(t, env)

	// validateGameScope reports errors from the meta database lookup.
	closedCtx := newClosedServiceContext(t)
	err := validateGameScope(ctx, closedCtx, "demo", "prod")
	assert.Error(t, err)
}

func TestGameScopeHelpers_EdgeCases(t *testing.T) {
	assert.Equal(t, GameScope{}, GameScopeFromContext(nil))

	ctx := WithGameScope(nil, GameScope{})
	assert.Empty(t, GameScopeFromContext(ctx).GameID)

	_, err := CurrentScope(ctx)
	assert.Error(t, err)
	assert.False(t, ScopeMatches(ctx, "demo", "prod"))

	scoped := WithGameScope(context.Background(), GameScope{GameID: "demo", Env: "prod"})
	scope, err := CurrentScope(scoped)
	require.NoError(t, err)
	assert.Equal(t, "demo", scope.GameID)
	assert.True(t, ScopeMatches(scoped, "DEMO", "prod"))
	assert.True(t, ScopeMatchesGame(scoped, "demo"))
	assert.Equal(t, "demo", ResolveGameID(scoped, "fallback"))
	assert.Equal(t, "fallback-env", ResolveEnv(ctx, "fallback-env"))
}
