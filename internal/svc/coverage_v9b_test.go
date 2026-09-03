package svc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/db/migrate"
	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---- cache_layer.go ----

// ---- service_context.go: cachedFetch direct branches ----

func TestCachedFetchDirectPathsV9(t *testing.T) {
	s := &ServiceContext{}
	ctx := context.Background()

	var out string
	err := s.cachedFetch(ctx, "", &out, func() (interface{}, error) {
		return nil, errors.New("loader boom")
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loader boom")

	err = s.cachedFetch(ctx, "", &out, func() (interface{}, error) {
		return make(chan int), nil // json.Marshal cannot encode channels
	})
	require.Error(t, err)

	require.NoError(t, s.cachedFetch(ctx, "", &out, func() (interface{}, error) {
		return "hello", nil
	}))
	assert.Equal(t, "hello", out)
}

func TestCacheLayerFailingStoreV9(t *testing.T) {
	store := &errorCacheStoreV9{}
	s := &ServiceContext{Cache: store, CacheHelper: cache.NewCacheHelper(store)}
	ctx := context.Background()

	// Both alias writes fail and are logged, not propagated.
	admin := &model.Admin{Username: "Bob"}
	admin.ID = 7
	s.cacheAdminAliases(ctx, admin)
	s.cacheAdminAliases(ctx, nil) // nil admin guard

	// Cache deletes fail and are logged, not propagated.
	s.InvalidateAdminCache(ctx, 7, "bob")
	s.InvalidateRoleCache(ctx, 7)
	s.InvalidateGameCache(ctx, 7)
}

type errorCacheStoreV9 struct{}

func (errorCacheStoreV9) Get(ctx context.Context, key string) ([]byte, error) {
	return nil, errors.New("get failed")
}

func (errorCacheStoreV9) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return errors.New("set failed")
}

func (errorCacheStoreV9) Delete(ctx context.Context, key string) error {
	return errors.New("delete failed")
}

func (errorCacheStoreV9) DeletePattern(ctx context.Context, pattern string) error {
	return errors.New("delete pattern failed")
}

func (errorCacheStoreV9) Exists(ctx context.Context, key string) (bool, error) {
	return false, errors.New("exists failed")
}

func (errorCacheStoreV9) Close() error {
	return errors.New("close failed")
}

// ---- service_context.go: migration error paths on a dead connection ----

func TestAutoMigrateClosedDBErrorsV9(t *testing.T) {
	db := newV9ClosedGormDB(t)
	assert.Error(t, autoMigrate(db))
	assert.Error(t, migrateAuxModels(db))
	assert.Error(t, autoMigrateMeta(db))
}

// With every model table present but audit_records shadowed by a view, the
// aux-model step inside autoMigrate/autoMigrateMeta fails.
func TestAutoMigrateAuxViewConflictV9(t *testing.T) {
	db := newV9TestDB(t)
	require.NoError(t, model.AutoMigrate(db))
	require.NoError(t, db.Exec("CREATE VIEW audit_records AS SELECT 1 AS id").Error)

	assert.Error(t, autoMigrate(db))
	assert.Error(t, autoMigrateMeta(db))
}

// ---- service_context.go: misc helpers ----

func TestGameDBNameForPrefixV9(t *testing.T) {
	cfg := config.Config{Database: config.DatabaseConfig{GameDBPrefix: "gp_"}}
	assert.Equal(t, "gp_demo_prod", gameDBNameFor(cfg, "Demo", "PROD"))
	assert.Equal(t, "gp_default_default", gameDBNameFor(cfg, "", ""))
	assert.Equal(t, "game_demo_prod", gameDBNameFor(config.Config{}, "demo", "prod"))
}

func TestDerivePermissionResourceActionV9(t *testing.T) {
	resource, action := derivePermissionResourceAction(":", "")
	assert.Equal(t, "global", resource)
	assert.Equal(t, "*", action)

	resource, action = derivePermissionResourceAction("games:read", "")
	assert.Equal(t, "games", resource)
	assert.Equal(t, "read", action)

	resource, action = derivePermissionResourceAction("", "monitoring")
	assert.Equal(t, "monitoring", resource)
	assert.Equal(t, "*", action)
}

func TestInitObjectStoreInvalidDriverV9(t *testing.T) {
	_, err := initObjectStore(context.Background(), config.StorageConfig{Driver: "carrier-pigeon"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "storage driver")
}

func TestNewAuthMiddlewareImplPrometheusPathV9(t *testing.T) {
	cfg := config.Config{Telemetry: config.TelemetryConfig{
		Prometheus: config.TelemetryPrometheusConfig{Enabled: true},
	}}
	m := NewAuthMiddlewareImpl(&ServiceContext{Config: cfg})
	_, ok := m.allowPaths["/metrics/prometheus"]
	assert.True(t, ok)
}

// ---- service_context.go: extension catalog seed error inputs ----

func TestSeedBootstrapExtensionCatalogBadInputsV9(t *testing.T) {
	dir := t.TempDir()

	// catalog.json shadowed by a directory: ReadFile fails with EISDIR.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "extensions", "catalog.json"), 0o755))
	ctx := &ServiceContext{
		Config: config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: dir}},
		DB:     newV9TestDB(t),
	}
	err := seedBootstrapExtensionCatalog(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read extension catalog seed failed")

	// Invalid JSON.
	dir2 := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir2, "extensions"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "extensions", "catalog.json"), []byte("{not-json"), 0o644))
	ctx.Config.BootstrapData.BaseDir = dir2
	err = seedBootstrapExtensionCatalog(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse extension catalog seed failed")

	// Empty items list is a no-op.
	dir3 := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir3, "extensions"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir3, "extensions", "catalog.json"), []byte(`{"items":[]}`), 0o644))
	ctx.Config.BootstrapData.BaseDir = dir3
	require.NoError(t, seedBootstrapExtensionCatalog(ctx))

	// Valid item but dead connection: the existence query fails.
	dir4 := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir4, "extensions"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir4, "extensions", "catalog.json"),
		[]byte(`{"items":[{"extensionId":"ext","name":"Ext"}]}`), 0o644))
	ctx.Config.BootstrapData.BaseDir = dir4
	ctx.DB = newV9ClosedGormDB(t)
	err = seedBootstrapExtensionCatalog(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query extension catalog seed failed")
}

// ---- game_seed.go ----

func TestBuildGameFromSeedAliasFallbackV9(t *testing.T) {
	game, err := buildGameFromSeed(bootstrapGameSeedEntry{
		GameID: "demo_x",
		Envs:   []model.GameEnv{{Env: "prod"}},
	}, nil, 0)
	require.NoError(t, err)
	assert.Equal(t, "demo_x", game.GameID)
	assert.Equal(t, "Demo X", game.AliasName)
	assert.True(t, game.Enabled)
	assert.Equal(t, "dev", game.Status)

	_, err = buildGameFromSeed(bootstrapGameSeedEntry{}, nil, 0)
	require.Error(t, err)
}

func TestSeedBootstrapGamesBranchesV9(t *testing.T) {
	newCtx := func(t *testing.T, gamesJSON string) *ServiceContext {
		t.Helper()
		dir := t.TempDir()
		if gamesJSON != "" {
			require.NoError(t, os.WriteFile(filepath.Join(dir, "games.json"), []byte(gamesJSON), 0o644))
		}
		db := newV9TestDB(t)
		require.NoError(t, model.AutoMigrateMeta(db))
		return &ServiceContext{
			DB:        db,
			GameModel: model.NewGameModel(db),
			Config:    config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: dir}},
		}
	}

	t.Run("default game fallback", func(t *testing.T) {
		ctx := newCtx(t, `{"defaultEnvs":[{"env":"prod"}],"games":[]}`)
		require.NoError(t, seedBootstrapGames(ctx))
		games, err := ctx.GameModel.ListAll(context.Background())
		require.NoError(t, err)
		require.Len(t, games, 1)
		assert.Equal(t, "default", games[0].GameID)
	})

	t.Run("env binding failure is logged not fatal", func(t *testing.T) {
		ctx := newCtx(t, `{"games":[{"gameId":"demo","env":"prod"}]}`)
		require.NoError(t, ctx.DB.Migrator().DropTable("game_envs"))
		require.NoError(t, seedBootstrapGames(ctx))
		games, err := ctx.GameModel.ListAll(context.Background())
		require.NoError(t, err)
		require.Len(t, games, 1)
	})

	t.Run("create failure is logged not fatal", func(t *testing.T) {
		ctx := newCtx(t, `{"games":[{"gameId":"demo","env":"prod"}]}`)
		require.NoError(t, ctx.DB.Migrator().DropTable("games"))
		require.NoError(t, ctx.DB.Exec("CREATE VIEW games AS SELECT 1 AS id, NULL AS deleted_at WHERE 1=0").Error)
		require.NoError(t, seedBootstrapGames(ctx))
	})
}

// ---- term_dictionary_seed.go ----

func TestSeedBootstrapTermDictionaryMissingKeyV9(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "term_dictionary.json"),
		[]byte(`{"items":[{"domain":"resource"}]}`), 0o644))
	db := newV9TestDB(t)
	ctx := &ServiceContext{
		DB:            db,
		TermDictModel: model.NewTermDictionaryModel(db),
		Config:        config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: dir}},
	}
	err := seedBootstrapTermDictionary(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key is required")
}

// ---- admin_manager.go ----

func TestAdminManagerBcryptTooLongV9(t *testing.T) {
	am := NewAdminManager(t.TempDir())
	require.NoError(t, am.Initialize())

	err := am.CreateAdmin(&AdminUser{Username: "u1", Password: strings.Repeat("x", 100)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to hash password")

	// Pre-hash the stored password so CreateAdmin stores it verbatim.
	hashed := "$2a$10$abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234567890123456789012"
	require.NoError(t, am.CreateAdmin(&AdminUser{Username: "u2", Password: hashed}))

	err = am.ResetPassword("u2", strings.Repeat("y", 100))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to hash password")
}

// ---- game_middleware.go ----

func TestResolveFirstAuthorizedGameEnvScopesV9(t *testing.T) {
	db := newV9TestDB(t)
	require.NoError(t, model.AutoMigrateMeta(db))

	game := &model.Game{GameID: "demo", Name: "demo", AliasName: "Demo", Enabled: true}
	require.NoError(t, db.Create(game).Error)
	admin := &model.Admin{Username: "scoped", Status: 1}
	require.NoError(t, db.Create(admin).Error)
	role := &model.Role{Name: "viewer"}
	require.NoError(t, db.Create(role).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: admin.ID, RoleID: role.ID}).Error)
	for _, env := range []string{"prod", "dev"} {
		require.NoError(t, db.Create(&model.AdminGameEnvScope{
			AdminID: admin.ID, GameID: game.ID, Env: env,
		}).Error)
	}

	svcCtx := &ServiceContext{
		DB:         db,
		AdminModel: model.NewAdminModel(db),
		GameModel:  model.NewGameModel(db),
	}
	gameID, env := resolveFirstAuthorizedGame(context.Background(), svcCtx, admin.ID)
	assert.Equal(t, "demo", gameID)
	assert.Equal(t, "prod", env) // ordered by scope ID: prod was inserted first
}

// ---- service_context.go: seed permission branches ----

func TestSeedBootstrapPermissionsBranchesV9(t *testing.T) {
	t.Run("empty name falls back to code", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "permissions.json"),
			[]byte(`[{"code":"a:b"}]`), 0o644))
		am := NewAdminManager(dir)
		require.NoError(t, am.Initialize())

		db := newV9TestDB(t)
		require.NoError(t, model.AutoMigrateMeta(db))
		ctx := &ServiceContext{
			DB:              db,
			AdminManager:    am,
			PermissionModel: model.NewPermissionModel(db),
		}
		require.NoError(t, seedBootstrapPermissions(ctx))

		var perm model.Permission
		require.NoError(t, db.Where("id = ?", "a:b").First(&perm).Error)
		assert.Equal(t, "a:b", perm.Name)

		// Existing permission: no error, no duplicate.
		require.NoError(t, seedBootstrapPermissions(ctx))
		var count int64
		require.NoError(t, db.Model(&model.Permission{}).Where("id = ?", "a:b").Count(&count).Error)
		assert.Equal(t, int64(1), count)
	})

	t.Run("query failure is logged", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "permissions.json"),
			[]byte(`[{"code":"a:b","name":"A B"}]`), 0o644))
		am := NewAdminManager(dir)
		require.NoError(t, am.Initialize())

		ctx := &ServiceContext{
			DB:              newV9ClosedGormDB(t),
			AdminManager:    am,
			PermissionModel: model.NewPermissionModel(nil),
		}
		require.NoError(t, seedBootstrapPermissions(ctx))
	})
}

// ---- service_context.go: read-only database drives write failures ----

func TestSeedBootstrapReadOnlyDBErrorsV9(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "ro.db")

	writable, err := gorm.Open(gsqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(writable))
	// Pre-existing admin with a stale status: bootstrapping tries to update
	// it, which must fail on the read-only file.
	pre := &model.Admin{Username: "preexist", Status: 1}
	require.NoError(t, writable.Create(pre).Error)
	// Force a stale status that differs from the bootstrap default (gorm's
	// default:1 tag would swallow an explicit zero on insert).
	require.NoError(t, writable.Model(pre).Update("status", 0).Error)
	viewer := &model.Role{Name: "viewer"}
	require.NoError(t, writable.Create(viewer).Error)
	// The permission referenced by the bootstrap role must validate.
	require.NoError(t, writable.Create(&model.Permission{ID: "perm:x", Name: "Perm X"}).Error)
	// Existing catalog record: the seed takes the update path.
	require.NoError(t, writable.Create(&model.ExtensionCatalog{ExtensionID: "ext", Name: "Ext"}).Error)
	sqlW, _ := writable.DB()
	require.NoError(t, sqlW.Close())
	require.NoError(t, os.Chmod(dbPath, 0o444))

	roDB, err := gorm.Open(gsqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)

	boot := filepath.Join(dir, "boot")
	require.NoError(t, os.MkdirAll(filepath.Join(boot, "extensions"), 0o755))
	hashed := "$2a$10$abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234567890123456789012"
	require.NoError(t, os.WriteFile(filepath.Join(boot, "admins.json"),
		[]byte(`[{"username":"fresh","password":"`+hashed+`","roles":["viewer"]},{"username":"preexist","password":"`+hashed+`","roles":["viewer"]}]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(boot, "roles.json"),
		[]byte(`[{"code":"viewer","permissions":["perm:x"]},{"code":"ghost","permissions":[]}]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(boot, "permissions.json"),
		[]byte(`[{"code":"perm:x"},{"code":"perm:y"}]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(boot, "extensions", "catalog.json"),
		[]byte(`{"items":[{"extensionId":"ext2"},{"extensionId":"ext","name":"Ext","releases":[{"version":"1.0.0"}]}]}`), 0o644))

	am := NewAdminManager(boot)
	require.NoError(t, am.Initialize())

	ctx := &ServiceContext{
		DB:              roDB,
		AdminManager:    am,
		AdminModel:      model.NewAdminModel(roDB),
		RoleModel:       model.NewRoleModel(roDB),
		PermissionModel: model.NewPermissionModel(roDB),
		GameModel:       model.NewGameModel(roDB),
		Config:          config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: boot}},
	}

	// None of these propagate the underlying read-only write errors.
	require.NoError(t, seedBootstrapPermissions(ctx))
	require.NoError(t, seedBootstrapRoles(ctx))
	require.NoError(t, seedBootstrapAdmins(ctx))
	// The catalog seed does propagate its write failure.
	err = seedBootstrapExtensionCatalog(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "attempt to write a readonly database")
}

// ---- service_context.go: NewServiceContext integration paths ----

// A game seeded before startup backfills game_envs bindings during boot, and
// a broken roles.json keeps AdminManager degraded but boot continues.
func TestNewServiceContextBackfillAndBrokenRolesV9(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "meta.db")

	pre, err := gorm.Open(gsqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	_, err = migrate.EnsureUpToDate(context.Background(), pre, migrate.ScopeSingle, autoMigrate)
	require.NoError(t, err)
	game := &model.Game{GameID: "preseeded", Name: "preseeded", AliasName: "Preseeded", Status: "dev", Enabled: true}
	require.NoError(t, game.SetEnvs([]model.GameEnv{{Env: "prod"}}))
	require.NoError(t, pre.Create(game).Error)
	sqlPre, _ := pre.DB()
	require.NoError(t, sqlPre.Close())

	boot := filepath.Join(dir, "boot")
	require.NoError(t, os.MkdirAll(boot, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(boot, "roles.json"), []byte("{invalid-json"), 0o644))

	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DATABASE_URL", dbPath)
	cfg := config.Config{
		Server:        config.ServerConfig{Mode: "dev"},
		Database:      config.DatabaseConfig{Driver: "sqlite", DataSource: dbPath},
		BootstrapData: config.BootstrapDataConfig{BaseDir: boot},
	}
	ctx := NewServiceContext(cfg)
	require.NotNil(t, ctx)

	// The pre-seeded game produced a routing binding at boot.
	bindings, err := ctx.GameModel.ListAllEnvBindings(context.Background())
	require.NoError(t, err)
	found := false
	for _, b := range bindings {
		if b.GameID == "preseeded" && b.Env == "prod" {
			found = true
		}
	}
	assert.True(t, found, "expected backfilled binding for preseeded/prod")
}

// A schema conflict makes the versioned migration baseline fail, which is a
// boot-time panic in both single-database and meta modes.
func TestNewServiceContextMigrateBaselinePanicsV9(t *testing.T) {
	prepare := func(t *testing.T) string {
		t.Helper()
		dbPath := filepath.Join(t.TempDir(), "conflict.db")
		raw, err := gorm.Open(gsqlite.Open(dbPath), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, raw.Exec("CREATE VIEW admins AS SELECT 1 AS id WHERE 1=0").Error)
		sqlRaw, _ := raw.DB()
		require.NoError(t, sqlRaw.Close())
		return dbPath
	}

	t.Run("single", func(t *testing.T) {
		dbPath := prepare(t)
		t.Setenv("DB_DRIVER", "sqlite")
		t.Setenv("DATABASE_URL", dbPath)
		cfg := config.Config{
			Server:   config.ServerConfig{Mode: "dev"},
			Database: config.DatabaseConfig{Driver: "sqlite", DataSource: dbPath},
		}
		assert.Panics(t, func() { NewServiceContext(cfg) })
	})

	t.Run("multigame", func(t *testing.T) {
		dbPath := prepare(t)
		t.Setenv("DB_DRIVER", "sqlite")
		t.Setenv("DATABASE_URL", dbPath)
		cfg := config.Config{
			Server:   config.ServerConfig{Mode: "dev"},
			Database: config.DatabaseConfig{Driver: "sqlite", DataSource: dbPath, MultiGame: true},
		}
		assert.Panics(t, func() { NewServiceContext(cfg) })
	})
}

// Without a JWT secret and outside development mode, boot must fail fast.
func TestNewServiceContextProdNoSecretPanicsV9(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "meta.db")
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DATABASE_URL", dbPath)
	t.Setenv("CROUPIER_MODE", "production")
	t.Setenv("CROUPIER_ENV", "")

	cfg := config.Config{
		Server:   config.ServerConfig{Mode: "production"},
		Database: config.DatabaseConfig{Driver: "sqlite", DataSource: dbPath},
	}
	assert.Panics(t, func() { NewServiceContext(cfg) })
}
