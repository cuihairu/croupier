package dispatch

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	apperrors "github.com/cuihairu/croupier/internal/errors"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
)

// ListFunctionAgents returns agent IDs that currently expose the function.
func (d *Dispatcher) ListFunctionAgents(functionID string) []string {
	now := time.Now()
	d.store.Mu().RLock()
	defer d.store.Mu().RUnlock()
	var ids []string
	for _, agent := range d.store.AgentsUnsafe() {
		if agent == nil || agent.AgentID == "" {
			continue
		}
		if !agent.ExpireAt.After(now) {
			continue
		}
		if meta, ok := agent.Functions[functionID]; ok && meta.Enabled {
			ids = append(ids, agent.AgentID)
		}
	}
	return ids
}

func pickAgentByHash(candidates []*reg.AgentSession, key string) *reg.AgentSession {
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return candidates[0]
	}
	// FNV-1a
	var h uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= 16777619
	}
	idx := int(h % uint32(len(candidates)))
	return candidates[idx]
}

// pickAgent returns a live agent that owns the function without a scope
// constraint. Scoped HTTP execution always uses pickAgentWithRouting.
func (d *Dispatcher) pickAgent(functionID string) (*reg.AgentSession, error) {
	return d.pickAgentInScope(functionID, "", "", false)
}

func (d *Dispatcher) pickAgentInScope(functionID, gameID, env string, scoped bool) (*reg.AgentSession, error) {
	candidates := d.listAgentsForFunctionInScope(functionID, gameID, env, scoped)
	return d.selectAgent(functionID, candidates, gameID, env, scoped)
}

func (d *Dispatcher) selectAgent(functionID string, candidates []*reg.AgentSession, gameID, env string, scoped bool) (*reg.AgentSession, error) {
	if len(candidates) == 0 {
		return nil, d.noLiveAgentError(functionID, gameID, env, scoped)
	}

	// Use load balancer if HA is enabled
	d.mu.RLock()
	useLoadBalancer := d.loadBalancer != nil
	tracker := d.healthTracker
	d.mu.RUnlock()

	if useLoadBalancer && tracker != nil {
		// Build candidates with health state
		candidateList := d.loadBalancer.BuildCandidates(candidates, functionID)
		if len(candidateList) == 0 {
			return nil, noHealthyAgentError(functionID, gameID, env, scoped)
		}

		selected, err := d.loadBalancer.Select(functionID, candidateList)
		if err != nil {
			return nil, err
		}
		return selected.Session, nil
	}

	// Fallback to original min_id selection
	var chosen *reg.AgentSession
	for _, agent := range candidates {
		if chosen == nil || agent.AgentID < chosen.AgentID {
			chosen = agent
		}
	}
	return chosen, nil
}

func (d *Dispatcher) pickAgentWithRouting(ctx context.Context, functionID string, metadata map[string]string, exclude ...map[string]bool) (*reg.AgentSession, error) {
	gameID, env, scoped, err := routingScopeFromMetadata(metadata)
	if err != nil {
		return nil, err
	}

	serviceID := strings.TrimSpace(metadata["targetServiceId"])
	hashKey := strings.TrimSpace(metadata["hashKey"])
	candidates := d.listAgentsForFunctionInScope(functionID, gameID, env, scoped)
	if len(exclude) > 0 && exclude[0] != nil {
		filtered := make([]*reg.AgentSession, 0, len(candidates))
		for _, a := range candidates {
			if !exclude[0][a.AgentID] {
				filtered = append(filtered, a)
			}
		}
		candidates = filtered
	}
	// 候选集扩展：exclude 后为空（本地无候选或 failover 已耗尽本地候选）
	// 且装配了远端目录时，同步查归属表补远端候选——30s
	// refreshRemoteSnapshots 回灌窗口内的新注册、回灌失败或快照缺失场景
	// 由此兜底；远端候选被选中后经 callAgentRouted 转发到 owner 执行。
	// 注意触发条件是「过滤后为空」而非「本地列表为空」，否则 failover
	// 的第 2、3 轮永远不会尝试远端。扩展结果同样按 exclude 过滤——已
	// 试失败的远端候选不重复供应，全部耗尽后由 noLiveAgentError 收口
	// （failover exhausted → 503 语义依赖这里返回空）。
	if len(candidates) == 0 {
		var excluded map[string]bool
		if len(exclude) > 0 {
			excluded = exclude[0]
		}
		for _, a := range d.expandRemoteCandidates(ctx, functionID, gameID, env, scoped) {
			if a == nil || (excluded != nil && excluded[a.AgentID]) {
				continue
			}
			candidates = append(candidates, a)
		}
	}
	if len(candidates) == 0 {
		return nil, d.noLiveAgentError(functionID, gameID, env, scoped)
	}

	// Targeted: choose the agent that owns the service_id.
	if serviceID != "" {
		var chosen *reg.AgentSession
		for _, agent := range candidates {
			if agentHasService(agent, serviceID, functionID) {
				if chosen == nil || agent.AgentID < chosen.AgentID {
					chosen = agent
				}
			}
		}
		if chosen != nil {
			return chosen, nil
		}
		return nil, apperrors.Newf(apperrors.ErrCodeServiceUnavailable, "select_agent", nil,
			"no live agent for function %s%s with service_id %s", functionID, formatRoutingScope(gameID, env, scoped), serviceID)
	}

	// Hash: choose a stable agent among the already scope-filtered candidates.
	if hashKey != "" {
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].AgentID < candidates[j].AgentID
		})
		chosen := pickAgentByHash(candidates, hashKey)
		// pickAgentByHash 仅在入参切片为空时返回 nil，上方 len(candidates)==0
		// 已提前返回错误，此处 chosen 恒非 nil，nil 分支为死代码已删。
		return chosen, nil
	}

	return d.selectAgent(functionID, candidates, gameID, env, scoped)
}

func routingScopeFromMetadata(metadata map[string]string) (gameID, env string, scoped bool, err error) {
	if len(metadata) == 0 {
		return "", "", false, nil
	}
	gameID = strings.TrimSpace(metadata["gameId"])
	env = strings.TrimSpace(metadata["env"])
	if gameID == "" && env == "" {
		return "", "", false, nil
	}
	if gameID == "" || env == "" {
		return "", "", false, fmt.Errorf("game_id and env must be supplied together for agent routing")
	}
	return gameID, env, true, nil
}

func agentCanInvoke(agent *reg.AgentSession, functionID string, now time.Time, gameID, env string, scoped bool) bool {
	if agent == nil || !agent.ExpireAt.After(now) {
		return false
	}
	// draining 节点不再接收新路由（ops drain；agent 侧 ProviderSession
	// draining 是第二道兜底）。在途请求由 drain 宽限窗收口，不在此取消。
	if agent.Draining {
		return false
	}
	if scoped && (strings.TrimSpace(agent.GameID) != gameID || strings.TrimSpace(agent.Env) != env) {
		return false
	}
	meta, ok := agent.Functions[functionID]
	return ok && meta.Enabled
}

func formatRoutingScope(gameID, env string, scoped bool) string {
	if !scoped {
		return ""
	}
	return fmt.Sprintf(" in game_id %s env %s", gameID, env)
}

// noLiveAgentError 收口「调度无可用候选」。函数被函数级版本门槛拦截时
// （注册警告 function_version_below_minimum 且门槛仍在配置），返回专属
// 错误码 FUNCTION_VERSION_BELOW_MINIMUM——与「agent 真没活」按码区分，
// 避免运维把门槛拦截误判为连接故障去查 agent；未命中回落 503
// service_unavailable 原语义（E9）。
func (d *Dispatcher) noLiveAgentError(functionID, gameID, env string, scoped bool) error {
	if w, ok := d.functionVersionFloorBlock(functionID, gameID, env, scoped); ok {
		version := w.Version
		if version == "" {
			version = "unknown"
		}
		return apperrors.Newf(apperrors.ErrCodeFunctionVersionBelowMinimum, "invoke", nil,
			"function %s%s blocked by version floor: registered version %s is below the configured minimum (upgrade the game server and re-register, or adjust the version floor in the function catalog)",
			functionID, formatRoutingScope(gameID, env, scoped), version)
	}
	return apperrors.Newf(apperrors.ErrCodeServiceUnavailable, "invoke", nil,
		"no live agent for function %s%s", functionID, formatRoutingScope(gameID, env, scoped))
}

// functionVersionFloorBlock 判定函数是否存在有效的版本门槛拦截证据：
// 注册警告（某次注册确被拒）与门槛仍配置（权威面，删除门槛不自动清
// 警告）双条件同时成立才命中——门槛已删的陈旧警告不得把「agent 真没
// 活」误述成「被门槛拦截」。
func (d *Dispatcher) functionVersionFloorBlock(functionID, gameID, env string, scoped bool) (reg.FunctionRegistrationWarning, bool) {
	if d == nil || d.store == nil || !scoped {
		return reg.FunctionRegistrationWarning{}, false
	}
	warnings := d.store.ListRegistrationWarnings(reg.RegistrationWarningFilter{
		GameID:     gameID,
		Env:        env,
		FunctionID: functionID,
		Code:       reg.WarningCodeFunctionVersionBelowMinimum,
		Limit:      1,
	})
	if len(warnings) == 0 {
		return reg.FunctionRegistrationWarning{}, false
	}
	if d.store.GetFunctionVersionFloor(gameID, env, functionID) == "" {
		return reg.FunctionRegistrationWarning{}, false
	}
	return warnings[0], true
}

func noHealthyAgentError(functionID, gameID, env string, scoped bool) error {
	return apperrors.Newf(apperrors.ErrCodeServiceUnavailable, "invoke", nil,
		"no healthy agents available for function %s%s", functionID, formatRoutingScope(gameID, env, scoped))
}
