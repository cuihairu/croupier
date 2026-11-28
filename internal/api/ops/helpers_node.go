package ops

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

func opsNodes(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNodesRequest) (*OpsNodesResponse, error) {
	// 全局 scope（X-Game-ID/X-Env header，GameDBMiddleware 注入）决定
	// 节点可见性：切换游戏/环境后列表随之过滤。
	scope := svc.GameScopeFromContext(ctx)
	gameID := strings.TrimSpace(scope.GameID)
	env := strings.TrimSpace(scope.Env)
	return &OpsNodesResponse{
		Nodes: listNodes(ctx, svcCtx, gameID, env, ""),
	}, nil
}

func opsNodeCommands(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNodeCommandsRequest) (*OpsNodeCommandsResponse, error) {
	// Return available commands for the node
	commands := []NodeCommand{
		{Name: "drain", Description: "Drain node from accepting new functions"},
		{Name: "undrain", Description: "Allow node to accept new functions"},
		{Name: "restart", Description: "Restart the node agent"},
	}

	return &OpsNodeCommandsResponse{
		Commands: commands,
	}, nil
}

func opsNodeDrain(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNodeCommandsRequest) (*OpsNodeDrainResponse, error) {
	nodeID := strings.TrimSpace(req.NodeId)
	if nodeID == "" {
		return nil, errorx.NewBadRequest("nodeId is required")
	}
	// 跨 scope 动作拒绝（AGENT-001）：drain 是节点级写动作，须与列表/详情
	// 同一 scope 口径——节点不属于当前请求 scope（X-Game-ID/X-Env 注入）时
	// fail-closed，禁止 A scope 管理员对 B scope 节点执行动作。
	if err := NewAgentService(svcCtx).requireAgentScope(ctx, nodeID); err != nil {
		return nil, err
	}

	// Verify node exists and get connection
	if svcCtx.AgentSessionResolver == nil {
		return nil, errors.New("session resolver unavailable")
	}

	caller, ok := svcCtx.AgentSessionResolver.ResolveAgentConn(nodeID)
	if !ok {
		return nil, errorx.NewNotFound("node not found or no active connection: " + nodeID)
	}

	// 先落注册表路由门：dispatcher 的 agentCanInvoke 从此刻起不再给该节点
	// 派新请求（wire contract：drain 先停新路由，再等在途），而非等到
	// agent ACK 之后才生效。
	if svcCtx.RegistryStore != nil {
		svcCtx.RegistryStore.SetAgentDraining(nodeID, true)
	}

	// Send drain command to agent via TCP. ACK 由 agent 侧在在途归零或宽限
	// 到点后返回（caller.Call 同步等待，至多一个宽限窗）。
	drainBody, err := proto.Marshal(&sdkv1.ProviderDrainRequest{})
	if err != nil {
		return nil, fmt.Errorf("marshal ProviderDrainRequest: %w", err)
	}
	_, _, err = caller.Call(ctx, protocol.MsgProviderDrainRequest, drainBody)
	if err != nil {
		slog.Warn("failed to send drain command to agent", "agent_id", nodeID, "error", err)
		// Continue anyway - record the drain state
	}

	// Record drain state in OpsStateStore (transient node state only) and
	// persist the audit trail in audit_records.
	if svcCtx.OpsStateStore != nil {
		_, _ = svcCtx.OpsStateStore.Update(func(state *svc.OpsState) {
			if state.Nodes.Drained == nil {
				state.Nodes.Drained = make(map[string]time.Time)
			}
			state.Nodes.Drained[nodeID] = time.Now()
			state.Nodes.UpdatedAt = time.Now()
		})
	}
	recordOpsAudit(ctx, svcCtx, audit.EventNodeDrain, nodeID, "success", nil)

	return &OpsNodeDrainResponse{
		NodeId: nodeID,
		Status: "draining",
	}, nil
}

func opsNodeMeta(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNodeMetaRequest) (*OpsNodeMetaResponse, error) {
	store := svcCtx.RegistryStore
	if store == nil {
		return nil, errors.New("registry store unavailable")
	}

	store.Mu().RLock()
	var found *registry.AgentSession
	for _, sess := range store.AgentsUnsafe() {
		if sess != nil && sess.AgentID == req.NodeID {
			found = sess
			break
		}
	}
	labels := map[string]string{}
	if found != nil {
		for k, v := range found.Labels {
			labels[k] = v
		}
	}
	store.Mu().RUnlock()

	// 未命中按契约回 404（此前裸 errors.New 被 response.Error 映射成
	// 500 internal_error，线上 /nodes/:id/meta 恒 500 的第二病灶）。
	if found == nil {
		return nil, errorx.NewNotFound("node not found: " + req.NodeID)
	}
	// 跨 scope 读拒绝（AGENT-001）：meta 与列表/详情同 scope 口径，节点不属于
	// 当前请求 scope 时 fail-closed（同 requireAgentScope 的 agent 端点口径）。
	if err := NewAgentService(svcCtx).requireAgentScope(ctx, req.NodeID); err != nil {
		return nil, err
	}

	return &OpsNodeMetaResponse{Labels: labels}, nil
}

// opsNodeDetail 单设备详情：与 GET /ops/nodes 列表同源同过滤（含 scope 头
// 可见性语义），按 id 命中返回完整 Node；未命中按契约回 404。
func opsNodeDetail(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNodeDetailRequest) (*OpsNodeDetailResponse, error) {
	nodeID := strings.TrimSpace(req.NodeID)
	if nodeID == "" {
		return nil, errorx.NewBadRequest("nodeId is required")
	}
	// 与 opsNodes 相同的 scope 口径：切换游戏/环境后详情可见性随列表一致。
	scope := svc.GameScopeFromContext(ctx)
	for _, node := range listNodes(ctx, svcCtx, strings.TrimSpace(scope.GameID), strings.TrimSpace(scope.Env), "") {
		if node.Id == nodeID {
			return &OpsNodeDetailResponse{Node: node}, nil
		}
	}
	return nil, errorx.NewNotFound("node not found: " + nodeID)
}

func opsNodeRestart(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNodeCommandsRequest) (*OpsNodeRestartResponse, error) {
	nodeID := strings.TrimSpace(req.NodeId)
	if nodeID == "" {
		return nil, errorx.NewBadRequest("nodeId is required")
	}
	// 跨 scope 动作拒绝（AGENT-001）：restart 与 drain 同口径 fail-closed。
	if err := NewAgentService(svcCtx).requireAgentScope(ctx, nodeID); err != nil {
		return nil, err
	}

	// Verify node exists and get connection
	if svcCtx.AgentSessionResolver == nil {
		return nil, errors.New("session resolver unavailable")
	}

	caller, ok := svcCtx.AgentSessionResolver.ResolveAgentConn(nodeID)
	if !ok {
		return nil, errorx.NewNotFound("node not found or no active connection: " + nodeID)
	}

	// Send restart command to agent via TCP
	_, _, err := caller.Call(ctx, protocol.MsgRestartProcessRequest, []byte(`{}`))
	if err != nil {
		slog.Warn("failed to send restart command to agent", "agent_id", nodeID, "error", err)
		// Continue anyway - record the restart
	}

	recordOpsAudit(ctx, svcCtx, audit.EventNodeRestart, nodeID, "initiated", nil)

	return &OpsNodeRestartResponse{
		NodeId: nodeID,
		Status: "restarting",
	}, nil
}

func opsNodeUndrain(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNodeCommandsRequest) (*OpsNodeUndrainResponse, error) {
	nodeID := strings.TrimSpace(req.NodeId)
	if nodeID == "" {
		return nil, errorx.NewBadRequest("nodeId is required")
	}
	// 跨 scope 动作拒绝（AGENT-001）：undrain 与 drain 同口径 fail-closed。
	if err := NewAgentService(svcCtx).requireAgentScope(ctx, nodeID); err != nil {
		return nil, err
	}

	// Verify node exists and get connection
	if svcCtx.AgentSessionResolver == nil {
		return nil, errors.New("session resolver unavailable")
	}

	caller, ok := svcCtx.AgentSessionResolver.ResolveAgentConn(nodeID)
	if !ok {
		return nil, errorx.NewNotFound("node not found or no active connection: " + nodeID)
	}

	// 先清注册表路由门：即使下行的 undrain 失败，server 侧调度也立即恢复
	//（agent 侧 draining 标记在连接存活时是收紧项，清不掉只影响该节点
	// 本地的兜底拒绝，不影响 server 恢复派发）。
	if svcCtx.RegistryStore != nil {
		svcCtx.RegistryStore.SetAgentDraining(nodeID, false)
	}

	// Send undrain command to agent via TCP (reason sentinel: agent clears its
	// ProviderSession draining marks; see protocol.ProviderDrainReasonUndrain).
	undrainPayload, err := proto.Marshal(&sdkv1.ProviderDrainRequest{
		Reason: protocol.ProviderDrainReasonUndrain,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal ProviderDrainRequest: %w", err)
	}
	_, _, err = caller.Call(ctx, protocol.MsgProviderDrainRequest, undrainPayload)
	if err != nil {
		slog.Warn("failed to send undrain command to agent", "agent_id", nodeID, "error", err)
		// Continue anyway - record the undrain
	}

	// Clear drain state (transient) and persist the audit trail.
	if svcCtx.OpsStateStore != nil {
		_, _ = svcCtx.OpsStateStore.Update(func(state *svc.OpsState) {
			if state.Nodes.Drained != nil {
				delete(state.Nodes.Drained, nodeID)
			}
			state.Nodes.UpdatedAt = time.Now()
		})
	}
	recordOpsAudit(ctx, svcCtx, audit.EventNodeUndrain, nodeID, "success", nil)

	return &OpsNodeUndrainResponse{
		NodeId: nodeID,
		Status: "active",
	}, nil
}

// Health and maintenance implementations
