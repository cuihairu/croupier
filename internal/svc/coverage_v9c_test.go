package svc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/db/migrate"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const v9GooseVersionTableDDL = `CREATE TABLE goose_db_version (
	id INTEGER PRIMARY KEY AUTOINCREMENT, version_id INTEGER, is_applied INTEGER,
	tstamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`

func v9ColumnMigrations() []*goose.Migration {
	return []*goose.Migration{
		supportContextMigration(), ticketCSATMigration(),
		agentSessionAddrMigration(), adminLoginSecurityMigration(),
		adminMfaMigration(), contractTimeoutMigration(),
		configNamespaceMigration(),
	}
}

func v9TableMigrations() []*goose.Migration {
	return []*goose.Migration{
		bugTrackerMigration(), toolRegistryMigration(),
		releaseMigration(), hotpatchMigration(),
		dbSourceMigration(), platformSettingsMigration(),
		taskSchedulesMigration(), announcementTablesMigration(),
	}
}

// On a read-only file every DDL migration fails at its write step while all
// existence probes (reads) succeed.
func TestGooseMigrationsReadOnlyWriteFailuresV9(t *testing.T) {
	dir := t.TempDir()

	// File A: tables exist with legacy (column-missing) shapes, so the
	// migrations reach their AddColumn step and fail.
	pathA := filepath.Join(dir, "ro_columns.db")
	dbA, err := gorm.Open(gsqlite.Open(pathA), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, dbA.Exec(v9GooseVersionTableDDL).Error)
	require.NoError(t, dbA.AutoMigrate(
		&model.FAQ{}, &model.Ticket{}, &reg.AgentSessionDB{}, &model.Admin{},
		&model.FunctionContract{}, &model.ConfigVersion{},
	))
	for _, col := range []string{"slug", "summary", "helpful_count", "unhelpful_count"} {
		require.NoError(t, dbA.Migrator().DropColumn(&model.FAQ{}, col))
	}
	for _, col := range []string{"server_id", "player_level", "device_os", "device_model", "language", "extra", "rating", "rated_by", "rated_at"} {
		require.NoError(t, dbA.Migrator().DropColumn(&model.Ticket{}, col))
	}
	require.NoError(t, dbA.Migrator().DropColumn(&reg.AgentSessionDB{}, "Addr"))
	for _, col := range []string{"FailedAttempts", "LockedUntil", "TokenVersion", "OTPEnabled"} {
		require.NoError(t, dbA.Migrator().DropColumn(&model.Admin{}, col))
	}
	require.NoError(t, dbA.Migrator().DropColumn(&model.FunctionContract{}, "TimeoutMs"))
	require.NoError(t, dbA.Migrator().DropColumn(&model.ConfigVersion{}, "namespace"))
	sqlA, _ := dbA.DB()
	require.NoError(t, sqlA.Close())
	require.NoError(t, os.Chmod(pathA, 0o444))

	roA, err := gorm.Open(gsqlite.Open(pathA), &gorm.Config{})
	require.NoError(t, err)
	roSQLA, err := roA.DB()
	require.NoError(t, err)
	for _, m := range v9ColumnMigrations() {
		err := runV9GooseUp(t, m, roSQLA)
		require.Error(t, err, "migration version %d", m.Version)
		assert.Contains(t, err.Error(), "readonly database")
	}

	// File B: tables missing entirely, so the migrations reach their
	// CreateTable step and fail.
	pathB := filepath.Join(dir, "ro_tables.db")
	dbB, err := gorm.Open(gsqlite.Open(pathB), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, dbB.Exec(v9GooseVersionTableDDL).Error)
	sqlB, _ := dbB.DB()
	require.NoError(t, sqlB.Close())
	require.NoError(t, os.Chmod(pathB, 0o444))

	roB, err := gorm.Open(gsqlite.Open(pathB), &gorm.Config{})
	require.NoError(t, err)
	roSQLB, err := roB.DB()
	require.NoError(t, err)
	for _, m := range v9TableMigrations() {
		err := runV9GooseUp(t, m, roSQLB)
		require.Error(t, err, "migration version %d", m.Version)
		assert.Contains(t, err.Error(), "readonly database")
	}

	// File B2: task_schedules exists but the run-log table is missing; the
	// second CreateTable step fails.
	pathB2 := filepath.Join(dir, "ro_runlogs.db")
	dbB2, err := gorm.Open(gsqlite.Open(pathB2), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, dbB2.Exec(v9GooseVersionTableDDL).Error)
	require.NoError(t, dbB2.AutoMigrate(&model.TaskSchedule{}))
	sqlB2, _ := dbB2.DB()
	require.NoError(t, sqlB2.Close())
	require.NoError(t, os.Chmod(pathB2, 0o444))

	roB2, err := gorm.Open(gsqlite.Open(pathB2), &gorm.Config{})
	require.NoError(t, err)
	roSQLB2, err := roB2.DB()
	require.NoError(t, err)
	err = runV9GooseUp(t, taskSchedulesMigration(), roSQLB2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "readonly database")

	// File C: page_specs carries the legacy unique index; the legacy cleanup
	// migration fails dropping it on a read-only file.
	pathC := filepath.Join(dir, "ro_legacy.db")
	dbC, err := gorm.Open(gsqlite.Open(pathC), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, dbC.Exec(v9GooseVersionTableDDL).Error)
	require.NoError(t, dbC.AutoMigrate(&model.PageSpec{}))
	require.NoError(t, dbC.Exec("CREATE UNIQUE INDEX uni_page_specs_page_key ON page_specs(page_key)").Error)
	sqlC, _ := dbC.DB()
	require.NoError(t, sqlC.Close())
	require.NoError(t, os.Chmod(pathC, 0o444))

	roC, err := gorm.Open(gsqlite.Open(pathC), &gorm.Config{})
	require.NoError(t, err)
	roSQLC, err := roC.DB()
	require.NoError(t, err)
	err = runV9GooseUp(t, legacyCleanupMigration(), roSQLC)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "readonly database")

	// File D: a legacy admin_records table exists while admins does not; the
	// rename step fails on a read-only file.
	pathD := filepath.Join(dir, "ro_rename.db")
	dbD, err := gorm.Open(gsqlite.Open(pathD), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, dbD.Exec(v9GooseVersionTableDDL).Error)
	require.NoError(t, dbD.Exec("CREATE TABLE admin_records (id INTEGER PRIMARY KEY)").Error)
	sqlD, _ := dbD.DB()
	require.NoError(t, sqlD.Close())
	require.NoError(t, os.Chmod(pathD, 0o444))

	roD, err := gorm.Open(gsqlite.Open(pathD), &gorm.Config{})
	require.NoError(t, err)
	roSQLD, err := roD.DB()
	require.NoError(t, err)
	err = runV9GooseUp(t, legacyCleanupMigration(), roSQLD)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "readonly database")
}

// 0009: the namespace backfill UPDATE can fail (here via an ABORT trigger)
// after the column was added.
func TestConfigNamespaceBackfillTriggerFailureV9(t *testing.T) {
	db := newV9TestDB(t)
	require.NoError(t, db.AutoMigrate(&model.ConfigVersion{}))
	require.NoError(t, db.Migrator().DropColumn(&model.ConfigVersion{}, "namespace"))
	require.NoError(t, db.Exec("INSERT INTO config_versions DEFAULT VALUES").Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER v9_no_backfill BEFORE UPDATE ON config_versions
		BEGIN SELECT RAISE(ABORT, 'v9 blocked'); END`).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	err = runV9GooseUp(t, configNamespaceMigration(), sqlDB)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backfill namespace")
	assert.True(t, db.Migrator().HasColumn(&model.ConfigVersion{}, "namespace"))
}

// seedBootstrapExtensionCatalog: a release row insert blocked by a trigger
// surfaces as a wrapped create error after the catalog row was stored.
func TestExtensionReleaseCreateTriggerFailureV9(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "extensions"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "extensions", "catalog.json"),
		[]byte(`{"items":[{"extensionId":"ext","name":"Ext","releases":[{"version":"1.0.0"}]}]}`), 0o644))

	db := newV9TestDB(t)
	require.NoError(t, model.AutoMigrateMeta(db))
	require.NoError(t, db.Exec(`CREATE TRIGGER v9_no_release BEFORE INSERT ON extension_releases
		BEGIN SELECT RAISE(ABORT, 'v9 blocked'); END`).Error)

	ctx := &ServiceContext{
		Config: config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: dir}},
		DB:     db,
	}
	err := seedBootstrapExtensionCatalog(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create extension release seed failed")

	var count int64
	require.NoError(t, db.Model(&model.ExtensionCatalog{}).Where("extension_id = ?", "ext").Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

// seedBootstrapAdmins: an admin lookup that fails outright (dead connection)
// is logged and the admin is skipped.
func TestSeedBootstrapAdminsQueryFailureV9(t *testing.T) {
	dir := t.TempDir()
	hashed := "$2a$10$abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234567890123456789012"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"),
		[]byte(`[{"username":"ghost","password":"`+hashed+`"}]`), 0o644))
	am := NewAdminManager(dir)
	require.NoError(t, am.Initialize())

	ctx := &ServiceContext{
		DB:           newV9ClosedGormDB(t),
		AdminManager: am,
		AdminModel:   model.NewAdminModel(nil),
	}
	require.NoError(t, seedBootstrapAdmins(ctx))
}

// seedBootstrapAdmins: a freshly created admin whose role-assignment count
// query fails (missing admin_roles table) is skipped with a log line.
func TestSeedBootstrapAdminsRoleCountFailureV9(t *testing.T) {
	dir := t.TempDir()
	hashed := "$2a$10$abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234567890123456789012"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"),
		[]byte(`[{"username":"ghost","password":"`+hashed+`","roles":["viewer"]}]`), 0o644))
	am := NewAdminManager(dir)
	require.NoError(t, am.Initialize())

	db := newV9TestDB(t)
	require.NoError(t, model.AutoMigrateMeta(db))
	require.NoError(t, db.Migrator().DropTable("admin_roles"))
	require.NoError(t, db.Create(&model.Role{Name: "viewer"}).Error)

	ctx := &ServiceContext{
		DB:           db,
		AdminManager: am,
		AdminModel:   model.NewAdminModel(db),
	}
	require.NoError(t, seedBootstrapAdmins(ctx))

	var count int64
	require.NoError(t, db.Model(&model.Admin{}).Where("username = ?", "ghost").Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

// A meta database that is current on schema but missing the game_envs
// registry table makes the fan-out fail listing bindings.
func TestRunMigrationFanoutListBindingsFailureV9(t *testing.T) {
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "meta.db")
	meta, err := gorm.Open(gsqlite.Open(metaPath), &gorm.Config{})
	require.NoError(t, err)
	_, err = migrate.EnsureUpToDate(context.Background(), meta, migrate.ScopeMeta, autoMigrateMeta)
	require.NoError(t, err)
	require.NoError(t, meta.Migrator().DropTable("game_envs"))
	sqlMeta, _ := meta.DB()
	require.NoError(t, sqlMeta.Close())

	cfg := v9FanoutConfig(t, true, metaPath)
	_, err = RunMigrationFanout(context.Background(), cfg, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fanout: list game_envs")
}

// seedBootstrapRoles: when the role_permissions table is missing, the
// existing-permissions probe fails and the role is skipped.
func TestSeedBootstrapRolesMissingPermsTableV9(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "roles.json"),
		[]byte(`[{"code":"viewer","permissions":["perm:x"]}]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "permissions.json"),
		[]byte(`[{"code":"perm:x"}]`), 0o644))
	am := NewAdminManager(dir)
	require.NoError(t, am.Initialize())

	db := newV9TestDB(t)
	require.NoError(t, model.AutoMigrateMeta(db))
	require.NoError(t, db.Migrator().DropTable("role_permissions"))
	require.NoError(t, db.Create(&model.Role{Name: "viewer"}).Error)
	require.NoError(t, db.Create(&model.Permission{ID: "perm:x", Name: "Perm X"}).Error)

	ctx := &ServiceContext{
		DB:           db,
		AdminManager: am,
		RoleModel:    model.NewRoleModel(db),
	}
	require.NoError(t, seedBootstrapRoles(ctx))
}

// Seed errors that are returned (extension catalog read failure, term
// dictionary validation) are logged by NewServiceContext without stopping boot.
func TestNewServiceContextSeedErrorsLoggedV9(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "meta.db")
	boot := filepath.Join(dir, "boot")
	require.NoError(t, os.MkdirAll(filepath.Join(boot, "extensions", "catalog.json"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(boot, "term_dictionary.json"),
		[]byte(`{"items":[{"domain":"resource"}]}`), 0o644))

	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DATABASE_URL", dbPath)
	cfg := config.Config{
		Server:        config.ServerConfig{Mode: "dev"},
		Database:      config.DatabaseConfig{Driver: "sqlite", DataSource: dbPath},
		BootstrapData: config.BootstrapDataConfig{BaseDir: boot},
	}
	ctx := NewServiceContext(cfg)
	require.NotNil(t, ctx)
}

// A games table shadowed by a column-less view breaks the environment
// backfill at boot, which is a hard failure.
func TestNewServiceContextBackfillPanicV9(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "meta.db")
	pre, err := gorm.Open(gsqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	_, err = migrate.EnsureUpToDate(context.Background(), pre, migrate.ScopeSingle, autoMigrate)
	require.NoError(t, err)
	game := &model.Game{GameID: "preseeded", Name: "preseeded", AliasName: "Preseeded", Status: "dev", Enabled: true}
	require.NoError(t, game.SetEnvs([]model.GameEnv{{Env: "prod"}}))
	require.NoError(t, pre.Create(game).Error)
	require.NoError(t, pre.Migrator().DropTable("games"))
	require.NoError(t, pre.Exec("CREATE VIEW games AS SELECT 1 AS id").Error)
	sqlPre, _ := pre.DB()
	require.NoError(t, sqlPre.Close())

	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DATABASE_URL", dbPath)
	cfg := config.Config{
		Server:   config.ServerConfig{Mode: "dev"},
		Database: config.DatabaseConfig{Driver: "sqlite", DataSource: dbPath},
	}
	assert.Panics(t, func() { NewServiceContext(cfg) })
}

// A pre-existing game database whose baseline cannot run makes the router's
// MigrateGame callback fail; background registration falls back to the meta
// scope after logging.
func TestNewServiceContextRouterGameMigrationFailureV9(t *testing.T) {
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "meta.db")

	gameDSN := DSNForDatabase("sqlite", metaPath, gameDBNameFor(config.Config{}, "default", "prod"))
	raw, err := gorm.Open(gsqlite.Open(gameDSN), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, raw.Exec("CREATE VIEW players AS SELECT 1 AS id WHERE 1=0").Error)
	sqlRaw, _ := raw.DB()
	require.NoError(t, sqlRaw.Close())

	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DATABASE_URL", metaPath)
	cfg := config.Config{
		Server:   config.ServerConfig{Mode: "dev"},
		Database: config.DatabaseConfig{Driver: "sqlite", DataSource: metaPath, MultiGame: true},
	}
	ctx := NewServiceContext(cfg)
	require.NotNil(t, ctx)

	out := ctx.scopeContextForBackgroundRegistration("default", "prod")
	scope := GameScopeFromContext(out)
	assert.Equal(t, "default", scope.GameID)
	assert.Equal(t, "prod", scope.Env)
}
