// 覆盖目标：MeshInterconnect.dropConn（0%）与 defaultDialPeer（0%），
// excel 编译的 validCellType/coerceCell 类型矩阵。
package cluster

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMeshInterconnect_DropConn_Idempotent(t *testing.T) {
	m := NewMeshInterconnect(PeerInfo{InstanceID: "self", AdvertiseAddr: "127.0.0.1:0"}, nil, nil)

	// 未存在的实例：幂等无操作
	m.dropConn("ghost")
	assert.NotNil(t, m)

	// Close 全部连接后再次 dropConn 仍安全
	m.Close()
	m.dropConn("any")
}

func TestDefaultDialPeer_NotWired(t *testing.T) {
	_, err := defaultDialPeer(context.Background(), "127.0.0.1:1", PeerInfo{InstanceID: "s"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not wired")
}
