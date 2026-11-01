package svc

// 0051：agent_metrics_history.networks/processes/ports 列 + agent_metrics_hourly
// 建表（OPS-010 节点资源指标持久化）。回归用例复刻存量形态：手工 CREATE
// TABLE 只有旧列（无三面 JSON）→ AddColumn 补齐 + 建聚合表；幂等；缺表跳过
// （fanout 重放到无指标表的库不报错、不建空壳）。

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
)

func TestAgentMetricsEnrichmentMigration_LegacyTableShape(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m51.db"), &gorm.Config{})
	require.NoError(t, err)
	// 存量老库形态（过 0044 前的表）：cpu/memory/disks/probe_json 列，
	// 无 networks/processes/ports 三面。
	require.NoError(t, db.Exec(`CREATE TABLE agent_metrics_history (
		id integer primary key autoincrement,
		created_at datetime,
		agent_id text NOT NULL,
		timestamp datetime NOT NULL,
		cpu_json json, memory_json json, disks_json json, probe_json json)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_metrics_history
		(created_at, agent_id, timestamp, cpu_json)
		VALUES (datetime(), 'legacy-agent', datetime(), '{"usagePercent":1.5}')`).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateAgentMetricsHistoryEnrichment(context.Background(), sqlDB))

	require.True(t, db.Migrator().HasColumn(&reg.AgentMetricsHistory{}, "NetworksJSON"))
	require.True(t, db.Migrator().HasColumn(&reg.AgentMetricsHistory{}, "ProcessesJSON"))
	require.True(t, db.Migrator().HasColumn(&reg.AgentMetricsHistory{}, "PortsJSON"))
	require.True(t, db.Migrator().HasTable(&reg.AgentMetricsHourly{}))

	// 存量行原样保留（三面留空——历史窗口无该数据，不回填）。
	var rows int64
	require.NoError(t, db.Raw(
		`SELECT count(*) FROM agent_metrics_history WHERE networks_json IS NULL
		 AND processes_json IS NULL AND ports_json IS NULL`).Scan(&rows).Error)
	require.EqualValues(t, 1, rows, "legacy row keeps NULL on all three faces")

	// 幂等。
	require.NoError(t, migrateAgentMetricsHistoryEnrichment(context.Background(), sqlDB))
}

func TestAgentMetricsEnrichmentMigration_MissingTableSkips(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m51skip.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateAgentMetricsHistoryEnrichment(context.Background(), sqlDB))
	require.False(t, db.Migrator().HasTable(&reg.AgentMetricsHistory{}))
	require.False(t, db.Migrator().HasTable(&reg.AgentMetricsHourly{}), "no empty shell table")
}

// 模型侧配套（registry）：持久化与 DB 回读重建由 registry 包自测
// （metrics_hourly_test.go + getFromDB 重建），此处只防迁移与模型列集漂移。
func TestAgentMetricsEnrichmentMigration_RoundTripsThroughModelColumns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m51round.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// 老库过迁移 → 模型（含三面）AutoMigrate 出的列集可直接读写。
	require.NoError(t, migrateAgentMetricsHistoryEnrichment(context.Background(), sqlDB))
	require.NoError(t, db.AutoMigrate(&reg.AgentMetricsHistory{}, &reg.AgentMetricsHourly{}))
	entry := reg.AgentMetricsHistory{
		AgentID:       "agent-round",
		NetworksJSON:  []byte(`[{"interface":"eth0","bytesSentRate":12.5}]`),
		ProcessesJSON: []byte(`[{"pid":1,"name":"init"}]`),
		PortsJSON:     []byte(`[{"port":8080,"protocol":"tcp","pid":1}]`),
	}
	require.NoError(t, db.Create(&entry).Error)
	var back reg.AgentMetricsHistory
	require.NoError(t, db.First(&back, entry.ID).Error)
	require.Contains(t, string(back.NetworksJSON), `"bytesSentRate":12.5`)
	require.Contains(t, string(back.ProcessesJSON), `"name":"init"`)
	require.Contains(t, string(back.PortsJSON), `"port":8080`)
}
