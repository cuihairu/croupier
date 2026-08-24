package model

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newMigrationTestDB opens a fresh in-memory sqlite database without any tables.
func newMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	return db
}

func TestAutoMigrate_FullRun(t *testing.T) {
	db := newMigrationTestDB(t)

	require.NoError(t, AutoMigrate(db))

	// Idempotent second run.
	require.NoError(t, AutoMigrate(db))

	// Core tables should exist after migration.
	migrator := db.Migrator()
	assert.True(t, migrator.HasTable(&Admin{}))
	assert.True(t, migrator.HasTable(&Game{}))
	assert.True(t, migrator.HasTable(&Function{}))
	assert.True(t, migrator.HasTable(&PageSpec{}))
}

func TestAutoMigrate_RenamesLegacyTables(t *testing.T) {
	db := newMigrationTestDB(t)
	require.NoError(t, db.Exec("CREATE TABLE admin_records (id INTEGER PRIMARY KEY)").Error)

	require.NoError(t, AutoMigrate(db))
	assert.False(t, db.Migrator().HasTable("admin_records"))
	assert.True(t, db.Migrator().HasTable("admins"))
}

func TestAutoMigrate_DropsLegacyPageIndex(t *testing.T) {
	db := newMigrationTestDB(t)
	require.NoError(t, AutoMigrate(db))
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX uni_page_specs_page_key ON page_specs (game_id, env, page_key)`).Error)
	assert.True(t, db.Migrator().HasIndex(&PageSpec{}, "uni_page_specs_page_key"))

	require.NoError(t, dropLegacyPageUniqueIndexes(db))
	assert.False(t, db.Migrator().HasIndex(&PageSpec{}, "uni_page_specs_page_key"))
}

func TestAutoMigrateMetaAndGame_Isolated(t *testing.T) {
	db := newMigrationTestDB(t)
	require.NoError(t, AutoMigrateMeta(db))
	assert.True(t, db.Migrator().HasTable(&Admin{}))
	assert.False(t, db.Migrator().HasTable(&Player{}))

	gameDB := newMigrationTestDB(t)
	require.NoError(t, AutoMigrateGame(gameDB))
	assert.True(t, gameDB.Migrator().HasTable(&Player{}))
	assert.False(t, gameDB.Migrator().HasTable(&Admin{}))

	// migrateModels on a plain (non-postgres) dialector.
	require.NoError(t, migrateModels(gameDB, GameModels()))
}

func TestAutoMigrate_BackfillsOpenAPILegacyColumns(t *testing.T) {
	db := newMigrationTestDB(t)
	require.NoError(t, AutoMigrate(db))

	// Simulate an old schema variant that stored operations in openapi_operation.
	require.NoError(t, db.Exec(`ALTER TABLE functions ADD COLUMN openapi_operation TEXT`).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO functions (function_id, name, created_at, updated_at, openapi_operation) VALUES ('legacy.fn', 'legacy', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, '{"openapi":"3.0.3"}')`,
	).Error)

	require.NoError(t, migrateFunctionOpenAPIColumns(db))

	var specFormat string
	require.NoError(t, db.Raw(`SELECT spec_format FROM functions WHERE function_id = 'legacy.fn'`).Scan(&specFormat).Error)
	assert.NotEmpty(t, specFormat)
}

func TestTryFixPostgresMissingConstraint(t *testing.T) {
	db := newMigrationTestDB(t)

	assert.False(t, tryFixPostgresMissingConstraint(nil, errors.New("boom")))
	assert.False(t, tryFixPostgresMissingConstraint(db, nil))
	assert.False(t, tryFixPostgresMissingConstraint(db, errors.New("unrelated failure")))

	// Non "uni_" constraint names are left untouched.
	assert.False(t, tryFixPostgresMissingConstraint(db,
		errors.New(`constraint "page_specs_page_key_key" of relation "page_specs" does not exist`)))

	// Legacy uni_* names trigger the self-healing path; the ignored Exec calls
	// fail against sqlite but the function still reports success.
	matching := fmt.Errorf(`ERROR: constraint "uni_page_specs_page_key" of relation "page_specs" does not exist`)
	assert.True(t, tryFixPostgresMissingConstraint(db, matching))

	// Wrapped error messages are unwrapped while scanning.
	wrapped := fmt.Errorf("migration failed: %w",
		errors.New(`constraint "uni_functions_function_id" of relation "functions" does not exist`))
	assert.True(t, tryFixPostgresMissingConstraint(db, wrapped))
}

func TestTryRenamePostgresDefaultUniqueConstraint_ArgChecks(t *testing.T) {
	db := newMigrationTestDB(t)

	assert.False(t, tryRenamePostgresDefaultUniqueConstraint(nil, "t", "uni_t_c"))
	assert.False(t, tryRenamePostgresDefaultUniqueConstraint(db, "", "uni_t_c"))
	assert.False(t, tryRenamePostgresDefaultUniqueConstraint(db, "t", ""))
	assert.False(t, tryRenamePostgresDefaultUniqueConstraint(db, "t", "other_prefix_c"))

	// Expected name must start with uni_<table>_.
	assert.False(t, tryRenamePostgresDefaultUniqueConstraint(db, "t", "uni_other_c"))

	// Valid shape: postgresHasConstraint fails against sqlite so no rename happens.
	assert.False(t, tryRenamePostgresDefaultUniqueConstraint(db, "t", "uni_t_c"))
}

func TestPostgresHelpers_WithSQLITEDB(t *testing.T) {
	db := newMigrationTestDB(t)

	sqlDB, ok := postgresSQLDB(db)
	require.True(t, ok)
	require.NotNil(t, sqlDB)

	_, ok = postgresSQLDB(nil)
	assert.False(t, ok)

	assert.False(t, postgresHasConstraint(nil, "t", "c"))
	assert.False(t, postgresHasConstraint(sqlDB, "", "c"))
	assert.False(t, postgresHasConstraint(sqlDB, "t", ""))
	// pg_constraint does not exist in sqlite: query errors and returns false.
	assert.False(t, postgresHasConstraint(sqlDB, "t", "c"))

	assert.NoError(t, postgresExec(nil, "SELECT 1"))
	assert.NoError(t, postgresExec(sqlDB, "   "))
	assert.Error(t, postgresExec(sqlDB, "NOT VALID SQL"))
}
