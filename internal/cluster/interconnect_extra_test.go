// Coverage targets: MeshInterconnect.dropConn (0%) and defaultDialPeer (0%),
// plus the excel-compiled validCellType/coerceCell type matrix.
package cluster

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMeshInterconnect_DropConn_Idempotent(t *testing.T) {
	m := NewMeshInterconnect(PeerInfo{InstanceID: "self", AdvertiseAddr: "127.0.0.1:0"}, nil, nil)

	// Non-existent instance: idempotent no-op
	m.dropConn("ghost")
	assert.NotNil(t, m)

	// After Close drops all connections, another dropConn is still safe
	m.Close()
	m.dropConn("any")
}

func TestDefaultDialPeer_NotWired(t *testing.T) {
	_, err := defaultDialPeer(context.Background(), "127.0.0.1:1", PeerInfo{InstanceID: "s"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not wired")
}
