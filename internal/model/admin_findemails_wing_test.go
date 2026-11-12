// 覆盖率巡检第十九轮（wt-api）：admin_model.go FindEmailsByDomain
// 整函数 0% 收口（#51c 注册别名归一查重新增）——LIKE 域后缀聚合主链，
// 并锁定 Unscoped 语义：软删行邮箱仍参与查重（防删号后原邮箱被重复
// 注册的别名占用判定漏报）。
package model

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAdminModel_FindEmailsByDomain(t *testing.T) {
	// 用例断言某域的邮箱全集；共享 cache=shared 库会被其他用例留下的
	// @example.com 行污染（setupTestDB 不清表）。用每用例隔离内存库，保证
	// 断言只看到本用例数据（-shuffle=on 下 CI 实证 elements differ，
	// 2026-11-12）。
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf(
		"file:findemails_%s_%d.db?mode=memory&cache=shared", t.Name(), time.Now().UnixNano())),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Admin{}))

	m := NewAdminModel(db)
	ctx := context.Background()

	active := &Admin{Username: "fe-active", Nickname: "A", Status: 1}
	require.NoError(t, m.Create(ctx, active, "passWord123"))
	require.NoError(t, db.Model(&Admin{}).Where("id = ?", active.ID).
		Update("email", "active@example.com").Error)

	softDeleted := &Admin{Username: "fe-deleted", Nickname: "D", Status: 1}
	require.NoError(t, m.Create(ctx, softDeleted, "passWord123"))
	require.NoError(t, db.Model(&Admin{}).Where("id = ?", softDeleted.ID).
		Update("email", "deleted@example.com").Error)
	require.NoError(t, db.Delete(&Admin{}, softDeleted.ID).Error)

	other := &Admin{Username: "fe-other", Nickname: "O", Status: 1}
	require.NoError(t, m.Create(ctx, other, "passWord123"))
	require.NoError(t, db.Model(&Admin{}).Where("id = ?", other.ID).
		Update("email", "someone@other.org").Error)

	emails, err := m.FindEmailsByDomain(ctx, "example.com")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"active@example.com", "deleted@example.com"}, emails,
		"同域邮箱全量返回，软删行不豁免（Unscoped 查重语义）")

	empty, err := m.FindEmailsByDomain(ctx, "nowhere.dev")
	require.NoError(t, err)
	assert.Empty(t, empty, "无匹配域返回空切片")
}
