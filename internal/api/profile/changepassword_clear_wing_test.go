// 覆盖率巡检第十九轮（wt-api）：service.go ChangePassword 的善后
// Update 错误翼（400-401）收口——改密成功后清除 must_change_password/
// 重算 password_expires_at 时存储故障须上抛。注入：复用本包
// registerFailUpdateCallback 只拦 must_change_password 列的 map 写
// （UpdatePassword/BumpTokenVersion 均为列名写法，不受影响）。
package profile

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_ChangePassword_ClearPolicyUpdateError(t *testing.T) {
	db := setupTestDB(t)
	service := newRealService(t)
	createPlainAdmin(t, db, "clearpolicyfailuser")

	registerFailUpdateCallback(t, db, "test_r19_fail_must_change", "must_change_password")

	_, err := service.ChangePassword(context.Background(), "clearpolicyfailuser", &ChangePasswordRequest{
		OldPassword: "password123",
		NewPassword: "newpassword456",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "修改密码失败")
}
