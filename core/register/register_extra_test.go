package register

// registration.go / loops.go 补测：SyncAttempts 的 attempts<=0 归一、前置
// ctx 取消退出与退避被 ctx 中断（sleepBackoff false）臂；Sync 的 nil
// receiver 臂；SetConn / SetOwnerInstance / SetHeartbeatInterval /
// SetRequestTimeout 的赋值与忽略（<=0 / nil）臂；Stop 在无生命周期时的
// 早退臂；Start 的 nil 与已停止早退臂；DialAndRegister 的 nil receiver 臂；
// lifecycleContext 惰性初始化；RunReconnectLoop 的退避中断退出与
// 继续重试臂；RunHeartbeatLoop 的默认间隔档、断连时主动重连（成功与
// 失败两臂）、心跳阈值重连失败后的继续臂与恢复后重注册失败臂；以及
// heartbeatTimeout / maxHeartbeatFailures 默认档。
//
// 诚实跳过：Start 的 DialAndRegister 成功分支（TestStartRegistersAnd
// ReportsOwner 已覆盖）与 RunReconnectLoop 的成功退出臂（TestStartInitial
// FailureReconnectsInBackground 已覆盖）；heartbeat 阈值重连后置零并
// 再次成功的臂（TestHeartbeatFailureThresholdTriggersReconnect 覆盖）；
// 恢复后重注册成功臂（TestHeartbeatRecoveryReregisters 覆盖）。本文件仅
// 补前述清单中未被既有用例触达的错误/守卫臂。

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	corebackoff "github.com/cuihairu/croupier/core/backoff"
)

func TestSyncAttemptsGuards(t *testing.T) {
	ctx := context.Background()

	// attempts<=0 → 按 1 次执行（成功即返）
	conn := newFakeConn()
	c := New(fastCfg(&dialerScript{conns: []*fakeConn{conn}}, nil))
	c.setConn(conn)
	require.NoError(t, c.SyncAttempts(ctx, 0))
	conn.mu.Lock()
	require.Equal(t, 1, conn.registerCalls)
	conn.mu.Unlock()

	// 前置 ctx 取消 → 立即退出
	c2 := New(fastCfg(&dialerScript{}, nil))
	c2.setConn(newFakeConn())
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	require.ErrorIs(t, c2.SyncAttempts(cctx, 3), context.Canceled)

	// 退避期间 ctx 取消 → 中断等待并返回 ctx.Err()
	failing := newFakeConn()
	failing.failNextRegister = 1
	cfg := fastCfg(&dialerScript{}, nil)
	cfg.NewSyncBackoff = func() corebackoff.BackOff {
		return corebackoff.Exponential(10*time.Second, 10*time.Second, 1)
	}
	c3 := New(cfg)
	c3.setConn(failing)
	bctx, bcancel := context.WithCancel(ctx)
	time.AfterFunc(30*time.Millisecond, bcancel)
	require.ErrorIs(t, c3.SyncAttempts(bctx, 2), context.Canceled)
}

func TestConfigSettersAndTrivialAccessors(t *testing.T) {
	cfg := fastCfg(&dialerScript{}, nil)
	c := New(cfg)

	// SetConn 注入快照
	conn := newFakeConn()
	c.SetConn(conn)
	require.Equal(t, Conn(conn), c.Conn())

	// SetOwnerInstance 直接记录
	c.SetOwnerInstance("inst-9")
	require.Equal(t, "inst-9", c.OwnerInstance())

	// SetHeartbeatInterval：正数生效，<=0 与 nil receiver 忽略
	c.SetHeartbeatInterval(9 * time.Second)
	require.Equal(t, 9*time.Second, c.cfg.HeartbeatInterval)
	c.SetHeartbeatInterval(0)
	require.Equal(t, 9*time.Second, c.cfg.HeartbeatInterval)
	c.SetHeartbeatInterval(-time.Second)
	require.Equal(t, 9*time.Second, c.cfg.HeartbeatInterval)
	var nilC *Client
	nilC.SetHeartbeatInterval(time.Second)

	// SetRequestTimeout 同理
	c.SetRequestTimeout(12 * time.Second)
	require.Equal(t, 12*time.Second, c.cfg.RequestTimeout)
	c.SetRequestTimeout(0)
	require.Equal(t, 12*time.Second, c.cfg.RequestTimeout)
	nilC.SetRequestTimeout(time.Second)

	// Stop 在无生命周期时早退（零值 Client 未走 New）
	var zero Client
	zero.Stop()

	// lifecycleContext 惰性初始化
	lazy := &Client{}
	require.NotNil(t, lazy.lifecycleContext())

	// heartbeatTimeout / maxHeartbeatFailures 默认档
	require.Equal(t, 3*time.Second, c.heartbeatTimeout())
	require.Equal(t, 2, c.maxHeartbeatFailures())
	cfg.HeartbeatTimeout = 5 * time.Second
	cfg.MaxHeartbeatFailures = 7
	c2 := New(cfg)
	require.Equal(t, 5*time.Second, c2.heartbeatTimeout())
	require.Equal(t, 7, c2.maxHeartbeatFailures())
}

func TestStartAndDialNilGuards(t *testing.T) {
	// Start：nil receiver 与已停止 client 均早退
	(*Client)(nil).Start(context.Background())
	(&Client{}).Start(context.Background())

	// DialAndRegister：nil receiver
	require.ErrorContains(t, (*Client)(nil).DialAndRegister(context.Background()),
		"register client is nil")
}

func TestRunReconnectLoopInterruptedAndContinue(t *testing.T) {
	// 退避被 ctx 中断 → 退出
	s := &dialerScript{errs: []error{errors.New("dial refused")}}
	cfg := fastCfg(s, nil)
	cfg.NewDialBackoff = func() corebackoff.BackOff {
		return corebackoff.Exponential(10*time.Second, 10*time.Second, 1)
	}
	c := New(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(30*time.Millisecond, cancel)
	c.RunReconnectLoop(ctx) // 同步返回即中断退出
	require.Equal(t, int64(1), s.count())

	// 退避成功 → 继续重试（短退避，至少两拨）
	s2 := &dialerScript{errs: []error{errors.New("dial refused")}}
	c2 := New(fastCfg(s2, nil))
	ctx2, cancel2 := context.WithCancel(context.Background())
	time.AfterFunc(80*time.Millisecond, cancel2)
	c2.RunReconnectLoop(ctx2)
	require.GreaterOrEqual(t, s2.count(), int64(2))
}

func TestHeartbeatLoopDefaultIntervalAndReconnect(t *testing.T) {
	// 默认间隔档（interval<=0 → 3s）：运行即返，不 panic
	cfg := fastCfg(&dialerScript{}, nil)
	cfg.HeartbeatInterval = 0
	c := New(cfg)
	c.setConn(newFakeConn())
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(40*time.Millisecond, cancel)
	c.RunHeartbeatLoop(ctx)

	// 断连（无连接）→ 主动重连成功 → continue
	good := newFakeConn()
	s := &dialerScript{conns: []*fakeConn{good}}
	c2 := New(fastCfg(s, nil))
	ctx2, cancel2 := context.WithCancel(context.Background())
	time.AfterFunc(80*time.Millisecond, cancel2)
	c2.RunHeartbeatLoop(ctx2)
	require.GreaterOrEqual(t, s.count(), int64(1))
	require.Equal(t, Conn(good), c2.Conn())

	// 心跳达阈值 → 重连失败 → 记录并 continue（后续断连路径同样触达）
	bad := newFakeConn()
	bad.failNextHB = 2
	s3 := &dialerScript{errs: []error{errors.New("dial down")}}
	c3 := New(fastCfg(s3, nil))
	c3.setConn(bad)
	ctx3, cancel3 := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel3)
	c3.RunHeartbeatLoop(ctx3)
	require.GreaterOrEqual(t, s3.count(), int64(1))
	require.True(t, bad.closed.Load())
}

func TestHeartbeatRecoveryReregisterFailure(t *testing.T) {
	// 心跳失败（未达阈值）后恢复 → 重注册失败 → 记录告警继续
	conn := newFakeConn()
	conn.failNextHB = 1
	conn.failNextRegister = 1
	s := &dialerScript{conns: []*fakeConn{conn}}
	c := New(fastCfg(s, nil))
	c.setConn(conn)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(120*time.Millisecond, cancel)
	c.RunHeartbeatLoop(ctx)

	conn.mu.Lock()
	require.GreaterOrEqual(t, conn.registerCalls, 1, "recovery re-register must be attempted")
	conn.mu.Unlock()
}
