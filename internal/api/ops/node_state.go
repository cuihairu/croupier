package ops

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/cluster"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
)

type nodeListItem struct {
	node     Node
	lastSeen time.Time
	rank     int
}

func listNodes(ctx context.Context, svcCtx *svc.ServiceContext, gameID, env, status string) []Node {
	if svcCtx == nil {
		return []Node{}
	}
	if ctx == nil {
		// bare-background-ok: a defensive nil fallback for the inbound ctx (the
		// main path always passes the request ctx from the caller), not a fresh
		// bare context (on the CR-FIX-22 exemption list).
		ctx = context.Background()
	}

	drainedNodes := make(map[string]bool)
	if svcCtx.OpsStateStore != nil {
		opsState := svcCtx.OpsStateStore.Snapshot()
		for nodeID := range opsState.Nodes.Drained {
			drainedNodes[nodeID] = true
		}
	}

	now := time.Now()
	roleFilter := normalizeAgentRoleFilter(normalizeNodeStatusFilter(status))
	statusFilter := ""
	if roleFilter == "" {
		statusFilter = normalizeNodeStatusFilter(status)
	}
	items := make([]nodeListItem, 0)
	registered := make(map[string]bool)

	// Cluster mode: the shared ownership table is the online universe (the
	// local registry only holds agents connected to this instance; reading
	// locally alone would make the online/offline verdict flip randomly when
	// the entry point load-balances across instances). Locally held agents get
	// full details (metrics/function counts/SDK info); non-local ones get the
	// owner annotation.
	aliveOwners := make(map[string]cluster.AgentOwnerRecord)
	if svcCtx.Cluster != nil && svcCtx.Cluster.ListAgentOwners != nil {
		if owners, err := svcCtx.Cluster.ListAgentOwners(ctx); err == nil {
			for _, rec := range owners {
				aliveOwners[rec.AgentID] = rec
			}
		}
	}

	// selfInstanceID distinguishes "owner is a peer": a snapshot in the local
	// registry may come from the startup LoadFromDB (the agent is actually
	// connected to a peer instance), after which only the peer side keeps
	// renewing heartbeats, so a frozen-expiry local ExpireAt would misjudge an
	// active agent as stale -- such nodes must be judged online from the
	// shared ownership table.
	selfInstanceID := ""
	if svcCtx.Cluster != nil {
		selfInstanceID = svcCtx.Cluster.InstanceID
	}

	if svcCtx.RegistryStore != nil {
		store := svcCtx.RegistryStore
		store.Mu().RLock()
		for _, sess := range store.AgentsUnsafe() {
			if sess == nil {
				continue
			}
			registered[sess.AgentID] = true
			if gameID != "" && sess.GameID != gameID {
				continue
			}
			if env != "" && sess.Env != env {
				continue
			}

			var nodeStatus, remoteOwner string
			if owner, ok := aliveOwners[sess.AgentID]; ok && selfInstanceID != "" &&
				owner.InstanceID != selfInstanceID {
				// Active agent held remotely: the local snapshot only supplies
				// details (function counts/labels), the online state follows the
				// ownership table, and the owning instance is annotated.
				nodeStatus = "online"
				if drainedNodes[sess.AgentID] {
					nodeStatus = "drained"
				}
				remoteOwner = owner.InstanceID
			} else {
				nodeStatus = resolveNodeStatus(sess, drainedNodes[sess.AgentID], now)
			}
			if !nodeStatusMatches(statusFilter, nodeStatus) {
				continue
			}
			if !agentRoleMatches(sess.Labels["role"], roleFilter) {
				continue
			}

			item := runtimeNodeListItem(sess, nodeStatus, svcCtx.MetricsStore)
			if remoteOwner != "" {
				labels := make(map[string]string, len(item.node.Labels)+1)
				for k, v := range item.node.Labels {
					labels[k] = v
				}
				labels["ownerInstance"] = remoteOwner
				item.node.Labels = labels
			}
			items = append(items, item)
		}
		store.Mu().RUnlock()

		// Nodes active in the ownership table but whose connection is not on
		// this instance (held by a peer): the online verdict must not flip
		// because of which instance the entry point routes to.
		for _, rec := range aliveOwners {
			if registered[rec.AgentID] {
				continue
			}
			if gameID != "" && rec.GameID != gameID {
				continue
			}
			if env != "" && rec.Env != env {
				continue
			}
			registered[rec.AgentID] = true
			if !nodeStatusMatches(statusFilter, "online") {
				continue
			}
			node := Node{
				Id:       rec.AgentID,
				GameId:   rec.GameID,
				Env:      rec.Env,
				Status:   "online",
				Labels:   appendOwnerLabels(rec),
				LastSeen: utils.FormatTimestamp(rec.LastSeenAt),
			}
			items = append(items, nodeListItem{
				node:     node,
				lastSeen: rec.LastSeenAt,
				rank:     nodeStatusRank("online"),
			})
		}
	}

	items = append(items, offlineDatabaseNodeItems(ctx, svcCtx, registered, gameID, env, statusFilter)...)

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].rank != items[j].rank {
			return items[i].rank < items[j].rank
		}
		if !items[i].lastSeen.Equal(items[j].lastSeen) {
			return items[i].lastSeen.After(items[j].lastSeen)
		}
		return items[i].node.Id < items[j].node.Id
	})

	nodes := make([]Node, 0, len(items))
	for _, item := range items {
		nodes = append(nodes, item.node)
	}
	return nodes
}

func appendOwnerLabels(rec cluster.AgentOwnerRecord) map[string]string {
	labels := map[string]string{"ownerInstance": rec.InstanceID}
	for key, value := range rec.LabelsMap {
		labels[key] = value
	}
	return labels
}

func normalizeAgentRoleFilter(statusFilter string) string {
	switch statusFilter {
	case "sidecar", "devops", "capture", "synthetics":
		return statusFilter
	default:
		return ""
	}
}

func agentRoleMatches(role, roleFilter string) bool {
	if roleFilter == "" {
		return true
	}
	return strings.TrimSpace(role) == roleFilter
}

func runtimeNodeListItem(sess *registry.AgentSession, nodeStatus string, metricsStore *registry.MetricsStore) nodeListItem {
	sdkLanguage, sdkVersion, sdkName := "", "", ""
	if len(sess.Providers) > 0 {
		sdkLanguage = sess.Providers[0].SDKLanguage
		sdkVersion = sess.Providers[0].SDKVersion
		sdkName = sess.Providers[0].SDKName
	}

	expiresInSec := int64(time.Until(sess.ExpireAt).Seconds())
	if expiresInSec < 0 {
		expiresInSec = 0
	}

	node := Node{
		Id:           sess.AgentID,
		Hostname:     sess.Labels["hostname"],
		Addr:         sess.Addr,
		GameId:       sess.GameID,
		Env:          sess.Env,
		Status:       nodeStatus,
		Labels:       sess.Labels,
		LastSeen:     utils.FormatTimestamp(sess.LastSeen),
		SDKLanguage:  sdkLanguage,
		SDKVersion:   sdkVersion,
		SDKName:      sdkName,
		Version:      sess.Version,
		Functions:    len(sess.Functions),
		ExpiresInSec: expiresInSec,
	}

	// Populate system metrics from MetricsStore
	if metricsStore != nil {
		if entry, ok := metricsStore.GetLatest(sess.AgentID); ok && entry != nil {
			report := entry.Report
			summary := summarizeSupervised(report.GetSupervisedProcesses())
			node.Supervisor = &summary
			if report.GetCpu() != nil {
				node.CPU = &CpuMetrics{
					UsagePercent: report.GetCpu().GetUsagePercent(),
					Cores:        report.GetCpu().GetCores(),
					PerCore:      report.GetCpu().GetPerCore(),
					Load1M:       report.GetCpu().GetLoad_1M(),
					Load5M:       report.GetCpu().GetLoad_5M(),
					Load15M:      report.GetCpu().GetLoad_15M(),
				}
			}
			if report.GetMemory() != nil {
				node.Memory = &MemoryMetrics{
					TotalBytes:     report.GetMemory().GetTotalBytes(),
					UsedBytes:      report.GetMemory().GetUsedBytes(),
					AvailableBytes: report.GetMemory().GetAvailableBytes(),
					UsagePercent:   report.GetMemory().GetUsagePercent(),
					SwapTotal:      report.GetMemory().GetSwapTotal(),
					SwapUsed:       report.GetMemory().GetSwapUsed(),
				}
			}
			if len(report.GetDisks()) > 0 {
				node.Disks = make([]DiskMetrics, len(report.GetDisks()))
				for i, disk := range report.GetDisks() {
					node.Disks[i] = DiskMetrics{
						MountPoint:     disk.GetMountPoint(),
						Device:         disk.GetDevice(),
						FsType:         disk.GetFsType(),
						TotalBytes:     disk.GetTotalBytes(),
						UsedBytes:      disk.GetUsedBytes(),
						AvailableBytes: disk.GetAvailableBytes(),
						UsagePercent:   disk.GetUsagePercent(),
						InodeTotal:     disk.GetInodeTotal(),
						InodeUsed:      disk.GetInodeUsed(),
					}
				}
			}
		}
	}

	return nodeListItem{
		node:     node,
		lastSeen: sess.LastSeen,
		rank:     nodeStatusRank(nodeStatus),
	}
}

func offlineDatabaseNodeItems(ctx context.Context, svcCtx *svc.ServiceContext, registered map[string]bool, gameID, env, statusFilter string) []nodeListItem {
	if svcCtx.NodeModel == nil {
		return []nodeListItem{}
	}

	dbNodes, err := svcCtx.NodeModel.List(ctx, model.ListNodesOptions{})
	if err != nil {
		return []nodeListItem{}
	}

	items := make([]nodeListItem, 0, len(dbNodes))
	for i := range dbNodes {
		dbNode := dbNodes[i]
		nodeID := strings.TrimSpace(dbNode.NodeID)
		if nodeID == "" || registered[nodeID] {
			continue
		}
		if dbNode.Type != "" && !strings.EqualFold(dbNode.Type, "agent") {
			continue
		}

		node := offlineDatabaseNode(dbNode)
		if gameID != "" && node.GameId != gameID {
			continue
		}
		if env != "" && node.Env != env {
			continue
		}
		if !nodeStatusMatches(statusFilter, node.Status) {
			continue
		}

		items = append(items, nodeListItem{
			node:     node,
			lastSeen: dbNode.UpdatedAt,
			rank:     nodeStatusRank(node.Status),
		})
	}
	return items
}

func offlineDatabaseNode(dbNode model.Node) Node {
	labels := map[string]string{}
	hostname := firstNonEmpty(databaseNodeString(dbNode, "hostname"), dbNode.Name, dbNode.NodeID)
	if hostname != "" {
		labels["hostname"] = hostname
	}
	if dbNode.Type != "" {
		labels["type"] = dbNode.Type
	}
	gameID := firstNonEmpty(databaseNodeString(dbNode, "gameId"), databaseNodeString(dbNode, "game_id"))
	env := databaseNodeString(dbNode, "env")

	return Node{
		Id:           dbNode.NodeID,
		Hostname:     hostname,
		Addr:         databaseNodeAddr(dbNode),
		GameId:       gameID,
		Env:          env,
		Status:       "offline",
		Labels:       labels,
		LastSeen:     firstNonEmpty(databaseNodeString(dbNode, "lastSeen"), databaseNodeString(dbNode, "last_seen")),
		Functions:    0,
		ExpiresInSec: 0,
	}
}

func databaseNodeString(dbNode model.Node, key string) string {
	if dbNode.Meta == nil {
		return ""
	}
	value, ok := dbNode.Meta[key]
	if !ok {
		return ""
	}
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func databaseNodeAddr(dbNode model.Node) string {
	ip := strings.TrimSpace(dbNode.IP)
	if ip == "" {
		return ""
	}
	if dbNode.Port <= 0 {
		return ip
	}
	return fmt.Sprintf("%s:%d", ip, dbNode.Port)
}

func resolveNodeStatus(sess *registry.AgentSession, drained bool, now time.Time) string {
	if drained {
		return "drained"
	}
	if sess == nil {
		return "stale"
	}
	if strings.TrimSpace(sess.Addr) == "" {
		return "stale"
	}
	if sess.LastSeen.IsZero() {
		return "stale"
	}
	if !sess.ExpireAt.IsZero() && !sess.ExpireAt.After(now) {
		return "stale"
	}
	return "active"
}

func normalizeNodeStatusFilter(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "*":
		return ""
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}

func nodeStatusMatches(statusFilter, nodeStatus string) bool {
	if statusFilter == "" {
		return true
	}
	if statusFilter == "offline" {
		return nodeStatus == "offline" || nodeStatus == "stale"
	}
	return nodeStatus == statusFilter
}

func nodeStatusRank(status string) int {
	switch status {
	case "active":
		return 0
	case "drained":
		return 1
	case "stale":
		return 2
	case "offline":
		return 3
	default:
		return 4
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
