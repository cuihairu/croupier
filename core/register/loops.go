package register

import (
	"context"
	"errors"
	"sync"
	"time"

	corebackoff "github.com/cuihairu/croupier/core/backoff"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
)

// Start 尝试初始连接+注册；失败转后台重连循环；随后启动心跳自愈循环，
// 直到 ctx 取消。serverAddr 未配置的场景由调用方短路（不打日志不建循环）。
func (c *Client) Start(ctx context.Context) {
	if c == nil || c.isStopped() {
		return
	}
	c.lifecycleWG.Add(1)
	go func() {
		defer c.lifecycleWG.Done()
		startCtx, cancel := context.WithCancel(ctx)
		stopOnLifecycleStop := context.AfterFunc(c.lifecycleContext(), cancel)
		defer stopOnLifecycleStop()
		defer cancel()

		loopWG := sync.WaitGroup{}
		if err := c.DialAndRegister(startCtx); err != nil {
			// AUD-093：Stop 已收尾（或首拨因停机被拒）时不重启重连/心跳循环——
			// 停止后的客户端不得自愈复活。
			if c.isStopped() || errors.Is(err, ErrClientStopped) {
				return
			}
			c.log.Warn("failed to connect to upstream server, will keep retrying in background...", "error", err)
			loopWG.Add(1)
			go func() {
				defer loopWG.Done()
				c.RunReconnectLoop(startCtx)
			}()
		} else {
			c.log.Info("upstream connected and registered successfully")
		}
		// AUD-093：停机收尾期间成功注册的连接也不启动心跳循环（Stop 已关闭该
		// 连接并撤销发布，心跳无对象可服务）。
		if c.isStopped() {
			return
		}
		loopWG.Add(1)
		go func() {
			defer loopWG.Done()
			c.RunHeartbeatLoop(startCtx)
		}()
		loopWG.Wait()
	}()
}

// DialAndRegister 建立连接并立即注册；失败时关闭半开连接并清理（重连/心跳自愈共用）。
func (c *Client) DialAndRegister(ctx context.Context) error {
	if c == nil {
		return errors.New("register client is nil")
	}
	if c.isStopped() {
		return ErrClientStopped
	}
	if !c.dialInflight.CompareAndSwap(false, true) {
		return ErrDialInProgress
	}
	defer c.dialInflight.Store(false)

	if old := c.current(); old != nil {
		_ = old.Close()
		c.setConn(nil)
		// AUD-092：旧连接关闭即同步撤销业务侧可见快照——置换窗口内业务
		// 侧看到 not-ready，而不是指向已关闭连接的过期 ready。
		c.clearPublishedIf(old)
	}
	conn, err := c.cfg.Dial(ctx)
	if err != nil {
		c.setConn(nil)
		return err
	}
	// AUD-093：拨号可能晚于 Stop 返回（dialer 不感知取消，或直接拨号不受生命
	// 周期 ctx 管辖）。落位前在同一停机闸下复核，且复核与落位在 mu 临界区内
	// 原子——覆盖「拨号前检查之后、落位之前」Stop 完成的窗口：已停机则关闭
	// 该连接并拒绝落位，禁止复活已停止的客户端。
	if !c.installDialedConn(conn) {
		_ = conn.Close()
		return ErrClientStopped
	}
	// AUD-092：此处只落内部工作连接（重连/心跳循环用），不发布业务侧可见
	// 快照——业务侧的连接与就绪状态待注册确认后由 registerOnce 发布。
	// ARCH-022：注册调用也应用 requestTimeout 边界（与 SyncAttempts 一致），
	// 避免无注册响应时 Register 永久阻塞重连/心跳路径；调用方 deadline 更早
	// 时以调用方为准（registerCtx 取较早者）。Dial 本身沿用调用方预算。
	regCtx, cancel := c.registerCtx(ctx)
	err = c.registerOnce(regCtx)
	cancel()
	if err != nil {
		_ = conn.Close()
		c.setConn(nil)
		// AUD-092：注册未确认，该连接不得留在业务侧可见快照（正常路径
		// registerOnce 失败不会发布，此处为代际守卫下的防御性撤销）。
		c.clearPublishedIf(conn)
		return errors.Join(errors.New("register after connect"), err)
	}
	return nil
}

func (c *Client) lifecycleContext() context.Context {
	c.startMu.Lock()
	defer c.startMu.Unlock()
	if c.lifecycleCtx == nil {
		c.lifecycleCtx, c.lifecycleCancel = context.WithCancel(context.Background())
	}
	return c.lifecycleCtx
}

func (c *Client) isStopped() bool {
	c.startMu.Lock()
	defer c.startMu.Unlock()
	return c.lifecycleCancel == nil || c.lifecycleCtx.Err() != nil
}

// RunReconnectLoop 按退避持续重试「连接+注册」，成功或 ctx 取消退出；手动
// 驱动 NextBackOff，不因累计时长放弃（退出只由 ctx 取消或成功触发）。
// Start 首拨失败后以后台 goroutine 驱动；导出供测试与自定义装配复用。
func (c *Client) RunReconnectLoop(ctx context.Context) {
	b := c.newDialBackoff()
	var attempt int
	for {
		if ctx.Err() != nil {
			c.log.Info("upstream reconnect cancelled, exiting retry loop")
			return
		}
		attempt++
		c.log.Info("waiting for upstream server to be ready...", "attempt", attempt)
		if err := c.DialAndRegister(ctx); err != nil {
			c.log.Debug("upstream dial attempt failed", "attempt", attempt, "error", err)
			if !corebackoff.Sleep(ctx, b.NextBackOff()) {
				c.log.Info("upstream reconnect cancelled, exiting retry loop")
				return
			}
			continue
		}
		c.log.Info("upstream reconnected and registered successfully", "attempt", attempt)
		return
	}
}

// heartbeatLoop 心跳自愈：断连时主动重连（而非跳过心跳——否则无路径恢复）；
// 心跳失败达阈值重连；恢复后立即重注册确保会话最新。
// RunHeartbeatLoop 心跳自愈循环：断连时主动重连（而非跳过心跳——否则无路径
// 恢复）；心跳失败达阈值重连；恢复后立即重注册确保会话最新。Start 导出驱动的
// 后台循环；导出供测试与自定义装配复用。
func (c *Client) RunHeartbeatLoop(ctx context.Context) {
	interval := c.cfg.HeartbeatInterval
	if interval <= 0 {
		interval = 3 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	consecutiveFailures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			conn := c.current()
			if conn == nil || !conn.Connected() {
				c.log.Warn("upstream disconnected, attempting re-connect and register...")
				if err := c.DialAndRegister(ctx); err != nil {
					c.log.Error("re-connect dial failed", "error", err)
				}
				continue
			}
			hbCtx, cancel := context.WithTimeout(ctx, c.heartbeatTimeout())
			_, err := conn.Heartbeat(hbCtx, &agentv1.HeartbeatRequest{
				AgentId:         c.cfg.AgentID,
				OwnerInstanceId: c.OwnerInstance(),
			})
			cancel()
			if err != nil {
				consecutiveFailures++
				c.log.Error("heartbeat failed", "error", err, "consecutive_failures", consecutiveFailures)
				if consecutiveFailures >= c.maxHeartbeatFailures() {
					c.log.Warn("heartbeat failed, attempting re-connect and register...", "failures", consecutiveFailures)
					if err := c.DialAndRegister(ctx); err != nil {
						c.log.Error("re-connect dial failed", "error", err)
						continue // 连接失败，继续尝试心跳
					}
					consecutiveFailures = 0
				}
			} else if consecutiveFailures > 0 {
				c.log.Info("heartbeat recovered, re-registering to ensure session is active...", "previous_failures", consecutiveFailures)
				if err := c.SyncAttempts(ctx, 1); err != nil {
					c.log.Warn("re-register after recovery failed", "error", err)
				} else {
					c.log.Info("re-registered successfully after recovery")
				}
				consecutiveFailures = 0
			}
		}
	}
}

func (c *Client) newDialBackoff() corebackoff.BackOff {
	if c.cfg.NewDialBackoff != nil {
		return c.cfg.NewDialBackoff()
	}
	return corebackoff.Exponential(5*time.Second, 60*time.Second, 1.5)
}

func (c *Client) newSyncBackoff() corebackoff.BackOff {
	if c.cfg.NewSyncBackoff != nil {
		return c.cfg.NewSyncBackoff()
	}
	return corebackoff.Exponential(200*time.Millisecond, 2*time.Second, 0)
}

func (c *Client) heartbeatTimeout() time.Duration {
	if c.cfg.HeartbeatTimeout > 0 {
		return c.cfg.HeartbeatTimeout
	}
	return 3 * time.Second
}

func (c *Client) maxHeartbeatFailures() int {
	if c.cfg.MaxHeartbeatFailures > 0 {
		return c.cfg.MaxHeartbeatFailures
	}
	return 2
}
