// Package healthprobe 三语义健康探针（对标 k8s probe 术语），三 agent 共用：
//
//	Liveness  存活——自己/受管进程活没活（sidecar-agent 游戏进程探活）
//	Readiness 就绪——依赖就绪没就绪（devops 探 CI 端点、capture 探库连通）
//	Heartbeat 心跳——周期打点最近活体时刻（supervisor 事件上下文/面板「最后上报」）
//
// 一个能力一份实现：业务 agent 只声明「探什么」（Checker），语义/失败阈值/
// 故障窗口/打点一律复用本包；窗口记录见 probe.go（双通道：本地时间线文件 +
// WindowListener 上报出口）。core 包零外部依赖。
package healthprobe

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Semantics 探针语义。
type Semantics string

const (
	SemanticsLiveness  Semantics = "liveness"
	SemanticsReadiness Semantics = "readiness"
	// SemanticsHeartbeat 由 HeartbeatFile 打点文件承载（非 Checker 轮询），
	// 仅用于窗口记录/上报时的语义标注。
	SemanticsHeartbeat Semantics = "heartbeat"
)

// Checker 单次探测：返回 nil 即本次通过。
type Checker interface {
	Check(ctx context.Context) error
}

// CheckerFunc 适配普通函数。
type CheckerFunc func(ctx context.Context) error

// Check implements Checker.
func (f CheckerFunc) Check(ctx context.Context) error { return f(ctx) }

// HTTPChecker 探测 HTTP 端点：状态码 2xx/3xx 视为通过。
type HTTPChecker struct {
	URL    string
	Client *http.Client
}

// NewHTTPChecker 构造 HTTP 探测（timeout<=0 用默认 5s）。
func NewHTTPChecker(rawURL string, timeout time.Duration) *HTTPChecker {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &HTTPChecker{URL: rawURL, Client: &http.Client{Timeout: timeout}}
}

// Check implements Checker.
func (c *HTTPChecker) Check(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("http status %d", resp.StatusCode)
	}
	return nil
}

// TCPChecker 探测 TCP 端口可达。
type TCPChecker struct {
	Addr        string
	DialTimeout time.Duration
}

// NewTCPChecker 构造 TCP 探测（timeout<=0 用默认 3s）。
func NewTCPChecker(addr string, timeout time.Duration) *TCPChecker {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &TCPChecker{Addr: addr, DialTimeout: timeout}
}

// Check implements Checker.
func (c *TCPChecker) Check(ctx context.Context) error {
	timeout := c.DialTimeout
	if deadline, ok := ctx.Deadline(); ok && deadline.Before(time.Now().Add(timeout)) {
		timeout = time.Until(deadline)
	}
	conn, err := net.DialTimeout("tcp", c.Addr, timeout)
	if err != nil {
		return err
	}
	return conn.Close()
}
