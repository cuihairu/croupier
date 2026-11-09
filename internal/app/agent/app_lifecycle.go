package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

func (a *App) startLocalServerSafely() error {
	err := a.StartLocalServer()
	if err != nil {
		return fmt.Errorf("failed to start local server: %w", err)
	}
	a.serveErrMu.Lock()
	serveErr := a.serveErr
	a.serveErrMu.Unlock()
	select {
	case err := <-serveErr:
		return fmt.Errorf("agent local server failed: %w", err)
	default:
		return nil
	}
}

func (a *App) currentServeErr() chan error {
	a.serveErrMu.Lock()
	defer a.serveErrMu.Unlock()
	return a.serveErr
}

func (a *App) startAgentServices(ctx context.Context) error {
	// Load providers before starting upstream
	if a.providerManager != nil {
		if err := a.providerManager.Load(ctx); err != nil {
			return err
		}
	}

	a.startMaintenance(ctx)
	a.startExtensionSync(ctx)
	a.startProbes(ctx)
	a.startProbePolicy(ctx)
	a.startProbeDBTargets(ctx)
	if err := a.upstream.Start(ctx); err != nil {
		return err
	}
	if err := a.opsServer.Start(); err != nil {
		return err
	}
	return nil
}

// Stop shuts down background upstream connection and local TCP server.
func (a *App) Stop() {
	if a == nil {
		return
	}
	// Stop function adapter plugin hosting first (todo.md:35): drain
	// in-flight plugin calls within the bounded window, then kill the
	// resident subprocesses — no plugin child outlives the agent. The
	// wired host is cleared so post-stop access sees "not wired".
	if a.funcHost != nil {
		a.funcHost.Stop()
		a.funcHost = nil
	}
	// Stop local TCP server（localServer 经锁快照读取，与 StartLocalServer
	// 的发布同步；启动未完成即收到停止信号时此处看到 nil，与原 nil 检查
	// 语义一致）
	a.localServerMu.Lock()
	ls := a.localServer
	a.localServerMu.Unlock()
	if ls != nil {
		_ = ls.Close()
	}
	// Stop upstream connection
	if a.upstream != nil {
		a.upstream.Stop()
	}
	if a.telemetry != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.telemetry.Shutdown(ctx); err != nil {
			slog.Warn("agent telemetry shutdown failed", "error", err)
		}
	}
	a.opsMu.Lock()
	opsServer := a.opsServer
	a.opsMu.Unlock()
	if opsServer != nil {
		opsServer.Stop()
	}
}
