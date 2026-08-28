package ops

import (
	"context"
	"errors"
)

// ---------- 集群实例视图（运维菜单拓扑展示） ----------

type ClusterInstance struct {
	InstanceID    string `json:"instanceId"`
	AdvertiseAddr string `json:"advertiseAddr"`
	Epoch         uint64 `json:"epoch"`
	StartedAt     string `json:"startedAt,omitempty"`
	// Self 标记当前处理请求的实例。
	Self bool `json:"self"`
	// Alive 来自成员表租约判定（未过期 = 在线）。
	Alive bool `json:"alive"`
	// AgentCount 该实例当前持有的 Agent 连接数（共享归属表统计）。
	AgentCount int64 `json:"agentCount"`
}

type ClusterInfoResponse struct {
	Enabled    bool              `json:"enabled"`
	Self       string            `json:"self,omitempty"`
	Items      []ClusterInstance `json:"items"`
	Total      int               `json:"total"`
	AliveCount int               `json:"aliveCount"`
	// LbStats LB 监控可用性提示（nil = 未配置 Prometheus，前端隐藏对账列）。
	LbStats *LBStatsInfo `json:"lbStats,omitempty"`
}

// LBStatsInfo 告知前端 LB 监控代理可用性与查询入口。
type LBStatsInfo struct {
	Enabled bool `json:"enabled"`
	// QueryURL 是受限 PromQL 查询的代理端点（POST {query}）。
	QueryURL string `json:"queryUrl"`
}

// GetClusterInfo serves GET /api/v1/ops/cluster：集群成员拓扑。
func (s *Service) GetClusterInfo(ctx context.Context) (*ClusterInfoResponse, error) {
	if s == nil || s.svcCtx == nil {
		return nil, errors.New("服务上下文不可用")
	}
	resp := &ClusterInfoResponse{Items: []ClusterInstance{}}

	// 单实例：registry 本地 agent 数即全部。
	if s.svcCtx.Cluster == nil {
		resp.Enabled = false
		count := int64(0)
		if s.svcCtx.RegistryStore != nil {
			s.svcCtx.RegistryStore.Mu().RLock()
			count = int64(len(s.svcCtx.RegistryStore.AgentsUnsafe()))
			s.svcCtx.RegistryStore.Mu().RUnlock()
		}
		resp.Items = append(resp.Items, ClusterInstance{
			InstanceID: "standalone", Self: true, Alive: true, AgentCount: count,
		})
		resp.Total, resp.AliveCount = 1, 1
		return resp, nil
	}

	resp.Enabled = true
	resp.Self = s.svcCtx.Cluster.InstanceID
	if s.svcCtx.Cluster.LBStats != nil && s.svcCtx.Cluster.LBStats.Enabled() {
		resp.LbStats = &LBStatsInfo{Enabled: true, QueryURL: "/api/v1/ops/cluster/lb-stats"}
	}

	// 在线成员（成员表租约判定）。
	if m := s.svcCtx.Cluster.Membership; m != nil {
		if peers, err := m.ListAlive(ctx); err == nil {
			for _, p := range peers {
				resp.Items = append(resp.Items, ClusterInstance{
					InstanceID:    p.InstanceID,
					AdvertiseAddr: p.AdvertiseAddr,
					Epoch:         p.Epoch,
					StartedAt:     p.StartedAt.UTC().Format("2006-01-02T15:04:05Z"),
					Alive:         true,
					Self:          p.InstanceID == resp.Self,
				})
			}
		}
	}
	// last-known 中已失联的对端（租约过期）：展示为离线。
	if s.svcCtx.Cluster.Mesh != nil {
		aliveMap := map[string]bool{}
		for _, it := range resp.Items {
			aliveMap[it.InstanceID] = true
		}
		for _, p := range s.svcCtx.Cluster.Mesh.Peers() {
			if aliveMap[p.InstanceID] {
				continue
			}
			resp.Items = append(resp.Items, ClusterInstance{
				InstanceID:    p.InstanceID,
				AdvertiseAddr: p.AdvertiseAddr,
				Epoch:         p.Epoch,
				StartedAt:     p.StartedAt.UTC().Format("2006-01-02T15:04:05Z"),
				Alive:         false,
				Self:          false,
			})
		}
	}

	// Agent 归属统计（共享表）。
	if r := s.svcCtx.Cluster.OwnerStats; r != nil {
		counts := r(ctx)
		for i := range resp.Items {
			resp.Items[i].AgentCount = counts[resp.Items[i].InstanceID]
		}
	}
	for _, it := range resp.Items {
		if it.Alive {
			resp.AliveCount++
		}
		resp.Total++
	}
	return resp, nil
}
