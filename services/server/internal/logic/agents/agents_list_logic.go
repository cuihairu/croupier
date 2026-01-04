package agents

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AgentsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAgentsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AgentsListLogic {
	return &AgentsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AgentsListLogic) AgentsList(req *types.AgentsListRequest) (*types.AgentsListResponse, error) {
	// Permission check
	_, roles, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	roleNames := utils.RoleNamesFromModels(roles)
	permIDs, err := utils.PermissionIDsFromRoles(l.ctx, l.svcCtx, roles)
	if err != nil {
		return nil, err
	}
	if !utils.HasAdminRole(roleNames) && !utils.HasPermissionID(permIDs, "agents:read") && !utils.HasPermissionID(permIDs, "*") {
		return nil, errorx.NewForbidden("无权查看 Agent 列表")
	}

	agents := make([]types.AgentItem, 0)

	if store := l.svcCtx.RegistryStore; store != nil {
		store.Mu().RLock()
		defer store.Mu().RUnlock()

		for _, sess := range store.AgentsUnsafe() {
			if sess == nil {
				continue
			}

			// Filter by game_id
			if req.GameID != "" && sess.GameID != req.GameID {
				continue
			}

			// Filter by env
			if req.Env != "" && sess.Env != req.Env {
				continue
			}

			ttl, healthy := ttlAndHealth(sess)

			agentItem := types.AgentItem{
				AgentID:      sess.AgentID,
				GameID:       sess.GameID,
				Env:          sess.Env,
				RPCAddr:      sess.RPCAddr,
				Functions:    utils.CountEnabledFunctions(sess.Functions),
				Healthy:      healthy,
				ExpiresInSec: ttl,
				LastSeen:     sess.ExpireAt.Add(-time.Duration(ttl) * time.Second).Format(time.RFC3339),
			}

			// Filter by status
			if req.Status != "" {
				if req.Status == "healthy" && !healthy {
					continue
				}
				if req.Status == "unhealthy" && healthy {
					continue
				}
			}

			// Get version from processes if available
			if len(sess.Processes) > 0 {
				agentItem.Version = sess.Processes[0].Version
			}

			agents = append(agents, agentItem)
		}
	}

	// Sort by game_id, then agent_id
	sort.Slice(agents, func(i, j int) bool {
		if agents[i].GameID == agents[j].GameID {
			return agents[i].AgentID < agents[j].AgentID
		}
		return agents[i].GameID < agents[j].GameID
	})

	// Pagination
	total := int64(len(agents))
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 || req.Size > 100 {
		req.Size = 20
	}

	start := (req.Page - 1) * req.Size
	end := start + req.Size
	if start >= len(agents) {
		return &types.AgentsListResponse{
			Agents: []types.AgentItem{},
			Total:  total,
			Page:   req.Page,
			Size:   req.Size,
		}, nil
	}
	if end > len(agents) {
		end = len(agents)
	}

	return &types.AgentsListResponse{
		Agents: agents[start:end],
		Total:  total,
		Page:   req.Page,
		Size:   req.Size,
	}, nil
}

func ttlAndHealth(sess *registry.AgentSession) (int, bool) {
	if sess == nil || sess.ExpireAt.IsZero() {
		return 0, false
	}
	ttl := int(time.Until(sess.ExpireAt).Seconds())
	if ttl < 0 {
		ttl = 0
	}
	return ttl, ttl > 0
}

func inferCategory(functionID string) string {
	if idx := strings.Index(functionID, "."); idx > 0 {
		return functionID[:idx]
	}
	return ""
}
