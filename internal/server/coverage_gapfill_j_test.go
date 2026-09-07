package server

// 覆盖率补洞（gap-fill）：只针对 agent_session.go / control_handler.go /
// tcp_listener.go 中尚未覆盖的可达分支。不修改产品代码与既有测试。

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync/atomic"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"

	"github.com/cuihairu/croupier/internal/platform/registry"
	transportcore "github.com/cuihairu/croupier/internal/transport"
	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// badProto 是对 protobuf unmarshal 恒非法的载荷。
var badProto = []byte{0xff, 0xff, 0xff, 0xff, 0x7f}

// L311: handleRegister 的 proto.Unmarshal 失败。
func TestHandleRegister_BadProto(t *testing.T) {
	svc := newTestControlService()
	_, err := svc.handleRegister(context.Background(), badProto)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal RegisterRequest")
}

// L323: handleHeartbeat 的 proto.Unmarshal 失败。
func TestHandleHeartbeat_BadProto(t *testing.T) {
	svc := newTestControlService()
	_, err := svc.handleHeartbeat(context.Background(), badProto)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal HeartbeatRequest")
}

// L335: handleRegisterCapabilities 的 proto.Unmarshal 失败。
func TestHandleRegisterCapabilities_BadProto(t *testing.T) {
	svc := newTestControlService()
	_, err := svc.handleRegisterCapabilities(context.Background(), badProto)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal RegisterCapabilitiesRequest")
}

// L425-427: handleMetricEvent 将报告写入 metricsStore。
func TestHandleMetricEvent_StoresReport(t *testing.T) {
	svc := newTestControlService()
	report, err := proto.Marshal(&opsv1.MetricsReport{
		AgentId: "agent-metrics",
		Cpu:     &opsv1.CpuMetrics{UsagePercent: 42.5},
	})
	require.NoError(t, err)

	_, err = svc.handleMetricEvent(context.Background(), report)
	require.NoError(t, err)
	_, ok := svc.metricsStore.GetLatest("agent-metrics")
	assert.True(t, ok, "metrics report must be stored")
}

// L526-528: ToOpenAPIOperation 失败（非法 JSON schema 透传到转换器）。
func TestHandleRegisterRequest_OpenAPIConversionFails(t *testing.T) {
	svc := newTestControlService()
	_, err := svc.handleRegisterRequest(context.Background(), &agentv1.RegisterRequest{
		AgentId: "agent-openapi",
		GameId:  "game-1",
		Env:     "dev",
		Functions: []*agentv1.FunctionDescriptor{
			{
				Id:          "player.bad",
				Version:     "1.0.0",
				Enabled:     true,
				InputSchema: "{invalid",
			},
		},
	}, "127.0.0.1:1")
	// 注册本身不因 OpenAPI 转换失败而失败（只告警）。
	require.NoError(t, err)
}

// L547: schema diff 的非破坏性发现被 continue 跳过。
func TestHandleRegisterRequest_SchemaDiffNonBreakingFinding(t *testing.T) {
	svc := newTestControlService()
	ctx := context.Background()

	first := &agentv1.RegisterRequest{
		AgentId: "agent-diff",
		GameId:  "game-1",
		Env:     "dev",
		Functions: []*agentv1.FunctionDescriptor{
			{
				Id:          "player.list",
				Version:     "1.0.0",
				Enabled:     true,
				InputSchema: `{"type":"object","properties":{"page":{"type":"integer"}}}`,
			},
		},
	}
	_, err := svc.handleRegisterRequest(ctx, first, "")
	require.NoError(t, err)

	// 第二次注册：输入 schema 新增可选字段（非破坏性）→ finding 非
	// SeverityBreaking → L547 continue。
	second := &agentv1.RegisterRequest{
		AgentId: "agent-diff",
		GameId:  "game-1",
		Env:     "dev",
		Functions: []*agentv1.FunctionDescriptor{
			{
				Id:          "player.list",
				Version:     "1.1.0",
				Enabled:     true,
				InputSchema: `{"type":"object","properties":{"page":{"type":"integer"},"q":{"type":"string"}}}`,
			},
		},
	}
	resp, err := svc.handleRegisterRequest(ctx, second, "")
	require.NoError(t, err)
	for _, warning := range resp.GetWarnings() {
		assert.NotContains(t, warning, "schema_breaking_change",
			"additive optional field must not be flagged as breaking")
	}
}

type stubOwnerLookup struct {
	gameID string
	env    string
	owns   bool
}

func (l *stubOwnerLookup) SelfOwnerScope(ctx context.Context, agentID string) (string, string, bool) {
	return l.gameID, l.env, l.owns
}

type recordingHooks struct {
	registered atomic.Bool
}

func (h *recordingHooks) OnAgentRegistered(context.Context, string, string, string) {
	h.registered.Store(true)
}
func (h *recordingHooks) OnAgentHeartbeat(context.Context, string)    {}
func (h *recordingHooks) OnAgentDisconnected(context.Context, string) {}

// L665-667/L685/L688: 未知 agent 心跳自愈：TTL<=0 走默认值、重建会话并
// 触发 OnAgentRegistered 钩子。
func TestHandleHeartbeatRequest_SelfHealDefaultTTLAndHooks(t *testing.T) {
	svc := newTestControlService()
	svc.SetDefaultSessionTTL(0)
	svc.SetHeartbeatOwnerLookup(&stubOwnerLookup{gameID: "game-1", env: "dev", owns: true})
	hooks := &recordingHooks{}
	svc.SetClusterHooks(hooks)

	resp, err := svc.handleHeartbeatRequest(context.Background(), &agentv1.HeartbeatRequest{AgentId: "agent-gone"})
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// 会话被重建（带 reseeded 标记，ExpireAt 使用默认 TTL）。
	svc.registry.Mu().RLock()
	agent := svc.registry.AgentsUnsafe()["agent-gone"]
	svc.registry.Mu().RUnlock()
	require.NotNil(t, agent, "self-heal must reseed the session")
	assert.Equal(t, "true", agent.Labels["reseeded"])
	assert.True(t, agent.ExpireAt.After(time.Now().Add(23*time.Hour)), "default 24h TTL applies")
	assert.True(t, hooks.registered.Load(), "OnAgentRegistered hook must fire for reseed")
}

// L642-646: 已存在 agent 心跳自报 owner 时补建 Labels map。
func TestHandleHeartbeatRequest_EnsureLabelsMap(t *testing.T) {
	svc := newTestControlService()
	svc.registry.UpsertAgent(&registry.AgentSession{AgentID: "agent-lb", GameID: "game-1"})

	_, err := svc.handleHeartbeatRequest(context.Background(), &agentv1.HeartbeatRequest{
		AgentId:         "agent-lb",
		OwnerInstanceId: "inst-7",
	})
	require.NoError(t, err)

	svc.registry.Mu().RLock()
	agent := svc.registry.AgentsUnsafe()["agent-lb"]
	svc.registry.Mu().RUnlock()
	require.NotNil(t, agent)
	assert.Equal(t, "inst-7", agent.Labels["reportedOwner"])
}

// L706-708: 心跳异步落库 goroutine 中的 Upsert 错误分支。
func TestHandleHeartbeatRequest_AsyncPersistError(t *testing.T) {
	loader := newMockAgentSessionLoader()
	loader.upsertErr = errors.New("db down")
	svc := newTestControlServiceWithLoader(loader)
	svc.registry.UpsertAgent(&registry.AgentSession{AgentID: "agent-persist", GameID: "game-1"})

	_, err := svc.handleHeartbeatRequest(context.Background(), &agentv1.HeartbeatRequest{AgentId: "agent-persist"})
	require.NoError(t, err)

	select {
	case <-loader.upsertDone:
	case <-time.After(2 * time.Second):
		t.Fatal("async persist goroutine did not run")
	}
}

// L728-729: 心跳落盘节流（间隔内第二次返回 false）。
func TestShouldPersistSession_Throttled(t *testing.T) {
	svc := newTestControlService()
	assert.True(t, svc.shouldPersistSession("agent-x"), "first persist allowed")
	assert.False(t, svc.shouldPersistSession("agent-x"), "second persist within interval throttled")
}

// L858-859: LoadAgentSessions 中 LoadFromDB 失败。
func TestLoadAgentSessions_LoadError(t *testing.T) {
	loader := newMockAgentSessionLoader()
	loader.loadErr = errors.New("load failed")
	svc := newTestControlServiceWithLoader(loader)

	err := svc.LoadAgentSessions()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load agent sessions")
}

// L158: RemoveSession 命中不存在的 agent。
func TestAgentSessionStore_RemoveSession_Missing(t *testing.T) {
	store := NewAgentSessionStore()
	assert.False(t, store.RemoveSession("nope", ""))
}

// agent_session.go L57/L218/L230: 带 conn 的会话 Addr/Resolve 路径。
func TestAgentSessionStore_WithMuxConn(t *testing.T) {
	serverEnd, clientEnd := net.Pipe()
	defer clientEnd.Close()
	defer serverEnd.Close()

	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		return nil, nil
	})
	mux := tcptr.NewMuxConn(serverEnd, &tcptr.Config{}, handler)
	store := NewAgentSessionStore()
	sess := &AgentSession{AgentID: "agent-conn", conn: mux}
	store.Upsert(sess)

	assert.False(t, sess.Addr() == "", "Addr must resolve through the live conn")

	conn, ok := store.ResolveAgentConn("agent-conn")
	require.True(t, ok)
	assert.NotNil(t, conn)

	caller, ok := store.ResolveSessionCaller("agent-conn")
	require.True(t, ok)
	assert.NotNil(t, caller)
}

// tcp_listener.go L121: Accept 非超时错误且 listener 未标记关闭。
func TestTCPListener_Serve_AcceptError(t *testing.T) {
	listener, err := NewTCPListener(&TCPListenerConfig{Address: "127.0.0.1:0", Insecure: true}, NewAgentSessionStore(), nil, nil)
	require.NoError(t, err)

	// 直接关闭底层 listener（不走 TCPListener.Close()，closing 保持打开）
	// → Serve 收到非超时 accept 错误 → 包装返回。
	require.NoError(t, listener.listener.Close())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = listener.Serve(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accept:")
}

// tcp_listener.go L353: 注册链路 handleRegisterRequest 失败（upstream 返回错误）。
func TestServeConn_RegisterRequestErrorWarn(t *testing.T) {
	config := &TCPListenerConfig{Address: "127.0.0.1:0", Insecure: true}
	listener, err := NewTCPListener(config, NewAgentSessionStore(), nil, nil)
	require.NoError(t, err)

	svc := newTestControlService()
	svc.SetUpstreamHandler(&mockHandler{registerErr: errors.New("upstream down")})
	listener.SetHandler(svc)

	serverEnd, clientEnd := net.Pipe()
	defer clientEnd.Close()
	go func() { // 排水：mux 响应帧同步写管道
		buf := make([]byte, 4096)
		for {
			if _, err := clientEnd.Read(buf); err != nil {
				return
			}
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go listener.serveConn(ctx, serverEnd)

	frame := protocol.NewMessageBody(protocol.MsgRegisterRequest, 1,
		mustProto(t, &agentv1.RegisterRequest{AgentId: "agent-upstream", GameId: "g"}))
	wrapped := make([]byte, 4+len(frame))
	wrapped[0] = byte(len(frame) >> 24)
	wrapped[1] = byte(len(frame) >> 16)
	wrapped[2] = byte(len(frame) >> 8)
	wrapped[3] = byte(len(frame))
	copy(wrapped[4:], frame)
	_, werr := clientEnd.Write(wrapped)
	require.NoError(t, werr)
	time.Sleep(150 * time.Millisecond)
	clientEnd.Close()
}

// tcp_listener.go L392: 心跳全路径 handleHeartbeatRequest 失败（upstream 错误）。
func TestServeConn_HeartbeatRequestErrorWarn(t *testing.T) {
	config := &TCPListenerConfig{Address: "127.0.0.1:0", Insecure: true}
	listener, err := NewTCPListener(config, NewAgentSessionStore(), nil, nil)
	require.NoError(t, err)

	svc := newTestControlService()
	svc.SetUpstreamHandler(&mockHandler{heartbeatErr: errors.New("upstream down")})
	listener.SetHandler(svc)

	serverEnd, clientEnd := net.Pipe()
	defer clientEnd.Close()
	go func() { // 排水
		buf := make([]byte, 4096)
		for {
			if _, err := clientEnd.Read(buf); err != nil {
				return
			}
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go listener.serveConn(ctx, serverEnd)

	// 先注册（upstream register 成功路径），再心跳（失败告警）。
	regFrame := protocol.NewMessageBody(protocol.MsgRegisterRequest, 1,
		mustProto(t, &agentv1.RegisterRequest{AgentId: "agent-hb", GameId: "g"}))
	hbFrame := protocol.NewMessageBody(protocol.MsgHeartbeatRequest, 2,
		mustProto(t, &agentv1.HeartbeatRequest{AgentId: "agent-hb"}))
	for _, frame := range [][]byte{regFrame, hbFrame} {
		wrapped := make([]byte, 4+len(frame))
		wrapped[0] = byte(len(frame) >> 24)
		wrapped[1] = byte(len(frame) >> 16)
		wrapped[2] = byte(len(frame) >> 8)
		wrapped[3] = byte(len(frame))
		copy(wrapped[4:], frame)
		_, werr := clientEnd.Write(wrapped)
		require.NoError(t, werr)
		time.Sleep(80 * time.Millisecond)
	}
	clientEnd.Close()
}

// L311/L323/L335: handleRegister/handleHeartbeat/handleRegisterCapabilities
// 中下游请求处理失败的错误传播（upstream 返回错误）。
func TestHandleMessage_UpstreamErrorPropagation(t *testing.T) {
	svc := newTestControlService()
	svc.SetUpstreamHandler(&mockHandler{
		registerErr:     errors.New("upstream register down"),
		heartbeatErr:    errors.New("upstream heartbeat down"),
		capabilitiesErr: errors.New("upstream capabilities down"),
	})

	body, err := proto.Marshal(&agentv1.RegisterRequest{AgentId: "a", GameId: "g"})
	require.NoError(t, err)
	_, err = svc.handleRegister(context.Background(), body)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upstream register down")

	hb, err := proto.Marshal(&agentv1.HeartbeatRequest{AgentId: "a"})
	require.NoError(t, err)
	_, err = svc.handleHeartbeat(context.Background(), hb)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upstream heartbeat down")

	caps, err := proto.Marshal(&agentv1.RegisterCapabilitiesRequest{Provider: &agentv1.ProviderMeta{Id: "p1"}})
	require.NoError(t, err)
	_, err = svc.handleRegisterCapabilities(context.Background(), caps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upstream capabilities down")
}

// L860: LoadAgentSessions 成功路径（带 DB 的 registry）。
func TestLoadAgentSessions_Success(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/sessions.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, registry.MigrateAgentSessions(db))

	store := registry.NewStoreWithDB(db)
	svc := NewControlService(store, newMockAgentSessionLoader())
	svc.SetLogger(slog.Default())
	require.NoError(t, svc.LoadAgentSessions())
}
