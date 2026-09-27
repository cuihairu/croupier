package profile

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OPEN-ISSUES #20 回归：自助修改密码后解除密码策略标记——
// must_change_password 清零、password_expires_at 置空（新密码视为干净状态），
// 下次登录不再被强制改密。

func TestService_ChangePassword_ClearsPasswordPolicy(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := model.NewAdminModel(db)
	service := NewService(adminModel, svcCtx.GameModel, model.NewRoleModel(db))

	expired := time.Now().Add(-24 * time.Hour)
	admin := &model.Admin{
		Username:           "clearpolicy",
		Nickname:           "Clear Policy",
		Status:             1,
		MustChangePassword: true,
		PasswordExpiresAt:  &expired,
	}
	require.NoError(t, adminModel.Create(context.Background(), admin, "oldpassword123"))

	resp, err := service.ChangePassword(context.Background(), "clearpolicy", &ChangePasswordRequest{
		OldPassword: "oldpassword123",
		NewPassword: "newpassword123",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Ok)

	var row model.Admin
	require.NoError(t, db.Where("username = ?", "clearpolicy").First(&row).Error)
	assert.False(t, row.MustChangePassword, "改密后必须改密标记应清除")
	assert.Nil(t, row.PasswordExpiresAt, "改密后有效期应清空")
}
