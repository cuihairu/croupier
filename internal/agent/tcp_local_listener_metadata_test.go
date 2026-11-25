// Instance metadata regression for the TCP provider connection path:
// ProviderConnectRequest.metadata must reach ProviderSession.Metadata and be
// folded into the agentlocal instance metadata at onConnect registration.
// Production once lost this whole segment on this path — handleConnect parsed
// the request but never read metadata, onConnect only refilled platform keys,
// and server-side sdk-stats reported meta=None for every TCP provider.
package agent

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	transportcore "github.com/cuihairu/croupier/core/transport"
	tcptr "github.com/cuihairu/croupier/core/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestTCPProviderConnectCarriesUserMetadata(t *testing.T) {
	listener, err := NewTCPLocalListener(&TCPLocalListenerConfig{Address: "127.0.0.1:0"}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })

	connected := make(chan *ProviderSession, 1)
	listener.SetOnConnect(func(sess *ProviderSession) { connected <- sess })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = listener.Serve(ctx) }()

	conn, err := net.Dial("tcp", listener.Addr())
	require.NoError(t, err)
	provider := tcptr.NewMuxConn(conn, nil, transportcore.HandlerFunc(
		func(_ context.Context, _ uint32, _ uint32, _ []byte) ([]byte, error) { return nil, nil }))
	go func() { _ = provider.Run(ctx) }()
	t.Cleanup(func() { _ = provider.Close() })

	connectBody, err := proto.Marshal(&sdkv1.ProviderConnectRequest{
		ServiceId:   "meta-provider",
		Version:     "1.0.0",
		SdkLanguage: "go",
		SdkVersion:  "v0.0.1",
		Metadata: map[string]string{
			"serverId":    "s1",
			"pod":         "claude-e2e",
			"sdkLanguage": "evil",  // reserved key, must be dropped
			"":            "blank", // blank key, must be skipped
		},
	})
	require.NoError(t, err)
	respID, respBody, err := provider.Call(ctx, protocol.MsgProviderConnectRequest, connectBody)
	require.NoError(t, err)
	require.EqualValues(t, protocol.MsgProviderConnectResponse, respID)
	resp := &sdkv1.ProviderConnectResponse{}
	require.NoError(t, proto.Unmarshal(respBody, resp))

	select {
	case sess := <-connected:
		assert.Equal(t, "s1", sess.Metadata["serverId"])
		assert.Equal(t, "claude-e2e", sess.Metadata["pod"])
		_, reserved := sess.Metadata["sdkLanguage"]
		assert.False(t, reserved, "reserved key must be dropped from session metadata")
		assert.NotContains(t, sess.Metadata, "", "blank key must be skipped")
	case <-time.After(5 * time.Second):
		t.Fatal("onConnect never fired")
	}

	// The instance in the session store also carries the user metadata.
	sess, ok := listener.SessionStore().GetByServiceID("meta-provider")
	require.True(t, ok)
	assert.Equal(t, "s1", sess.Metadata["serverId"])

	reservedFlagged := false
	for _, w := range resp.GetWarnings() {
		if strings.Contains(w, "reserved") {
			reservedFlagged = true
		}
	}
	assert.True(t, reservedFlagged, "response warnings should flag reserved key, got %v", resp.GetWarnings())
}

// onConnect-side registration metadata = fixed platform keys + the user KVs
// carried on the session (reserved keys already stripped at parse).
func TestMergeProviderInstanceMetadata(t *testing.T) {
	sess := &ProviderSession{
		SDKLanguage: "go",
		SDKVersion:  "v1.2.3",
		SDKName:     "croupier-go-sdk",
		Metadata:    map[string]string{"serverId": "s9", "pod": "p1"},
	}
	got := MergeProviderInstanceMetadata(sess, "default", "dev")
	assert.Equal(t, map[string]string{
		"sdkLanguage": "go",
		"sdkVersion":  "v1.2.3",
		"sdkName":     "croupier-go-sdk",
		"gameId":      "default",
		"env":         "dev",
		"serverId":    "s9",
		"pod":         "p1",
	}, got)

	// No user metadata, no panic; platform keys all present (equivalent to the
	// old behavior).
	bare := MergeProviderInstanceMetadata(&ProviderSession{}, "g", "e")
	assert.Equal(t, map[string]string{
		"sdkLanguage": "", "sdkVersion": "", "sdkName": "", "gameId": "g", "env": "e",
	}, bare)
}
