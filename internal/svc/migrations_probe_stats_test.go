package svc

// 0044：agent_metrics_history.probe_json 列（探测统计持久化，
// agent-health-recovery-review.md §13.1）。回归用例复刻存量形态：
// 手工 CREATE TABLE 只有旧列（无 probe_json）→ AddColumn 补齐；幂等；缺表跳过。

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
)

func TestAgentMetricsProbeJSONMigration_LegacyTableShape(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m44.db"), &gorm.Config{})
	require.NoError(t, err)
	// 存量老库形态（过 0001 baseline 的表）：只有 cpu/memory/disks 三列。
	require.NoError(t, db.Exec(`CREATE TABLE agent_metrics_history (
		id integer primary key autoincrement,
		created_at datetime,
		agent_id text NOT NULL,
		timestamp datetime NOT NULL,
		cpu_json json, memory_json json, disks_json json)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_metrics_history
		(created_at, agent_id, timestamp, cpu_json)
		VALUES (datetime(), 'legacy-agent', datetime(), '{"usagePercent":1.5}')`).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateAgentMetricsProbeJSON(context.Background(), sqlDB))
	require.True(t, db.Migrator().HasColumn(&reg.AgentMetricsHistory{}, "ProbeJSON"))

	// 存量行原样保留（probe_json 留空——历史窗口无探测统计快照，不回填）。
	var rows int64
	require.NoError(t, db.Raw(
		`SELECT count(*) FROM agent_metrics_history WHERE probe_json IS NULL`).Scan(&rows).Error)
	require.EqualValues(t, 1, rows, "legacy row keeps NULL probe_json")

	// 幂等。
	require.NoError(t, migrateAgentMetricsProbeJSON(context.Background(), sqlDB))
}

func TestAgentMetricsProbeJSONMigration_MissingTableSkips(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m44skip.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateAgentMetricsProbeJSON(context.Background(), sqlDB))
	require.False(t, db.Migrator().HasTable(&reg.AgentMetricsHistory{}))
}

// 模型侧配套（registry）：持久化与 DB 回读重建由 registry 包自测
// （metrics_probe_test.go），此处只防迁移与模型列集漂移。
func TestAgentMetricsProbeJSONMigration_RoundTripsThroughModelColumn(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m44round.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// 老库过迁移 → 模型（含 ProbeJSON）AutoMigrate 出的列集可直接读写。
	require.NoError(t, migrateAgentMetricsProbeJSON(context.Background(), sqlDB))
	// dev/CI 形态：迁移加列 + AutoMigrate 新库 → 列集一致（ProbeJSON 模型字段）。
	require.NoError(t, db.AutoMigrate(&reg.AgentMetricsHistory{}))
	entry := reg.AgentMetricsHistory{
		AgentID:   "agent-round",
		Timestamp: time.Now(),
		ProbeJSON: []byte(`[{"target":"t1","checked":2,"failed":1}]`),
	}
	require.NoError(t, db.Create(&entry).Error)
	var back reg.AgentMetricsHistory
	require.NoError(t, db.First(&back, entry.ID).Error)
	require.Contains(t, string(back.ProbeJSON), `"target":"t1"`)
}
