package register

// AUD-093 回归：Stop 纳入 Agent 启动路径与直接拨号的生命周期。在飞拨号
// （pre-dial 检查之后、拨号返回之前的窗口）与在飞注册 ack 晚于 Stop 返回时：
// (a) 该连接必须被关闭；(b) 业务侧可见快照保持 nil（不复活 Connected）；
// (c) OnConnected 不触发；(d) 重连/心跳循环不重启。
//
// 场景对照：(a) 直接拨号晚于 Stop 返回 → 连接关闭、状态死亡、无回调；
// (a') Start 驱动的首拨忽略取消、在 Stop 收尾等待期间返回 → 同样收尾且无
// 回调；(b) 注册 ack 晚于 Stop 返回 → 不发布、不回调；(c) Stop 后循环有界
// 沉淀（不再拨号，Start 不得重启循环）；(d) 正常 Start/Stop 周期行为不变
// （含 AUD-092 语义：ack 后才发布、发布先于 OnConnected）。

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gatedDialer 构造忽略 ctx 取消的拨号器（复刻真实 dialer 不感知取消的窗口）：
// 进入即上报 started（缓冲 1，非阻塞），随后挂起直到 release 关闭。
func gatedDialer(started chan<- struct{}, release <-chan struct{}, conn Conn) Dialer {
	return func(context.Context) (Conn, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		return conn, nil
	}
}

// (a) 直接拨号晚于 Stop 返回：连接关闭、不落位、不发布、不回调——复刻
// 「Stop returned, then a pending dial was released, client flipped back
// to Connected (and OnConnected fired)」的双探针事故形态。
func TestDialLateReturnAfterStopStaysDead(t *testing.T) {
	conn := newFakeConn()
	dialStarted := make(chan struct{}, 1)
	release := make(chan struct{})
	var onConnected atomic.Bool
	cfg := fastCfg(&dialerScript{}, func() { onConnected.Store(true) })
	cfg.Dial = gatedDialer(dialStarted, release, conn)
	c := New(cfg)

	// 直接拨号（调用方自有 ctx，不受生命周期管辖）——pre-dial 检查通过后挂起。
	dialErr := make(chan error, 1)
	go func() { dialErr <- c.DialAndRegister(context.Background()) }()
	waitFor(t, time.Second, func() bool {
		select {
		case <-dialStarted:
			return true
		default:
			return false
		}
	})

	// Stop 先完成（此时无连接可关），拨号随后才被释放。
	c.Stop()
	close(release)

	require.ErrorIs(t, <-dialErr, ErrClientStopped, "late dial must be rejected with ErrClientStopped")
	assert.True(t, conn.closed.Load(), "late-returned dial conn must be closed")
	assert.Nil(t, c.Conn(), "published snapshot must stay nil after stop")
	assert.False(t, c.Connected(), "client must not resurrect to Connected after stop")
	assert.False(t, onConnected.Load(), "OnConnected must not fire after stop")
	assert.ErrorIs(t, c.DialAndRegister(context.Background()), ErrClientStopped)
}

// (a') Start 驱动的首拨忽略取消、在 Stop 的 lifecycleWG.Wait 等待期间返回：
// 同一停机闸下拒绝落位/发布，OnConnected 不触发（Stop 等待收尾不复活）。
func TestStartDrivenLateDialReinedInAtStop(t *testing.T) {
	conn := newFakeConn()
	dialStarted := make(chan struct{}, 1)
	var onConnected atomic.Bool
	var c *Client
	cfg := fastCfg(&dialerScript{}, func() { onConnected.Store(true) })
	// 忽略 ctx 取消的拨号器，且仅在生命周期取消（晚于停机闸置位）后才返回——
	// 确定性复刻「拨号在 Stop 收尾等待期间返回」，排除「拨号先于 Stop 开始」
	// 的无关竞速（那种情况下先装位后收尾由 Stop 自身清理覆盖）。闭包引用变量
	// c，须先声明后赋值：Config 按值拷贝，Dial 必须在 New 之前落位。
	cfg.Dial = func(context.Context) (Conn, error) {
		select {
		case dialStarted <- struct{}{}:
		default:
		}
		<-c.lifecycleCtx.Done()
		return conn, nil
	}
	c = New(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)
	waitFor(t, time.Second, func() bool {
		select {
		case <-dialStarted:
			return true
		default:
			return false
		}
	})

	// Stop 阻塞在循环收尾等待上；拨号在该等待期间返回（晚于停机闸置位）。
	stopped := make(chan struct{})
	go func() {
		c.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop did not return after late dial settled")
	}

	assert.True(t, conn.closed.Load(), "late dial conn must be closed")
	assert.Nil(t, c.Conn())
	assert.False(t, c.Connected())
	assert.False(t, onConnected.Load(), "late dial must not fire OnConnected")
	assert.ErrorIs(t, c.DialAndRegister(context.Background()), ErrClientStopped)
}

// (b) 注册 ack 晚于 Stop 返回：ack 不发布业务侧可见状态、不触发 OnConnected、
// 不再发起新拨号。
func TestRegisterAckRacingStopDoesNotResurrect(t *testing.T) {
	release := make(chan struct{})
	conn := newFakeConn()
	conn.registerRelease = release
	s := &dialerScript{conns: []*fakeConn{conn}}
	var onConnected atomic.Bool
	cfg := fastCfg(s, func() { onConnected.Store(true) })
	c := New(cfg)

	// 直接拨号已落位工作连接、注册挂起中。
	dialErr := make(chan error, 1)
	go func() { dialErr <- c.DialAndRegister(context.Background()) }()
	waitFor(t, time.Second, func() bool {
		conn.mu.Lock()
		defer conn.mu.Unlock()
		return conn.registerCalls >= 1
	})

	// Stop 完整收尾：关闭在飞连接、清空快照；随后 ack 才被释放。
	c.Stop()
	require.True(t, conn.closed.Load(), "Stop must close the in-flight conn")
	require.Nil(t, c.Conn(), "Stop must clear the published snapshot")
	require.False(t, onConnected.Load())

	close(release)
	require.ErrorIs(t, <-dialErr, ErrClientStopped, "post-stop ack must be rejected")
	assert.Nil(t, c.Conn(), "post-stop ack must not publish business-visible state")
	assert.False(t, c.Connected())
	assert.False(t, onConnected.Load(), "post-stop ack must not fire OnConnected")
	assert.Equal(t, int64(1), s.count(), "no further dials after stop")
}

// (c) Stop 后循环有界沉淀：重连循环停止拨号（计数冻结），Start 不得重启
// 任何循环，DialAndRegister 直接拒绝。
func TestLoopsDoNotRestartAfterStop(t *testing.T) {
	s := &dialerScript{} // 脚本耗尽即拨号失败 → 重连循环持续退避重试
	var onConnected atomic.Bool
	c := New(fastCfg(s, func() { onConnected.Store(true) }))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)

	waitFor(t, 2*time.Second, func() bool { return s.count() >= 3 })
	c.Stop()
	settled := s.count()

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, settled, s.count(), "no dial attempts may happen after Stop returns")
	assert.Nil(t, c.Conn())
	assert.False(t, c.Connected())
	assert.False(t, onConnected.Load())

	// 已停止客户端上 Start 早退——不重启循环、不拨号。
	c.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, settled, s.count(), "Start after Stop must not restart loops")
	assert.ErrorIs(t, c.DialAndRegister(context.Background()), ErrClientStopped)
	assert.Equal(t, settled, s.count())
}

// (d) 正常 Start/Stop 周期不变：ack 后发布（AUD-092 语义保持）、OnConnected
// 恰好一次；Stop 关闭连接、清空快照、无额外回调。
func TestStartStopCycleUnchanged(t *testing.T) {
	conn := newFakeConn()
	conn.instanceID = "inst-1"
	s := &dialerScript{conns: []*fakeConn{conn}}
	var onConnected atomic.Int64
	c := New(fastCfg(s, func() { onConnected.Add(1) }))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)

	waitFor(t, time.Second, func() bool { return c.Connected() })
	assert.Equal(t, Conn(conn), c.Conn())
	assert.Equal(t, "inst-1", c.OwnerInstance())
	assert.Equal(t, int64(1), onConnected.Load())

	c.Stop()
	assert.True(t, conn.closed.Load())
	assert.Nil(t, c.Conn())
	assert.False(t, c.Connected())
	assert.Equal(t, int64(1), onConnected.Load(), "no extra OnConnected after stop")
}
