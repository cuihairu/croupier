// ARCH-016 registry 侧 drain 路由门：SetAgentDraining 置/清；未在册返回
// false；UpsertAgent 会话刷新保留 drain（不因注册/心跳静默清除）。
package registry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetAgentDraining(t *testing.T) {
	s := NewStore()
	require.NoError(t, s.UpsertAgent(&AgentSession{
		AgentID:   "a-drain",
		GameID:    "g1",
		Env:       "prod",
		ExpireAt:  time.Now().Add(time.Minute),
		Functions: map[string]FunctionMeta{"fn.x": {Enabled: true}},
	}))

	// 置位：在册 → true 且状态落库。
	assert.True(t, s.SetAgentDraining("a-drain", true))
	s.mu.RLock()
	assert.True(t, s.agents["a-drain"].Draining)
	s.mu.RUnlock()

	// 清除。
	assert.True(t, s.SetAgentDraining("a-drain", false))
	s.mu.RLock()
	assert.False(t, s.agents["a-drain"].Draining)
	s.mu.RUnlock()

	// 未在册 → false，无副作用。
	assert.False(t, s.SetAgentDraining("ghost", true))
}

func TestUpsertAgentCarriesDrainingOver(t *testing.T) {
	s := NewStore()
	require.NoError(t, s.UpsertAgent(&AgentSession{
		AgentID:   "a-carry",
		GameID:    "g1",
		Env:       "prod",
		ExpireAt:  time.Now().Add(time.Minute),
		Functions: map[string]FunctionMeta{"fn.x": {Enabled: true}},
	}))
	require.True(t, s.SetAgentDraining("a-carry", true))

	// 会话刷新（心跳/重连：同 AgentID 再 Upsert，函数集不变）。
	require.NoError(t, s.UpsertAgent(&AgentSession{
		AgentID:   "a-carry",
		GameID:    "g1",
		Env:       "prod",
		ExpireAt:  time.Now().Add(time.Minute),
		Functions: map[string]FunctionMeta{"fn.x": {Enabled: true}},
	}))

	s.mu.RLock()
	defer s.mu.RUnlock()
	assert.True(t, s.agents["a-carry"].Draining, "会话刷新不得清除 drain")
}
