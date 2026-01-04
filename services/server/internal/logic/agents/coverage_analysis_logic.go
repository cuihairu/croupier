package agents

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/assignment"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CoverageAnalysisLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCoverageAnalysisLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CoverageAnalysisLogic {
	return &CoverageAnalysisLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CoverageAnalysisLogic) CoverageAnalysis(req *types.CoverageAnalysisRequest) (*types.CoverageAnalysisResponse, error) {
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
		return nil, errorx.NewForbidden("无权查看覆盖率分析")
	}

	functionMap := make(map[string]*CoverageDetail)
	agentsByGame := make(map[string]int)
	functionsByCategory := make(map[string]int)

	// Collect from registry
	if store := l.svcCtx.RegistryStore; store != nil {
		store.Mu().RLock()
		defer store.Mu().RUnlock()

		for _, sess := range store.AgentsUnsafe() {
			if sess == nil {
				continue
			}

			// Filter by game_id if specified
			if req.GameID != "" && sess.GameID != req.GameID {
				continue
			}

			// Filter by env if specified
			if req.Env != "" && sess.Env != req.Env {
				continue
			}

			gameEnv := buildGameEnvKey(sess.GameID, sess.Env)
			agentsByGame[gameEnv]++

			healthy := sess.ExpireAt.After(time.Now())

			for fnID, meta := range sess.Functions {
				if !meta.Enabled || !healthy {
					continue
				}

				detail, exists := functionMap[fnID]
				if !exists {
					category := inferCategoryForCoverage(fnID)
					functionsByCategory[category]++
					detail = &CoverageDetail{
						FunctionID: fnID,
						Category:   category,
						Agents:     []string{},
					}
					functionMap[fnID] = detail
				}

				// Add agent if not already present
				found := false
				for _, a := range detail.Agents {
					if a == sess.AgentID {
						found = true
						break
					}
				}
				if !found {
					detail.Agents = append(detail.Agents, sess.AgentID)
				}
			}
		}
	}

	// Load assignments for required functions
	assignments, _ := assignment.LoadAllAssignments(l.svcCtx)
	for gameEnv, functions := range assignments {
		// Filter by game_id/env if specified
		if req.GameID != "" || req.Env != "" {
			parts := strings.Split(gameEnv, "|")
			if len(parts) == 2 {
				if req.GameID != "" && parts[0] != req.GameID {
					continue
				}
				if req.Env != "" && parts[1] != req.Env {
					continue
				}
			}
		}

		for _, fn := range functions {
			fn = strings.TrimSpace(fn)
			if fn == "" {
				continue
			}

			if _, exists := functionMap[fn]; !exists {
				category := inferCategoryForCoverage(fn)
				functionsByCategory[category]++
				functionMap[fn] = &CoverageDetail{
					FunctionID: fn,
					Category:   category,
					Agents:     []string{},
					Status:     "uncovered",
				}
			}
		}
	}

	// Build response items
	details := make([]types.CoverageDetailItem, 0, len(functionMap))
	totalFunctions := len(functionMap)
	coveredFunctions := 0

	for _, detail := range functionMap {
		if len(detail.Agents) > 0 {
			detail.Status = "covered"
			coveredFunctions++
		} else if detail.Status == "" {
			detail.Status = "uncovered"
		}

		details = append(details, types.CoverageDetailItem{
			FunctionID: detail.FunctionID,
			Category:   detail.Category,
			Agents:     detail.Agents,
			Status:     detail.Status,
		})
	}

	// Sort by function_id
	sort.Slice(details, func(i, j int) bool {
		return details[i].FunctionID < details[j].FunctionID
	})

	coveragePercentage := 0.0
	if totalFunctions > 0 {
		coveragePercentage = float64(coveredFunctions) / float64(totalFunctions) * 100
	}

	return &types.CoverageAnalysisResponse{
		TotalFunctions:      int64(totalFunctions),
		CoveredFunctions:    coveredFunctions,
		CoveragePercentage:  coveragePercentage,
		FunctionsByCategory: functionsByCategory,
		AgentsByGame:        agentsByGame,
		CoverageDetails:     details,
	}, nil
}

type CoverageDetail struct {
	FunctionID string
	Category   string
	Agents     []string
	Status     string
}

func buildGameEnvKey(gameID, env string) string {
	return strings.TrimSpace(gameID) + "|" + strings.TrimSpace(env)
}

// inferCategoryForCoverage is a local version to avoid conflicts
func inferCategoryForCoverage(functionID string) string {
	if idx := strings.Index(functionID, "."); idx > 0 {
		return functionID[:idx]
	}
	return ""
}
