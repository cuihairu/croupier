package jwtutil

import (
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
	jwt.RegisteredClaims
}

const tokenTTL = 24 * time.Hour

// Sign issues a JWT for the provided user/roles using the shared secret.
// tokenVersion 应取签发时刻 admins.token_version 的当前值。
func Sign(secret string, username string, roles []string, adminID uint, tokenVersion int, issuedAt time.Time) (string, error) {
	if secret == "" {
		return "", errors.New("jwt secret is empty")
	}
	if issuedAt.IsZero() {
		issuedAt = time.Now().UTC()
	}
	claims := Claims{
		Username:     username,
		Roles:        roles,
		AdminID:      adminID,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(issuedAt.Add(tokenTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// Parse validates and parses a JWT returning our structured claims.
func Parse(tokenStr, secret string) (*Claims, error) {
	if secret == "" {
		return nil, errors.New("jwt secret is empty")
	}

	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
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
