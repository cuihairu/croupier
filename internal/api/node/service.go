package node

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"encoding/json"
	"gorm.io/datatypes"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/pkg/protocol"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List returns the list of nodes
func (s *Service) List(ctx context.Context, req *NodesListRequest) (*NodesListResponse, error) {
	opts := model.ListNodesOptions{
		Type:   strings.TrimSpace(req.Type),
		Status: strings.TrimSpace(req.Status),
	}

	nodes, err := s.svcCtx.NodeModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]Node, 0, len(nodes))
	for i := range nodes {
		n := utils.BuildNode(&nodes[i])
		items = append(items, Node{
			ID:        n.Id,
			Name:      n.Name,
			Type:      n.Type,
			Status:    n.Status,
			IP:        n.IP,
			Port:      n.Port,
			Resources: n.Resources,
			UpdatedAt: n.UpdatedAt,
		})
	}

	return &NodesListResponse{
		Items: items,
	}, nil
}

// GetMeta returns the metadata of a node
func (s *Service) GetMeta(ctx context.Context, req *NodeMetaRequest) (*NodeMetaResponse, error) {
	nodeID, err := utils.ValidateNodeID(req.ID)
	if err != nil {
		return nil, err
	}

	node, err := s.svcCtx.NodeModel.FindByNodeID(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	return &NodeMetaResponse{
		Meta: node.Meta,
	}, nil
}

// UpdateMeta updates the metadata of a node
func (s *Service) UpdateMeta(ctx context.Context, req *NodeMetaUpdateRequest) (*NodeMetaResponse, error) {
	nodeID, err := utils.ValidateNodeID(req.ID)
	if err != nil {
		return nil, err
	}

	metaMap, ok := req.Meta.(map[string]interface{})
	if !ok {
		return nil, errors.New("meta 必须是对象")
	}

	if err := s.svcCtx.NodeModel.UpdateMeta(ctx, nodeID, map[string]interface{}{
		"meta": datatypes.JSONMap(metaMap),
	}); err != nil {
		return nil, err
	}

	node, err := s.svcCtx.NodeModel.FindByNodeID(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	return &NodeMetaResponse{
		Meta: node.Meta,
	}, nil
}

// Drain drains a node
func (s *Service) Drain(ctx context.Context, req *NodeDrainRequest) error {
	nodeID, err := utils.ValidateNodeID(req.ID)
	if err != nil {
		return err
	}

	if _, err := s.svcCtx.NodeModel.FindByNodeID(ctx, nodeID); err != nil {
		return err
	}

	status := "draining"
	if req.Timeout > 0 {
		status = fmt.Sprintf("draining:%d", req.Timeout)
	}

	return s.svcCtx.NodeModel.UpdateStatus(ctx, nodeID, status)
}

// Undrain undrains a node
func (s *Service) Undrain(ctx context.Context, req *NodeActionRequest) error {
	nodeID, err := utils.ValidateNodeID(req.ID)
	if err != nil {
		return err
	}

	if _, err := s.svcCtx.NodeModel.FindByNodeID(ctx, nodeID); err != nil {
		return err
	}

	return s.svcCtx.NodeModel.UpdateStatus(ctx, nodeID, "active")
}

// Restart restarts a node
func (s *Service) Restart(ctx context.Context, req *NodeActionRequest) error {
	nodeID, err := utils.ValidateNodeID(req.ID)
	if err != nil {
		return err
	}

	if _, err := s.svcCtx.NodeModel.FindByNodeID(ctx, nodeID); err != nil {
		return err
	}

	return s.svcCtx.NodeModel.UpdateStatus(ctx, nodeID, "restarting")
}

// NodeCronJob 主机定时任务条目（agent 侧解析 crontab + /etc/cron.d）。
type NodeCronJob struct {
	Schedule   string `json:"schedule"`
	Command    string `json:"command"`
	User       string `json:"user"`
	SourceFile string `json:"sourceFile"`
	Enabled    bool   `json:"enabled"`
}

// ListCronJobs 经会话表代理到在线 Agent，读取其所在主机的定时任务。
func (s *Service) ListCronJobs(ctx context.Context, nodeID string) ([]NodeCronJob, error) {
	if s.svcCtx.AgentSessions == nil {
		return nil, errors.New("会话表未初始化")
	}
	caller, ok := s.svcCtx.AgentSessions.ResolveSessionCaller(nodeID)
	if !ok {
		return nil, errors.New("节点不在线")
	}
	_, respBody, err := caller.Call(ctx, protocol.MsgListCronJobsRequest, []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("调用 agent 失败: %w", err)
	}
	var resp struct {
		Jobs []NodeCronJob `json:"jobs"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("解析 agent 响应失败: %w", err)
	}
	return resp.Jobs, nil
}

// ListCommands returns the list of available node commands
func (s *Service) ListCommands(ctx context.Context, req *NodeCommandsRequest) (*NodeCommandsResponse, error) {
	commands, err := s.svcCtx.NodeModel.ListCommands(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]NodeCommand, 0, len(commands))
	for _, cmd := range commands {
		items = append(items, NodeCommand{
			Name:        cmd.Name,
			Description: cmd.Description,
		})
	}

	return &NodeCommandsResponse{
		Items: items,
	}, nil
}
