package cluster

import (
	"context"
)

// OwnerStore 是共享归属表的存储抽象。生产实现两份：
//   - DBOwnerResolver：共享关系库（postgres/mysql/sqlite，默认）
//   - RedisOwnerResolver：Redis（cluster.store: redis，租约靠 key TTL）
//
// 单实例部署不构造 OwnerStore（Cluster 为 nil，hooks no-op）。
type OwnerStore interface {
	// ClaimOwner 声明 instanceID 持有该 Agent（注册/重连时覆盖写）。
	ClaimOwner(ctx context.Context, agentID, gameID, env, instanceID string, epoch uint64) error
	// Touch 续期（仅当记录仍归属本实例；被接管时静默不动）。
	Touch(ctx context.Context, agentID string) error
	// Release 释放归属（仅本实例持有的记录）。
	Release(ctx context.Context, agentID string) error

	// ResolveOwner 返回持有该 Agent 的存活实例；无存活 owner 返回 (nil, nil)。
	// 存活判定 = owner 记录未过期 AND 实例成员租约未过期（peers 交叉验证）。
	ResolveOwner(ctx context.Context, agentID string) (*PeerInfo, error)
	// FindOwner 回读归属记录（不做 TTL 过滤；心跳自愈回读 scope 用）。
	// Redis 实现中 key 过期即视为不存在（临期窗口差异可接受）。
	FindOwner(ctx context.Context, agentID string) (*AgentOwnerRecord, error)
	// ListAliveOwners 返回 TTL 内全部归属记录（跨实例 agent 列表全集）。
	ListAliveOwners(ctx context.Context) ([]AgentOwnerRecord, error)
	// CountAgentsByOwner 聚合每实例持有的活跃 agent 数（集群拓扑页）。
	CountAgentsByOwner(ctx context.Context) (map[string]int64, error)
	// SelfOwnerScope 返回 agent 的 scope，仅当本实例是当前 Claim 持有者。
	SelfOwnerScope(ctx context.Context, agentID string) (gameID, env string, ok bool)

	// SetMesh 注入 mesh（ResolveOwner 交叉验证实例租约用）。
	SetMesh(m *MeshInterconnect)
}

// 编译期断言：DB 实现满足抽象。
var _ OwnerStore = (*DBOwnerResolver)(nil)
