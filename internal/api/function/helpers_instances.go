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

// sessionMatchesScope reports whether the session falls within scope (an
// empty scope lets everything through).
func sessionMatchesScope(scope svc.GameScope, gameID, env string) bool {
	if scope.GameID == "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(gameID), scope.GameID) &&
		strings.EqualFold(strings.TrimSpace(env), scope.Env)
}

// remoteAgentSnapshot is a remote agent session snapshot derived as a fallback
// from the ownership table, together with its owner instance.
type remoteAgentSnapshot struct {
	sess          *reg.AgentSession
	ownerInstance string
}

// clusterOwnerRecords reads the full ownership table. It returns nil when the
// cluster is not enabled (Cluster/ListAgentOwners not wired) or the read
// fails, and the instances view then falls back to local semantics.
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

// clusterSelfInstanceID returns this instance's cluster identity (empty when
// not wired).
func clusterSelfInstanceID(svcCtx *svc.ServiceContext) string {
	if svcCtx.Cluster == nil {
		return ""
	}
	return svcCtx.Cluster.InstanceID
}

// remoteOwnerMap builds agentID -> peer-instance annotations from the
// ownership table: DB snapshots in the local registry that are owned by a peer
// (fed back by refreshRemoteSnapshots) get their true owner, while entries
// held by this instance are excluded (the frontend renders them as "this
// instance"). When selfInstanceID is empty (no cluster identity wired) local
// ownership cannot be distinguished, so nothing is annotated at all.
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

// remoteAgentSnapshots returns session snapshots of agents that are active in
// the ownership table but not held by the local registry -- the cross-instance
// fallback source of the instances view in cluster mode: the local registry
// only contains agents connected to this instance, while agents held by peer
// instances use the shared ownership table (cluster_agent_owners, active
// within its TTL) as the online universe, with details read from the shared
// agent_sessions snapshot table (Providers/Functions are already landed in the
// database as JSON). If the ownership table is unreachable or a snapshot row is
// missing (a race where the owner already Released), it silently falls back to
// the local view and the read path does not escalate to a 5xx. Non-cluster
// deployments (Cluster/DB not wired) always return empty.
// owners are read once by the caller and shared (local entry annotation needs
// them too, avoiding a double query of the ownership table).
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
	// Design-debt cleanup: sessions comes from LoadActiveSessionsByAgentIDs,
	// whose elements are produced by toDomainSession (skipped and not appended
	// when err != nil), so they are always non-nil; the original nil defensive
	// branch was unreachable and removed.
	for _, sess := range sessions {
		out = append(out, remoteAgentSnapshot{sess: sess, ownerInstance: ownerByAgent[sess.AgentID]})
	}
	return out
}

// instanceItemForFunction checks whether the session has registered the target
// function and, if so, produces the instance item.
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

// appendInstanceSummaries expands the function registrations of a single agent
// session into instance items: provider sessions take precedence (carrying
// ServiceID/SDK/addr details), and functions not declared by any provider still
// get an agent-level fallback entry. Sessions/providers whose scope does not
// match are skipped.
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

	// Local entries are built inside the read lock (session objects are shared
	// with concurrent writes to the registry and must not be accessed outside
	// the lock); the ownership table is read once before taking the lock --
	// entries in the local snapshot that are owned by a peer are annotated from
	// the ownership table too (DB snapshots fed back by refreshRemoteSnapshots
	// are not direct connections of this instance), and the cluster fallback IO
	// (ownership table/snapshot table) happens outside the lock.
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

	// Same as functionInstances: local entries are built while holding the
	// read lock (DB snapshots owned by a peer are annotated from the ownership
	// table), and the cluster fallback IO happens outside the lock.
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
