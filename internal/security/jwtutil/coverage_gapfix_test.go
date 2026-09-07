package jwtutil

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// golang-jwt/v4 校验通过（err == nil）时恒置 token.Valid = true，
// Parse 的 "!parsed.Valid" 分支只能通过缝隙注入 (nil-error, invalid) 组合触达。
func TestParse_InvalidTokenFlagWithoutError(t *testing.T) {
	orig := parseWithClaims
	parseWithClaims = func(tokenString string, claims jwt.Claims, keyFunc jwt.Keyfunc, options ...jwt.ParserOption) (*jwt.Token, error) {
		return &jwt.Token{Raw: tokenString, Claims: claims, Valid: false}, nil
	}
	t.Cleanup(func() { parseWithClaims = orig })

	claims, err := Parse("header.payload.signature", "secret")
	require.Error(t, err, "invalid token flag must surface as error")
	assert.Nil(t, claims)
	assert.Equal(t, "invalid token", err.Error())
}

// 缝隙还原后 Parse 必须恢复真实签名/校验行为。
func TestParse_SeamRestoredAfterInjection(t *testing.T) {
	orig := parseWithClaims
	parseWithClaims = func(tokenString string, claims jwt.Claims, keyFunc jwt.Keyfunc, options ...jwt.ParserOption) (*jwt.Token, error) {
		return &jwt.Token{Raw: tokenString, Claims: claims, Valid: false}, nil
	}
	t.Cleanup(func() { parseWithClaims = orig })

	if _, err := Parse("whatever", "secret"); err == nil {
		t.Fatal("injected seam should fail parse")
	}

	parseWithClaims = orig

	token, err := Sign("secret", "u", []string{"admin"}, 1, 1, time.Now())
	require.NoError(t, err)
	claims, err := Parse(token, "secret")
	require.NoError(t, err)
	assert.Equal(t, "u", claims.Username)
}

// 真实路径健全性：错误 token / 错误密钥仍由 jwt 库报错（非缝隙路径）。
func TestParse_RealLibraryErrorsUntouched(t *testing.T) {
	_, err := Parse("not-a-jwt", "secret")
	require.Error(t, err)

	token, err := Sign("secret-a", "u", []string{"admin"}, 1, 0, time.Now())
	require.NoError(t, err)
	_, err = Parse(token, "secret-b")
	require.Error(t, err, "wrong secret must fail signature verification")
}
