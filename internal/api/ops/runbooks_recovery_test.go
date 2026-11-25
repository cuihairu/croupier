package ops

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// AUD-024: the runbook terminal-state background recovery loop must be a single
// instance shared across the service lifetime. Previously each
// runbookOrchestrator call constructed a fresh orchestrator, each with its own
// recovery loop — once a request triggered enqueue, each left behind a
// background goroutine that never exits. This test asserts that two consecutive
// orchestrator constructions on the same Service reuse the same shared loop
// (same pointer).
func TestService_RunbookOrchestratorSharesRecoveryLoop(t *testing.T) {
	s, _ := newRunbookTestService(t)

	_, err := s.runbookOrchestrator()
	require.NoError(t, err)
	first := s.runbookRecovery
	require.NotNil(t, first, "首次构造必须创建共享恢复循环")

	_, err = s.runbookOrchestrator()
	require.NoError(t, err)
	require.Same(t, first, s.runbookRecovery, "同一服务生命周期内恢复循环必须共享单实例，不得每请求新建")
}

// AUD-024: on service shutdown (ServiceContext.Close) the registered closer must
// be invoked to release the recovery loop. This test goes through the real
// registration chain — the first runbookOrchestrator call registers Stop as a
// closer, and Close invokes it (no panic, no blocking).
func TestService_CloseStopsRunbookRecoveryLoop(t *testing.T) {
	s, svcCtx := newRunbookTestService(t)

	_, err := s.runbookOrchestrator()
	require.NoError(t, err)
	require.NotNil(t, s.runbookRecovery)

	require.NotPanics(t, func() {
		svcCtx.Close() // triggers the recovery.Stop registered via RegisterCloser
	})

	// constructing the orchestrator again after Close still works (the shared loop
	// is stopped; enqueue exits immediately without blocking).
	_, err = s.runbookOrchestrator()
	require.NoError(t, err)
}
