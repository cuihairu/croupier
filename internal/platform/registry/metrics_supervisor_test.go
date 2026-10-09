// Supervisor 事件环测试：上报捎带入库、重传去重、agent 重启 seq 归零的
// 重新入库、增量拉取与 limit、Clear 清理。
package registry

import (
	"testing"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func supReport(events ...*opsv1.SupervisorEvent) *opsv1.MetricsReport {
	return &opsv1.MetricsReport{AgentId: "a1", SupervisorEvents: events}
}

func TestMetricsStoreSupervisorEventsIngest(t *testing.T) {
	s := NewMetricsStore()
	s.Add("a1", supReport(
		&opsv1.SupervisorEvent{Seq: 1, TsUnix: 100, Process: "app", Event: "detect_down"},
		&opsv1.SupervisorEvent{Seq: 2, TsUnix: 110, Process: "app", Event: "auto_restart"},
	))

	evs := s.GetSupervisorEvents("a1", 0, 0)
	require.Len(t, evs, 2)
	assert.Equal(t, "detect_down", evs[0].Event)
	assert.Equal(t, int64(2), s.SupervisorLatestSeq("a1"))

	// 增量游标：sinceSeq 之后的子集。
	evs = s.GetSupervisorEvents("a1", 1, 0)
	require.Len(t, evs, 1)
	assert.Equal(t, "auto_restart", evs[0].Event)

	// limit 取最新。
	evs = s.GetSupervisorEvents("a1", 0, 1)
	require.Len(t, evs, 1)
	assert.Equal(t, int64(2), evs[0].Seq)

	assert.Empty(t, s.GetSupervisorEvents("ghost", 0, 0))
	assert.Empty(t, s.GetSupervisorEvents("", 0, 0))
	assert.Equal(t, int64(0), s.SupervisorLatestSeq("ghost"))
}

func TestMetricsStoreSupervisorEventsDedupRetransmit(t *testing.T) {
	s := NewMetricsStore()
	report := supReport(
		&opsv1.SupervisorEvent{Seq: 1, TsUnix: 100, Event: "detect_down"},
		&opsv1.SupervisorEvent{Seq: 2, TsUnix: 110, Event: "auto_restart"},
	)
	s.Add("a1", report)
	// 同一批事件随下一报重传：seq 与 ts 都不新 → 丢弃。
	s.Add("a1", report)
	assert.Len(t, s.GetSupervisorEvents("a1", 0, 0), 2)

	// 部分新事件：只追加新的。
	s.Add("a1", supReport(
		&opsv1.SupervisorEvent{Seq: 2, TsUnix: 110, Event: "auto_restart"},
		&opsv1.SupervisorEvent{Seq: 3, TsUnix: 120, Event: "manual_stop"},
	))
	evs := s.GetSupervisorEvents("a1", 0, 0)
	require.Len(t, evs, 3)
	assert.Equal(t, "manual_stop", evs[2].Event)
}

func TestMetricsStoreSupervisorEventsSeqResetAfterAgentRestart(t *testing.T) {
	s := NewMetricsStore()
	s.Add("a1", supReport(&opsv1.SupervisorEvent{Seq: 5, TsUnix: 100, Event: "detect_down"}))
	// agent 重启后游标归零：seq 回退但 ts 更新 → 按新事件入库。
	s.Add("a1", supReport(&opsv1.SupervisorEvent{Seq: 1, TsUnix: 900, Event: "manual_start"}))
	evs := s.GetSupervisorEvents("a1", 0, 0)
	require.Len(t, evs, 2)
	assert.Equal(t, "manual_start", evs[1].Event)
	assert.Equal(t, int64(1), s.SupervisorLatestSeq("a1"))

	// 旧 ts 的重传依旧被去重（seq=2 假设未出现；seq=1/ts 更旧 → 丢弃）。
	s.Add("a1", supReport(&opsv1.SupervisorEvent{Seq: 1, TsUnix: 100, Event: "manual_start"}))
	assert.Len(t, s.GetSupervisorEvents("a1", 0, 0), 2)
}

func TestMetricsStoreSupervisorEventsRingCapAndClear(t *testing.T) {
	s := NewMetricsStore()
	for i := int64(1); i <= supervisorEventRingSize+5; i++ {
		s.Add("a1", supReport(&opsv1.SupervisorEvent{Seq: i, TsUnix: 100 + i, Event: "auto_restart"}))
	}
	evs := s.GetSupervisorEvents("a1", 0, 0)
	assert.Len(t, evs, supervisorEventRingSize)
	assert.Equal(t, int64(6), evs[0].Seq)

	// Clear 连 supervisor 事件一起清。
	s.Clear("a1")
	assert.Empty(t, s.GetSupervisorEvents("a1", 0, 0))
	assert.Equal(t, int64(0), s.SupervisorLatestSeq("a1"))
}

func TestMetricsStoreSupervisorEventsNilSkipped(t *testing.T) {
	s := NewMetricsStore()
	s.Add("a1", supReport(nil, &opsv1.SupervisorEvent{Seq: 1, TsUnix: 1, Event: "detect_down"}, nil))
	assert.Len(t, s.GetSupervisorEvents("a1", 0, 0), 1)
}
