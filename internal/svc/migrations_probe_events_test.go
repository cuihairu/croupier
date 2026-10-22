package svc

// 0046：probe_events 建表（synthetics 探测事件持久化）。回归用例复刻线上
// 形态：过 0045 的库没有 probe_events 表（该模型此前只走 AutoMigrate，
// dev/CI 有表、线上 postgres 无表）→ 迁移建表且列集齐全；已存在的表
// （dev 老形态，payload 曾用 MySQL 专有的 `type:blob`）跳过不动，保留
// 存量行与列集——不做整模型 AutoMigrate。payload 的方言映射
// （pg bytea / mssql varbinary(max)）由 Migration Matrix 集成测试承担，
// sqlite 测不出方言差异。

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
)

func TestProbeEventsTableMigration_CreatesMissingTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m46.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.False(t, db.Migrator().HasTable(&model.ProbeEvent{}), "0046 之前的库无 probe_events")

	require.NoError(t, migrateProbeEventsTable(context.Background(), sqlDB))

	migrator := db.Migrator()
	require.True(t, migrator.HasTable(&model.ProbeEvent{}), "0046 必须建 probe_events")
	// 列集来自 model.ProbeEvent：诊断查询按 kind/status/error_class 过滤，
	// payload 是唯一承载原始事件的列，缺一即存储面与页面同时不可用。
	for _, col := range []string{"ID", "CreatedAt", "AgentID", "GameID", "Env", "Kind", "Target", "Status", "LatencyMS", "ErrorClass", "TargetCategory", "Payload"} {
		require.True(t, migrator.HasColumn(&model.ProbeEvent{}, col), "缺列 %s", col)
	}

	// 写入/回读走通（Payload 是 proto 序列化的原始字节，这里用任意字节验证映射）。
	event := model.ProbeEvent{
		AgentID: "synthetics-1", GameID: "demo", Env: "prod",
		Kind: "probe_result", Target: "https://example.com/health", Status: "up",
		Payload: []byte{0x0a, 0x02, 0x64, 0x65},
	}
	require.NoError(t, db.Create(&event).Error)
	require.NotZero(t, event.ID)
	var got model.ProbeEvent
	require.NoError(t, db.First(&got, event.ID).Error)
	require.Equal(t, event.Payload, got.Payload)
}

// 已有表跳过：dev 老形态是 AutoMigrate 建的（payload 当时是 blob 声明），
// 0046 不得对它做整模型对齐——存量行、多余列都必须原样保留。
func TestProbeEventsTableMigration_SkipsExistingTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m46exist.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE probe_events (
		id integer primary key autoincrement,
		created_at datetime,
		agent_id text, game_id text, env text, kind text, target text,
		status text, latency_ms integer default 0, error_class text,
		payload blob,
		extra_legacy_column text)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO probe_events
		(agent_id, game_id, env, kind, status, error_class, payload, extra_legacy_column)
		VALUES ('synthetics-1', 'demo', 'prod', 'probe_result', 'down', 'conn_refused', x'0a026465', 'keep')`).Error)

	require.NoError(t, migrateProbeEventsTable(context.Background(), sqlDB))
	// 幂等重跑同样不改动。
	require.NoError(t, migrateProbeEventsTable(context.Background(), sqlDB))

	migrator := db.Migrator()
	require.True(t, migrator.HasColumn(&model.ProbeEvent{}, "Payload"))

	// 多余列仍在（模型里没有该字段，走 pragma 断言列名而非 HasColumn）。
	var cols []string
	require.NoError(t, db.Raw("SELECT name FROM pragma_table_info('probe_events')").Scan(&cols).Error)
	require.Contains(t, cols, "payload")
	require.Contains(t, cols, "extra_legacy_column", "已有表不得被整模型对齐")

	var got model.ProbeEvent
	require.NoError(t, db.Where("agent_id = ?", "synthetics-1").First(&got).Error)
	require.Equal(t, []byte{0x0a, 0x02, 0x64, 0x65}, got.Payload)
	var extra []string
	require.NoError(t, db.Table("probe_events").Where("agent_id = ?", "synthetics-1").Pluck("extra_legacy_column", &extra).Error)
	require.Equal(t, []string{"keep"}, extra)
}

// 0047：已有表补类别列并保留存量数据；0046 新建表已含该列时幂等跳过。
func TestProbeEventTargetCategoryMigration_AddsColumn(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m47.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE probe_events (
		id integer primary key autoincrement,
		created_at datetime,
		agent_id text, game_id text, env text, kind text, target text,
		status text, latency_ms integer default 0, error_class text,
		payload blob)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO probe_events
		(agent_id, game_id, env, kind, status, error_class, payload)
		VALUES ('synthetics-1', 'demo', 'prod', 'probe_result', 'down', 'conn_refused', x'0a026465')`).Error)

	require.NoError(t, migrateProbeEventTargetCategory(context.Background(), sqlDB))
	require.NoError(t, migrateProbeEventTargetCategory(context.Background(), sqlDB))

	migrator := db.Migrator()
	require.True(t, migrator.HasColumn(&model.ProbeEvent{}, "TargetCategory"))

	var got model.ProbeEvent
	require.NoError(t, db.Where("agent_id = ?", "synthetics-1").First(&got).Error)
	require.Empty(t, got.TargetCategory, "historical rows must remain unclassified")

	upsert := model.ProbeEvent{AgentID: "synthetics-1", GameID: "demo", Env: "prod",
		Kind: "probe_result", Target: "payment", Status: "up", TargetCategory: "third_party"}
	require.NoError(t, db.Create(&upsert).Error)
}
