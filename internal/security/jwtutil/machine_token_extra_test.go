package jwtutil

// machine_token.go 补测：机器身份上下文键的存取两臂、SignMachine 的
// issuedAt 零值兜底臂、ParseMachine 的空 secret / 非 HMAC 签名方法 /
// invalid 标记（缝隙注入，golang-jwt/v4 校验通过恒置 Valid=true）/
// 缺 agentId 各拒绝臂。
//
// 诚实跳过：无。

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMachineIdentityContextRoundTrip(t *testing.T) {
	// 无机器身份 → false
	_, ok := MachineIdentityFrom(context.Background())
	assert.False(t, ok)

	claims := &MachineClaims{TokenType: MachineTokenType, AgentID: "agent-ctx"}
	ctx := WithMachineIdentity(context.Background(), claims)
	got, ok := MachineIdentityFrom(ctx)
	require.True(t, ok)
	require.Same(t, claims, got)
}

// issuedAt 零值 → 取当前 UTC；ttl<=0 → 默认 30 天。
func TestSignMachineZeroIssuedAtFallsBackToNow(t *testing.T) {
	before := time.Now().UTC().Add(-time.Second)
	token, err := SignMachine("secret", "agent-1", "game", "prod", 0, time.Time{})
	require.NoError(t, err)

	claims, err := ParseMachine(token, "secret")
	require.NoError(t, err)
	require.NotNil(t, claims.IssuedAt)
	require.False(t, claims.IssuedAt.Time.Before(before))
	require.True(t, claims.IssuedAt.Time.Equal(claims.ExpiresAt.Add(-30*24*time.Hour)))
}

func TestParseMachineRejectionArms(t *testing.T) {
	// 空 secret
	_, err := ParseMachine("token", "")
	require.EqualError(t, err, "jwt secret is empty")

	// 非 HMAC 签名方法（alg=none）
	noneToken := jwt.NewWithClaims(jwt.SigningMethodNone, &MachineClaims{
		TokenType: MachineTokenType, AgentID: "agent-1",
	})
	noneStr, err := noneToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)
	_, err = ParseMachine(noneStr, "secret")
	require.ErrorContains(t, err, "unexpected signing method")

	// tokenType 是 machine 但缺 agentId（SignMachine 拒绝空 agentID，
	// 只能手工构造声明）
	noAgent := jwt.NewWithClaims(jwt.SigningMethodHS256, &MachineClaims{
		TokenType: MachineTokenType,
	})
	noAgentStr, err := noAgent.SignedString([]byte("secret"))
	require.NoError(t, err)
	_, err = ParseMachine(noAgentStr, "secret")
	require.EqualError(t, err, "machine token missing agent id")
}

// golang-jwt/v4 校验通过（err == nil）时恒置 token.Valid = true，
// ParseMachine 的 "!parsed.Valid" 分支只能通过缝隙注入触达（与 Parse 同款）。
func TestParseMachine_InvalidTokenFlagWithoutError(t *testing.T) {
	orig := parseWithClaims
	parseWithClaims = func(tokenString string, claims jwt.Claims, keyFunc jwt.Keyfunc, options ...jwt.ParserOption) (*jwt.Token, error) {
		return &jwt.Token{Raw: tokenString, Claims: claims, Valid: false}, nil
	}
	t.Cleanup(func() { parseWithClaims = orig })

	_, err := ParseMachine("header.payload.signature", "secret")
	require.Error(t, err, "invalid token flag must surface as error")
	assert.Equal(t, "invalid token", err.Error())
}
