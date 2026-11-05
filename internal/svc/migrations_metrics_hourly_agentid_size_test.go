package svc

// 0063：已执行过 0061 的存量库上，agent_metrics_hourly 重新收敛——
// agent_id 回填 VARCHAR(64) + 去重 + 唯一索引。编辑 0061 就地不会在已升级
// 实例重跑，须新增版本迁移覆盖（幂等；缺表跳过；fanout 重放安全）。

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
)

func TestAgentMetricsHourlyAgentIDSizeMigration_AlreadyConvergedIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m63.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&reg.AgentMetricsHourly{}))
	// 复刻已执行过 0061 的存量形态：表存在且已带唯一索引。
	require.True(t, db.Migrator().HasIndex(&reg.AgentMetricsHourly{}, "uidx_agent_hour"))

	hour := time.Now().UTC().Truncate(time.Hour)
	require.NoError(t, db.Create(&reg.AgentMetricsHourly{AgentID: "a1", HourStart: hour, SampleCount: 1}).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	// 首次运行 0063：不报错，唯一索引保留，行不丢。
	require.NoError(t, migrateAgentMetricsHourlyAgentIDSize(context.Background(), sqlDB))
	var rows []reg.AgentMetricsHourly
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
	require.True(t, db.Migrator().HasIndex(&reg.AgentMetricsHourly{}, "uidx_agent_hour"))
	require.False(t, db.Migrator().HasIndex(&reg.AgentMetricsHourly{}, "idx_agent_hour"))

	// 幂等：再跑一次仍不报错。
	require.NoError(t, migrateAgentMetricsHourlyAgentIDSize(context.Background(), sqlDB))
}

func TestAgentMetricsHourlyAgentIDSizeMigration_CollapsesDuplicates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m63dup.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&reg.AgentMetricsHourly{}))
	// 去掉唯一索引复刻存量（0051 建表 + 旧 0061 前形态）以插入重复行。
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
	require.NoError(t, migrateAgentMetricsHourlyAgentIDSize(context.Background(), sqlDB))

	var rows []reg.AgentMetricsHourly
	require.NoError(t, db.Where("agent_id = ? AND hour_start = ?", "a1", hour).Find(&rows).Error)
	require.Len(t, rows, 1, "重复 (agent, hour) 行必须去重为 1 条")
	require.True(t, db.Migrator().HasIndex(&reg.AgentMetricsHourly{}, "uidx_agent_hour"))
	require.False(t, db.Migrator().HasIndex(&reg.AgentMetricsHourly{}, "idx_agent_hour"))
}

func TestAgentMetricsHourlyAgentIDSizeMigration_MissingTableSkips(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m63skip.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, migrateAgentMetricsHourlyAgentIDSize(context.Background(), sqlDB))
}

// 0063 审核发现：注册入口未限制 agent_id 长度时，严格库（PostgreSQL）上
// 既有超 64 字节的 agent_id 会让缩窄 ALTER 报错并阻止启动。迁移必须不
// 因存量超长行而失败，且超长行保留（注册入口限长后不再新增）。
func TestAgentMetricsHourlyAgentIDSizeMigration_OverlongExistingRowsSurvive(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m63long.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&reg.AgentMetricsHourly{}))

	hour := time.Now().UTC().Truncate(time.Hour)
	longID := strings.Repeat("x", 100)
	require.NoError(t, db.Create(&reg.AgentMetricsHourly{AgentID: longID, HourStart: hour, SampleCount: 1}).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	// 迁移不得因存量超长 agent_id 报错。
	require.NoError(t, migrateAgentMetricsHourlyAgentIDSize(context.Background(), sqlDB))

	var rows []reg.AgentMetricsHourly
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, longID, rows[0].AgentID, "超长 agent_id 行必须保留")
}
