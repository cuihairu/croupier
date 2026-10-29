// ARCH-016 provider drain 真实摘流：DrainProviderSession 的标记/等待在途/
// 宽限超窗/undrain/drain-all；callProvider 跳过 draining 会话、全部候选
// drain 时显式拒绝、任务延续放行、在途计数随调用增减；handleProviderDrain
// 的 reason 哨兵 undrain 与空 session_id 节点级 drain-all。
package agent

import (
	"context"
	"net"
	"testing"
	"time"

	transportcore "github.com/cuihairu/croupier/core/transport"
	tcptr "github.com/cuihairu/croupier/core/transport/tcp"
	"github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"

	"github.com/cuihairu/croupier/agents/sidecar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// newDrainTestHandler 构建带 agentlocal 存储 + provider 会话存储的 handler。
func newDrainTestHandler(t *testing.T) (*LocalHandler, *ProviderSessionStore) {
	t.Helper()
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, t.TempDir(), "agent-drain", nil)
	sessStore := NewProviderSessionStore()
	handler.SetProviderSessionStore(sessStore)
	return handler, sessStore
}

// newEchoSession 建立一条可用的 provider 会话：client 侧作为会话 Conn（运行
// 读循环接收响应），server 侧回显。gate 非 nil 时 server 在处理请求前先关闭
// started 再等待 gate（用于观测在途计数）；gate 为 nil 时不阻塞。
func newEchoSession(t *testing.T, sessionID, serviceID, functionID string, started chan<- struct{}, gate <-chan struct{}) *ProviderSession {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() { _ = serverConn.Close(); _ = clientConn.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	server := tcptr.NewMuxConn(serverConn, &tcptr.Config{RecvTimeout: 5 * time.Second, SendTimeout: 5 * time.Second},
		transportcore.HandlerFunc(func(_ context.Context, msgID uint32, _ uint32, body []byte) ([]byte, error) {
			if started != nil {
				close(started)
			}
			if gate != nil {
				<-gate
			}
			return append([]byte("echo:"), body...), nil
		}))
	serverDone := make(chan struct{})
	go func() { defer close(serverDone); _ = server.Run(ctx) }()
	t.Cleanup(func() { _ = server.Close(); <-serverDone })

	client := tcptr.NewMuxConn(clientConn, &tcptr.Config{RecvTimeout: 5 * time.Second, SendTimeout: 5 * time.Second}, nil)
	clientDone := make(chan struct{})
	go func() { defer close(clientDone); _ = client.Run(ctx) }()
	t.Cleanup(func() { _ = client.Close(); <-clientDone })

	sess := sidecar.NewProviderSession(client)
	sess.SessionID = sessionID
	sess.ServiceID = serviceID
	if functionID != "" {
		sess.Functions = []*sdkv1.ProviderFunctionDescriptor{{Id: functionID}}
	}
	return sess
}

// registerDrainSession 把会话同时放入会话存储与 agentlocal 实例表。
func registerDrainSession(t *testing.T, handler *LocalHandler, store *ProviderSessionStore, sess *ProviderSession, functionID string) {
	t.Helper()
	require.NoError(t, store.Add(sess))
	if handler.store != nil && functionID != "" {
		handler.store.Register(sess.SessionID, sess.ServiceID, "127.0.0.1:1", "1.0.0",
			[]*sdkv1.ProviderFunctionDescriptor{{Id: functionID}}, nil)
	}
}

func agentLocalHasProvider(store *agentlocal.LocalStore, functionID, serviceID string) bool {
	if store == nil {
		return false
	}
	svc := store.ListByService()[functionID]
	if svc == nil {
		return false
	}
	return len(svc[serviceID]) > 0
}

// --- DrainProviderSession 语义 ---

func TestDrainProviderSession_MarksDrainingAndRemovesFromAgentLocal(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	sess := newEchoSession(t, "drain-1", "svc-drain", "fn.drain", nil, nil)
	registerDrainSession(t, handler, store, sess, "fn.drain")
	require.True(t, agentLocalHasProvider(handler.store, "fn.drain", "svc-drain"))

	n, err := handler.DrainProviderSession(context.Background(), "drain-1", false, 500*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.True(t, sess.Draining())
	// agentlocal 实例表已摘除（legacy 地址拨号不再路由）
	assert.False(t, agentLocalHasProvider(handler.store, "fn.drain", "svc-drain"))
}

func TestDrainProviderSession_WaitsForInflightZero(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	sess := newEchoSession(t, "drain-2", "svc-2", "fn.2", nil, nil)
	registerDrainSession(t, handler, store, sess, "fn.2")

	sess.AddInflight(1)
	go func() {
		time.Sleep(50 * time.Millisecond)
		sess.AddInflight(-1)
	}()

	start := time.Now()
	n, err := handler.DrainProviderSession(context.Background(), "drain-2", false, 3*time.Second)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	// 在途 50ms 后归零，drain 应远早于宽限返回
	assert.Less(t, time.Since(start), 2*time.Second)
	assert.Equal(t, int64(0), sess.Inflight())
}

func TestDrainProviderSession_TimesOutOnGrace(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	sess := newEchoSession(t, "drain-3", "svc-3", "fn.3", nil, nil)
	registerDrainSession(t, handler, store, sess, "fn.3")

	// 永不归零的在途：drain 在宽限到点后仍返回（不阻塞，不取消执行）
	sess.AddInflight(1)
	start := time.Now()
	n, err := handler.DrainProviderSession(context.Background(), "drain-3", false, 100*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.GreaterOrEqual(t, time.Since(start), 90*time.Millisecond)
	assert.True(t, sess.Draining())
	assert.Equal(t, int64(1), sess.Inflight()) // 执行未被取消
}

func TestDrainProviderSession_UndrainClearsDraining(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	sess := newEchoSession(t, "drain-4", "svc-4", "fn.4", nil, nil)
	registerDrainSession(t, handler, store, sess, "fn.4")

	_, err := handler.DrainProviderSession(context.Background(), "drain-4", false, 500*time.Millisecond)
	require.NoError(t, err)
	require.True(t, sess.Draining())

	start := time.Now()
	n, err := handler.DrainProviderSession(context.Background(), "drain-4", true, 500*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.False(t, sess.Draining())
	assert.Less(t, time.Since(start), 200*time.Millisecond) // undrain 即时返回
}

func TestDrainProviderSession_DrainAll(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	s1 := newEchoSession(t, "all-1", "svc-a", "fn.a", nil, nil)
	s2 := newEchoSession(t, "all-2", "svc-b", "fn.b", nil, nil)
	registerDrainSession(t, handler, store, s1, "fn.a")
	registerDrainSession(t, handler, store, s2, "fn.b")

	n, err := handler.DrainProviderSession(context.Background(), "", false, 500*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.True(t, s1.Draining())
	assert.True(t, s2.Draining())
}

// --- handleProviderDrain：reason 哨兵 undrain / drain-all ---

func TestHandleProviderDrain_UndrainSentinel(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	sess := newEchoSession(t, "u-1", "svc-u", "fn.u", nil, nil)
	registerDrainSession(t, handler, store, sess, "fn.u")

	// drain 先置位
	drainBody, _ := protoMarshalDrain(t, "u-1", "deploy", 0)
	_, err := handler.handleProviderDrain(context.Background(), drainBody)
	require.NoError(t, err)
	require.True(t, sess.Draining())

	// reason=__undrain__ 清除
	undrainBody, _ := protoMarshalDrain(t, "u-1", protocol.ProviderDrainReasonUndrain, 0)
	resp, err := handler.handleProviderDrain(context.Background(), undrainBody)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, sess.Draining())
}

func TestHandleProviderDrain_DrainAll(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	s1 := newEchoSession(t, "ha-1", "svc-ha1", "fn.ha", nil, nil)
	s2 := newEchoSession(t, "ha-2", "svc-ha2", "fn.ha", nil, nil)
	registerDrainSession(t, handler, store, s1, "fn.ha")
	registerDrainSession(t, handler, store, s2, "fn.ha")

	body, _ := protoMarshalDrain(t, "", "deploy", 0)
	resp, err := handler.handleProviderDrain(context.Background(), body)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, s1.Draining())
	assert.True(t, s2.Draining())
}

func protoMarshalDrain(t *testing.T, sessionID, reason string, retryAfterMs uint32) ([]byte, error) {
	t.Helper()
	req := &sdkv1.ProviderDrainRequest{SessionId: sessionID, Reason: reason, RetryAfterMs: retryAfterMs}
	data, err := proto.Marshal(req)
	return data, err
}

// --- callProvider：draining 会话的选路与拒绝 ---

func TestCallProvider_SkipsDrainingSession(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	// 同函数两个会话：draining 的在前或后都不影响——选中非 draining 的可用会话。
	draining := newEchoSession(t, "cp-1", "svc-cp1", "fn.cp", nil, nil)
	good := newEchoSession(t, "cp-2", "svc-cp2", "fn.cp", nil, nil)
	registerDrainSession(t, handler, store, draining, "fn.cp")
	registerDrainSession(t, handler, store, good, "fn.cp")
	draining.SetDraining(true)

	resp, err := handler.callProvider(context.Background(), "fn.cp", map[string]string{}, "127.0.0.1:1", protocol.MsgInvokeRequest, []byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, "echo:hello", string(resp))
	assert.False(t, good.Draining())
}

func TestCallProvider_RejectsWhenAllDraining(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	sess := newEchoSession(t, "cp-3", "svc-cp3", "fn.cp3", nil, nil)
	registerDrainSession(t, handler, store, sess, "fn.cp3")
	sess.SetDraining(true)

	// 函数匹配路径：唯一候选 draining → 显式拒绝（不落到 legacy 地址拨号）
	_, err := handler.callProvider(context.Background(), "fn.cp3", map[string]string{}, "127.0.0.1:1", protocol.MsgInvokeRequest, []byte("{}"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is draining")

	// service_id 定向路径：目标会话 draining → 同样拒绝
	_, err = handler.callProvider(context.Background(), "fn.cp3", map[string]string{"serviceId": "svc-cp3"}, "127.0.0.1:1", protocol.MsgInvokeRequest, []byte("{}"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is draining")
	assert.Contains(t, err.Error(), "svc-cp3")
}

func TestCallProvider_TaskContinuationAllowedOnDraining(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	sess := newEchoSession(t, "cp-4", "svc-cp4", "fn.cp4", nil, nil)
	registerDrainSession(t, handler, store, sess, "fn.cp4")
	sess.SetDraining(true)

	// 任务延续（drain 前已接受）：draining 会话仍被选路，调用正常完成。
	resp, err := handler.callProvider(context.Background(), "fn.cp4",
		map[string]string{"taskId": "task-1"}, "127.0.0.1:1", protocol.MsgInvokeRequest, []byte("payload"))
	require.NoError(t, err)
	assert.Equal(t, "echo:payload", string(resp))
}

func TestCallProvider_InflightCountedAroundCall(t *testing.T) {
	handler, store := newDrainTestHandler(t)
	started := make(chan struct{})
	gate := make(chan struct{})
	sess := newEchoSession(t, "cp-5", "svc-cp5", "fn.cp5", started, gate)
	registerDrainSession(t, handler, store, sess, "fn.cp5")

	callDone := make(chan struct{})
	go func() {
		defer close(callDone)
		_, _ = handler.callProvider(context.Background(), "fn.cp5", map[string]string{}, "127.0.0.1:1", protocol.MsgInvokeRequest, []byte("x"))
	}()

	<-started // server 已收到请求 ⇒ callSession 已加在途
	assert.Equal(t, int64(1), sess.Inflight())

	close(gate)
	<-callDone
	assert.Equal(t, int64(0), sess.Inflight())
}
