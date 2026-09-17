package svc

import (
	"context"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	"github.com/cuihairu/croupier/internal/platform/registry"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// ---- truncate（此前 0%） ----

func TestTruncateShortAndOverflow(t *testing.T) {
	assert.Equal(t, "short", truncate("short", 10))
	assert.Equal(t, "0123456789...", truncate("0123456789X", 10))
}

// ---- StartScheduler / StopScheduler（此前 0%） ----

func TestSchedulerNilGuards(t *testing.T) {
	var nilCtx *ServiceContext
	assert.Nil(t, nilCtx.StartScheduler())
	nilCtx.StopScheduler() // 不 panic

	ctx := &ServiceContext{}
	assert.Nil(t, ctx.StartScheduler()) // 依赖缺失
	ctx.StopScheduler()
}

func TestSchedulerStartStopWithDeps(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/sched.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.TaskSchedule{}, &model.TaskRun{}, &model.TaskScheduleRunLog{}))

	ctx := &ServiceContext{
		TaskScheduleModel: model.NewTaskScheduleModel(db),
		Dispatcher:        dispatch.NewDispatcher(registry.NewStore()),
	}
	mgr := ctx.StartScheduler()
	require.NotNil(t, mgr)
	// 幂等：重复启动返回同一实例
	assert.Same(t, mgr, ctx.StartScheduler())
	ctx.StopScheduler()
}

// ---- openGorm sqlite 分支（44.9%） ----

func TestOpenGormSQLiteVariants(t *testing.T) {
	db, err := openGorm("sqlite", ":memory:")
	require.NoError(t, err)
	require.NoError(t, db.Exec("SELECT 1").Error)

	db2, err := openGorm("sqlite3", t.TempDir()+"/x.db")
	require.NoError(t, err)
	require.NoError(t, db2.Exec("SELECT 1").Error)

	_, err = openGorm("oracle", "whatever")
	assert.Error(t, err)
}

// ---- probeDialect（27.3%）：mysql 命中与全部失败 ----

func TestProbeDialectMySQLViaMock(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	mock.ExpectQuery("SELECT COUNT").WillReturnError(assert.AnError)
	mock.ExpectQuery("SELECT CURRENT_SETTING").WillReturnError(assert.AnError)
	mock.ExpectQuery("SELECT @@version_comment").WillReturnRows(
		sqlmock.NewRows([]string{"v"}).AddRow("MySQL Community"))

	dialect, err := probeDialect(sqlDB)
	require.NoError(t, err)
	assert.Equal(t, "mysql", dialect)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProbeDialectUnknown(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	mock.ExpectQuery("SELECT COUNT").WillReturnError(assert.AnError)
	mock.ExpectQuery("SELECT CURRENT_SETTING").WillReturnError(assert.AnError)
	mock.ExpectQuery("SELECT @@version_comment").WillReturnError(assert.AnError)

	dialect, err := probeDialect(sqlDB)
	assert.Error(t, err)
	assert.Empty(t, dialect)
}

// ---- wrapGorm（61.5%）：mysql 连接包装与探测失败 ----

func TestWrapGormMySQLConnAndProbeFailure(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	mock.ExpectQuery("SELECT COUNT").WillReturnError(assert.AnError)
	mock.ExpectQuery("SELECT CURRENT_SETTING").WillReturnError(assert.AnError)
	mock.ExpectQuery("SELECT @@version_comment").WillReturnRows(
		sqlmock.NewRows([]string{"v"}).AddRow("MySQL"))

	gdb, err := wrapGorm(sqlDB)
	require.NoError(t, err)
	require.NotNil(t, gdb)

	sqlDB2, mock2, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB2.Close() })
	mock2.ExpectQuery("SELECT COUNT").WillReturnError(assert.AnError)
	mock2.ExpectQuery("SELECT CURRENT_SETTING").WillReturnError(assert.AnError)
	mock2.ExpectQuery("SELECT @@version_comment").WillReturnError(assert.AnError)
	_, err = wrapGorm(sqlDB2)
	assert.Error(t, err)
}

// ---- sqliteFileDSN（66.7%） ----

func TestSQLiteFileDSNPragmaInjection(t *testing.T) {
	assert.Contains(t, sqliteFileDSN("/tmp/a.db"), "_pragma=busy_timeout")
	assert.Contains(t, sqliteFileDSN("file:x.db?cache=shared"), "_pragma=busy_timeout")
}

// 0016：function_contracts.timeout_ms 列迁移（幂等 + 缺表跳过）。
func TestContractTimeoutMigration(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/m16.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.FunctionContract{}))
	// 模拟存量库：删列后跑迁移补列
	require.NoError(t, db.Migrator().DropColumn(&model.FunctionContract{}, "TimeoutMs"))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, addContractTimeoutColumn(context.Background(), sqlDB))
	require.True(t, db.Migrator().HasColumn(&model.FunctionContract{}, "TimeoutMs"))

	// 幂等：重复执行不报错
	require.NoError(t, addContractTimeoutColumn(context.Background(), sqlDB))

	// 缺表库（fanout 重放到无该表的 game 库）：跳过不建空壳表
	db2, err := gorm.Open(gsqlite.Open(t.TempDir()+"/m16b.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB2, err := db2.DB()
	require.NoError(t, err)
	require.NoError(t, addContractTimeoutColumn(context.Background(), sqlDB2))
	require.False(t, db2.Migrator().HasTable(&model.FunctionContract{}))
}

// 0021：function_contracts.prev_input/output_schema 列迁移（幂等 + 缺表
// 跳过）。sync-selectors 上线时漏配存量库迁移导致线上 agent 注册 upsert
// 报 column not exist——本用例锁死该路径。
func TestContractPrevSchemaMigration(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/m21.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.FunctionContract{}))
	// 模拟存量库：删列后跑迁移补列
	require.NoError(t, db.Migrator().DropColumn(&model.FunctionContract{}, "PrevInputSchema"))
	require.NoError(t, db.Migrator().DropColumn(&model.FunctionContract{}, "PrevOutputSchema"))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, addContractPrevSchemaColumns(context.Background(), sqlDB))
	require.True(t, db.Migrator().HasColumn(&model.FunctionContract{}, "PrevInputSchema"))
	require.True(t, db.Migrator().HasColumn(&model.FunctionContract{}, "PrevOutputSchema"))

	// 幂等：重复执行不报错
	require.NoError(t, addContractPrevSchemaColumns(context.Background(), sqlDB))

	// 缺表库（fanout 重放到无该表的 game 库）：跳过不建空壳表
	db2, err := gorm.Open(gsqlite.Open(t.TempDir()+"/m21b.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB2, err := db2.DB()
	require.NoError(t, err)
	require.NoError(t, addContractPrevSchemaColumns(context.Background(), sqlDB2))
	require.False(t, db2.Migrator().HasTable(&model.FunctionContract{}))
}

// 0022：term_dictionary.display 列迁移——旧双列存量库补 display 列并回填；
// 已是 JSON 列形态的库幂等；缺表库跳过。
func TestTermDictionaryDisplayMigration(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/m22.db"), &gorm.Config{})
	require.NoError(t, err)
	// 模拟 57fac95df 之前的存量形态：旧双列建表、无 display 列。
	require.NoError(t, db.Exec(`CREATE TABLE term_dictionary (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		domain TEXT NOT NULL,
		term_key TEXT NOT NULL,
		alias TEXT NOT NULL,
		display_zh TEXT,
		display_en TEXT,
		sort_order INTEGER DEFAULT 100,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO term_dictionary
		(domain, term_key, alias, display_zh, display_en, sort_order)
		VALUES ('resource', 'player', 'player', '玩家', 'Player', 10)`).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateTermDictionaryDisplayColumn(context.Background(), sqlDB))

	items, err := model.NewTermDictionaryModel(db).List(t.Context(), "resource")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, map[string]string{"zh-CN": "玩家", "en-US": "Player"}, items[0].Display)

	// 幂等：重复执行不报错
	require.NoError(t, migrateTermDictionaryDisplayColumn(context.Background(), sqlDB))

	// 缺表库：跳过不建空壳表
	db2, err := gorm.Open(gsqlite.Open(t.TempDir()+"/m22b.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB2, err := db2.DB()
	require.NoError(t, err)
	require.NoError(t, migrateTermDictionaryDisplayColumn(context.Background(), sqlDB2))
	require.False(t, db2.Migrator().HasTable(&model.TermDictionary{}))
}

// dispatcherAdapter.StartTask 委托覆盖（此前 0%：调度触发仅在真实 cron 命中时执行）。
func TestDispatcherAdapter_StartTask_Delegates(t *testing.T) {
	called := false
	adapter := dispatcherAdapter{d: &fakeDispatcher{onStart: func() { called = true }}}
	resp, err := adapter.StartTask(context.Background(), &sdkv1.InvokeRequest{FunctionId: "f"})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, called, "应委托给 Dispatcher.StartTaskRequest")
}

type fakeDispatcher struct {
	onStart func()
}

func (f *fakeDispatcher) StartTaskRequest(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error) {
	f.onStart()
	return &sdkv1.StartTaskResponse{TaskId: "t-1"}, nil
}

// 0023：component_templates.params/digest 列迁移（幂等 + 缺表跳过）。
// U6 加 Params、U11 加 Digest 时均只改了模型，存量 game 库过 baseline 后
// 不再跑 AutoMigrate——线上创建/更新模板报 column "params" does not
// exist。本用例锁死补列路径。
func TestComponentTemplateColumnsMigration(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/m23.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ComponentTemplate{}))
	// 模拟存量库：删掉两列后跑迁移补列
	require.NoError(t, db.Migrator().DropColumn(&model.ComponentTemplate{}, "Params"))
	require.NoError(t, db.Migrator().DropColumn(&model.ComponentTemplate{}, "Digest"))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateComponentTemplateColumns(context.Background(), sqlDB))
	require.True(t, db.Migrator().HasColumn(&model.ComponentTemplate{}, "Params"))
	require.True(t, db.Migrator().HasColumn(&model.ComponentTemplate{}, "Digest"))

	// 幂等：重复执行不报错
	require.NoError(t, migrateComponentTemplateColumns(context.Background(), sqlDB))

	// 缺表库（fanout 重放到无该表的 game 库）：跳过不建空壳表
	db2, err := gorm.Open(gsqlite.Open(t.TempDir()+"/m23b.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB2, err := db2.DB()
	require.NoError(t, err)
	require.NoError(t, migrateComponentTemplateColumns(context.Background(), sqlDB2))
	require.False(t, db2.Migrator().HasTable(&model.ComponentTemplate{}))
}

// TestComponentTemplateColumnsMigration_LegacyTableShape 复刻线上存量表形
// （2026-09-12 部署事故根因）：key 列唯一约束是建表期老写法（约束名
// component_templates_key_key），与模型 uniqueIndex 默认名不一致；且无
// params/digest 列。迁移必须逐列 AddColumn 补列且不动既有约束——若实现
// 退化为整模型 AutoMigrate，索引对齐会在 postgres 上报 constraint does
// not exist 直接 panic（sqlite 不报但行为同源）。
func TestComponentTemplateColumnsMigration_LegacyTableShape(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/m23c.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE component_templates (
		id integer primary key autoincrement,
		created_at datetime, updated_at datetime, deleted_at datetime,
		key text NOT NULL,
		name json NOT NULL,
		description json, category text, icon text, required_functions json,
		tree json NOT NULL, builtin numeric DEFAULT 0, created_by text,
		CONSTRAINT component_templates_key_key UNIQUE (key))`).Error)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateComponentTemplateColumns(context.Background(), sqlDB))
	require.True(t, db.Migrator().HasColumn(&model.ComponentTemplate{}, "Params"))
	require.True(t, db.Migrator().HasColumn(&model.ComponentTemplate{}, "Digest"))
	// 既有唯一约束（origin='u' 的自动索引）原样保留（不触发索引改名/删除）
	var uniIdx []struct {
		Origin string
	}
	require.NoError(t, db.Raw("PRAGMA index_list(component_templates)").Scan(&uniIdx).Error)
	require.NotEmpty(t, uniIdx, "legacy unique constraint must be untouched")
	// 补列后表可正常写入（迁移未破坏表结构）
	require.NoError(t, db.Exec(
		`INSERT INTO component_templates (created_at, updated_at, key, name, tree, builtin)
		 VALUES (datetime(), datetime(), 'smoke', '{"zh-CN":"冒烟"}', '[]', 0)`).Error)
	// 幂等
	require.NoError(t, migrateComponentTemplateColumns(context.Background(), sqlDB))
}

// 0024：软删除残留行清理迁移（幂等 + 缺表跳过）。删除路径改硬删后，
// 存量库里历史软删行仍占着物理唯一索引（同 key 重建 duplicate-key 500，
// 线上实证 page_specs id=60 / component_templates 5 行）。本用例锁死
// 五张表的残留清除路径。
func TestSoftDeleteResidueCleanupMigration(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/m24.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.PageSpec{}, &model.PageProposal{},
		&model.ComponentTemplate{}, &model.OpenAPISourceBinding{}, &model.RegistrationWarningDB{},
	))

	// 各表塞一行软删残留 + 一行活跃行。
	require.NoError(t, db.Exec(`INSERT INTO page_specs (created_at, updated_at, deleted_at, game_id, env, page_key, type, spec_json)
		VALUES (datetime(), datetime(), datetime(), 'demo', 'dev', 'legacy', 'composite', '{}'),
		(datetime(), datetime(), NULL, 'demo', 'dev', 'live', 'composite', '{}')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO page_proposals (created_at, updated_at, deleted_at, game_id, env, proposal_key, page_key)
		VALUES (datetime(), datetime(), datetime(), 'demo', 'dev', 'legacy', 'legacy'),
		(datetime(), datetime(), NULL, 'demo', 'dev', 'live', 'live')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO component_templates (created_at, updated_at, deleted_at, key, name, tree, builtin)
		VALUES (datetime(), datetime(), datetime(), 'legacy', '{}', '[]', 0),
		(datetime(), datetime(), NULL, 'live', '{}', '[]', 0)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO openapi_source_bindings (created_at, updated_at, deleted_at, game_id, env, source_id, binding_id, operation_id, kind)
		VALUES (datetime(), datetime(), datetime(), 'demo', 'dev', 'src', 'legacy', 'op', 'operation'),
		(datetime(), datetime(), NULL, 'demo', 'dev', 'src', 'live', 'op', 'operation')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO registration_warnings (created_at, updated_at, deleted_at, key, game_id, env, agent_id, function_id, code, message, status, count, first_seen, last_seen)
		VALUES (datetime(), datetime(), datetime(), 'legacy', 'demo', 'dev', 'a1', 'f1', 'schema_mismatch', 'm', 'resolved', 1, datetime(), datetime()),
		(datetime(), datetime(), NULL, 'live', 'demo', 'dev', 'a1', 'f1', 'schema_mismatch', 'm', 'active', 1, datetime(), datetime())`).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateSoftDeleteResidue(context.Background(), sqlDB))

	// 残留行被物理清除，活跃行保留。
	for _, table := range []string{
		"page_specs", "page_proposals",
		"component_templates", "openapi_source_bindings", "registration_warnings",
	} {
		var dead, alive int64
		require.NoError(t, db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE deleted_at IS NOT NULL", table)).Scan(&dead).Error)
		require.NoError(t, db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE deleted_at IS NULL", table)).Scan(&alive).Error)
		assert.Zero(t, dead, table+" 的软删残留应被清除")
		assert.Equal(t, int64(1), alive, table+" 的活跃行应保留")
	}

	// 幂等：重复执行不报错。
	require.NoError(t, migrateSoftDeleteResidue(context.Background(), sqlDB))

	// 缺表库（不含这些模型的库）：跳过不建空壳表。
	db2, err := gorm.Open(gsqlite.Open(t.TempDir()+"/m24b.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB2, err := db2.DB()
	require.NoError(t, err)
	require.NoError(t, migrateSoftDeleteResidue(context.Background(), sqlDB2))
	require.False(t, db2.Migrator().HasTable(&model.PageSpec{}))
}

// TestRoleAdminSoftDeleteCleanupMigration 验证 0026：roles/admins 的软删残留
// 行被物理清除、活跃行保留、admin_roles 悬挂关联收口、幂等、缺表跳过。
func TestRoleAdminSoftDeleteCleanupMigration(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/m26.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Role{}, &model.Admin{}, &model.AdminRole{}))

	// roles/admins 各塞一行软删残留 + 一行活跃行；admin_roles 指向残留角色 + 活跃角色。
	require.NoError(t, db.Exec(`INSERT INTO roles (created_at, updated_at, deleted_at, name)
		VALUES (datetime(), datetime(), datetime(), 'legacy-role'),
		(datetime(), datetime(), NULL, 'live-role')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO admins (created_at, updated_at, deleted_at, username)
		VALUES (datetime(), datetime(), datetime(), 'legacy-admin'),
		(datetime(), datetime(), NULL, 'live-admin')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO admin_roles (created_at, updated_at, admin_id, role_id)
		VALUES (datetime(), datetime(), 1, 1),
		(datetime(), datetime(), 2, 2)`).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateRoleAdminSoftDelete(context.Background(), sqlDB))

	for _, table := range []string{"roles", "admins"} {
		var dead, alive int64
		require.NoError(t, db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE deleted_at IS NOT NULL", table)).Scan(&dead).Error)
		require.NoError(t, db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE deleted_at IS NULL", table)).Scan(&alive).Error)
		assert.Zero(t, dead, table+" 的软删残留应被清除")
		assert.Equal(t, int64(1), alive, table+" 的活跃行应保留")
	}
	var dangling int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM admin_roles ar
		LEFT JOIN roles r ON r.id = ar.role_id WHERE r.id IS NULL`).Scan(&dangling).Error)
	assert.Zero(t, dangling, "admin_roles 悬挂关联应被清除")

	// 幂等：重复执行不报错。
	require.NoError(t, migrateRoleAdminSoftDelete(context.Background(), sqlDB))

	// 缺表库：跳过不建空壳表。
	db2, err := gorm.Open(gsqlite.Open(t.TempDir()+"/m26b.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB2, err := db2.DB()
	require.NoError(t, err)
	require.NoError(t, migrateRoleAdminSoftDelete(context.Background(), sqlDB2))
	require.False(t, db2.Migrator().HasTable(&model.Role{}))
}

// TestRoleAdminSoftDeleteCleanupMigration_ErrorBranches 补 0026 的错误与
// 跳过分支：wrapGorm 探测失败、表存在但无 deleted_at 列（HasColumn 跳过）、
// purge / 悬挂收口的 DELETE 失败（raw Exec 不走 gorm 回调，用 sqlite
// RAISE 触发器注入）。
func TestRoleAdminSoftDeleteCleanupMigration_ErrorBranches(t *testing.T) {
	// wrapGorm 方言探测全失败 → 返回错误。
	t.Run("wrapGorm 探测失败", func(t *testing.T) {
		sqlDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = sqlDB.Close() })
		mock.ExpectQuery("SELECT COUNT").WillReturnError(assert.AnError)
		mock.ExpectQuery("SELECT CURRENT_SETTING").WillReturnError(assert.AnError)
		mock.ExpectQuery("SELECT @@version_comment").WillReturnError(assert.AnError)
		assert.Error(t, migrateRoleAdminSoftDelete(context.Background(), sqlDB))
	})

	// 表存在但缺 deleted_at 列（列裁剪过的存量形态）→ 跳过且不建列。
	t.Run("缺 deleted_at 列跳过", func(t *testing.T) {
		db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/m26c.db"), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.Exec(`CREATE TABLE roles (id integer primary key, name text)`).Error)
		require.NoError(t, db.Exec(`CREATE TABLE admins (id integer primary key, username text)`).Error)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, migrateRoleAdminSoftDelete(context.Background(), sqlDB))
		assert.False(t, db.Migrator().HasColumn(&model.Role{}, "DeletedAt"), "不应给存量表补列")
	})

	// roles 表 RAISE 触发器拦截 DELETE → purge 失败向上传播。
	t.Run("purge DELETE 失败", func(t *testing.T) {
		db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/m26d.db"), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&model.Role{}))
		require.NoError(t, db.Exec(`INSERT INTO roles (created_at, updated_at, deleted_at, name)
			VALUES (datetime(), datetime(), datetime(), 'legacy')`).Error)
		require.NoError(t, db.Exec(`CREATE TRIGGER block_purge BEFORE DELETE ON roles
			BEGIN SELECT RAISE(ABORT, 'injected purge failure'); END`).Error)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		err = migrateRoleAdminSoftDelete(context.Background(), sqlDB)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "purge soft-deleted rows from roles")
	})

	// admin_roles 表 RAISE 触发器拦截悬挂收口 DELETE（roles/admins purge 空转成功）。
	t.Run("悬挂收口 DELETE 失败", func(t *testing.T) {
		db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/m26e.db"), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&model.Role{}, &model.Admin{}, &model.AdminRole{}))
		require.NoError(t, db.Exec(`INSERT INTO roles (created_at, updated_at, name)
			VALUES (datetime(), datetime(), 'live')`).Error)
		require.NoError(t, db.Exec(`INSERT INTO admin_roles (created_at, updated_at, admin_id, role_id)
			VALUES (datetime(), datetime(), 1, 999)`).Error)
		require.NoError(t, db.Exec(`CREATE TRIGGER block_dangling BEFORE DELETE ON admin_roles
			BEGIN SELECT RAISE(ABORT, 'injected dangling failure'); END`).Error)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		err = migrateRoleAdminSoftDelete(context.Background(), sqlDB)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "purge dangling admin_roles")
	})
}
