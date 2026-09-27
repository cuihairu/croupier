package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/cuihairu/croupier/internal/model"
)

// openTestDB 建临时 sqlite 库（含建表）。
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "seed-test.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := model.AutoMigrate(gdb); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return gdb
}

// count 轻量计数。
func count(t *testing.T, gdb *gorm.DB, m interface{}, cond string, args ...interface{}) int64 {
	t.Helper()
	var n int64
	if err := gdb.Model(m).Where(cond, args...).Count(&n).Error; err != nil {
		t.Fatalf("count %v: %v", cond, err)
	}
	return n
}

func TestSeedAll_Idempotent(t *testing.T) {
	gdb := openTestDB(t)

	if err := seedAll(gdb, false); err != nil {
		t.Fatalf("seedAll #1: %v", err)
	}
	snapshot := map[string]int64{
		"games":         count(t, gdb, &model.Game{}, "1=1"),
		"admins":        count(t, gdb, &model.Admin{}, "username LIKE 'seed-%'"),
		"tickets":       count(t, gdb, &model.Ticket{}, "title LIKE ?", "【%】%"),
		"bugs":          count(t, gdb, &model.Bug{}, "title LIKE ?", "【seed】%"),
		"announcements": count(t, gdb, &model.Announcement{}, "title LIKE ?", "【seed】%"),
		"players_edge":  count(t, gdb, &model.Player{}, "game_id LIKE 'edge-p%'"),
	}

	// 第二次全量执行：不新增行（幂等）
	if err := seedAll(gdb, false); err != nil {
		t.Fatalf("seedAll #2: %v", err)
	}
	for name, want := range snapshot {
		var got int64
		switch name {
		case "games":
			got = count(t, gdb, &model.Game{}, "1=1")
		case "admins":
			got = count(t, gdb, &model.Admin{}, "username LIKE 'seed-%'")
		case "tickets":
			got = count(t, gdb, &model.Ticket{}, "title LIKE ?", "【%】%")
		case "bugs":
			got = count(t, gdb, &model.Bug{}, "title LIKE ?", "【seed】%")
		case "announcements":
			got = count(t, gdb, &model.Announcement{}, "title LIKE ?", "【seed】%")
		case "players_edge":
			got = count(t, gdb, &model.Player{}, "game_id LIKE 'edge-p%'")
		}
		if got != want {
			t.Errorf("%s: 重跑后计数 %d != 首跑 %d（非幂等）", name, got, want)
		}
	}
}

func TestSeedAll_Coverage(t *testing.T) {
	gdb := openTestDB(t)
	if err := seedAll(gdb, false); err != nil {
		t.Fatalf("seedAll: %v", err)
	}

	// 多游戏多环境
	if got := count(t, gdb, &model.Game{}, "1=1"); got < 4 {
		t.Errorf("games = %d, want >= 4（demo/rpg/slg/edge-p0）", got)
	}
	if got := count(t, gdb, &model.GameEnvBinding{}, "1=1"); got < 6 {
		t.Errorf("game envs = %d, want >= 6", got)
	}
	// 角色用户 + 多角色交叉
	if got := count(t, gdb, &model.AdminRole{}, "admin_id IN (SELECT id FROM admins WHERE username='seed-admin')"); got < 2 {
		t.Errorf("seed-admin 角色交叉 = %d, want >= 2", got)
	}
	// bcrypt 密码可校验（能真登录）
	var admin model.Admin
	if err := gdb.Where(model.Admin{Username: "seed-admin"}).First(&admin).Error; err != nil {
		t.Fatalf("seed-admin not found: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte("seed-Admin123")); err != nil {
		t.Errorf("bcrypt 校验失败: %v", err)
	}
	// 工单边界行
	if got := count(t, gdb, &model.Ticket{}, "title LIKE ?", "%<script>%"); got == 0 {
		t.Error("缺少 XSS 注入尝试样例")
	}
	if got := count(t, gdb, &model.Ticket{}, "player_id = ?", "ghost-player-404"); got == 0 {
		t.Error("缺少悬空玩家引用样例")
	}
	// bugs↔工单关联（含悬空）
	if got := count(t, gdb, &model.Bug{}, "source_ticket_id = 424242"); got == 0 {
		t.Error("缺少悬空 SourceTicketID 样例")
	}
	// 调度任务三态
	if got := count(t, gdb, &model.TaskSchedule{}, "status IN ('active','paused','dead_letter')"); got < 3 {
		t.Errorf("schedules = %d, want >= 3", got)
	}
	// 配置版本多版本历史
	if got := count(t, gdb, &model.ConfigVersion{}, "key = ? AND version >= 3", "seed.mail_template"); got == 0 {
		t.Error("缺少多版本配置历史")
	}
}

func TestSeedHeavy_BulkVolume(t *testing.T) {
	if testing.Short() {
		t.Skip("short 模式跳过万级数据")
	}
	gdb := openTestDB(t)
	if err := seedHeavy(gdb); err != nil {
		t.Fatalf("seedHeavy: %v", err)
	}
	if got := count(t, gdb, &model.Player{}, "game_id = ?", "heavy-load"); got != 10000 {
		t.Errorf("heavy players = %d, want 10000", got)
	}
	if got := count(t, gdb, &model.Ticket{}, "game_id = ?", "heavy-load"); got != 10000 {
		t.Errorf("heavy tickets = %d, want 10000", got)
	}
	// 幂等：重跑不翻倍
	if err := seedHeavy(gdb); err != nil {
		t.Fatalf("seedHeavy #2: %v", err)
	}
	if got := count(t, gdb, &model.Player{}, "game_id = ?", "heavy-load"); got != 10000 {
		t.Errorf("heavy players 重跑 = %d, want 10000", got)
	}
}

func TestRunMain_FlagErrors(t *testing.T) {
	var out bytes.Buffer
	if code := runMain("dev-seed", []string{"-nope"}, &out); code != 2 {
		t.Errorf("bad flag exit = %d, want 2", code)
	}
	if !strings.Contains(out.String(), "flag") && !strings.Contains(out.String(), "nope") {
		t.Logf("诊断输出: %s", out.String())
	}
}
