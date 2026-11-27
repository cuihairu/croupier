package functioninvoke

// GAME-010 slice 4：取消确认生产 invoker 的派发面单测（经 Dispatcher，与
// 收敛循环同一传输面）。凭据状态语义在 gameops 包测过，这里只验传输行为。

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/gameops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDispatcherCancelInvoker_ReturnsGameEnvelope(t *testing.T) {
	converger, svcCtx := setupConverger(t, &perFuncCaller{payloads: map[string][]byte{
		"demo.cancel": []byte(`{"operationId":"` + convergeOpID + `","outcome":"canceled","gameReceipt":"cancel-ok"}`),
	}})
	_ = converger
	registerDispatchAgent(t, svcCtx.RegistryStore, "a1", "demo.query", "demo.durable", "demo.cancel")
	invoker := NewDispatcherCancelInvoker(svcCtx)
	payload, err := invoker.InvokeCancel(context.Background(), "demo.cancel", []byte(`{"operationId":"x"}`), "test-game", "test-env", "alice")
	require.NoError(t, err)
	assert.JSONEq(t, `{"operationId":"`+convergeOpID+`","outcome":"canceled","gameReceipt":"cancel-ok"}`, string(payload))
}

// 传输失败原样上抛：ConfirmCancellation 据此保持凭据原状（不发明取消结果）。
func TestDispatcherCancelInvoker_TransportErrorPropagates(t *testing.T) {
	_, svcCtx := setupConverger(t, &perFuncCaller{err: context.DeadlineExceeded})
	invoker := NewDispatcherCancelInvoker(svcCtx)
	_, err := invoker.InvokeCancel(context.Background(), "demo.cancel", []byte(`{}`), "test-game", "test-env", "alice")
	require.Error(t, err)
}

// Dispatcher 未装配 → 确认通道不可用（≠ 取消成功）。
func TestDispatcherCancelInvoker_NilDispatcherUnavailable(t *testing.T) {
	invoker := NewDispatcherCancelInvoker(nil)
	_, err := invoker.InvokeCancel(context.Background(), "demo.cancel", []byte(`{}`), "g", "e", "alice")
	require.ErrorIs(t, err, gameops.ErrCancelChannelUnavailable)
}

// 空响应体不可采信：信封缺失即失败，不当作 canceled。
func TestDispatcherCancelInvoker_EmptyResponseRejected(t *testing.T) {
	_, svcCtx := setupConverger(t, &perFuncCaller{payloads: map[string][]byte{}})
	invoker := NewDispatcherCancelInvoker(svcCtx)
	_, err := invoker.InvokeCancel(context.Background(), "demo.cancel", []byte(`{}`), "test-game", "test-env", "alice")
	require.Error(t, err)
}
