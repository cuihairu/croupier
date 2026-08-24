package svc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/db/router"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestNewServiceContext_PolicyAndCacheFallbacks(t *testing.T) {
	cfg := newSvcConfig(t, false)

	require.NoError(t, os.MkdirAll(cfg.BootstrapData.BaseDir, 0o755))
	// Broken YAML makes the policy manager fall back to nil.
	require.NoError(t, os.WriteFile(filepath.Join(cfg.BootstrapData.BaseDir, "default-policies.yaml"),
		[]byte(":not-a-valid-yaml-map:["), 0o644))

	cfg.Cache = config.CacheConfig{Enabled: true, Type: "bogus-cache"}
	cfg.AgentDispatch.TaskRoutingTTL = "1h"
	cfg.AgentDispatch.ToAgentTLS.Enabled = true

	ctx := NewServiceContext(cfg)
	require.NotNil(t, ctx)
	assert.Nil(t, ctx.PolicyManager)
	assert.NotNil(t, ctx.Cache)
	assert.NotNil(t, ctx.Dispatcher)
}

func TestAutoMigrateWrappers_FailOnClosedDB(t *testing.T) {
	db := newClosedServiceContext(t).DB

	assert.Error(t, autoMigrate(db))
	assert.Error(t, autoMigrateMeta(db))
}

func TestScopeContextForBackgroundRegistration_RouterErrorFallsBack(t *testing.T) {
	metaDB := setupTestDB(t)
	ctx := &ServiceContext{
		Router: router.New(router.Config{
			Driver:      "sqlite",
			NameForGame: func(string, string) string { return "" },
			Open:        func(string, string) (*gorm.DB, error) { return nil, nil },
		}, metaDB),
	}

	out := ctx.scopeContextForBackgroundRegistration("demo", "prod")
	assert.Equal(t, "demo", GameScopeFromContext(out).GameID)
	assert.Equal(t, "prod", GameScopeFromContext(out).Env)
}

func TestInitObjectStore_ObjectStorageDriversConstructLocally(t *testing.T) {
	ctx := context.Background()

	s3Store, err := initObjectStore(ctx, config.StorageConfig{
		Driver: "s3", Bucket: "bkt", AccessKey: "k", SecretKey: "s", Region: "us-east-1",
	})
	assert.NoError(t, err)
	assert.NotNil(t, s3Store)

	ossStore, err := initObjectStore(ctx, config.StorageConfig{
		Driver: "oss", Bucket: "bkt", Endpoint: "https://oss-cn-hangzhou.example.com", AccessKey: "k", SecretKey: "s",
	})
	assert.NoError(t, err)
	assert.NotNil(t, ossStore)

	cosStore, err := initObjectStore(ctx, config.StorageConfig{
		Driver: "cos", Bucket: "bkt", Endpoint: "https://cos.ap-beijing.example.com", AccessKey: "k", SecretKey: "s",
	})
	assert.NoError(t, err)
	assert.NotNil(t, cosStore)
}

func TestSeedBootstrapRoles_ReplacesMismatchedPermissions(t *testing.T) {
	svcCtx := setupTestServiceContext(t)
	managerDir := t.TempDir()
	manager := NewAdminManager(managerDir)

	require.NoError(t, os.WriteFile(filepath.Join(managerDir, "permissions.json"), []byte(
		`[{"code":"real.perm","name":"Real"},{"code":"other.perm","name":"Other"}]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(managerDir, "roles.json"), []byte(
		`[{"code":"ops","name":"Ops","description":"Operators","level":2,"permissions":["real.perm"]}]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(managerDir, "admins.json"), []byte(`[]`), 0o644))
	require.NoError(t, manager.Initialize())
	svcCtx.AdminManager = manager

	require.NoError(t, seedBootstrapPermissions(svcCtx))
	require.NoError(t, seedBootstrapRoles(svcCtx))

	// Desynchronise stored permissions (same length, different content), then
	// reseed: the equality shortcut must notice the difference and replace.
	ctx := context.Background()
	roleModel := svcCtx.RoleModel
	roles, _, err := roleModel.List(ctx, model.ListRolesOptions{})
	require.NoError(t, err)
	require.Len(t, roles, 1)
	require.NoError(t, roleModel.ReplacePermissions(ctx, roles[0].ID, []string{"other.perm"}))

	require.NoError(t, seedBootstrapRoles(svcCtx))
	ids, err := roleModel.GetRolePermissionIDs(ctx, roles[0].ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"real.perm"}, ids)
}

func TestSeedBootstrapRolesAndAdmins_ClosedDatabaseContinues(t *testing.T) {
	svcCtx := newClosedServiceContext(t)
	managerDir := t.TempDir()
	manager := NewAdminManager(managerDir)

	require.NoError(t, os.WriteFile(filepath.Join(managerDir, "admins.json"), []byte(
		`[{"username":"ghost","password":"secret123","roles":["admin"],"status":1}]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(managerDir, "roles.json"), []byte(
		`[{"code":"admin","name":"Admin","description":"Administrator","level":1,"permissions":["*"]}]`), 0o644))
	require.NoError(t, manager.Initialize())
	svcCtx.AdminManager = manager

	// Every database touch fails; game seeding surfaces the error while the
	// admin and role seeders only log-and-continue.
	assert.Error(t, seedBootstrapGames(svcCtx))
	assert.NoError(t, seedBootstrapRoles(svcCtx))
	assert.NoError(t, seedBootstrapAdmins(svcCtx))
}

func TestSeedBootstrapGames_EmptyEntriesAreSkipped(t *testing.T) {
	baseDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "games.json"), []byte(
		`{"games":[{},{"gameId":"   "}]}`), 0o644))

	svcCtx := setupTestServiceContext(t)
	svcCtx.Config.Auth.GamesConfig = filepath.Join(baseDir, "games.json")
	require.NoError(t, seedBootstrapGames(svcCtx))

	games, err := svcCtx.GameModel.ListAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, games)
}
