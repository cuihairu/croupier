package router_test

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/db/router"
	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// arch006Router bundles a Router with a recording of the physical database
// names handed to EnsureDatabase (one per first-open). This lets tests assert
// WHICH physical DB a scope resolved to, instead of merely that some DB opened.
type arch006Router struct {
	r        *router.Router
	openedMu sync.Mutex
	opened   []string // physical dbName handed to EnsureDatabase
	closeAll func()
}

// newArch006Router builds a Router over a meta DB seeded with the given
// bindings. nameForGame, when non-nil, overrides the configured naming function
// (to simulate a custom prefix). EnsureDatabase records each requested name.
func newArch006Router(t *testing.T, metaDB *gorm.DB, bindings []model.GameEnvBinding, nameForGame func(gameID, env string) string) *arch006Router {
	t.Helper()
	tmp := t.TempDir()
	a := &arch006Router{}

	cfg := router.Config{
		Driver:  "sqlite",
		MetaDSN: filepath.Join(tmp, "meta.db"),
		DSNForDatabase: func(metaDSN, dbName string) string {
			return filepath.Join(tmp, dbName+".db")
		},
		EnsureDatabase: func(driver, metaDSN, dbName string) (string, error) {
			a.openedMu.Lock()
			a.opened = append(a.opened, dbName)
			a.openedMu.Unlock()
			return filepath.Join(tmp, dbName+".db"), nil
		},
		Open: func(driver, dsn string) (*gorm.DB, error) {
			return gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
		},
		MigrateGame: func(db *gorm.DB) error { return nil },
	}
	if nameForGame != nil {
		cfg.NameForGame = nameForGame
	}

	a.r = router.New(cfg, metaDB)
	a.closeAll = func() { _ = a.r.Close() }
	return a
}

// seedBindings creates game_envs rows in the meta DB.
func seedBindings(t *testing.T, metaDB *gorm.DB, bindings []model.GameEnvBinding) {
	t.Helper()
	for i := range bindings {
		if err := metaDB.Create(&bindings[i]).Error; err != nil {
			t.Fatalf("seed binding %s/%s: %v", bindings[i].GameID, bindings[i].Env, err)
		}
	}
}

// recorded asserts the set of physical DB names opened matches want (any order).
func (a *arch006Router) recorded(t *testing.T, want ...string) {
	t.Helper()
	a.openedMu.Lock()
	defer a.openedMu.Unlock()
	if len(a.opened) != len(want) {
		t.Fatalf("opened %v, want %v", a.opened, want)
	}
	got := map[string]bool{}
	for _, n := range a.opened {
		got[n] = true
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("physical DB %q not opened (opened=%v)", w, a.opened)
		}
	}
}

// TestArch006_CustomBindingNameRoutesToBinding 验证自定义名：binding 的
// database_name 与默认前缀不同时，必须路由到 binding 名（旧实现会重算成
// game_demo_prod 而打开错误的库）。
func TestArch006_CustomBindingNameRoutesToBinding(t *testing.T) {
	tmp := t.TempDir()
	metaDB := openSQLite(t, filepath.Join(tmp, "meta.db"))
	if err := model.AutoMigrateMeta(metaDB); err != nil {
		t.Fatalf("migrate meta: %v", err)
	}
	defer closeDB(t, metaDB)

	seedBindings(t, metaDB, []model.GameEnvBinding{
		{GameID: "demo", Env: "prod", DatabaseName: "legacy_custom_db"},
	})
	a := newArch006Router(t, metaDB, nil, nil)
	defer a.closeAll()

	if _, err := a.r.GameDB(context.Background(), "demo", "prod"); err != nil {
		t.Fatalf("GameDB: %v", err)
	}
	a.recorded(t, "legacy_custom_db")
}

// TestArch006_LegacyDefaultNameRoutesToBinding 验证旧名：binding 存的是默认
// 前缀名（历史上由 DefaultGameDBName 创建），路由仍用该名（不因真值切换而变）。
func TestArch006_LegacyDefaultNameRoutesToBinding(t *testing.T) {
	tmp := t.TempDir()
	metaDB := openSQLite(t, filepath.Join(tmp, "meta.db"))
	if err := model.AutoMigrateMeta(metaDB); err != nil {
		t.Fatalf("migrate meta: %v", err)
	}
	defer closeDB(t, metaDB)

	seedBindings(t, metaDB, []model.GameEnvBinding{
		{GameID: "demo", Env: "prod", DatabaseName: "game_demo_prod"},
	})
	a := newArch006Router(t, metaDB, nil, nil)
	defer a.closeAll()

	if _, err := a.r.GameDB(context.Background(), "demo", "prod"); err != nil {
		t.Fatalf("GameDB: %v", err)
	}
	a.recorded(t, "game_demo_prod")
}

// TestArch006_PrefixChangeKeepsBindingName 验证前缀变更：配置前缀已改为
// "gp_"，但存量 binding 仍叫 game_demo_prod——必须路由到 binding 名，而非
// 重算的 gp_demo_prod（旧实现会在前缀变更后找不到既有库）。
func TestArch006_PrefixChangeKeepsBindingName(t *testing.T) {
	tmp := t.TempDir()
	metaDB := openSQLite(t, filepath.Join(tmp, "meta.db"))
	if err := model.AutoMigrateMeta(metaDB); err != nil {
		t.Fatalf("migrate meta: %v", err)
	}
	defer closeDB(t, metaDB)

	seedBindings(t, metaDB, []model.GameEnvBinding{
		{GameID: "demo", Env: "prod", DatabaseName: "game_demo_prod"},
	})
	a := newArch006Router(t, metaDB, nil, func(gameID, env string) string {
		return "gp_" + gameID + "_" + env // 模拟前缀变更后的重算结果
	})
	defer a.closeAll()

	if _, err := a.r.GameDB(context.Background(), "demo", "prod"); err != nil {
		t.Fatalf("GameDB: %v", err)
	}
	a.recorded(t, "game_demo_prod")
}

// TestArch006_UnderscoreCollisionDisambiguatedByBinding 验证下划线碰撞：
// ("ab","c_d") 与 ("a_b","cd") 的前缀重算都是 game_ab_c_d（碰撞）；binding
// 给二者不同的自定义名时，必须分别路由到各自库（旧实现两者塌缩到同一库）。
func TestArch006_UnderscoreCollisionDisambiguatedByBinding(t *testing.T) {
	tmp := t.TempDir()
	metaDB := openSQLite(t, filepath.Join(tmp, "meta.db"))
	if err := model.AutoMigrateMeta(metaDB); err != nil {
		t.Fatalf("migrate meta: %v", err)
	}
	defer closeDB(t, metaDB)

	seedBindings(t, metaDB, []model.GameEnvBinding{
		{GameID: "ab", Env: "c_d", DatabaseName: "db_ab_cd_first"},
		{GameID: "a_b", Env: "cd", DatabaseName: "db_ab_cd_second"},
	})
	a := newArch006Router(t, metaDB, nil, nil)
	defer a.closeAll()

	if _, err := a.r.GameDB(context.Background(), "ab", "c_d"); err != nil {
		t.Fatalf("GameDB ab/c_d: %v", err)
	}
	if _, err := a.r.GameDB(context.Background(), "a_b", "cd"); err != nil {
		t.Fatalf("GameDB a_b/cd: %v", err)
	}
	a.recorded(t, "db_ab_cd_first", "db_ab_cd_second")
}

// TestArch006_NoBindingFallsBackToPrefix 验证缺绑定回退：metaDB 无对应
// binding 行时（如 backfill 前），才回退到配置命名函数（前缀重算仅作兜底）。
func TestArch006_NoBindingFallsBackToPrefix(t *testing.T) {
	tmp := t.TempDir()
	metaDB := openSQLite(t, filepath.Join(tmp, "meta.db"))
	if err := model.AutoMigrateMeta(metaDB); err != nil {
		t.Fatalf("migrate meta: %v", err)
	}
	defer closeDB(t, metaDB)

	a := newArch006Router(t, metaDB, nil, func(gameID, env string) string {
		return "gp_" + gameID + "_" + env
	})
	defer a.closeAll()

	if _, err := a.r.GameDB(context.Background(), "demo", "prod"); err != nil {
		t.Fatalf("GameDB: %v", err)
	}
	a.recorded(t, "gp_demo_prod")
}

// TestArch006_SoftDeletedBindingNotRoutable 验证软删 binding 不可路由：
// 软删行不作为真值（与 LookupDatabaseName 的 First 语义一致），回退前缀名。
func TestArch006_SoftDeletedBindingNotRoutable(t *testing.T) {
	tmp := t.TempDir()
	metaDB := openSQLite(t, filepath.Join(tmp, "meta.db"))
	if err := model.AutoMigrateMeta(metaDB); err != nil {
		t.Fatalf("migrate meta: %v", err)
	}
	defer closeDB(t, metaDB)

	binding := model.GameEnvBinding{GameID: "demo", Env: "prod", DatabaseName: "custom_prod"}
	if err := metaDB.Create(&binding).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := metaDB.Delete(&binding).Error; err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	a := newArch006Router(t, metaDB, nil, func(gameID, env string) string {
		return "gp_" + gameID + "_" + env
	})
	defer a.closeAll()

	if _, err := a.r.GameDB(context.Background(), "demo", "prod"); err != nil {
		t.Fatalf("GameDB: %v", err)
	}
	a.recorded(t, "gp_demo_prod")
}

// TestArch006_GameDBByPhysicalName_SkipsBindingLookup 验证 GameDBByPhysicalName
// 直接按已授权名打开（中间件路径）：绑定名与重算名不同也打开绑定名，且空名报错。
func TestArch006_GameDBByPhysicalName_SkipsBindingLookup(t *testing.T) {
	tmp := t.TempDir()
	metaDB := openSQLite(t, filepath.Join(tmp, "meta.db"))
	if err := model.AutoMigrateMeta(metaDB); err != nil {
		t.Fatalf("migrate meta: %v", err)
	}
	defer closeDB(t, metaDB)

	seedBindings(t, metaDB, []model.GameEnvBinding{
		{GameID: "demo", Env: "prod", DatabaseName: "authorized_db"},
	})
	a := newArch006Router(t, metaDB, nil, nil)
	defer a.closeAll()

	if _, err := a.r.GameDBByPhysicalName(context.Background(), "authorized_db", "demo", "prod"); err != nil {
		t.Fatalf("GameDBByPhysicalName: %v", err)
	}
	a.recorded(t, "authorized_db")

	if _, err := a.r.GameDBByPhysicalName(context.Background(), "  ", "demo", "prod"); err == nil {
		t.Fatal("expected error for blank physical name")
	} else if !strings.Contains(err.Error(), "empty physical database name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestArch006_ConcurrentFirstOpenSameBinding 验证并发首次开库：同一 scope 的
// 并发 GameDB 调用（binding 真值路径）仍由 singleflight 去重，只打开一次。
func TestArch006_ConcurrentFirstOpenSameBinding(t *testing.T) {
	tmp := t.TempDir()
	metaDB := openSQLite(t, filepath.Join(tmp, "meta.db"))
	if err := model.AutoMigrateMeta(metaDB); err != nil {
		t.Fatalf("migrate meta: %v", err)
	}
	defer closeDB(t, metaDB)

	seedBindings(t, metaDB, []model.GameEnvBinding{
		{GameID: "demo", Env: "prod", DatabaseName: "concurrent_db"},
	})
	a := newArch006Router(t, metaDB, nil, nil)
	defer a.closeAll()

	const n = 8
	var wg sync.WaitGroup
	results := make([]*gorm.DB, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			db, err := a.r.GameDB(context.Background(), "demo", "prod")
			if err != nil {
				t.Errorf("GameDB: %v", err)
				return
			}
			results[i] = db
		}(i)
	}
	wg.Wait()

	for i := 1; i < n; i++ {
		if results[i] != results[0] {
			t.Errorf("concurrent GameDB returned different connections")
		}
	}
	a.recorded(t, "concurrent_db")
}
