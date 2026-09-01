package agent

import (
	"context"
	"net"
	"testing"
	"time"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	transportcore "github.com/cuihairu/croupier/internal/transport"
	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// providerCallDeadline：metadata timeout_ms clamp 与默认值语义。
func TestProviderCallDeadlineSemantics(t *testing.T) {
	h := NewLocalHandler(nil, "/tmp", "agent-1", nil)

	// 默认 15s（对齐 Server 派发层，替代旧硬编码 10s）
	assert.Equal(t, 15*time.Second, h.providerCallDeadline(nil))
	assert.Equal(t, 15*time.Second, h.providerCallDeadline(map[string]string{"timeout_ms": "garbage"}))

	// 请求声明更小值 → 生效
	assert.Equal(t, 3*time.Second, h.providerCallDeadline(map[string]string{"timeout_ms": "3000"}))
	// 低于 1s 提到 1s；高于配置默认 → clamp 回默认
	assert.Equal(t, time.Second, h.providerCallDeadline(map[string]string{"timeout_ms": "50"}))
	assert.Equal(t, 15*time.Second, h.providerCallDeadline(map[string]string{"timeout_ms": "999999"}))

	// 配置自定义默认：请求声明可小于配置，不可超过
	h.SetProviderCallTimeout(2 * time.Second)
	assert.Equal(t, 2*time.Second, h.providerCallDeadline(nil))
	assert.Equal(t, 1500*time.Millisecond, h.providerCallDeadline(map[string]string{"timeout_ms": "1500"}))
	assert.Equal(t, 2*time.Second, h.providerCallDeadline(map[string]string{"timeout_ms": "30000"}))

	// 非法配置回落默认；超上限截断
	h.SetProviderCallTimeout(-1)
	assert.Equal(t, 15*time.Second, h.providerCallDeadline(nil))
	h.SetProviderCallTimeout(5 * time.Minute)
	assert.Equal(t, 60*time.Second, h.providerCallDeadline(nil))
}

// 端到端：慢 provider 在 metadata 声明的预算内未完成 → 调用方拿到超时错误。
func TestHandleInvokeRespectsMetadataTimeout(t *testing.T) {
	listener, err := NewTCPLocalListener(&TCPLocalListenerConfig{Address: "127.0.0.1:0"}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = listener.Serve(ctx) }()

	conn, err := net.Dial("tcp", listener.Addr())
	require.NoError(t, err)
	provider := tcptr.NewMuxConn(conn, nil, transportcore.HandlerFunc(
		func(_ context.Context, msgID uint32, _ uint32, body []byte) ([]byte, error) {
			if msgID == protocol.MsgInvokeRequest {
				req := &sdkv1.InvokeRequest{}
				require.NoError(t, proto.Unmarshal(body, req))
				// 慢于声明的 1200ms 预算
				time.Sleep(2500 * time.Millisecond)
				return proto.Marshal(&sdkv1.InvokeResponse{Payload: []byte(`{}`)})
			}
			return nil, nil
		}))
	go func() { _ = provider.Run(ctx) }()
	t.Cleanup(func() { _ = provider.Close() })

	connectBody, err := proto.Marshal(&sdkv1.ProviderConnectRequest{
		ServiceId: "slow-provider",
		Version:   "1.0.0",
		Functions: []*sdkv1.ProviderFunctionDescriptor{{Id: "slow.fn", Version: "1.0.0"}},
	})
	require.NoError(t, err)
	_, _, err = provider.Call(ctx, protocol.MsgProviderConnectRequest, connectBody)
	require.NoError(t, err)

	session, ok := listener.SessionStore().GetByServiceID("slow-provider")
	require.True(t, ok)
	require.NotNil(t, session.Conn())

	// 生产装配形态：session 连接回调把函数注册进 local store（与
	// app.go SetOnConnect 相同），pickInstance 才能路由到该函数。
	store := agentlocal.NewLocalStore()
	listener.SetOnConnect(func(sess *ProviderSession) {
		store.Register(sess.SessionID, sess.ServiceID, sess.Conn().RemoteAddr(), sess.Version, sess.Functions, nil)
	})
	// 重新握手触发 onConnect（上面的注册发生在 SetOnConnect 之前）
	conn2, err := net.Dial("tcp", listener.Addr())
	require.NoError(t, err)
	provider2 := tcptr.NewMuxConn(conn2, nil, transportcore.HandlerFunc(
		func(_ context.Context, msgID uint32, _ uint32, _ []byte) ([]byte, error) {
			if msgID == protocol.MsgInvokeRequest {
				time.Sleep(2500 * time.Millisecond)
				return proto.Marshal(&sdkv1.InvokeResponse{Payload: []byte(`{}`)})
			}
			return nil, nil
		}))
	go func() { _ = provider2.Run(ctx) }()
	t.Cleanup(func() { _ = provider2.Close() })
	_, _, err = provider2.Call(ctx, protocol.MsgProviderConnectRequest, connectBody)
	require.NoError(t, err)

	h := NewLocalHandler(store, "/tmp", "agent-1", nil)
	h.SetProviderSessionStore(listener.SessionStore())

	// 1) 声明 1200ms 预算，provider 睡 2.5s → 必须在预算内超时
	invokeBody, err := proto.Marshal(&sdkv1.InvokeRequest{
		FunctionId: "slow.fn",
		Payload:    []byte(`{}`),
		Metadata:   map[string]string{"timeout_ms": "1200"},
	})
	require.NoError(t, err)

	start := time.Now()
	_, err = h.Handle(ctx, protocol.MsgInvokeRequest, 1, invokeBody)
	require.Error(t, err, "调用必须超时")
	elapsed := time.Since(start)
	assert.Less(t, elapsed, 2300*time.Millisecond, "应在声明的 ~1.2s 预算内失败")
	assert.GreaterOrEqual(t, elapsed, 1100*time.Millisecond, "不应早于预算误伤")

	// 2) 不声明预算 → 走默认 15s，provider 2.5s 正常返回成功
	invokeBody2, err := proto.Marshal(&sdkv1.InvokeRequest{
		FunctionId: "slow.fn",
		Payload:    []byte(`{}`),
	})
	require.NoError(t, err)
	resp, err := h.Handle(ctx, protocol.MsgInvokeRequest, 2, invokeBody2)
	require.NoError(t, err, "默认预算下慢 provider 应成功")
	assert.NotEmpty(t, resp)
}
