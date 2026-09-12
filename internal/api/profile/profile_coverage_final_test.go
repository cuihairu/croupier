// 覆盖目标（coverage final）：
//  1. Service.resolveLastLoginAt：admin 无 last_login_at 且 audit_records 无
//     成功登录记录 → MAX(timestamp) 为 NULL，回退空串（service.go:98）。
//  2. Service.GetUserGames：games 表出现 TrimSpace 后同 ID 的两条记录
//     （唯一索引按原文判重，业务键按 TrimSpace 判重）→ 第二条被去重跳过
//     （service.go:181）。
//  3. Service.ChangePassword：旧密码校验通过后 FindByUsername 失败
//     （service.go:276）——用计数 query 回调只在第 2 次 admins 查询注错，
//     复现「ValidatePassword 内部查询成功、外层查询失败」的窗口。
package profile

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestGetProfile_LastLoginAtEmptyWithoutAuditRecords(t *testing.T) {
	db := setupTestDB(t)
	createTestAdminWithRole(t, db, "nologin-user", "pw123456", "admin")
	// 空结果集时 MAX(timestamp) 为 NULL，Scan 报错走 err 分支；这里补一条
	// timestamp 为空串的原始行，让 MAX 返回 '' 且扫描成功，覆盖 last==""
	// 分支（sqlite 动态类型允许 datetime 列存 ''）。
	require.NoError(t, db.Exec(`INSERT INTO audit_records
		(event_type, category, severity, outcome, actor_id, chain_hash, chain_sequence, created_at, timestamp)
		VALUES ('auth.login', 'auth', 'info', 'success', 'nologin-user', 'hash-x', 1, CURRENT_TIMESTAMP, '')`).Error)

	svc := NewService(model.NewAdminModel(db), model.NewGameModel(db), model.NewRoleModel(db)).WithDB(db)

	resp, err := svc.GetProfile(context.Background(), "nologin-user")
	require.NoError(t, err)
	// 审计表无可用时间戳 → 空串兜底。
	assert.Equal(t, "", resp.LastLoginAt)
}

func TestGetUserGames_DuplicateGameIDDeduped(t *testing.T) {
	db := setupTestDB(t)
	createTestAdminWithRole(t, db, "dupgame-admin", "pw123456", "admin")

	// GameID 唯一索引按原文判重：前导空格绕过索引，业务键 TrimSpace 后相同。
	// AliasName 同为唯一索引，需各自显式赋不同值避免空串冲突。
	g1 := &model.Game{GameID: "dupgame", Name: "Dup A", AliasName: "dup-a"}
	g2 := &model.Game{GameID: " dupgame", Name: "Dup B", AliasName: "dup-b"}
	require.NoError(t, db.Create(g1).Error)
	require.NoError(t, db.Create(g2).Error)
	require.NoError(t, db.Create(&model.GameEnvBinding{
		GameID: "dupgame", Env: "prod", DatabaseName: "db_dupgame",
	}).Error)

	svc := NewService(model.NewAdminModel(db), model.NewGameModel(db), model.NewRoleModel(db))
	resp, err := svc.GetUserGames(context.Background(), "dupgame-admin")
	require.NoError(t, err)

	found := 0
	for _, g := range resp.Games {
		if g.GameId == "dupgame" {
			found++
			assert.Equal(t, []string{"prod"}, g.Envs)
		}
	}
	assert.Equal(t, 1, found, "TrimSpace 后同 GameID 的游戏应去重为一条")
}

// profileCovSchemaCache 供语句表名解析缓存复用。
var profileCovSchemaCache = &sync.Map{}

// profileCovTable 解析当前语句操作的物理表（显式 Table 优先，其次
// Model/Dest 的 schema）。
func profileCovTable(tx *gorm.DB) string {
	if tx.Statement == nil {
		return ""
	}
	if tx.Statement.Table != "" {
		return tx.Statement.Table
	}
	for _, v := range []interface{}{tx.Statement.Model, tx.Statement.Dest} {
		if v == nil {
			continue
		}
		if s, err := schema.Parse(v, profileCovSchemaCache, schema.NamingStrategy{}); err == nil {
			return s.Table
		}
	}
	return ""
}

func TestChangePassword_FindByUsernameFailsAfterValidation(t *testing.T) {
	// 独立内存库 + 回调注入，不影响共享测试库。
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	ctx := context.Background()
	adminModel := model.NewAdminModel(db)
	admin := &model.Admin{Username: "chpw-user", Nickname: "CHPW", Status: 1}
	require.NoError(t, adminModel.Create(ctx, admin, "oldpass123"))

	// admins 查询计数：第 1 次（ValidatePassword 内部）成功，第 2 次
	//（ChangePassword 外层）注入失败。
	var adminQueries int
	require.NoError(t, db.Callback().Query().Before("gorm:query").
		Register("profile_cov:second_admin_query_fails", func(tx *gorm.DB) {
			if profileCovTable(tx) != "admins" {
				return
			}
			adminQueries++
			if adminQueries == 2 {
				_ = tx.AddError(errors.New("injected find failure"))
			}
		}))

	svc := NewService(adminModel, model.NewGameModel(db), model.NewRoleModel(db))
	_, err = svc.ChangePassword(ctx, "chpw-user", &ChangePasswordRequest{
		OldPassword: "oldpass123",
		NewPassword: "newpass456",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "用户不存在")
}
