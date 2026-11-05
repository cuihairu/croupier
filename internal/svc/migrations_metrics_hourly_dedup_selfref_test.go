package svc

// MySQL/MSSQL 不允许 DELETE/UPDATE 的子查询直接引用目标表（MySQL Error 1093）。
// 回归：0061 去重必须经派生表绕开该限制，在方言语义下同样完成去重。

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
)

func TestAgentMetricsHourlyDedupSelfReferenceSafe(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/m61self.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&reg.AgentMetricsHourly{}))
	// 复刻存量形态（0051 建表尚无唯一索引）：去掉唯一索引，才能插入重复行。
	if db.Migrator().HasIndex(&reg.AgentMetricsHourly{}, "uidx_agent_hour") {
		require.NoError(t, db.Migrator().DropIndex(&reg.AgentMetricsHourly{}, "uidx_agent_hour"))
	}
	if db.Migrator().HasIndex(&reg.AgentMetricsHourly{}, "idx_agent_hour") {
		require.NoError(t, db.Migrator().DropIndex(&reg.AgentMetricsHourly{}, "idx_agent_hour"))
	}

	hour := time.Now().UTC().Truncate(time.Hour)
	for i := 0; i < 3; i++ {
		require.NoError(t, db.Create(&reg.AgentMetricsHourly{AgentID: "a1", HourStart: hour, SampleCount: i}).Error)
	}

	require.NoError(t, db.Exec(`
DELETE FROM agent_metrics_hourly
WHERE id NOT IN (
  SELECT keep_id FROM (
    SELECT MIN(id) AS keep_id FROM agent_metrics_hourly GROUP BY agent_id, hour_start
  ) AS keep_rows
)`).Error)

	var rows []reg.AgentMetricsHourly
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
}
