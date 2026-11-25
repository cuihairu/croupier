// Coverage targets: agent callProvider falling back to callLocalProvider when no
// session exists (0%), the connection-failed address error branch; and the bypass
// of the unreachable marshal failure path in mustMarshal.
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

	// No provider session + unreachable fallback address -> connection error.
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
