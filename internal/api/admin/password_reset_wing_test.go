// Coverage inspection round 19 (wt-api): close out the follow-up Update error
// wing (354-355) of PasswordReset in service.go — a storage failure while clearing
// the must_change_password/password_expires_at flags must be surfaced so callers can
// tell "password changed but flags not cleared" apart from full success. Injection:
// a map-target-column UPDATE intercept callback (UpdatePassword uses column-name
// Update without a map, so it is unaffected).
package admin

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestService_PasswordReset_ClearUpdateErrorWing(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)
	ctx, _ := createTestAdminWithContext(t, db, "pwresetfailadmin", "MyPass123", "admin")
	service := NewService(svcCtx)

	expired := time.Now().Add(-time.Hour)
	flagged := &model.Admin{
		Username:           "pwresetflagged",
		Nickname:           "Flagged",
		Status:             model.StatusEnabled,
		MustChangePassword: true,
		PasswordExpiresAt:  &expired,
	}
	require.NoError(t, svcCtx.AdminModel.Create(ctx, flagged, "initialPass123"))

	// Intercept only the map-based "cleanup Update" (must_change_password column);
	// the column-name Update of UpdatePassword for password_hash passes through
	require.NoError(t, db.Callback().Update().Before("gorm:update").
		Register("test_r19_fail_pwreset_clear", func(tx *gorm.DB) {
			if m, ok := tx.Statement.Dest.(map[string]interface{}); ok {
				if _, hit := m["must_change_password"]; hit {
					_ = tx.AddError(errors.New("update blocked by test callback"))
				}
			}
		}))
	t.Cleanup(func() {
		_ = db.Callback().Update().Remove("test_r19_fail_pwreset_clear")
	})

	err := service.PasswordReset(ctx, &PasswordResetRequest{
		ID:          strconv.FormatUint(uint64(flagged.ID), 10),
		NewPassword: "resetPass123",
	})
	require.Error(t, err, "善后标记清除失败须上抛")

	// The password itself was already swapped (UpdatePassword ran and succeeded
	// before the cleanup), so the flags stay as they were — do not pretend clean
	var row model.Admin
	require.NoError(t, db.Where("username = ?", "pwresetflagged").First(&row).Error)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(row.PasswordHash), []byte("resetPass123")),
		"UpdatePassword 先于善后，密码本体应已更新")
	assert.True(t, row.MustChangePassword, "善后失败时标记保持原样")
}
