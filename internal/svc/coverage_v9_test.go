package svc

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newV9TestDB opens a fresh file-backed sqlite database with the goose
// version table pre-created so individual Go migrations can be replayed via
// (*goose.Migration).UpContext.
func newV9TestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "v9.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS goose_db_version (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER,
		is_applied INTEGER,
		tstamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`).Error)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	return db
}

// newV9ClosedSQLDB returns a *sql.DB whose pool has been closed: every probe
// query fails, which drives the wrapGorm error branches inside migrations.
func newV9ClosedSQLDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "closed.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	return sqlDB
}

// newV9ClosedGormDB returns a *gorm.DB whose underlying pool is closed, for
// exercising query-error branches in seed functions.
func newV9ClosedGormDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "closed.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	return db
}

// runV9GooseUp replays one numbered Go migration directly. goose dispatches on
// the migration source extension, so a synthetic .go source is attached.
func runV9GooseUp(t *testing.T, m *goose.Migration, sqlDB *sql.DB) error {
	t.Helper()
	m.Source = fmt.Sprintf("%05d_v9.go", m.Version)
	return m.UpContext(context.Background(), sqlDB)
}

// v9MigrationConstructors lists every numbered Go migration builder so tests
// can replay them against crafted database states.
func v9MigrationConstructors() []*goose.Migration {
	return []*goose.Migration{
		openapiBackfillMigration(),
		legacyCleanupMigration(),
		enumColumnsMigration(),
		supportContextMigration(),
		bugTrackerMigration(),
		toolRegistryMigration(),
		releaseMigration(),
		configNamespaceMigration(),
		ticketCSATMigration(),
		hotpatchMigration(),
		dbSourceMigration(),
		platformSettingsMigration(),
		taskSchedulesMigration(),
		agentSessionAddrMigration(),
		contractTimeoutMigration(),
		announcementTablesMigration(),
		adminLoginSecurityMigration(),
		adminMfaMigration(),
	}
}

// Every Go migration must fail fast when the *sql.DB handed over by goose is
// unusable: wrapGorm cannot probe the dialect.
func TestGooseMigrationsWrapGormFailureV9(t *testing.T) {
	sqlDB := newV9ClosedSQLDB(t)
	for _, m := range v9MigrationConstructors() {
		err := runV9GooseUp(t, m, sqlDB)
		require.Error(t, err, "migration version %d", m.Version)
		assert.Contains(t, err.Error(), "probe dialect")
	}
}

// On a database with tables missing entirely, the table-creating migrations
// build their tables; a second replay is idempotent.
func TestGooseMigrationsCreateTablesOnEmptyDBV9(t *testing.T) {
	db := newV9TestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)

	for _, m := range []*goose.Migration{
		bugTrackerMigration(),
		toolRegistryMigration(),
		releaseMigration(),
		hotpatchMigration(),
		dbSourceMigration(),
		platformSettingsMigration(),
		taskSchedulesMigration(),
		announcementTablesMigration(),
	} {
		require.NoError(t, runV9GooseUp(t, m, sqlDB), "migration version %d", m.Version)
	}

	for _, mdl := range []interface{}{
		&model.Bug{}, &model.ToolLink{}, &model.GameRelease{}, &model.Hotpatch{},
		&model.DBSource{}, &model.PlatformSetting{}, &model.TaskSchedule{},
		&model.TaskScheduleRunLog{}, &model.Announcement{}, &model.AnnouncementRead{},
	} {
		assert.True(t, db.Migrator().HasTable(mdl), "table for %T", mdl)
	}

	// Idempotent replay: HasTable short-circuits.
	for _, m := range []*goose.Migration{
		bugTrackerMigration(),
		toolRegistryMigration(),
		releaseMigration(),
		hotpatchMigration(),
		dbSourceMigration(),
		platformSettingsMigration(),
		taskSchedulesMigration(),
		announcementTablesMigration(),
	} {
		require.NoError(t, runV9GooseUp(t, m, sqlDB), "replay version %d", m.Version)
	}
}

// 0015: legacy agent_sessions without the addr column gets it back.
func TestAgentSessionAddrMigrationV9(t *testing.T) {
	db := newV9TestDB(t)
	require.NoError(t, db.AutoMigrate(&reg.AgentSessionDB{}))
	require.NoError(t, db.Migrator().DropColumn(&reg.AgentSessionDB{}, "Addr"))
	assert.False(t, db.Migrator().HasColumn(&reg.AgentSessionDB{}, "Addr"))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, runV9GooseUp(t, agentSessionAddrMigration(), sqlDB))
	assert.True(t, db.Migrator().HasColumn(&reg.AgentSessionDB{}, "Addr"))

	// Databases without agent_sessions (game DBs) skip silently.
	db2 := newV9TestDB(t)
	sqlDB2, err := db2.DB()
	require.NoError(t, err)
	require.NoError(t, runV9GooseUp(t, agentSessionAddrMigration(), sqlDB2))
	assert.False(t, db2.Migrator().HasTable(&reg.AgentSessionDB{}))
}

// 0017: legacy admins missing the login-security columns get them back.
func TestAdminLoginSecurityMigrationV9(t *testing.T) {
	db := newV9TestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Admin{}))
	for _, col := range []string{"FailedAttempts", "LockedUntil", "TokenVersion"} {
		require.NoError(t, db.Migrator().DropColumn(&model.Admin{}, col))
	}

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, runV9GooseUp(t, adminLoginSecurityMigration(), sqlDB))
	for _, col := range []string{"FailedAttempts", "LockedUntil", "TokenVersion"} {
		assert.True(t, db.Migrator().HasColumn(&model.Admin{}, col))
	}
	// Idempotent.
	require.NoError(t, runV9GooseUp(t, adminLoginSecurityMigration(), sqlDB))
}

// 0018: legacy admins without otp_enabled get it back.
func TestAdminMfaMigrationV9(t *testing.T) {
	db := newV9TestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Admin{}))
	require.NoError(t, db.Migrator().DropColumn(&model.Admin{}, "OTPEnabled"))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, runV9GooseUp(t, adminMfaMigration(), sqlDB))
	assert.True(t, db.Migrator().HasColumn(&model.Admin{}, "OTPEnabled"))
	require.NoError(t, runV9GooseUp(t, adminMfaMigration(), sqlDB))
}

// 0005: game-support context columns are re-added on legacy schemas.
func TestSupportContextMigrationV9(t *testing.T) {
	db := newV9TestDB(t)
	require.NoError(t, db.AutoMigrate(&model.FAQ{}, &model.Ticket{}))

	faqCols := []string{"slug", "summary", "helpful_count", "unhelpful_count"}
	ticketCols := []string{"server_id", "player_level", "device_os", "device_model", "language", "extra"}
	for _, col := range faqCols {
		require.NoError(t, db.Migrator().DropColumn(&model.FAQ{}, col))
	}
	for _, col := range ticketCols {
		require.NoError(t, db.Migrator().DropColumn(&model.Ticket{}, col))
	}

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, runV9GooseUp(t, supportContextMigration(), sqlDB))
	for _, col := range faqCols {
		assert.True(t, db.Migrator().HasColumn(&model.FAQ{}, col), "faqs.%s", col)
	}
	for _, col := range ticketCols {
		assert.True(t, db.Migrator().HasColumn(&model.Ticket{}, col), "tickets.%s", col)
	}
	// Idempotent replay.
	require.NoError(t, runV9GooseUp(t, supportContextMigration(), sqlDB))
}

// 0010: ticket CSAT columns are re-added on legacy schemas.
func TestTicketCSATMigrationV9(t *testing.T) {
	db := newV9TestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Ticket{}))
	for _, col := range []string{"rating", "rated_by", "rated_at"} {
		require.NoError(t, db.Migrator().DropColumn(&model.Ticket{}, col))
	}

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, runV9GooseUp(t, ticketCSATMigration(), sqlDB))
	for _, col := range []string{"rating", "rated_by", "rated_at"} {
		assert.True(t, db.Migrator().HasColumn(&model.Ticket{}, col))
	}
	require.NoError(t, runV9GooseUp(t, ticketCSATMigration(), sqlDB))
}

// 0009: config_versions.namespace is re-added and existing rows backfilled.
func TestConfigNamespaceMigrationV9(t *testing.T) {
	db := newV9TestDB(t)
	require.NoError(t, db.AutoMigrate(&model.ConfigVersion{}))
	require.NoError(t, db.Migrator().DropColumn(&model.ConfigVersion{}, "namespace"))
	require.NoError(t, db.Exec("INSERT INTO config_versions DEFAULT VALUES").Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, runV9GooseUp(t, configNamespaceMigration(), sqlDB))
	assert.True(t, db.Migrator().HasColumn(&model.ConfigVersion{}, "namespace"))

	var namespace string
	require.NoError(t, db.Model(&model.ConfigVersion{}).Select("namespace").Scan(&namespace).Error)
	assert.Equal(t, model.ConfigNamespaceDefault, namespace)
	// Idempotent replay.
	require.NoError(t, runV9GooseUp(t, configNamespaceMigration(), sqlDB))
}

// wrapGorm selects the postgres dialector when probing succeeds; gorm's own
// initialization then reads the server version.
func TestWrapGormPostgresViaMockV9(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	mock.ExpectQuery("SELECT COUNT").WillReturnError(assert.AnError)
	mock.ExpectQuery("SELECT CURRENT_SETTING").WillReturnRows(
		sqlmock.NewRows([]string{"s"}).AddRow("15.0"))
	mock.ExpectQuery("SELECT VERSION()").WillReturnRows(
		sqlmock.NewRows([]string{"version"}).AddRow("PostgreSQL 16.2"))

	gdb, err := wrapGorm(sqlDB)
	require.NoError(t, err)
	require.NotNil(t, gdb)
}

// When the probe succeeds but gorm initialization fails (sqlite connection
// whose version probe errors), wrapGorm surfaces the wrapped error.
func TestWrapGormOpenFailureV9(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	mock.ExpectQuery("SELECT COUNT").WillReturnRows(
		sqlmock.NewRows([]string{"c"}).AddRow(0))
	mock.ExpectQuery("select sqlite_version()").WillReturnError(assert.AnError)

	_, err = wrapGorm(sqlDB)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wrap gorm for migration")
}

// probeDialect recognizes SQL Server via @@VERSION, and wrapGorm wires the
// mssql dialector for such connections.
func TestWrapGormMssqlDialectV9(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	mock.ExpectQuery("SELECT COUNT").WillReturnError(assert.AnError)
	mock.ExpectQuery("SELECT CURRENT_SETTING").WillReturnError(assert.AnError)
	mock.ExpectQuery("SELECT @@version_comment").WillReturnError(assert.AnError)
	mock.ExpectQuery("SELECT @@VERSION").WillReturnRows(
		sqlmock.NewRows([]string{"v"}).AddRow("Microsoft SQL Server 2019"))

	dialect, err := probeDialect(sqlDB)
	require.NoError(t, err)
	assert.Equal(t, "mssql", dialect)

	// wrapGorm probes again internally, then selects the sqlserver dialector.
	sqlDB2, mock2, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB2.Close() })
	mock2.ExpectQuery("SELECT COUNT").WillReturnError(assert.AnError)
	mock2.ExpectQuery("SELECT CURRENT_SETTING").WillReturnError(assert.AnError)
	mock2.ExpectQuery("SELECT @@version_comment").WillReturnError(assert.AnError)
	mock2.ExpectQuery("SELECT @@VERSION").WillReturnRows(
		sqlmock.NewRows([]string{"v"}).AddRow("Microsoft SQL Server 2019"))

	gdb, err := wrapGorm(sqlDB2)
	require.NoError(t, err)
	require.NotNil(t, gdb)
}

// sqliteFileDSN is idempotent for DSNs that already carry pragmas.
func TestSQLiteFileDSNIdempotentV9(t *testing.T) {
	in := "file:data/x.db?cache=shared&_pragma=busy_timeout(1000)"
	assert.Equal(t, in, sqliteFileDSN(in))
}

// openReadOnlyGorm falls back to the conventional data/croupier.db path when
// no DSN is configured. The file is a local test artifact and may be absent in
// fresh checkouts, so it is created on demand.
func TestOpenReadOnlyGormDefaultDSNV9(t *testing.T) {
	const defaultPath = "data/croupier.db"
	created := false
	if _, err := os.Stat(defaultPath); os.IsNotExist(err) {
		dir := filepath.Dir(defaultPath)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		seed, err := gorm.Open(gsqlite.Open(defaultPath), &gorm.Config{})
		require.NoError(t, err)
		sqlSeed, _ := seed.DB()
		require.NoError(t, sqlSeed.Close())
		created = true
		t.Cleanup(func() { _ = os.Remove(defaultPath) })
	}

	db, err := openReadOnlyGorm("sqlite", "")
	require.NoError(t, err)
	require.NotNil(t, db)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	var n int
	require.NoError(t, db.Raw("SELECT COUNT(1) FROM sqlite_master").Scan(&n).Error)
	_ = created
}

// A fan-out against an unopenable meta database fails immediately.
func TestRunMigrationFanoutOpenMetaFailureV9(t *testing.T) {
	t.Setenv("DB_DRIVER", "bogus")
	t.Setenv("DATABASE_URL", "")
	cfg := config.Config{Database: config.DatabaseConfig{Driver: "bogus", DataSource: "x"}}
	_, err := RunMigrationFanout(context.Background(), cfg, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fanout: open meta database")
}

func v9FanoutConfig(t *testing.T, multiGame bool, dsn string) config.Config {
	t.Helper()
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DATABASE_URL", dsn)
	return config.Config{
		Server: config.ServerConfig{Mode: "dev"},
		Database: config.DatabaseConfig{
			Driver:     "sqlite",
			DataSource: dsn,
			MultiGame:  multiGame,
		},
	}
}

// A meta database whose baseline cannot run (conflicting object name) is
// reported as an error row instead of aborting the pass.
func TestRunMigrationFanoutMetaBaselineFailureV9(t *testing.T) {
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "meta.db")
	raw, err := gorm.Open(gsqlite.Open(metaPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, raw.Exec("CREATE VIEW admins AS SELECT 1 AS id WHERE 1=0").Error)
	sqlRaw, _ := raw.DB()
	require.NoError(t, sqlRaw.Close())

	cfg := v9FanoutConfig(t, false, metaPath)
	reports, err := RunMigrationFanout(context.Background(), cfg, false)
	require.NoError(t, err)
	require.Len(t, reports, 1)
	assert.Equal(t, FanoutStatusError, reports[0].Status)
	assert.NotEmpty(t, reports[0].Err)
}

// A registry binding whose physical database cannot be opened is reported as
// missing; the runtime creates game databases lazily.
func TestRunMigrationFanoutMissingGameDatabaseV9(t *testing.T) {
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "meta.db")
	cfg := v9FanoutConfig(t, true, metaPath)
	seedEnvBinding(t, cfg, "demo", "prod")

	driver, metaDSN := resolveDriverAndDSN(cfg)
	gameDSN := DSNForDatabase(driver, metaDSN, gameDBNameFor(cfg, "demo", "prod"))
	// Occupy the game database path with a directory: sqlite cannot open it.
	require.NoError(t, os.MkdirAll(gameDSN, 0o755))

	reports, err := RunMigrationFanout(context.Background(), cfg, false)
	require.NoError(t, err)
	require.Len(t, reports, 2)
	assert.Equal(t, FanoutStatusMissing, reports[1].Status)
	assert.NotEmpty(t, reports[1].Err)
	assert.Equal(t, "demo", reports[1].GameID)
}

// A game database whose baseline fails is reported as an error row.
func TestRunMigrationFanoutGameBaselineFailureV9(t *testing.T) {
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "meta.db")
	cfg := v9FanoutConfig(t, true, metaPath)
	seedEnvBinding(t, cfg, "demo", "prod")

	driver, metaDSN := resolveDriverAndDSN(cfg)
	gameDSN := DSNForDatabase(driver, metaDSN, gameDBNameFor(cfg, "demo", "prod"))
	raw, err := gorm.Open(gsqlite.Open(gameDSN), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, raw.Exec("CREATE VIEW players AS SELECT 1 AS id WHERE 1=0").Error)
	sqlRaw, _ := raw.DB()
	require.NoError(t, sqlRaw.Close())

	reports, err := RunMigrationFanout(context.Background(), cfg, false)
	require.NoError(t, err)
	require.Len(t, reports, 2)
	assert.Equal(t, FanoutStatusError, reports[1].Status)
	assert.NotEmpty(t, reports[1].Err)
}

// FormatFanoutReports annotates error rows, and the summary line counts both
// migrated and error statuses.
func TestFormatFanoutReportsStatusesV9(t *testing.T) {
	reports := []FanoutReport{
		{GameID: "-", Env: "-", Database: "(meta)", Before: 0, After: 19, Status: FanoutStatusMigrated},
		{GameID: "a", Env: "prod", Database: "game_a", Status: FanoutStatusError, Err: "boom"},
		{GameID: "b", Env: "prod", Database: "game_b", Status: FanoutStatusMissing, Err: "gone"},
	}
	out := FormatFanoutReports(reports)
	assert.Contains(t, out, "error (boom)")
	assert.Contains(t, out, "missing-database")
	assert.Contains(t, out, "total=3 migrated=1 error=1")
}
