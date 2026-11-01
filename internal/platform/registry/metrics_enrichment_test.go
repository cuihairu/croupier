package registry

// OPS-010 三面持久化（迁移 0051 的模型侧配套）：persistToDB 落
// Networks/Processes/Ports JSON 列、getFromDB 重建 report 三面；
// 无该面上报不落列（NULL 而非 "[]"）。

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMetricsStore_PersistsAndRebuildsEnrichmentFaces(t *testing.T) {
	db := openMetricsTestDB(t)
	store := NewMetricsStore()
	store.SetDB(db)

	report := &opsv1.MetricsReport{
		AgentId:   "agent-ops010",
		Timestamp: timestamppb.Now(),
		Cpu:       &opsv1.CpuMetrics{UsagePercent: 42},
		Networks: []*opsv1.NetworkMetrics{{
			Interface:     "eth0",
			BytesSent:     1000,
			BytesRecv:     2000,
			BytesSentRate: 12.5,
			BytesRecvRate: 25.0,
		}},
		Processes: []*opsv1.ProcessMetrics{{
			Pid:            123,
			Name:           "gameserver",
			MemoryBytes:    4096,
			ListeningPorts: []int32{8080, 8081},
		}},
		Ports: []*opsv1.PortMetrics{{
			Port:        8080,
			Protocol:    "tcp",
			Pid:         123,
			ProcessName: "gameserver",
			State:       "LISTEN",
		}},
	}
	store.Add("agent-ops010", report)

	require.Eventually(t, func() bool {
		var count int64
		require.NoError(t, db.Model(&AgentMetricsHistory{}).Count(&count).Error)
		return count >= 1
	}, 5*time.Second, 10*time.Millisecond)

	// 清内存后走 DB 回读：三面完整重建（ops 历史查询同口径）。
	store.Clear("agent-ops010")
	history := store.GetHistory("agent-ops010", time.Now().Add(-time.Minute), 10)
	require.Len(t, history, 1)
	rebuilt := history[0].Report

	require.Len(t, rebuilt.GetNetworks(), 1)
	assert.Equal(t, "eth0", rebuilt.GetNetworks()[0].GetInterface())
	assert.InDelta(t, 12.5, rebuilt.GetNetworks()[0].GetBytesSentRate(), 0.001)
	assert.InDelta(t, 25.0, rebuilt.GetNetworks()[0].GetBytesRecvRate(), 0.001)

	require.Len(t, rebuilt.GetProcesses(), 1)
	assert.Equal(t, int32(123), rebuilt.GetProcesses()[0].GetPid())
	assert.Equal(t, "gameserver", rebuilt.GetProcesses()[0].GetName())
	assert.Equal(t, []int32{8080, 8081}, rebuilt.GetProcesses()[0].GetListeningPorts())

	require.Len(t, rebuilt.GetPorts(), 1)
	assert.Equal(t, int32(8080), rebuilt.GetPorts()[0].GetPort())
	assert.Equal(t, "tcp", rebuilt.GetPorts()[0].GetProtocol())
	assert.Equal(t, "gameserver", rebuilt.GetPorts()[0].GetProcessName())
}

func TestMetricsStore_EmptyEnrichmentFacesLeaveColumnsNull(t *testing.T) {
	db := openMetricsTestDB(t)
	store := NewMetricsStore()
	store.SetDB(db)

	// 无三面的上报（老版本 agent / 关闭采集）：列 NULL（不落 "[]"）。
	store.Add("agent-ops010-empty", &opsv1.MetricsReport{
		Cpu: &opsv1.CpuMetrics{UsagePercent: 1},
	})

	require.Eventually(t, func() bool {
		var count int64
		require.NoError(t, db.Model(&AgentMetricsHistory{}).Count(&count).Error)
		return count >= 1
	}, 5*time.Second, 10*time.Millisecond)

	var nulls int64
	require.NoError(t, db.Raw(
		`SELECT count(*) FROM agent_metrics_history WHERE networks_json IS NULL
		 AND processes_json IS NULL AND ports_json IS NULL`).Scan(&nulls).Error)
	require.EqualValues(t, 1, nulls, "empty enrichment faces must persist as NULL, not []")

	store.Clear("agent-ops010-empty")
	history := store.GetHistory("agent-ops010-empty", time.Now().Add(-time.Minute), 10)
	require.Len(t, history, 1)
	assert.Empty(t, history[0].Report.GetNetworks())
	assert.Empty(t, history[0].Report.GetProcesses())
	assert.Empty(t, history[0].Report.GetPorts())
}
