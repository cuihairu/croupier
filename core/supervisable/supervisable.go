// Package supervisable 被监管对象契约（agent-core §3，与 supervisor 合并定稿）：
// 信号纪律（SIGTERM/SIGINT→gracefulTimeout 内落盘退出）、退出码语义
// （0=主动停，supervisor 置 STOPPED 不拉起；非 0=异常退出，进 S2 退避/熔断）、
// 心跳打点（本地文件 mtime 供监管方 last_heartbeat_unix 采样）。
// 任何一个 core 系 agent 用 Main 作入口即出厂合格被监管对象。
package supervisable

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cuihairu/croupier/core/healthprobe"
)

// RunFunc 业务主循环：ctx 取消即应尽快落盘收尾并返回。
type RunFunc func(ctx context.Context) error

// exit 可注入的进程退出（测试替换）。
var exit = os.Exit

// Main 统一入口：安装信号、周期心跳打点、执行 run、按契约定退出码。
// 首个 SIGTERM/SIGINT 取消 ctx；gracefulTimeout 内 run 未返回或再次收到
// 信号则强退（exit 1）。返回退出码（0=干净停机）。
func Main(name string, run RunFunc, opts ...Option) int {
	cfg := config{gracefulTimeout: 10 * time.Second, heartbeatInterval: 10 * time.Second}
	for _, o := range opts {
		o(&cfg)
	}

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGTERM, os.Interrupt)
	defer signal.Stop(sigCh)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go watchSignals(name, sigCh, cancel, done, cfg.gracefulTimeout)
	if cfg.heartbeatPath != "" {
		go beatLoop(ctx, cfg.heartbeatPath, cfg.heartbeatInterval)
	}

	err := run(ctx)
	cancel()
	close(done) // 通知信号协程：run 已返回，无需强退
	if err != nil {
		slog.Error("agent exited with error", "agent", name, "error", err)
		return 1
	}
	slog.Info("agent stopped cleanly", "agent", name)
	return 0
}

// watchSignals 首个信号触发 cancel；等待 run 返回（done），超时或二次信号
// 强退（异常退出码，supervisor 按 S2 退避处理）。
func watchSignals(name string, sigCh chan os.Signal, cancel context.CancelFunc, done chan struct{}, graceful time.Duration) {
	<-sigCh
	slog.Info("shutdown signal received", "agent", name)
	cancel()
	select {
	case <-sigCh:
		slog.Error("forced exit on repeated signal", "agent", name)
		exit(1)
	case <-time.After(graceful):
		slog.Error("graceful timeout, forcing exit", "agent", name, "timeout", graceful)
		exit(1)
	case <-done:
	}
}

// beatLoop 周期心跳打点（Heartbeat 语义，core/healthprobe 的打点文件），
// ctx 取消即停。
func beatLoop(ctx context.Context, path string, interval time.Duration) {
	hb := healthprobe.NewHeartbeatFile(path)
	if err := hb.Beat(); err != nil {
		slog.Warn("heartbeat beat failed", "path", path, "error", err)
	}
	if interval <= 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := hb.Beat(); err != nil {
				slog.Warn("heartbeat beat failed", "path", path, "error", err)
			}
		}
	}
}
