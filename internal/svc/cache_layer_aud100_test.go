package svc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// AUD-100: admin profile cache must serialize only business fields. Every
// assertion below reads the raw cached bytes (never a typed getter), so
// accidentally caching the full model.Admin is caught — json/omitempty on a
// typed DTO would hide the defect.

var adminCredentialFieldNames = []string{
	"PasswordHash", "password_hash", "passwordHash",
	"OTPSecret", "otp_secret", "otpSecret",
}

func assertCachedAdminBytesAreCredentialFree(t *testing.T, data []byte) {
	t.Helper()
	raw := string(data)
	for _, name := range adminCredentialFieldNames {
		assert.NotContains(t, raw, name, "cached admin JSON must not contain %q", name)
	}
	// The old full-model payload also exposed bcrypt hashes and base32 secrets;
	// check the actual value shapes, not just field names.
	assert.NotContains(t, raw, "$2a$", "cached admin JSON must not carry a bcrypt hash")
	assert.NotContains(t, raw, "JBSWY3DPEHPK3PXP", "cached admin JSON must not carry a TOTP secret")

	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &fields), "cached admin must be JSON: %s", raw)
	assert.Equal(t, float64(adminCacheSchemaVersion), mustJSONNumber(t, fields["schemaVersion"]))
	assert.NotEmpty(t, fields["username"], "profile cache must keep username")
}

func mustJSONNumber(t *testing.T, raw json.RawMessage) float64 {
	t.Helper()
	var n float64
	require.NoError(t, json.Unmarshal(raw, &n), "expected JSON number, got %s", raw)
	return n
}

// aud100AdminFixture creates a real DB admin with credential material and
// caches it via both public getters.
type aud100Fixture struct {
	ctx    *ServiceContext
	bg     context.Context
	admin  *model.Admin
	secret string
}

func newAud100Fixture(t *testing.T, store cache.CacheStore) aud100Fixture {
	t.Helper()
	svcCtx := newSvcTestContext(t)
	svcCtx.Cache = store
	svcCtx.CacheHelper = cache.NewCacheHelper(store)

	bg := context.Background()
	locked := time.Now().Add(30 * time.Minute).UTC().Truncate(time.Second)
	admin := &model.Admin{
		Username:           "cacheadmin",
		Nickname:           "Cached Nick",
		Email:              "cache@example.test",
		Phone:              "13800000000",
		Avatar:             "avatar-key",
		Status:             1,
		MustChangePassword: true,
		PasswordExpiresAt:  &locked,
		LastLoginAt:        &locked,
		LastGameID:         "demo",
		LastEnv:            "prod",
	}
	require.NoError(t, svcCtx.AdminModel.Create(bg, admin, "CorrectPass123"))
	require.NoError(t, svcCtx.AdminModel.SetOTPSecret(bg, admin.ID, "JBSWY3DPEHPK3PXP"))
	require.NoError(t, svcCtx.AdminModel.EnableOTP(bg, admin.ID))

	byID, err := svcCtx.GetAdminCached(bg, admin.ID)
	require.NoError(t, err)
	require.NotNil(t, byID)
	byName, err := svcCtx.GetAdminByUsernameCached(bg, " CacheAdmin ")
	require.NoError(t, err)
	require.NotNil(t, byName)

	return aud100Fixture{ctx: svcCtx, bg: bg, admin: admin, secret: "JBSWY3DPEHPK3PXP"}
}

func TestAUD100_LocalCacheBothKeysSerializeCredentialFreeDTO(t *testing.T) {
	store := cache.NewLocalCache(time.Minute, 10*time.Minute)
	f := newAud100Fixture(t, store)

	idBytes, err := store.Get(f.bg, cache.AdminIDCacheKey(f.admin.ID))
	require.NoError(t, err)
	assertCachedAdminBytesAreCredentialFree(t, idBytes)

	aliasBytes, err := store.Get(f.bg, cache.AdminCacheKey("cacheadmin"))
	require.NoError(t, err)
	assertCachedAdminBytesAreCredentialFree(t, aliasBytes)
	assert.JSONEq(t, string(idBytes), string(aliasBytes), "ID and username alias must share one DTO payload")
}

func TestAUD100_RedisCacheBothKeysSerializeCredentialFreeDTO(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	store, err := cache.NewRedisCache(mr.Addr(), "", 0, 10, time.Minute)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()
	f := newAud100Fixture(t, store)

	idBytes, err := store.Get(f.bg, cache.AdminIDCacheKey(f.admin.ID))
	require.NoError(t, err)
	assertCachedAdminBytesAreCredentialFree(t, idBytes)

	aliasBytes, err := store.Get(f.bg, cache.AdminCacheKey("cacheadmin"))
	require.NoError(t, err)
	assertCachedAdminBytesAreCredentialFree(t, aliasBytes)
	assert.JSONEq(t, string(idBytes), string(aliasBytes), "ID and username alias must share one DTO payload")
}

func TestAUD100_LegacyFullModelRecordIsIgnoredAndOverwritten(t *testing.T) {
	store := cache.NewLocalCache(time.Minute, 10*time.Minute)
	f := newAud100Fixture(t, store)

	// Simulate the pre-fix cache entry: full model.Admin JSON, credentials
	// included, no schemaVersion.
	legacy := map[string]string{
		"PasswordHash": "$2a$10$legacyhashplaceholdervalue000000000000000000000000000000000",
		"OTPSecret":    f.secret,
		"Username":     "cacheadmin",
		"Nickname":     "Legacy Nick",
	}
	legacyBytes, err := json.Marshal(legacy)
	require.NoError(t, err)
	require.NoError(t, store.Set(f.bg, cache.AdminIDCacheKey(f.admin.ID), legacyBytes, time.Minute))
	require.NoError(t, store.Set(f.bg, cache.AdminCacheKey("cacheadmin"), legacyBytes, time.Minute))

	got, err := f.ctx.GetAdminCached(f.bg, f.admin.ID)
	require.NoError(t, err)
	assert.Equal(t, "Cached Nick", got.Nickname, "stale legacy data must not be returned")

	cleaned, err := store.Get(f.bg, cache.AdminIDCacheKey(f.admin.ID))
	require.NoError(t, err)
	assertCachedAdminBytesAreCredentialFree(t, cleaned)

	byName, err := f.ctx.GetAdminByUsernameCached(f.bg, "cacheadmin")
	require.NoError(t, err)
	assert.Equal(t, "Cached Nick", byName.Nickname)
	aliasCleaned, err := store.Get(f.bg, cache.AdminCacheKey("cacheadmin"))
	require.NoError(t, err)
	assertCachedAdminBytesAreCredentialFree(t, aliasCleaned)
}

func TestAUD100_CacheReadsPreserveProfileAndRoleFields(t *testing.T) {
	store := cache.NewLocalCache(time.Minute, 10*time.Minute)
	f := newAud100Fixture(t, store)

	role := &model.Role{Name: "cacheauditor", Description: "d"}
	require.NoError(t, f.ctx.DB.Create(role).Error)
	require.NoError(t, f.ctx.AdminModel.AssignRole(f.bg, f.admin.ID, role.ID))

	// Drop the DB row to prove the second read is served from the cache.
	require.NoError(t, f.ctx.DB.Delete(f.admin).Error)

	byID, err := f.ctx.GetAdminCached(f.bg, f.admin.ID)
	require.NoError(t, err)
	assert.Equal(t, f.admin.ID, byID.ID)
	assert.Equal(t, "cacheadmin", byID.Username)
	assert.Equal(t, "Cached Nick", byID.Nickname)
	assert.Equal(t, "cache@example.test", byID.Email)
	assert.Equal(t, "13800000000", byID.Phone)
	assert.Equal(t, "avatar-key", byID.Avatar)
	assert.Equal(t, 1, byID.Status)
	assert.True(t, byID.MustChangePassword)
	assert.Equal(t, "demo", byID.LastGameID)
	assert.Equal(t, "prod", byID.LastEnv)

	byName, err := f.ctx.GetAdminByUsernameCached(f.bg, "cacheadmin")
	require.NoError(t, err)
	assert.Equal(t, f.admin.ID, byName.ID)
	assert.Equal(t, "cacheadmin", byName.Username)

	roles, err := f.ctx.GetAdminRolesCached(f.bg, byID.ID)
	require.NoError(t, err)
	require.Len(t, roles, 1)
	assert.Equal(t, "cacheauditor", roles[0].Name)
}

func TestAUD100_CacheNeverSuppliesCredentials_DBRemainsAuthoritative(t *testing.T) {
	store := cache.NewLocalCache(time.Minute, 10*time.Minute)
	f := newAud100Fixture(t, store)

	// Cache hit must not expose the credential material.
	fromCache, err := f.ctx.GetAdminCached(f.bg, f.admin.ID)
	require.NoError(t, err)
	assert.Empty(t, fromCache.PasswordHash)
	assert.Empty(t, fromCache.OTPSecret)
	assert.False(t, fromCache.OTPEnabled)

	// But the DB still holds the credentials, so password and TOTP verification
	// paths (which go through AdminModel) keep working.
	fromDB, err := f.ctx.AdminModel.FindByUsername(f.bg, "cacheadmin")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(fromDB.PasswordHash, "$2a$"), "bcrypt hash must stay in DB")
	assert.Equal(t, f.secret, fromDB.OTPSecret)
	assert.True(t, fromDB.OTPEnabled)

	validated, err := f.ctx.AdminModel.ValidatePassword(f.bg, "cacheadmin", "CorrectPass123")
	require.NoError(t, err)
	assert.Equal(t, f.admin.ID, validated.ID)

	// Changing credentials only in the DB must not leak through the cached DTO.
	require.NoError(t, f.ctx.AdminModel.SetOTPSecret(f.bg, f.admin.ID, "NEWSECRET234567"))
	fromCache, err = f.ctx.GetAdminCached(f.bg, f.admin.ID)
	require.NoError(t, err)
	assert.Empty(t, fromCache.OTPSecret)
	assert.NotContains(t, string(mustGetRaw(t, store, cache.AdminIDCacheKey(f.admin.ID))), "NEWSECRET234567")
}

func mustGetRaw(t *testing.T, store cache.CacheStore, key string) []byte {
	t.Helper()
	data, err := store.Get(context.Background(), key)
	require.NoError(t, err)
	return data
}

// Ensure gorm.Model's ID is the only embedded field reconstructed: no hidden
// credential field rides along via the embedded struct on a cache hit.
func TestAUD100_ReconstructedAdminHasNoEmbeddedCredentialFields(t *testing.T) {
	data, err := json.Marshal(adminCacheEntryFromAdmin(&model.Admin{
		Model:        gorm.Model{ID: 7},
		Username:     "u",
		PasswordHash: "hash",
		OTPSecret:    "secret",
	}))
	require.NoError(t, err)
	assertCachedAdminBytesAreCredentialFree(t, data)
}
