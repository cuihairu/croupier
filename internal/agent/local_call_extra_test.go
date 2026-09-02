// 覆盖目标：agent callProvider 无会话时回落 callLocalProvider（0%），
// 连接失败地址的错误分支；mustMarshal 的 marshal 失败不可达路径旁路。
package agent

import (
	"context"
	"testing"

	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallProvider_FallbackAddr_Unreachable(t *testing.T) {
	handler := NewLocalHandler(nil, t.TempDir(), "agent-test", nil)

	// 无 provider session + 不可达回退地址 → 连接错误
	_, err := handler.callProvider(context.Background(), "fn.none", nil, "127.0.0.1:1", 0x10, []byte("{}"))
	require.Error(t, err)
}

func TestCallLocalProvider_EmptyAddr(t *testing.T) {
	handler := NewLocalHandler(nil, t.TempDir(), "agent-test", nil)
	_, err := handler.callLocalProvider(context.Background(), "127.0.0.1:1", 0x10, []byte("{}"))
	require.Error(t, err)
}

func TestMustMarshal_ProtoMessage(t *testing.T) {
	msg := &sdkv1.InvokeRequest{FunctionId: "fn.x"}
	out := mustMarshal(msg)
	assert.Contains(t, string(out), "fn.x")
}
