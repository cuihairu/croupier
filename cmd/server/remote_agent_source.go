package main

import (
	"context"

	"github.com/cuihairu/croupier/internal/cluster"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"gorm.io/gorm"
)

// ownerAgentSource 实现 dispatch.RemoteAgentSource：以共享归属表
// （cluster_agent_owners，TTL 内即活跃）为远端在线全集，只保留对端实例
// 持有的 agent（本实例自有的走本地 registry，不重复供应），明细从共享
// agent_sessions 快照表读取（Providers/Functions 已 JSON 落库，dispatcher
// 侧按同一 agentCanInvoke 复检）。归属表不可达/快照行缺失（owner 已
// Release 的竞态）返回空或部分集——候选集兜底是尽力而为，不放大故障。
type ownerAgentSource struct {
	owners func(ctx context.Context) ([]cluster.AgentOwnerRecord, error)
	db     *gorm.DB
	selfID string
}

// RemoteAgentSessions 返回对端实例持有、scope 匹配且快照表有活跃行的
// agent 会话。scoped=false 时不过滤 scope（与本地候选同语义）。
func (s *ownerAgentSource) RemoteAgentSessions(ctx context.Context, gameID, env string, scoped bool) ([]*reg.AgentSession, error) {
	if s == nil || s.owners == nil || s.db == nil {
		return nil, nil
	}
	recs, err := s.owners(ctx)
	if err != nil {
		return nil, err
	}
	agentIDs := make([]string, 0, len(recs))
	for _, rec := range recs {
		if rec.InstanceID == s.selfID {
			continue
		}
		if scoped && (rec.GameID != gameID || rec.Env != env) {
			continue
		}
		agentIDs = append(agentIDs, rec.AgentID)
	}
	if len(agentIDs) == 0 {
		return nil, nil
	}
	// 快照行缺失的 agent 静默跳过（归属表 TTL 与快照行生命期不同步的
	// 竞态窗口）：与 /functions/instances 聚合的兜底语义一致。
	return reg.NewAgentSessionModel(s.db).LoadActiveSessionsByAgentIDs(ctx, agentIDs)
}

var _ dispatch.RemoteAgentSource = (*ownerAgentSource)(nil)

// activeAgentIDDirectory 适配 ControlService.SetActiveAgentDirectory：共享
// 归属表（TTL 内即活跃）的 agent 全集，不限实例——本实例与对端持有连接
// 的 agent 都有 ClaimOwner 行，无行 = 无任何实例持有 = 快照行是僵尸。
type activeAgentIDDirectory struct {
	resolver cluster.OwnerStore
}

// ActiveAgentIDs 返回归属表活跃 agent ID 去重集合。
func (d activeAgentIDDirectory) ActiveAgentIDs(ctx context.Context) ([]string, error) {
	if d.resolver == nil {
		return nil, nil
	}
	recs, err := d.resolver.ListAliveOwners(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(recs))
	seen := make(map[string]bool, len(recs))
	for _, rec := range recs {
		if seen[rec.AgentID] {
			continue
		}
		seen[rec.AgentID] = true
		ids = append(ids, rec.AgentID)
	}
	return ids, nil
}
