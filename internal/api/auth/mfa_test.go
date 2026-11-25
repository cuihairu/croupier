package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/identity"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// currentTOTP generates the current window's code with the same algorithm the
// otp package uses (test fixture).
func currentTOTP(t *testing.T, secret string) string {
	t.Helper()
	dec, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	require.NoError(t, err)
	counter := uint64(time.Now().Unix() / 30)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)
	mac := hmac.New(sha1.New, dec)
	mac.Write(buf)
	sum := mac.Sum(nil)
	offset := int(sum[len(sum)-1] & 0x0F)
	bin := (int(sum[offset])&0x7f)<<24 | (int(sum[offset+1])&0xff)<<16 | (int(sum[offset+2])&0xff)<<8 | (int(sum[offset+3]) & 0xff)
	return pad6(bin % 1000000)
}

func pad6(v int) string {
	s := ""
	for i := 0; i < 6; i++ {
		s = string(rune('0'+v%10)) + s
		v /= 10
	}
	return s
}

func TestMFA_SetupConfirmDisable_Lifecycle(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
	ctx := context.Background()
	createTestAdminWithRole(t, db, "mfauser", "CorrectPass123", "admin")

	// setup: returns the secret, not yet enabled
	setup, err := svc.MFASetup(ctx, "mfauser")
	require.NoError(t, err)
	require.NotEmpty(t, setup.Secret)
	assert.False(t, setup.AlreadyDone)
	assert.Contains(t, setup.OtpauthURL, "otpauth://totp/")

	// a wrong code cannot confirm
	_, err = svc.MFAConfirm(ctx, "mfauser", "000000")
	require.Error(t, err)

	// the right code confirms and enables it
	_, err = svc.MFAConfirm(ctx, "mfauser", currentTOTP(t, setup.Secret))
	require.NoError(t, err)
	admin, err := adminModel.FindByUsername(ctx, "mfauser")
	require.NoError(t, err)
	assert.True(t, admin.OTPEnabled)

	// calling setup again returns alreadyEnabled
	again, err := svc.MFASetup(ctx, "mfauser")
	require.NoError(t, err)
	assert.True(t, again.AlreadyDone)

	// disable: a wrong password is rejected
	err = svc.MFADisable(ctx, "mfauser", currentTOTP(t, setup.Secret), "wrong-pass")
	require.Error(t, err)
	// the right code + password disables it
	err = svc.MFADisable(ctx, "mfauser", currentTOTP(t, setup.Secret), "CorrectPass123")
	require.NoError(t, err)
	admin, err = adminModel.FindByUsername(ctx, "mfauser")
	require.NoError(t, err)
	assert.False(t, admin.OTPEnabled)
	assert.Empty(t, admin.OTPSecret)
}

func TestLogin_MFA_RequiredAndVerified(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
	ctx := context.Background()
	createTestAdminWithRole(t, db, "guard", "CorrectPass123", "admin")

	// enable MFA
	setup, err := svc.MFASetup(ctx, "guard")
	require.NoError(t, err)
	_, err = svc.MFAConfirm(ctx, "guard", currentTOTP(t, setup.Secret))
	require.NoError(t, err)

	// no totpCode: returns ErrMFARequired
	_, err = svc.Login(ctx, &LoginRequest{Username: "guard", Password: "CorrectPass123"})
	require.ErrorIs(t, err, ErrMFARequired)

	// a wrong code: rejected
	_, err = svc.Login(ctx, &LoginRequest{Username: "guard", Password: "CorrectPass123", TOTPCode: "000000"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "二次验证码错误")

	// the right code: allowed through
	resp, err := svc.Login(ctx, &LoginRequest{Username: "guard", Password: "CorrectPass123", TOTPCode: currentTOTP(t, setup.Secret)})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
}

func TestLogin_MFA_SkippedForExternalProvider(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
	ctx := context.Background()

	// a shadow account: no password_hash (the external-provider JIT shape), but
	// OTPEnabled unexpectedly true
	shadow := &model.Admin{Username: "ext-user", Status: 1, OTPEnabled: true, OTPSecret: "JBSWY3DPEHPK3PXP"}
	require.NoError(t, db.Create(shadow).Error)

	svc.WithPasswordProvider(stubPasswordProvider{kind: identity.KindLDAP, username: "ext-user"})

	// an external identity provider login does not take the MFA branch
	resp, err := svc.Login(ctx, &LoginRequest{Username: "ext-user", Password: "whatever"})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
}

// stubPasswordProvider simulates an external password identity provider (any
// password is accepted).
type stubPasswordProvider struct {
	kind     string
	username string
}

func (p stubPasswordProvider) Kind() string { return p.kind }

func (p stubPasswordProvider) Authenticate(_ context.Context, username, _ string) (*identity.Identity, error) {
	if username != p.username {
		return nil, identity.ErrInvalidCredentials
	}
	return &identity.Identity{Provider: p.kind, Username: username}, nil
}
