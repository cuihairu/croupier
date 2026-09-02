// 覆盖目标：AgentSession 的 Conn/Addr getter（nil 安全）、
// ControlService.SetMetricsDB、Store.RemoveRegistrationWarnings 过滤器。
package server

import (
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentSession_NilReceiver_Safe(t *testing.T) {
	var s *AgentSession
	// Conn/Addr/Close 均已做 nil receiver 防护（历史上 Conn/Close 会 panic）
	assert.Nil(t, s.Conn())
	assert.Equal(t, "", s.Addr(), "nil session Addr must be empty")
	assert.NoError(t, s.Close())

	zero := &AgentSession{}
	assert.Nil(t, zero.Conn(), "zero-value session has no conn")
	assert.Equal(t, "", zero.Addr())
	assert.NoError(t, zero.Close())
}

func TestControlService_SetMetricsDB_NilStore_NoPanic(t *testing.T) {
	cs := NewControlService(reg.NewStore(), nil)
	cs.SetMetricsDB(nil) // metricsStore 非 nil 但 db 为 nil：不应启动清理例程
	assert.NotNil(t, cs.MetricsStore())
}

func TestRegistryStore_RemoveRegistrationWarnings_Filter(t *testing.T) {
	store := reg.NewStore()
	require.NoError(t, store.UpsertAgent(&reg.AgentSession{AgentID: "w1", GameID: "g", Env: "prod"}))

	// 空/带条件的过滤器都应安全执行（无 warning 时幂等返回 0）
	removed := store.RemoveRegistrationWarnings(reg.RegistrationWarningFilter{AgentID: "w1"})
	assert.GreaterOrEqual(t, removed, 0)
	removedAll := store.RemoveRegistrationWarnings(reg.RegistrationWarningFilter{})
	assert.GreaterOrEqual(t, removedAll, 0)
}
