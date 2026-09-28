package main

// 覆盖率巡检补测（dev-seed 包 82%，main 0%、runMain 35%、seeder 错误翼全空）：
//
//  1. main()/runMain 全分支——exit 注入点 + os.Args 覆盖 main 体；死端口
//     postgres dsn → 打库失败（1）；垃圾文件 dsn → 建表失败（1）；games 表
//     预置 NOT NULL 无默认列 → AutoMigrate 容忍但 seedGames INSERT 违约
//     → seedAll 失败（1）；空 dsn + t.Chdir 临时目录 → 成功（0）。
//  2. seeder 错误翼双注入（同 internal/svc C 批口径）：
//     读翼（哨兵 Count / Pluck）→ DropTable 缺表即错；
//     写翼（FirstOrCreate / Create / CreateInBatches）→ 全量预铺后
//     Unscoped 硬删第 k 组行 + PRAGMA query_only 拒写——前组读命中，
//     第 k 组读未命中、写入被拒。k 遍历各组即覆盖该 seeder 全部写翼。
//  3. seedHeavy 万级全量只铺一次底，后续错误翼在其上做缺表/拒写变体。

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/db"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/require"
)

// seededDB 全量预铺的库（seedAll 轻量模式），作为错误注入的基底。
func seededDB(t *testing.T) *gorm.DB {
	t.Helper()
	gdb := openTestDB(t)
	require.NoError(t, seedAll(gdb, false))
	return gdb
}

// delAll 硬删整表行（Unscoped 绕过软删，避免软删行污染读命中判定）。
func delAll(m interface{}) func(*gorm.DB) error {
	return func(gdb *gorm.DB) error {
		return gdb.Unscoped().Where("1 = 1").Delete(m).Error
	}
}

// writeRefusingDB 连接级只读开关：PRAGMA 作用于单连接，锁死连接池大小 1
// 保证后续操作复用同一被拒写连接（internal/svc coverage_c 同款注入）。
func writeRefusingDB(t *testing.T, gdb *gorm.DB) {
	t.Helper()
	sqlDB, err := gdb.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, gdb.Exec("PRAGMA query_only = ON").Error)
}

// ---- main / runMain ----

// main 体此前 0%：覆盖 exit 注入点 → runMain（-h → 0）→ 注入点收到退出码。
func TestMain_OverridesExitAndCallsRunMain(t *testing.T) {
	called := -1
	origExit, origArgs := exit, os.Args
	t.Cleanup(func() { exit, os.Args = origExit, origArgs })
	exit = func(code int) { called = code }
	os.Args = []string{"dev-seed", "-h"}
	main()
	require.Equal(t, 0, called, "main 应把 runMain 返回码交给 exit")
}

func TestRunMain_HelpFlagReturnsZero(t *testing.T) {
	require.Equal(t, 0, runMain("dev-seed", []string{"-h"}, io.Discard))
}

// 死端口 postgres：gorm.Open 初始化期 ping 失败 → 「打开数据库失败」→ 1。
func TestRunMain_OpenDatabaseFailed(t *testing.T) {
	var out bytes.Buffer
	code := runMain("dev-seed", []string{
		"-dsn", "postgres://croupier:croupier@127.0.0.1:1/croupier?sslmode=disable",
	}, &out)
	require.Equal(t, 1, code)
	require.Contains(t, out.String(), "打开数据库失败")
}

// 只读库文件：合法空库 chmod 0400 → Open 以只读回落成功，AutoMigrate 首个
// CREATE TABLE 即「attempt to write a readonly database」→ 「建表失败」→ 1。
// （垃圾文件打不进本分支：glebarez 在 Open 期即校验文件头，落到打开失败。）
func TestRunMain_AutoMigrateFailed(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "readonly.db")
	gdb, err := db.Open(dsn)
	require.NoError(t, err)
	sqlDB, err := gdb.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	require.NoError(t, os.Chmod(strings.TrimPrefix(dsn, "file:"), 0o400))

	var out bytes.Buffer
	code := runMain("dev-seed", []string{"-dsn", dsn}, &out)
	require.Equal(t, 1, code)
	require.Contains(t, out.String(), "建表失败")
}

// games 表预置一个模型不认识的 NOT NULL 无默认列：AutoMigrate 只加列不删列
// （建表阶段通过），seedGames 的 INSERT 不带该列 → 违约 → seedAll 包装 → 1。
func TestRunMain_SeedAllFailed(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "partial-schema.db")
	gdb, err := db.Open(dsn)
	require.NoError(t, err)
	require.NoError(t, gdb.Exec(
		"CREATE TABLE games (id INTEGER PRIMARY KEY AUTOINCREMENT, extra_required TEXT NOT NULL)",
	).Error)
	sqlDB, err := gdb.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	var out bytes.Buffer
	code := runMain("dev-seed", []string{"-dsn", dsn}, &out)
	require.Equal(t, 1, code)
	require.Contains(t, out.String(), "games:", "seedAll 包装错误应带 step 名")
}

// 成功路径：空 dsn 落默认 data/croupier.db（t.Chdir 隔离 CWD）→ 0 + 完成提示。
func TestRunMain_SuccessOnFreshDefaultDSN(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	code := runMain("dev-seed", nil, &out)
	require.Equal(t, 0, code)
	require.Contains(t, out.String(), "完成")
}

// ---- seedAll 装配翼 ----

// 拒写库上 seedAll(heavy=true)：steps append 块 + 首个 seeder 失败的包装分支。
func TestSeedAll_AppendsHeavyAndWrapsSeederFailure(t *testing.T) {
	gdb := seededDB(t)
	writeRefusingDB(t, gdb)
	err := seedAll(gdb, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "games:", "包装错误应带首个失败 step 名")
}

// ---- seeder 写翼：预铺 → 硬删第 k 组 → 拒写重跑 ----

func TestSeeders_WriteGroupRefusedByQueryOnly(t *testing.T) {
	cases := []struct {
		name   string
		seeder func(*gorm.DB) error
		wipe   func(*gorm.DB) error
	}{
		{"seedGames/games", seedGames, delAll(&model.Game{})},
		{"seedGames/env_bindings", seedGames, delAll(&model.GameEnvBinding{})},
		{"seedAccounts/roles", seedAccounts, delAll(&model.Role{})},
		{"seedAccounts/permissions", seedAccounts, delAll(&model.Permission{})},
		{"seedAccounts/role_permissions", seedAccounts, delAll(&model.RolePermission{})},
		{"seedAccounts/admins", seedAccounts, delAll(&model.Admin{})},
		{"seedAccounts/admin_roles", seedAccounts, delAll(&model.AdminRole{})},
		{"seedAnnouncements/announcements", seedAnnouncements, delAll(&model.Announcement{})},
		{"seedAnnouncements/reads", seedAnnouncements, delAll(&model.AnnouncementRead{})},
		{"seedMessages/messages", seedMessages, delAll(&model.Message{})},
		{"seedFAQ/categories", seedFAQ, delAll(&model.FAQCategory{})},
		{"seedFAQ/faqs", seedFAQ, delAll(&model.FAQ{})},
		{"seedFunctions/functions", seedFunctions, delAll(&model.Function{})},
		{"seedPages/page_specs", seedPages, delAll(&model.PageSpec{})},
		{"seedPlayers/players", seedPlayers, delAll(&model.Player{})},
		{"seedTickets/tickets", seedTickets, delAll(&model.Ticket{})},
		{"seedFeedback/feedbacks", seedFeedback, delAll(&model.Feedback{})},
		{"seedBugs/bugs", seedBugs, delAll(&model.Bug{})},
		{"seedSchedules/task_schedules", seedSchedules, delAll(&model.TaskSchedule{})},
		{"seedConfigVersions/versions", seedConfigVersions, func(gdb *gorm.DB) error {
			return gdb.Unscoped().Where("key LIKE ?", "seed.%").Delete(&model.ConfigVersion{}).Error
		}},
		{"seedEdges/edge_p0_game", seedEdges, func(gdb *gorm.DB) error {
			return gdb.Unscoped().Where("game_id = ?", "edge-p0").Delete(&model.Game{}).Error
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gdb := seededDB(t)
			require.NoError(t, tc.wipe(gdb), "擦除第 k 组基底行")
			writeRefusingDB(t, gdb)
			require.Error(t, tc.seeder(gdb), "第 k 组读未命中后写入应被 query_only 拒绝")
		})
	}
}

// seedEdges 写翼特例：空库上计数 0 < 目标 → 首个 edge 组即 CreateInBatches 被拒。
func TestSeedEdges_CreateInBatchesRefusedOnEmpty(t *testing.T) {
	gdb := openTestDB(t)
	writeRefusingDB(t, gdb)
	require.Error(t, seedEdges(gdb))
}

// 评论写翼特例：评论写在工单哨兵早退之后，拒写注入到不了——改缺表注入：
// 硬删工单行（哨兵未命中 → 工单 Create 正常）+ drop 评论表 → 评论 Create 撞缺表。
func TestSeedTickets_CommentCreateFailsOnMissingTable(t *testing.T) {
	gdb := seededDB(t)
	require.NoError(t, delAll(&model.Ticket{})(gdb))
	require.NoError(t, gdb.Migrator().DropTable(&model.TicketComment{}))
	require.Error(t, seedTickets(gdb))
}

// admins/adminRoles 写翼特例：role_permissions 的 OnConflict Create 无存在性
// 检查（重放为 INSERT…DO NOTHING），拒写注入下必先在 links 翼报错、两翼被
// 拦截到不了——改缺表注入（links 重放 no-op 放行）。
func TestSeedAccounts_AdminGroupsFailOnMissingTables(t *testing.T) {
	t.Run("admins_first_or_create_missing_table", func(t *testing.T) {
		gdb := seededDB(t)
		require.NoError(t, gdb.Migrator().DropTable(&model.Admin{}))
		require.Error(t, seedAccounts(gdb), "links no-op 后 admins FirstOrCreate 撞缺表")
	})
	t.Run("admin_roles_create_missing_table", func(t *testing.T) {
		gdb := seededDB(t)
		require.NoError(t, gdb.Migrator().DropTable(&model.AdminRole{}))
		require.Error(t, seedAccounts(gdb), "links/admins 放行后 adminRoles Create 撞缺表")
	})
}

// ---- 读翼：缺表即错 ----

func TestSeeder_ReadSentinels_FailOnMissingTables(t *testing.T) {
	cases := []struct {
		name   string
		seeder func(*gorm.DB) error
		drop   interface{}
	}{
		{"seedTickets/sentinel_count", seedTickets, &model.Ticket{}},
		{"seedFeedback/sentinel_count", seedFeedback, &model.Feedback{}},
		{"seedBugs/sentinel_count", seedBugs, &model.Bug{}},
		{"seedBugs/ticket_pluck", seedBugs, &model.Ticket{}}, // bugs 哨兵空库通过 → Pluck 撞缺表
		{"seedConfigVersions/sentinel_count", seedConfigVersions, &model.ConfigVersion{}},
		{"seedEdges/player_count", seedEdges, &model.Player{}},
		{"seedHeavy/player_count", seedHeavy, &model.Player{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gdb := openTestDB(t)
			require.NoError(t, gdb.Migrator().DropTable(tc.drop))
			require.Error(t, tc.seeder(gdb), "读哨兵/Pluck 撞缺表应报错")
		})
	}
}

// ---- seedHeavy 万级：铺一次底覆盖剩余两翼 ----

// 空库拒写：玩家计数 0 < total → 首个 CreateInBatches 即被拒（免铺万级）。
func TestSeedHeavy_PlayersCreateRefusedOnEmpty(t *testing.T) {
	gdb := openTestDB(t)
	writeRefusingDB(t, gdb)
	require.Error(t, seedHeavy(gdb))
}

// 万级全量铺底后：tickets 缺表 → 玩家哨兵命中跳过、工单计数撞缺表；
// 重建空表后拒写 → 工单计数 0 → 尾部 CreateInBatches 被拒。
func TestSeedHeavy_TicketsCountErrorAndFinalCreateRefused(t *testing.T) {
	gdb := openTestDB(t)
	require.NoError(t, seedHeavy(gdb))

	require.NoError(t, gdb.Migrator().DropTable(&model.Ticket{}))
	require.Error(t, seedHeavy(gdb), "玩家哨兵命中跳过 → 工单计数撞缺表")

	require.NoError(t, gdb.AutoMigrate(&model.Ticket{}))
	writeRefusingDB(t, gdb)
	require.Error(t, seedHeavy(gdb), "工单计数 0 → 尾部 CreateInBatches 应被拒")
}

// ---- 小工具 ----

// 非法 JSON 返回空 map（种子数据全为字面量，非法即 bug）。
func TestModelJSONMap_InvalidJSONReturnsEmpty(t *testing.T) {
	require.Empty(t, modelJSONMap("{bad"))
	m := modelJSONMap(`{"a":1}`)
	require.Equal(t, float64(1), m["a"])
}
