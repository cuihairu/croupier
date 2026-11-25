package agent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Serve should return ctx.Err() when ctx is cancelled during the accept
// timeout window.
func TestTCPLocalListener_ServeReturnsContextErrorAfterCancelDuringAcceptTimeout(t *testing.T) {
	listener, err := NewTCPLocalListener(&TCPLocalListenerConfig{Address: "127.0.0.1:0"}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- listener.Serve(ctx) }()

	// the accept deadline is 1s: cancelling within the 1s~2s window makes the
	// next timeout round hit ctx.Done.
	time.Sleep(1100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after context cancellation")
	}
}

// With a nil config and an empty env var, fall back to the default local
// address. Only address resolution is verified, no actual binding — the
// constructor calls net.Listen immediately, and when a deployed agent is
// running on this machine (holding 19091) the bind would necessarily fail,
// unrelated to the default-value logic under test.
func TestTCPLocalListener_NilConfigDefaultsAddressWhenEnvEmpty(t *testing.T) {
	t.Setenv("CROUPIER_AGENT_LOCAL_ADDR", "")
	assert.Equal(t, "127.0.0.1:19091", resolveLocalListenerAddress(nil).Address)

	t.Setenv("CROUPIER_AGENT_LOCAL_ADDR", "127.0.0.1:17777")
	assert.Equal(t, "127.0.0.1:17777", resolveLocalListenerAddress(nil).Address)

	assert.Equal(t, "127.0.0.1:18888",
		resolveLocalListenerAddress(&TCPLocalListenerConfig{Address: "127.0.0.1:18888"}).Address)
}
