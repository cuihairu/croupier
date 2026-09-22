// 覆盖目标（B 批）：service.go RoleDelete 事务内删除 admin_roles 悬挂
// 引用失败的错误分支（复用既有 covFailureInjector 错误注入基建）。
package role

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RoleDelete：role_permissions 清理成功后，admin_roles 绑定行删除失败
// → 事务回滚并返回「删除用户角色绑定失败」（角色改硬删后悬挂引用
// 清理失败的传播路径）。
func TestRoleCovB_Delete_AdminRoleRowsError(t *testing.T) {
	s, ctx, inj, db := newRoleCovEnv(t)
	id := seedCovRole(t, db, "covdelete-adminrole")
	inj.failAll["delete:admin_roles"] = true

	err := s.RoleDelete(ctx, &RoleDeleteRequest{ID: id})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "删除用户角色绑定失败")
}
