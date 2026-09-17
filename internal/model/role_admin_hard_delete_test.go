package model

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 软删除 + 物理唯一索引回归（0026 同族）：roles.name 与 admins.username 的
// 唯一索引是物理索引，删除路径改硬删（Unscoped）后，同名重建必须成功且不
// 留残留行。软删行占着索引位会让重建 INSERT 直接 duplicate-key 500
//（T-M9 菜单 E2E 实证：删除角色后重建报 UNIQUE constraint failed: roles.name）。

var roleAdminHardDeleteSeq int

func setupRoleAdminHardDeleteDB(t *testing.T) *gorm.DB {
	t.Helper()
	roleAdminHardDeleteSeq++
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:roledel%d?mode=memory&cache=shared", roleAdminHardDeleteSeq)),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Role{}, &Admin{}, &AdminRole{}, &RolePermission{}))
	return db
}

// TestRoleDeleteThenRecreateSameName 角色删除后同名重建必须不撞唯一索引。
func TestRoleDeleteThenRecreateSameName(t *testing.T) {
	db := setupRoleAdminHardDeleteDB(t)
	m := NewRoleModel(db)
	ctx := context.Background()

	first := &Role{Name: "e2e-viewer", Description: "first"}
	require.NoError(t, m.Create(ctx, first))
	require.NoError(t, m.Delete(ctx, first.ID))

	reborn := &Role{Name: "e2e-viewer", Description: "reborn"}
	require.NoError(t, m.Create(ctx, reborn), "recreating the same role name after delete must not hit the unique index")

	var rawCount int64
	require.NoError(t, db.Unscoped().Model(&Role{}).Where("name = ?", "e2e-viewer").Count(&rawCount).Error)
	assert.Equal(t, int64(1), rawCount, "soft-deleted residue must not linger")
}

// TestAdminDeleteThenRecreateSameUsername 用户删除后同名重建必须不撞唯一索引。
func TestAdminDeleteThenRecreateSameUsername(t *testing.T) {
	db := setupRoleAdminHardDeleteDB(t)
	m := NewAdminModel(db)
	ctx := context.Background()

	first := &Admin{Username: "e2e_menu_viewer", Nickname: "first", Status: 1}
	require.NoError(t, m.Create(ctx, first, "Croupier#E2E1"))
	require.NoError(t, m.Delete(ctx, first.ID))

	reborn := &Admin{Username: "e2e_menu_viewer", Nickname: "reborn", Status: 1}
	require.NoError(t, m.Create(ctx, reborn, "Croupier#E2E1"), "recreating the same username after delete must not hit the unique index")

	var rawCount int64
	require.NoError(t, db.Unscoped().Model(&Admin{}).Where("username = ?", "e2e_menu_viewer").Count(&rawCount).Error)
	assert.Equal(t, int64(1), rawCount, "soft-deleted residue must not linger")
}
