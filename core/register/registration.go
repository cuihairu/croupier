package register

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	corebackoff "github.com/cuihairu/croupier/core/backoff"
)

// New 构造客户端（不建连接；Start 驱动）。
func New(cfg Config) *Client {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Client{cfg: cfg, log: log}
}

// registerOnce 组装 payload 并注册；成功记录 ownerInstance、透出 server 侧
// 注册告警（否则 agent 以为注册成功而函数实际不可用），并触发 OnConnected。
func (c *Client) registerOnce(ctx context.Context) error {
	req, err := c.cfg.Payload(ctx)
	if err != nil {
		return fmt.Errorf("build register payload: %w", err)
	}
	// payload 组装（业务钩子）可能触发连接置换/置空，注册前重新取连接。
	conn := c.current()
	if conn == nil {
		return errors.New("upstream client not connected")
	}
	resp, err := conn.Register(ctx, req)
	if err != nil {
		return err
	}
	if v := resp.GetInstanceId(); v != "" {
		c.setOwnerInstance(v)
		c.log.Info("upstream owner instance reported", "instance_id", v)
	}
	if ws := resp.GetWarnings(); len(ws) > 0 {
		c.log.Warn("upstream register reported warnings", "agent_id", c.cfg.AgentID, "warnings", ws)
	}
	c.log.Info("synced with upstream server")
	if c.cfg.OnConnected != nil {
		c.cfg.OnConnected()
	}
	return nil
}

// SyncOnce 单次注册（不重试）：组装 payload→Register→记录 ownerInstance
// 并触发 OnConnected。Start/重连路径内用；导出供测试与手动同步。
func (c *Client) SyncOnce(ctx context.Context) error {
	return c.registerOnce(ctx)
}

// SyncAttempts 带退避重试注册（每次尝试独立超时，attempts<=0 按 1 次）。
func (c *Client) SyncAttempts(ctx context.Context, attempts int) error {
	if attempts <= 0 {
		attempts = 1
	}
	var lastErr error
	b := c.newSyncBackoff()
	for i := 0; i < attempts; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		syncCtx, cancel := context.WithTimeout(ctx, c.requestTimeout())
		err := c.registerOnce(syncCtx)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if i == attempts-1 {
			break // 末次失败不空转尾部等待
		}
		if !sleepBackoff(ctx, b.NextBackOff()) {
			return ctx.Err()
		}
	}
	return lastErr
}

// Sync 立即重注册（3 次退避重试）。
func (c *Client) Sync(ctx context.Context) error {
	if c == nil {
		return errors.New("register client is nil")
	}
	return c.SyncAttempts(ctx, 3)
}

// Conn 返回当前连接快照（业务侧扩展发送用：任务事件/指标等）。
// 无连接时返回 nil。
func (c *Client) Conn() Conn {
	return c.current()
}

// Connected 当下是否持有存活连接。
func (c *Client) Connected() bool {
	conn := c.current()
	return conn != nil && conn.Connected()
}

// OwnerInstance 最近注册响应中的集群实例 ID（心跳携带）。
func (c *Client) OwnerInstance() string {
	if c == nil {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ownerInstance
}

// SetConn 注入当前连接（测试与高级装配用；正常路径经 Dial 建立，重连会覆盖）。
// 传 nil 等价断开。
func (c *Client) SetConn(conn Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conn = conn
}

// SetOwnerInstance 直接记录集群实例 ID（测试用；正常路径来自注册响应）。
func (c *Client) SetOwnerInstance(v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ownerInstance = v
}

// SetHeartbeatInterval 运行时调整心跳间隔（<=0 忽略，保用现值）。
func (c *Client) SetHeartbeatInterval(d time.Duration) {
	if c == nil || d <= 0 {
		return
	}
	c.cfg.HeartbeatInterval = d
}

// SetRequestTimeout 运行时调整注册调用超时（<=0 忽略，保用现值）。
func (c *Client) SetRequestTimeout(d time.Duration) {
	if c == nil || d <= 0 {
		return
	}
	c.cfg.RequestTimeout = d
}

// Stop 关闭当前连接（重连循环由 ctx 取消退出）。
func (c *Client) Stop() {
	if c == nil {
		return
	}
	if conn := c.current(); conn != nil {
		_ = conn.Close()
	}
}

func (c *Client) requestTimeout() time.Duration {
	if c.cfg.RequestTimeout > 0 {
		return c.cfg.RequestTimeout
	}
	return 10 * time.Second
}

// current 返回当前连接（加锁快照；重连与发送路径并发读写的字段）。
func (c *Client) current() Conn {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn
}

func (c *Client) setConn(conn Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conn = conn
}

func (c *Client) setOwnerInstance(v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ownerInstance = v
}

// sleepBackoff 等待退避时长或 ctx 取消（core/backoff.Sleep 转发）。
func sleepBackoff(ctx context.Context, d time.Duration) bool {
	return corebackoff.Sleep(ctx, d)
}
