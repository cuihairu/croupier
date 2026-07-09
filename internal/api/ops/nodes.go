package ops

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
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
	store := s.svcCtx.RegistryStore
	if store == nil {
		return []Node{}, nil
	}

	store.Mu().RLock()
	defer store.Mu().RUnlock()

	nodes := make([]Node, 0)
	for _, sess := range store.AgentsUnsafe() {
		if sess == nil {
			continue
		}
		if gameId != "" && sess.GameID != gameId {
			continue
		}

		nodes = append(nodes, Node{
			Id:       sess.AgentID,
			Hostname: sess.Labels["hostname"],
			Addr:     sess.RPCAddr,
			GameId:   sess.GameID,
			Env:      sess.Env,
			Status:   "active",
			Labels:   sess.Labels,
			LastSeen: utils.FormatTimestamp(sess.LastSeen),
		})
	}

	return nodes, nil
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
	return errorx.NewNotImplemented("node drain is not implemented")
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
	return errorx.NewNotImplemented("node restart is not implemented")
}

func (s *NodeService) Undrain(ctx context.Context, nodeId string) error {
	return errorx.NewNotImplemented("node undrain is not implemented")
}
