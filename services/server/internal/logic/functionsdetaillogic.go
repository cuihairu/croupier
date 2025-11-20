// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"encoding/json"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionsDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionsDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionsDetailLogic {
	return &FunctionsDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionsDetailLogic) FunctionsDetail(req *types.FunctionDetailRequest) (resp *types.FunctionDetailResponse, err error) {
	functionID := req.Id
	if functionID == "" {
		return &types.FunctionDetailResponse{}, nil
	}

	var (
		function  *types.FunctionDetail
		agents    []types.FunctionAgentInfo
		providers []types.FunctionProviderInfo
	)

	store := l.svcCtx.RegistryStore
	if store != nil {
		store.Mu().RLock()
		for aid, agent := range store.AgentsUnsafe() {
			if agent == nil {
				continue
			}
			if meta, ok := agent.Functions[functionID]; ok {
				agents = append(agents, types.FunctionAgentInfo{
					AgentId:  aid,
					GameId:   agent.GameID,
					Env:      agent.Env,
					RpcAddr:  agent.RPCAddr,
					Version:  agent.Version,
					Enabled:  meta.Enabled,
					LastSeen: formatTime(agent.ExpireAt),
				})
				if function == nil {
					function = &types.FunctionDetail{
						Id:      functionID,
						Source:  "agent",
						Version: agent.Version,
					}
				}
			}
		}
		store.Mu().RUnlock()

		for _, caps := range store.ListProviderCaps() {
			var manifest map[string]any
			if err := json.Unmarshal(caps.Manifest, &manifest); err != nil {
				continue
			}
			list, _ := manifest["functions"].([]any)
			for _, item := range list {
				obj, _ := item.(map[string]any)
				if obj == nil {
					continue
				}
				if id, _ := obj["id"].(string); id != functionID {
					continue
				}
				providers = append(providers, types.FunctionProviderInfo{
					ProviderId: caps.ID,
					Version:    strFromMap(obj, "version"),
					Category:   strFromMap(obj, "category"),
					Risk:       strFromMap(obj, "risk"),
					UpdatedAt:  formatTime(caps.UpdatedAt),
				})
				if function == nil {
					function = &types.FunctionDetail{
						Id:       functionID,
						Source:   "provider",
						Version:  strFromMap(obj, "version"),
						Category: strFromMap(obj, "category"),
						Risk:     strFromMap(obj, "risk"),
					}
				}
			}
		}
	}

	if function == nil {
		if desc := l.svcCtx.FunctionDescriptor(functionID); desc != nil {
			function = &types.FunctionDetail{
				Id:       desc.ID,
				Version:  desc.Version,
				Category: desc.Category,
				Risk:     desc.Risk,
				Source:   "descriptor",
			}
		}
	}

	return &types.FunctionDetailResponse{
		Function:  function,
		Agents:    agents,
		Providers: providers,
	}, nil
}
