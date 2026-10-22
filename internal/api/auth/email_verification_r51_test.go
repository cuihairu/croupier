package auth

// R51 覆盖率补缺：email_verification.go 两翼
// - :78-79 siteServerURL() 当 settings.Current()=nil → 空串
//   由 resetForTest() 置 layered=nil 触发，无需额外搭建
// - :162-164 VerifyEmailToken Consume 返回 false（并发消费/已用）分支
//   用 sqlite BEFORE UPDATE TRIGGER + RAISE(IGNORE) 让 UPDATE 静默跳过
//   → RowsAffected=0 → Consume 返回 false → 走 :162-164 报错

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/cuihairu/croupier/internal/security/jwtutil"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSiteServerURL_ReturnsEmptyWhenSettingsNil(t *testing.T) {
	// resetForTest 置 layered=nil，覆盖 :78-79
	settings.ResetForTest()
	defer func() {
		// 恢复默认层，避免污染后续测试
		settings.ResetForTest()
	}()

	url := siteServerURL()
	assert.Empty(t, url, "settings 为 nil 时应返回空串（:78-79）")
}

func TestVerifyEmailToken_ConsumeReturnsFalse_RaisesError(t *testing.T) {
	// 构造带验证模型的注册服务
	seedEmailPolicy(t, map[string]string{settings.KeyAuthEmailVerificationRequired: `true`})
	db := setupTestDB(t)
	verifModel := model.NewEmailVerificationModel(db)
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), jwtutil.DevSecret()).
		WithRecoveryDB(db).
		WithRoleModel(model.NewRoleModel(db)).
		WithVerificationModel(verifModel)
	ip, err := BuildIdentityProviders(config.AuthProvidersConfig{
		Register: config.RegisterConfig{Enabled: true},
	})
	require.NoError(t, err)
	svc = ip.Attach(svc)

	// 注册一个用户并获取明文 token
	sent, restore := hookVerificationSender(t, nil)
	defer restore()
	_, err = svc.Register(context.Background(), &RegisterRequest{
		Username: "consume-false-1", Password: "Str0ngPass!x", Email: "consume-false-1@example.com",
	})
	require.NoError(t, err)
	require.Len(t, *sent, 1)
	token := (*sent)[0].token

	// 注入 BEFORE UPDATE TRIGGER + RAISE(IGNORE) 让 Consume 的 UPDATE 静默跳过
	// UPDATE 语句：UPDATE email_verifications SET used_at = ? WHERE id = ? AND used_at IS NULL
	require.NoError(t, db.Exec(`CREATE TRIGGER ev_ignore_consume BEFORE UPDATE ON email_verifications
		WHEN NEW.used_at IS NOT NULL AND OLD.used_at IS NULL
		BEGIN SELECT RAISE(IGNORE); END`).Error)

	// 再次验证同一 token → FindValidByTokenHash 能查到行（未被标记 used_at）
	// 但 Consume 的 UPDATE 被 trigger 跳过 → RowsAffected=0 → Consume 返回 false
	// → VerifyEmailToken 走 :162-164 返回 "验证链接无效或已过期"
	err = svc.VerifyEmailToken(context.Background(), token)
	require.Error(t, err, "Consume 返回 false 时应报错（:162-164）")
	assert.Contains(t, err.Error(), "验证链接无效或已过期")

	// 清理 trigger（同库后续用例不受影响）
	require.NoError(t, db.Exec("DROP TRIGGER ev_ignore_consume").Error)
}
