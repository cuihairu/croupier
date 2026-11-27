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
	client := &Client{cfg: cfg}
	client.startMu.Lock()
	client.lifecycleCtx, client.lifecycleCancel = context.WithCancel(context.Background())
	client.startMu.Unlock()
	client.log = cfg.Logger
	if client.log == nil {
		client.log = slog.Default()
	}
	return client
}

// registerOnce 组装 payload 并注册；成功记录 ownerInstance、把确认连接发布
// 为业务侧可见快照（AUD-092：发布只发生在 server ack 之后）、透出 server 侧
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
	// AUD-093：ack 可能晚于 Stop 返回——消费 ack 的任何副作用（ownerInstance
	// 记录、业务侧发布、OnConnected）前先复核停机闸，ack 不复活已停止的客户端。
	if c.stopGuard.Load() {
		return ErrClientStopped
	}
	if v := resp.GetInstanceId(); v != "" {
		c.setOwnerInstance(v)
		c.log.Info("upstream owner instance reported", "instance_id", v)
	}
	if ws := resp.GetWarnings(); len(ws) > 0 {
		c.log.Warn("upstream register reported warnings", "agent_id", c.cfg.AgentID, "warnings", ws)
	}
	c.log.Info("synced with upstream server")
	// AUD-092：注册确认（server ack）后才发布业务侧可见连接——发布先于
	// OnConnected，回调里业务侧已能观察到 ready 状态与已确认连接。
	// AUD-093：publishRegistered 在 mu 临界区内复核停机闸（覆盖上一次复核与
	// 落锁之间 Stop 完成的窗口）；被拒绝时不触发 OnConnected。
	if !c.publishRegistered(conn) {
		return ErrClientStopped
	}
	if c.cfg.OnConnected != nil {
		c.cfg.OnConnected()
	}
	return nil
}

// SyncOnce 单次注册（不重试）：组装 payload→Register→记录 ownerInstance
// 并触发 OnConnected。Start/重连路径内用；导出供测试与手动同步。
// ARCH-022：与 SyncAttempts 一致，单次注册也应用 requestTimeout 边界，
// 调用方 deadline 更早时以调用方为准（registerCtx 取较早者）。
func (c *Client) SyncOnce(ctx context.Context) error {
	syncCtx, cancel := c.registerCtx(ctx)
	defer cancel()
	return c.registerOnce(syncCtx)
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
		syncCtx, cancel := c.registerCtx(ctx)
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

// registerCtx 为一次注册调用建立超时边界：requestTimeout 与调用方
// deadline 取较早者（context.WithTimeout 语义，调用方 deadline 优先）。
// 无注册响应时按预算退出，避免重连/心跳路径被挂起的 Register 永久阻塞
// （ARCH-022）。
func (c *Client) registerCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, c.requestTimeout())
}

// Sync 立即重注册（3 次退避重试）。
func (c *Client) Sync(ctx context.Context) error {
	if c == nil {
		return errors.New("register client is nil")
	}
	return c.SyncAttempts(ctx, 3)
}

// Conn 返回业务侧可见的连接快照（业务侧扩展发送用：任务事件/指标等）。
// 仅在注册确认（server ack）后发布，替换/注册失败/断线/Stop 时同步撤销；
// 未确认（拨号成功但注册未返回）或已撤销时返回 nil。
func (c *Client) Conn() Conn {
	return c.publishedConn()
}

// Connected 当下业务侧是否处于注册确认且存活的 ready 状态。重连窗口内
// （旧连接已撤销、新连接尚未 ack）恒为 false，不残留过期 ready。
func (c *Client) Connected() bool {
	conn := c.publishedConn()
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

// SetConn 注入当前连接（测试与高级装配用；正常路径经 Dial 建立，重连会
// 覆盖）。注入即视为已确认会话，同时落业务侧可见快照（注入面无注册握手
// 可等）；传 nil 等价断开（工作连接与业务侧可见快照一并撤销）。
func (c *Client) SetConn(conn Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conn = conn
	c.published = conn
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
	c.startMu.Lock()
	hasLifecycle := c.lifecycleCancel != nil
	c.startMu.Unlock()
	if !hasLifecycle {
		return
	}
	c.stopOnce.Do(func() {
		// AUD-093：停机闸先于一切收尾置位——晚于 Stop 的拨号返回/注册 ack 在
		// 连接落位与业务侧发布的 mu 临界区内据此拒绝，不得复活已停止的客户端。
		c.stopGuard.Store(true)
		c.lifecycleCancel()
		if conn := c.current(); conn != nil {
			_ = conn.Close()
		}
		c.lifecycleWG.Wait()
		c.setConn(nil)
		// AUD-092：停机即撤销业务侧可见快照（在 lifecycleWG.Wait 之后，
		// 覆盖在飞注册刚确认发布的残尾）。
		c.clearPublished()
	})
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

// publishedConn 返回业务侧可见连接快照（加锁读；仅注册确认后非 nil）。
func (c *Client) publishedConn() Conn {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.published
}

// installDialedConn 落位拨号返回的内部工作连接；已停机（AUD-093 停机闸置位）
// 时拒绝并返回 false，由调用方关闭该连接。复核与落位在 mu 临界区内原子完成，
// 覆盖「拨号前检查之后、落位之前」Stop 完成的复活窗口。
func (c *Client) installDialedConn(conn Conn) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopGuard.Load() {
		return false
	}
	c.conn = conn
	return true
}

// publishRegistered 把注册确认的连接发布为业务侧可见快照；已停机（AUD-093
// 停机闸置位）时拒绝发布并返回 false——检查在 mu 临界区内与落位原子，晚于
// Stop 的 ack 无法复活业务侧可见状态。
func (c *Client) publishRegistered(conn Conn) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopGuard.Load() {
		return false
	}
	c.published = conn
	return true
}

// clearPublishedIf 撤销业务侧可见快照——仅当仍指向 target（代际守卫：
// 旧代的失败/关闭不得撤销新代已确认的发布）。
func (c *Client) clearPublishedIf(target Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.published == target {
		c.published = nil
	}
}

// clearPublished 无条件撤销业务侧可见快照（Stop 收尾用）。
func (c *Client) clearPublished() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.published = nil
}

// sleepBackoff 等待退避时长或 ctx 取消（core/backoff.Sleep 转发）。
func sleepBackoff(ctx context.Context, d time.Duration) bool {
	return corebackoff.Sleep(ctx, d)
}
