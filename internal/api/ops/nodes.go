package ops

import (
	"context"
	"errors"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
)

// Node operations sub-service

type NodeService struct {
	svcCtx *svc.ServiceContext
}

func NewNodeService(svcCtx *svc.ServiceContext) *NodeService {
	return &NodeService{svcCtx: svcCtx}
}

func (s *NodeService) List(ctx context.Context, gameId, env, status string) ([]Node, error) {
	return listNodes(ctx, s.svcCtx, gameId, env, status), nil
}

func (s *NodeService) GetCommands(ctx context.Context, nodeId string) ([]NodeCommand, error) {
	// Return available commands for the node
	commands := []NodeCommand{
		{Name: "drain", Description: "Drain node from accepting new functions"},
		{Name: "undrain", Description: "Allow node to accept new functions"},
		{Name: "restart", Description: "Restart the node agent"},
	}

	return commands, nil
}

func (s *NodeService) Drain(ctx context.Context, nodeId string) error {
	store := s.svcCtx.RegistryStore
	if store == nil {
		return errors.New("registry store unavailable")
	}

	// Verify node exists
	store.Mu().RLock()
	found := false
	for _, sess := range store.AgentsUnsafe() {
		if sess != nil && sess.AgentID == nodeId {
			found = true
			break
		}
	}
	store.Mu().RUnlock()
	if !found {
		return errorx.NewNotFound("node not found: " + nodeId)
	}

	// Update transient node state; the audit trail goes to audit_records.
	if s.svcCtx.OpsStateStore != nil {
		_, _ = s.svcCtx.OpsStateStore.Update(func(state *svc.OpsState) {
			if state.Nodes.Drained == nil {
				state.Nodes.Drained = make(map[string]time.Time)
			}
			state.Nodes.Drained[nodeId] = time.Now()
			state.Nodes.UpdatedAt = time.Now()
		})
	}
	recordOpsAudit(ctx, s.svcCtx, audit.EventNodeDrain, nodeId, "success", nil)

	return nil
}

func (s *NodeService) GetMeta(ctx context.Context, nodeId string) (map[string]string, error) {
	store := s.svcCtx.RegistryStore
	if store == nil {
		return nil, errors.New("registry store unavailable")
	}

	store.Mu().RLock()
	defer store.Mu().RUnlock()

	for _, sess := range store.AgentsUnsafe() {
		if sess != nil && sess.AgentID == nodeId {
			return sess.Labels, nil
		}
	}

	return nil, errors.New("node not found")
}

func (s *NodeService) Restart(ctx context.Context, nodeId string) error {
	store := s.svcCtx.RegistryStore
	if store == nil {
		return errors.New("registry store unavailable")
	}

	// Verify node exists
	store.Mu().RLock()
	found := false
	for _, sess := range store.AgentsUnsafe() {
		if sess != nil && sess.AgentID == nodeId {
			found = true
			break
		}
	}
	store.Mu().RUnlock()
	if !found {
		return errorx.NewNotFound("node not found: " + nodeId)
	}

	// Record restart in the persistent audit trail
	recordOpsAudit(ctx, s.svcCtx, audit.EventNodeRestart, nodeId, "initiated", nil)

	return nil
}

func (s *NodeService) Undrain(ctx context.Context, nodeId string) error {
	store := s.svcCtx.RegistryStore
	if store == nil {
		return errors.New("registry store unavailable")
	}

	// Verify node exists
	store.Mu().RLock()
	found := false
	for _, sess := range store.AgentsUnsafe() {
		if sess != nil && sess.AgentID == nodeId {
			found = true
			break
		}
	}
	store.Mu().RUnlock()
	if !found {
		return errorx.NewNotFound("node not found: " + nodeId)
	}

	// Clear transient node state; audit goes to audit_records.
	if s.svcCtx.OpsStateStore != nil {
		_, _ = s.svcCtx.OpsStateStore.Update(func(state *svc.OpsState) {
			if state.Nodes.Drained != nil {
				delete(state.Nodes.Drained, nodeId)
			}
			state.Nodes.UpdatedAt = time.Now()
		})
	}
	recordOpsAudit(ctx, s.svcCtx, audit.EventNodeUndrain, nodeId, "success", nil)

	return nil
}
