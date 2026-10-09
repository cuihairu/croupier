package register

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corebackoff "github.com/cuihairu/croupier/core/backoff"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
)

type fakeConn struct {
	mu               sync.Mutex
	connected        bool
	registerCalls    int
	heartbeatCalls   int
	failNextRegister int
	failNextHB       int
	warnings         []string
	instanceID       string
	closed           atomic.Bool
}

func newFakeConn() *fakeConn { return &fakeConn{connected: true} }

func (f *fakeConn) Connected() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.connected && !f.closed.Load()
}

func (f *fakeConn) Close() error {
	f.closed.Store(true)
	return nil
}

func (f *fakeConn) Register(_ context.Context, _ *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registerCalls++
	if f.failNextRegister > 0 {
		f.failNextRegister--
		return nil, errors.New("register rejected")
	}
	return &agentv1.RegisterResponse{InstanceId: f.instanceID, Warnings: f.warnings}, nil
}

func (f *fakeConn) Heartbeat(_ context.Context, _ *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.heartbeatCalls++
	if f.failNextHB > 0 {
		f.failNextHB--
		return nil, errors.New("heartbeat timeout")
	}
	return &agentv1.HeartbeatResponse{}, nil
}

// dialerScript 按脚本回放连接/错误：先消费 errs，再依次返回 conns。
type dialerScript struct {
	mu       sync.Mutex
	conns    []*fakeConn
	errs     []error
	dialCnt  int
	payloadN atomic.Int64
}

func (s *dialerScript) dial(context.Context) (Conn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dialCnt++
	if len(s.errs) > 0 {
		err := s.errs[0]
		s.errs = s.errs[1:]
		return nil, err
	}
	if len(s.conns) == 0 {
		return nil, errors.New("dial script exhausted")
	}
	c := s.conns[0]
	s.conns = s.conns[1:]
	return c, nil
}

func (s *dialerScript) payload(context.Context) (*agentv1.RegisterRequest, error) {
	s.payloadN.Add(1)
	return &agentv1.RegisterRequest{AgentId: "agent-1"}, nil
}

func fastCfg(s *dialerScript, onConnected func()) Config {
	return Config{
		AgentID:           "agent-1",
		Dial:              s.dial,
		Payload:           s.payload,
		OnConnected:       onConnected,
		HeartbeatInterval: 5 * time.Millisecond,
		NewDialBackoff:    func() corebackoff.BackOff { return corebackoff.Exponential(time.Millisecond, 2*time.Millisecond, 2) },
		NewSyncBackoff:    func() corebackoff.BackOff { return corebackoff.Exponential(time.Millisecond, 2*time.Millisecond, 2) },
	}
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func TestStartRegistersAndReportsOwner(t *testing.T) {
	conn := newFakeConn()
	conn.instanceID = "inst-1"
	conn.warnings = []string{"registration_materialize_failed"}
	s := &dialerScript{conns: []*fakeConn{conn}}

	var connected int
	c := New(fastCfg(s, func() { connected++ }))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)

	waitFor(t, time.Second, func() bool { return connected > 0 })
	assert.Equal(t, "inst-1", c.OwnerInstance())
	assert.True(t, c.Connected())
	assert.Equal(t, int64(1), s.payloadN.Load())
	conn.mu.Lock()
	assert.Equal(t, 1, conn.registerCalls)
	conn.mu.Unlock()

	// Conn() 快照供扩展发送；Stop 关闭当前连接。
	assert.NotNil(t, c.Conn())
	c.Stop()
	assert.True(t, conn.closed.Load())
	assert.False(t, c.Connected())
}

func TestStartInitialFailureReconnectsInBackground(t *testing.T) {
	good := newFakeConn()
	s := &dialerScript{errs: []error{errors.New("dial refused")}, conns: []*fakeConn{good}}
	c := New(fastCfg(s, nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)

	waitFor(t, 2*time.Second, func() bool { return c.Connected() })
	assert.Equal(t, 2, s.dialCnt) // 首拨失败 + 重连成功
}

func TestHeartbeatFailureThresholdTriggersReconnect(t *testing.T) {
	bad := newFakeConn()
	bad.failNextHB = 2 // 连续 2 次心跳失败 = 达默认阈值
	good := newFakeConn()
	s := &dialerScript{conns: []*fakeConn{bad, good}}

	c := New(fastCfg(s, nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)

	// 心跳失败达阈值 → 重连换新连接（新连接不再失败）。
	waitFor(t, 2*time.Second, func() bool { return c.Conn() == Conn(good) })
	assert.True(t, bad.closed.Load())
	assert.Equal(t, 2, s.dialCnt)
}

func TestHeartbeatRecoveryReregisters(t *testing.T) {
	conn := newFakeConn()
	conn.failNextHB = 1 // 首次心跳失败（未达阈值），随后恢复
	s := &dialerScript{conns: []*fakeConn{conn}}

	c := New(fastCfg(s, nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)

	// 恢复路径：重注册确保会话最新 → registerCalls 达 2。
	waitFor(t, 2*time.Second, func() bool {
		conn.mu.Lock()
		defer conn.mu.Unlock()
		return conn.registerCalls >= 2
	})
	conn.mu.Lock()
	assert.GreaterOrEqual(t, conn.heartbeatCalls, 2)
	conn.mu.Unlock()
}

func TestSyncRetriesWithBackoff(t *testing.T) {
	conn := newFakeConn()
	conn.failNextRegister = 2
	s := &dialerScript{conns: []*fakeConn{conn}}
	c := New(fastCfg(s, nil))
	c.setConn(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	assert.NoError(t, c.Sync(ctx)) // 3 次尝试：败、败、成
	conn.mu.Lock()
	assert.Equal(t, 3, conn.registerCalls)
	conn.mu.Unlock()

	// 无连接时 Sync 失败。
	c.setConn(nil)
	assert.Error(t, c.Sync(ctx))
}

func TestNilClientSafe(t *testing.T) {
	var c *Client
	assert.Empty(t, c.OwnerInstance())
	c.Stop()
	assert.Error(t, c.Sync(context.Background()))
}

func TestPayloadErrorPropagates(t *testing.T) {
	s := &dialerScript{}
	cfg := fastCfg(s, nil)
	cfg.Payload = func(context.Context) (*agentv1.RegisterRequest, error) {
		return nil, errors.New("no payload")
	}
	c := New(cfg)
	c.setConn(newFakeConn())
	err := c.registerOnce(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "build register payload")
}

// 默认退避档回归（原 internal/app/agent 断言随 K2 上收迁入）：重连 5s→60s
// ×1.5 抖动 0.5；注册同步 200ms→2s。
func TestDefaultBackoffKnobs(t *testing.T) {
	c := &Client{}
	d, ok := c.newDialBackoff().(*corebackoff.ExponentialBackOff)
	require.True(t, ok)
	require.Equal(t, 5*time.Second, d.InitialInterval)
	require.Equal(t, 1.5, d.Multiplier)
	require.Equal(t, 60*time.Second, d.MaxInterval)
	require.Equal(t, 0.5, d.RandomizationFactor)

	s, ok := c.newSyncBackoff().(*corebackoff.ExponentialBackOff)
	require.True(t, ok)
	require.Equal(t, 200*time.Millisecond, s.InitialInterval)
	require.Equal(t, 2*time.Second, s.MaxInterval)
}
