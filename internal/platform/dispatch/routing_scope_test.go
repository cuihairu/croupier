package dispatch

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
)

// 路由 scope 谓词：game/env 必须成对、跨 scope agent 不得被选中、过期 agent 拒绝。

func TestRoutingScopeFromMetadata(t *testing.T) {
	// 空 metadata / 双空 → 不 scoped。
	gameID, env, scoped, err := routingScopeFromMetadata(nil)
	require.NoError(t, err)
	assert.False(t, scoped)
	assert.Empty(t, gameID)

	gameID, env, scoped, err = routingScopeFromMetadata(map[string]string{"other": "x"})
	require.NoError(t, err)
	assert.False(t, scoped)

	// 只给一半 → 错误（成对约束）。
	_, _, scoped, err = routingScopeFromMetadata(map[string]string{"gameId": "g1"})
	require.Error(t, err)
	assert.False(t, scoped)
	assert.Contains(t, err.Error(), "together")

	_, _, scoped, err = routingScopeFromMetadata(map[string]string{"env": "prod"})
	require.Error(t, err)
	assert.False(t, scoped)

	// 成对 + trim → scoped。
	gameID, env, scoped, err = routingScopeFromMetadata(map[string]string{
		"gameId": "  g1 ",
		"env":    " prod\t",
	})
	require.NoError(t, err)
	assert.True(t, scoped)
	assert.Equal(t, "g1", gameID)
	assert.Equal(t, "prod", env)
}

func TestAgentCanInvoke_Matrix(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	live := now.Add(time.Minute)
	dead := now.Add(-time.Second)

	base := func() *reg.AgentSession {
		return &reg.AgentSession{
			AgentID:  "a1",
			GameID:   "g1",
			Env:      "prod",
			ExpireAt: live,
			Functions: map[string]reg.FunctionMeta{
				"player.ban": {Enabled: true},
				"player.off": {Enabled: false},
			},
		}
	}

	// nil / 过期 → 拒绝。
	assert.False(t, agentCanInvoke(nil, "player.ban", now, "g1", "prod", true))
	expired := base()
	expired.ExpireAt = dead
	assert.False(t, agentCanInvoke(expired, "player.ban", now, "g1", "prod", true))

	// scoped 跨 game / 跨 env → 拒绝。
	assert.False(t, agentCanInvoke(base(), "player.ban", now, "g2", "prod", true))
	assert.False(t, agentCanInvoke(base(), "player.ban", now, "g1", "dev", true))
	// agent 侧 GameID 带空白：实现对 agent.GameID 做 TrimSpace 后比较 → 匹配放行。
	trimmed := base()
	trimmed.GameID = " g1 "
	assert.True(t, agentCanInvoke(trimmed, "player.ban", now, "g1", "prod", true))
	// trim 后仍不同 → 拒绝。
	trimmed.GameID = "g1x"
	assert.False(t, agentCanInvoke(trimmed, "player.ban", now, "g1", "prod", true))

	// scoped 匹配 → 通过。
	assert.True(t, agentCanInvoke(base(), "player.ban", now, "g1", "prod", true))

	// 非 scoped（全局路由）忽略 game/env。
	assert.True(t, agentCanInvoke(base(), "player.ban", now, "g2", "dev", false))

	// 函数不存在 / 未启用 → 拒绝。
	assert.False(t, agentCanInvoke(base(), "ghost.fn", now, "g1", "prod", true))
	assert.False(t, agentCanInvoke(base(), "player.off", now, "g1", "prod", true))
}

func TestFormatRoutingScope(t *testing.T) {
	assert.Empty(t, formatRoutingScope("g1", "prod", false))
	assert.Equal(t, " in game_id g1 env prod", formatRoutingScope("g1", "prod", true))
}
