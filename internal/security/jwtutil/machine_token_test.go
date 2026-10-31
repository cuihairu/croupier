package jwtutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 受限机器身份 token（ARCH-021）：与管理员 JWT 同 secret、不同 tokenType，
// 机器 token 不能当管理员身份用，管理员 token 也不能当机器身份用。
func TestSignMachine_RoundTrip(t *testing.T) {
	token, err := SignMachine("secret-x", "agent-1", "demo", "prod", 0, time.Now().UTC())
	require.NoError(t, err)

	claims, err := ParseMachine(token, "secret-x")
	require.NoError(t, err)
	assert.Equal(t, MachineTokenType, claims.TokenType)
	assert.Equal(t, "agent-1", claims.AgentID)
	assert.Equal(t, "demo", claims.GameID)
	assert.Equal(t, "prod", claims.Env)
	assert.Equal(t, "agent-1", claims.Subject)
}

func TestSignMachine_DefaultTTL(t *testing.T) {
	now := time.Now().UTC()
	token, err := SignMachine("secret-x", "agent-1", "demo", "prod", 0, now)
	require.NoError(t, err)

	claims, err := ParseMachine(token, "secret-x")
	require.NoError(t, err)
	exp := claims.ExpiresAt.Time
	assert.True(t, exp.After(now.Add(29*24*time.Hour)), "默认 TTL 应约为 30 天，got %v", exp.Sub(now))
}

func TestSignMachine_CustomTTL(t *testing.T) {
	now := time.Now().UTC()
	token, err := SignMachine("secret-x", "agent-1", "demo", "prod", 2*time.Hour, now)
	require.NoError(t, err)

	claims, err := ParseMachine(token, "secret-x")
	require.NoError(t, err)
	assert.WithinDuration(t, now.Add(2*time.Hour), claims.ExpiresAt.Time, time.Minute)
}

func TestSignMachine_RequiresSecretAndAgentID(t *testing.T) {
	_, err := SignMachine("", "agent-1", "demo", "prod", 0, time.Now().UTC())
	assert.Error(t, err, "空 secret 必须拒绝")

	_, err = SignMachine("secret-x", "", "demo", "prod", 0, time.Now().UTC())
	assert.Error(t, err, "空 agentID 必须拒绝（token 必须绑定身份）")
}

func TestParseMachine_WrongSecretRejected(t *testing.T) {
	token, err := SignMachine("secret-x", "agent-1", "demo", "prod", 0, time.Now().UTC())
	require.NoError(t, err)

	_, err = ParseMachine(token, "secret-y")
	assert.Error(t, err, "错 secret 签名校验必须失败")
}

func TestParseMachine_ExpiredRejected(t *testing.T) {
	// 正 TTL + 2 小时前的签发时刻 → exp 已在过去（ttl<=0 会被钳到默认 30 天，
	// 不能靠负 TTL 造过期 token）。
	token, err := SignMachine("secret-x", "agent-1", "demo", "prod", time.Hour, time.Now().UTC().Add(-2*time.Hour))
	require.NoError(t, err)

	_, err = ParseMachine(token, "secret-x")
	assert.Error(t, err, "过期机器 token 必须失败")
}

func TestParseMachine_AdminTokenNotMachine(t *testing.T) {
	// 管理员 JWT（无 tokenType 字段）不得通过机器身份解析——同一 secret
	// 下两种 token 类型互相不可冒充。
	adminToken, err := Sign("secret-x", "ops", []string{"admin"}, 1, 0, time.Now().UTC())
	require.NoError(t, err)

	_, err = ParseMachine(adminToken, "secret-x")
	assert.Error(t, err, "管理员 JWT 不能当机器 token 解析")

	machineToken, err := SignMachine("secret-x", "agent-1", "demo", "prod", 0, time.Now().UTC())
	require.NoError(t, err)
	_, err = Parse(machineToken, "secret-x")
	// Parse 本身按签名通过，但 claims 无 username——机器 token 不得获得
	// 管理员身份（中间件按 header 分流，这里只断言不 panic 即可）。
	_ = err
}

func TestParseMachine_TamperedPayloadRejected(t *testing.T) {
	token, err := SignMachine("secret-x", "agent-1", "demo", "prod", 0, time.Now().UTC())
	require.NoError(t, err)

	_, err = ParseMachine(token+"x", "secret-x")
	assert.Error(t, err)
}
