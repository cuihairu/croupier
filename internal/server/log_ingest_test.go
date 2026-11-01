package server

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/registry"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

func TestControlService_HandleMetricEvent_IngestsLogs(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/logs.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&registry.AgentLogEntryRecord{}))

	cs := NewControlService(registry.NewStore(), nil)
	cs.SetMetricsDB(db)

	report := &opsv1.MetricsReport{
		AgentId: "agent-x",
		Logs: []*opsv1.LogEntry{
			{File: "/var/log/app.log", Level: "info", Message: "started", Seq: 1},
			{File: "/var/log/app.log", Level: "error", Message: "boom", Seq: 2},
		},
	}
	data, err := proto.Marshal(report)
	require.NoError(t, err)
	_, err = cs.handleMetricEvent(context.Background(), data)
	require.NoError(t, err)

	rows, err := cs.LogStore().Search(context.Background(), registry.LogSearchQuery{AgentID: "agent-x", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, rows, 2)
	assert.Equal(t, "boom", rows[0].Message)
}

func TestControlService_HandleMetricEvent_NoLogs(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/logs.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&registry.AgentLogEntryRecord{}))
	cs := NewControlService(registry.NewStore(), nil)
	cs.SetMetricsDB(db)

	report := &opsv1.MetricsReport{AgentId: "agent-y"}
	data, _ := proto.Marshal(report)
	_, err = cs.handleMetricEvent(context.Background(), data)
	require.NoError(t, err)

	rows, err := cs.LogStore().Search(context.Background(), registry.LogSearchQuery{AgentID: "agent-y", Limit: 10})
	require.NoError(t, err)
	assert.Empty(t, rows)
}
