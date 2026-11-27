package register

// AUD-092 回归：业务侧可见连接与就绪状态（Conn/Connected）只在注册确认
// （server ack）后发布；注册失败/超时、置换（新连接取代旧连接）、断线时
// 同步撤销。重连窗口内任务事件/指标等扩展发送观察到明确的 not-ready
// （Conn()==nil 或 Connected()==false），而非指向未确认/已关闭连接的
// 过期 ready。
//
// 场景对照：(a) ack 前业务侧 not-ready；(b) ack 后 ready 且为确认连接；
// (c) 注册失败/超时撤销；(d) 置换恰好交换一次（不暴露未确认新代、不残留
// 已关闭旧代）；(e) 断线撤销。另补 SetConn 注入面（注入即视为已确认会话，
// nil 等价断开）。

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// (a)+(b) 注册 ack 前业务侧 not-ready；ack 后 ready 且为确认连接；发布先于
// OnConnected（回调里已能观察到 ready）。
func TestConnPublishedOnlyAfterRegisterAck(t *testing.T) {
	release := make(chan struct{})
	conn := newFakeConn()
	conn.registerRelease = release
	s := &dialerScript{conns: []*fakeConn{conn}}

	var c *Client
	var onConnectedSeen atomic.Bool
	var onConnectedReady atomic.Bool
	cfg := fastCfg(s, func() {
		onConnectedSeen.Store(true)
		onConnectedReady.Store(c.Connected())
	})
	c = New(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)

	// Register 已被调用（必然晚于拨号与工作连接落位）但尚未 ack：
	// 业务侧必须看到 not-ready，不得暴露未确认连接。
	waitFor(t, time.Second, func() bool {
		conn.mu.Lock()
		defer conn.mu.Unlock()
		return conn.registerCalls >= 1
	})
	assert.Nil(t, c.Conn(), "dialed-but-unregistered conn must not be exposed to business consumers")
	assert.False(t, c.Connected(), "business readiness must stay false before register ack")

	close(release)
	waitFor(t, time.Second, func() bool { return c.Conn() == Conn(conn) })
	assert.True(t, c.Connected())
	waitFor(t, time.Second, func() bool { return onConnectedSeen.Load() })
	assert.True(t, onConnectedReady.Load(),
		"OnConnected must fire after the business-facing publish (ready already observable)")
}

// (c)-失败臂 置换代注册失败：旧代已关闭，业务侧撤销到 not-ready——既不残留
// 指向已关闭 gen1 的过期 ready，也不暴露未确认的 gen2。
func TestRegisterFailureRevokesPublished(t *testing.T) {
	gen1 := newFakeConn()
	gen2 := newFakeConn()
	gen2.failNextRegister = 1
	s := &dialerScript{conns: []*fakeConn{gen1, gen2}}
	c := New(fastCfg(s, nil))
	ctx := context.Background()

	require.NoError(t, c.DialAndRegister(ctx))
	require.Equal(t, Conn(gen1), c.Conn())
	require.True(t, c.Connected())

	require.Error(t, c.DialAndRegister(ctx))
	assert.Nil(t, c.Conn(), "failed replacement must not leave any exposed conn")
	assert.False(t, c.Connected())
	assert.True(t, gen1.closed.Load(), "replaced conn must be closed")
	assert.True(t, gen2.closed.Load(), "unregisterable conn must be closed")
}

// (c)-超时臂 置换代注册挂起（无响应）按 requestTimeout 预算退出：挂起窗口
// 与超时后业务侧均为明确 not-ready。
func TestRegisterTimeoutRevokesPublished(t *testing.T) {
	gen1 := newFakeConn()
	hang := newFakeConn()
	hang.registerRelease = make(chan struct{}) // 永不关闭 → Register 阻塞到 regCtx.Done
	s := &dialerScript{conns: []*fakeConn{gen1, hang}}
	cfg := fastCfg(s, nil)
	cfg.RequestTimeout = 60 * time.Millisecond
	c := New(cfg)
	ctx := context.Background()

	require.NoError(t, c.DialAndRegister(ctx))
	require.Equal(t, Conn(gen1), c.Conn())

	dialErr := make(chan error, 1)
	go func() { dialErr <- c.DialAndRegister(ctx) }()
	waitFor(t, time.Second, func() bool {
		hang.mu.Lock()
		defer hang.mu.Unlock()
		return hang.registerCalls >= 1
	})
	// 重连窗口：旧代已撤销、新代未确认——业务侧明确 not-ready。
	assert.Nil(t, c.Conn(), "during reconnect window no stale or unconfirmed conn may be exposed")
	assert.False(t, c.Connected())

	require.Error(t, <-dialErr, "hanging register must exit within requestTimeout budget")
	assert.Nil(t, c.Conn())
	assert.False(t, c.Connected())
	assert.True(t, gen1.closed.Load())
	assert.True(t, hang.closed.Load())
}

// (d) 置换恰好交换一次： exposed 状态沿 gen1→（窗口 nil）→gen2 单次迁移，
// 不暴露未确认新代、不残留已关闭旧代；发布次数与确认次数 1:1（OnConnected
// 计数），快照采样全程只见确认过的连接以 ready 形态出现。
func TestReplacementSwapsExposedStateExactlyOnce(t *testing.T) {
	gen1 := newFakeConn()
	gen2 := newFakeConn()
	gen2Release := make(chan struct{})
	gen2.registerRelease = gen2Release
	s := &dialerScript{conns: []*fakeConn{gen1, gen2}}

	var c *Client
	var publishes atomic.Int64
	cfg := fastCfg(s, func() { publishes.Add(1) })
	c = New(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	require.NoError(t, c.DialAndRegister(ctx))
	require.Equal(t, Conn(gen1), c.Conn())

	var pubMu sync.Mutex
	readySeen := map[Conn]bool{}
	var violation atomic.Bool
	stop := make(chan struct{})
	samplerDone := make(chan struct{})
	go func() {
		defer close(samplerDone)
		for {
			select {
			case <-stop:
				return
			default:
			}
			if cur := c.Conn(); cur != nil && cur.Connected() {
				pubMu.Lock()
				readySeen[cur] = true
				pubMu.Unlock()
				// AUD-092 修正（顺带 AUD-093）：acked 由 fakeConn.Register 在
				// 产生 ack 的瞬间置位——先于发布，且经 c.mu 与快照读配对，采样
				// 观察到 gen2 ready 时必见 acked；不再用测试 goroutine 在
				// <-dialErr 之后才落位的迟到标记（存在发布先于标记的窗口，
				// 曾致 ~1/200 次假阳性红）。
				if cur == Conn(gen2) && !gen2.acked.Load() {
					violation.Store(true) // 未确认的 gen2 不得以 ready 形态暴露
				}
			}
			time.Sleep(time.Millisecond)
		}
	}()
	waitFor(t, time.Second, func() bool {
		pubMu.Lock()
		defer pubMu.Unlock()
		return readySeen[Conn(gen1)]
	})

	dialErr := make(chan error, 1)
	go func() { dialErr <- c.DialAndRegister(ctx) }()
	waitFor(t, time.Second, func() bool {
		gen2.mu.Lock()
		defer gen2.mu.Unlock()
		return gen2.registerCalls >= 1
	})
	// 置换窗口：gen1 已关闭撤销、gen2 未确认——业务侧不得看到两者任一 ready。
	assert.Nil(t, c.Conn(), "replacement window must expose neither the closed gen1 nor the unconfirmed gen2")
	assert.False(t, c.Connected())
	assert.Equal(t, int64(1), publishes.Load(), "only gen1 ack may have published so far")

	close(gen2Release)
	require.NoError(t, <-dialErr)
	assert.Equal(t, Conn(gen2), c.Conn())
	assert.True(t, c.Connected())
	assert.True(t, gen1.closed.Load())

	waitFor(t, time.Second, func() bool {
		pubMu.Lock()
		defer pubMu.Unlock()
		return readySeen[Conn(gen2)]
	})
	close(stop)
	<-samplerDone

	pubMu.Lock()
	defer pubMu.Unlock()
	assert.Equal(t, map[Conn]bool{Conn(gen1): true, Conn(gen2): true}, readySeen,
		"only confirmed generations may ever be observed ready")
	assert.False(t, violation.Load(), "unconfirmed gen2 must never be observed ready")
	assert.Equal(t, int64(2), publishes.Load(), "each confirmed registration publishes exactly once")
}

// (e) 断线撤销：socket 死亡即业务侧可观察 not-ready；断线自愈拨号失败
// （重连窗口）撤销保持——不回到指向死连接的过期 ready。
func TestDisconnectRevokesPublished(t *testing.T) {
	gen1 := newFakeConn()
	s := &dialerScript{conns: []*fakeConn{gen1}} // 再次拨号即脚本耗尽（自愈拨号失败）
	c := New(fastCfg(s, nil))
	ctx := context.Background()

	require.NoError(t, c.DialAndRegister(ctx))
	require.True(t, c.Connected())

	require.NoError(t, gen1.Close())
	assert.False(t, c.Connected(), "dead socket must be observable as not-ready")

	require.Error(t, c.DialAndRegister(ctx), "reconnect dial failure")
	assert.Nil(t, c.Conn(), "disconnect must revoke the exposed conn, not keep a stale one")
	assert.False(t, c.Connected())
	assert.True(t, gen1.closed.Load())
}

// SetConn 注入面语义：注入即视为已确认会话（同时落业务侧可见快照，注入面
// 无注册握手可等）；传 nil 等价断开（工作连接与可见快照一并撤销）。
func TestSetConnPublishSemantics(t *testing.T) {
	c := New(fastCfg(&dialerScript{}, nil))
	conn := newFakeConn()
	c.SetConn(conn)
	assert.Equal(t, Conn(conn), c.Conn())
	assert.True(t, c.Connected())

	c.SetConn(nil)
	assert.Nil(t, c.Conn())
	assert.False(t, c.Connected())
}
