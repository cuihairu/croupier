package agent

import (
	"context"
	"log/slog"
	"testing"
	"time"

	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// --- Unit Tests for NewTCPLocalListener ---

func TestNewTCPLocalListener(t *testing.T) {
	t.Run("with valid config", func(t *testing.T) {
		config := &TCPLocalListenerConfig{
			Address:     "127.0.0.1:0",
			RecvTimeout: 30 * time.Second,
			SendTimeout: 30 * time.Second,
		}

		listener, err := NewTCPLocalListener(config, nil, nil)
		require.NoError(t, err)
		assert.NotNil(t, listener)
		assert.NotNil(t, listener.listener)
		assert.NotNil(t, listener.sessionStore)
		assert.NotNil(t, listener.closing)

		listener.Close()
	})

	t.Run("with nil config", func(t *testing.T) {
		listener, err := NewTCPLocalListener(nil, nil, nil)
		require.NoError(t, err)
		assert.NotNil(t, listener)

		listener.Close()
	})

	t.Run("with custom session store", func(t *testing.T) {
		config := &TCPLocalListenerConfig{
			Address:     "127.0.0.1:0",
			RecvTimeout: 30 * time.Second,
			SendTimeout: 30 * time.Second,
		}
		store := NewProviderSessionStore()

		listener, err := NewTCPLocalListener(config, store, nil)
		require.NoError(t, err)
		assert.Equal(t, store, listener.sessionStore)

		listener.Close()
	})

	t.Run("with custom logger", func(t *testing.T) {
		config := &TCPLocalListenerConfig{
			Address:     "127.0.0.1:0",
			RecvTimeout: 30 * time.Second,
			SendTimeout: 30 * time.Second,
		}
		logger := slog.Default()

		listener, err := NewTCPLocalListener(config, nil, logger)
		require.NoError(t, err)
		assert.Equal(t, logger, listener.logger)

		listener.Close()
	})
}

// --- Unit Tests for SetOnConnect ---

func TestTCPLocalListener_SetOnConnect(t *testing.T) {
	config := &TCPLocalListenerConfig{
		Address:     "127.0.0.1:0",
		RecvTimeout: 30 * time.Second,
		SendTimeout: 30 * time.Second,
	}

	listener, err := NewTCPLocalListener(config, nil, nil)
	require.NoError(t, err)
	defer listener.Close()

	listener.SetOnConnect(func(sess *ProviderSession) {
		// callback
	})

	assert.NotNil(t, listener.onConnect)
}

// --- Unit Tests for SetOnDisconnect ---

func TestTCPLocalListener_SetOnDisconnect(t *testing.T) {
	config := &TCPLocalListenerConfig{
		Address:     "127.0.0.1:0",
		RecvTimeout: 30 * time.Second,
		SendTimeout: 30 * time.Second,
	}

	listener, err := NewTCPLocalListener(config, nil, nil)
	require.NoError(t, err)
	defer listener.Close()

	listener.SetOnDisconnect(func(sess *ProviderSession) {
		// callback
	})

	assert.NotNil(t, listener.onDisconnect)
}

// --- Unit Tests for SetLocalHandler ---

func TestTCPLocalListener_SetLocalHandler(t *testing.T) {
	config := &TCPLocalListenerConfig{
		Address:     "127.0.0.1:0",
		RecvTimeout: 30 * time.Second,
		SendTimeout: 30 * time.Second,
	}

	listener, err := NewTCPLocalListener(config, nil, nil)
	require.NoError(t, err)
	defer listener.Close()

	handler := &mockLocalHandler{}
	listener.SetLocalHandler(handler)

	assert.Equal(t, handler, listener.localHandler)
}

// --- Unit Tests for Addr ---

func TestTCPLocalListener_Addr(t *testing.T) {
	t.Run("with active listener", func(t *testing.T) {
		config := &TCPLocalListenerConfig{
			Address:     "127.0.0.1:0",
			RecvTimeout: 30 * time.Second,
			SendTimeout: 30 * time.Second,
		}

		listener, err := NewTCPLocalListener(config, nil, nil)
		require.NoError(t, err)
		defer listener.Close()

		addr := listener.Addr()
		assert.NotEmpty(t, addr)
	})
}

// --- Unit Tests for SessionStore ---

func TestTCPLocalListener_SessionStore(t *testing.T) {
	config := &TCPLocalListenerConfig{
		Address:     "127.0.0.1:0",
		RecvTimeout: 30 * time.Second,
		SendTimeout: 30 * time.Second,
	}

	listener, err := NewTCPLocalListener(config, nil, nil)
	require.NoError(t, err)
	defer listener.Close()

	store := listener.SessionStore()
	assert.NotNil(t, store)
}

// --- Unit Tests for Close ---

func TestTCPLocalListener_Close(t *testing.T) {
	t.Run("close active listener", func(t *testing.T) {
		config := &TCPLocalListenerConfig{
			Address:     "127.0.0.1:0",
			RecvTimeout: 30 * time.Second,
			SendTimeout: 30 * time.Second,
		}

		listener, err := NewTCPLocalListener(config, nil, nil)
		require.NoError(t, err)

		assert.False(t, listener.IsClosed())

		err = listener.Close()
		assert.NoError(t, err)
		assert.True(t, listener.IsClosed())
	})

	t.Run("close multiple times", func(t *testing.T) {
		config := &TCPLocalListenerConfig{
			Address:     "127.0.0.1:0",
			RecvTimeout: 30 * time.Second,
			SendTimeout: 30 * time.Second,
		}

		listener, err := NewTCPLocalListener(config, nil, nil)
		require.NoError(t, err)

		err = listener.Close()
		assert.NoError(t, err)

		// Second close should not panic
		err = listener.Close()
		assert.NoError(t, err)
	})
}

// --- Unit Tests for IsClosed ---

func TestTCPLocalListener_IsClosed(t *testing.T) {
	config := &TCPLocalListenerConfig{
		Address:     "127.0.0.1:0",
		RecvTimeout: 30 * time.Second,
		SendTimeout: 30 * time.Second,
	}

	listener, err := NewTCPLocalListener(config, nil, nil)
	require.NoError(t, err)

	assert.False(t, listener.IsClosed())

	listener.Close()
	assert.True(t, listener.IsClosed())
}

// --- Mock LocalHandler ---

type mockLocalHandler struct{}

func (m *mockLocalHandler) Handle(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
	return nil, nil
}

// --- Integration Tests ---

func TestTCPLocalListenerProviderSessionLifecycle(t *testing.T) {
	t.Parallel()

	store := NewProviderSessionStore()
	listener, err := NewTCPLocalListener(&TCPLocalListenerConfig{
		Address:     "127.0.0.1:0",
		RecvTimeout: 30 * time.Second,
		SendTimeout: 30 * time.Second,
	}, store, nil)
	if err != nil {
		t.Fatalf("new listener: %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- listener.Serve(ctx)
	}()

	client, err := tcptr.NewClient(&tcptr.Config{
		Address:        listener.Addr(),
		Insecure:       true,
		ConnectTimeout: 5 * time.Second,
		RecvTimeout:    30 * time.Second,
		SendTimeout:    30 * time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	connectReq := &sdkv1.ProviderConnectRequest{
		ServiceId:       "provider-1",
		Version:         "1.0.0",
		SdkLanguage:     "go",
		SdkVersion:      "test",
		Functions:       []*sdkv1.ProviderFunctionDescriptor{{Id: "player.ban", Version: "1.0.0"}},
		ProtocolVersion: "v1",
	}
	connectBody, err := proto.Marshal(connectReq)
	if err != nil {
		t.Fatalf("marshal connect: %v", err)
	}

	respMsgID, respBody, err := client.Call(ctx, protocol.MsgProviderConnectRequest, connectBody)
	if err != nil {
		t.Fatalf("connect call: %v", err)
	}
	if respMsgID != protocol.MsgProviderConnectResponse {
		t.Fatalf("response msgID = %s, want %s", protocol.MsgIDString(respMsgID), protocol.MsgIDString(protocol.MsgProviderConnectResponse))
	}

	connectResp := &sdkv1.ProviderConnectResponse{}
	if err := proto.Unmarshal(respBody, connectResp); err != nil {
		t.Fatalf("unmarshal connect response: %v", err)
	}
	if connectResp.GetSessionId() == "" {
		t.Fatal("expected session id")
	}

	sess, ok := store.GetByServiceID("provider-1")
	if !ok {
		t.Fatal("provider session not stored")
	}
	initialLastSeen := sess.GetLastSeen()

	time.Sleep(1100 * time.Millisecond)

	hbReq := &sdkv1.ProviderHeartbeatRequest{
		ServiceId: "provider-1",
		SessionId: connectResp.GetSessionId(),
	}
	hbBody, err := proto.Marshal(hbReq)
	if err != nil {
		t.Fatalf("marshal heartbeat: %v", err)
	}
	respMsgID, _, err = client.Call(ctx, protocol.MsgProviderHeartbeatRequest, hbBody)
	if err != nil {
		t.Fatalf("heartbeat call: %v", err)
	}
	if respMsgID != protocol.MsgProviderHeartbeatResponse {
		t.Fatalf("heartbeat response msgID = %s, want %s", protocol.MsgIDString(respMsgID), protocol.MsgIDString(protocol.MsgProviderHeartbeatResponse))
	}

	sess, ok = store.GetBySessionID(connectResp.GetSessionId())
	if !ok {
		t.Fatal("provider session not found by session id")
	}
	if !sess.GetLastSeen().After(initialLastSeen) {
		t.Fatalf("heartbeat did not update last seen: before=%v after=%v", initialLastSeen, sess.GetLastSeen())
	}

	drainReq := &sdkv1.ProviderDrainRequest{
		SessionId:    connectResp.GetSessionId(),
		Reason:       "deploy",
		RetryAfterMs: 5000,
	}
	drainBody, err := proto.Marshal(drainReq)
	if err != nil {
		t.Fatalf("marshal drain: %v", err)
	}
	respMsgID, _, err = client.Call(ctx, protocol.MsgProviderDrainRequest, drainBody)
	if err != nil {
		t.Fatalf("drain call: %v", err)
	}
	if respMsgID != protocol.MsgProviderDrainResponse {
		t.Fatalf("drain response msgID = %s, want %s", protocol.MsgIDString(respMsgID), protocol.MsgIDString(protocol.MsgProviderDrainResponse))
	}

	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := store.GetBySessionID(connectResp.GetSessionId()); !ok {
			cancel()
			_ = listener.Close()
			<-done
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("provider session was not removed after disconnect")
}

func TestTCPLocalListenerRejectsNonProviderConnectFirstFrame(t *testing.T) {
	t.Parallel()

	store := NewProviderSessionStore()
	listener, err := NewTCPLocalListener(&TCPLocalListenerConfig{
		Address:     "127.0.0.1:0",
		RecvTimeout: 30 * time.Second,
		SendTimeout: 30 * time.Second,
	}, store, nil)
	if err != nil {
		t.Fatalf("new listener: %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- listener.Serve(ctx)
	}()

	client, err := tcptr.NewClient(&tcptr.Config{
		Address:        listener.Addr(),
		Insecure:       true,
		ConnectTimeout: 5 * time.Second,
		RecvTimeout:    30 * time.Second,
		SendTimeout:    30 * time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	_, _, err = client.Call(ctx, protocol.MsgProviderHeartbeatRequest, []byte("{}"))
	if err == nil {
		t.Fatal("expected connection close after invalid first frame")
	}

	if got := store.Count(); got != 0 {
		t.Fatalf("unexpected provider sessions stored: %d", got)
	}

	cancel()
	_ = listener.Close()
	<-done
}

// --- Tests for Addr with nil listener ---

func TestTCPLocalListener_Addr_NilListener(t *testing.T) {
	ln := &TCPLocalListener{}
	addr := ln.Addr()
	assert.Empty(t, addr)
}

// --- Tests for providerSessionHandler.Handle ---

func TestProviderSessionHandler_Handle(t *testing.T) {
	t.Run("invoke request allowed without registration", func(t *testing.T) {
		config := &TCPLocalListenerConfig{
			Address:     "127.0.0.1:0",
			RecvTimeout: 30 * time.Second,
			SendTimeout: 30 * time.Second,
		}

		listener, err := NewTCPLocalListener(config, nil, nil)
		require.NoError(t, err)
		defer listener.Close()

		handler := &providerSessionHandler{
			listener:   listener,
			registered: false,
		}

		// InvokeRequest is allowed without registration
		req := &sdkv1.InvokeRequest{
			FunctionId: "test.func",
			Payload:    []byte("{}"),
		}
		data, _ := proto.Marshal(req)

		// Will delegate to localHandler which is nil, so should error
		_, err = handler.Handle(context.Background(), protocol.MsgInvokeRequest, 1, data)
		assert.Error(t, err)
	})

	t.Run("unsupported message without registration", func(t *testing.T) {
		config := &TCPLocalListenerConfig{
			Address:     "127.0.0.1:0",
			RecvTimeout: 30 * time.Second,
			SendTimeout: 30 * time.Second,
		}

		listener, err := NewTCPLocalListener(config, nil, nil)
		require.NoError(t, err)
		defer listener.Close()

		handler := &providerSessionHandler{
			listener:   listener,
			registered: false,
		}

		// HeartbeatRequest is not allowed without registration
		_, err = handler.Handle(context.Background(), protocol.MsgProviderHeartbeatRequest, 1, []byte("{}"))
		assert.Error(t, err)
	})

	t.Run("delegates to local handler when registered", func(t *testing.T) {
		config := &TCPLocalListenerConfig{
			Address:     "127.0.0.1:0",
			RecvTimeout: 30 * time.Second,
			SendTimeout: 30 * time.Second,
		}

		listener, err := NewTCPLocalListener(config, nil, nil)
		require.NoError(t, err)
		defer listener.Close()

		localHandler := &mockLocalHandler{}
		listener.SetLocalHandler(localHandler)

		handler := &providerSessionHandler{
			listener:   listener,
			registered: true,
		}

		// Unknown message type delegates to local handler
		resp, err := handler.Handle(context.Background(), protocol.MsgGetSystemInfoRequest, 1, []byte{})
		assert.NoError(t, err)
		assert.Nil(t, resp)
	})

	t.Run("unsupported message without local handler", func(t *testing.T) {
		config := &TCPLocalListenerConfig{
			Address:     "127.0.0.1:0",
			RecvTimeout: 30 * time.Second,
			SendTimeout: 30 * time.Second,
		}

		listener, err := NewTCPLocalListener(config, nil, nil)
		require.NoError(t, err)
		defer listener.Close()

		handler := &providerSessionHandler{
			listener:   listener,
			registered: true,
		}

		_, err = handler.Handle(context.Background(), 0xFFFFFF, 1, []byte{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported message type")
	})
}

// --- Tests for convertProtoFunctions ---

func TestConvertProtoFunctions(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		result := convertProtoFunctions(nil)
		assert.Nil(t, result)
	})

	t.Run("empty slice", func(t *testing.T) {
		result := convertProtoFunctions([]*sdkv1.ProviderFunctionDescriptor{})
		assert.Empty(t, result)
	})

	t.Run("with functions", func(t *testing.T) {
		funcs := []*sdkv1.ProviderFunctionDescriptor{
			{Id: "func-1", Version: "1.0.0"},
			{Id: "func-2", Version: "2.0.0"},
		}
		result := convertProtoFunctions(funcs)
		assert.Len(t, result, 2)
		assert.Equal(t, "func-1", result[0].Id)
		assert.Equal(t, "func-2", result[1].Id)
	})
}
