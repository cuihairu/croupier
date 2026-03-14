package registry

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/logic/assignment"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// GetRegistry retrieves registry information including agents, functions, and coverage
func (s *Service) GetRegistry(ctx context.Context, req *RegistryRequest) (*RegistryResponse, error) {
	// Allow public read access to agents and functions (for function catalog browsing)
	// Only require permission for sensitive operations
	username, _ := utils.CurrentUsername(ctx)
	if username != "" {
		if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权查看注册表", "admin:all", "registry:read", "registry:manage"); err != nil {
			return nil, err
		}
	}
	// Unauthenticated request: allow read-only access to agents and functions

	agents := make([]RegistryAgent, 0)
	functionMap := make(map[string]*RegistryFunction)
	coverage := make(map[string]*RegistryCoverage)

	if store := s.svcCtx.RegistryStore; store != nil {
		store.Mu().RLock()
		for _, sess := range store.AgentsUnsafe() {
			if sess == nil || strings.TrimSpace(sess.AgentID) == "" {
				continue
			}
			ttl, healthy := ttlAndHealth(sess)
			agents = append(agents, RegistryAgent{
				AgentID:      sess.AgentID,
				GameID:       sess.GameID,
				Env:          sess.Env,
				RpcAddr:      sess.RPCAddr,
				Functions:    utils.CountEnabledFunctions(sess.Functions),
				Healthy:      healthy,
				ExpiresInSec: ttl,
			})
			for fnID, meta := range sess.Functions {
				if !meta.Enabled {
					continue
				}
				fnKey := sess.GameID + "::" + fnID
				fnEntry, ok := functionMap[fnKey]
				if !ok {
					fnEntry = &RegistryFunction{
						GameID: sess.GameID,
						ID:     fnID,
						Agents: []string{},
					}
					functionMap[fnKey] = fnEntry
				}
				fnEntry.Agents = append(fnEntry.Agents, sess.AgentID)

				gameEnv := buildGameEnvKey(sess.GameID, sess.Env)
				cov := coverage[gameEnv]
				if cov == nil {
					cov = &RegistryCoverage{
						GameEnv:   gameEnv,
						Functions: map[string]RegistryCoverageStat{},
					}
					coverage[gameEnv] = cov
				}
				stat := cov.Functions[fnID]
				stat.Healthy++
				if stat.Total < stat.Healthy {
					stat.Total = stat.Healthy
				}
				cov.Functions[fnID] = stat
			}
		}
		store.Mu().RUnlock()
	}

	assignments, _ := assignment.LoadAllAssignments(s.svcCtx)
	for key, functions := range assignments {
		cov := coverage[key]
		if cov == nil {
			cov = &RegistryCoverage{
				GameEnv:   key,
				Functions: map[string]RegistryCoverageStat{},
			}
			coverage[key] = cov
		}
		seenMissing := make(map[string]struct{})
		for _, fn := range functions {
			fn = strings.TrimSpace(fn)
			if fn == "" {
				continue
			}
			stat := cov.Functions[fn]
			if stat.Total < 1 {
				stat.Total = 1
			}
			if stat.Healthy == 0 {
				if _, ok := seenMissing[fn]; !ok {
					cov.Uncovered = append(cov.Uncovered, fn)
					seenMissing[fn] = struct{}{}
				}
			}
			cov.Functions[fn] = stat
		}
	}

	sort.Slice(agents, func(i, j int) bool {
		if agents[i].GameID == agents[j].GameID {
			return agents[i].AgentID < agents[j].AgentID
		}
		return agents[i].GameID < agents[j].GameID
	})

	functions := make([]RegistryFunction, 0, len(functionMap))
	for _, fn := range functionMap {
		sort.Strings(fn.Agents)
		functions = append(functions, *fn)
	}
	sort.Slice(functions, func(i, j int) bool {
		if functions[i].GameID == functions[j].GameID {
			return functions[i].ID < functions[j].ID
		}
		return functions[i].GameID < functions[j].GameID
	})

	covList := make([]RegistryCoverage, 0, len(coverage))
	for _, cov := range coverage {
		sort.Strings(cov.Uncovered)
		covList = append(covList, *cov)
	}
	sort.Slice(covList, func(i, j int) bool {
		return covList[i].GameEnv < covList[j].GameEnv
	})

	return &RegistryResponse{
		Agents:      agents,
		Functions:   functions,
		Assignments: assignments,
		Coverage:    covList,
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

func buildGameEnvKey(gameID, env string) string {
	return strings.TrimSpace(gameID) + "|" + strings.TrimSpace(env)
}
