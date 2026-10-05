package agent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Serve 在 accept 超时窗口内收到 ctx 取消时应返回 ctx.Err()。
func TestTCPLocalListener_ServeReturnsContextErrorAfterCancelDuringAcceptTimeout(t *testing.T) {
	listener, err := NewTCPLocalListener(&TCPLocalListenerConfig{Address: "127.0.0.1:0"}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- listener.Serve(ctx) }()

	// accept deadline 为 1s：在 1s~2s 窗口内取消，下一轮超时即命中 ctx.Done。
	time.Sleep(1100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after context cancellation")
	}
}

// config 为 nil 且环境变量为空时应回落到默认本地地址。只验地址解析、不实际
// 绑定——构造函数会立即 net.Listen，本机跑着部署的 agent（占用 19091）时
// 绑定必失败，与被测的默认值逻辑无关。
func TestTCPLocalListener_NilConfigDefaultsAddressWhenEnvEmpty(t *testing.T) {
	t.Setenv("CROUPIER_AGENT_LOCAL_ADDR", "")
	assert.Equal(t, "127.0.0.1:19091", resolveLocalListenerAddress(nil).Address)

	t.Setenv("CROUPIER_AGENT_LOCAL_ADDR", "127.0.0.1:17777")
	assert.Equal(t, "127.0.0.1:17777", resolveLocalListenerAddress(nil).Address)

	assert.Equal(t, "127.0.0.1:18888",
		resolveLocalListenerAddress(&TCPLocalListenerConfig{Address: "127.0.0.1:18888"}).Address)
}
