package jwtutil

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// Claims represents the JWT claims we issue/parse.
type Claims struct {
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	AdminID  uint     `json:"adminId"`
	// TokenVersion 与 admins.token_version 对应；中间件比对不一致即拒绝，
	// 使改密码/禁用/登出后旧 token 立即失效。旧 token（无此字段）解析为
	// 0，与库中初始版本 0 一致，平滑兼容。
	TokenVersion int `json:"tokenVersion"`
	// TokenType 声明令牌用途（AUD-052）："" = 既有 access（存量 token 无
	// 此字段，解析为空、行为不变——灰度兼容），"access" = 短寿命 access，
	// "refresh" = 刷新令牌。刷新令牌不得用作 API 访问凭据，Parse 拒绝。
	TokenType string `json:"typ,omitempty"`
	jwt.RegisteredClaims
}

// tokenTTL 是 legacy 签发（Sign）的 24h 访问令牌：未启用刷新令牌的既有
// 客户端（移动端 Flutter、旧 web 会话）行为与 AUD-052 之前完全一致。
const tokenTTL = 24 * time.Hour

// AccessTokenTTL 是启用刷新令牌的会话的访问令牌有效期：短寿命 + 刷新
// 轮换，泄露窗口从 24h 收窄到 2h。
const AccessTokenTTL = 2 * time.Hour

// RefreshTokenTTL 是刷新令牌的绝对有效期；兑换签发的新令牌获得自己的
// 7 天窗口（滚动重置），不做滑动续期——长期不活跃的会话自然过期。
const RefreshTokenTTL = 7 * 24 * time.Hour

const (
	// TokenTypeAccess 写入新签发的 access token；解析侧空串与 "access"
	// 等价（存量 legacy token 无 typ 字段）。
	TokenTypeAccess = "access"
	// TokenTypeRefresh 标记刷新令牌；只允许出现在 /auth/refresh 兑换路径。
	TokenTypeRefresh = "refresh"
)

// ErrRefreshTokenRejected 表示刷新令牌被用在 API 访问路径（Authorization
// Bearer）。刷新令牌权限面只有 /auth/refresh，中间件统一经 Parse 拒绝。
var ErrRefreshTokenRejected = errors.New("refresh token cannot be used as access token")

// parseWithClaims 是 JWT 解析的可注入缝隙（生产实现为 jwt.ParseWithClaims）。
// golang-jwt/v4 在校验通过（err == nil）时恒置 token.Valid = true，
// Parse 中 "!parsed.Valid 且 err == nil" 分支仅能通过测试注入触达。
var parseWithClaims = jwt.ParseWithClaims

// Sign issues a JWT for the provided user/roles using the shared secret.
// tokenVersion 应取签发时刻 admins.token_version 的当前值。
//
// AUD-052 起 Sign 是 legacy 签发路径：24h 单 token、无 typ 声明，未启用
// 刷新令牌的既有客户端（登录请求未带 useRefresh）行为完全不变。
func Sign(secret string, username string, roles []string, adminID uint, tokenVersion int, issuedAt time.Time) (string, error) {
	return signClaims(secret, Claims{
		Username:     username,
		Roles:        roles,
		AdminID:      adminID,
		TokenVersion: tokenVersion,
	}, tokenTTL, issuedAt)
}

// SignAccess issues a short-lived access token (typ=access, 2h) for sessions
// that opted into refresh rotation (AUD-052)。
func SignAccess(secret string, username string, roles []string, adminID uint, tokenVersion int, issuedAt time.Time) (string, error) {
	return signClaims(secret, Claims{
		Username:     username,
		Roles:        roles,
		AdminID:      adminID,
		TokenVersion: tokenVersion,
		TokenType:    TokenTypeAccess,
	}, AccessTokenTTL, issuedAt)
}

// SignRefresh issues a refresh token (typ=refresh, 7d, AUD-052)。刷新令牌
// 不携带 roles：兑换时从 admins 行重新加载最新角色，避免陈旧授权滚动续期。
// jti 写入 16 字节随机数：NumericDate 只有秒级精度，同账号同一秒内的两次
// 签发（登录 + 立即兑换）若无 jti 会产出字节级相同的令牌——哈希唯一索引
// 冲突，且两枚「不同」令牌实际是同一份，一次性语义失真。
func SignRefresh(secret string, username string, adminID uint, tokenVersion int, issuedAt time.Time) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate refresh jti: %w", err)
	}
	return signClaims(secret, Claims{
		Username:     username,
		AdminID:      adminID,
		TokenVersion: tokenVersion,
		TokenType:    TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			ID: hex.EncodeToString(buf),
		},
	}, RefreshTokenTTL, issuedAt)
}

// signClaims 统一签发：同一 HMAC secret、同一 Claims 结构，差别只在 TTL
// 与 typ；claims 里预置的 RegisteredClaims 字段（如刷新令牌的 jti）保留。
func signClaims(secret string, claims Claims, ttl time.Duration, issuedAt time.Time) (string, error) {
	if secret == "" {
		return "", errors.New("jwt secret is empty")
	}
	if issuedAt.IsZero() {
		issuedAt = time.Now().UTC()
	}
	if claims.Subject == "" {
		claims.Subject = claims.Username
	}
	claims.IssuedAt = jwt.NewNumericDate(issuedAt)
	claims.ExpiresAt = jwt.NewNumericDate(issuedAt.Add(ttl))
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// Parse validates and parses a JWT returning our structured claims.
// 刷新令牌在此被拒绝：所有认证中间件与 WebSocket 升级路径都经 Parse，
// typ=refresh 的令牌不能当访问凭据（AUD-052 权限面收敛）。
func Parse(tokenStr, secret string) (*Claims, error) {
	claims, err := parseRaw(tokenStr, secret)
	if err != nil {
		return nil, err
	}
	if claims.TokenType == TokenTypeRefresh {
		return nil, ErrRefreshTokenRejected
	}
	return claims, nil
}

// ParseRefresh parses a refresh token, rejecting anything that is not one
// （access/legacy token 送进兑换端点同样拒绝，双方向都收口）。
func ParseRefresh(tokenStr, secret string) (*Claims, error) {
	claims, err := parseRaw(tokenStr, secret)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeRefresh {
		return nil, errors.New("token is not a refresh token")
	}
	return claims, nil
}

func parseRaw(tokenStr, secret string) (*Claims, error) {
	if secret == "" {
		return nil, errors.New("jwt secret is empty")
	}

	claims := &Claims{}
	parsed, err := parseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %T", token.Method)
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
