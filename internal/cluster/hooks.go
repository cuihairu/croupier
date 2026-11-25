package cluster

import (
	"context"
	"log/slog"
)

// SessionHooks is the Agent session lifecycle hook set (called by
// ControlService/TCPListener; implementations write the shared ownership
// table). A nil implementation means clustering is disabled and everything is
// a no-op.
type SessionHooks interface {
	// OnAgentRegistered: the Agent registered/reconnected successfully and this
	// instance holds the connection.
	OnAgentRegistered(ctx context.Context, agentID, gameID, env string, labels map[string]string)
	// OnAgentHeartbeat: a heartbeat renews ownership.
	OnAgentHeartbeat(ctx context.Context, agentID string)
	// OnAgentDisconnected: the connection dropped (only called when the session
	// is actually removed).
	OnAgentDisconnected(ctx context.Context, agentID string)
}

// OwnerHooks implements SessionHooks on top of OwnerStore (DB or Redis
// implementation) plus this instance's identity.
type OwnerHooks struct {
	resolver   OwnerStore
	instanceID string
	epoch      uint64
}

// NewOwnerHooks creates the ownership hooks.
func NewOwnerHooks(resolver OwnerStore, instanceID string, epoch uint64) *OwnerHooks {
	return &OwnerHooks{resolver: resolver, instanceID: instanceID, epoch: epoch}
}

// OnAgentRegistered claims ownership.
func (h *OwnerHooks) OnAgentRegistered(ctx context.Context, agentID, gameID, env string, labels map[string]string) {
	if h == nil || h.resolver == nil {
		return
	}
	if err := h.resolver.ClaimOwner(ctx, agentID, gameID, env, h.instanceID, h.epoch, labels); err != nil {
		// A failed ownership write is a degraded path only (forwarding cannot
		// find the agent); the main registration flow is unaffected.
		slog.Default().Warn("cluster: claim owner failed", "agent", agentID, "error", err)
	}
}

// OnAgentHeartbeat touches the lease.
func (h *OwnerHooks) OnAgentHeartbeat(ctx context.Context, agentID string) {
	if h == nil || h.resolver == nil {
		return
	}
	if err := h.resolver.Touch(ctx, agentID); err != nil {
		// A failed Touch may mean the record was taken over (after partition
		// recovery the agent reconnected elsewhere) — the ownership table
		// defers to the latest Claim. But an observation point must remain:
		// this log line is the only way to diagnose zombie connections where
		// TCP is alive but the ownership row is frozen (production once saw an
		// owner stop renewing for 343s with nothing in the logs).
		slog.Default().Warn("cluster: touch owner failed", "agent", agentID, "error", err)
	}
}

// OnAgentDisconnected releases ownership.
func (h *OwnerHooks) OnAgentDisconnected(ctx context.Context, agentID string) {
	if h == nil || h.resolver == nil {
		return
	}
	if err := h.resolver.Release(ctx, agentID); err != nil {
		slog.Default().Warn("cluster: release owner failed", "agent", agentID, "error", err)
	}
}

// LocalInvoker is the owner-side local execution interface (injected by the
// dispatch layer). req.Caller carries the original caller identity and
// implementations must re-authenticate based on it (never trust by default).
type LocalInvoker interface {
	InvokeLocal(ctx context.Context, req *ForwardedInvoke) (*ForwardedResult, error)
}
