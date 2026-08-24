package svc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitObjectStore_SignedURLTTLVariants(t *testing.T) {
	ctx := context.Background()
	base := filepath.Join(t.TempDir(), "uploads")

	store, err := initObjectStore(ctx, config.StorageConfig{
		Driver: "file", BaseDir: base, SignedURLTTL: "30s",
	})
	require.NoError(t, err)
	require.NotNil(t, store)

	// An unparsable TTL is ignored and the driver default applies.
	store, err = initObjectStore(ctx, config.StorageConfig{
		Driver: "file", BaseDir: base, SignedURLTTL: "bogus",
	})
	assert.NoError(t, err)
	assert.NotNil(t, store)
}

func TestSeedBootstrapManagers_NilAndEmptyEntries(t *testing.T) {
	svcCtx := setupTestServiceContext(t)
	managerDir := t.TempDir()
	manager := NewAdminManager(managerDir)

	require.NoError(t, os.WriteFile(filepath.Join(managerDir, "roles.json"), []byte(
		`[null,{"code":"","name":"Empty"},{"code":"ops","name":"Ops","permissions":[]}]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(managerDir, "admins.json"), []byte(
		`[{"username":"mixed","password":"secret123","roles":["","ops"],"status":1}]`), 0o644))
	require.NoError(t, manager.Initialize())
	svcCtx.AdminManager = manager

	require.NoError(t, seedBootstrapPermissions(svcCtx))
	require.NoError(t, seedBootstrapRoles(svcCtx))
	require.NoError(t, seedBootstrapAdmins(svcCtx))
}

func TestResolveFirstAuthorizedGame_OrderingAndMissingGame(t *testing.T) {
	ctx := context.Background()
	svcCtx := setupTestServiceContext(t)

	admin := &model.Admin{Username: "root2", Status: 1}
	require.NoError(t, svcCtx.AdminModel.Create(ctx, admin, "password"))
	role := &model.Role{Name: "super_admin"}
	require.NoError(t, svcCtx.RoleModel.Create(ctx, role))
	require.NoError(t, svcCtx.AdminModel.AssignRole(ctx, admin.ID, role.ID))

	// Admin with no bindings at all -> empty result.
	gameID, env := resolveFirstAuthorizedGame(ctx, svcCtx, admin.ID)
	assert.Empty(t, gameID)
	assert.Empty(t, env)

	// Same game, two environments: the env comparison arm of the sorter runs
	// and "dev" sorts before "prod".
	require.NoError(t, svcCtx.GameModel.Create(ctx, &model.Game{Name: "Beta", GameID: "beta", AliasName: "beta"}))
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(ctx, "beta", "prod", "db_beta_prod", "", ""))
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(ctx, "beta", "dev", "db_beta_dev", "", ""))
	gameID, env = resolveFirstAuthorizedGame(ctx, svcCtx, admin.ID)
	assert.Equal(t, "beta", gameID)
	assert.Equal(t, "dev", env)

	// A binding whose game row vanished yields an empty fallback.
	player := &model.Admin{Username: "plain", Status: 1}
	require.NoError(t, svcCtx.AdminModel.Create(ctx, player, "password"))
	require.NoError(t, svcCtx.AdminModel.SetGameEnvScope(ctx, player.ID, 424242, "prod"))
	gameID, env = resolveFirstAuthorizedGame(ctx, svcCtx, player.ID)
	assert.Empty(t, gameID)
	assert.Empty(t, env)
}

func TestGameDBMiddleware_InjectsGameDatabase(t *testing.T) {
	ctx := context.Background()
	svcCtx := setupTestServiceContext(t)

	require.NoError(t, svcCtx.GameModel.Create(ctx, &model.Game{Name: "Demo", GameID: "demo", AliasName: "demo"}))
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(ctx, "demo", "prod", "db_demo_prod", "", ""))

	metaDSN := filepath.Join(t.TempDir(), "meta.db")
	svcCtx.Router = newGameRouter(config.Config{
		Database: config.DatabaseConfig{Driver: "sqlite", DataSource: metaDSN},
	}, svcCtx.DB)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/scoped", nil)
	c.Request.Header.Set(GameDBHeader, "demo")
	c.Request.Header.Set(EnvHeader, "prod")

	GameDBMiddleware(svcCtx)(c)
	assert.False(t, c.IsAborted())
	scope := GameScopeFromContext(c.Request.Context())
	assert.Equal(t, "demo", scope.GameID)
	assert.Equal(t, "prod", scope.Env)
}
