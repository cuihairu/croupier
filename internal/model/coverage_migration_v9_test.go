package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// fakePostgresDialectorV9 wraps the real dialector but reports "postgres" so
// the postgres-specific migration branches execute against sqlite.
type fakePostgresDialectorV9 struct {
	gorm.Dialector
}

func (d *fakePostgresDialectorV9) Name() string { return "postgres" }

type fakeMySQLDialectorV9 struct {
	gorm.Dialector
}

func (d *fakeMySQLDialectorV9) Name() string { return "mysql" }

type fakeOtherDialectorV9 struct {
	gorm.Dialector
}

func (d *fakeOtherDialectorV9) Name() string { return "sqlserver" }

// asFakePostgresV9 swaps the dialector name on a freshly opened sqlite db.
func asFakePostgresV9(t *testing.T, db *gorm.DB) *gorm.DB {
	t.Helper()
	db.Dialector = &fakePostgresDialectorV9{Dialector: db.Dialector}
	return db
}

func TestAutoMigratePostgresDialectPathV9(t *testing.T) {
	// Full legacy AutoMigrate entry point through the postgres self-healing loop.
	db := asFakePostgresV9(t, newMigrationTestDB(t))
	require.NoError(t, AutoMigrate(db))
	assert.True(t, db.Migrator().HasTable(&Admin{}))
	assert.True(t, db.Migrator().HasTable(&Player{}))
	// Idempotent second run.
	require.NoError(t, AutoMigrate(db))

	// migrateModels / AutoMigrateMeta / AutoMigrateGame postgres paths.
	db2 := asFakePostgresV9(t, newMigrationTestDB(t))
	require.NoError(t, migrateModels(db2, MetaModels()))
	assert.True(t, db2.Migrator().HasTable(&Admin{}))

	db3 := asFakePostgresV9(t, newMigrationTestDB(t))
	require.NoError(t, AutoMigrateMeta(db3))
	assert.True(t, db3.Migrator().HasTable(&Admin{}))
	assert.False(t, db3.Migrator().HasTable(&Player{}))

	db4 := asFakePostgresV9(t, newMigrationTestDB(t))
	require.NoError(t, AutoMigrateGame(db4))
	assert.True(t, db4.Migrator().HasTable(&Player{}))
	assert.False(t, db4.Migrator().HasTable(&Admin{}))
}

func TestMigrationExportedWrappersV9(t *testing.T) {
	db := newMigrationTestDB(t)

	// Wrappers on an empty database are no-ops.
	require.NoError(t, RenameLegacyTables(db))
	require.NoError(t, DropLegacyPageUniqueIndexes(db))
	require.NoError(t, MigrateFunctionOpenAPIColumns(db))
	require.NoError(t, MigrateEnumColumns(db))

	// RenameLegacyTables actually renames a legacy table.
	require.NoError(t, db.Exec("CREATE TABLE admin_records (id INTEGER PRIMARY KEY)").Error)
	require.NoError(t, RenameLegacyTables(db))
	assert.False(t, db.Migrator().HasTable("admin_records"))
	assert.True(t, db.Migrator().HasTable("admins"))
}

func TestRenameLegacyTablesFailureV9(t *testing.T) {
	db := newMigrationTestDB(t)
	// Renaming onto an existing view name fails: HasTable(view) is false so
	// both guards pass, but ALTER TABLE ... RENAME TO errors out.
	require.NoError(t, db.Exec("CREATE TABLE admin_records (id INTEGER PRIMARY KEY)").Error)
	require.NoError(t, db.Exec("CREATE VIEW admins AS SELECT 1 AS id").Error)

	err := renameLegacyTables(db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rename table admin_records -> admins")
}

func TestDropLegacyPageUniqueIndexesPostgresBranchV9(t *testing.T) {
	db := asFakePostgresV9(t, newMigrationTestDB(t))

	require.NoError(t, db.AutoMigrate(&PageSpec{}))
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX uni_page_specs_page_key ON page_specs (game_id, env, page_key)`).Error)
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX page_specs_page_key_key ON page_specs (game_id, env, page_key)`).Error)
	assert.True(t, db.Migrator().HasIndex(&PageSpec{}, "uni_page_specs_page_key"))

	require.NoError(t, DropLegacyPageUniqueIndexes(db))
	assert.False(t, db.Migrator().HasIndex(&PageSpec{}, "uni_page_specs_page_key"))
}

func TestMigrateFunctionOpenAPIColumnsErrorsV9(t *testing.T) {
	db := newMigrationTestDB(t)

	// functions table missing -> early return.
	require.NoError(t, migrateFunctionOpenAPIColumns(db))

	// open_api_spec is a generated column: the backfill UPDATE must fail.
	require.NoError(t, db.Exec(`
		CREATE TABLE functions (
			id INTEGER PRIMARY KEY,
			open_api_spec TEXT GENERATED ALWAYS AS ('x') VIRTUAL,
			openapi_operation TEXT,
			spec_format TEXT
		)`).Error)
	err := migrateFunctionOpenAPIColumns(db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backfill open_api_spec")

	// spec_format CHECK rejects 'openapi3.0.3' for openapi rows.
	db2 := newMigrationTestDB(t)
	require.NoError(t, db2.Exec(`
		CREATE TABLE functions (
			id INTEGER PRIMARY KEY,
			open_api_spec TEXT,
			openapi_operation TEXT,
			spec_format TEXT CHECK (spec_format <> 'openapi3.0.3')
		)`).Error)
	require.NoError(t, db2.Exec(`INSERT INTO functions (open_api_spec, openapi_operation, spec_format) VALUES ('{}', '{}', NULL)`).Error)
	err = migrateFunctionOpenAPIColumns(db2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "set spec_format for openapi rows")

	// spec_format CHECK rejects the 'legacy' default.
	db3 := newMigrationTestDB(t)
	require.NoError(t, db3.Exec(`
		CREATE TABLE functions (
			id INTEGER PRIMARY KEY,
			spec_format TEXT CHECK (spec_format <> 'legacy')
		)`).Error)
	require.NoError(t, db3.Exec(`INSERT INTO functions (spec_format) VALUES (NULL)`).Error)
	err = migrateFunctionOpenAPIColumns(db3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "set legacy spec_format default")
}

// TestMigrateEnumColumnsLegacyVarcharV9 builds legacy tables whose enum
// columns are still varchar and runs the full string->int swap, then verifies
// the integer-type probe used for idempotency.
func TestMigrateEnumColumnsLegacyVarcharV9(t *testing.T) {
	db := newMigrationTestDB(t)

	require.NoError(t, db.Exec(`CREATE TABLE function_contracts (
		id INTEGER PRIMARY KEY,
		function_id TEXT,
		capability TEXT,
		risk TEXT)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO function_contracts (function_id, capability, risk) VALUES
		('demo.list', 'collection_query', 'safe'),
		('demo.get', 'item_query', 'warning'),
		('demo.create', 'create', 'high'),
		('weird.fn', 'bogus', 'danger')`).Error)

	require.NoError(t, db.Exec(`CREATE TABLE page_proposals (id INTEGER PRIMARY KEY, status TEXT)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO page_proposals (status) VALUES ('pending'), ('accepted'), ('rejected'), ('expired')`).Error)

	require.NoError(t, db.Exec(`CREATE TABLE tickets (id INTEGER PRIMARY KEY, status TEXT)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO tickets (status) VALUES ('open'), ('in_progress'), ('resolved'), ('closed'), ('new')`).Error)

	require.NoError(t, db.Exec(`CREATE TABLE messages (id INTEGER PRIMARY KEY, status TEXT)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO messages (status) VALUES ('unread'), ('read')`).Error)

	require.NoError(t, db.Exec(`CREATE TABLE feedbacks (id INTEGER PRIMARY KEY, status TEXT)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO feedbacks (status) VALUES ('open'), ('triaged'), ('closed')`).Error)

	require.NoError(t, MigrateEnumColumns(db))

	// capability values were translated through the mapping; the unmapped
	// "bogus" fell back to 1 and the function-id inference backfilled it to 7.
	var caps []int
	require.NoError(t, db.Raw(`SELECT capability FROM function_contracts ORDER BY id`).Scan(&caps).Error)
	assert.Equal(t, []int{2, 3, 4, 7}, caps)

	// Risk mapping.
	var risks []int
	require.NoError(t, db.Raw(`SELECT risk FROM function_contracts ORDER BY id`).Scan(&risks).Error)
	assert.Equal(t, []int{1, 2, 3, 4}, risks)

	// Proposal statuses (expired is not in the mapping -> default 1).
	var statuses []int
	require.NoError(t, db.Raw(`SELECT status FROM page_proposals ORDER BY id`).Scan(&statuses).Error)
	assert.Equal(t, []int{1, 2, 3, 1}, statuses)

	// Ticket statuses.
	var tstat []int
	require.NoError(t, db.Raw(`SELECT status FROM tickets ORDER BY id`).Scan(&tstat).Error)
	assert.Equal(t, []int{1, 2, 3, 4, 1}, tstat)

	// Integer-type probes.
	assert.True(t, columnIsInteger(db, "tickets", "status"))
	assert.False(t, columnIsInteger(db, "tickets", "statusX"))

	// A second run is a no-op now that the columns are integers.
	require.NoError(t, MigrateEnumColumns(db))
}

func TestColumnIsIntegerProbeBranchesV9(t *testing.T) {
	db := newMigrationTestDB(t)
	require.NoError(t, db.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, num INTEGER, txt TEXT)`).Error)
	assert.True(t, columnIsInteger(db, "t", "num"))
	assert.False(t, columnIsInteger(db, "t", "txt"))

	// Postgres probe against sqlite errors out -> assumes migrated.
	pgDB := asFakePostgresV9(t, newMigrationTestDB(t))
	assert.True(t, columnIsInteger(pgDB, "t", "num"))

	// Probe failure on a closed sqlite connection also assumes migrated.
	closed := newClosedDB(t)
	assert.True(t, columnIsInteger(closed, "t", "num"))
}

func TestLegacyCleanupBranchesV9(t *testing.T) {
	// nil-db guards.
	require.NoError(t, CleanupLegacyPageTables(nil))
	require.NoError(t, CleanupLegacyUIColumns(nil))
	require.NoError(t, CleanupAllLegacy(nil))
	tables, columns := LegacyCleanupReport(nil)
	assert.Empty(t, tables)
	assert.Empty(t, columns)

	pgDB := asFakePostgresV9(t, newMigrationTestDB(t))
	require.NoError(t, pgDB.Exec(`CREATE TABLE functions (id INTEGER PRIMARY KEY, category_display TEXT)`).Error)
	assert.False(t, tableHasColumn(pgDB, "functions", "category_displayX"))
	// sqlite rejects DROP COLUMN IF EXISTS syntax -> postgres branch errors.
	assert.Error(t, dropColumnFromTable(pgDB, "functions", "category_display"))

	myDB := newMigrationTestDB(t)
	myDB.Dialector = &fakeMySQLDialectorV9{Dialector: myDB.Dialector}
	require.NoError(t, myDB.Exec(`CREATE TABLE functions (id INTEGER PRIMARY KEY, category_display TEXT)`).Error)
	assert.False(t, tableHasColumn(myDB, "functions", "missing_col"))
	// The mysql branch emits plain DROP COLUMN, which sqlite accepts.
	require.NoError(t, dropColumnFromTable(myDB, "functions", "category_display"))

	otherDB := newMigrationTestDB(t)
	otherDB.Dialector = &fakeOtherDialectorV9{Dialector: otherDB.Dialector}
	require.NoError(t, otherDB.Exec(`CREATE TABLE functions (id INTEGER PRIMARY KEY, category_display TEXT)`).Error)
	assert.False(t, tableHasColumn(otherDB, "functions", "missing_col"))
	// Default branch delegates to the gorm migrator.
	require.NoError(t, dropColumnFromTable(otherDB, "functions", "category_display"))

	// sqlite PRAGMA probe on a healthy and on a closed connection.
	plain := newMigrationTestDB(t)
	require.NoError(t, plain.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)`).Error)
	assert.True(t, tableHasColumn(plain, "t", "name"))
	assert.False(t, tableHasColumn(plain, "missing", "name"))
	closed := newClosedDB(t)
	assert.False(t, tableHasColumn(closed, "functions", "category_display"))

	// SQLite refuses to drop UNIQUE columns -> the wrapped error branch.
	uni := newMigrationTestDB(t)
	require.NoError(t, uni.Exec(`CREATE TABLE functions (id INTEGER PRIMARY KEY, category_display TEXT UNIQUE)`).Error)
	err := CleanupLegacyUIColumns(uni)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "drop legacy column category_display")
}

func TestMigrateTermDictionaryDisplayBranchesV9(t *testing.T) {
	// nil db.
	require.NoError(t, MigrateTermDictionaryDisplay(nil))

	// table missing -> early return.
	db := newMigrationTestDB(t)
	require.NoError(t, MigrateTermDictionaryDisplay(db))

	// no legacy columns after a regular migration -> early return.
	require.NoError(t, db.AutoMigrate(&TermDictionary{}))
	require.NoError(t, MigrateTermDictionaryDisplay(db))

	// Find fails: legacy columns exist but the id column is missing.
	db2 := newMigrationTestDB(t)
	require.NoError(t, db2.Exec(`CREATE TABLE term_dictionary (display_zh TEXT, display_en TEXT)`).Error)
	err := MigrateTermDictionaryDisplay(db2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scan legacy term_dictionary rows")

	// Backfill Update fails: display column is missing.
	db3 := newMigrationTestDB(t)
	require.NoError(t, db3.Exec(`CREATE TABLE term_dictionary (id INTEGER PRIMARY KEY, display_zh TEXT)`).Error)
	require.NoError(t, db3.Exec(`INSERT INTO term_dictionary (display_zh) VALUES ('中文')`).Error)
	err = MigrateTermDictionaryDisplay(db3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backfill term_dictionary")

	// Happy path plus non-droppable UNIQUE legacy columns (warn-only).
	db4 := newMigrationTestDB(t)
	require.NoError(t, db4.Exec(`CREATE TABLE term_dictionary (
		id INTEGER PRIMARY KEY,
		display TEXT,
		display_zh TEXT UNIQUE,
		display_en TEXT UNIQUE)`).Error)
	require.NoError(t, db4.Exec(`INSERT INTO term_dictionary (display, display_zh, display_en) VALUES ('{"zh":"旧"}', '中文', 'english')`).Error)
	require.NoError(t, db4.Exec(`INSERT INTO term_dictionary (display, display_zh, display_en) VALUES (NULL, '', '')`).Error)
	require.NoError(t, MigrateTermDictionaryDisplay(db4))

	var zh string
	require.NoError(t, db4.Raw(`SELECT display FROM term_dictionary WHERE id = 1`).Scan(&zh).Error)
	assert.Contains(t, zh, "zh-CN")
}
