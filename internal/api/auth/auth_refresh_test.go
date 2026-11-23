package auth

// AUD-052 刷新令牌轮换测试：
// - legacy 登录（缺省请求）行为不变：单 24h token、无 refreshToken 字段
// - opt-in 登录（useRefresh=true）签发 access+refresh 对并落轮换状态
// - 兑换轮换：旧令牌标记已兑换、新令牌挂兑换链
// - 重放检测：已兑换令牌再现 → 该账号全部刷新令牌吊销 + 审计
// - 双向拒绝：access 进不了兑换端点；版本 bump / 过期 / 未知令牌一律 401
// - 存储不可用回退 legacy（登录不因轮换基础设施失败而阻断）
// 断言一律走子集过滤（InMemoryAuditStore 底层 map 遍历随机，AUD-049 教训）。

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

// aud052Env 组装带刷新令牌存储与审计的业务库（routes 生产装配同构：
// WithRecoveryDB(db) → refreshTokenModel 懒建）。
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

// legacy 登录（缺省请求）完全不变：无 refreshToken、token 无 typ。
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

// opt-in 登录签发 access+refresh 对，落库哈希行（未兑换、非轮换）。
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

// 兑换轮换：返回新对；旧令牌标记已兑换；新令牌挂兑换链；新 access 可用。
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

// 重放检测：已兑换的旧令牌再次出现 → 该账号全部刷新令牌吊销 + 审计。
func TestAud052_RefreshReplayRevokesFamily(t *testing.T) {
	svc, adminModel, _, store, db := aud052Env(t)
	aud052Admin(t, adminModel)
	ctx := context.Background()

	login, err := svc.Login(ctx, &LoginRequest{Username: "aud052", Password: "Str0ngPass!x", UseRefresh: true})
	require.NoError(t, err)
	first, err := svc.Refresh(ctx, &RefreshRequest{RefreshToken: login.RefreshToken})
	require.NoError(t, err)

	// 重放旧令牌（攻击者手里的第一环）。
	_, err = svc.Refresh(ctx, &RefreshRequest{RefreshToken: login.RefreshToken})
	require.Error(t, err)

	// 整族吊销：攻击者经正常轮换持有的最新令牌也一并作废（fail-closed）。
	_, err = svc.Refresh(ctx, &RefreshRequest{RefreshToken: first.RefreshToken})
	require.Error(t, err, "重放吊销后同族新令牌不得再兑换")

	var revoked int64
	require.NoError(t, db.Model(&model.AdminRefreshToken{}).
		Where("admin_id = ? AND revoked = ?", 1, true).Count(&revoked).Error)
	assert.EqualValues(t, 2, revoked, "登录直发 + 轮换两行都应被吊销")

	// 两次失败兑换各留一条重放审计（已兑换旧令牌 + 已吊销新令牌各一）。
	reuseRecords := aud052Records(t, store, audit.EventTokenRefreshReuse)
	require.Len(t, reuseRecords, 2, "重放必须留审计痕")
	for _, r := range reuseRecords {
		assert.Equal(t, "failure", r.Outcome)
	}
}

// 双向拒绝：access 令牌进不了兑换端点（ParseRefresh 拒绝非 refresh 型别）。
func TestAud052_RefreshRejectsAccessToken(t *testing.T) {
	svc, adminModel, _, _, _ := aud052Env(t)
	aud052Admin(t, adminModel)

	login, err := svc.Login(context.Background(), &LoginRequest{Username: "aud052", Password: "Str0ngPass!x", UseRefresh: true})
	require.NoError(t, err)
	_, err = svc.Refresh(context.Background(), &RefreshRequest{RefreshToken: login.Token})
	assert.Error(t, err)
}

// token_version bump（改密/禁用/登出）后旧刷新令牌族立即失效。
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

// 落库行过期（JWT 本体未过期）拒绝兑换；未知哈希（行被删/异库）同样拒绝。
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

// 存储不可用：opt-in 登录回退 legacy 单 token（无持久状态不发 refresh）。
func TestAud052_StoreUnavailableFallsBackToLegacy(t *testing.T) {
	db := setupTestDB(t)
	adminModel := model.NewAdminModel(db)
	svc := NewService(adminModel, nil, "aud052-secret") // 无 WithRecoveryDB
	admin := &model.Admin{Username: "aud052f", Nickname: "f", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "Str0ngPass!x"))

	resp, err := svc.Login(context.Background(), &LoginRequest{Username: "aud052f", Password: "Str0ngPass!x", UseRefresh: true})
	require.NoError(t, err, "存储不可用不得阻断登录")
	assert.Empty(t, resp.RefreshToken, "无持久状态不得签发刷新令牌")
	claims, err := jwtutil.Parse(resp.Token, "aud052-secret")
	require.NoError(t, err)
	assert.Empty(t, claims.TokenType, "回退路径发 legacy token")
}
