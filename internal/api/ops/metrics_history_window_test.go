package ops

// OPS-010 history API 面：window=hourly 走 90 天小时聚合表，默认（raw）
// 走原始 7 天表；原始面重建 networks/processes/ports 三 DTO。

import (
	"context"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

func newHistorySvcCtx(t *testing.T) (*svc.ServiceContext, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&reg.AgentMetricsHistory{}, &reg.AgentMetricsHourly{}))

	store := reg.NewMetricsStore()
	store.SetDB(db)
	registryStore := reg.NewStore()
	require.NoError(t, registryStore.UpsertAgent(&reg.AgentSession{
		AgentID: "a1", GameID: "demo", Env: "prod",
	}))
	return &svc.ServiceContext{MetricsStore: store, RegistryStore: registryStore}, db
}

func TestAgentMetricsHistory_RawFaceRebuildsEnrichment(t *testing.T) {
	svcCtx, db := newHistorySvcCtx(t)

	report := &opsv1.MetricsReport{
		Cpu:      &opsv1.CpuMetrics{UsagePercent: 20},
		Memory:   &opsv1.MemoryMetrics{UsagePercent: 30},
		Networks: []*opsv1.NetworkMetrics{{Interface: "eth0", BytesSentRate: 55.5}},
		Processes: []*opsv1.ProcessMetrics{{
			Pid: 7, Name: "nginx", MemoryBytes: 1024, ListeningPorts: []int32{80},
		}},
		Ports: []*opsv1.PortMetrics{{Port: 80, Protocol: "tcp", Pid: 7, ProcessName: "nginx"}},
	}
	svcCtx.MetricsStore.Add("a1", report)
	require.Eventually(t, func() bool {
		var count int64
		require.NoError(t, db.Model(&reg.AgentMetricsHistory{}).Count(&count).Error)
		return count >= 1
	}, 5*time.Second, 10*time.Millisecond)

	resp, err := agentMetricsHistory(context.Background(), svcCtx, &AgentMetricsHistoryRequest{
		AgentID: "a1",
		Since:   time.Now().Add(-time.Minute).Format(time.RFC3339),
		Limit:   10,
	})
	require.NoError(t, err)
	require.Len(t, resp.Entries, 1)
	e := resp.Entries[0]
	require.Len(t, e.Networks, 1)
	assert.Equal(t, "eth0", e.Networks[0].Interface)
	assert.InDelta(t, 55.5, e.Networks[0].BytesSentRate, 0.001)
	require.Len(t, e.Processes, 1)
	assert.Equal(t, int32(7), e.Processes[0].Pid)
	assert.Equal(t, []int32{80}, e.Processes[0].ListeningPorts)
	require.Len(t, e.Ports, 1)
	assert.Equal(t, int32(80), e.Ports[0].Port)
	assert.Equal(t, "tcp", e.Ports[0].Protocol)
}

func TestAgentMetricsHistory_HourlyWindow(t *testing.T) {
	svcCtx, db := newHistorySvcCtx(t)

	hourStart := time.Now().Add(-time.Hour).Truncate(time.Hour)
	hourly := reg.AgentMetricsHourly{
		AgentID:         "a1",
		HourStart:       hourStart,
		CPUUsageAvg:     42.5,
		MemUsageAvg:     61.25,
		ProcessCountAvg: 8,
		PortCountAvg:    5,
		SampleCount:     20,
	}
	require.NoError(t, db.Create(&hourly).Error)

	resp, err := agentMetricsHistory(context.Background(), svcCtx, &AgentMetricsHistoryRequest{
		AgentID: "a1",
		Window:  "hourly",
		Since:   time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		Limit:   10,
	})
	require.NoError(t, err)
	require.Len(t, resp.Entries, 1)
	e := resp.Entries[0]
	assert.InDelta(t, 42.5, e.CPU.UsagePercent, 0.001)
	assert.InDelta(t, 61.25, e.Memory.UsagePercent, 0.001)
}

func TestAgentMetricsHistory_HourlyWindowRespectsScope(t *testing.T) {
	svcCtx, _ := newHistorySvcCtx(t)
	// Unknown agent is out of scope: requireAgentScope must fail-closed for
	// the hourly window just as for the raw window.
	_, err := agentMetricsHistory(context.Background(), svcCtx, &AgentMetricsHistoryRequest{
		AgentID: "a2",
		Window:  "hourly",
	})
	require.Error(t, err)
}
