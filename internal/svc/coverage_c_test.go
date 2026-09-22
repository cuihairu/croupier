package svc

// C 批覆盖补齐（db.go / menu_seeder.go / approval_seed.go / service_context.go）：
// 各函数剩余缺口集中在「错误注入/守卫分支」，全部通过既有包级接缝
// （openMemorySQLite / setSQLiteQueryOnly / sqlOpen / validateStoreConfig）或
// 纯函数边界构造，均为确定性注入，不依赖时序。
// 注意：接缝替换仅出现在非并行测试中（Go 顶层顺序测试先于并行测试执行，
// coverage_group_g_test.go 已有同模式先例），defer 恢复，不会与
// t.Parallel 用例重叠。

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	objstore "github.com/cuihairu/croupier/internal/platform/objstore"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// --- 测试工具 ---

// openCoverageCFileDB 打开一个临时文件 sqlite（gorm 预迁表格用）。
func openCoverageCFileDB(t *testing.T, name string, models ...interface{}) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), name)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if len(models) > 0 {
		require.NoError(t, db.AutoMigrate(models...))
	}
	return db
}

// --- db.go：三个接缝注入分支 ---

// openGorm 的内存库打开失败透传：openMemorySQLite 是包级接缝，注入失败
// 即可驱动「DSN 恒合法、生产无确定性失败输入」的签名契约分支。
func TestCoverageC_OpenGorm_MemoryOpenError(t *testing.T) {
	orig := openMemorySQLite
	openMemorySQLite = func(uint64) (*gorm.DB, error) { return nil, errors.New("coverage: memory open refused") }
	defer func() { openMemorySQLite = orig }()

	db, err := openGorm("sqlite", ":memory:")
	require.Error(t, err)
	require.Nil(t, db)
	require.Contains(t, err.Error(), "coverage: memory open refused")
}

// openReadOnlyGorm 的 query_only 设置失败透传：同上，接缝注入。
// 前置：dsn 文件必须真实存在（ensureSQLiteFileExists 守卫）。
func TestCoverageC_OpenReadOnlyGorm_QueryOnlyError(t *testing.T) {
	// 先正常建一次库并关闭，保证 dsn 指向的文件存在
	existing := filepath.Join(t.TempDir(), "seed.db")
	seed, err := gorm.Open(gsqlite.Open(existing), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlSeed, err := seed.DB()
	require.NoError(t, err)
	require.NoError(t, sqlSeed.Close())

	orig := setSQLiteQueryOnly
	setSQLiteQueryOnly = func(*gorm.DB) error { return errors.New("coverage: query_only refused") }
	defer func() { setSQLiteQueryOnly = orig }()

	ro, err := openReadOnlyGorm("sqlite", existing)
	require.Error(t, err)
	require.Nil(t, ro)
	require.Contains(t, err.Error(), "coverage: query_only refused")
}

// createPostgresDatabase 的 sql.Open 失败透传：sqlOpen 是包级接缝
// （注释声明该分支由接缝测试驱动）。
func TestCoverageC_CreatePostgresDatabase_SqlOpenError(t *testing.T) {
	orig := sqlOpen
	sqlOpen = func(string, string) (*sql.DB, error) { return nil, errors.New("coverage: open refused") }
	defer func() { sqlOpen = orig }()

	err := createPostgresDatabase("postgres://u:p@127.0.0.1:1/postgres", "cov_c_missing_db")
	require.Error(t, err)
	require.Contains(t, err.Error(), "coverage: open refused")
}

// --- menu_seeder.go ---

// menuSeedC 构造合法种子（menuKey 满足校验、labels 非空）。
func menuSeedC(key string) SeedMenuItem {
	return SeedMenuItem{MenuKey: key, Labels: map[string]string{"zh-CN": "菜单-" + key}}
}

// 非空 scope 且无缺失（missing==0）→ 直接返回，不再补种（155-157）。
func TestCoverageC_MenuSeeder_NonEmptyScopeNoMissing(t *testing.T) {
	db := openCoverageCFileDB(t, "menu_a.db", &model.MenuItem{}, &model.PlatformSetting{})
	menuModel := model.NewMenuItemModel(db)
	settingsModel := model.NewPlatformSettingModel(db)
	seeds := []SeedMenuItem{menuSeedC("ops"), menuSeedC("player")}

	// 第一轮：空 scope 全量种入
	NewMenuSeeder(menuModel, settingsModel, seeds).EnsureSeeded(context.Background(), "demo", "dev")
	count := int64(0)
	require.NoError(t, db.Model(&model.MenuItem{}).Where("game_id = ? AND env = ?", "demo", "dev").Count(&count).Error)
	require.EqualValues(t, 2, count)

	// 第二轮（新实例绕过进程内 tried 标记）：非空 scope、missing==0 → return
	NewMenuSeeder(menuModel, settingsModel, seeds).EnsureSeeded(context.Background(), "demo", "dev")
	require.NoError(t, db.Model(&model.MenuItem{}).Where("game_id = ? AND env = ?", "demo", "dev").Count(&count).Error)
	require.EqualValues(t, 2, count, "无缺失时不应重复种入")
}

// 非空 scope 有缺失且 settings==nil → 保守返回，不补缺（158-160）。
func TestCoverageC_MenuSeeder_NonEmptyScopeNilSettings(t *testing.T) {
	db := openCoverageCFileDB(t, "menu_b.db", &model.MenuItem{})
	menuModel := model.NewMenuItemModel(db)
	// 手工种一条，模拟 T-M10 之前已有数据的旧 scope
	require.NoError(t, menuModel.Create(context.Background(), &model.MenuItem{
		GameID: "demo", Env: "dev", MenuKey: "player", IsVisible: true,
	}))
	seeds := []SeedMenuItem{menuSeedC("ops"), menuSeedC("player")}

	NewMenuSeeder(menuModel, nil, seeds).EnsureSeeded(context.Background(), "demo", "dev")

	count := int64(0)
	require.NoError(t, db.Model(&model.MenuItem{}).Where("game_id = ? AND env = ?", "demo", "dev").Count(&count).Error)
	require.EqualValues(t, 1, count, "settings 未接线时不改已有数据的 scope")
}

// 非空 scope 有缺失、settings 读标记失败（platform_settings 表缺失）→
// 放弃本轮补种（162-165）。
func TestCoverageC_MenuSeeder_BackfillMarkerReadError(t *testing.T) {
	db := openCoverageCFileDB(t, "menu_c.db", &model.MenuItem{})
	menuModel := model.NewMenuItemModel(db)
	settingsModel := model.NewPlatformSettingModel(db) // 表未迁移 → Get 报错
	require.NoError(t, menuModel.Create(context.Background(), &model.MenuItem{
		GameID: "demo", Env: "dev", MenuKey: "player", IsVisible: true,
	}))
	seeds := []SeedMenuItem{menuSeedC("ops"), menuSeedC("player")}

	NewMenuSeeder(menuModel, settingsModel, seeds).EnsureSeeded(context.Background(), "demo", "dev")

	count := int64(0)
	require.NoError(t, db.Model(&model.MenuItem{}).Where("game_id = ? AND env = ?", "demo", "dev").Count(&count).Error)
	require.EqualValues(t, 1, count, "标记读取失败应放弃补种")
}

// 空 scope 且 settings==nil：全量种入后 markSeedBackfilled 直接返回（214-216）。
func TestCoverageC_MenuSeeder_EmptyScopeNilSettingsMarksBackfilled(t *testing.T) {
	db := openCoverageCFileDB(t, "menu_d.db", &model.MenuItem{})
	menuModel := model.NewMenuItemModel(db)
	seeds := []SeedMenuItem{menuSeedC("ops"), menuSeedC("player")}

	NewMenuSeeder(menuModel, nil, seeds).EnsureSeeded(context.Background(), "demo", "dev")

	count := int64(0)
	require.NoError(t, db.Model(&model.MenuItem{}).Where("game_id = ? AND env = ?", "demo", "dev").Count(&count).Error)
	require.EqualValues(t, 2, count)
}

// 空 scope、标记写入失败（platform_settings 表缺失）→ 菜单照常种入，
// 仅 Warn（217-219）。
func TestCoverageC_MenuSeeder_MarkBackfillWriteError(t *testing.T) {
	db := openCoverageCFileDB(t, "menu_e.db", &model.MenuItem{})
	menuModel := model.NewMenuItemModel(db)
	settingsModel := model.NewPlatformSettingModel(db) // 表未迁移 → Set 报错
	seeds := []SeedMenuItem{menuSeedC("ops"), menuSeedC("player")}

	NewMenuSeeder(menuModel, settingsModel, seeds).EnsureSeeded(context.Background(), "demo", "dev")

	count := int64(0)
	require.NoError(t, db.Model(&model.MenuItem{}).Where("game_id = ? AND env = ?", "demo", "dev").Count(&count).Error)
	require.EqualValues(t, 2, count, "标记写入失败不应影响菜单种入")
}

// --- approval_seed.go ---

// failingApprovalStoreC 只覆写 Create，其余方法不会被 seed 路径触达。
type failingApprovalStoreC struct {
	approvals.Store
}

func (f *failingApprovalStoreC) Create(*approvals.Approval) (*approvals.Approval, error) {
	return nil, errors.New("coverage: create refused")
}

// demoApprovalScopes 的三个过滤边界：空白 env 丢弃（148-149）、同 env
// 去重（151-152）、达到单游戏最大环境数即停（156-157）。
func TestCoverageC_DemoApprovalScopes_FilterEdges(t *testing.T) {
	bindings := []model.GameEnvBinding{
		{GameID: "default", Env: "   "}, // 空白 env：丢弃
		{GameID: "default", Env: "dev"},
		{GameID: "default", Env: "dev"}, // 重复 env：去重
		{GameID: "default", Env: "prod"},
		{GameID: "default", Env: "test"},
		{GameID: "default", Env: "staging"},
		{GameID: "default", Env: "qa"},
		{GameID: "default", Env: "uat"}, // 第 6 个 append 后触发 maxEnvs break
		{GameID: "default", Env: "perf"},
	}
	scopes := demoApprovalScopes(bindings)
	require.Len(t, scopes, demoApprovalScopesMaxEnvs)
	seen := map[string]bool{}
	for _, s := range scopes {
		require.Equal(t, "default", s.GameID)
		require.False(t, seen[s.Env], "env %s 应去重", s.Env)
		seen[s.Env] = true
	}
	require.Contains(t, seen, "dev")
	require.NotContains(t, seen, "perf", "达到上限后不再采集")
}

// seedDemoApprovalsInto 守卫：nil store 与空 scopes 均返回 0（172-174）；
// store.Create 失败时跳过该条继续、返回 0（201-204）。
func TestCoverageC_SeedDemoApprovalsInto_GuardsAndCreateError(t *testing.T) {
	now := time.Now()
	scopes := []GameScope{{GameID: "default", Env: "dev"}}
	require.Equal(t, 0, seedDemoApprovalsInto(nil, scopes, now), "nil store 返回 0")
	require.Equal(t, 0, seedDemoApprovalsInto(approvals.NewMemStore(), nil, now), "空 scopes 返回 0")

	created := seedDemoApprovalsInto(&failingApprovalStoreC{}, scopes, now)
	require.Equal(t, 0, created, "Create 全失败应跳过所有条目")
}

// seedDemoApprovals 的 env 绑定读取失败分支：game_envs 表缺失 → Warn 返回
// （104-107）。其余守卫（非 dev 配置/nil store/nil game model）已由
// TestSeedDemoApprovals_GateGuards 覆盖。
func TestCoverageC_SeedDemoApprovals_ListBindingsError(t *testing.T) {
	db := openCoverageCFileDB(t, "approval_c.db", &model.Game{}) // 故意缺 game_envs 表
	cfg := config.Config{Server: config.ServerConfig{Mode: "dev"}}
	ctx := &ServiceContext{
		Config:         cfg,
		ApprovalsStore: approvals.NewMemStore(),
		GameModel:      model.NewGameModel(db),
	}
	require.NotPanics(t, func() { seedDemoApprovals(ctx) })
}

// --- service_context.go ---

// initObjectStore 的未知驱动兜底分支：validateStoreConfig 接缝放行后，
// switch default 显式报错（1244-1251，跨包隐式契约的防御分支）。
func TestCoverageC_InitObjectStore_UnknownDriver(t *testing.T) {
	orig := validateStoreConfig
	validateStoreConfig = func(objstore.Config) error { return nil }
	defer func() { validateStoreConfig = orig }()

	store, err := initObjectStore(context.Background(), config.StorageConfig{Driver: "gcs"})
	require.Error(t, err)
	require.Nil(t, store)
	require.Contains(t, err.Error(), "unsupported storage driver")
}

// NewServiceContext 的菜单种子文件两种非默认形态：解析失败 → Error 禁用
// 但不阻断启动（276-278）；合法文件 → Info 启用（278-280）。
func TestCoverageC_NewServiceContext_MenuSeedFileStates(t *testing.T) {
	// 非法 JSON：种子整体禁用，服务照常构造
	cfg := newSvcConfig(t, false)
	require.NoError(t, os.MkdirAll(cfg.BootstrapData.BaseDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cfg.BootstrapData.BaseDir, DefaultMenusFilename), []byte("{invalid"), 0o644))
	require.NotPanics(t, func() { NewServiceContext(cfg) })

	// 合法文件：种子启用
	cfg2 := newSvcConfig(t, false)
	require.NoError(t, os.MkdirAll(cfg2.BootstrapData.BaseDir, 0o755))
	seeds := []map[string]interface{}{
		{"menuKey": "ops", "labels": map[string]string{"zh-CN": "运营"}},
	}
	blob, err := json.Marshal(seeds)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cfg2.BootstrapData.BaseDir, DefaultMenusFilename), blob, 0o644))
	ctx := NewServiceContext(cfg2)
	require.NotNil(t, ctx)
}
