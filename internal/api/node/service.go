package node

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"encoding/json"
	"gorm.io/datatypes"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/pkg/protocol"
)

type Service struct {
	svcCtx *svc.ServiceContext

	// #24: short-TTL cache for host task aggregation — the /ops/schedules
	// overview stats and grouped table share one collection pass, so every
	// render does not hammer the agent sessions
	cronJobsMu       sync.Mutex
	cronJobsCache    []NodeCronJobsReport
	cronJobsCacheExp time.Time
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

// NodeCronJob is a host cron job entry (parsed on the agent side from crontab +
// /etc/cron.d).
type NodeCronJob struct {
	Schedule   string `json:"schedule"`
	Command    string `json:"command"`
	User       string `json:"user"`
	SourceFile string `json:"sourceFile"`
	Enabled    bool   `json:"enabled"`
}

// ListCronJobs proxies through the session table to the online Agent and reads
// the cron jobs of the host it runs on.
func (s *Service) ListCronJobs(ctx context.Context, nodeID string) ([]NodeCronJob, error) {
	if err := s.ensureCronJobsDeps(); err != nil {
		return nil, err
	}
	return s.collectCronJobs(ctx, nodeID)
}

// ensureCronJobsDeps checks the collection chain dependencies (session table).
func (s *Service) ensureCronJobsDeps() error {
	if s.svcCtx.AgentSessions == nil {
		return errors.New("会话表未初始化")
	}
	return nil
}

// collectCronJobs performs the raw single-node collection (no cache), shared by
// the single-node query and the full aggregation.
func (s *Service) collectCronJobs(ctx context.Context, nodeID string) ([]NodeCronJob, error) {
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

// NodeCronJobsReport is the host cron collection result for one node; an
// unreachable node gets ok=false with a reason and does not interrupt the
// overall aggregation (#24: /ops/schedules groups and displays by source).
type NodeCronJobsReport struct {
	NodeID   string        `json:"nodeId"`
	NodeName string        `json:"nodeName"`
	Status   string        `json:"status"`
	OK       bool          `json:"ok"`
	Error    string        `json:"error,omitempty"`
	Jobs     []NodeCronJob `json:"jobs"`
}

// nodeCronJobsCacheTTL bounds how stale the aggregated result may get (same
// order of magnitude as the #14 classification aggregation).
const nodeCronJobsCacheTTL = 30 * time.Second

// ListAllCronJobs aggregates the host cron jobs of all nodes. Online nodes are
// collected in parallel through the session table; offline/failed nodes are
// marked ok=false with a reason. The result is cached for
// nodeCronJobsCacheTTL so the overview + table double render does not hammer
// the agents.
func (s *Service) ListAllCronJobs(ctx context.Context) ([]NodeCronJobsReport, error) {
	s.cronJobsMu.Lock()
	if s.cronJobsCache != nil && time.Now().Before(s.cronJobsCacheExp) {
		cached := s.cronJobsCache
		s.cronJobsMu.Unlock()
		return cached, nil
	}
	s.cronJobsMu.Unlock()

	if s.svcCtx.NodeModel == nil {
		return nil, errors.New("节点表未初始化")
	}
	nodes, err := s.svcCtx.NodeModel.List(ctx, model.ListNodesOptions{})
	if err != nil {
		return nil, err
	}

	reports := make([]NodeCronJobsReport, len(nodes))
	var wg sync.WaitGroup
	for i := range nodes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			n := utils.BuildNode(&nodes[i])
			rep := NodeCronJobsReport{
				NodeID:   n.Id,
				NodeName: n.Name,
				Status:   n.Status,
				Jobs:     []NodeCronJob{},
			}
			if err := s.ensureCronJobsDeps(); err != nil {
				rep.Error = err.Error()
			} else if jobs, err := s.collectCronJobs(ctx, n.Id); err != nil {
				rep.Error = err.Error()
			} else {
				rep.OK = true
				rep.Jobs = jobs
			}
			reports[i] = rep
		}(i)
	}
	wg.Wait()

	s.cronJobsMu.Lock()
	s.cronJobsCache = reports
	s.cronJobsCacheExp = time.Now().Add(nodeCronJobsCacheTTL)
	s.cronJobsMu.Unlock()
	return reports, nil
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
