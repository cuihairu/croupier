package auth

// Coverage gap fill for mfa.go / service.go (audit 98.4%→):
// - the With* builder options were previously at 0% (their comments target
//   test injection anyway)
// - WithOTPRecoveryModel injects a "closed database" recovery-code model to
//   drive the Consume/Count/DeleteAll error branches and the degraded paths in
//   MFAStatus and the login second-factor check
// - guard branches such as requireLocalAdmin on a missing account and
//   OtpauthURI rejecting a blank account
// Every case is a reachable branch; no filler tests.

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newBrokenRecoveryModel is a separate database that is opened then closed
// immediately: every recovery-code model operation errors, without affecting
// the business database (which stays usable).
func newBrokenRecoveryModel(t *testing.T) *model.AdminOTPRecoveryCodeModel {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	return model.NewAdminOTPRecoveryCodeModel(db)
}

// enableMFAFor runs the full Setup+Confirm to enable TOTP, returning the setup
// secret for generating current-window codes afterwards.
func enableMFAFor(t *testing.T, db *gorm.DB, svc *Service, username string) string {
	t.Helper()
	setup, err := svc.MFASetup(context.Background(), username)
	require.NoError(t, err)
	_, err = svc.MFAConfirm(context.Background(), username, currentTOTP(t, setup.Secret))
	require.NoError(t, err)
	return setup.Secret
}

// With* builder options: return the receiver and wire the fields
// (service.go was previously at 0%).
func TestMFA_ServiceBuilderOptions(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	recoveryModel := model.NewAdminOTPRecoveryCodeModel(db)
	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret")

	got := svc.WithAdminModel(adminModel)
	assert.Same(t, svc, got)
	assert.Same(t, adminModel, got.adminModel)

	got = svc.WithOTPRecoveryModel(recoveryModel)
	assert.Same(t, svc, got)
	assert.Same(t, recoveryModel, got.otpRecoveryModel)
}

// MFASetup passes through requireLocalAdmin's error for a missing account.
func TestMFA_Setup_UnknownUser(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)

	_, err := svc.MFASetup(context.Background(), "nobody")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "用户不存在")
}

// A username that is exactly searchable but trims to empty → OtpauthURI
// rejects the blank account, covering the "generate bind link failed" guard
// branch (so a blank account cannot quietly produce a broken link).
func TestMFA_Setup_BlankUsername_OtpauthURIError(t *testing.T) {
	db := setupTestDB(t)
	createTestAdminWithRole(t, db, " ", "CorrectPass123", "admin")
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)

	_, err := svc.MFASetup(context.Background(), " ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "生成绑定链接失败")
}

// A recovery-code cleanup failure must not block the disable flow, but it has
// to leave an error log (leftover codes are a security hole).
func TestMFA_Disable_RecoveryCleanupError_NotBlocking(t *testing.T) {
	db := setupTestDB(t)
	createTestAdminWithRole(t, db, "dluser", "CorrectPass123", "admin")
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
	secret := enableMFAFor(t, db, svc, "dluser")

	svc.WithOTPRecoveryModel(newBrokenRecoveryModel(t))
	require.NoError(t, svc.MFADisable(context.Background(), "dluser", currentTOTP(t, secret), "CorrectPass123"))

	admin, err := svc.adminModel.FindByUsername(context.Background(), "dluser")
	require.NoError(t, err)
	assert.False(t, admin.OTPEnabled)
}

// Consume/Count storage errors pass through unchanged; a malformed code is
// rejected before any database access.
func TestMFA_RecoveryCode_ModelErrors_PassThrough(t *testing.T) {
	db := setupTestDB(t)
	id := createTestAdminWithRole(t, db, "rcuser", "CorrectPass123", "admin")
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").
		WithOTPRecoveryModel(newBrokenRecoveryModel(t))
	ctx := context.Background()

	// ABDC23EFGH: well formed (10 characters, within the alphabet) but the
	// storage fails → the error is passed through
	ok, err := svc.ConsumeRecoveryCode(ctx, id, "ABCD23EFGH")
	require.Error(t, err)
	assert.False(t, ok)

	// malformed → rejected without touching the database
	ok, err = svc.ConsumeRecoveryCode(ctx, id, "short")
	require.NoError(t, err)
	assert.False(t, ok)

	n, err := svc.CountRecoveryCodes(ctx, id)
	require.Error(t, err)
	assert.Zero(t, n)
}

// Login second-factor check: a well-formed recovery code whose storage fails →
// verification fails and the login is denied (never let it through by mistake).
func TestMFA_Login_RecoveryModelError_Denies(t *testing.T) {
	db := setupTestDB(t)
	createTestAdminWithRole(t, db, "mfaerr", "CorrectPass123", "admin")
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
	enableMFAFor(t, db, svc, "mfaerr")

	svc.WithOTPRecoveryModel(newBrokenRecoveryModel(t))
	_, err := svc.Login(context.Background(), &LoginRequest{
		Username: "mfaerr",
		Password: "CorrectPass123",
		TOTPCode: "ABCD23EFGH",
	})
	require.Error(t, err)
}

// MFAStatus: a remaining-count failure only degrades to 0 (advisory data must
// not block the status query).
func TestMFA_Status_RecoveryCountError_Degrades(t *testing.T) {
	db := setupTestDB(t)
	createTestAdminWithRole(t, db, "stuser", "CorrectPass123", "admin")
	svc := NewService(model.NewAdminModel(db), permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)
	enableMFAFor(t, db, svc, "stuser")

	svc.WithOTPRecoveryModel(newBrokenRecoveryModel(t))
	out := svc.MFAStatus(context.Background(), "stuser")
	assert.True(t, out.Enabled)
	assert.True(t, out.Local)
	assert.Zero(t, out.RecoveryCodesRemaining)
	assert.Zero(t, out.RecoveryCodeTotal)
}
