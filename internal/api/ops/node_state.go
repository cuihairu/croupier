package ops

import (
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
)

type nodeListItem struct {
	node     Node
	lastSeen time.Time
	rank     int
}

func listNodes(svcCtx *svc.ServiceContext, gameID, env, status string) []Node {
	if svcCtx == nil || svcCtx.RegistryStore == nil {
		return []Node{}
	}

	store := svcCtx.RegistryStore
	store.Mu().RLock()
	defer store.Mu().RUnlock()

	drainedNodes := make(map[string]bool)
	if svcCtx.OpsStateStore != nil {
		opsState := svcCtx.OpsStateStore.Snapshot()
		for nodeID := range opsState.Nodes.Drained {
			drainedNodes[nodeID] = true
		}
	}

	now := time.Now()
	statusFilter := normalizeNodeStatusFilter(status)
	items := make([]nodeListItem, 0, len(store.AgentsUnsafe()))
	for _, sess := range store.AgentsUnsafe() {
		if sess == nil {
			continue
		}
		if gameID != "" && sess.GameID != gameID {
			continue
		}
		if env != "" && sess.Env != env {
			continue
		}

		nodeStatus := resolveNodeStatus(sess, drainedNodes[sess.AgentID], now)
		if statusFilter != "" && nodeStatus != statusFilter {
			continue
		}

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

		items = append(items, nodeListItem{
			node: Node{
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
				Functions:    len(sess.Functions),
				ExpiresInSec: expiresInSec,
			},
			lastSeen: sess.LastSeen,
			rank:     nodeStatusRank(nodeStatus),
		})
	}

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
	case "offline":
		return "stale"
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}

func nodeStatusRank(status string) int {
	switch status {
	case "active":
		return 0
	case "drained":
		return 1
	case "stale":
		return 2
	default:
		return 3
	}
}
