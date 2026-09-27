package admin

import (
	"strconv"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OPEN-ISSUES #20 回归：创建管理员时的密码策略——
// ① mustChangePassword=true 落库并在响应视图透出；
// ② passwordExpiresDays 换算为绝对截止时间 password_expires_at；
// ③ 缺省（0/不标记）保持干净状态；
// ④ 管理员重置密码清除两个标记（新密码视为干净状态）。

func TestService_Create_PasswordPolicy(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	ctx, _ := createTestAdminWithContext(t, db, "superadmin", "MyPass123", "admin")
	service := NewService(svcCtx)

	resp, err := service.Create(ctx, &CreateRequest{
		Username:            "policyadmin",
		Password:            "policyPassword123",
		MustChangePassword:  true,
		PasswordExpiresDays: 90,
	})
	require.NoError(t, err)
	assert.True(t, resp.MustChangePassword, "响应视图应透出 mustChangePassword")
	assert.NotEmpty(t, resp.PasswordExpiresAt, "响应视图应透出密码有效期截止")

	// 响应里的截止时间应落在 now+90d ±1h（换算边界容差）
	expiresAt, err := time.Parse(time.RFC3339, resp.PasswordExpiresAt)
	require.NoError(t, err)
	want := time.Now().Add(90 * 24 * time.Hour)
	assert.WithinDuration(t, want, expiresAt, time.Hour)

	// 落库断言：读回原始行
	var row model.Admin
	require.NoError(t, db.Where("username = ?", "policyadmin").First(&row).Error)
	assert.True(t, row.MustChangePassword)
	require.NotNil(t, row.PasswordExpiresAt)
	assert.WithinDuration(t, want, *row.PasswordExpiresAt, time.Hour)
}

func TestService_Create_PasswordPolicy_Defaults(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	ctx, _ := createTestAdminWithContext(t, db, "superadmin", "MyPass123", "admin")
	service := NewService(svcCtx)

	resp, err := service.Create(ctx, &CreateRequest{
		Username: "plainadmin",
		Password: "plainPassword123",
	})
	require.NoError(t, err)
	assert.False(t, resp.MustChangePassword)
	assert.Empty(t, resp.PasswordExpiresAt)

	var row model.Admin
	require.NoError(t, db.Where("username = ?", "plainadmin").First(&row).Error)
	assert.False(t, row.MustChangePassword)
	assert.Nil(t, row.PasswordExpiresAt)
}

func TestService_PasswordReset_ClearsPasswordPolicy(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	ctx, creatorID := createTestAdminWithContext(t, db, "superadmin", "MyPass123", "admin")
	service := NewService(svcCtx)

	expired := time.Now().Add(-24 * time.Hour)
	flagged := &model.Admin{
		Username:           "flaggedadmin",
		Nickname:           "Flagged",
		Status:             model.StatusEnabled,
		MustChangePassword: true,
		PasswordExpiresAt:  &expired,
	}
	require.NoError(t, svcCtx.AdminModel.Create(ctx, flagged, "initialPass123"))
	_ = creatorID

	require.NoError(t, service.PasswordReset(ctx, &PasswordResetRequest{
		ID:          strconv.FormatUint(uint64(flagged.ID), 10),
		NewPassword: "resetPass123",
	}))

	var row model.Admin
	require.NoError(t, db.Where("username = ?", "flaggedadmin").First(&row).Error)
	assert.False(t, row.MustChangePassword, "重置后必须改密标记应清除")
	assert.Nil(t, row.PasswordExpiresAt, "重置后有效期应清空（长期有效）")
}
