package agent

// coverage_v9b_test.go 补充 upstream.go 未覆盖分支：后台重连循环、updateLoop
// 二次去抖、心跳断连重连/连续失败恢复、syncOnce 标签与 owner instance 记录、
// reportMetrics 惰性初始化，以及 extension_sync_puller 的 HTTP fake。

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	transportcore "github.com/cuihairu/croupier/internal/transport"
	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// mockControlClientV9 是 controlClient 的可编程 fake，用于直接驱动 syncOnce。
type mockControlClientV9 struct {
	mu         sync.Mutex
	connected  bool
	closed     bool
	instanceID string
	registers  []*agentv1.RegisterRequest
}

func (m *mockControlClientV9) Connected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connected
}

func (m *mockControlClientV9) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = false
	m.closed = true
	return nil
}

func (m *mockControlClientV9) Register(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	m.mu.Lock()
	cp := proto.Clone(req).(*agentv1.RegisterRequest)
	m.registers = append(m.registers, cp)
	m.mu.Unlock()
	return &agentv1.RegisterResponse{SessionId: "sess-v9", InstanceId: m.instanceID}, nil
}

func (m *mockControlClientV9) Heartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	return &agentv1.HeartbeatResponse{}, nil
}

func (m *mockControlClientV9) RegisterCapabilities(ctx context.Context, req *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error) {
	return &agentv1.RegisterCapabilitiesResponse{}, nil
}

func (m *mockControlClientV9) SendTaskEvent(ctx context.Context, body []byte) error { return nil }

func (m *mockControlClientV9) SendMetricEvent(ctx context.Context, body []byte) error { return nil }

// heartbeatFlakyServerV9 让心跳按次数注入失败：前 failFirst 次返回
// 非 protobuf 的垃圾响应（client 反序列化失败但 TCP 连接保持存活），
// 之后恢复正常——这样才能覆盖“连续心跳失败→重连”而非“断连→重连”路径。
// failRegisterFrom > 0 时，第 N 次开始的 register 请求返回错误，用于覆盖
// 心跳恢复后 re-register 失败的分支。
type heartbeatFlakyServerV9 struct {
	addrVal          string
	closeFn          func()
	mu               sync.Mutex
	heartbeats       int
	registers        int
	failFirst        int64
	failRegisterFrom int64
}

func newHeartbeatFlakyServerV9(t *testing.T, failFirst, failRegisterFrom int64) *heartbeatFlakyServerV9 {
	t.Helper()
	f := &heartbeatFlakyServerV9{failFirst: failFirst, failRegisterFrom: failRegisterFrom}
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		switch msgID {
		case protocol.MsgRegisterRequest:
			f.mu.Lock()
			f.registers++
			seen := int64(f.registers)
			f.mu.Unlock()
			if f.failRegisterFrom > 0 && seen >= f.failRegisterFrom {
				return nil, fmt.Errorf("register rejected #%d", seen)
			}
			return proto.Marshal(&agentv1.RegisterResponse{SessionId: "sess-v9hb"})
		case protocol.MsgHeartbeatRequest:
			f.mu.Lock()
			f.heartbeats++
			seen := int64(f.heartbeats)
			f.mu.Unlock()
			if seen <= atomic.LoadInt64(&f.failFirst) {
				return []byte("this-is-not-proto-v9"), nil
			}
			return proto.Marshal(&agentv1.HeartbeatResponse{})
		default:
			return proto.Marshal(&agentv1.RegisterCapabilitiesResponse{})
		}
	})
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
	t.Cleanup(f.closeFn)
	return f
}

func (f *heartbeatFlakyServerV9) addr() string { return f.addrVal }

func (f *heartbeatFlakyServerV9) counts() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.registers, f.heartbeats
}

// newSyncAPIV9 返回一个固定 payload 的扩展同步 API；onPull 在每次请求时回调。
func newSyncAPIV9(t *testing.T, onPull func()) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if onPull != nil {
			onPull()
		}
		_ = r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","payload":{"agentId":"agent-v9","installations":[]}}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// --- upstream.go: Start / reconnectLoop ---

func TestUpstreamClientStart_DialFailureV9(t *testing.T) {
	client := NewUpstreamClient("127.0.0.1:1", "agent-v9-dial", agentlocal.NewLocalStore(), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, client.Start(ctx)) // 首次拨号失败，进入后台重连
	time.Sleep(100 * time.Millisecond)
	client.Stop()
}

func TestUpstreamReconnectLoop_AttemptV9(t *testing.T) {
	client := NewUpstreamClient("127.0.0.1:1", "agent-v9-reconn", agentlocal.NewLocalStore(), nil)
	ctx, cancel := context.WithCancel(context.Background())
	go client.reconnectLoop(ctx, true)
	time.Sleep(200 * time.Millisecond) // 覆盖首次尝试 + 失败日志
	cancel()
}

// --- upstream.go: updateLoop 去抖的第二个事件（timer 非 nil 路径）与 sync 失败 ---

func TestUpstreamUpdateLoop_SecondEventAndSyncFailureV9(t *testing.T) {
	f := newFakeControlServer(t)
	store := agentlocal.NewLocalStore()
	client := NewUpstreamClient(f.addr(), "agent-v9-upd", store, &UpstreamMetadata{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, client.Start(ctx))
	defer client.Stop()

	require.Eventually(t, func() bool { return f.registerCount() >= 1 }, 3*time.Second, 50*time.Millisecond)

	// 第一次更新：创建 timer。
	store.Register("s1", "svc", "127.0.0.1:9", "1.0", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "fn-1", Version: "1.0"},
	}, nil)
	time.Sleep(100 * time.Millisecond)

	// 第二次更新（timer 已存在）：走 timer.Stop/Reset 分支。
	store.Register("s2", "svc", "127.0.0.1:9", "1.0", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "fn-2", Version: "1.0"},
	}, nil)
	time.Sleep(100 * time.Millisecond)

	// debounce 到期前杀掉 server：timer fire 后 syncWithRetry 失败并记录错误日志。
	f.closeFn()
	time.Sleep(800 * time.Millisecond)
}

// --- upstream.go: heartbeatLoop ---

func TestUpstreamHeartbeatLoop_DisconnectDialFailsV9(t *testing.T) {
	f := newFakeMuxControlServer(t)
	client := NewUpstreamClient(f.addr(), "agent-v9-hbdown", agentlocal.NewLocalStore(), nil)
	client.WithMetadata(UpstreamMetadata{HeartbeatInterval: 50 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, client.Start(ctx))

	require.Eventually(t, func() bool { return f.registerCount() >= 1 }, 3*time.Second, 50*time.Millisecond)

	// server 下线：client 检测断连后每次 tick 重拨失败。
	f.closeFn()
	time.Sleep(400 * time.Millisecond)
	client.Stop()
}

func TestUpstreamHeartbeatLoop_CloseThenReconnectV9(t *testing.T) {
	f := newFakeMuxControlServer(t)
	client := NewUpstreamClient(f.addr(), "agent-v9-hbreconn", agentlocal.NewLocalStore(), nil)
	client.WithMetadata(UpstreamMetadata{HeartbeatInterval: 50 * time.Millisecond})
	reconnected := make(chan struct{}, 8)
	client.OnConnected(func() { reconnected <- struct{}{} })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, client.Start(ctx))
	defer client.Stop()

	// 排空初始注册产生的信号（dialServer/syncOnce 各回调一次）。
	<-reconnected
	time.Sleep(100 * time.Millisecond)
	for len(reconnected) > 0 {
		<-reconnected
	}

	// 主动关闭底层连接（server 仍存活）：心跳循环发现断连后重新拨号注册成功。
	require.NoError(t, client.currentClient().Close())
	select {
	case <-reconnected:
	case <-time.After(3 * time.Second):
		t.Fatal("reconnect after explicit close not observed")
	}
	time.Sleep(50 * time.Millisecond) // 确保重连成功分支完整执行
}

func TestUpstreamHeartbeatLoop_ConsecutiveFailuresAndRecoveryV9(t *testing.T) {
	// 前 3 次心跳失败：#1/#2 触发连续失败重连（重连成功），
	// #3 失败后 #4 成功 → 走“恢复后重新注册”分支；
	// 第 3 次起的 register 失败 → 恢复后的 re-register 记录告警。
	f := newHeartbeatFlakyServerV9(t, 3, 3)
	client := NewUpstreamClient(f.addr(), "agent-v9-hbflaky", agentlocal.NewLocalStore(), nil)
	client.WithMetadata(UpstreamMetadata{HeartbeatInterval: 60 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, client.Start(ctx))
	defer client.Stop()

	require.Eventually(t, func() bool {
		regs, hbs := f.counts()
		return regs >= 2 && hbs >= 4
	}, 6*time.Second, 50*time.Millisecond)
}

// --- upstream.go: syncOnce / owner instance / reportMetrics ---

func TestUpstreamSyncOnce_TagsAndOwnerInstanceV9(t *testing.T) {
	store := agentlocal.NewLocalStore()
	store.Register("sess-1", "svc-1", "127.0.0.1:1", "1.0", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "fn-tags", Version: "1.0", Tags: []string{"core"}},
	}, nil)

	client := NewUpstreamClient("mock", "agent-v9-sync", store, nil)
	mock := &mockControlClientV9{connected: true, instanceID: "inst-v9"}
	client.setClient(mock)

	require.NoError(t, client.syncOnce(context.Background()))

	mock.mu.Lock()
	require.Len(t, mock.registers, 1)
	reg := mock.registers[0]
	mock.mu.Unlock()
	require.Len(t, reg.Functions, 1)
	assert.Equal(t, []string{"core"}, reg.Functions[0].Tags)

	// 注册响应中的集群实例 ID 被记录。
	assert.Equal(t, "inst-v9", client.ownerInstance())
}

func TestUpstreamSetOwnerInstanceV9(t *testing.T) {
	client := NewUpstreamClient("127.0.0.1:1", "agent-v9-owner", agentlocal.NewLocalStore(), nil)
	client.setOwnerInstance("inst-42")
	assert.Equal(t, "inst-42", client.ownerInstance())
}

func TestUpstreamReportMetrics_CollectorInitV9(t *testing.T) {
	client := NewUpstreamClient("127.0.0.1:1", "agent-v9-metrics", agentlocal.NewLocalStore(), nil)
	client.reportMetrics(context.Background()) // 惰性创建 collector；发送失败仅记录日志
	require.NotNil(t, client.metricsCollector)
}
