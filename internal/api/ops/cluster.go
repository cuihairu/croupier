package ops

import (
	"context"
	"errors"
)

// ---------- Cluster instance views (ops menu topology) ----------

type ClusterInstance struct {
	InstanceID    string `json:"instanceId"`
	AdvertiseAddr string `json:"advertiseAddr"`
	Epoch         uint64 `json:"epoch"`
	StartedAt     string `json:"startedAt,omitempty"`
	// Self marks the instance handling the current request.
	Self bool `json:"self"`
	// Alive derives from the member table lease check (not expired = online).
	Alive bool `json:"alive"`
	// AgentCount is the number of Agent connections this instance currently
	// holds (shared ownership table statistics).
	AgentCount int64 `json:"agentCount"`
}

type ClusterInfoResponse struct {
	Enabled    bool              `json:"enabled"`
	Self       string            `json:"self,omitempty"`
	Items      []ClusterInstance `json:"items"`
	Total      int               `json:"total"`
	AliveCount int               `json:"aliveCount"`
	// LbStats LB monitoring availability hint (nil = Prometheus not
	// configured, frontend hides the reconciliation column).
	LbStats *LBStatsInfo `json:"lbStats,omitempty"`
}

// LBStatsInfo tells the frontend the LB monitoring proxy availability and
// query entry point.
type LBStatsInfo struct {
	Enabled bool `json:"enabled"`
	// QueryURL is the proxy endpoint for restricted PromQL queries (POST
	// {query}).
	QueryURL string `json:"queryUrl"`
}

// GetClusterInfo serves GET /api/v1/ops/cluster: cluster member topology.
func (s *Service) GetClusterInfo(ctx context.Context) (*ClusterInfoResponse, error) {
	if s == nil || s.svcCtx == nil {
		return nil, errors.New("服务上下文不可用")
	}
	resp := &ClusterInfoResponse{Items: []ClusterInstance{}}

	// Single instance: the registry local agent count is everything.
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

	// Online members (member table lease determination).
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
	// Peers from last-known that have lost contact (lease expired): shown as
	// offline.
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

	// Agent ownership statistics (shared table).
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
