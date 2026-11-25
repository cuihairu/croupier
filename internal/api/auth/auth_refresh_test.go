package auth

// AUD-052 refresh-token rotation tests:
// - legacy login (default request) behavior unchanged: single 24h token, no
//   refreshToken field
// - opt-in login (useRefresh=true) issues an access+refresh pair and lands the
//   rotation state
// - redemption rotation: old token marked redeemed, new token chained to it
// - replay detection: an already-redeemed token reappearing → all refresh
//   tokens for the account revoked + audited
// - bidirectional rejection: an access token cannot enter the redemption
//   endpoint; version bump / expiry / unknown token all rejected
// - store unavailable falls back to legacy (login not blocked by rotation
//   infrastructure failure)
// Assertions always use subset filtering (the InMemoryAuditStore's underlying
// map iteration is unordered — the AUD-049 lesson).

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/jwtutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aud052Env assembles the business database with refresh-token store and audit
// (isomorphic to production routes assembly: WithRecoveryDB(db) →
// refreshTokenModel is lazily built).
func aud052Env(t *testing.T) (*Service, *model.AdminModel, *model.AdminRefreshTokenModel, *audit.InMemoryAuditStore, *gorm.DB) {
	t.Helper()
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AdminRefreshToken{}))
	adminModel := model.NewAdminModel(db)
	store := audit.NewInMemoryAuditStore()
	svc := NewService(adminModel, nil, "aud052-secret").
		WithRecoveryDB(db).
		WithAuditService(audit.NewAuditService(store, nil))
	return svc, adminModel, model.NewAdminRefreshTokenModel(db), store, db
}

func aud052Admin(t *testing.T, adminModel *model.AdminModel) uint {
	t.Helper()
	admin := &model.Admin{Username: "aud052", Nickname: "aud052", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "Str0ngPass!x"))
	return admin.ID
}

func aud052Records(t *testing.T, store *audit.InMemoryAuditStore, event audit.AuditEventType) []*audit.AuditRecord {
	t.Helper()
	records, _, err := store.List(audit.AuditFilter{EventType: []audit.AuditEventType{event}}, audit.AuditPage{Page: 1, PageSize: 50})
	require.NoError(t, err)
	return records
}

// Legacy login (default request) is completely unchanged: no refreshToken, the
// token has no typ.
func TestAud052_LegacyLoginUnchanged(t *testing.T) {
	svc, adminModel, _, _, _ := aud052Env(t)
	aud052Admin(t, adminModel)

	resp, err := svc.Login(context.Background(), &LoginRequest{Username: "aud052", Password: "Str0ngPass!x"})
	require.NoError(t, err)
	assert.Empty(t, resp.RefreshToken, "legacy 登录不得携带刷新令牌")
	claims, err := jwtutil.Parse(resp.Token, "aud052-secret")
	require.NoError(t, err)
	assert.Empty(t, claims.TokenType)
	span := claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time)
	assert.Equal(t, 24*time.Hour, span, "legacy token 仍为 24h")
}

// Opt-in login issues the access+refresh pair and lands the hashed row
// (not redeemed, not rotated).
func TestAud052_LoginWithRefreshIssuesPair(t *testing.T) {
	svc, adminModel, refreshModel, _, _ := aud052Env(t)
	aud052Admin(t, adminModel)

	resp, err := svc.Login(context.Background(), &LoginRequest{Username: "aud052", Password: "Str0ngPass!x", UseRefresh: true})
	require.NoError(t, err)
	require.NotEmpty(t, resp.RefreshToken)
	assert.Equal(t, int64(7200), resp.ExpiresIn)

	access, err := jwtutil.Parse(resp.Token, "aud052-secret")
	require.NoError(t, err)
	assert.Equal(t, jwtutil.TokenTypeAccess, access.TokenType)

	refresh, err := jwtutil.ParseRefresh(resp.RefreshToken, "aud052-secret")
	require.NoError(t, err)
	assert.Equal(t, "aud052", refresh.Username)

	row, err := refreshModel.FindByHash(context.Background(), model.HashRefreshToken(resp.RefreshToken))
	require.NoError(t, err)
	assert.Nil(t, row.UsedAt, "新签发的刷新令牌未兑换")
	assert.False(t, row.Revoked)
	assert.Zero(t, row.RotatedFromID, "登录直发无兑换链上游")
}

// Redemption rotation: returns the new pair; the old token is marked redeemed;
// the new token is chained to it; the new access token works.
func TestAud052_RefreshRotates(t *testing.T) {
	svc, adminModel, refreshModel, _, _ := aud052Env(t)
	aud052Admin(t, adminModel)
	ctx := context.Background()

	login, err := svc.Login(ctx, &LoginRequest{Username: "aud052", Password: "Str0ngPass!x", UseRefresh: true})
	require.NoError(t, err)

	out, err := svc.Refresh(ctx, &RefreshRequest{RefreshToken: login.RefreshToken, ClientIP: "127.0.0.1"})
	require.NoError(t, err)
	assert.NotEqual(t, login.RefreshToken, out.RefreshToken, "轮换必须签发新刷新令牌")
	assert.Equal(t, int64(7200), out.ExpiresIn)

	oldRow, err := refreshModel.FindByHash(ctx, model.HashRefreshToken(login.RefreshToken))
	require.NoError(t, err)
	require.NotNil(t, oldRow.UsedAt, "旧令牌应已标记兑换")

	newRow, err := refreshModel.FindByHash(ctx, model.HashRefreshToken(out.RefreshToken))
	require.NoError(t, err)
	assert.Equal(t, oldRow.ID, newRow.RotatedFromID, "新令牌挂兑换链")

	newAccess, err := jwtutil.Parse(out.Token, "aud052-secret")
	require.NoError(t, err)
	assert.Equal(t, jwtutil.TokenTypeAccess, newAccess.TokenType)
}

// Replay detection: an already-redeemed old token reappearing → all refresh
// tokens for the account revoked + audited.
func TestAud052_RefreshReplayRevokesFamily(t *testing.T) {
	svc, adminModel, _, store, db := aud052Env(t)
	aud052Admin(t, adminModel)
	ctx := context.Background()

	login, err := svc.Login(ctx, &LoginRequest{Username: "aud052", Password: "Str0ngPass!x", UseRefresh: true})
	require.NoError(t, err)
	first, err := svc.Refresh(ctx, &RefreshRequest{RefreshToken: login.RefreshToken})
	require.NoError(t, err)

	// Replay the old token (the first link in the attacker's hand).
	_, err = svc.Refresh(ctx, &RefreshRequest{RefreshToken: login.RefreshToken})
	require.Error(t, err)

	// Family-wide revocation: the newest token the attacker legitimately
	// obtained through normal rotation is voided too (fail-closed).
	_, err = svc.Refresh(ctx, &RefreshRequest{RefreshToken: first.RefreshToken})
	require.Error(t, err, "重放吊销后同族新令牌不得再兑换")

	var revoked int64
	require.NoError(t, db.Model(&model.AdminRefreshToken{}).
		Where("admin_id = ? AND revoked = ?", 1, true).Count(&revoked).Error)
	assert.EqualValues(t, 2, revoked, "登录直发 + 轮换两行都应被吊销")

	// Each of the two failed redemptions leaves one replay audit record (one
	// each for the redeemed old token and the revoked new token).
	reuseRecords := aud052Records(t, store, audit.EventTokenRefreshReuse)
	require.Len(t, reuseRecords, 2, "重放必须留审计痕")
	for _, r := range reuseRecords {
		assert.Equal(t, "failure", r.Outcome)
	}
}

// Bidirectional rejection: an access token cannot enter the redemption endpoint
// (ParseRefresh rejects non-refresh token types).
func TestAud052_RefreshRejectsAccessToken(t *testing.T) {
	svc, adminModel, _, _, _ := aud052Env(t)
	aud052Admin(t, adminModel)

	login, err := svc.Login(context.Background(), &LoginRequest{Username: "aud052", Password: "Str0ngPass!x", UseRefresh: true})
	require.NoError(t, err)
	_, err = svc.Refresh(context.Background(), &RefreshRequest{RefreshToken: login.Token})
	assert.Error(t, err)
}

// After a token_version bump (password change/disable/logout), the old
// refresh-token family is immediately invalid.
func TestAud052_RefreshRejectsAfterTokenVersionBump(t *testing.T) {
	svc, adminModel, _, _, _ := aud052Env(t)
	adminID := aud052Admin(t, adminModel)
	ctx := context.Background()

	login, err := svc.Login(ctx, &LoginRequest{Username: "aud052", Password: "Str0ngPass!x", UseRefresh: true})
	require.NoError(t, err)

	require.NoError(t, adminModel.Update(ctx, adminID, map[string]interface{}{"token_version": 7}))

	_, err = svc.Refresh(ctx, &RefreshRequest{RefreshToken: login.RefreshToken})
	require.Error(t, err, "版本不一致的刷新令牌必须拒绝")
}

// An expired persisted row (the JWT itself not yet expired) rejects redemption;
// an unknown hash (row deleted / different database) is rejected likewise.
func TestAud052_RefreshRejectsExpiredAndUnknownRows(t *testing.T) {
	svc, adminModel, refreshModel, _, db := aud052Env(t)
	aud052Admin(t, adminModel)
	ctx := context.Background()

	login, err := svc.Login(ctx, &LoginRequest{Username: "aud052", Password: "Str0ngPass!x", UseRefresh: true})
	require.NoError(t, err)

	row, err := refreshModel.FindByHash(ctx, model.HashRefreshToken(login.RefreshToken))
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.AdminRefreshToken{}).Where("id = ?", row.ID).
		Update("expires_at", time.Now().Add(-time.Minute)).Error)
	_, err = svc.Refresh(ctx, &RefreshRequest{RefreshToken: login.RefreshToken})
	assert.Error(t, err, "落库过期必须拒绝")

	unknown, err := jwtutil.SignRefresh("aud052-secret", "aud052", 999, 0, time.Now())
	require.NoError(t, err)
	_, err = svc.Refresh(ctx, &RefreshRequest{RefreshToken: unknown})
	assert.Error(t, err, "无落库行的刷新令牌必须拒绝")
}

// Store unavailable: opt-in login falls back to the legacy single token (no
// persistent state, no refresh issued).
func TestAud052_StoreUnavailableFallsBackToLegacy(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, nil, "aud052-secret") // no WithRecoveryDB
	admin := &model.Admin{Username: "aud052f", Nickname: "f", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "Str0ngPass!x"))

	resp, err := svc.Login(context.Background(), &LoginRequest{Username: "aud052f", Password: "Str0ngPass!x", UseRefresh: true})
	require.NoError(t, err, "存储不可用不得阻断登录")
	assert.Empty(t, resp.RefreshToken, "无持久状态不得签发刷新令牌")
	claims, err := jwtutil.Parse(resp.Token, "aud052-secret")
	require.NoError(t, err)
	assert.Empty(t, claims.TokenType, "回退路径发 legacy token")
}
