package ops

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// AUD-024：runbook 终态后台恢复循环须按服务生命周期共享单实例。此前每请求
// runbookOrchestrator 都 New 一个临时编排器，各自自带恢复循环——请求一旦触发
// enqueue 就各自遗留一个永不退出的后台 goroutine。本用例断言同一 Service 连续
// 两次构造编排器复用同一共享循环（指针同源）。
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

// AUD-024：服务退出（ServiceContext.Close）必须调用注册的 closer 释放恢复循环。
// 本用例走真实注册链路——runbookOrchestrator 首次调用把 Stop 注册为 closer，
// Close 调用它（不 panic、不阻塞）。
func TestService_CloseStopsRunbookRecoveryLoop(t *testing.T) {
	s, svcCtx := newRunbookTestService(t)

	_, err := s.runbookOrchestrator()
	require.NoError(t, err)
	require.NotNil(t, s.runbookRecovery)

	require.NotPanics(t, func() {
		svcCtx.Close() // 触发 RegisterCloser 注册的 recovery.Stop
	})

	// 关闭后再次构造编排器仍可用（共享循环已 Stop，enqueue 会立即退出，不阻塞）。
	_, err = s.runbookOrchestrator()
	require.NoError(t, err)
}
