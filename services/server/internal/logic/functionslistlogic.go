// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionsListLogic {
	return &FunctionsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionsListLogic) FunctionsList(req *types.FunctionsQuery) (resp *types.FunctionsListResponse, err error) {
	results := make([]types.FunctionRecord, 0)
	store := l.svcCtx.RegistryStore
	enabledVal, enabledSet := parseBoolFilter(req.Enabled)
	categoryFilter := strings.TrimSpace(req.Category)

	if store != nil {
		store.Mu().RLock()
		for agentID, agent := range store.AgentsUnsafe() {
			if agent == nil {
				continue
			}
			if req.GameId != "" && agent.GameID != req.GameId {
				continue
			}
			if req.Env != "" && agent.Env != req.Env {
				continue
			}
			if req.AgentId != "" && agentID != req.AgentId {
				continue
			}
			for funcID, meta := range agent.Functions {
				if enabledSet && meta.Enabled != enabledVal {
					continue
				}
				desc := l.svcCtx.FunctionDescriptor(funcID)
				if categoryFilter != "" && desc != nil && desc.Category != categoryFilter {
					continue
				}
				record := types.FunctionRecord{
					Id:       funcID,
					Enabled:  meta.Enabled,
					Source:   "agent",
					SourceId: agentID,
					AgentId:  agentID,
					GameId:   agent.GameID,
					Env:      agent.Env,
					RpcAddr:  agent.RPCAddr,
					Version:  agent.Version,
					LastSeen: formatTime(agent.ExpireAt),
				}
				if desc != nil {
					record.Version = desc.Version
					record.Category = desc.Category
					record.Risk = desc.Risk
				}
				results = append(results, record)
			}
		}
		store.Mu().RUnlock()

		for _, caps := range store.ListProviderCaps() {
			var manifest map[string]any
			if err := json.Unmarshal(caps.Manifest, &manifest); err != nil {
				continue
			}
			arr, _ := manifest["functions"].([]any)
			for _, item := range arr {
				obj, _ := item.(map[string]any)
				if obj == nil {
					continue
				}
				id := strFromMap(obj, "id")
				if id == "" {
					continue
				}
				if categoryFilter != "" {
					if strFromMap(obj, "category") != categoryFilter {
						continue
					}
				}
				if req.AgentId != "" || req.GameId != "" || req.Env != "" {
					// provider entries unrelated to these filters
					if req.AgentId != "" {
						continue
					}
				}
				record := types.FunctionRecord{
					Id:         id,
					Enabled:    true,
					Source:     "provider",
					SourceId:   caps.ID,
					ProviderId: caps.ID,
					Version:    strFromMap(obj, "version"),
					Category:   strFromMap(obj, "category"),
					Risk:       strFromMap(obj, "risk"),
					UpdatedAt:  formatTime(caps.UpdatedAt),
				}
				results = append(results, record)
			}
		}
	}

	return &types.FunctionsListResponse{
		Functions: results,
		Total:     int64(len(results)),
	}, nil
}
