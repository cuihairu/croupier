package batchop

import (
	"context"
	"fmt"

	"github.com/cuihairu/croupier/internal/platform/registry"
)

// RegistrySelector selects agents from the in-memory registry by scope and
// label/agent-id filters. Cluster-owned sessions are handled by the registry
// being the shared directory (the caller injects the resolver's store).
type RegistrySelector struct {
	store *registry.Store
}

// NewRegistrySelector builds a selector over a registry store.
func NewRegistrySelector(store *registry.Store) *RegistrySelector {
	return &RegistrySelector{store: store}
}

// Select returns agent IDs matching game/env AND (all label matches) AND (in
// the explicit agent list when provided). With both labels and agentIDs, the
// intersection is returned. Unknown labels simply match nothing.
func (s *RegistrySelector) Select(ctx context.Context, gameID, env string, labels map[string]string, agentIDs []string) ([]string, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("registry store unavailable")
	}
	if gameID == "" || env == "" {
		return nil, fmt.Errorf("gameId and env required")
	}
	explicit := make(map[string]bool, len(agentIDs))
	for _, id := range agentIDs {
		explicit[id] = true
	}
	s.store.Mu().RLock()
	defer s.store.Mu().RUnlock()
	var out []string
	for _, sess := range s.store.AgentsUnsafe() {
		if sess == nil || sess.AgentID == "" {
			continue
		}
		if sess.GameID != gameID || sess.Env != env {
			continue
		}
		if len(explicit) > 0 && !explicit[sess.AgentID] {
			continue
		}
		if !matchLabels(sess.Labels, labels) {
			continue
		}
		out = append(out, sess.AgentID)
	}
	return out, nil
}

func matchLabels(sess map[string]string, want map[string]string) bool {
	if len(want) == 0 {
		return true
	}
	for k, v := range want {
		if sess[k] != v {
			return false
		}
	}
	return true
}
