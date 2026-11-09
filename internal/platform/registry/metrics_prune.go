package registry

import (
	"context"

	"time"
)

// Prune removes old entries from database.
func (s *MetricsStore) Prune(olderThan time.Duration) {
	if olderThan <= 0 {
		return
	}

	// Prune memory
	s.mu.Lock()
	cutoff := time.Now().Add(-olderThan)
	for agentID, indices := range s.byAgent {
		keep := make([]int, 0, len(indices))
		for _, idx := range indices {
			if s.entries[idx].Received.After(cutoff) {
				keep = append(keep, idx)
			} else {
				s.entries[idx] = MetricsEntry{}
			}
		}
		if len(keep) == 0 {
			delete(s.byAgent, agentID)
		} else {
			s.byAgent[agentID] = keep
		}
	}
	s.mu.Unlock()

	// Prune database
	if db := s.db.Load(); db != nil {
		db.Where("timestamp < ?", cutoff).Delete(&AgentMetricsHistory{})
	}
}

// StartCleanupRoutine starts a background routine to clean up old metrics
// and roll up the previous hour into the 90-day aggregate (OPS-010). If
// interval is 0, uses the configured CleanupInterval.
func (s *MetricsStore) StartCleanupRoutine(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = s.config.CleanupInterval
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if s.db.Load() != nil {
					s.RollupHourly(ctx)
					s.PruneHourly(MetricsHourlyRetention)
				}
				s.Prune(s.config.Retention)
			}
		}
	}()
}

// PruneHourly removes hourly aggregate rows older than the given duration
// (OPS-010 聚合 90 天）。
func (s *MetricsStore) PruneHourly(olderThan time.Duration) {
	db := s.db.Load()
	if olderThan <= 0 || db == nil {
		return
	}
	cutoff := time.Now().Add(-olderThan)
	db.Where("hour_start < ?", cutoff).Delete(&AgentMetricsHourly{})
}
