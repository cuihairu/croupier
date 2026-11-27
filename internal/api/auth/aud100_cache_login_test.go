package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/model"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AUD-100 end-to-end assertion: warming the admin profile cache (ID + username
// alias, explicit credential-free DTO) must not affect login or the TOTP
// second factor — both read the authoritative DB record through AdminModel.
func TestAUD100_LoginAndMFAReadCredentialsFromDBNotCache(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	store := cache.NewLocalCache(time.Minute, 10*time.Minute)
	svcCtx := &svc.ServiceContext{
		DB:          db,
		AdminModel:  adminModel,
		Cache:       store,
		CacheHelper: cache.NewCacheHelper(store),
	}
	ctx := context.Background()
	createTestAdminWithRole(t, db, "aud100user", "CorrectPass123", "admin")

	svc := NewService(adminModel, permissionservice.NewPermissionService(db), "test-secret").WithRecoveryDB(db)

	// Enable TOTP, then warm the profile cache via the cache-layer getters.
	setup, err := svc.MFASetup(ctx, "aud100user")
	require.NoError(t, err)
	_, err = svc.MFAConfirm(ctx, "aud100user", currentTOTP(t, setup.Secret))
	require.NoError(t, err)

	cached, err := svcCtx.GetAdminByUsernameCached(ctx, "AUD100User")
	require.NoError(t, err)
	assert.Empty(t, cached.PasswordHash, "cache must not expose the password hash")
	assert.Empty(t, cached.OTPSecret, "cache must not expose the TOTP secret")
	assert.False(t, cached.OTPEnabled, "cache must not expose MFA state")

	raw, err := store.Get(ctx, cache.AdminCacheKey("aud100user"))
	require.NoError(t, err)
	assert.False(t, strings.Contains(string(raw), "PasswordHash") ||
		strings.Contains(string(raw), "OTPSecret") ||
		strings.Contains(string(raw), setup.Secret),
		"raw cached admin JSON must not contain credential material: %s", string(raw))

	// DB is the authority: password and TOTP verification still succeed.
	_, err = svc.Login(ctx, &LoginRequest{Username: "aud100user", Password: "wrong-password", TOTPCode: currentTOTP(t, setup.Secret)})
	require.Error(t, err, "wrong password must fail even with a warm cache")

	_, err = svc.Login(ctx, &LoginRequest{Username: "aud100user", Password: "CorrectPass123"})
	require.ErrorIs(t, err, ErrMFARequired, "MFA gate must still trigger from DB state")

	resp, err := svc.Login(ctx, &LoginRequest{
		Username: "aud100user",
		Password: "CorrectPass123",
		TOTPCode: currentTOTP(t, setup.Secret),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, "aud100user", resp.User.Username)
}
