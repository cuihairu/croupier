package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/tasks"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// --- Helper Functions ---

func compressGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func newTestControlService() *ControlService {
	store := registry.NewStore()
	svc := NewControlService(store, nil)
	svc.SetLogger(slog.Default())
	return svc
}

func newTestControlServiceWithLoader(loader AgentSessionLoader) *ControlService {
	store := registry.NewStore()
	svc := NewControlService(store, loader)
	svc.SetLogger(slog.Default())
	return svc
}

// --- Mock AgentSessionLoader ---

type mockAgentSessionLoader struct {
	sessions     []*registry.AgentSession
	upsertErr    error
	deleteCount  int64
	deleteErr    error
	loadErr      error
	upsertCalled int32 // atomic
	deleteCalled int32 // atomic
	upsertDone   chan struct{}
}

func newMockAgentSessionLoader() *mockAgentSessionLoader {
	return &mockAgentSessionLoader{
		upsertDone: make(chan struct{}, 1),
	}
}

func (m *mockAgentSessionLoader) LoadActiveSessions(ctx context.Context) ([]*registry.AgentSession, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	return m.sessions, nil
}

func (m *mockAgentSessionLoader) Upsert(ctx context.Context, sess *registry.AgentSession) error {
	atomic.AddInt32(&m.upsertCalled, 1)
	if m.upsertErr != nil {
		select {
		case m.upsertDone <- struct{}{}:
		default:
		}
		return m.upsertErr
	}
	m.sessions = append(m.sessions, sess)
	select {
	case m.upsertDone <- struct{}{}:
	default:
	}
	return nil
}

func (m *mockAgentSessionLoader) DeleteExpired(ctx context.Context) (int64, error) {
	atomic.AddInt32(&m.deleteCalled, 1)
	if m.deleteErr != nil {
		return 0, m.deleteErr
	}
	return m.deleteCount, nil
}

// --- Mock TaskStore ---

type mockTaskStore struct {
	updateRunErr    error
	appendEventErr  error
	updateRunCalled int
	appendCalled    int
	lastUpdates     map[string]interface{}
	lastTaskID      string
	lastEventType   tasks.EventType
}

func (m *mockTaskStore) UpdateRun(ctx context.Context, taskID string, updates map[string]interface{}) error {
	m.updateRunCalled++
	m.lastTaskID = taskID
	m.lastUpdates = updates
	return m.updateRunErr
}

func (m *mockTaskStore) AppendEvent(ctx context.Context, taskID string, eventType tasks.EventType, progress int32, message string, payload []byte) error {
	m.appendCalled++
	m.lastEventType = eventType
	return m.appendEventErr
}

// --- Tests for ParseListenAddr ---

func TestParseListenAddr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ListenAddr
	}{
		{
			name:  "tcp address without scheme",
			input: ":19090",
			expected: ListenAddr{
				Addr:      ":19090",
				Transport: "tcp",
				URL:       "tcp://:19090",
			},
		},
		{
			name:  "tcp address with scheme",
			input: "tcp://:19090",
			expected: ListenAddr{
				Addr:      ":19090",
				Transport: "tcp",
				URL:       "tcp://:19090",
			},
		},
		{
			name:  "ipc address",
			input: "ipc://croupier-server",
			expected: ListenAddr{
				Addr:      "croupier-server",
				Transport: "ipc",
				URL:       "ipc://croupier-server",
			},
		},
		{
			name:  "host:port address",
			input: "192.168.1.1:8080",
			expected: ListenAddr{
				Addr:      "192.168.1.1:8080",
				Transport: "tcp",
				URL:       "tcp://192.168.1.1:8080",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseListenAddr(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- Tests for IsLocalTCP ---

func TestIsLocalTCP(t *testing.T) {
	tests := []struct {
		name     string
		addr     string
		expected bool
	}{
		{"empty address", "", true},
		{"localhost", "localhost:8080", true},
		{"127.0.0.1", "127.0.0.1:8080", true},
		{"ipv6 loopback", "[::1]:8080", true},
		{"public ip", "192.168.1.1:8080", false},
		{"external host", "example.com:8080", false},
		{"port only", ":8080", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsLocalTCP(tt.addr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- Tests for NewControlService ---

func TestNewControlService(t *testing.T) {
	t.Run("with nil registry", func(t *testing.T) {
		svc := NewControlService(nil, nil)
		assert.NotNil(t, svc)
		assert.NotNil(t, svc.registry)
		assert.NotNil(t, svc.metricsStore)
		assert.NotNil(t, svc.systemInfoCache)
		assert.Equal(t, 5*time.Minute, svc.defaultSessionTTL)
	})

	t.Run("with custom registry", func(t *testing.T) {
		store := registry.NewStore()
		svc := NewControlService(store, nil)
		assert.NotNil(t, svc)
		assert.Equal(t, store, svc.registry)
	})

	t.Run("with loader", func(t *testing.T) {
		loader := &mockAgentSessionLoader{}
		svc := NewControlService(nil, loader)
		assert.NotNil(t, svc)
		assert.Equal(t, loader, svc.agentSessionLoader)
	})
}

// --- Tests for ControlService Methods ---

func TestControlService_SetTaskStore(t *testing.T) {
	svc := newTestControlService()
	store := &mockTaskStore{}
	svc.SetTaskStore(store)

	svc.mu.RLock()
	assert.Equal(t, store, svc.taskStore)
	svc.mu.RUnlock()
}

func TestControlService_Store(t *testing.T) {
	svc := newTestControlService()
	assert.NotNil(t, svc.Store())
}

func TestControlService_MetricsStore(t *testing.T) {
	svc := newTestControlService()
	assert.NotNil(t, svc.MetricsStore())
}

func TestControlService_SystemInfoCache(t *testing.T) {
	svc := newTestControlService()
	assert.NotNil(t, svc.SystemInfoCache())
}

func TestControlService_SetDefaultSessionTTL(t *testing.T) {
	svc := newTestControlService()
	svc.SetDefaultSessionTTL(10 * time.Minute)
	assert.Equal(t, 10*time.Minute, svc.defaultSessionTTL)
}

func TestControlService_SetUpstreamHandler(t *testing.T) {
	svc := newTestControlService()
	handler := &mockHandler{}
	svc.SetUpstreamHandler(handler)

	svc.mu.RLock()
	assert.Equal(t, handler, svc.upstream)
	svc.mu.RUnlock()
}

func TestControlService_SetLogger(t *testing.T) {
	svc := newTestControlService()
	logger := slog.Default()
	svc.SetLogger(logger)

	svc.mu.RLock()
	assert.Equal(t, logger, svc.logger)
	svc.mu.RUnlock()
}

func TestControlService_GetStats(t *testing.T) {
	svc := newTestControlService()

	// Register an agent first
	agent := &registry.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-1",
	}
	svc.registry.UpsertAgent(agent)

	stats := svc.GetStats()
	assert.Equal(t, 1, stats["agent_count"])
	assert.Equal(t, "5m0s", stats["session_ttl"])
}

func TestControlService_Stop(t *testing.T) {
	svc := newTestControlService()
	assert.NotPanics(t, func() {
		svc.Stop()
	})
}

// --- Tests for handleRegisterRequest ---

func TestControlService_HandleRegisterRequest(t *testing.T) {
	t.Run("successful registration", func(t *testing.T) {
		svc := newTestControlService()

		req := &agentv1.RegisterRequest{
			AgentId: "agent-1",
			GameId:  "game-1",
			Env:     "prod",
			Version: "1.0.0",
			Functions: []*agentv1.FunctionDescriptor{
				{
					Id:                "game.player.get",
					Version:           "1.0.0",
					Enabled:           true,
					ApprovalRequired:  true,
					ApprovalPolicyKey: "two_person",
				},
			},
		}

		resp, err := svc.handleRegisterRequest(context.Background(), req, "")
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Empty(t, resp.Warnings)

		// Verify agent was registered
		svc.registry.Mu().RLock()
		agent := svc.registry.AgentsUnsafe()["agent-1"]
		svc.registry.Mu().RUnlock()
		assert.NotNil(t, agent)
		assert.Equal(t, "game-1", agent.GameID)
		assert.Equal(t, "prod", agent.Env)
		meta := agent.Functions["game.player.get"]
		assert.True(t, meta.ApprovalRequired)
		assert.Equal(t, "two_person", meta.ApprovalPolicyKey)
	})

	t.Run("with custom TTL", func(t *testing.T) {
		svc := newTestControlService()

		req := &agentv1.RegisterRequest{
			AgentId:    "agent-1",
			GameId:     "game-1",
			TtlSeconds: 600,
		}

		resp, err := svc.handleRegisterRequest(context.Background(), req, "")
		require.NoError(t, err)
		assert.NotNil(t, resp)

		svc.registry.Mu().RLock()
		agent := svc.registry.AgentsUnsafe()["agent-1"]
		svc.registry.Mu().RUnlock()
		assert.NotNil(t, agent)
		// TTL should be around 10 minutes
		assert.True(t, agent.ExpireAt.After(time.Now().Add(9*time.Minute)))
	})

	t.Run("with invalid functions", func(t *testing.T) {
		svc := newTestControlService()

		req := &agentv1.RegisterRequest{
			AgentId: "agent-1",
			GameId:  "game-1",
			Functions: []*agentv1.FunctionDescriptor{
				{
					Id:      "", // Empty ID
					Version: "1.0.0",
				},
				{
					Id:      "invalid id!", // Invalid format
					Version: "1.0.0",
				},
			},
		}

		resp, err := svc.handleRegisterRequest(context.Background(), req, "")
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Warnings)
	})

	t.Run("with processes", func(t *testing.T) {
		svc := newTestControlService()

		req := &agentv1.RegisterRequest{
			AgentId: "agent-1",
			GameId:  "game-1",
			Processes: []*agentv1.AgentProcess{
				{
					ServiceId:    "svc-1",
					Addr:         "localhost:8080",
					Version:      "1.0.0",
					FunctionIds:  []string{"game.player.get"},
					LastSeenUnix: time.Now().Unix(),
				},
			},
		}

		resp, err := svc.handleRegisterRequest(context.Background(), req, "")
		require.NoError(t, err)
		assert.NotNil(t, resp)

		svc.registry.Mu().RLock()
		agent := svc.registry.AgentsUnsafe()["agent-1"]
		svc.registry.Mu().RUnlock()
		assert.NotNil(t, agent)
		assert.Len(t, agent.Providers, 1)
		assert.Equal(t, "svc-1", agent.Providers[0].ProviderID)
	})

	t.Run("with upstream handler", func(t *testing.T) {
		svc := newTestControlService()
		mockHandler := &mockHandler{
			registerResp: &agentv1.RegisterResponse{Warnings: []string{"upstream warning"}},
		}
		svc.SetUpstreamHandler(mockHandler)

		req := &agentv1.RegisterRequest{
			AgentId: "agent-1",
			GameId:  "game-1",
		}

		resp, err := svc.handleRegisterRequest(context.Background(), req, "")
		require.NoError(t, err)
		assert.Equal(t, []string{"upstream warning"}, resp.Warnings)
	})

	t.Run("with database loader", func(t *testing.T) {
		loader := &mockAgentSessionLoader{}
		svc := newTestControlServiceWithLoader(loader)

		req := &agentv1.RegisterRequest{
			AgentId: "agent-1",
			GameId:  "game-1",
		}

		resp, err := svc.handleRegisterRequest(context.Background(), req, "")
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, int32(1), atomic.LoadInt32(&loader.upsertCalled))
	})

	t.Run("with functions having display name and summary", func(t *testing.T) {
		svc := newTestControlService()

		req := &agentv1.RegisterRequest{
			AgentId: "agent-1",
			GameId:  "game-1",
			Functions: []*agentv1.FunctionDescriptor{
				{
					Id:           "game.player.get",
					Version:      "1.0.0",
					Enabled:      true,
					Resource:     "player",
					Risk:         "safe",
					Operation:    "get",
					Permission:   "player.get",
					Summary:      "Get player",
					Description:  "Read a player profile",
					InputSchema:  `{"type":"object","properties":{"player_id":{"type":"string"}}}`,
					OutputSchema: `{"type":"object","properties":{"name":{"type":"string"}}}`,
				},
			},
		}

		resp, err := svc.handleRegisterRequest(context.Background(), req, "")
		require.NoError(t, err)
		assert.NotNil(t, resp)

		op, err := svc.registry.GetOpenAPI("game.player.get")
		require.NoError(t, err)
		assert.Equal(t, "Get player", op.Summary)
		assert.Equal(t, "Read a player profile", op.Description)
		assert.Equal(t, "player", op.Extensions["x-resource"])
		assert.Equal(t, "safe", op.Extensions["x-risk"])
		assert.Equal(t, "get", op.Extensions["x-operation"])
		assert.Equal(t, "player.get", op.Extensions["x-permission"])
		require.NotNil(t, op.RequestBody)
		require.NotNil(t, op.Responses)
	})

	t.Run("preserves descriptor v2 metadata in registry and openapi", func(t *testing.T) {
		svc := newTestControlService()

		req := &agentv1.RegisterRequest{
			AgentId: "agent-1",
			GameId:  "game-1",
			Env:     "prod",
			Version: "1.0.0",
			Functions: []*agentv1.FunctionDescriptor{
				{
					Id:           "player.ban",
					Version:      "1.2.3",
					Enabled:      true,
					Tags:         []string{"player", "moderation"},
					Summary:      "Ban player",
					Description:  "Ban a player account",
					Resource:     "player",
					Risk:         "danger",
					Operation:    "ban",
					Permission:   "player.ban",
					InputSchema:  `{"type":"object","properties":{"player_id":{"type":"string"}}}`,
					OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
				},
			},
		}

		resp, err := svc.handleRegisterRequest(context.Background(), req, "")
		require.NoError(t, err)
		require.Empty(t, resp.Warnings)

		svc.registry.Mu().RLock()
		agent := svc.registry.AgentsUnsafe()["agent-1"]
		require.NotNil(t, agent)
		meta := agent.Functions["player.ban"]
		svc.registry.Mu().RUnlock()

		assert.True(t, meta.Enabled)
		assert.Equal(t, "1.2.3", meta.Version)
		assert.Equal(t, []string{"player", "moderation"}, meta.Tags)
		assert.Equal(t, "Ban player", meta.Summary)
		assert.Equal(t, "Ban a player account", meta.Description)
		assert.Equal(t, "player", meta.Resource)
		assert.Equal(t, "danger", meta.Risk)
		assert.Equal(t, "ban", meta.Operation)
		assert.Equal(t, "player.ban", meta.Permission)
		assert.Contains(t, meta.InputSchema, "player_id")
		assert.Contains(t, meta.OutputSchema, "success")

		op, err := svc.registry.GetOpenAPI("player.ban")
		require.NoError(t, err)
		assert.Equal(t, "player", op.Extensions["x-resource"])
		assert.Equal(t, "ban", op.Extensions["x-operation"])
		assert.Equal(t, "danger", op.Extensions["x-risk"])
		assert.Equal(t, "player.ban", op.Extensions["x-permission"])
	})

	t.Run("with nil process in processes list", func(t *testing.T) {
		svc := newTestControlService()

		req := &agentv1.RegisterRequest{
			AgentId: "agent-1",
			GameId:  "game-1",
			Processes: []*agentv1.AgentProcess{
				nil,
				{
					ServiceId: "svc-1",
					Addr:      "localhost:8080",
				},
			},
		}

		resp, err := svc.handleRegisterRequest(context.Background(), req, "")
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("with empty service ID in process", func(t *testing.T) {
		svc := newTestControlService()

		req := &agentv1.RegisterRequest{
			AgentId: "agent-1",
			GameId:  "game-1",
			Processes: []*agentv1.AgentProcess{
				{
					ServiceId: "", // Empty service ID
					Addr:      "localhost:8080",
				},
			},
		}

		resp, err := svc.handleRegisterRequest(context.Background(), req, "")
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("with database loader error", func(t *testing.T) {
		loader := &mockAgentSessionLoader{
			upsertErr: fmt.Errorf("database error"),
		}
		svc := newTestControlServiceWithLoader(loader)

		req := &agentv1.RegisterRequest{
			AgentId: "agent-1",
			GameId:  "game-1",
		}

		// Should not fail even if database write fails
		resp, err := svc.handleRegisterRequest(context.Background(), req, "")
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("returns error when dashboard contract rebuild fails", func(t *testing.T) {
		svc := newTestControlService()
		svc.registry.SetContractService(failingRegisterContractService{err: fmt.Errorf("rebuild failed")})

		req := &agentv1.RegisterRequest{
			AgentId: "agent-1",
			GameId:  "game-1",
			Env:     "prod",
			Functions: []*agentv1.FunctionDescriptor{
				{
					Id:      "player.query",
					Version: "1.0.0",
					Enabled: true,
				},
			},
		}

		resp, err := svc.handleRegisterRequest(context.Background(), req, "")
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.ErrorContains(t, err, "register agent dashboard contract rebuild failed")

		svc.registry.Mu().RLock()
		agent := svc.registry.AgentsUnsafe()["agent-1"]
		svc.registry.Mu().RUnlock()
		assert.Nil(t, agent)
	})
}

// --- Tests for handleHeartbeatRequest ---

func TestControlService_HandleHeartbeatRequest(t *testing.T) {
	t.Run("heartbeat for existing agent", func(t *testing.T) {
		svc := newTestControlService()

		// Register agent first
		agent := &registry.AgentSession{
			AgentID:  "agent-1",
			GameID:   "game-1",
			ExpireAt: time.Now().Add(5 * time.Minute),
			LastSeen: time.Now().Add(-1 * time.Minute),
		}
		svc.registry.UpsertAgent(agent)

		req := &agentv1.HeartbeatRequest{
			AgentId: "agent-1",
		}

		resp, err := svc.handleHeartbeatRequest(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, resp)

		// Verify agent was updated
		svc.registry.Mu().RLock()
		updatedAgent := svc.registry.AgentsUnsafe()["agent-1"]
		svc.registry.Mu().RUnlock()
		assert.NotNil(t, updatedAgent)
		assert.True(t, updatedAgent.LastSeen.After(time.Now().Add(-5*time.Second)))
	})

	t.Run("heartbeat for non-existing agent", func(t *testing.T) {
		svc := newTestControlService()

		req := &agentv1.HeartbeatRequest{
			AgentId: "non-existing",
		}

		resp, err := svc.handleHeartbeatRequest(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("heartbeat with empty agent ID", func(t *testing.T) {
		svc := newTestControlService()

		req := &agentv1.HeartbeatRequest{
			AgentId: "",
		}

		resp, err := svc.handleHeartbeatRequest(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("heartbeat with upstream handler", func(t *testing.T) {
		svc := newTestControlService()
		mockHandler := &mockHandler{
			heartbeatResp: &agentv1.HeartbeatResponse{},
		}
		svc.SetUpstreamHandler(mockHandler)

		req := &agentv1.HeartbeatRequest{
			AgentId: "agent-1",
		}

		resp, err := svc.handleHeartbeatRequest(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 1, mockHandler.heartbeatCalled)
	})
}

// --- Tests for handleRegisterCapabilitiesRequest ---

func TestControlService_HandleRegisterCapabilitiesRequest(t *testing.T) {
	t.Run("successful registration", func(t *testing.T) {
		svc := newTestControlService()

		manifest := []byte(`{"openapi":"3.0.3","info":{"title":"Test"}}`)
		compressed, err := compressGzip(manifest)
		require.NoError(t, err)

		req := &agentv1.RegisterCapabilitiesRequest{
			Provider: &agentv1.ProviderMeta{
				Id:      "provider-1",
				Version: "1.0.0",
				Lang:    "go",
				Sdk:     "1.0.0",
			},
			ManifestJsonGz: compressed,
		}

		resp, err := svc.handleRegisterCapabilitiesRequest(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("with nil provider", func(t *testing.T) {
		svc := newTestControlService()

		req := &agentv1.RegisterCapabilitiesRequest{
			Provider: nil,
		}

		_, err := svc.handleRegisterCapabilitiesRequest(context.Background(), req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider metadata is required")
	})

	t.Run("with empty provider ID", func(t *testing.T) {
		svc := newTestControlService()

		req := &agentv1.RegisterCapabilitiesRequest{
			Provider: &agentv1.ProviderMeta{
				Id: "",
			},
		}

		_, err := svc.handleRegisterCapabilitiesRequest(context.Background(), req)
		assert.Error(t, err)
	})

	t.Run("with invalid gzip data", func(t *testing.T) {
		svc := newTestControlService()

		req := &agentv1.RegisterCapabilitiesRequest{
			Provider: &agentv1.ProviderMeta{
				Id: "provider-1",
			},
			ManifestJsonGz: []byte("invalid gzip"),
		}

		_, err := svc.handleRegisterCapabilitiesRequest(context.Background(), req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid manifest (gzip)")
	})

	t.Run("with empty manifest data", func(t *testing.T) {
		svc := newTestControlService()

		req := &agentv1.RegisterCapabilitiesRequest{
			Provider: &agentv1.ProviderMeta{
				Id: "provider-1",
			},
			ManifestJsonGz: []byte{},
		}

		_, err := svc.handleRegisterCapabilitiesRequest(context.Background(), req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "manifest data is empty")
	})

	t.Run("with upstream handler", func(t *testing.T) {
		svc := newTestControlService()
		mockHandler := &mockHandler{
			capabilitiesResp: &agentv1.RegisterCapabilitiesResponse{},
		}
		svc.SetUpstreamHandler(mockHandler)

		req := &agentv1.RegisterCapabilitiesRequest{
			Provider: &agentv1.ProviderMeta{
				Id: "provider-1",
			},
		}

		resp, err := svc.handleRegisterCapabilitiesRequest(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 1, mockHandler.capabilitiesCalled)
	})
}

// --- Tests for handleTaskEvent ---

func TestControlService_HandleTaskEvent(t *testing.T) {
	t.Run("without task store", func(t *testing.T) {
		svc := newTestControlService()

		data, _ := proto.Marshal(&sdkv1.TaskEvent{
			TaskId: "task-1",
			Type:   string(tasks.EventStarted),
		})

		_, err := svc.handleTaskEvent(context.Background(), data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task store not configured")
	})

	t.Run("empty task ID", func(t *testing.T) {
		svc := newTestControlService()
		svc.SetTaskStore(&mockTaskStore{})

		data, _ := proto.Marshal(&sdkv1.TaskEvent{
			TaskId: "",
			Type:   string(tasks.EventStarted),
		})

		_, err := svc.handleTaskEvent(context.Background(), data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task_id is required")
	})

	t.Run("invalid protobuf data", func(t *testing.T) {
		svc := newTestControlService()
		svc.SetTaskStore(&mockTaskStore{})

		_, err := svc.handleTaskEvent(context.Background(), []byte("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal TaskEvent")
	})

	t.Run("event started", func(t *testing.T) {
		mock := &mockTaskStore{}
		svc := newTestControlService()
		svc.SetTaskStore(mock)

		data, _ := proto.Marshal(&sdkv1.TaskEvent{
			TaskId:   "task-1",
			Type:     string(tasks.EventStarted),
			Progress: 0,
			Message:  "starting",
		})

		resp, err := svc.handleTaskEvent(context.Background(), data)
		assert.NoError(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, 1, mock.updateRunCalled)
		assert.Equal(t, 1, mock.appendCalled)
		assert.Equal(t, tasks.StatusRunning, mock.lastUpdates["status"])
		assert.NotNil(t, mock.lastUpdates["started_at"])
	})

	t.Run("event progress", func(t *testing.T) {
		mock := &mockTaskStore{}
		svc := newTestControlService()
		svc.SetTaskStore(mock)

		data, _ := proto.Marshal(&sdkv1.TaskEvent{
			TaskId:   "task-2",
			Type:     string(tasks.EventProgress),
			Progress: 50,
			Message:  "halfway",
		})

		resp, err := svc.handleTaskEvent(context.Background(), data)
		assert.NoError(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, tasks.StatusRunning, mock.lastUpdates["status"])
	})

	t.Run("event log", func(t *testing.T) {
		mock := &mockTaskStore{}
		svc := newTestControlService()
		svc.SetTaskStore(mock)

		data, _ := proto.Marshal(&sdkv1.TaskEvent{
			TaskId:  "task-3",
			Type:    string(tasks.EventLog),
			Message: "log entry",
		})

		resp, err := svc.handleTaskEvent(context.Background(), data)
		assert.NoError(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, tasks.StatusRunning, mock.lastUpdates["status"])
	})

	t.Run("event completed", func(t *testing.T) {
		mock := &mockTaskStore{}
		svc := newTestControlService()
		svc.SetTaskStore(mock)

		data, _ := proto.Marshal(&sdkv1.TaskEvent{
			TaskId:   "task-4",
			Type:     string(tasks.EventCompleted),
			Progress: 90,
			Message:  "done",
			Payload:  []byte(`{"result":"ok"}`),
		})

		resp, err := svc.handleTaskEvent(context.Background(), data)
		assert.NoError(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, tasks.StatusSucceeded, mock.lastUpdates["status"])
		assert.Equal(t, int32(100), mock.lastUpdates["progress"])
		assert.NotNil(t, mock.lastUpdates["finished_at"])
	})

	t.Run("event failed", func(t *testing.T) {
		mock := &mockTaskStore{}
		svc := newTestControlService()
		svc.SetTaskStore(mock)

		data, _ := proto.Marshal(&sdkv1.TaskEvent{
			TaskId:  "task-5",
			Type:    string(tasks.EventFailed),
			Message: "something broke",
		})

		resp, err := svc.handleTaskEvent(context.Background(), data)
		assert.NoError(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, tasks.StatusFailed, mock.lastUpdates["status"])
		assert.NotNil(t, mock.lastUpdates["finished_at"])
		assert.Equal(t, "something broke", mock.lastUpdates["error_message"])
	})

	t.Run("event cancel requested", func(t *testing.T) {
		mock := &mockTaskStore{}
		svc := newTestControlService()
		svc.SetTaskStore(mock)

		data, _ := proto.Marshal(&sdkv1.TaskEvent{
			TaskId:  "task-6",
			Type:    string(tasks.EventCancelRequested),
			Message: "user cancelled",
		})

		resp, err := svc.handleTaskEvent(context.Background(), data)
		assert.NoError(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, tasks.StatusCancelRequested, mock.lastUpdates["status"])
		assert.NotNil(t, mock.lastUpdates["cancel_requested_at"])
	})

	t.Run("event cancelled", func(t *testing.T) {
		mock := &mockTaskStore{}
		svc := newTestControlService()
		svc.SetTaskStore(mock)

		data, _ := proto.Marshal(&sdkv1.TaskEvent{
			TaskId:  "task-7",
			Type:    string(tasks.EventCancelled),
			Message: "cancelled",
		})

		resp, err := svc.handleTaskEvent(context.Background(), data)
		assert.NoError(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, tasks.StatusCancelled, mock.lastUpdates["status"])
		assert.NotNil(t, mock.lastUpdates["finished_at"])
	})

	t.Run("unknown event type defaults to running", func(t *testing.T) {
		mock := &mockTaskStore{}
		svc := newTestControlService()
		svc.SetTaskStore(mock)

		data, _ := proto.Marshal(&sdkv1.TaskEvent{
			TaskId:  "task-8",
			Type:    "unknown_type",
			Message: "something",
		})

		resp, err := svc.handleTaskEvent(context.Background(), data)
		assert.NoError(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, tasks.StatusRunning, mock.lastUpdates["status"])
	})

	t.Run("update run error", func(t *testing.T) {
		mock := &mockTaskStore{updateRunErr: fmt.Errorf("db error")}
		svc := newTestControlService()
		svc.SetTaskStore(mock)

		data, _ := proto.Marshal(&sdkv1.TaskEvent{
			TaskId: "task-9",
			Type:   string(tasks.EventStarted),
		})

		_, err := svc.handleTaskEvent(context.Background(), data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "update task run")
	})

	t.Run("append event error", func(t *testing.T) {
		mock := &mockTaskStore{appendEventErr: fmt.Errorf("append error")}
		svc := newTestControlService()
		svc.SetTaskStore(mock)

		data, _ := proto.Marshal(&sdkv1.TaskEvent{
			TaskId: "task-10",
			Type:   string(tasks.EventStarted),
		})

		_, err := svc.handleTaskEvent(context.Background(), data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "append task event")
	})

	t.Run("whitespace task ID trimmed", func(t *testing.T) {
		mock := &mockTaskStore{}
		svc := newTestControlService()
		svc.SetTaskStore(mock)

		data, _ := proto.Marshal(&sdkv1.TaskEvent{
			TaskId: "   task-11   ",
			Type:   string(tasks.EventStarted),
		})

		resp, err := svc.handleTaskEvent(context.Background(), data)
		assert.NoError(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, "task-11", mock.lastTaskID)
	})
}

// --- Tests for handleRequest ---

func TestControlService_HandleRequest(t *testing.T) {
	svc := newTestControlService()

	t.Run("register request", func(t *testing.T) {
		req := &agentv1.RegisterRequest{
			AgentId: "agent-1",
			GameId:  "game-1",
		}
		data, _ := proto.Marshal(req)

		resp, err := svc.handleRequest(context.Background(), protocol.MsgRegisterRequest, data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("heartbeat request", func(t *testing.T) {
		req := &agentv1.HeartbeatRequest{
			AgentId: "agent-1",
		}
		data, _ := proto.Marshal(req)

		resp, err := svc.handleRequest(context.Background(), protocol.MsgHeartbeatRequest, data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("register capabilities request", func(t *testing.T) {
		manifest := []byte(`{"openapi":"3.0.3"}`)
		compressed, _ := compressGzip(manifest)

		req := &agentv1.RegisterCapabilitiesRequest{
			Provider: &agentv1.ProviderMeta{
				Id: "provider-1",
			},
			ManifestJsonGz: compressed,
		}
		data, _ := proto.Marshal(req)

		resp, err := svc.handleRequest(context.Background(), protocol.MsgRegisterCapabilitiesReq, data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("task event without store", func(t *testing.T) {
		req := &sdkv1.TaskEvent{
			TaskId: "task-1",
			Type:   "started",
		}
		data, _ := proto.Marshal(req)

		_, err := svc.handleRequest(context.Background(), protocol.MsgTaskEvent, data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task store not configured")
	})

	t.Run("unknown message type", func(t *testing.T) {
		_, err := svc.handleRequest(context.Background(), 0xFFFFFF, []byte{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown message type")
	})
}

// --- Tests for handleMetricEvent ---

func TestHandleMetricEvent(t *testing.T) {
	svc := &ControlService{logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))}

	t.Run("valid report", func(t *testing.T) {
		req := &opsv1.MetricsReport{
			AgentId: "agent-1",
			Cpu:     &opsv1.CpuMetrics{UsagePercent: 12.5, Cores: 4},
			Memory:  &opsv1.MemoryMetrics{TotalBytes: 1000, UsedBytes: 500},
		}
		data, _ := proto.Marshal(req)

		resp, err := svc.handleMetricEvent(context.Background(), data)
		require.NoError(t, err)
		assert.Nil(t, resp, "metric event is one-way, response body should be nil")
	})

	t.Run("missing agent id", func(t *testing.T) {
		req := &opsv1.MetricsReport{AgentId: ""}
		data, _ := proto.Marshal(req)

		_, err := svc.handleMetricEvent(context.Background(), data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "agent_id is required")
	})

	t.Run("invalid payload", func(t *testing.T) {
		_, err := svc.handleMetricEvent(context.Background(), []byte("not protobuf"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal MetricsReport")
	})

	t.Run("dispatched via handleRequest", func(t *testing.T) {
		req := &opsv1.MetricsReport{AgentId: "agent-via-dispatch"}
		data, _ := proto.Marshal(req)

		resp, err := svc.handleRequest(context.Background(), protocol.MsgMetricEvent, data)
		require.NoError(t, err)
		assert.Nil(t, resp)
	})
}

// --- Tests for validateAndNormalizeFunctions ---

func TestValidateAndNormalizeFunctions(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		functions, warnings := validateAndNormalizeFunctions(nil)
		assert.Nil(t, functions)
		assert.Nil(t, warnings)
	})

	t.Run("valid functions", func(t *testing.T) {
		items := []*agentv1.FunctionDescriptor{
			{
				Id:      "game.player.get",
				Version: "1.0.0",
				Enabled: true,
			},
			{
				Id:      "game.player.update",
				Version: "2.0.0",
				Enabled: true,
			},
		}

		functions, warnings := validateAndNormalizeFunctions(items)
		assert.Len(t, functions, 2)
		assert.Empty(t, warnings)
	})

	t.Run("nil function in list", func(t *testing.T) {
		items := []*agentv1.FunctionDescriptor{
			nil,
			{
				Id:      "game.player.get",
				Version: "1.0.0",
			},
		}

		functions, warnings := validateAndNormalizeFunctions(items)
		assert.Len(t, functions, 1)
		assert.Len(t, warnings, 1)
		assert.Equal(t, "nil_function", warnings[0].Code)
	})

	t.Run("empty function ID", func(t *testing.T) {
		items := []*agentv1.FunctionDescriptor{
			{
				Id:      "",
				Version: "1.0.0",
			},
		}

		functions, warnings := validateAndNormalizeFunctions(items)
		assert.Empty(t, functions)
		assert.Len(t, warnings, 1)
		assert.Equal(t, "empty_function_id", warnings[0].Code)
	})

	t.Run("invalid function ID format", func(t *testing.T) {
		items := []*agentv1.FunctionDescriptor{
			{
				Id:      "Invalid ID!",
				Version: "1.0.0",
			},
		}

		functions, warnings := validateAndNormalizeFunctions(items)
		assert.Empty(t, functions)
		assert.Len(t, warnings, 1)
		assert.Equal(t, "invalid_function_id", warnings[0].Code)
	})

	t.Run("invalid version", func(t *testing.T) {
		items := []*agentv1.FunctionDescriptor{
			{
				Id:      "game.player.get",
				Version: "invalid",
			},
		}

		functions, warnings := validateAndNormalizeFunctions(items)
		assert.Empty(t, functions)
		assert.Len(t, warnings, 1)
		assert.Equal(t, "invalid_version", warnings[0].Code)
	})

	t.Run("duplicate function IDs - keep higher version", func(t *testing.T) {
		items := []*agentv1.FunctionDescriptor{
			{
				Id:      "game.player.get",
				Version: "1.0.0",
			},
			{
				Id:      "game.player.get",
				Version: "2.0.0",
			},
		}

		functions, warnings := validateAndNormalizeFunctions(items)
		assert.Len(t, functions, 1)
		assert.Equal(t, "2.0.0", functions[0].Version)
		assert.Len(t, warnings, 1)
		assert.Equal(t, "duplicate_function_id", warnings[0].Code)
	})

	t.Run("duplicate function IDs - keep existing higher version", func(t *testing.T) {
		items := []*agentv1.FunctionDescriptor{
			{
				Id:      "game.player.get",
				Version: "2.0.0",
			},
			{
				Id:      "game.player.get",
				Version: "1.0.0",
			},
		}

		functions, warnings := validateAndNormalizeFunctions(items)
		assert.Len(t, functions, 1)
		assert.Equal(t, "2.0.0", functions[0].Version)
		assert.Len(t, warnings, 1)
	})

	t.Run("case normalization", func(t *testing.T) {
		items := []*agentv1.FunctionDescriptor{
			{
				Id:      "Game.Player.Get",
				Version: "1.0.0",
			},
		}

		functions, warnings := validateAndNormalizeFunctions(items)
		assert.Len(t, functions, 1)
		assert.Equal(t, "game.player.get", functions[0].Id)
		assert.Empty(t, warnings)
	})
}

// --- Tests for isValidSemver ---

func TestIsValidSemver(t *testing.T) {
	tests := []struct {
		version  string
		expected bool
	}{
		{"1.0.0", true},
		{"v1.0.0", true},
		{"0.1.0", true},
		{"1.0.0-alpha", true},
		{"1.0.0+build", true},
		{"1.0.0-alpha.1", true},
		{"invalid", false},
		{"1", false},
		{"1.0", false},
		{"1.0.0.0", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			result := isValidSemver(tt.version)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- Tests for compareSemver ---

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		a, b     string
		expected int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},
		{"1.0.0", "1.1.0", -1},
		{"1.1.0", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"v1.0.0", "1.0.0", 0},
		{"1.0.0-alpha", "1.0.0", 0}, // Pre-release is ignored in comparison
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_vs_%s", tt.a, tt.b), func(t *testing.T) {
			result := compareSemver(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- Tests for decompressManifest ---

func TestDecompressManifest(t *testing.T) {
	t.Run("valid gzip data", func(t *testing.T) {
		original := []byte(`{"key":"value"}`)
		compressed, err := compressGzip(original)
		require.NoError(t, err)

		result, err := decompressManifest(compressed)
		require.NoError(t, err)
		assert.Equal(t, original, result)
	})

	t.Run("empty data", func(t *testing.T) {
		_, err := decompressManifest([]byte{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "manifest data is empty")
	})

	t.Run("invalid gzip data", func(t *testing.T) {
		_, err := decompressManifest([]byte("not gzip"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create gzip reader")
	})
}

// --- Tests for ControlHandler ---

func TestControlHandler(t *testing.T) {
	svc := newTestControlService()
	handler := NewControlHandler(svc)

	t.Run("HandleRegister", func(t *testing.T) {
		req := &agentv1.RegisterRequest{
			AgentId: "agent-1",
			GameId:  "game-1",
		}

		resp, err := handler.HandleRegister(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("HandleHeartbeat", func(t *testing.T) {
		req := &agentv1.HeartbeatRequest{
			AgentId: "agent-1",
		}

		resp, err := handler.HandleHeartbeat(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("HandleRegisterCapabilities", func(t *testing.T) {
		manifest := []byte(`{"openapi":"3.0.3"}`)
		compressed, _ := compressGzip(manifest)

		req := &agentv1.RegisterCapabilitiesRequest{
			Provider: &agentv1.ProviderMeta{
				Id: "provider-1",
			},
			ManifestJsonGz: compressed,
		}

		resp, err := handler.HandleRegisterCapabilities(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

// --- Tests for LoadAgentSessions ---

func TestControlService_LoadAgentSessions(t *testing.T) {
	t.Run("without loader", func(t *testing.T) {
		svc := newTestControlService()
		err := svc.LoadAgentSessions()
		assert.NoError(t, err)
	})

	t.Run("with loader error", func(t *testing.T) {
		loader := &mockAgentSessionLoader{
			loadErr: fmt.Errorf("database error"),
		}
		svc := newTestControlServiceWithLoader(loader)

		err := svc.LoadAgentSessions()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load agent sessions")
	})

	t.Run("with loader returning sessions", func(t *testing.T) {
		loader := &mockAgentSessionLoader{
			sessions: []*registry.AgentSession{
				{AgentID: "agent-1", GameID: "game-1"},
				{AgentID: "agent-2", GameID: "game-2"},
			},
		}
		svc := newTestControlServiceWithLoader(loader)

		// This will fail because the registry doesn't have a database
		// but it exercises the loader path
		_ = svc.LoadAgentSessions()
	})
}

// --- Tests for StartBackgroundTasks ---

func TestControlService_StartBackgroundTasks(t *testing.T) {
	t.Run("starts only once", func(t *testing.T) {
		svc := newTestControlService()

		// Should not panic
		svc.StartBackgroundTasks()
		svc.StartBackgroundTasks() // Second call should be no-op

		// Clean up
		svc.Stop()
	})

	t.Run("with loader starts cleanup loop", func(t *testing.T) {
		loader := &mockAgentSessionLoader{}
		svc := newTestControlServiceWithLoader(loader)

		svc.StartBackgroundTasks()
		// Give goroutines time to start
		time.Sleep(50 * time.Millisecond)

		svc.Stop()
	})
}

// --- Tests for TransportHandler ---

func TestControlService_TransportHandler(t *testing.T) {
	t.Run("returns valid handler", func(t *testing.T) {
		svc := newTestControlService()
		handler := svc.TransportHandler()
		assert.NotNil(t, handler)
	})

	t.Run("handler dispatches register request", func(t *testing.T) {
		svc := newTestControlService()
		handler := svc.TransportHandler()

		req := &agentv1.RegisterRequest{
			AgentId: "agent-1",
			GameId:  "game-1",
		}
		data, _ := proto.Marshal(req)

		resp, err := handler.Handle(context.Background(), protocol.MsgRegisterRequest, 1, data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("handler dispatches heartbeat request", func(t *testing.T) {
		svc := newTestControlService()
		handler := svc.TransportHandler()

		req := &agentv1.HeartbeatRequest{
			AgentId: "agent-1",
		}
		data, _ := proto.Marshal(req)

		resp, err := handler.Handle(context.Background(), protocol.MsgHeartbeatRequest, 1, data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("handler returns error for unknown message", func(t *testing.T) {
		svc := newTestControlService()
		handler := svc.TransportHandler()

		_, err := handler.Handle(context.Background(), 0xFFFFFF, 1, []byte{})
		assert.Error(t, err)
	})
}

// --- Tests for handleRegisterCapabilities (byte-level) ---

func TestControlService_HandleRegisterCapabilitiesBytes(t *testing.T) {
	t.Run("valid capabilities request", func(t *testing.T) {
		svc := newTestControlService()

		manifest := []byte(`{"openapi":"3.0.3","info":{"title":"Test"}}`)
		compressed, _ := compressGzip(manifest)

		req := &agentv1.RegisterCapabilitiesRequest{
			Provider: &agentv1.ProviderMeta{
				Id:      "provider-1",
				Version: "1.0.0",
			},
			ManifestJsonGz: compressed,
		}
		data, _ := proto.Marshal(req)

		resp, err := svc.handleRegisterCapabilities(context.Background(), data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("invalid protobuf data", func(t *testing.T) {
		svc := newTestControlService()

		_, err := svc.handleRegisterCapabilities(context.Background(), []byte("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal RegisterCapabilitiesRequest")
	})
}

// --- Tests for handleRegister (byte-level) ---

func TestControlService_HandleRegisterBytes(t *testing.T) {
	t.Run("valid register request", func(t *testing.T) {
		svc := newTestControlService()

		req := &agentv1.RegisterRequest{
			AgentId: "agent-1",
			GameId:  "game-1",
		}
		data, _ := proto.Marshal(req)

		resp, err := svc.handleRegister(context.Background(), data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("invalid protobuf data", func(t *testing.T) {
		svc := newTestControlService()

		_, err := svc.handleRegister(context.Background(), []byte("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal RegisterRequest")
	})
}

// --- Tests for handleHeartbeat (byte-level) ---

func TestControlService_HandleHeartbeatBytes(t *testing.T) {
	t.Run("valid heartbeat request", func(t *testing.T) {
		svc := newTestControlService()

		req := &agentv1.HeartbeatRequest{
			AgentId: "agent-1",
		}
		data, _ := proto.Marshal(req)

		resp, err := svc.handleHeartbeat(context.Background(), data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("invalid protobuf data", func(t *testing.T) {
		svc := newTestControlService()

		_, err := svc.handleHeartbeat(context.Background(), []byte("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal HeartbeatRequest")
	})
}

// --- Tests for pruneOldMetrics ---

func TestControlService_PruneOldMetrics(t *testing.T) {
	t.Run("exits on context cancel", func(t *testing.T) {
		svc := newTestControlService()

		done := make(chan struct{})
		go func() {
			svc.pruneOldMetrics()
			close(done)
		}()

		// Cancel context to stop the loop
		svc.cancel()

		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatal("pruneOldMetrics did not exit after context cancel")
		}
	})
}

// --- Tests for handleHeartbeatRequest with database loader ---

func TestControlService_HandleHeartbeatRequest_WithLoader(t *testing.T) {
	t.Run("heartbeat updates database", func(t *testing.T) {
		loader := newMockAgentSessionLoader()
		svc := newTestControlServiceWithLoader(loader)

		// Register agent first
		agent := &registry.AgentSession{
			AgentID:  "agent-1",
			GameID:   "game-1",
			ExpireAt: time.Now().Add(5 * time.Minute),
			LastSeen: time.Now().Add(-1 * time.Minute),
		}
		svc.registry.UpsertAgent(agent)

		req := &agentv1.HeartbeatRequest{
			AgentId: "agent-1",
		}

		resp, err := svc.handleHeartbeatRequest(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, resp)

		// Wait for async goroutine to complete
		select {
		case <-loader.upsertDone:
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for upsert goroutine")
		}
		assert.Equal(t, int32(1), atomic.LoadInt32(&loader.upsertCalled))
	})
}

// --- Tests for cleanupLoop ---

func TestControlService_CleanupLoop(t *testing.T) {
	t.Run("exits on context cancel", func(t *testing.T) {
		loader := &mockAgentSessionLoader{}
		svc := newTestControlServiceWithLoader(loader)

		done := make(chan struct{})
		go func() {
			svc.cleanupLoop()
			close(done)
		}()

		// Cancel context to stop the loop
		svc.cancel()

		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatal("cleanupLoop did not exit after context cancel")
		}
	})
}

// --- Mock Handler ---

type mockHandler struct {
	registerResp       *agentv1.RegisterResponse
	heartbeatResp      *agentv1.HeartbeatResponse
	capabilitiesResp   *agentv1.RegisterCapabilitiesResponse
	registerErr        error
	heartbeatErr       error
	capabilitiesErr    error
	registerCalled     int
	heartbeatCalled    int
	capabilitiesCalled int
}

func (m *mockHandler) HandleRegister(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	m.registerCalled++
	return m.registerResp, m.registerErr
}

func (m *mockHandler) HandleHeartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	m.heartbeatCalled++
	return m.heartbeatResp, m.heartbeatErr
}

func (m *mockHandler) HandleRegisterCapabilities(ctx context.Context, req *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error) {
	m.capabilitiesCalled++
	return m.capabilitiesResp, m.capabilitiesErr
}

type failingRegisterContractService struct {
	err error
}

func (f failingRegisterContractService) RebuildContractFromFunctionMeta(context.Context, string, string, string, spec.FunctionContractInput) error {
	return f.err
}

func (f failingRegisterContractService) RebuildResourceCapability(context.Context, string, string, string) error {
	return f.err
}

func (f failingRegisterContractService) RebuildProposalsForResource(context.Context, string, string, string) error {
	return f.err
}

func (f failingRegisterContractService) RebuildProposalForFunction(context.Context, string, string, string) error {
	return f.err
}
