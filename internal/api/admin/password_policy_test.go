package admin

import (
	"strconv"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OPEN-ISSUES #20 regression: password policy when creating an admin —
// ① mustChangePassword=true is landed in the database and surfaced in the response view;
// ② passwordExpiresDays is converted into an absolute deadline password_expires_at;
// ③ defaults (0 / not flagged) keep a clean state;
// ④ an admin password reset clears both flags (the new password is considered clean).

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

	// The deadline in the response should fall within now+90d +/-1h (conversion boundary tolerance)
	expiresAt, err := time.Parse(time.RFC3339, resp.PasswordExpiresAt)
	require.NoError(t, err)
	want := time.Now().Add(90 * 24 * time.Hour)
	assert.WithinDuration(t, want, expiresAt, time.Hour)

	// Persistence assertion: read the raw row back
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
