// 覆盖率巡检第十九轮（wt-api）：admin_model.go FindEmailsByDomain
// 整函数 0% 收口（#51c 注册别名归一查重新增）——LIKE 域后缀聚合主链，
// 并锁定 Unscoped 语义：软删行邮箱仍参与查重（防删号后原邮箱被重复
// 注册的别名占用判定漏报）。
package model

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminModel_FindEmailsByDomain(t *testing.T) {
	db := setupTestDB(t)
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
