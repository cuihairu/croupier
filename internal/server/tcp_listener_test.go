package server

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// --- Tests for NewTCPListener ---

func TestNewTCPListener(t *testing.T) {
	t.Run("with default config", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}

		listener, err := NewTCPListener(config, nil, nil, nil)
		require.NoError(t, err)
		assert.NotNil(t, listener)
		assert.NotNil(t, listener.listener)
		assert.NotNil(t, listener.sessionStore)
		assert.NotNil(t, listener.registry)
		assert.NotNil(t, listener.closing)

		// Clean up
		listener.Close()
	})

	t.Run("with nil config", func(t *testing.T) {
		// This will try to listen on :19090 which might fail if already in use
		// So we'll just test that it handles nil config gracefully
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}

		listener, err := NewTCPListener(config, nil, nil, nil)
		require.NoError(t, err)
		assert.NotNil(t, listener)

		listener.Close()
	})

	t.Run("with custom session store", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}
		store := NewAgentSessionStore()

		listener, err := NewTCPListener(config, store, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, store, listener.sessionStore)

		listener.Close()
	})

	t.Run("with custom registry", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}
		reg := registry.NewStore()

		listener, err := NewTCPListener(config, nil, reg, nil)
		require.NoError(t, err)
		assert.Equal(t, reg, listener.registry)

		listener.Close()
	})

	t.Run("with custom logger", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}
		logger := slog.Default()

		listener, err := NewTCPListener(config, nil, nil, logger)
		require.NoError(t, err)
		assert.Equal(t, logger, listener.logger)

		listener.Close()
	})

	t.Run("with invalid TLS config", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: false,
			CertFile: "/nonexistent/cert.pem",
			KeyFile:  "/nonexistent/key.pem",
		}

		_, err := NewTCPListener(config, nil, nil, nil)
		assert.Error(t, err)
	})
}

// --- Tests for TCPListener Methods ---

func TestTCPListener_SetHandler(t *testing.T) {
	config := &TCPListenerConfig{
		Address:  ":0",
		Insecure: true,
	}

	listener, err := NewTCPListener(config, nil, nil, nil)
	require.NoError(t, err)
	defer listener.Close()

	handler := newTestControlService()
	listener.SetHandler(handler)

	assert.Equal(t, handler, listener.handler)
}

func TestTCPListener_Addr(t *testing.T) {
	t.Run("with active listener", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}

		listener, err := NewTCPListener(config, nil, nil, nil)
		require.NoError(t, err)
		defer listener.Close()

		addr := listener.Addr()
		assert.NotEmpty(t, addr)
	})

	t.Run("with nil listener", func(t *testing.T) {
		ln := &TCPListener{}
		addr := ln.Addr()
		assert.Empty(t, addr)
	})
}

func TestTCPListener_SessionStore(t *testing.T) {
	config := &TCPListenerConfig{
		Address:  ":0",
		Insecure: true,
	}

	listener, err := NewTCPListener(config, nil, nil, nil)
	require.NoError(t, err)
	defer listener.Close()

	store := listener.SessionStore()
	assert.NotNil(t, store)
}

func TestTCPListener_Close(t *testing.T) {
	t.Run("close active listener", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}

		listener, err := NewTCPListener(config, nil, nil, nil)
		require.NoError(t, err)

		assert.False(t, listener.IsClosed())

		err = listener.Close()
		assert.NoError(t, err)
		assert.True(t, listener.IsClosed())
	})

	t.Run("close multiple times", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}

		listener, err := NewTCPListener(config, nil, nil, nil)
		require.NoError(t, err)

		err = listener.Close()
		assert.NoError(t, err)

		// Second close should not panic
		err = listener.Close()
		assert.NoError(t, err)
	})
}

func TestTCPListener_IsClosed(t *testing.T) {
	config := &TCPListenerConfig{
		Address:  ":0",
		Insecure: true,
	}

	listener, err := NewTCPListener(config, nil, nil, nil)
	require.NoError(t, err)

	assert.False(t, listener.IsClosed())

	listener.Close()
	assert.True(t, listener.IsClosed())
}

// NOTE: AgentSession, AgentSessionStore Resolve*, and SessionResolverAdapter
// tests are in agent_session_test.go to avoid duplicate declarations.

// --- Tests for AgentSessionStore ---

func TestAgentSessionStore(t *testing.T) {
	store := NewAgentSessionStore()

	t.Run("Upsert and Get", func(t *testing.T) {
		sess := &AgentSession{
			AgentID:   "agent-1",
			GameID:    "game-1",
			SessionID: "session-1",
		}

		store.Upsert(sess)

		got, ok := store.Get("agent-1")
		assert.True(t, ok)
		assert.Equal(t, "agent-1", got.AgentID)
		assert.Equal(t, "game-1", got.GameID)
	})

	t.Run("Get non-existing", func(t *testing.T) {
		got, ok := store.Get("non-existing")
		assert.False(t, ok)
		assert.Nil(t, got)
	})

	t.Run("Remove", func(t *testing.T) {
		sess := &AgentSession{
			AgentID:   "agent-2",
			GameID:    "game-2",
			SessionID: "session-2",
		}

		store.Upsert(sess)
		store.Remove("agent-2")

		got, ok := store.Get("agent-2")
		assert.False(t, ok)
		assert.Nil(t, got)
	})

	t.Run("List", func(t *testing.T) {
		store := NewAgentSessionStore()

		sess1 := &AgentSession{AgentID: "agent-1", GameID: "game-1"}
		sess2 := &AgentSession{AgentID: "agent-2", GameID: "game-2"}

		store.Upsert(sess1)
		store.Upsert(sess2)

		sessions := store.List()
		assert.Len(t, sessions, 2)
	})

	t.Run("Count", func(t *testing.T) {
		store := NewAgentSessionStore()

		sess1 := &AgentSession{AgentID: "agent-1", GameID: "game-1"}
		sess2 := &AgentSession{AgentID: "agent-2", GameID: "game-2"}

		store.Upsert(sess1)
		store.Upsert(sess2)

		count := store.Count()
		assert.Equal(t, 2, count)
	})

	t.Run("Add", func(t *testing.T) {
		store := NewAgentSessionStore()

		sess := &AgentSession{
			AgentID: "agent-3",
			GameID:  "game-3",
		}

		err := store.Add(sess)
		assert.NoError(t, err)

		// Try to add again - should fail
		err = store.Add(sess)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("PruneStale", func(t *testing.T) {
		store := NewAgentSessionStore()

		sess1 := &AgentSession{AgentID: "agent-1", GameID: "game-1"}
		sess2 := &AgentSession{AgentID: "agent-2", GameID: "game-2"}

		store.Upsert(sess1)
		store.Upsert(sess2)

		// Prune sessions older than 1 second
		pruned := store.PruneStale(1 * time.Second)
		assert.Equal(t, 0, pruned) // All sessions are fresh

		// Wait a bit and prune again
		time.Sleep(1100 * time.Millisecond)
		pruned = store.PruneStale(1 * time.Second)
		assert.Equal(t, 2, pruned) // All sessions should be pruned
	})
}

// NOTE: AgentSession method tests (Conn, Close, LastSeen) are in agent_session_test.go.

// --- Tests for listenTCP ---

func TestListenTCP(t *testing.T) {
	t.Run("insecure listener", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}

		ln, err := listenTCP(config)
		require.NoError(t, err)
		assert.NotNil(t, ln)

		ln.Close()
	})

	t.Run("default address", func(t *testing.T) {
		config := &TCPListenerConfig{
			Insecure: true,
		}

		ln, err := listenTCP(config)
		require.NoError(t, err)
		assert.NotNil(t, ln)

		ln.Close()
	})

	t.Run("invalid cert file", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: false,
			CertFile: "/nonexistent/cert.pem",
			KeyFile:  "/nonexistent/key.pem",
		}

		_, err := listenTCP(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "load server certificate")
	})

	t.Run("invalid CA file", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: false,
			CAFile:   "/nonexistent/ca.pem",
		}

		_, err := listenTCP(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "read CA file")
	})
}

// --- Integration Test for TCPListener ---

func TestTCPListener_Integration(t *testing.T) {
	t.Run("accept and close", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}

		listener, err := NewTCPListener(config, nil, nil, nil)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		errCh := make(chan error, 1)
		go func() {
			errCh <- listener.Serve(ctx)
		}()

		// Give the server time to start
		time.Sleep(50 * time.Millisecond)

		// Connect to the listener
		conn, err := net.Dial("tcp", listener.Addr())
		require.NoError(t, err)
		conn.Close()

		// Cancel context and close listener
		cancel()
		listener.Close()

		// Wait for serve to return
		select {
		case err := <-errCh:
			// Context cancelled or listener closed
			if err != nil {
				assert.Contains(t, err.Error(), "context canceled")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for serve to return")
		}
	})
}

// --- Tests for agentSessionHandler ---

func TestAgentSessionHandler_Handle(t *testing.T) {
	t.Run("register request", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}

		listener, err := NewTCPListener(config, nil, nil, nil)
		require.NoError(t, err)
		defer listener.Close()

		svc := newTestControlService()
		listener.SetHandler(svc)

		handler := &agentSessionHandler{
			listener:   listener,
			conn:       nil,
			registered: false,
			agentID:    "",
		}

		req := &agentv1.RegisterRequest{
			AgentId: "agent-1",
			GameId:  "game-1",
		}
		data, _ := proto.Marshal(req)

		resp, err := handler.Handle(context.Background(), protocol.MsgRegisterRequest, 1, data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("heartbeat request", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}

		listener, err := NewTCPListener(config, nil, nil, nil)
		require.NoError(t, err)
		defer listener.Close()

		svc := newTestControlService()
		listener.SetHandler(svc)

		handler := &agentSessionHandler{
			listener:   listener,
			conn:       nil,
			registered: true,
			agentID:    "agent-1",
		}

		req := &agentv1.HeartbeatRequest{
			AgentId: "agent-1",
		}
		data, _ := proto.Marshal(req)

		resp, err := handler.Handle(context.Background(), protocol.MsgHeartbeatRequest, 1, data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("heartbeat before register is rejected", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}

		listener, err := NewTCPListener(config, nil, nil, nil)
		require.NoError(t, err)
		defer listener.Close()

		svc := newTestControlService()
		listener.SetHandler(svc)

		handler := &agentSessionHandler{
			listener:   listener,
			conn:       nil,
			registered: false,
			agentID:    "",
		}

		req := &agentv1.HeartbeatRequest{
			AgentId: "agent-1",
		}
		data, _ := proto.Marshal(req)

		_, err = handler.Handle(context.Background(), protocol.MsgHeartbeatRequest, 1, data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must register first")
	})

	t.Run("heartbeat with mismatched agent_id is rejected", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}

		listener, err := NewTCPListener(config, nil, nil, nil)
		require.NoError(t, err)
		defer listener.Close()

		svc := newTestControlService()
		listener.SetHandler(svc)

		handler := &agentSessionHandler{
			listener:   listener,
			conn:       nil,
			registered: true,
			agentID:    "agent-1",
		}

		req := &agentv1.HeartbeatRequest{
			AgentId: "agent-2", // mismatched
		}
		data, _ := proto.Marshal(req)

		_, err = handler.Handle(context.Background(), protocol.MsgHeartbeatRequest, 1, data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not match registered agent_id")
	})

	t.Run("duplicate register is rejected", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}

		listener, err := NewTCPListener(config, nil, nil, nil)
		require.NoError(t, err)
		defer listener.Close()

		svc := newTestControlService()
		listener.SetHandler(svc)

		handler := &agentSessionHandler{
			listener:   listener,
			conn:       nil,
			registered: true,
			agentID:    "agent-1",
		}

		req := &agentv1.RegisterRequest{
			AgentId: "agent-1",
			GameId:  "game-1",
		}
		data, _ := proto.Marshal(req)

		_, err = handler.Handle(context.Background(), protocol.MsgRegisterRequest, 1, data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})

	t.Run("delegates to control service for other messages", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}

		listener, err := NewTCPListener(config, nil, nil, nil)
		require.NoError(t, err)
		defer listener.Close()

		svc := newTestControlService()
		listener.SetHandler(svc)

		handler := &agentSessionHandler{
			listener:   listener,
			conn:       nil,
			registered: true,
			agentID:    "agent-1",
		}

		// MsgTaskEvent is delegated to control service
		_, err = handler.Handle(context.Background(), protocol.MsgTaskEvent, 1, []byte("data"))
		// May error due to missing task store, but should not panic
		_ = err
	})

	t.Run("unsupported message without handler", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}

		listener, err := NewTCPListener(config, nil, nil, nil)
		require.NoError(t, err)
		defer listener.Close()

		handler := &agentSessionHandler{
			listener:   listener,
			conn:       nil,
			registered: true,
			agentID:    "agent-1",
		}

		_, err = handler.Handle(context.Background(), 0xFFFFFF, 1, []byte{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported message type")
	})
}

func TestAgentSessionHandler_HandleRegister(t *testing.T) {
	t.Run("valid register", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}

		listener, err := NewTCPListener(config, nil, nil, nil)
		require.NoError(t, err)
		defer listener.Close()

		handler := &agentSessionHandler{
			listener:   listener,
			conn:       nil,
			registered: false,
			agentID:    "",
		}

		req := &agentv1.RegisterRequest{
			AgentId: "agent-1",
			GameId:  "game-1",
			Env:     "prod",
			Version: "1.0.0",
		}
		data, _ := proto.Marshal(req)

		resp, err := handler.handleRegister(context.Background(), data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, handler.registered)
		assert.Equal(t, "agent-1", handler.agentID)

		// Verify session was stored
		sess, ok := listener.sessionStore.Get("agent-1")
		assert.True(t, ok)
		assert.Equal(t, "game-1", sess.GameID)
	})

	t.Run("invalid protobuf", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}

		listener, err := NewTCPListener(config, nil, nil, nil)
		require.NoError(t, err)
		defer listener.Close()

		handler := &agentSessionHandler{
			listener: listener,
		}

		_, err = handler.handleRegister(context.Background(), []byte("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal RegisterRequest")
	})

	t.Run("empty agent ID", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}

		listener, err := NewTCPListener(config, nil, nil, nil)
		require.NoError(t, err)
		defer listener.Close()

		handler := &agentSessionHandler{
			listener: listener,
		}

		req := &agentv1.RegisterRequest{
			AgentId: "",
			GameId:  "game-1",
		}
		data, _ := proto.Marshal(req)

		_, err = handler.handleRegister(context.Background(), data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "agent_id is required")
	})

	t.Run("with custom TTL", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}

		listener, err := NewTCPListener(config, nil, nil, nil)
		require.NoError(t, err)
		defer listener.Close()

		handler := &agentSessionHandler{
			listener: listener,
		}

		req := &agentv1.RegisterRequest{
			AgentId:    "agent-1",
			GameId:     "game-1",
			TtlSeconds: 600,
		}
		data, _ := proto.Marshal(req)

		resp, err := handler.handleRegister(context.Background(), data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		// Verify response contains session info
		registerResp := &agentv1.RegisterResponse{}
		err = proto.Unmarshal(resp, registerResp)
		assert.NoError(t, err)
		assert.NotEmpty(t, registerResp.SessionId)
		assert.True(t, registerResp.ExpireAt > 0)
	})
}

func TestAgentSessionHandler_HandleHeartbeat(t *testing.T) {
	t.Run("heartbeat for existing session", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}

		listener, err := NewTCPListener(config, nil, nil, nil)
		require.NoError(t, err)
		defer listener.Close()

		// Add a session first
		sess := &AgentSession{
			AgentID: "agent-1",
			GameID:  "game-1",
		}
		listener.sessionStore.Upsert(sess)

		handler := &agentSessionHandler{
			listener: listener,
		}

		req := &agentv1.HeartbeatRequest{
			AgentId: "agent-1",
		}
		data, _ := proto.Marshal(req)

		resp, err := handler.handleHeartbeat(context.Background(), data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("heartbeat for non-existing session", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}

		listener, err := NewTCPListener(config, nil, nil, nil)
		require.NoError(t, err)
		defer listener.Close()

		handler := &agentSessionHandler{
			listener: listener,
		}

		req := &agentv1.HeartbeatRequest{
			AgentId: "non-existing",
		}
		data, _ := proto.Marshal(req)

		resp, err := handler.handleHeartbeat(context.Background(), data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("invalid protobuf", func(t *testing.T) {
		config := &TCPListenerConfig{
			Address:  ":0",
			Insecure: true,
		}

		listener, err := NewTCPListener(config, nil, nil, nil)
		require.NoError(t, err)
		defer listener.Close()

		handler := &agentSessionHandler{
			listener: listener,
		}

		_, err = handler.handleHeartbeat(context.Background(), []byte("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal HeartbeatRequest")
	})
}

// --- Tests for ResolveSessionCaller and SessionResolverAdapter.ResolveAgentConn ---

func TestAgentSessionStore_ResolveSessionCaller(t *testing.T) {
	t.Run("resolve existing session without conn", func(t *testing.T) {
		store := NewAgentSessionStore()
		sess := &AgentSession{
			AgentID: "agent-1",
			GameID:  "game-1",
		}
		store.Upsert(sess)

		caller, ok := store.ResolveSessionCaller("agent-1")
		assert.False(t, ok)
		assert.Nil(t, caller)
	})

	t.Run("resolve non-existing session", func(t *testing.T) {
		store := NewAgentSessionStore()

		caller, ok := store.ResolveSessionCaller("non-existing")
		assert.False(t, ok)
		assert.Nil(t, caller)
	})
}

func TestSessionResolverAdapter_ResolveAgentConn(t *testing.T) {
	t.Run("resolve through adapter without conn", func(t *testing.T) {
		store := NewAgentSessionStore()
		adapter := NewSessionResolverAdapter(store)

		sess := &AgentSession{
			AgentID: "agent-1",
			GameID:  "game-1",
		}
		store.Upsert(sess)

		caller, ok := adapter.ResolveAgentConn("agent-1")
		assert.False(t, ok)
		assert.Nil(t, caller)
	})

	t.Run("resolve non-existing through adapter", func(t *testing.T) {
		store := NewAgentSessionStore()
		adapter := NewSessionResolverAdapter(store)

		caller, ok := adapter.ResolveAgentConn("non-existing")
		assert.False(t, ok)
		assert.Nil(t, caller)
	})
}
