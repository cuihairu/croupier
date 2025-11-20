// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegistryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegistryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegistryLogic {
	return &RegistryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RegistryLogic) Registry() (resp *types.RegistryResponse, err error) {
	assignments := l.svcCtx.AssignmentsSnapshot()
	if assignments == nil {
		assignments = map[string][]string{}
	}

	regStore := l.svcCtx.RegistryStore
	if regStore == nil {
		return &types.RegistryResponse{
			Agents:      []types.RegistryAgent{},
			Functions:   []types.RegistryFunction{},
			Assignments: assignments,
			Coverage:    []types.RegistryCoverage{},
		}, nil
	}

	var agents []types.RegistryAgent
	fnCountAll := map[string]map[string]int64{}
	fnCountHealthy := map[string]map[string]int64{}

	now := time.Now()
	regStore.Mu().RLock()
	for _, agent := range regStore.AgentsUnsafe() {
		if agent == nil {
			continue
		}
		healthy := now.Before(agent.ExpireAt)
		expiresIn := int64(time.Until(agent.ExpireAt).Seconds())
		if expiresIn < 0 {
			expiresIn = 0
		}
		agents = append(agents, types.RegistryAgent{
			AgentId:      agent.AgentID,
			GameId:       agent.GameID,
			Env:          agent.Env,
			RpcAddr:      agent.RPCAddr,
			Ip:           pickAgentIP(agent.RPCAddr),
			Type:         "agent",
			Version:      agent.Version,
			Functions:    int64(len(agent.Functions)),
			Healthy:      healthy,
			ExpiresInSec: expiresIn,
		})
		if len(agent.Functions) == 0 {
			continue
		}
		for fid := range agent.Functions {
			if fnCountAll[agent.GameID] == nil {
				fnCountAll[agent.GameID] = map[string]int64{}
			}
			fnCountAll[agent.GameID][fid]++
			if healthy {
				if fnCountHealthy[agent.GameID] == nil {
					fnCountHealthy[agent.GameID] = map[string]int64{}
				}
				fnCountHealthy[agent.GameID][fid]++
			}
		}
	}
	regStore.Mu().RUnlock()

	var functions []types.RegistryFunction
	for gid, fnMap := range fnCountHealthy {
		for fid, count := range fnMap {
			functions = append(functions, types.RegistryFunction{
				GameId: gid,
				Id:     fid,
				Agents: count,
			})
		}
	}

	coverage := make([]types.RegistryCoverage, 0, len(assignments))
	for key, fids := range assignments {
		parts := strings.SplitN(key, "|", 2)
		gameID := parts[0]
		cov := make(map[string]types.RegistryFuncCoverage, len(fids))
		var uncovered []string
		for _, fid := range fids {
			healthy := fnCountHealthy[gameID][fid]
			total := fnCountAll[gameID][fid]
			cov[fid] = types.RegistryFuncCoverage{
				Healthy: healthy,
				Total:   total,
			}
			if healthy == 0 {
				uncovered = append(uncovered, fid)
			}
		}
		coverage = append(coverage, types.RegistryCoverage{
			GameEnv:   key,
			Functions: cov,
			Uncovered: uncovered,
		})
	}

	return &types.RegistryResponse{
		Agents:      agents,
		Functions:   functions,
		Assignments: assignments,
		Coverage:    coverage,
	}, nil
}

func pickAgentIP(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	if idx := strings.LastIndex(addr, ":"); idx > 0 {
		return addr[:idx]
	}
	return addr
}
