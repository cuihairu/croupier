package jwtutil

// AUD-052 刷新令轮换的令牌型别测试：
// - SignAccess / SignRefresh 的 TTL 与 typ 声明
// - Parse（API 访问路径的统一入口）拒绝 typ=refresh、放行 legacy（无 typ）
//   与 typ=access
// - ParseRefresh（兑换端点专用）反向拒绝 access/legacy 令牌
// 权限面双向收口：刷新令牌换不了 API 访问，访问令牌也进不了兑换端点。

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignAccessClaimsAndTTL(t *testing.T) {
	secret := "aud052-access-secret"
	issuedAt := time.Now().UTC().Add(-time.Minute)

	token, err := SignAccess(secret, "alice", []string{"admin"}, 42, 3, issuedAt)
	require.NoError(t, err)
	claims, err := Parse(token, secret)
	require.NoError(t, err)
	assert.Equal(t, "alice", claims.Username)
	assert.Equal(t, []string{"admin"}, claims.Roles)
	assert.Equal(t, uint(42), claims.AdminID)
	assert.Equal(t, 3, claims.TokenVersion)
	assert.Equal(t, TokenTypeAccess, claims.TokenType)
	// TTL ≈ AccessTokenTTL（2h）：允许秒级时钟量化误差。
	span := claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time)
	assert.Equal(t, AccessTokenTTL, span)
}

func TestSignRefreshClaimsAndTTL(t *testing.T) {
	secret := "aud052-refresh-secret"
	issuedAt := time.Now().UTC().Add(-time.Minute)

	token, err := SignRefresh(secret, "alice", 42, 3, issuedAt)
	require.NoError(t, err)
	claims, err := ParseRefresh(token, secret)
	require.NoError(t, err)
	assert.Equal(t, "alice", claims.Username)
	assert.Equal(t, uint(42), claims.AdminID)
	assert.Equal(t, TokenTypeRefresh, claims.TokenType)
	// 刷新令牌不携带 roles：兑换时从 admins 行重载最新角色。
	assert.Empty(t, claims.Roles)
	span := claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time)
	assert.Equal(t, RefreshTokenTTL, span)
}

// 访问路径拒绝刷新令牌：中间件/认证路径全部经 Parse，typ=refresh 一律
// 拒绝（AUD-052 权限面收敛的落点）。
func TestParseRejectsRefreshToken(t *testing.T) {
	secret := "aud052-cross-secret"
	refresh, err := SignRefresh(secret, "alice", 42, 0, time.Now())
	require.NoError(t, err)

	_, err = Parse(refresh, secret)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRefreshTokenRejected)
}

// legacy（Sign，无 typ）与 typ=access 的令牌在访问路径均放行：存量客户端
// 与新 access 各自兼容。
func TestParseAcceptsLegacyAndAccessTokens(t *testing.T) {
	secret := "aud052-legacy-secret"

	legacy, err := Sign(secret, "alice", []string{"admin"}, 42, 0, time.Now())
	require.NoError(t, err)
	claims, err := Parse(legacy, secret)
	require.NoError(t, err)
	assert.Empty(t, claims.TokenType)

	access, err := SignAccess(secret, "alice", nil, 42, 0, time.Now())
	require.NoError(t, err)
	_, err = Parse(access, secret)
	require.NoError(t, err)
}

// 兑换端点反向拒绝：access 与 legacy 令牌不能当刷新令牌用。
func TestParseRefreshRejectsAccessAndLegacyTokens(t *testing.T) {
	secret := "aud052-reverse-secret"

	access, err := SignAccess(secret, "alice", nil, 42, 0, time.Now())
	require.NoError(t, err)
	_, err = ParseRefresh(access, secret)
	assert.Error(t, err, "access token must not be exchangeable as refresh token")

	legacy, err := Sign(secret, "alice", nil, 42, 0, time.Now())
	require.NoError(t, err)
	_, err = ParseRefresh(legacy, secret)
	assert.Error(t, err, "legacy token must not be exchangeable as refresh token")
}

// 同一秒内两次签发（登录 + 立即兑换）不得产出相同令牌：NumericDate 只有
// 秒级精度，jti 随机数保证字节级唯一（哈希唯一索引 + 一次性语义的前提）。
func TestSignRefreshUniqueWithinSameSecond(t *testing.T) {
	secret := "aud052-jti-secret"
	issuedAt := time.Now().UTC()

	a, err := SignRefresh(secret, "alice", 42, 0, issuedAt)
	require.NoError(t, err)
	b, err := SignRefresh(secret, "alice", 42, 0, issuedAt)
	require.NoError(t, err)
	assert.NotEqual(t, a, b, "同秒签发的两枚刷新令牌必须字节级不同")

	claims, err := ParseRefresh(a, secret)
	require.NoError(t, err)
	assert.NotEmpty(t, claims.ID, "刷新令牌必须带 jti")
}
