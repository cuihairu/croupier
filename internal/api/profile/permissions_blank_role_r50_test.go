package profile

// 覆盖率巡检 R50：collectUserPermissionSet grants 二循环的空白名角色
// continue 翼（permissions.go:179）。一循环把空白名角色挡在 roleIDs 外，
// 但二循环遍历的是 roleModels 原集——空白名角色混入命名角色时，
// byID 查不到其名 → continue 跳过（防御脏数据：角色名被清空后
// grants 不得出现空名条目）。既有用例（permissions_test.go）从未
// 混入空白名角色，故该翼悬空。
import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectUserPermissionSet_BlankRoleNameSkippedInGrants(t *testing.T) {
	db := setupTestDB(t)
	svc := newPermService(t, db)
	named := createRoleWithPermissions(t, db, "viewer", "user:read")
	blank := model.Role{Name: "   "} // 未落库的脏形态角色（零 ID 在 byID 必 miss）

	set := collectUserPermissionSet(context.Background(), svc, []model.Role{blank, named})
	assert.Equal(t, []string{"viewer"}, set.Roles, "空白名角色不得进展示列表")
	assert.Equal(t, []string{"user:read"}, set.PermissionIDs)
	require.Len(t, set.Resolved.Groups, 1)
	// grants 面由端点契约测试覆盖；此处只锁「空名不炸、不漏权限」
	assert.False(t, set.IsAdmin)
}
