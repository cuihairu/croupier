// 覆盖率巡检第十九轮（wt-api）：service.go PasswordReset 的善后
// Update 错误翼（354-355）收口——重置密码后清除 must_change_password/
// password_expires_at 标记时存储故障须上抛（调用方能区分「密码已换但
// 标记未清」与全部成功）。注入：map 目标列 UPDATE 拦截回调
// （UpdatePassword 走列名 Update、不带 map，不受影响）。
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

	// 只拦「善后 Update」的 map 写（must_change_password 列）；
	// UpdatePassword 的 password_hash 列名写法照常放行
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

	// 密码本体已换（UpdatePassword 先于善后执行成功），
	// 标记保持原样——不假装干净
	var row model.Admin
	require.NoError(t, db.Where("username = ?", "pwresetflagged").First(&row).Error)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(row.PasswordHash), []byte("resetPass123")),
		"UpdatePassword 先于善后，密码本体应已更新")
	assert.True(t, row.MustChangePassword, "善后失败时标记保持原样")
}
