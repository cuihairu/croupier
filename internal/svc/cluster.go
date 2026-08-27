package svc

import (
	"context"

	"github.com/cuihairu/croupier/internal/cluster"
)

// ClusterRuntime 暴露多实例 HA 的运行句柄（诊断/ops 视图/转发接线用）。
type ClusterRuntime struct {
	InstanceID string
	Epoch      uint64
	Mesh       *cluster.MeshInterconnect
	// OwnerHooks 是 Agent 会话归属钩子（control/tcp listener 注入）。
	OwnerHooks *cluster.OwnerHooks
	// Membership 成员表（ops 集群视图读取在线实例）。
	Membership cluster.Membership
	// Resolver 共享归属解析器（心跳自愈回读 scope）。
	Resolver *cluster.DBOwnerResolver
	// OwnerStats 返回 instance -> 持有 agent 数（共享归属表聚合）。
	OwnerStats func(ctx context.Context) map[string]int64
	// ListAgentOwners 返回 TTL 内全部归属记录（集群模式 agent 列表
	// 的全集来源；本地 registry 只含本实例连接）。
	ListAgentOwners func(ctx context.Context) ([]cluster.AgentOwnerRecord, error)
}
