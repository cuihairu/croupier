package auth

// R51 coverage gap fill: the two flanks of email_verification.go
// - :78-79 siteServerURL() when settings.Current()=nil → empty string,
//   triggered by resetForTest() setting layered=nil; no extra scaffolding
// - :162-164 VerifyEmailToken Consume returning false (concurrent consume /
//   already used) branch: a sqlite BEFORE UPDATE TRIGGER + RAISE(IGNORE)
//   makes the UPDATE silently skip → RowsAffected=0 → Consume returns false
//   → the error path at :162-164

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
	// resetForTest sets layered=nil, covering :78-79
	settings.ResetForTest()
	defer func() {
		// restore the default layer so later tests are not polluted
		settings.ResetForTest()
	}()

	url := siteServerURL()
	assert.Empty(t, url, "settings 为 nil 时应返回空串（:78-79）")
}

func TestVerifyEmailToken_ConsumeReturnsFalse_RaisesError(t *testing.T) {
	// build a registration service with the verification model
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

	// register a user and capture the plaintext token
	sent, restore := hookVerificationSender(t, nil)
	defer restore()
	_, err = svc.Register(context.Background(), &RegisterRequest{
		Username: "consume-false-1", Password: "Str0ngPass!x", Email: "consume-false-1@example.com",
	})
	require.NoError(t, err)
	require.Len(t, *sent, 1)
	token := (*sent)[0].token

	// inject a BEFORE UPDATE TRIGGER + RAISE(IGNORE) so Consume's UPDATE is
	// silently skipped. The UPDATE statement is:
	// UPDATE email_verifications SET used_at = ? WHERE id = ? AND used_at IS NULL
	require.NoError(t, db.Exec(`CREATE TRIGGER ev_ignore_consume BEFORE UPDATE ON email_verifications
		WHEN NEW.used_at IS NOT NULL AND OLD.used_at IS NULL
		BEGIN SELECT RAISE(IGNORE); END`).Error)

	// verifying the same token again → FindValidByTokenHash still finds the
	// row (used_at not stamped), but Consume's UPDATE is skipped by the trigger
	// → RowsAffected=0 → Consume returns false → VerifyEmailToken takes
	// :162-164 and returns the "verification link invalid or expired" message
	err = svc.VerifyEmailToken(context.Background(), token)
	require.Error(t, err, "Consume 返回 false 时应报错（:162-164）")
	assert.Contains(t, err.Error(), "验证链接无效或已过期")

	// drop the trigger so later cases sharing this DB are unaffected
	require.NoError(t, db.Exec("DROP TRIGGER ev_ignore_consume").Error)
}
