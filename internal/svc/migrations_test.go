package svc

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/db/migrate"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "migrate.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

// TestGoMigrationsRegistered pins the numbered Go migrations so the provider
// sees exactly versions 1..4 on the production path.
func TestGoMigrationsRegistered(t *testing.T) {
	db := openMigrationTestDB(t)
	version, err := migrate.EnsureUpToDate(context.Background(), db, migrate.ScopeSingle, autoMigrate)
	if err != nil {
		t.Fatalf("EnsureUpToDate: %v", err)
	}
	if version != migrate.MinimumRequiredVersion {
		t.Fatalf("version = %d, want %d (Go migrations not collected by provider?)", version, migrate.MinimumRequiredVersion)
	}

	// Second boot is a no-op catch-up and must not re-run the baseline.
	calls := 0
	wrapped := func(db *gorm.DB) error {
		calls++
		return autoMigrate(db)
	}
	if _, err := migrate.EnsureUpToDate(context.Background(), db, migrate.ScopeSingle, wrapped); err != nil {
		t.Fatalf("second EnsureUpToDate: %v", err)
	}
	if calls != 0 {
		t.Fatalf("baseline ran %d times on up-to-date database, want 0", calls)
	}
}

// TestEnsureUpToDate_ConvertsLegacyVarcharEnum simulates a pre-versioning
// database whose enum column is still varchar: the baseline bridge must
// convert values (not destroy them) and the Go migrations must complete.
func TestEnsureUpToDate_ConvertsLegacyVarcharEnum(t *testing.T) {
	db := openMigrationTestDB(t)
	if err := db.Exec("CREATE TABLE function_contracts (id INTEGER PRIMARY KEY, capability VARCHAR(16))").Error; err != nil {
		t.Fatalf("seed legacy table: %v", err)
	}
	if err := db.Exec("INSERT INTO function_contracts (id, capability) VALUES (1, 'delete'), (2, 'create')").Error; err != nil {
		t.Fatalf("seed legacy rows: %v", err)
	}

	if _, err := migrate.EnsureUpToDate(context.Background(), db, migrate.ScopeSingle, autoMigrate); err != nil {
		t.Fatalf("EnsureUpToDate: %v", err)
	}

	var caps []int
	if err := db.Raw("SELECT capability FROM function_contracts ORDER BY id").Scan(&caps).Error; err != nil {
		t.Fatalf("read back capability: %v", err)
	}
	if len(caps) != 2 || caps[0] != 6 || caps[1] != 4 {
		t.Fatalf("capability values = %v, want [6 4] ('delete'->6, 'create'->4)", caps)
	}
}

// TestProbeDialect_Sqlite covers the dialect probe used by Go migrations.
func TestProbeDialect_Sqlite(t *testing.T) {
	db := openMigrationTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql.DB: %v", err)
	}
	dialect, err := probeDialect(sqlDB)
	if err != nil {
		t.Fatalf("probeDialect: %v", err)
	}
	if dialect != "sqlite" {
		t.Fatalf("dialect = %q, want sqlite", dialect)
	}
	wrapped, err := wrapGorm(sqlDB)
	if err != nil {
		t.Fatalf("wrapGorm: %v", err)
	}
	if wrapped.Dialector == nil || wrapped.Name() != "sqlite" {
		t.Fatalf("wrapped dialector = %+v, want sqlite", wrapped.Dialector)
	}
}

// TestGoMigrations_TaskSchedulesCatchUp 回归：已过 baseline 的存量库
// （不再跑 AutoMigrate）通过 0014 catch-up 拿到 task_schedules 两张表，
// 否则 GET /api/v1/schedules 在部署库上 500（表缺失）。
func TestGoMigrations_TaskSchedulesCatchUp(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()

	// 模拟存量部署：只跑 baseline（AutoMigrate 全量模型）+ goose 到 0013
	// 时代的表集合，其中 TaskSchedule 两张表被当作「尚未存在的 0014 新表」。
	if err := autoMigrate(db); err != nil {
		t.Fatalf("autoMigrate: %v", err)
	}
	// 删掉 0014 要补的表，模拟 6aba002b6 之前 baseline 过的库。
	if err := db.Migrator().DropTable("task_schedules", "task_schedule_run_logs"); err != nil {
		t.Fatalf("drop: %v", err)
	}
	if _, err := migrate.EnsureUpToDate(ctx, db, migrate.ScopeSingle, func(db *gorm.DB) error {
		return nil // baseline 已完成，禁止再跑 AutoMigrate
	}); err != nil {
		t.Fatalf("EnsureUpToDate: %v", err)
	}
	if !db.Migrator().HasTable("task_schedules") {
		t.Fatal("task_schedules 未由 0014 迁移创建")
	}
	if !db.Migrator().HasTable("task_schedule_run_logs") {
		t.Fatal("task_schedule_run_logs 未由 0014 迁移创建")
	}
}

// TestGoMigrations_AgentSessionAddrCatchUp 回归：存量 meta 库通过 0015
// 拿到 agent_sessions.addr 列（HA 跨实例节点视图的 IP 依赖）；
// game 库（无该表）重放时不建空壳表。
func TestGoMigrations_AgentSessionAddrCatchUp(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()

	if err := reg.MigrateAgentSessions(db); err != nil {
		t.Fatalf("migrate agent sessions: %v", err)
	}
	// 模拟 0014 时代的旧 schema（无 addr 列）——sqlite 的 DropColumn 会
	// 重建表并丢失唯一索引，直接按旧结构建表更真实。
	if err := db.Migrator().DropTable(&reg.AgentSessionDB{}); err != nil {
		t.Fatalf("drop table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE agent_sessions (
		id integer primary key autoincrement,
		agent_id text not null, game_id text, env text, version text,
		region text, zone text, labels text, functions text, providers text,
		expire_at datetime, last_seen datetime,
		created_at datetime, updated_at datetime, deleted_at datetime)`).Error; err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX idx_agent_sessions_agent ON agent_sessions(agent_id)").Error; err != nil {
		t.Fatalf("create unique index: %v", err)
	}
	if _, err := migrate.EnsureUpToDate(ctx, db, migrate.ScopeSingle, func(db *gorm.DB) error {
		return nil
	}); err != nil {
		t.Fatalf("EnsureUpToDate: %v", err)
	}
	if !db.Migrator().HasColumn(&reg.AgentSessionDB{}, "Addr") {
		t.Fatal("addr column not backfilled by 0015")
	}

	// 持久化往返：Addr 落库并回读。
	m := reg.NewAgentSessionModel(db)
	if err := m.Upsert(ctx, &reg.AgentSession{
		AgentID: "ag-1", GameID: "g", Env: "prod",
		Addr: "10.1.2.3:54321", ExpireAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	sessions, err := m.LoadActiveSessions(ctx)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("load: %v (n=%d)", err, len(sessions))
	}
	if sessions[0].Addr != "10.1.2.3:54321" {
		t.Fatalf("addr round-trip = %q, want 10.1.2.3:54321", sessions[0].Addr)
	}
}

// TestGoMigrations_ContractExecutionStateCatchUp 回归（D2/T3）：已过
// baseline 的存量库（无 execution_state 列、已有契约行）经 0025 catch-up
// 拿到该列，存量行随 DEFAULT 回填 bound；显式写入 unbound 的新行往返保持。
func TestGoMigrations_ContractExecutionStateCatchUp(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()

	if err := autoMigrate(db); err != nil {
		t.Fatalf("autoMigrate: %v", err)
	}
	// 先落一行 0025 时代之前的契约，再删列模拟存量形态（sqlite 的
	// DropColumn 重建表但保留行数据）。
	if err := db.Exec(`INSERT INTO function_contracts
		(game_id, env, function_id, execution, created_at, updated_at)
		VALUES ('g', 'e', 'player.get', 'sync', datetime('now'), datetime('now'))`).Error; err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}
	if err := db.Migrator().DropColumn(&model.FunctionContract{}, "ExecutionState"); err != nil {
		t.Fatalf("drop column: %v", err)
	}
	if _, err := migrate.EnsureUpToDate(ctx, db, migrate.ScopeSingle, func(db *gorm.DB) error {
		return nil // baseline 已完成，禁止再跑 AutoMigrate
	}); err != nil {
		t.Fatalf("EnsureUpToDate: %v", err)
	}
	if !db.Migrator().HasColumn(&model.FunctionContract{}, "ExecutionState") {
		t.Fatal("execution_state column not backfilled by 0025")
	}
	var state string
	if err := db.Raw("SELECT execution_state FROM function_contracts WHERE function_id = 'player.get'").Scan(&state).Error; err != nil {
		t.Fatalf("read legacy row state: %v", err)
	}
	if state != "bound" {
		t.Fatalf("legacy row execution_state = %q, want bound (DEFAULT backfill)", state)
	}

	// 新行显式 unbound 往返保持（T4 上传管线的写入形态）。
	if err := db.Exec("UPDATE function_contracts SET execution_state = 'unbound' WHERE function_id = 'player.get'").Error; err != nil {
		t.Fatalf("flip to unbound: %v", err)
	}
	state = ""
	if err := db.Raw("SELECT execution_state FROM function_contracts WHERE function_id = 'player.get'").Scan(&state).Error; err != nil {
		t.Fatalf("read flipped state: %v", err)
	}
	if state != "unbound" {
		t.Fatalf("execution_state round-trip = %q, want unbound", state)
	}

	// gorm 零值插入走列 DEFAULT → bound。
	if err := db.Exec(`INSERT INTO function_contracts
		(game_id, env, function_id, execution, created_at, updated_at)
		VALUES ('g', 'e', 'mail.send', 'sync', datetime('now'), datetime('now'))`).Error; err != nil {
		t.Fatalf("insert default row: %v", err)
	}
	state = ""
	if err := db.Raw("SELECT execution_state FROM function_contracts WHERE function_id = 'mail.send'").Scan(&state).Error; err != nil {
		t.Fatalf("read default row state: %v", err)
	}
	if state != "bound" {
		t.Fatalf("new row execution_state = %q, want bound (column DEFAULT)", state)
	}
}

// TestAddContractExecutionStateColumnIdempotent：列已存在时 0025 跳过
// （幂等）；表不存在的库（meta 表在 game 库重放等场景）也跳过。
func TestAddContractExecutionStateColumnIdempotent(t *testing.T) {
	sqlDB := openRawSQLiteDBG(t)
	if err := addContractExecutionStateColumn(context.Background(), sqlDB); err != nil {
		t.Fatalf("missing table should skip, got %v", err)
	}

	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "idem.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.FunctionContract{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqlDB2, err := db.DB()
	if err != nil {
		t.Fatalf("sql.DB: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := addContractExecutionStateColumn(context.Background(), sqlDB2); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
}

// TestGoMigrations_MenuItemTablesCatchUp 回归（T-M1/T-M4）：已过 baseline
// 的存量库（无 menu_items 表、无 page_specs.menu_id 列、已有页面行）经
// 0027 catch-up 建表补列，存量行保留——线上 postgres 实证：menus API
// 因表缺失 500、页面保存/发布链因全字段 INSERT 报 column "menu_id"
// does not exist 中断（sqlite/dev 环境走 AutoMigrateGame 建全列，CI 拦不住）。
func TestGoMigrations_MenuItemTablesCatchUp(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()

	if err := autoMigrate(db); err != nil {
		t.Fatalf("autoMigrate: %v", err)
	}
	// 先落一行菜单时代之前的页面，再删列 + 删表模拟存量形态（sqlite 的
	// DropColumn 重建表但保留行数据）。
	if err := db.Exec(`INSERT INTO page_specs
		(game_id, env, page_key, status, spec_json, created_at, updated_at)
		VALUES ('demo_game', 'dev', 'resource--player', 'published', '{}', datetime('now'), datetime('now'))`).Error; err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}
	if err := db.Migrator().DropColumn(&model.PageSpec{}, "MenuID"); err != nil {
		t.Fatalf("drop menu_id: %v", err)
	}
	if err := db.Migrator().DropTable(&model.MenuItem{}); err != nil {
		t.Fatalf("drop menu_items: %v", err)
	}
	if _, err := migrate.EnsureUpToDate(ctx, db, migrate.ScopeSingle, func(db *gorm.DB) error {
		return nil // baseline 已完成，禁止再跑 AutoMigrate
	}); err != nil {
		t.Fatalf("EnsureUpToDate: %v", err)
	}
	if !db.Migrator().HasTable(&model.MenuItem{}) {
		t.Fatal("menu_items table not created by 0027")
	}
	if !db.Migrator().HasColumn(&model.PageSpec{}, "MenuID") {
		t.Fatal("page_specs.menu_id not backfilled by 0027")
	}
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM page_specs WHERE page_key = 'resource--player'").Scan(&count).Error; err != nil {
		t.Fatalf("read legacy row: %v", err)
	}
	if count != 1 {
		t.Fatalf("legacy page_specs row lost: %d", count)
	}
	// 建出的 menu_items 可正常写入（含 uniqueIndex，scope 同键二次写入应被拒）
	if err := db.Exec(`INSERT INTO menu_items (created_at, updated_at, game_id, env, menu_key, labels, is_visible)
		VALUES (datetime('now'), datetime('now'), 'demo_game', 'dev', 'ops', '{}', 1)`).Error; err != nil {
		t.Fatalf("insert menu_items: %v", err)
	}
	if err := db.Exec(`INSERT INTO menu_items (created_at, updated_at, game_id, env, menu_key, labels, is_visible)
		VALUES (datetime('now'), datetime('now'), 'demo_game', 'dev', 'ops', '{}', 1)`).Error; err == nil {
		t.Fatal("duplicate scope key should violate unique index")
	}
	// 补列后页面挂菜单（全字段 UPDATE 带 menu_id 列）不再报缺列
	var menuID uint
	if err := db.Raw("SELECT id FROM menu_items LIMIT 1").Scan(&menuID).Error; err != nil {
		t.Fatalf("read menu id: %v", err)
	}
	if err := db.Exec("UPDATE page_specs SET menu_id = ? WHERE page_key = 'resource--player'", menuID).Error; err != nil {
		t.Fatalf("update menu_id: %v", err)
	}
}

// TestMigrateMenuItemTablesIdempotent：表列均已存在时 0027 跳过（幂等）；
// 空库不因缺 page_specs 表报错（menu_items 按 0019 建表先例照常创建）。
func TestMigrateMenuItemTablesIdempotent(t *testing.T) {
	sqlDB := openRawSQLiteDBG(t)
	if err := migrateMenuItemTables(context.Background(), sqlDB); err != nil {
		t.Fatalf("empty db should not error, got %v", err)
	}

	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "m27.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.PageSpec{}, &model.MenuItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqlDB2, err := db.DB()
	if err != nil {
		t.Fatalf("sql.DB: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := migrateMenuItemTables(context.Background(), sqlDB2); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
}
