package agent

import (
	"context"
	"errors"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
)

// Store exposes the local instance registry.
func (a *App) Store() *agentlocal.LocalStore {
	if a == nil {
		return nil
	}
	return a.store
}

// SyncUpstream forces a best-effort upstream register call.
func (a *App) SyncUpstream(ctx context.Context) error {
	if a == nil || a.upstream == nil {
		return errors.New("agent upstream not initialized")
	}
	return a.upstream.Sync(ctx)
}

// HeartbeatUpstream triggers a best-effort upstream heartbeat call.
func (a *App) HeartbeatUpstream(ctx context.Context) error {
	if a == nil || a.upstream == nil {
		return errors.New("agent upstream not initialized")
	}
	return a.upstream.Heartbeat(ctx)
}

// AgentID returns the agent id the caller resolved once and passed into
// NewWithConfigDir (test/diagnostic accessor; 同一 id 供 recorder base 与
// core 注册共用，读回用于验证装配一致性)。
func (a *App) AgentID() string {
	if a == nil {
		return ""
	}
	return a.agentID
}

// OpsServer returns the ops server instance (for testing or direct access).
func (a *App) OpsServer() *OpsServer {
	if a == nil {
		return nil
	}
	a.opsMu.Lock()
	defer a.opsMu.Unlock()
	return a.opsServer
}

// MetricsCollector returns a new metrics collector for this agent.
func (a *App) MetricsCollector() *MetricsCollector {
	if a == nil {
		return nil
	}
	return NewMetricsCollector(a.agentID)
}

// Tasks returns the task index for tracking active tasks.
func (a *App) Tasks() *taskIndex {
	if a == nil {
		return nil
	}
	return a.tasks
}

// ActiveTaskCount returns the number of currently active tasks.
func (a *App) ActiveTaskCount() int {
	if a == nil || a.tasks == nil {
		return 0
	}
	return a.tasks.Len()
}

func (a *App) ExtensionRuntime() *ExtensionRuntime {
	if a == nil {
		return nil
	}
	return a.extensionRuntime
}
