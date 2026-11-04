package svc

// 0061：agent_metrics_hourly 的 (agent_id, hour_start) 索引从非唯一改为唯一。
// 回归复刻存量形态：表已存在且带非唯一 idx_agent_hour + 重复行 → 去重 +
// 换唯一索引；幂等；缺表跳过（fanout 重放到无指标表的库不报错、不建空壳）。

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
)

func TestAgentMetricsHourlyUniqueIndexMigration_ExistingNonUniqueIndex(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m61.db"), &gorm.Config{})
	require.NoError(t, err)
	// 存量形态：迁移 0051 建表（非唯一 idx_agent_hour 索引 + 重复行）。
	require.NoError(t, db.AutoMigrate(&reg.AgentMetricsHourly{}))
	// AutoMigrate 建的是模型当前（唯一）索引；要复刻存量需先去掉唯一索引、
	// 改回非唯一，再插重复行。
	if db.Migrator().HasIndex(&reg.AgentMetricsHourly{}, "uidx_agent_hour") {
		require.NoError(t, db.Migrator().DropIndex(&reg.AgentMetricsHourly{}, "uidx_agent_hour"))
	}
	if db.Migrator().HasIndex(&reg.AgentMetricsHourly{}, "idx_agent_hour") {
		require.NoError(t, db.Migrator().DropIndex(&reg.AgentMetricsHourly{}, "idx_agent_hour"))
	}
	require.NoError(t, db.Exec(`CREATE INDEX idx_agent_hour ON agent_metrics_hourly (agent_id, hour_start)`).Error)

	hour := time.Now().UTC().Truncate(time.Hour)
	require.NoError(t, db.Create(&reg.AgentMetricsHourly{AgentID: "a1", HourStart: hour, SampleCount: 1}).Error)
	require.NoError(t, db.Create(&reg.AgentMetricsHourly{AgentID: "a1", HourStart: hour, SampleCount: 2}).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateAgentMetricsHourlyUniqueIndex(context.Background(), sqlDB))

	// 重复行已去重，唯一索引已建。
	var rows []reg.AgentMetricsHourly
	require.NoError(t, db.Where("agent_id = ? AND hour_start = ?", "a1", hour).Find(&rows).Error)
	require.Len(t, rows, 1, "重复 (agent, hour) 行必须去重为 1 条")
	require.True(t, db.Migrator().HasIndex(&reg.AgentMetricsHourly{}, "uidx_agent_hour"))
	require.False(t, db.Migrator().HasIndex(&reg.AgentMetricsHourly{}, "idx_agent_hour"), "旧非唯一索引必须删除")

	// 幂等：再跑一次不报错。
	require.NoError(t, migrateAgentMetricsHourlyUniqueIndex(context.Background(), sqlDB))
}

func TestAgentMetricsHourlyUniqueIndexMigration_MissingTableSkips(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m61skip.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateAgentMetricsHourlyUniqueIndex(context.Background(), sqlDB))
}
