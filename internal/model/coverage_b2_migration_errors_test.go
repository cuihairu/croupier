package model

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func b2OpenFileDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	return db
}

// newReadOnlyDB 先以可写连接准备库结构，再以 mode=ro 只读重开：
// 读操作正常、一切 DDL/DML 失败，用于覆盖迁移错误分支。
func newReadOnlyDB(t *testing.T, setup func(w *gorm.DB)) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "b2ro.db")
	w := b2OpenFileDB(t, path)
	setup(w)
	sqlDB, err := w.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	ro, err := gorm.Open(sqlite.Open("file:"+path+"?mode=ro"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	return ro
}

func b2LegacyTables(t *testing.T, w *gorm.DB) {
	require.NoError(t, w.Exec("CREATE TABLE admin_records (id INTEGER PRIMARY KEY)").Error)
}

func b2LegacyEnumTable(t *testing.T, w *gorm.DB) {
	require.NoError(t, w.Exec("CREATE TABLE function_contracts (id INTEGER PRIMARY KEY, capability VARCHAR(16), risk VARCHAR(16))").Error)
}

func b2LegacyPageIndex(t *testing.T, w *gorm.DB) {
	require.NoError(t, w.Exec("CREATE TABLE page_specs (id INTEGER PRIMARY KEY, page_key VARCHAR(128))").Error)
	require.NoError(t, w.Exec("CREATE UNIQUE INDEX uni_page_specs_page_key ON page_specs(page_key)").Error)
}

func TestB2AutoMigrateReadOnlyErrors(t *testing.T) {
	t.Run("rename legacy table blocked", func(t *testing.T) {
		ro := newReadOnlyDB(t, func(w *gorm.DB) { b2LegacyTables(t, w) })
		assert.Error(t, AutoMigrate(ro))
		assert.Error(t, AutoMigrateMeta(ro))
	})

	t.Run("enum migration blocked", func(t *testing.T) {
		ro := newReadOnlyDB(t, func(w *gorm.DB) { b2LegacyEnumTable(t, w) })
		assert.Error(t, AutoMigrateMeta(ro))
		assert.Error(t, AutoMigrateGame(ro))
		assert.Error(t, MigrateEnumColumns(ro))
	})

	t.Run("legacy page index drop blocked", func(t *testing.T) {
		ro := newReadOnlyDB(t, func(w *gorm.DB) { b2LegacyPageIndex(t, w) })
		assert.Error(t, AutoMigrateMeta(ro))
		assert.Error(t, AutoMigrate(ro))
		assert.Error(t, DropLegacyPageUniqueIndexes(ro))
	})

	t.Run("create tables blocked", func(t *testing.T) {
		ro := newReadOnlyDB(t, func(w *gorm.DB) {})
		assert.Error(t, AutoMigrate(ro))
		assert.Error(t, AutoMigrateGame(ro))
	})

	t.Run("legacy ui table drop blocked", func(t *testing.T) {
		ro := newReadOnlyDB(t, func(w *gorm.DB) {
			require.NoError(t, w.Exec("CREATE TABLE function_ui_schemas (id INTEGER PRIMARY KEY)").Error)
		})
		assert.Error(t, CleanupLegacyPageTables(ro))
		assert.Error(t, CleanupAllLegacy(ro))
	})
}

func TestB2AutoMigrateUpdatePhaseErrors(t *testing.T) {
	t.Run("openapi backfill blocked", func(t *testing.T) {
		db := setupAllModelsDB(t)
		require.NoError(t, db.Exec(`INSERT INTO functions (function_id, game_id, name, spec_format, open_api_spec, created_at, updated_at) VALUES ('b2fn', 'b2g', 'n', '', '{"x":1}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)
		b2Trigger(t, db, `CREATE TRIGGER b2_stop_fn_upd BEFORE UPDATE ON functions BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
		assert.Error(t, AutoMigrate(db))
	})

	t.Run("term display backfill blocked", func(t *testing.T) {
		db := setupAllModelsDB(t)
		require.NoError(t, db.Exec("ALTER TABLE term_dictionary ADD COLUMN display_zh TEXT").Error)
		require.NoError(t, db.Exec(`INSERT INTO term_dictionary (domain, term_key, alias, display_zh, created_at, updated_at) VALUES ('resource', 'b2k', 'b2a', '中文', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)
		b2Trigger(t, db, `CREATE TRIGGER b2_stop_term_upd BEFORE UPDATE ON term_dictionary BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
		assert.Error(t, AutoMigrate(db))
	})
}

func TestB2MigrateEnumColumnErrors(t *testing.T) {
	t.Run("backfill warn", func(t *testing.T) {
		db := setupAllModelsDB(t)
		require.NoError(t, db.Exec(`INSERT INTO function_contracts (function_id, capability, created_at, updated_at) VALUES ('b2fn.x', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)
		b2Trigger(t, db, `CREATE TRIGGER b2_stop_fc_upd BEFORE UPDATE ON function_contracts BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
		assert.NoError(t, MigrateEnumColumns(db))
	})

	t.Run("count error returns nil", func(t *testing.T) {
		db := setupAllModelsDB(t)
		require.NoError(t, db.Exec("ALTER TABLE function_contracts DROP COLUMN capability").Error)
		assert.NoError(t, MigrateEnumColumns(db))
	})

	t.Run("translate blocked", func(t *testing.T) {
		db := setupAllModelsDB(t)
		require.NoError(t, db.Exec("ALTER TABLE function_contracts DROP COLUMN capability").Error)
		require.NoError(t, db.Exec("ALTER TABLE function_contracts ADD COLUMN capability VARCHAR(16)").Error)
		require.NoError(t, db.Exec("ALTER TABLE function_contracts ADD COLUMN capability_enum_migrate INTEGER").Error)
		require.NoError(t, db.Exec(`INSERT INTO function_contracts (capability, created_at, updated_at) VALUES ('safe', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)
		b2Trigger(t, db, `CREATE TRIGGER b2_stop_fc_upd2 BEFORE UPDATE ON function_contracts BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
		assert.Error(t, MigrateEnumColumns(db))
	})

	t.Run("drop legacy column blocked by index", func(t *testing.T) {
		db := setupAllModelsDB(t)
		require.NoError(t, db.Exec("ALTER TABLE function_contracts DROP COLUMN capability").Error)
		require.NoError(t, db.Exec("ALTER TABLE function_contracts ADD COLUMN capability VARCHAR(16)").Error)
		require.NoError(t, db.Exec("ALTER TABLE function_contracts ADD COLUMN capability_enum_migrate INTEGER").Error)
		require.NoError(t, db.Exec(`INSERT INTO function_contracts (capability, created_at, updated_at) VALUES ('safe', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)
		require.NoError(t, db.Exec("CREATE INDEX b2_idx_fc_cap ON function_contracts(capability)").Error)
		assert.Error(t, MigrateEnumColumns(db))
	})
}

type b2NopPool struct{}

func (b2NopPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return nil, sql.ErrConnDone
}
func (b2NopPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return nil, sql.ErrConnDone
}
func (b2NopPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return nil, sql.ErrConnDone
}
func (b2NopPool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return nil
}

func TestB2PostgresHelpersBranches(t *testing.T) {
	assert.NoError(t, dropLegacyPageUniqueIndexes(nil))

	db := setupAllModelsDB(t)
	assert.False(t, tryRenamePostgresDefaultUniqueConstraint(db, "tbl", "uni_tbl_"))
	assert.False(t, tryRenamePostgresDefaultUniqueConstraint(nil, "tbl", "uni_tbl_col"))

	fake := &gorm.DB{Config: &gorm.Config{ConnPool: b2NopPool{}}}
	assert.False(t, tryRenamePostgresDefaultUniqueConstraint(fake, "tbl", "uni_tbl_col"))
}
