package registry

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// TestMetricsStore_RollupHourly_AggregatesCompletedHour verifies the OPS-010
// 90-day hourly rollup: raw rows in the just-completed hour are averaged into
// one (agent, hour) row; older rows are excluded; the same hour re-rolled
// overwrites (idempotent).
func TestMetricsStore_RollupHourly_AggregatesCompletedHour(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&AgentMetricsHistory{}, &AgentMetricsHourly{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	hourStart := time.Now().Add(-time.Hour).Truncate(time.Hour)
	seedRaw(t, db, "a1", hourStart.Add(10*time.Minute), 30, 10, 3, 2, 100)
	seedRaw(t, db, "a1", hourStart.Add(20*time.Minute), 50, 20, 5, 4, 200)
	// Older hour row must NOT be rolled up.
	seedRaw(t, db, "a1", hourStart.Add(-2*time.Hour), 99, 99, 99, 99, 99)

	store := NewMetricsStore()
	store.SetDB(db)
	store.RollupHourly(context.Background())

	var rows []AgentMetricsHourly
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("query hourly: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("hourly rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.AgentID != "a1" {
		t.Fatalf("agent = %q, want a1", row.AgentID)
	}
	if row.SampleCount != 2 {
		t.Fatalf("sample count = %d, want 2", row.SampleCount)
	}
	if row.CPUUsageAvg != 40 {
		t.Fatalf("cpu avg = %v, want 40", row.CPUUsageAvg)
	}
	if row.MemUsageAvg != 15 {
		t.Fatalf("mem avg = %v, want 15", row.MemUsageAvg)
	}
	if row.ProcessCountAvg != 4 {
		t.Fatalf("process avg = %v, want 4", row.ProcessCountAvg)
	}
	if row.PortCountAvg != 3 {
		t.Fatalf("port avg = %v, want 3", row.PortCountAvg)
	}
	var nets []struct {
		Interface     string  `json:"interface"`
		BytesSentRate float64 `json:"bytes_sent_rate"`
	}
	if err := json.Unmarshal(row.NetRatesJSON, &nets); err != nil {
		t.Fatalf("unmarshal netrates: %v", err)
	}
	if len(nets) != 1 || nets[0].BytesSentRate != 150 {
		t.Fatalf("netrates = %+v, want [eth0 150]", nets)
	}

	// GetHourlyHistory rebuilds entries oldest-first.
	entries := store.GetHourlyHistory("a1", time.Time{}, 100)
	if len(entries) != 1 {
		t.Fatalf("hourly history = %d entries, want 1", len(entries))
	}
	e := entries[0]
	if e.Report.GetCpu().GetUsagePercent() != 40 {
		t.Fatalf("history cpu = %v, want 40", e.Report.GetCpu().GetUsagePercent())
	}
	if len(e.Report.GetNetworks()) != 1 || e.Report.GetNetworks()[0].GetBytesSentRate() != 150 {
		t.Fatalf("history netrates = %+v, want [eth0 150]", e.Report.GetNetworks())
	}

	// Re-rolling the same hour must not duplicate the row.
	store.RollupHourly(context.Background())
	var again []AgentMetricsHourly
	if err := db.Find(&again).Error; err != nil {
		t.Fatalf("re-query: %v", err)
	}
	if len(again) != 1 {
		t.Fatalf("after re-roll hourly rows = %d, want 1 (idempotent)", len(again))
	}
}

// TestMetricsStore_PruneHourly_RemovesBeyondRetention verifies the 90-day
// aggregate prune only touches rows older than the window.
func TestMetricsStore_PruneHourly_RemovesBeyondRetention(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&AgentMetricsHourly{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now()
	old := now.Add(-91 * 24 * time.Hour)
	fresh := now.Add(-1 * time.Hour)
	db.Create(&AgentMetricsHourly{AgentID: "a1", HourStart: old, SampleCount: 1})
	db.Create(&AgentMetricsHourly{AgentID: "a1", HourStart: fresh, SampleCount: 1})

	store := NewMetricsStore()
	store.SetDB(db)
	store.PruneHourly(MetricsHourlyRetention)

	var rows []AgentMetricsHourly
	db.Find(&rows)
	if len(rows) != 1 {
		t.Fatalf("after prune rows = %d, want 1", len(rows))
	}
	if rows[0].HourStart.Before(fresh.Add(-time.Minute)) {
		t.Fatalf("pruned wrong row: %v", rows[0].HourStart)
	}
}

func seedRaw(t *testing.T, db *gorm.DB, agentID string, ts time.Time, cpu, mem float64, procs, ports int, netRate float64) {
	t.Helper()
	cpuJSON, _ := json.Marshal(&opsv1.CpuMetrics{UsagePercent: cpu})
	memJSON, _ := json.Marshal(&opsv1.MemoryMetrics{UsagePercent: mem})
	disksJSON, _ := json.Marshal([]*opsv1.DiskMetrics{{MountPoint: "/data", UsagePercent: 50}})
	netsJSON, _ := json.Marshal([]*opsv1.NetworkMetrics{{Interface: "eth0", BytesSentRate: netRate, BytesRecvRate: netRate / 2}})
	procsJSON, _ := json.Marshal(make([]*opsv1.ProcessMetrics, procs))
	portsJSON, _ := json.Marshal(make([]*opsv1.PortMetrics, ports))
	if err := db.Create(&AgentMetricsHistory{
		AgentID:       agentID,
		Timestamp:     ts,
		CPUJSON:       cpuJSON,
		MemoryJSON:    memJSON,
		DisksJSON:     disksJSON,
		NetworksJSON:  netsJSON,
		ProcessesJSON: procsJSON,
		PortsJSON:     portsJSON,
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// TestAggregateHourly_DiskUsagePercentAverage verifies #667: the hourly rollup
// averages per-mount usage_percent over samples instead of summing them, so two
// samples at 50% and 100% produce 75% (not 150%).
func TestAggregateHourly_DiskUsagePercentAverage(t *testing.T) {
	mk := func(usage float64) []byte {
		b, _ := json.Marshal([]*opsv1.DiskMetrics{{MountPoint: "/data", UsagePercent: usage}})
		return b
	}
	rows := []AgentMetricsHistory{
		{AgentID: "a1", DisksJSON: mk(50)},
		{AgentID: "a1", DisksJSON: mk(100)},
	}
	agg := aggregateHourly(rows)
	if len(agg.DisksJSON) == 0 {
		t.Fatalf("disks json empty, want one mount point")
	}
	var disks []struct {
		MountPoint   string  `json:"mount_point"`
		UsagePercent float64 `json:"usage_percent"`
	}
	if err := json.Unmarshal(agg.DisksJSON, &disks); err != nil {
		t.Fatalf("unmarshal disks: %v", err)
	}
	if len(disks) != 1 {
		t.Fatalf("disks = %d, want 1", len(disks))
	}
	if disks[0].MountPoint != "/data" {
		t.Fatalf("mount = %q, want /data", disks[0].MountPoint)
	}
	if disks[0].UsagePercent != 75 {
		t.Fatalf("usage_percent = %v, want 75 (average of 50 and 100)", disks[0].UsagePercent)
	}
}

// TestMetricsStore_SetDBConcurrentNoRace verifies #668: SetDB must not data-
// race with concurrent readers (persistToDB / RollupHourly / GetHistory). Run
// under -race to catch unsynchronized s.db access.
func TestMetricsStore_SetDBConcurrentNoRace(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&AgentMetricsHistory{}, &AgentMetricsHourly{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store := NewMetricsStore()
	store.SetDB(db)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Reader path touching s.db.Load() and DB ops.
			store.Add("a1", &opsv1.MetricsReport{Cpu: &opsv1.CpuMetrics{UsagePercent: 50}})
			store.GetHistory("a1", time.Time{}, 10)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.RollupHourly(context.Background())
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.SetDB(db)
		}()
	}
	wg.Wait()

	// After SetDB(nil), readers must not panic or write to a closed DB.
	store.SetDB(nil)
}

// TestMetricsStore_RollupHourly_ConcurrentNoDuplicateRows verifies the
// find-or-create race guard: multiple concurrent RollupHourly calls over the
// same (agent, hour) must collapse to exactly one row (the unique index
// rejects duplicates; the loser retries onto the winner). Run under -race.
func TestMetricsStore_RollupHourly_ConcurrentNoDuplicateRows(t *testing.T) {
	// 并发协程各自持连接：sqlite `:memory:` 每连接独立库，会看不到表。
	// 用临时文件共享同一物理库（GLEBAREZ SQLITE 也走 file 语义）。
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/rollup-race.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&AgentMetricsHistory{}, &AgentMetricsHourly{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	hourStart := time.Now().Add(-time.Hour).Truncate(time.Hour)
	seedRaw(t, db, "a1", hourStart.Add(5*time.Minute), 30, 10, 3, 2, 100)

	store := NewMetricsStore()
	store.SetDB(db)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.RollupHourly(context.Background())
		}()
	}
	wg.Wait()

	var rows []AgentMetricsHourly
	if err := db.Where("agent_id = ? AND hour_start = ?", "a1", hourStart).Find(&rows).Error; err != nil {
		t.Fatalf("query hourly: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("concurrent rollup rows = %d, want exactly 1 (unique index + race retry)", len(rows))
	}
}
