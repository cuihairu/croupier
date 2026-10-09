package register

import (
	"context"
	"errors"
	"time"

	corebackoff "github.com/cuihairu/croupier/core/backoff"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
)

// Start 尝试初始连接+注册；失败转后台重连循环；随后启动心跳自愈循环，
// 直到 ctx 取消。serverAddr 未配置的场景由调用方短路（不打日志不建循环）。
func (c *Client) Start(ctx context.Context) {
	if err := c.DialAndRegister(ctx); err != nil {
		c.log.Warn("failed to connect to upstream server, will keep retrying in background...", "error", err)
		go c.RunReconnectLoop(ctx)
	} else {
		c.log.Info("upstream connected and registered successfully")
	}
	go c.RunHeartbeatLoop(ctx)
}

// DialAndRegister 建立连接并立即注册；失败时关闭半开连接并清理（重连/心跳自愈共用）。
func (c *Client) DialAndRegister(ctx context.Context) error {
	if old := c.current(); old != nil {
		_ = old.Close()
		c.setConn(nil)
	}
	conn, err := c.cfg.Dial(ctx)
	if err != nil {
		c.setConn(nil)
		return err
	}
	c.setConn(conn)
	if err := c.registerOnce(ctx); err != nil {
		_ = conn.Close()
		c.setConn(nil)
		return errors.Join(errors.New("register after connect"), err)
	}
	return nil
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
