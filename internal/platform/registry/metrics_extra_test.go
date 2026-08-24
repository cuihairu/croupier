package registry

import (
	"context"
	"testing"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openMetricsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	// A shared-cache in-memory database keeps the AutoMigrated table visible
	// across every pooled connection used by the async persistence path.
	db, err := gorm.Open(gsqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func TestAgentMetricsHistory_TableName(t *testing.T) {
	assert.Equal(t, "agent_metrics_history", AgentMetricsHistory{}.TableName())
}

func TestNewMetricsStoreWithConfig_AppliesDefaultsForZeroValues(t *testing.T) {
	store := NewMetricsStoreWithConfig(MetricsStoreConfig{})

	assert.Equal(t, 10, store.config.MaxMemoryEntries)
	assert.Equal(t, 2000, store.config.MaxTotalEntries)
	assert.Equal(t, 7*24*time.Hour, store.config.Retention)
	assert.Equal(t, time.Hour, store.config.CleanupInterval)
	assert.NotNil(t, store.entries)
	assert.NotNil(t, store.byAgent)
}

func TestMetricsStore_SetDB_PersistsAndFallsBackToDB(t *testing.T) {
	db := openMetricsTestDB(t)
	store := NewMetricsStore()
	store.SetDB(db)

	report := &opsv1.MetricsReport{
		Cpu:    &opsv1.CpuMetrics{UsagePercent: 42.5},
		Memory: &opsv1.MemoryMetrics{UsedBytes: 100, TotalBytes: 200},
		Disks:  []*opsv1.DiskMetrics{{MountPoint: "/", UsedBytes: 1, TotalBytes: 2}},
	}
	store.Add("agent-db", report)

	require.Eventually(t, func() bool {
		var count int64
		require.NoError(t, db.Model(&AgentMetricsHistory{}).Count(&count).Error)
		return count == 1
	}, 2*time.Second, 10*time.Millisecond)

	// Memory still holds the entry, so history is served from memory.
	history := store.GetHistory("agent-db", time.Now().Add(-time.Minute), 10)
	require.Len(t, history, 1)

	// Clear memory so the query falls back to the database.
	store.Clear("agent-db")
	history = store.GetHistory("agent-db", time.Now().Add(-time.Minute), 10)
	require.Len(t, history, 1)
	require.NotNil(t, history[0].Report.GetCpu())
	assert.InDelta(t, 42.5, history[0].Report.GetCpu().GetUsagePercent(), 0.001)
	require.NotNil(t, history[0].Report.GetMemory())
	require.Len(t, history[0].Report.GetDisks(), 1)

	// A future since cutoff yields an empty DB result.
	history = store.GetHistory("agent-db", time.Now().Add(time.Hour), 10)
	assert.Empty(t, history)
}

func TestMetricsStore_SetDB_NilDisablesPersistence(t *testing.T) {
	store := NewMetricsStore()
	store.SetDB(nil)
	assert.Nil(t, store.db)

	store.Add("agent-nodb", &opsv1.MetricsReport{})
	assert.True(t, len(store.ListAgents()) == 1)
}

func TestMetricsStore_Add_EvictsWhenMemoryLimitReached(t *testing.T) {
	store := NewMetricsStoreWithConfig(MetricsStoreConfig{
		MaxMemoryEntries: 2,
		MaxTotalEntries:  16,
		Retention:        time.Hour,
		CleanupInterval:  time.Hour,
	})

	store.Add("agent-evict", &opsv1.MetricsReport{Cpu: &opsv1.CpuMetrics{UsagePercent: 1}})
	store.Add("agent-evict", &opsv1.MetricsReport{Cpu: &opsv1.CpuMetrics{UsagePercent: 2}})
	store.Add("agent-evict", &opsv1.MetricsReport{Cpu: &opsv1.CpuMetrics{UsagePercent: 3}})

	entries := store.GetAgentMetrics("agent-evict", 0)
	require.Len(t, entries, 2)
	assert.InDelta(t, 2.0, entries[0].Report.GetCpu().GetUsagePercent(), 0.001)
	assert.InDelta(t, 3.0, entries[1].Report.GetCpu().GetUsagePercent(), 0.001)
}

func TestMetricsStore_StartCleanupRoutine_PrunesExpiredEntries(t *testing.T) {
	store := NewMetricsStoreWithConfig(MetricsStoreConfig{
		MaxMemoryEntries: 10,
		MaxTotalEntries:  64,
		Retention:        time.Millisecond,
		CleanupInterval:  time.Hour,
	})
	store.Add("agent-cleanup", &opsv1.MetricsReport{})
	// Let the entry become older than the retention window.
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store.StartCleanupRoutine(ctx, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		_, ok := store.GetLatest("agent-cleanup")
		return !ok
	}, 2*time.Second, 10*time.Millisecond)
}

func TestMetricsStore_StartCleanupRoutine_DefaultInterval(t *testing.T) {
	store := NewMetricsStoreWithConfig(MetricsStoreConfig{
		MaxMemoryEntries: 10,
		MaxTotalEntries:  64,
		Retention:        time.Hour,
		CleanupInterval:  10 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	store.StartCleanupRoutine(ctx, 0)
	cancel()
}
