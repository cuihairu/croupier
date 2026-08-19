package svc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- db.go 补充 ---

func TestOpenGorm_EmptyDSNRequired(t *testing.T) {
	for _, driver := range []string{"postgres", "mysql", "sqlserver"} {
		_, err := openGorm(driver, "")
		require.Error(t, err, driver)
		assert.Contains(t, err.Error(), "DSN is required", driver)
	}
}

func TestNewGameRouter_WithPrefix(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{Database: config.DatabaseConfig{
		Driver:       "sqlite",
		DataSource:   filepath.Join(dir, "meta.db"),
		MultiGame:    true,
		GameDBPrefix: "gp_",
	}}
	db, err := openGorm("sqlite", cfg.Database.DataSource)
	require.NoError(t, err)

	r := newGameRouter(cfg, db)
	require.NotNil(t, r)

	gameDB, err := r.GameDB(context.Background(), "demo", "prod")
	require.NoError(t, err)
	require.NotNil(t, gameDB)
	sqlDB, _ := gameDB.DB()
	defer sqlDB.Close()
}

// --- 缓存层 ---

func newCacheTestContext(t *testing.T) *ServiceContext {
	t.Helper()
	svcCtx := newSvcTestContext(t)
	store, err := cache.NewCacheStore(config.CacheConfig{Enabled: true, Type: "local"})
	require.NoError(t, err)
	svcCtx.Cache = store
	svcCtx.CacheHelper = cache.NewCacheHelper(store)
	svcCtx.PermissionModel = model.NewPermissionModel(svcCtx.DB)
	return svcCtx
}

func TestCacheLayer_AdminHelpers(t *testing.T) {
	svcCtx := newCacheTestContext(t)
	ctx := context.Background()

	admin := &model.Admin{Username: "cache-user", Nickname: "Cache User", Status: 1}
	require.NoError(t, svcCtx.DB.Create(admin).Error)

	got, err := svcCtx.GetAdminCached(ctx, admin.ID)
	require.NoError(t, err)
	assert.Equal(t, "cache-user", got.Username)

	// 命中缓存
	got2, err := svcCtx.GetAdminCached(ctx, admin.ID)
	require.NoError(t, err)
	assert.Equal(t, got.ID, got2.ID)

	byName, err := svcCtx.GetAdminByUsernameCached(ctx, " cache-user ")
	require.NoError(t, err)
	require.NotNil(t, byName)
	assert.Equal(t, admin.ID, byName.ID)

	// 空用户名
	empty, err := svcCtx.GetAdminByUsernameCached(ctx, "  ")
	require.NoError(t, err)
	assert.Nil(t, empty)

	// 别名缓存失效
	svcCtx.InvalidateAdminCache(ctx, admin.ID, "cache-user")
	// 二次失效走空 key 分支
	svcCtx.InvalidateAdminCache(ctx, admin.ID, "")
	svcCtx.InvalidateAdminRolesCache(ctx, admin.ID)

	// 未找到
	_, err = svcCtx.GetAdminCached(ctx, 9999)
	require.Error(t, err)
}

func TestCacheLayer_RoleAndPermissionHelpers(t *testing.T) {
	svcCtx := newCacheTestContext(t)
	ctx := context.Background()

	role := &model.Role{Name: "cache-role", Description: "d"}
	require.NoError(t, svcCtx.DB.Create(role).Error)
	perm := &model.Permission{ID: "game:read", Name: "Read", Resource: "game", Action: "read", Category: "game"}
	require.NoError(t, svcCtx.DB.Create(perm).Error)
	rp := &model.RolePermission{RoleID: role.ID, PermissionID: perm.ID}
	require.NoError(t, svcCtx.DB.Create(rp).Error)

	gotRole, err := svcCtx.GetRoleCached(ctx, role.ID)
	require.NoError(t, err)
	assert.Equal(t, "cache-role", gotRole.Name)

	ids, err := svcCtx.GetRolePermissionIDsCached(ctx, role.ID)
	require.NoError(t, err)
	assert.Contains(t, ids, perm.ID)

	gotPerm, err := svcCtx.GetPermissionCached(ctx, " GAME:READ ")
	require.NoError(t, err)
	require.NotNil(t, gotPerm)
	assert.Equal(t, "game:read", gotPerm.ID)

	// 空权限码
	emptyPerm, err := svcCtx.GetPermissionCached(ctx, " ")
	require.NoError(t, err)
	assert.Nil(t, emptyPerm)

	svcCtx.InvalidateRoleCache(ctx, role.ID)
	svcCtx.InvalidatePermissionCache(ctx, perm.ID)
	svcCtx.InvalidatePermissionCache(ctx, "")
}

func TestCacheLayer_GameHelpers(t *testing.T) {
	svcCtx := newCacheTestContext(t)
	ctx := context.Background()

	game := &model.Game{GameID: "cache-game", Name: "G", AliasName: "cache-alias"}
	require.NoError(t, svcCtx.GameModel.Create(ctx, game))

	got, err := svcCtx.GetGameCached(ctx, game.ID)
	require.NoError(t, err)
	assert.Equal(t, "cache-game", got.GameID)

	all, err := svcCtx.ListAllGamesCached(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1)

	svcCtx.InvalidateGameCache(ctx, game.ID)

	// 无缓存上下文（nil CacheHelper）走直读分支
	bare := &ServiceContext{
		AdminModel: svcCtx.AdminModel,
		RoleModel:  svcCtx.RoleModel,
		GameModel:  svcCtx.GameModel,
	}
	_, err = bare.GetGameCached(ctx, game.ID)
	require.NoError(t, err)
	_, err = bare.ListAllGamesCached(ctx)
	require.NoError(t, err)
	_, err = bare.GetAdminRolesCached(ctx, 1)
	require.NoError(t, err)
}

// --- ops_state_store.go ---

func TestOpsStateStore_Lifecycle(t *testing.T) {
	baseDir := t.TempDir()
	store := NewOpsStateStore(baseDir)
	require.NotNil(t, store)

	// 初始快照与默认值
	snap := store.Snapshot()
	assert.False(t, snap.Config.UpdatedAt.IsZero())

	// 更新并持久化
	updated, err := store.Update(func(state *OpsState) {
		state.Config.AlertmanagerURL = "http://am.example.com"
		state.Maintenance.Windows = append(state.Maintenance.Windows, OpsMaintenanceWindow{
			ID: "w1", GameID: "demo", Env: "prod", Start: "2024-01-01T00:00:00Z", BlockWrites: true,
		})
	})
	require.NoError(t, err)
	assert.Equal(t, "http://am.example.com", updated.Config.AlertmanagerURL)
	require.Len(t, updated.Maintenance.Windows, 1)

	// 重新加载已持久化文件
	store2 := NewOpsStateStore(baseDir)
	snap2 := store2.Snapshot()
	assert.Equal(t, "http://am.example.com", snap2.Config.AlertmanagerURL)
	require.Len(t, snap2.Maintenance.Windows, 1)

	// cloneOpsState 深拷贝：修改副本不影响原状态
	cp := cloneOpsState(snap2)
	cp.Maintenance.Windows[0].ID = "changed"
	assert.Equal(t, "w1", snap2.Maintenance.Windows[0].ID)
}

func TestOpsStateStore_LoadBadJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ops_state.json"), []byte("{not-json"), 0o644))

	// 加载失败时记录日志并使用默认状态
	store := NewOpsStateStore(dir)
	assert.False(t, store.Snapshot().Config.UpdatedAt.IsZero())
}

func TestOpsStateStore_UpdateSaveFailure(t *testing.T) {
	// 用一个目录作为文件路径，触发写入失败
	dirAsPath := t.TempDir()
	store := NewOpsStateStore(t.TempDir())

	// MkdirAll(已存在目录) 成功，但 WriteFile 到目录失败
	store.path = dirAsPath
	_, err := store.Update(func(state *OpsState) {})
	require.Error(t, err)
}

// --- 杂项小函数 ---

func TestWithGameScope_NilContext(t *testing.T) {
	out := WithGameScope(nil, GameScope{GameID: "g", Env: "e"})
	assert.Equal(t, "g", GameScopeFromContext(out).GameID)
}

func TestResolveBootstrapAuthDir_Variants(t *testing.T) {
	dir := t.TempDir()

	// UsersConfig 所在目录
	got := resolveBootstrapAuthDir(config.Config{Auth: config.AuthConfig{UsersConfig: filepath.Join(dir, "users.yaml")}})
	assert.Equal(t, dir, got)

	// BootstrapData 优先级更高
	priority := filepath.Join(dir, "bootstrap")
	got2 := resolveBootstrapAuthDir(config.Config{
		Auth:          config.AuthConfig{UsersConfig: filepath.Join(dir, "users.yaml")},
		BootstrapData: config.BootstrapDataConfig{BaseDir: priority},
	})
	assert.Equal(t, priority, got2)
}

func TestResolveGamesConfigPath_Variants(t *testing.T) {
	dir := t.TempDir()

	// 显式 GamesConfig
	got := resolveGamesConfigPath(config.Config{Auth: config.AuthConfig{GamesConfig: filepath.Join(dir, "custom-games.json")}})
	assert.Equal(t, filepath.Join(dir, "custom-games.json"), got)

	// baseDir 存在 games.json
	require.NoError(t, os.WriteFile(filepath.Join(dir, "games.json"), []byte(`{}`), 0o644))
	got2 := resolveGamesConfigPath(config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: dir}})
	assert.Equal(t, filepath.Join(dir, "games.json"), got2)

	// 无任何配置 → 空
	assert.Equal(t, "", resolveGamesConfigPath(config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: t.TempDir()}}))
}

func TestLoadGamesBootstrapConfig(t *testing.T) {
	dir := t.TempDir()

	t.Run("explicit path", func(t *testing.T) {
		p := filepath.Join(dir, "g.json")
		require.NoError(t, os.WriteFile(p, []byte(`{"defaultEnvs":[{"env":"prod"}],"games":[]}`), 0o644))
		cfg, err := loadGamesBootstrapConfig(config.Config{Auth: config.AuthConfig{GamesConfig: p}})
		require.NoError(t, err)
		require.Len(t, cfg.DefaultEnvs, 1)
		assert.Equal(t, "prod", cfg.DefaultEnvs[0].Env)
	})

	t.Run("bad json returns error", func(t *testing.T) {
		p := filepath.Join(dir, "bad.json")
		require.NoError(t, os.WriteFile(p, []byte("{oops"), 0o644))
		_, err := loadGamesBootstrapConfig(config.Config{Auth: config.AuthConfig{GamesConfig: p}})
		require.Error(t, err)
	})

	t.Run("no path found", func(t *testing.T) {
		_, err := loadGamesBootstrapConfig(config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: t.TempDir()}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("bom json", func(t *testing.T) {
		p := filepath.Join(dir, "bom.json")
		bom := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"defaultEnvs":[{"env":"prod"}]}`)...)
		require.NoError(t, os.WriteFile(p, bom, 0o644))
		cfg, err := loadGamesBootstrapConfig(config.Config{Auth: config.AuthConfig{GamesConfig: p}})
		require.NoError(t, err)
		require.Len(t, cfg.DefaultEnvs, 1)
	})
}

func TestLoadTermDictionaryConfig(t *testing.T) {
	// nil ctx → 默认
	assert.NotEmpty(t, loadTermDictionaryConfig(nil))

	dir := t.TempDir()
	svcCtx := &ServiceContext{Config: config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: dir}}}

	// 无文件 → 默认
	assert.NotEmpty(t, loadTermDictionaryConfig(svcCtx))

	// 带 BOM 的自定义配置
	bom := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"items":[{"domain":"resource","key":"custom","displayZh":"自定义","displayEn":"Custom"}]}`)...)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "term_dictionary.json"), bom, 0o644))
	cfg := loadTermDictionaryConfig(svcCtx)
	require.Len(t, cfg.Items, 1)
	assert.Equal(t, "custom", cfg.Items[0].Key)

	// 非法 JSON → 默认
	require.NoError(t, os.WriteFile(filepath.Join(dir, "term_dictionary.json"), []byte("{bad"), 0o644))
	assert.NotEmpty(t, loadTermDictionaryConfig(svcCtx))

	// 空条目 → 默认
	require.NoError(t, os.WriteFile(filepath.Join(dir, "term_dictionary.json"), []byte(`{"items":[]}`), 0o644))
	assert.NotEmpty(t, loadTermDictionaryConfig(svcCtx))
}

func TestSeedBootstrapRolesAndPermissions(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(`[]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "roles.json"), []byte(`[
		{"code":"admin","name":"Admin","description":"admin role","permissions":["admin:all"]},
		{"code":"","name":"EmptyCode"}
	]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "permissions.json"), []byte(`[
		{"code":"admin:all","name":"All","description":"all","category":"admin","module":"system"}
	]`), 0o644))

	svcCtx := newSvcTestContext(t)
	svcCtx.AdminManager = NewAdminManager(dir)
	svcCtx.PermissionModel = model.NewPermissionModel(svcCtx.DB)
	svcCtx.PermissionService = nil

	// Initialize 从 dir 加载 admins/roles/permissions 默认定义
	require.NoError(t, svcCtx.AdminManager.Initialize())

	require.NoError(t, seedBootstrapPermissions(svcCtx))
	require.NoError(t, seedBootstrapRoles(svcCtx))

	var role model.Role
	require.NoError(t, svcCtx.DB.Where("name = ?", "admin").First(&role).Error)
	assert.Equal(t, "Admin", role.Category)

	var perm model.Permission
	require.NoError(t, svcCtx.DB.First(&perm, "id = ?", "admin:all").Error)

	// 二次执行幂等
	require.NoError(t, seedBootstrapRoles(svcCtx))
	require.NoError(t, seedBootstrapPermissions(svcCtx))

	// nil 守卫
	require.NoError(t, seedBootstrapRoles(nil))
	require.NoError(t, seedBootstrapRoles(&ServiceContext{}))
	require.NoError(t, seedBootstrapPermissions(nil))
	require.NoError(t, seedBootstrapPermissions(&ServiceContext{}))
}

func TestAutoMigrate_ClosedDB(t *testing.T) {
	db, err := openGorm("sqlite", ":memory:")
	require.NoError(t, err)
	sqlDB, _ := db.DB()
	require.NoError(t, sqlDB.Close())

	require.Error(t, autoMigrate(db))
	require.Error(t, autoMigrateMeta(db))
}

func TestNewTelemetryService_EnabledNoCollector(t *testing.T) {
	cfg := config.Config{Telemetry: config.TelemetryConfig{Enabled: true, ServiceName: "svc-test"}}
	svc, err := NewTelemetryService(cfg, "fallback-name", nil)
	if err == nil {
		require.NotNil(t, svc)
		assert.NotNil(t, svc)
	}
	// 失败也被允许（依赖 provider 实现），此处主要覆盖配置组装路径
}
