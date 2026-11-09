package function

import (
	"context"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/cluster"
	"github.com/cuihairu/croupier/internal/logic/utils"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
)

// sessionMatchesScope 判断会话是否落入 scope（空 scope 全通过）。
func sessionMatchesScope(scope svc.GameScope, gameID, env string) bool {
	if scope.GameID == "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(gameID), scope.GameID) &&
		strings.EqualFold(strings.TrimSpace(env), scope.Env)
}

// remoteAgentSnapshot 是归属表兜底出的远端 agent 会话快照及其 owner 实例。
type remoteAgentSnapshot struct {
	sess          *reg.AgentSession
	ownerInstance string
}

// clusterOwnerRecords 读归属表全集。集群未启用（Cluster/ListAgentOwners
// 未装配）或读失败返回 nil，instances 视图回落本地语义。
func clusterOwnerRecords(ctx context.Context, svcCtx *svc.ServiceContext) []cluster.AgentOwnerRecord {
	if svcCtx.Cluster == nil || svcCtx.Cluster.ListAgentOwners == nil {
		return nil
	}
	recs, err := svcCtx.Cluster.ListAgentOwners(ctx)
	if err != nil {
		return nil
	}
	return recs
}

// clusterSelfInstanceID 返回本实例的集群身份（未装配返回空）。
func clusterSelfInstanceID(svcCtx *svc.ServiceContext) string {
	if svcCtx.Cluster == nil {
		return ""
	}
	return svcCtx.Cluster.InstanceID
}

// remoteOwnerMap 从归属表构建 agentID → 对端实例标注：本地 registry 里
// 由对端持有的 DB 快照（refreshRemoteSnapshots 回灌的）标真归属，本实例
// 自持的排除（前端渲染「本实例」）。selfInstanceID 为空（未装配集群身份）
// 时无法区分自持，整体不标注。
func remoteOwnerMap(owners []cluster.AgentOwnerRecord, selfInstanceID string) map[string]string {
	if len(owners) == 0 || selfInstanceID == "" {
		return nil
	}
	out := make(map[string]string, len(owners))
	for _, rec := range owners {
		if rec.InstanceID == selfInstanceID {
			continue
		}
		out[rec.AgentID] = rec.InstanceID
	}
	return out
}

// remoteAgentSnapshots 返回「归属表活跃但本地 registry 未持有」的 agent
// 会话快照——集群模式下 instances 视图的跨实例兜底来源：本地 registry 只
// 含连到本实例的 agent，对端实例持有的 agent 以共享归属表
// （cluster_agent_owners，TTL 内即活跃）为在线全集，明细从共享
// agent_sessions 快照表读取（Providers/Functions 已 JSON 落库）。归属表
// 不可达或快照行缺失（owner 已 Release 的竞态）时静默回落本地视图，
// 读路径不升级成 5xx。非集群部署（Cluster/DB 未装配）恒返回空。
// owners 由调用方一次读出共用（本地条目标注也要用，避免双查归属表）。
func remoteAgentSnapshots(ctx context.Context, svcCtx *svc.ServiceContext, scope svc.GameScope, localAgentIDs map[string]struct{}, owners []cluster.AgentOwnerRecord) []remoteAgentSnapshot {
	if svcCtx.Cluster == nil || svcCtx.Cluster.ListAgentOwners == nil || svcCtx.DB == nil {
		return nil
	}
	missing := make([]string, 0, len(owners))
	for _, rec := range owners {
		if _, ok := localAgentIDs[rec.AgentID]; ok {
			continue
		}
		if !sessionMatchesScope(scope, rec.GameID, rec.Env) {
			continue
		}
		missing = append(missing, rec.AgentID)
	}
	if len(missing) == 0 {
		return nil
	}
	sessions, err := reg.NewAgentSessionModel(svcCtx.DB).LoadActiveSessionsByAgentIDs(ctx, missing)
	if err != nil {
		return nil
	}
	ownerByAgent := make(map[string]string, len(owners))
	for _, rec := range owners {
		ownerByAgent[rec.AgentID] = rec.InstanceID
	}
	out := make([]remoteAgentSnapshot, 0, len(sessions))
	// 设计债清理：sessions 来自 LoadActiveSessionsByAgentIDs，其元素由
	// toDomainSession 产出（err!=nil 时被跳过不 append），恒非 nil，
	// 原 nil 防御分支不可达已删除。
	for _, sess := range sessions {
		out = append(out, remoteAgentSnapshot{sess: sess, ownerInstance: ownerByAgent[sess.AgentID]})
	}
	return out
}

// instanceItemForFunction 判断会话是否注册了目标函数并产出条目。
func instanceItemForFunction(sess *reg.AgentSession, scope svc.GameScope, functionID, ownerInstance string) (FunctionInstance, bool) {
	if sess == nil || !sessionMatchesScope(scope, sess.GameID, sess.Env) {
		return FunctionInstance{}, false
	}
	if _, ok := sess.Functions[functionID]; ok {
		return FunctionInstance{
			AgentId:       sess.AgentID,
			AgentName:     sess.AgentID, // Use AgentID as name since AgentName doesn't exist
			Status:        "active",
			UpdatedAt:     utils.FormatTimestamp(sess.LastSeen),
			OwnerInstance: ownerInstance,
		}, true
	}
	return FunctionInstance{}, false
}

// appendInstanceSummaries 把单个 agent 会话的函数注册展开为实例条目：
// provider 会话优先（携带 ServiceID/SDK/addr 明细），未被任何 provider
// 声明的函数仍以 agent 级条目兜底。scope 不匹配的会话/provider 跳过。
func appendInstanceSummaries(instances []FunctionInstanceSummary, sess *reg.AgentSession, scope svc.GameScope, ownerInstance string) []FunctionInstanceSummary {
	if sess == nil || !sessionMatchesScope(scope, sess.GameID, sess.Env) {
		return instances
	}
	claimed := map[string]struct{}{}
	for i := range sess.Providers {
		prov := sess.Providers[i]
		seen := map[string]struct{}{}
		for _, fid := range prov.FunctionIDs {
			if _, ok := seen[fid]; ok {
				continue
			}
			seen[fid] = struct{}{}
			if scope.GameID != "" && !strings.EqualFold(strings.TrimSpace(prov.GameID), scope.GameID) {
				continue
			}
			instances = append(instances, FunctionInstanceSummary{
				FunctionID:    fid,
				AgentID:       sess.AgentID,
				AgentName:     sess.AgentID,
				ServiceID:     prov.ProviderID,
				Addr:          prov.Addr,
				Version:       prov.Version,
				SDKName:       prov.SDKName,
				SDKLang:       prov.SDKLanguage,
				SDKVersion:    prov.SDKVersion,
				AgentVersion:  sess.Version,
				GameID:        sess.GameID,
				Env:           sess.Env,
				Status:        "active",
				UpdatedAt:     utils.FormatTimestamp(sess.LastSeen),
				OwnerInstance: ownerInstance,
			})
			claimed[fid] = struct{}{}
		}
	}
	for fid := range sess.Functions {
		if _, ok := claimed[fid]; ok {
			continue
		}
		instances = append(instances, FunctionInstanceSummary{
			FunctionID:    fid,
			AgentID:       sess.AgentID,
			AgentName:     sess.AgentID,
			AgentVersion:  sess.Version,
			GameID:        sess.GameID,
			Env:           sess.Env,
			Status:        "active",
			UpdatedAt:     utils.FormatTimestamp(sess.LastSeen),
			OwnerInstance: ownerInstance,
		})
	}
	return instances
}

func functionInstances(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionInstancesRequest) (*FunctionInstancesResponse, error) {
	scope := currentFunctionScope(ctx)
	store := svcCtx.RegistryStore
	if store == nil {
		return &FunctionInstancesResponse{Items: []FunctionInstance{}}, nil
	}

	// 本地条目在读锁内构建（会话对象与 registry 并发写共享，不能锁外
	// 访问）；归属表在锁前一次读出——本地快照中被对端持有的条目也按
	// 归属表标注（refreshRemoteSnapshots 回灌的 DB 快照不是本实例直连），
	// 集群兜底 IO（归属表/快照表）在锁外。
	owners := clusterOwnerRecords(ctx, svcCtx)
	ownerOf := remoteOwnerMap(owners, clusterSelfInstanceID(svcCtx))
	items := []FunctionInstance{}
	localAgentIDs := map[string]struct{}{}
	store.Mu().RLock()
	for _, sess := range store.AgentsUnsafe() {
		if sess == nil {
			continue
		}
		localAgentIDs[sess.AgentID] = struct{}{}
		if item, ok := instanceItemForFunction(sess, scope, req.ID, ownerOf[sess.AgentID]); ok {
			items = append(items, item)
		}
	}
	store.Mu().RUnlock()

	for _, snap := range remoteAgentSnapshots(ctx, svcCtx, scope, localAgentIDs, owners) {
		if item, ok := instanceItemForFunction(snap.sess, scope, req.ID, snap.ownerInstance); ok {
			items = append(items, item)
		}
	}

	return &FunctionInstancesResponse{
		Items: items,
	}, nil
}

func functionInstancesAll(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionInstancesAllRequest) (*FunctionInstancesAllResponse, error) {
	scope := currentFunctionScope(ctx)
	store := svcCtx.RegistryStore
	if store == nil {
		return &FunctionInstancesAllResponse{Instances: []FunctionInstanceSummary{}, Total: 0}, nil
	}

	// 同 functionInstances：本地条目持读锁构建（对端持有的 DB 快照按
	// 归属表标注），集群兜底 IO 在锁外。
	owners := clusterOwnerRecords(ctx, svcCtx)
	ownerOf := remoteOwnerMap(owners, clusterSelfInstanceID(svcCtx))
	instances := []FunctionInstanceSummary{}
	localAgentIDs := map[string]struct{}{}
	store.Mu().RLock()
	for _, sess := range store.AgentsUnsafe() {
		if sess == nil {
			continue
		}
		localAgentIDs[sess.AgentID] = struct{}{}
		instances = appendInstanceSummaries(instances, sess, scope, ownerOf[sess.AgentID])
	}
	store.Mu().RUnlock()

	for _, snap := range remoteAgentSnapshots(ctx, svcCtx, scope, localAgentIDs, owners) {
		instances = appendInstanceSummaries(instances, snap.sess, scope, snap.ownerInstance)
	}

	sort.Slice(instances, func(i, j int) bool {
		if instances[i].FunctionID != instances[j].FunctionID {
			return instances[i].FunctionID < instances[j].FunctionID
		}
		return instances[i].ServiceID < instances[j].ServiceID
	})

	return &FunctionInstancesAllResponse{
		Instances: instances,
		Total:     len(instances),
	}, nil
}

// Permission management implementations
