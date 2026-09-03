// 覆盖目标：handleListCronJobs、marshalTaskInvoke、executeTask、
// callProvider 会话路由、callLocalProvider 成功路径、provider connect
// scope 告警、providerSessionHandler 握手分支、Serve/listener 错误路径。
package agent

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/agentlocal"
	transportcore "github.com/cuihairu/croupier/internal/transport"
	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func newTestLoggerV9(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.Default()
}

func TestLocalHandlerHandleListCronJobsV9(t *testing.T) {
	t.Run("nil ops server errors", func(t *testing.T) {
		h := NewLocalHandler(agentlocal.NewLocalStore(), "/tmp", "agent-1", nil)
		_, err := h.handleListCronJobs(context.Background(), nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ops server not configured")
	})

	t.Run("delegates to ops server and dispatches by message id", func(t *testing.T) {
		h := NewLocalHandler(agentlocal.NewLocalStore(), "/tmp", "agent-1", nil)
		h.SetOpsServer(&mockOpsServer{cronJobsResp: []byte(`{"jobs":[]}`)})

		resp, err := h.handleListCronJobs(context.Background(), nil)
		require.NoError(t, err)
		assert.Equal(t, `{"jobs":[]}`, string(resp))

		out, err := h.Handle(context.Background(), protocol.MsgListCronJobsRequest, 1, nil)
		require.NoError(t, err)
		assert.Equal(t, `{"jobs":[]}`, string(out))
	})

	t.Run("ops server error propagates", func(t *testing.T) {
		h := NewLocalHandler(agentlocal.NewLocalStore(), "/tmp", "agent-1", nil)
		h.SetOpsServer(&mockOpsServer{cronJobsErr: errors.New("cron boom")})
		_, err := h.handleListCronJobs(context.Background(), nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cron boom")
	})
}

func TestMarshalTaskInvokeV9(t *testing.T) {
	assert.Nil(t, marshalTaskInvoke(nil))

	req := &sdkv1.InvokeRequest{FunctionId: "fn.v9", Payload: []byte(`{}`)}
	data := marshalTaskInvoke(req)
	require.NotNil(t, data)

	decoded := &sdkv1.InvokeRequest{}
	require.NoError(t, proto.Unmarshal(data, decoded))
	assert.Equal(t, "fn.v9", decoded.GetFunctionId())
}

func TestExecuteTaskRoutesThroughInvokePathV9(t *testing.T) {
	newHandlerWithPM := func(resp []byte, callErr error) *LocalHandler {
		h := NewLocalHandler(agentlocal.NewLocalStore(), "/tmp", "agent-1", nil)
		h.SetProviderManager(&mockProviderManager{isPlatformFunc: true, callResp: resp, callErr: callErr})
		return h
	}

	t.Run("payload passthrough", func(t *testing.T) {
		h := newHandlerWithPM([]byte(`{"ok":true}`), nil)
		out, err := h.executeTask(context.Background(), &sdkv1.InvokeRequest{FunctionId: "platform.fn"})
		require.NoError(t, err)
		assert.Equal(t, `{"ok":true}`, string(out))
	})

	t.Run("empty payload normalizes to null", func(t *testing.T) {
		h := newHandlerWithPM(nil, nil)
		out, err := h.executeTask(context.Background(), &sdkv1.InvokeRequest{FunctionId: "platform.fn"})
		require.NoError(t, err)
		assert.Equal(t, "null", string(out))
	})

	t.Run("provider failure surfaces error", func(t *testing.T) {
		h := newHandlerWithPM(nil, errors.New("provider exploded"))
		_, err := h.executeTask(context.Background(), &sdkv1.InvokeRequest{FunctionId: "platform.fn"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "provider call failed")
	})

	t.Run("unregistered function fails", func(t *testing.T) {
		h := NewLocalHandler(agentlocal.NewLocalStore(), "/tmp", "agent-1", nil)
		_, err := h.executeTask(context.Background(), &sdkv1.InvokeRequest{FunctionId: "fn.missing"})
		require.Error(t, err)
	})
}

func TestMustMarshalInvalidUTF8FallsBackV9(t *testing.T) {
	msg := &sdkv1.InvokeRequest{FunctionId: "\xff\xfe"}
	out := mustMarshal(msg)
	require.NotNil(t, out)
	assert.Contains(t, string(out), `"error"`)
}

func TestRecordSpanResultNilSpanV9(t *testing.T) {
	assert.NotPanics(t, func() {
		recordSpanResult(nil, nil)
		recordSpanResult(nil, errors.New("boom"))
	})
}

func TestProviderDescriptorPresentationViolationNilV9(t *testing.T) {
	_, ok := providerDescriptorPresentationViolation(nil)
	assert.False(t, ok)
}

func TestNewTaskRunnerNilExecutorPanicsV9(t *testing.T) {
	assert.Panics(t, func() { NewTaskRunner(nil, nil, nil) })
}

func TestTaskRunnerReportEmptyPayloadV9(t *testing.T) {
	r := NewTaskRunner(func(context.Context, *sdkv1.InvokeRequest) ([]byte, error) {
		return nil, nil
	}, nil, nil)
	require.NoError(t, r.report(context.Background(), &sdkv1.TaskEvent{}))

	reporter := &mockTaskEventReporter{}
	r.SetReporter(reporter)
	event := &sdkv1.TaskEvent{TaskId: "t-v9"}
	require.NoError(t, r.report(context.Background(), event))
	assert.Equal(t, []byte("null"), event.Payload)
	assert.Equal(t, 1, reporter.callCount())
}

func TestHandleCancelTaskNilRunnerV9(t *testing.T) {
	h := &LocalHandler{}
	_, err := h.handleCancelTask(context.Background(), []byte{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "task tracking not available")
}

func TestHandleProviderConnectScopeMismatchV9(t *testing.T) {
	h := NewLocalHandler(agentlocal.NewLocalStore(), "/tmp", "agent-1", nil)
	h.SetExpectedGameEnv("game-a", "prod")

	mismatch := &sdkv1.ProviderConnectRequest{
		ServiceId: "svc-mismatch",
		GameId:    "game-b",
		Env:       "dev",
		Functions: []*sdkv1.ProviderFunctionDescriptor{{Id: "fn.mismatch", Version: "1"}},
	}
	data, err := proto.Marshal(mismatch)
	require.NoError(t, err)

	respData, err := h.handleProviderConnect(context.Background(), data)
	require.NoError(t, err)
	resp := &sdkv1.ProviderConnectResponse{}
	require.NoError(t, proto.Unmarshal(respData, resp))
	require.Len(t, resp.Warnings, 2)
	assert.Contains(t, resp.Warnings[0], "game_id mismatch")
	assert.Contains(t, resp.Warnings[1], "env mismatch")

	match := &sdkv1.ProviderConnectRequest{ServiceId: "svc-ok", GameId: "game-a", Env: "prod"}
	matchData, err := proto.Marshal(match)
	require.NoError(t, err)
	respData2, err := h.handleProviderConnect(context.Background(), matchData)
	require.NoError(t, err)
	resp2 := &sdkv1.ProviderConnectResponse{}
	require.NoError(t, proto.Unmarshal(respData2, resp2))
	assert.Empty(t, resp2.Warnings)
}

func TestHandleProviderConnectNilStoreV9(t *testing.T) {
	h := &LocalHandler{logger: newTestLoggerV9(t)}
	req := &sdkv1.ProviderConnectRequest{
		ServiceId: "svc-nostore",
		Functions: []*sdkv1.ProviderFunctionDescriptor{{Id: "fn.x"}},
	}
	data, err := proto.Marshal(req)
	require.NoError(t, err)

	respData, err := h.handleProviderConnect(context.Background(), data)
	require.NoError(t, err)
	assert.NotEmpty(t, respData)
}

func TestOpsProcessHandlersInvalidProtoV9(t *testing.T) {
	h := NewLocalHandler(agentlocal.NewLocalStore(), "/tmp", "agent-1", nil)

	_, err := h.handleStopProcess(context.Background(), []byte("invalid"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal StopProcessRequest")

	_, err = h.handleStartProcess(context.Background(), []byte("invalid"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal StartProcessRequest")

	_, err = h.handleExecuteCommand(context.Background(), []byte("invalid"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal ExecuteCommandRequest")
}

func TestProviderSessionHandlerDuplicateConnectRejectedV9(t *testing.T) {
	listener, err := NewTCPLocalListener(&TCPLocalListenerConfig{Address: "127.0.0.1:0"}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })

	h := &providerSessionHandler{listener: listener, registered: true, sessionID: "ps-dup"}
	body, err := proto.Marshal(&sdkv1.ProviderConnectRequest{ServiceId: "svc"})
	require.NoError(t, err)

	_, err = h.Handle(context.Background(), protocol.MsgProviderConnectRequest, 1, body)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already connected")
}

func TestProviderSessionHandlerConnectErrorsV9(t *testing.T) {
	listener, err := NewTCPLocalListener(&TCPLocalListenerConfig{Address: "127.0.0.1:0"}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })

	h := &providerSessionHandler{listener: listener}

	_, err = h.handleConnect(context.Background(), []byte("invalid"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal ProviderConnectRequest")

	body, err := proto.Marshal(&sdkv1.ProviderConnectRequest{ServiceId: ""})
	require.NoError(t, err)
	_, err = h.handleConnect(context.Background(), body)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service_id is required")
}

func TestProviderSessionHandlerHeartbeatBranchesV9(t *testing.T) {
	listener, err := NewTCPLocalListener(&TCPLocalListenerConfig{Address: "127.0.0.1:0"}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })

	h := &providerSessionHandler{listener: listener}

	_, err = h.handleHeartbeat(context.Background(), []byte("invalid"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal ProviderHeartbeatRequest")

	// Unknown session id: heartbeat still acks without touching anything.
	unknown, err := proto.Marshal(&sdkv1.ProviderHeartbeatRequest{SessionId: "missing"})
	require.NoError(t, err)
	_, err = h.handleHeartbeat(context.Background(), unknown)
	require.NoError(t, err)

	// Known session + failing local handler: error must propagate.
	listener.SetLocalHandler(transportcore.HandlerFunc(
		func(context.Context, uint32, uint32, []byte) ([]byte, error) {
			return nil, errors.New("local handler boom")
		}))
	listener.sessionStore.Upsert(&ProviderSession{SessionID: "s-v9", ServiceID: "svc-v9"})
	known, err := proto.Marshal(&sdkv1.ProviderHeartbeatRequest{SessionId: "s-v9"})
	require.NoError(t, err)
	_, err = h.handleHeartbeat(context.Background(), known)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "local handler boom")
}

func TestProviderSessionHandlerDrainInvalidProtoV9(t *testing.T) {
	listener, err := NewTCPLocalListener(&TCPLocalListenerConfig{Address: "127.0.0.1:0"}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })

	h := &providerSessionHandler{listener: listener}
	_, err = h.handleDrain(context.Background(), []byte("invalid"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal ProviderDrainRequest")
}

func TestNewTCPLocalListenerListenErrorV9(t *testing.T) {
	_, err := NewTCPLocalListener(&TCPLocalListenerConfig{Address: "302.1.1.1:1"}, nil, nil)
	require.Error(t, err)
}

func TestServeReturnsErrorWhenListenerClosedV9(t *testing.T) {
	listener, err := NewTCPLocalListener(&TCPLocalListenerConfig{Address: "127.0.0.1:0"}, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	errCh := make(chan error, 1)
	go func() { errCh <- listener.Serve(context.Background()) }()

	time.Sleep(100 * time.Millisecond)
	require.NoError(t, listener.listener.Close())

	select {
	case serveErr := <-errCh:
		require.Error(t, serveErr)
		assert.Contains(t, serveErr.Error(), "accept")
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not return after raw listener close")
	}
}

func TestTCPLocalListenerOnDisconnectFiresV9(t *testing.T) {
	store := NewProviderSessionStore()
	listener, err := NewTCPLocalListener(&TCPLocalListenerConfig{Address: "127.0.0.1:0"}, store, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = listener.Serve(ctx) }()

	gone := make(chan *ProviderSession, 1)
	listener.SetOnDisconnect(func(sess *ProviderSession) { gone <- sess })

	client, err := tcptr.NewClient(&tcptr.Config{
		Address:        listener.Addr(),
		Insecure:       true,
		ConnectTimeout: 5 * time.Second,
		RecvTimeout:    30 * time.Second,
		SendTimeout:    30 * time.Second,
	})
	require.NoError(t, err)

	connectBody, err := proto.Marshal(&sdkv1.ProviderConnectRequest{ServiceId: "svc-gone"})
	require.NoError(t, err)
	_, _, err = client.Call(ctx, protocol.MsgProviderConnectRequest, connectBody)
	require.NoError(t, err)
	require.NoError(t, client.Close())

	select {
	case sess := <-gone:
		assert.Equal(t, "svc-gone", sess.ServiceID)
	case <-time.After(3 * time.Second):
		t.Fatal("onDisconnect callback was not invoked")
	}
}

func TestCallProviderSessionRoutingV9(t *testing.T) {
	sessStore, session := newKeepaliveTestProvider(t, "svc-v9", func(msgID uint32, body []byte) ([]byte, error) {
		if msgID != protocol.MsgInvokeRequest {
			return nil, nil
		}
		req := &sdkv1.InvokeRequest{}
		if err := proto.Unmarshal(body, req); err != nil {
			return nil, err
		}
		if bytes.Contains(req.GetPayload(), []byte("garbage")) {
			return []byte("not-proto"), nil
		}
		return proto.Marshal(&sdkv1.InvokeResponse{Payload: []byte(`{"r":"ok"}`)})
	})
	require.NotNil(t, session)

	store := agentlocal.NewLocalStore()
	store.Register("p-v9", "svc-v9", "127.0.0.1:1", "1.0.0",
		[]*sdkv1.ProviderFunctionDescriptor{{Id: "f.ping", Version: "1.0.0"}}, nil)

	h := NewLocalHandler(store, "/tmp", "agent-1", nil)
	h.SetProviderSessionStore(sessStore)

	invoke := func(t *testing.T, meta map[string]string, payload []byte) ([]byte, error) {
		t.Helper()
		req := &sdkv1.InvokeRequest{FunctionId: "f.ping", Payload: payload, Metadata: meta}
		data, err := proto.Marshal(req)
		require.NoError(t, err)
		return h.handleInvoke(context.Background(), data)
	}

	t.Run("routes by service id over established session", func(t *testing.T) {
		out, err := invoke(t, map[string]string{"serviceId": "svc-v9"}, []byte(`{}`))
		require.NoError(t, err)
		resp := &sdkv1.InvokeResponse{}
		require.NoError(t, proto.Unmarshal(out, resp))
		assert.Equal(t, `{"r":"ok"}`, string(resp.GetPayload()))
	})

	t.Run("routes by function id when service id absent", func(t *testing.T) {
		out, err := invoke(t, nil, []byte(`{}`))
		require.NoError(t, err)
		resp := &sdkv1.InvokeResponse{}
		require.NoError(t, proto.Unmarshal(out, resp))
		assert.Equal(t, `{"r":"ok"}`, string(resp.GetPayload()))
	})

	t.Run("garbage response fails invoke response unmarshal", func(t *testing.T) {
		_, err := invoke(t, map[string]string{"serviceId": "svc-v9"}, []byte("garbage"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal InvokeResponse")
	})
}

// startV9RawProvider 起一个裸 TCP provider（服务端 MuxConn），返回监听地址。
func startV9RawProvider(t *testing.T, handler transportcore.Handler) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			mux := tcptr.NewMuxConn(conn, nil, handler)
			go func() { _ = mux.Run(context.Background()) }()
		}
	}()
	return ln.Addr().String()
}

func TestCallLocalProviderReachesFallbackProviderV9(t *testing.T) {
	addr := startV9RawProvider(t, transportcore.HandlerFunc(
		func(_ context.Context, msgID uint32, _ uint32, _ []byte) ([]byte, error) {
			if msgID == protocol.MsgInvokeRequest {
				return proto.Marshal(&sdkv1.InvokeResponse{Payload: []byte(`{"fallback":true}`)})
			}
			return nil, nil
		}))

	store := agentlocal.NewLocalStore()
	// Handler without a provider session store: routing falls back to dialing
	// the instance address via the TCP client path.
	h := NewLocalHandler(store, "/tmp", "agent-1", nil)
	store.Register("p-fallback", "svc-fallback", addr, "1.0.0",
		[]*sdkv1.ProviderFunctionDescriptor{{Id: "fallback.fn", Version: "1.0.0"}}, nil)

	req := &sdkv1.InvokeRequest{FunctionId: "fallback.fn", Payload: []byte(`{}`)}
	data, err := proto.Marshal(req)
	require.NoError(t, err)

	out, err := h.handleInvoke(context.Background(), data)
	require.NoError(t, err)
	resp := &sdkv1.InvokeResponse{}
	require.NoError(t, proto.Unmarshal(out, resp))
	assert.Equal(t, `{"fallback":true}`, string(resp.GetPayload()))
}

func TestPickInstanceServiceWithoutInstancesV9(t *testing.T) {
	store := agentlocal.NewLocalStore()
	h := &LocalHandler{store: store, logger: newTestLoggerV9(t)}
	store.Register("p", "svc-a", "127.0.0.1:12345", "1.0.0",
		[]*sdkv1.ProviderFunctionDescriptor{{Id: "fn.svc", Version: "1"}}, nil)

	_, err := h.pickInstance("fn.svc", map[string]string{"serviceId": "svc-b"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no instance for service")
}

func TestProviderCallDeadlineNonPositiveDefaultV9(t *testing.T) {
	h := NewLocalHandler(nil, "/tmp", "agent-1", nil)
	h.mu.Lock()
	h.providerCallTimeout = -time.Second
	h.mu.Unlock()
	assert.Equal(t, 15*time.Second, h.providerCallDeadline(nil))
}
