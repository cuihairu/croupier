// Coverage target (batch B): the error branch inside the RoleDelete transaction
// where deleting dangling admin_roles references fails (reuses the existing
// covFailureInjector failure-injection harness).
package role

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RoleDelete: role_permissions cleanup succeeds, then deleting the admin_roles
// binding rows fails -> the transaction rolls back and returns the "failed to delete
// user role bindings" error (the propagation path for dangling-reference cleanup
// failure after roles switched to hard delete).
func TestRoleCovB_Delete_AdminRoleRowsError(t *testing.T) {
	s, ctx, inj, db := newRoleCovEnv(t)
	id := seedCovRole(t, db, "covdelete-adminrole")
	inj.failAll["delete:admin_roles"] = true

	err := s.RoleDelete(ctx, &RoleDeleteRequest{ID: id})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "删除用户角色绑定失败")
}
