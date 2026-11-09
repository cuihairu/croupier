package registry

import (
	"sync"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/proto"
)

// SystemInfoCache stores system info for agents.
type SystemInfoCache struct {
	mu    sync.RWMutex
	infos map[string]*SystemInfoEntry
}

type SystemInfoEntry struct {
	Info      *opsv1.SystemInfo
	UpdatedAt time.Time
}

// NewSystemInfoCache creates a new system info cache.
func NewSystemInfoCache() *SystemInfoCache {
	return &SystemInfoCache{
		infos: make(map[string]*SystemInfoEntry),
	}
}

// Set stores system info for an agent.
func (c *SystemInfoCache) Set(agentID string, info *opsv1.SystemInfo) {
	if agentID == "" || info == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.infos[agentID] = &SystemInfoEntry{
		Info:      info,
		UpdatedAt: time.Now(),
	}
}

// Get retrieves system info for an agent.
func (c *SystemInfoCache) Get(agentID string) (*opsv1.SystemInfo, bool) {
	if agentID == "" {
		return nil, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.infos[agentID]
	if !ok {
		return nil, false
	}

	// Return a copy to avoid mutation. proto.Clone 逐字段拷贝——手写列清单会随
	// proto 加字段静默漂移（agent_binary_sha256 落地时即踩中），Clone 根治。
	if cloned, ok := proto.Clone(entry.Info).(*opsv1.SystemInfo); ok && cloned != nil {
		return cloned, true
	}
	return entry.Info, true
}

// List returns all cached system info.
func (c *SystemInfoCache) List() map[string]*opsv1.SystemInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*opsv1.SystemInfo, len(c.infos))
	for agentID, entry := range c.infos {
		result[agentID] = entry.Info
	}
	return result
}

// Remove removes system info for an agent.
func (c *SystemInfoCache) Remove(agentID string) {
	if agentID == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.infos, agentID)
}

// Prune removes entries older than the given duration.
func (c *SystemInfoCache) Prune(olderThan time.Duration) {
	if olderThan <= 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	for agentID, entry := range c.infos {
		if entry.UpdatedAt.Before(cutoff) {
			delete(c.infos, agentID)
		}
	}
}
