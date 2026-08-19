package agent

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/agent"
	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	transportcore "github.com/cuihairu/croupier/internal/transport"
	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// fakeControlServer 模拟控制面服务器，响应 Register/Heartbeat/RegisterCapabilities。
// mux 为 true 时服务端使用 MuxConn（双向会话，支持单向事件），否则使用普通
// 请求-响应 TCP 服务器。
type fakeControlServer struct {
	t              *testing.T
	mux            bool
	addrVal        string
	closeFn        func()
	mu             sync.Mutex
	registers      []*agentv1.RegisterRequest
	heartbeats     int
	taskEvents     [][]byte
	metricEvents   int
	registerFail   atomic.Bool
	muxCancel      context.CancelFunc
	heartbeatsSeen []*agentv1.HeartbeatRequest
}

func newFakeControlServer(t *testing.T) *fakeControlServer {
	t.Helper()
	return newFakeControlServerMode(t, false)
}

func newFakeMuxControlServer(t *testing.T) *fakeControlServer {
	t.Helper()
	return newFakeControlServerMode(t, true)
}

func newFakeControlServerMode(t *testing.T, mux bool) *fakeControlServer {
	t.Helper()
	f := &fakeControlServer{t: t, mux: mux}
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		switch msgID {
		case protocol.MsgRegisterRequest:
			var req agentv1.RegisterRequest
			if err := proto.Unmarshal(body, &req); err != nil {
				return nil, err
			}
			f.mu.Lock()
			f.registers = append(f.registers, &req)
			f.mu.Unlock()
			if f.registerFail.Load() {
				return nil, fmt.Errorf("register rejected")
			}
			resp := &agentv1.RegisterResponse{SessionId: "sess-1"}
			return proto.Marshal(resp)
		case protocol.MsgHeartbeatRequest:
			var req agentv1.HeartbeatRequest
			if err := proto.Unmarshal(body, &req); err != nil {
				return nil, err
			}
			f.mu.Lock()
			f.heartbeats++
			f.heartbeatsSeen = append(f.heartbeatsSeen, &req)
			f.mu.Unlock()
			resp := &agentv1.HeartbeatResponse{}
			return proto.Marshal(resp)
		case protocol.MsgRegisterCapabilitiesReq:
			resp := &agentv1.RegisterCapabilitiesResponse{}
			return proto.Marshal(resp)
		case protocol.MsgTaskEvent:
			f.mu.Lock()
			f.taskEvents = append(f.taskEvents, append([]byte(nil), body...))
			f.mu.Unlock()
			return nil, nil
		case protocol.MsgMetricEvent:
			f.mu.Lock()
			f.metricEvents++
			f.mu.Unlock()
			return nil, nil
		default:
			return nil, fmt.Errorf("unexpected msgID %#x", msgID)
		}
	})

	if mux {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		f.muxCancel = cancel
		f.addrVal = ln.Addr().String()
		f.closeFn = func() {
			cancel()
			_ = ln.Close()
		}
		go func() {
			for {
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				m := tcptr.NewMuxConn(conn, &tcptr.Config{
					RecvTimeout: 30 * time.Second,
					SendTimeout: 10 * time.Second,
				}, handler)
				go func() { _ = m.Run(ctx) }()
			}
		}()
	} else {
		srv, err := tcptr.NewServer(&tcptr.Config{
			Address:        "127.0.0.1:0",
			Insecure:       true,
			RecvTimeout:    5 * time.Second,
			SendTimeout:    5 * time.Second,
			ConnectTimeout: time.Second,
		}, handler)
		require.NoError(t, err)
		f.addrVal = srv.Addr()
		f.closeFn = func() { _ = srv.Close() }
		go func() { _ = srv.Serve(context.Background()) }()
	}
	t.Cleanup(f.close)
	return f
}

func (f *fakeControlServer) close() {
	if f.muxCancel != nil {
		f.muxCancel()
	}
	f.closeFn()
}

func (f *fakeControlServer) addr() string { return f.addrVal }

func (f *fakeControlServer) registerCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.registers)
}

func (f *fakeControlServer) heartbeatCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.heartbeats
}

// TestUpstreamClientLifecycle_MuxTransport 覆盖 mux 客户端完整的
// 连接 → 注册 → 心跳 → 任务/指标事件上报流程。
func TestUpstreamClientLifecycle_MuxTransport(t *testing.T) {
	f := newFakeMuxControlServer(t)
	store := agentlocal.NewLocalStore()
	store.Register("sess-1", "svc-1", "127.0.0.1:1", "v1", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "fn-1", Version: "1.0", Resource: "player", Operation: "kick"},
	}, nil)

	client := NewUpstreamClient(f.addr(), "agent-mux", store, nil)
	// 注意：NewUpstreamClient 不会应用 meta 的超时/心跳字段，需走 WithMetadata
	client.WithMetadata(UpstreamMetadata{
		GameID:            "game-1",
		Env:               "prod",
		HeartbeatInterval: 100 * time.Millisecond,
	})

	connected := make(chan struct{}, 4)
	client.OnConnected(func() { connected <- struct{}{} })
	disconnectCalled := make(chan error, 4)
	client.OnDisconnected(func(err error) { disconnectCalled <- err })

	localHandler := agent.NewLocalHandler(store, t.TempDir(), "agent-mux", slog.Default())
	client.SetLocalHandler(localHandler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, client.Start(ctx))

	select {
	case <-connected:
	case <-time.After(3 * time.Second):
		t.Fatal("OnConnected callback not fired")
	}
	assert.True(t, client.Connected())
	assert.Equal(t, "game-1", client.GameID())
	assert.Equal(t, "prod", client.Env())

	// 注册请求带上了函数与元数据
	require.Eventually(t, func() bool { return f.registerCount() >= 1 }, 2*time.Second, 50*time.Millisecond)
	f.mu.Lock()
	reg := f.registers[0]
	f.mu.Unlock()
	assert.Equal(t, "agent-mux", reg.AgentId)
	assert.Equal(t, "game-1", reg.GameId)
	require.Len(t, reg.Functions, 1)
	assert.Equal(t, "fn-1", reg.Functions[0].Id)

	// 手动 Sync / Heartbeat
	require.NoError(t, client.Sync(ctx))
	require.NoError(t, client.Heartbeat(ctx))

	// 任务事件与指标事件
	require.NoError(t, client.ReportTaskEvent(ctx, &sdkv1.TaskEvent{TaskId: "task-1"}))
	require.NoError(t, client.SendMetricEvent(ctx, &opsv1.MetricsReport{AgentId: "agent-mux"}))

	require.Eventually(t, func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		return len(f.taskEvents) == 1 && f.metricEvents == 1
	}, 2*time.Second, 50*time.Millisecond)

	// 心跳循环持续发送
	base := f.heartbeatCount()
	require.Eventually(t, func() bool { return f.heartbeatCount() > base }, 3*time.Second, 50*time.Millisecond)

	client.Stop()
	assert.False(t, client.Connected())
}

// TestUpstreamClientLifecycle_TCPTransport 覆盖非 mux 的简单 TCP 客户端路径。
func TestUpstreamClientLifecycle_TCPTransport(t *testing.T) {
	f := newFakeControlServer(t)
	store := agentlocal.NewLocalStore()

	client := NewUpstreamClient(f.addr(), "agent-tcp", store, &UpstreamMetadata{HeartbeatInterval: 200 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, client.Start(ctx))
	defer client.Stop()

	require.Eventually(t, func() bool { return f.registerCount() >= 1 }, 2*time.Second, 50*time.Millisecond)
	require.NoError(t, client.Sync(ctx))
	require.NoError(t, client.Heartbeat(ctx))
	require.NoError(t, client.SendMetricEvent(ctx, &opsv1.MetricsReport{AgentId: "a"}))

	// 已连接客户端 SendTaskEvent / ReportTaskEvent
	require.NoError(t, client.SendTaskEvent(ctx, &sdkv1.TaskEvent{TaskId: "t2"}))
}

func TestUpstreamClientSyncAfterServerRejectsRegister(t *testing.T) {
	f := newFakeControlServer(t)
	f.registerFail.Store(true)
	store := agentlocal.NewLocalStore()
	client := NewUpstreamClient(f.addr(), "agent-rej", store, &UpstreamMetadata{})

	// Start 内 dialServer 注册失败进入后台重连，但 Start 仍返回 nil
	err := client.Start(context.Background())
	require.NoError(t, err)
	client.Stop()
}

// TestTCPControlClientDirect 直接驱动 tcpControlClient 覆盖 nil 守卫与调用路径。
func TestTCPControlClientDirect(t *testing.T) {
	f := newFakeControlServer(t)
	ctx := context.Background()

	cc, err := newControlClient("tcp", f.addr(), nil)
	require.NoError(t, err)
	require.True(t, cc.Connected())

	reg, err := cc.Register(ctx, &agentv1.RegisterRequest{AgentId: "a1"})
	require.NoError(t, err)
	assert.Equal(t, "sess-1", reg.SessionId)

	_, err = cc.Heartbeat(ctx, &agentv1.HeartbeatRequest{AgentId: "a1"})
	require.NoError(t, err)

	caps, err := cc.RegisterCapabilities(ctx, &agentv1.RegisterCapabilitiesRequest{})
	require.NoError(t, err)
	assert.NotNil(t, caps)

	require.NoError(t, cc.SendTaskEvent(ctx, []byte("task")))
	require.NoError(t, cc.SendMetricEvent(ctx, []byte("metric")))
	require.NoError(t, cc.Close())
	assert.False(t, cc.Connected())

	// Close 二次调用不应 panic
	require.NoError(t, cc.Close())
}

func TestTCPControlClientNilGuards(t *testing.T) {
	var c *tcpControlClient
	assert.False(t, c.Connected())
	require.NoError(t, c.Close())
	require.Error(t, c.SendTaskEvent(context.Background(), nil))
	require.Error(t, c.SendMetricEvent(context.Background(), nil))
	_, err := c.Register(context.Background(), &agentv1.RegisterRequest{})
	require.Error(t, err)
	_, err = c.Heartbeat(context.Background(), &agentv1.HeartbeatRequest{})
	require.Error(t, err)
	_, err = c.RegisterCapabilities(context.Background(), &agentv1.RegisterCapabilitiesRequest{})
	require.Error(t, err)
}

func TestMuxControlClientNilGuards(t *testing.T) {
	var c *muxControlClient
	assert.False(t, c.Connected())
	require.NoError(t, c.Close())
	require.Error(t, c.SendTaskEvent(context.Background(), nil))
	require.Error(t, c.SendMetricEvent(context.Background(), nil))
	_, err := c.Register(context.Background(), &agentv1.RegisterRequest{})
	require.Error(t, err)
	_, err = c.Heartbeat(context.Background(), &agentv1.HeartbeatRequest{})
	require.Error(t, err)
	_, err = c.RegisterCapabilities(context.Background(), &agentv1.RegisterCapabilitiesRequest{})
	require.Error(t, err)
}

func TestMuxControlClientDirect(t *testing.T) {
	f := newFakeControlServer(t)
	ctx := context.Background()

	mc, err := newMuxControlClient(f.addr(), nil, nil)
	require.NoError(t, err)
	require.True(t, mc.Connected())

	_, err = mc.Register(ctx, &agentv1.RegisterRequest{AgentId: "m1"})
	require.NoError(t, err)

	_, err = mc.Heartbeat(ctx, &agentv1.HeartbeatRequest{AgentId: "m1"})
	require.NoError(t, err)

	caps, err := mc.RegisterCapabilities(ctx, &agentv1.RegisterCapabilitiesRequest{})
	require.NoError(t, err)
	assert.NotNil(t, caps)

	require.NoError(t, mc.SendTaskEvent(ctx, []byte("task")))
	require.NoError(t, mc.SendMetricEvent(ctx, []byte("metric")))
	require.NoError(t, mc.Close())
	assert.False(t, mc.Connected())
}

// TestUpstreamClientDialFailure 覆盖 dialServer 连接失败分支。
func TestUpstreamClientDialFailure(t *testing.T) {
	store := agentlocal.NewLocalStore()
	// 使用一个保证不可达的端口
	client := NewUpstreamClient("127.0.0.1:1", "agent-x", store, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	require.Error(t, client.dialServer(ctx))
	assert.False(t, client.Connected())
}

// TestUpstreamClientGameEnvDefaults 覆盖 GameID/Env nil 守卫。
func TestUpstreamClientGameEnvDefaults(t *testing.T) {
	var c *UpstreamClient
	assert.Equal(t, "", c.GameID())
	assert.Equal(t, "", c.Env())
	assert.False(t, c.Connected())
}

// TestOpsServerWrapperDelegation 覆盖 opsServerWrapper 的全部委托方法。
func TestOpsServerWrapperDelegation(t *testing.T) {
	// 注意：ExecTimeout=0 会使 WithTimeout(ctx,0) 立即过期，命令被立刻杀死。
	// ExecuteCommand 依赖调用方先执行 Validate() 归一化，此处显式设置。
	ops := NewOpsServer(&OpsConfig{
		Enabled:      true,
		AllowExec:    true,
		AllowRestart: true,
		ExecTimeout:  10 * time.Second,
	}, "a", "v", nil)
	w := &opsServerWrapper{ops: ops}
	ctx := context.Background()

	info, err := w.GetSystemInfo(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	assert.NotEmpty(t, info.Arch)

	procs, err := w.ListProcesses(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	assert.Empty(t, procs.Processes)

	_, err = w.ReportMetrics(ctx, &opsv1.MetricsReport{})
	require.NoError(t, err)

	_, err = w.RestartProcess(ctx, &opsv1.RestartProcessRequest{ProcessName: "nope"})
	require.Error(t, err)

	_, err = w.StopProcess(ctx, &opsv1.StopProcessRequest{ProcessName: "nope"})
	require.Error(t, err)

	_, err = w.StartProcess(ctx, &opsv1.StartProcessRequest{ProcessName: "nope"})
	require.Error(t, err)

	execResp, err := w.ExecuteCommand(ctx, &opsv1.ExecuteCommandRequest{Command: "echo", Args: []string{"hello"}})
	require.NoError(t, err)
	assert.Zero(t, execResp.ExitCode)
	assert.Contains(t, execResp.StdOut, "hello")

	_, err = w.ListServicesJSON(ctx, []byte("{}"))
	// 无 systemd 的环境可能返回错误，仅验证委托路径不 panic
	_ = err
	_, _ = w.GetServiceStatusJSON(ctx, []byte(`{"name":"nonexistent-xyz"}`))
	_, _ = w.ListCronJobsJSON(ctx)
}

// TestProviderManagerWrapper 覆盖 providerManagerWrapper 的分支。
func TestProviderManagerWrapper(t *testing.T) {
	t.Run("nil wrapper", func(t *testing.T) {
		var w *providerManagerWrapper
		assert.False(t, w.IsPlatformFunction("f"))
		_, err := w.Call(context.Background(), "f", nil)
		require.Error(t, err)
	})
	t.Run("nil pm falls back", func(t *testing.T) {
		w := &providerManagerWrapper{}
		assert.False(t, w.IsPlatformFunction("f"))
		_, err := w.Call(context.Background(), "f", nil)
		require.Error(t, err)
	})
	t.Run("extension function routes to app", func(t *testing.T) {
		app := New("127.0.0.1:1", "agent-1")
		app.extensionMu.Lock()
		app.extensionRoutes["ext-fn"] = extensionFunctionRoute{Driver: "workflow-driver"}
		app.extensionMu.Unlock()
		w := &providerManagerWrapper{app: app}
		assert.True(t, w.IsPlatformFunction("ext-fn"))

		// workflow-driver 是内置 noop 驱动，原样返回 payload
		out, err := w.Call(context.Background(), "ext-fn", []byte("payload"))
		require.NoError(t, err)
		assert.Equal(t, []byte("payload"), out)

		// 未知驱动名应报错
		app.extensionMu.Lock()
		app.extensionRoutes["ext-fn2"] = extensionFunctionRoute{Driver: "no-such-driver"}
		app.extensionMu.Unlock()
		_, err = w.Call(context.Background(), "ext-fn2", []byte("{}"))
		require.Error(t, err)
	})
}
