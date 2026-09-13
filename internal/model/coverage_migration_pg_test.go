// 覆盖目标：migration.go 的 postgres 自愈循环分支（AutoMigrate / migrateModels）、
// tryFixPostgresMissingConstraint 的改名成功路径、tryRenamePostgresDefaultUniqueConstraint
// 的 RENAME 执行路径、postgresHasConstraint 的成功返回、columnIsInteger 的 postgres
// 探测成功分支、dropLegacyPageUniqueIndexes 的 postgres DropIndex 失败分支，以及
// AutoMigrate 主循环内 migrateFunctionOpenAPIColumns / MigrateTermDictionaryDisplay
// 的错误出口。
package model

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// scriptPostgresMigrator 包装真实 sqlite migrator：AutoMigrate / DropIndex
// 可脚本化返回指定结果，其余方法保持原行为。
type scriptPostgresMigrator struct {
	gorm.Migrator
	// autoMigrateScript 非 nil 时启用脚本：每次调用弹出一项；
	// 项为非 nil error 时返回之，脚本耗尽或项为 nil 时返回假成功。
	autoMigrateScript []error
	dropIndexErr      error
}

func (m *scriptPostgresMigrator) AutoMigrate(dst ...interface{}) error {
	if m.autoMigrateScript != nil {
		if len(m.autoMigrateScript) == 0 {
			return nil
		}
		err := m.autoMigrateScript[0]
		m.autoMigrateScript = m.autoMigrateScript[1:]
		return err
	}
	return m.Migrator.AutoMigrate(dst...)
}

func (m *scriptPostgresMigrator) DropIndex(value interface{}, name string) error {
	if m.dropIndexErr != nil {
		return m.dropIndexErr
	}
	return m.Migrator.DropIndex(value, name)
}

type scriptPostgresDialector struct {
	gorm.Dialector
	mig *scriptPostgresMigrator
}

func (d *scriptPostgresDialector) Name() string { return "postgres" }

func (d *scriptPostgresDialector) Migrator(db *gorm.DB) gorm.Migrator { return d.mig }

// asScriptedPostgres 把 sqlite db 的方言名换成 postgres 并接管 AutoMigrate /
// DropIndex，用于驱动自愈重试循环的各分支。
func asScriptedPostgres(t *testing.T, db *gorm.DB, script []error, dropIndexErr error) *gorm.DB {
	t.Helper()
	mig := &scriptPostgresMigrator{
		Migrator:          db.Migrator(),
		autoMigrateScript: script,
		dropIndexErr:      dropIndexErr,
	}
	db.Dialector = &scriptPostgresDialector{Dialector: db.Dialector, mig: mig}
	return db
}

func pgConstraintMissingErr() error {
	return fmt.Errorf(`ERROR: constraint "uni_functions_function_id" of relation "functions" does not exist`)
}

// swapConnPool 把 gorm 连接池替换为 sqlmock 的 *sql.DB，使原生 SQL
// （pg_constraint 探测等）打到 mock 上。
func swapConnPool(db *gorm.DB, sqlDB *sql.DB) {
	db.ConnPool = sqlDB
	if db.Statement != nil {
		db.Statement.ConnPool = sqlDB
	}
}

// migrateModels：第一次失败命中 uni_ 约束缺失（可自愈）→ continue，
// 第二次成功 → return nil。
func TestMigrateModelsScriptedPostgresLoopRecovers(t *testing.T) {
	db := asScriptedPostgres(t, newMigrationTestDB(t), []error{pgConstraintMissingErr()}, nil)
	require.NoError(t, migrateModels(db, MetaModels()))
}

// migrateModels：不可自愈错误直接返回。
func TestMigrateModelsScriptedPostgresLoopUnfixable(t *testing.T) {
	db := asScriptedPostgres(t, newMigrationTestDB(t), []error{errors.New("disk exploded")}, nil)
	err := migrateModels(db, MetaModels())
	require.Error(t, err)
	assert.EqualError(t, err, "disk exploded")
}

// migrateModels：连续 5 次可自愈失败耗尽预算，返回 lastErr。
func TestMigrateModelsScriptedPostgresLoopExhausted(t *testing.T) {
	script := make([]error, 5)
	for i := range script {
		script[i] = pgConstraintMissingErr()
	}
	db := asScriptedPostgres(t, newMigrationTestDB(t), script, nil)
	err := migrateModels(db, MetaModels())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uni_functions_function_id")
}

// AutoMigrate：postgres 主循环第一次可自愈失败 → continue，第二次成功走完全链。
func TestAutoMigrateScriptedPostgresLoopRecovers(t *testing.T) {
	db := asScriptedPostgres(t, newMigrationTestDB(t), []error{pgConstraintMissingErr()}, nil)
	require.NoError(t, AutoMigrate(db))
}

// AutoMigrate：postgres 主循环遇不可自愈错误立即返回。
func TestAutoMigrateScriptedPostgresLoopUnfixable(t *testing.T) {
	db := asScriptedPostgres(t, newMigrationTestDB(t), []error{errors.New("fatal")}, nil)
	err := AutoMigrate(db)
	require.Error(t, err)
	assert.EqualError(t, err, "fatal")
}

// AutoMigrate：postgres 主循环 5 次可自愈失败耗尽预算后返回 lastErr。
func TestAutoMigrateScriptedPostgresLoopExhausted(t *testing.T) {
	script := make([]error, 5)
	for i := range script {
		script[i] = pgConstraintMissingErr()
	}
	db := asScriptedPostgres(t, newMigrationTestDB(t), script, nil)
	err := AutoMigrate(db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uni_functions_function_id")
}

// 设计债回归（AutoMigrate 兜底重试不可达已删除）：循环耗尽必然
// lastErr 非 nil，直接返回 lastErr，不得再执行循环外的第 6 次
// autoMigrateAllModels 兜底重试。脚本预置 6 项可自愈错误：前 5 项
// 供循环耗尽，第 6 项是哨兵——若兜底重试仍存在会被消费，断言其
// 必须残留。
func TestAutoMigrateScriptedPostgresLoopExhaustedSkipsFallbackRetry(t *testing.T) {
	script := make([]error, 6)
	for i := range script {
		script[i] = pgConstraintMissingErr()
	}
	db := newMigrationTestDB(t)
	mig := &scriptPostgresMigrator{Migrator: db.Migrator(), autoMigrateScript: script}
	db.Dialector = &scriptPostgresDialector{Dialector: db.Dialector, mig: mig}

	err := AutoMigrate(db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uni_functions_function_id")
	assert.Len(t, mig.autoMigrateScript, 1,
		"循环耗尽后必须直接返回 lastErr，禁止第 6 次 AutoMigrate 兜底重试")
}

// AutoMigrate：enum 列类型置换失败（UNIQUE 列不可 DROP）在进入 postgres
// 主循环之前就返回。
func TestAutoMigrateEnumSwapFailurePropagatesEarly(t *testing.T) {
	db := newMigrationTestDB(t)
	require.NoError(t, db.Exec(`CREATE TABLE messages (id INTEGER PRIMARY KEY, status TEXT UNIQUE)`).Error)
	err := AutoMigrate(db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "drop legacy column messages.status")
}

// AutoMigrate：postgres 主循环成功后 migrateFunctionOpenAPIColumns 报错
// （生成列不可回填）。
func TestAutoMigrateScriptedPostgresOpenAPIColumnsError(t *testing.T) {
	db := asScriptedPostgres(t, newMigrationTestDB(t), []error{}, nil)
	require.NoError(t, db.Exec(`
		CREATE TABLE functions (
			id INTEGER PRIMARY KEY,
			open_api_spec TEXT GENERATED ALWAYS AS ('x') VIRTUAL,
			openapi_operation TEXT
		)`).Error)
	err := AutoMigrate(db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backfill open_api_spec")
}

// AutoMigrate：postgres 主循环成功后 MigrateTermDictionaryDisplay 报错
// （旧列存在但缺少 id 列导致扫描失败）。
func TestAutoMigrateScriptedPostgresTermDictionaryError(t *testing.T) {
	db := asScriptedPostgres(t, newMigrationTestDB(t), []error{}, nil)
	require.NoError(t, db.Exec(`CREATE TABLE term_dictionary (display_zh TEXT, display_en TEXT)`).Error)
	err := AutoMigrate(db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scan legacy term_dictionary rows")
}

// dropLegacyPageUniqueIndexes：postgres 分支 DropIndex 失败被包装返回。
func TestDropLegacyPageUniqueIndexesScriptedDropIndexError(t *testing.T) {
	db := newMigrationTestDB(t)
	require.NoError(t, db.AutoMigrate(&PageSpec{}))
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX uni_page_specs_page_key ON page_specs (game_id, env, page_key)`).Error)
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX page_specs_page_key_key ON page_specs (game_id, env, page_key)`).Error)

	db = asScriptedPostgres(t, db, nil, errors.New("drop rejected"))
	err := dropLegacyPageUniqueIndexes(db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "drop legacy page index uni_page_specs_page_key")
}

// tryRenamePostgresDefaultUniqueConstraint：legacy <table>_<col>_key 存在且
// 期望的 uni_<table>_<col> 不存在 → 执行 RENAME 并返回 true。
func TestTryRenamePostgresLegacyKeySuccess(t *testing.T) {
	db := newMigrationTestDB(t)
	db.Dialector = &fakePostgresDialectorV9{Dialector: db.Dialector}
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	swapConnPool(db, sqlDB)

	constraintQuery := regexp.QuoteMeta("pg_constraint")
	mock.ExpectQuery(constraintQuery).WithArgs("functions", "functions_function_id_key").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(constraintQuery).WithArgs("functions", "uni_functions_function_id").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec(regexp.QuoteMeta(`ALTER TABLE "functions" RENAME CONSTRAINT "functions_function_id_key" TO "uni_functions_function_id"`)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	assert.True(t, tryRenamePostgresDefaultUniqueConstraint(db, "functions", "uni_functions_function_id"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// tryFixPostgresMissingConstraint：改名路径成功 → 直接返回 true。
func TestTryFixPostgresMissingConstraintRenameSucceeds(t *testing.T) {
	db := newMigrationTestDB(t)
	db.Dialector = &fakePostgresDialectorV9{Dialector: db.Dialector}
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	swapConnPool(db, sqlDB)

	constraintQuery := regexp.QuoteMeta("pg_constraint")
	mock.ExpectQuery(constraintQuery).WithArgs("functions", "functions_function_id_key").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(constraintQuery).WithArgs("functions", "uni_functions_function_id").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec(regexp.QuoteMeta(`ALTER TABLE "functions" RENAME CONSTRAINT`)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	assert.True(t, tryFixPostgresMissingConstraint(db, pgConstraintMissingErr()))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// postgresHasConstraint：查询成功且存在 / 不存在两种返回。
func TestPostgresHasConstraintQuerySucceeds(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	constraintQuery := regexp.QuoteMeta("pg_constraint")
	mock.ExpectQuery(constraintQuery).WithArgs("t", "t_c_key").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(constraintQuery).WithArgs("t", "uni_t_c").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	assert.True(t, postgresHasConstraint(sqlDB, "t", "t_c_key"))
	assert.False(t, postgresHasConstraint(sqlDB, "t", "uni_t_c"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// columnIsInteger：postgres 探测成功——integer 家族返回 true，其余 false。
func TestColumnIsIntegerPostgresProbeSucceeds(t *testing.T) {
	db := newMigrationTestDB(t)
	db.Dialector = &fakePostgresDialectorV9{Dialector: db.Dialector}
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	swapConnPool(db, sqlDB)

	probe := regexp.QuoteMeta("information_schema.columns")
	mock.ExpectQuery(probe).WithArgs("t", "num").
		WillReturnRows(sqlmock.NewRows([]string{"data_type"}).AddRow("integer"))
	mock.ExpectQuery(probe).WithArgs("t", "txt").
		WillReturnRows(sqlmock.NewRows([]string{"data_type"}).AddRow("character varying"))

	assert.True(t, columnIsInteger(db, "t", "num"))
	assert.False(t, columnIsInteger(db, "t", "txt"))
	assert.NoError(t, mock.ExpectationsWereMet())
}
