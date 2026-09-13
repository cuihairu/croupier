package agent

// 覆盖率补洞（group L）：provider_keepalive.go probeOnce 的探测帧
// proto.Marshal 失败 continue 分支；task_runner.go marshalTaskInvoke 的
// Marshal 失败分支。只新增用例，不改产品代码与既有测试。

import (
	"context"
	"net"
	"testing"
	"time"

	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// proto3 string 字段含无效 UTF-8 时 Marshal 必败。探测帧的 SessionID 来自
// ProviderSessionStore（不经 proto 反序列化闸门直填），可以构造出编码必败
// 的会话——probeOnce 必须跳过它而不是误摘除。
func TestProviderKeepaliveProbeOnce_MarshalFailureSkipsWithoutRemoval(t *testing.T) {
	store := NewProviderSessionStore()

	agentSide, peerSide := net.Pipe()
	defer func() { _ = peerSide.Close() }()
	conn := tcptr.NewMuxConn(agentSide, nil, nil)
	defer func() { _ = conn.Close() }()

	sess := &ProviderSession{
		conn:      conn,
		SessionID: "\xff\xfe", // 无效 UTF-8：心跳探测帧编码必败
		ServiceID: "svc-bad-utf8",
	}
	require.NoError(t, store.Add(sess))

	k := NewProviderKeepalive(store, time.Hour, nil)
	k.probeOnce(context.Background())

	_, ok := store.GetBySessionID("\xff\xfe")
	assert.True(t, ok, "marshal-failed session must be skipped, not removed")
}

func TestMarshalTaskInvoke_NilAndInvalidUTF8(t *testing.T) {
	assert.Nil(t, marshalTaskInvoke(nil))
	assert.Nil(t, marshalTaskInvoke(&sdkv1.InvokeRequest{
		FunctionId: "demo.ping",
		Metadata:   map[string]string{"k": "\xff\xfe"},
	}))

	data := marshalTaskInvoke(&sdkv1.InvokeRequest{FunctionId: "demo.ok"})
	require.NotNil(t, data)
	var back sdkv1.InvokeRequest
	require.NoError(t, proto.Unmarshal(data, &back))
	assert.Equal(t, "demo.ok", back.GetFunctionId())
}
