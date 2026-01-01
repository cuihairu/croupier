package assignments

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB 创建一个内存 SQLite 数据库用于测试
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	return db
}

// TestNewStore 测试创建 Store
func TestNewStore(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)

	if store == nil {
		t.Fatal("NewStore() should return non-nil store")
	}

	if store.db != db {
		t.Error("Store should contain the provided db")
	}
}

// TestAssignment_TableName 测试表名
func TestAssignment_TableName(t *testing.T) {
	a := Assignment{}
	if a.TableName() != "assignments" {
		t.Errorf("TableName() = %q, want 'assignments'", a.TableName())
	}
}

// TestStore_AutoMigrate 测试自动迁移
func TestStore_AutoMigrate(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)

	err := store.AutoMigrate()
	if err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	// 验证表已创建
	if !db.Migrator().HasTable(&Assignment{}) {
		t.Error("AutoMigrate() should create assignments table")
	}
}

// TestStore_Get_NotFound 测试获取不存在的分配
func TestStore_Get_NotFound(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	_ = store.AutoMigrate()

	functions, err := store.Get("nonexistent-game", "dev")
	if err != nil {
		t.Errorf("Get() should not return error for nonexistent game, got %v", err)
	}

	if functions == nil {
		t.Error("Get() should return empty slice, not nil")
	}

	if len(functions) != 0 {
		t.Errorf("Get() should return empty slice, got %d functions", len(functions))
	}
}

// TestStore_SetAndGet 测试设置和获取分配
func TestStore_SetAndGet(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	_ = store.AutoMigrate()

	// 设置分配
	functions := []string{"func1", "func2", "func3"}
	err := store.Set("game1", "dev", functions, "admin")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// 获取分配
	retrieved, err := store.Get("game1", "dev")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if len(retrieved) != 3 {
		t.Errorf("Get() should return 3 functions, got %d", len(retrieved))
	}

	expected := []string{"func1", "func2", "func3"}
	for i, f := range retrieved {
		if f != expected[i] {
			t.Errorf("Get()[%d] = %q, want %q", i, f, expected[i])
		}
	}
}

// TestStore_Set_EmptyFunctions 测试设置空函数列表
func TestStore_Set_EmptyFunctions(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	_ = store.AutoMigrate()

	err := store.Set("game1", "dev", []string{}, "admin")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	retrieved, err := store.Get("game1", "dev")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if len(retrieved) != 0 {
		t.Errorf("Get() should return empty slice, got %d functions", len(retrieved))
	}
}

// TestStore_Set_Update 测试更新分配
func TestStore_Set_Update(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	_ = store.AutoMigrate()

	// 初始设置
	err := store.Set("game1", "dev", []string{"func1"}, "admin")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// 更新
	err = store.Set("game1", "dev", []string{"func1", "func2", "func3"}, "admin")
	if err != nil {
		t.Fatalf("Set() update error = %v", err)
	}

	retrieved, err := store.Get("game1", "dev")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if len(retrieved) != 3 {
		t.Errorf("Get() should return 3 functions after update, got %d", len(retrieved))
	}
}

// TestStore_List 测试列出所有分配
func TestStore_List(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	_ = store.AutoMigrate()

	// 添加多个分配
	_ = store.Set("game1", "dev", []string{"func1"}, "admin")
	_ = store.Set("game1", "prod", []string{"func2"}, "admin")
	_ = store.Set("game2", "dev", []string{"func3"}, "admin")

	// 列出所有
	all, err := store.List("", "")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(all) != 3 {
		t.Errorf("List() should return 3 assignments, got %d", len(all))
	}
}

// TestStore_List_FilterByGame 测试按游戏筛选
func TestStore_List_FilterByGame(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	_ = store.AutoMigrate()

	_ = store.Set("game1", "dev", []string{"func1"}, "admin")
	_ = store.Set("game1", "prod", []string{"func2"}, "admin")
	_ = store.Set("game2", "dev", []string{"func3"}, "admin")

	// 按 game1 筛选
	filtered, err := store.List("game1", "")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(filtered) != 2 {
		t.Errorf("List('game1', '') should return 2 assignments, got %d", len(filtered))
	}

	// 验证键
	if _, ok := filtered["game1|dev"]; !ok {
		t.Error("List() should contain 'game1|dev' key")
	}
	if _, ok := filtered["game1|prod"]; !ok {
		t.Error("List() should contain 'game1|prod' key")
	}
	if _, ok := filtered["game2|dev"]; ok {
		t.Error("List() should not contain 'game2|dev' key when filtered by game1")
	}
}

// TestStore_List_FilterByEnv 测试按环境筛选
func TestStore_List_FilterByEnv(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	_ = store.AutoMigrate()

	_ = store.Set("game1", "dev", []string{"func1"}, "admin")
	_ = store.Set("game2", "dev", []string{"func2"}, "admin")
	_ = store.Set("game3", "prod", []string{"func3"}, "admin")

	// 按 dev 筛选
	filtered, err := store.List("", "dev")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(filtered) != 2 {
		t.Errorf("List('', 'dev') should return 2 assignments, got %d", len(filtered))
	}
}

// TestStore_List_FilterByBoth 测试同时按游戏和环境筛选
func TestStore_List_FilterByBoth(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	_ = store.AutoMigrate()

	_ = store.Set("game1", "dev", []string{"func1"}, "admin")
	_ = store.Set("game1", "prod", []string{"func2"}, "admin")
	_ = store.Set("game2", "dev", []string{"func3"}, "admin")

	// 按 game1 + dev 筛选
	filtered, err := store.List("game1", "dev")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(filtered) != 1 {
		t.Errorf("List('game1', 'dev') should return 1 assignment, got %d", len(filtered))
	}

	if _, ok := filtered["game1|dev"]; !ok {
		t.Error("List() should contain 'game1|dev' key")
	}
}

// TestStore_Delete 测试删除分配
func TestStore_Delete(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	_ = store.AutoMigrate()

	// 添加分配
	_ = store.Set("game1", "dev", []string{"func1"}, "admin")

	// 验证存在
	functions, _ := store.Get("game1", "dev")
	if len(functions) != 1 {
		t.Fatal("Assignment should exist before delete")
	}

	// 删除
	err := store.Delete("game1", "dev")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// 验证已删除
	functions, _ = store.Get("game1", "dev")
	if len(functions) != 0 {
		t.Error("Assignment should not exist after delete")
	}
}

// TestStore_Delete_Nonexistent 测试删除不存在的分配
func TestStore_Delete_Nonexistent(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	_ = store.AutoMigrate()

	// 删除不存在的分配不应该报错
	err := store.Delete("nonexistent", "dev")
	if err != nil {
		t.Errorf("Delete() should not return error for nonexistent assignment, got %v", err)
	}
}

// BenchmarkNewStore 性能基准测试
func BenchmarkNewStore(b *testing.B) {
	db, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewStore(db)
	}
}

// BenchmarkSetAndGet 性能基准测试
func BenchmarkSetAndGet(b *testing.B) {
	db, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	store := NewStore(db)
	_ = store.AutoMigrate()

	functions := []string{"func1", "func2", "func3"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Set("game1", "dev", functions, "admin")
		_, _ = store.Get("game1", "dev")
	}
}
