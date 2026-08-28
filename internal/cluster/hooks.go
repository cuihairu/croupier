package cluster

import (
	"context"
	"log/slog"
)

// SessionHooks 是 Agent 会话生命周期钩子（由 ControlService/TCPListener 调用，
// 实现负责写共享归属表）。nil 实现表示未启用集群，全部 no-op。
type SessionHooks interface {
	// OnAgentRegistered：Agent 注册/重连成功（本实例持有连接）。
	OnAgentRegistered(ctx context.Context, agentID, gameID, env string)
	// OnAgentHeartbeat：心跳续期归属。
	OnAgentHeartbeat(ctx context.Context, agentID string)
	// OnAgentDisconnected：连接断开（仅当会话真正被移除时调用）。
	OnAgentDisconnected(ctx context.Context, agentID string)
}

// OwnerHooks 基于 OwnerStore（DB 或 Redis 实现）+ 本实例身份实现
// SessionHooks。
type OwnerHooks struct {
	resolver   OwnerStore
	instanceID string
	epoch      uint64
}

// NewOwnerHooks 创建归属钩子。
func NewOwnerHooks(resolver OwnerStore, instanceID string, epoch uint64) *OwnerHooks {
	return &OwnerHooks{resolver: resolver, instanceID: instanceID, epoch: epoch}
}

// OnAgentRegistered claims ownership.
func (h *OwnerHooks) OnAgentRegistered(ctx context.Context, agentID, gameID, env string) {
	if h == nil || h.resolver == nil {
		return
	}
	if err := h.resolver.ClaimOwner(ctx, agentID, gameID, env, h.instanceID, h.epoch); err != nil {
		// 归属写失败只降级（转发找不到该 agent），不影响注册主流程。
		slog.Default().Warn("cluster: claim owner failed", "agent", agentID, "error", err)
	}
}

// OnAgentHeartbeat touches the lease.
func (h *OwnerHooks) OnAgentHeartbeat(ctx context.Context, agentID string) {
	if h == nil || h.resolver == nil {
		return
	}
	if err := h.resolver.Touch(ctx, agentID); err != nil {
		// Touch 失败可能是记录被接管（分区恢复后 agent 重连到别处）——
		// 归属表以最后 Claim 为准。但必须留观测点：TCP 活着而归属行冻结
		// 的僵尸连接排查全靠这条日志（曾在线上出现 owner 停续 343s 无任何
		// 日志可查）。
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

// LocalInvoker 是 owner 侧本地执行接口（dispatch 层注入）。
// req.Caller 携带原始调用者身份，实现必须据此重新鉴权（不得默认信任）。
type LocalInvoker interface {
	InvokeLocal(ctx context.Context, req *ForwardedInvoke) (*ForwardedResult, error)
}
