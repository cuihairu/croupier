package svc

// server 本机探针装配测试：空段不装配、重名报错、窗口/明细经上行桥到达
// 入站回调（视角=server）、手动诊断按名取探针与周期探测共用状态机。

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/core/healthprobe"
	"github.com/cuihairu/croupier/core/report"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

func serverProbeTargets() []healthprobe.TargetConfig {
	return []healthprobe.TargetConfig{
		{Name: "server-db", Type: healthprobe.TargetTypeTCP, Address: "127.0.0.1:1",
			FailThreshold: 1, Interval: 20 * time.Second},
		{Name: "platform-api", Type: healthprobe.TargetTypeTCP, Address: "127.0.0.1:1",
			FailThreshold: 1, Interval: 25 * time.Second},
	}
}

// eventRecorder 收入站回调的事件（并发安全）。
type eventRecorder struct {
	mu     sync.Mutex
	events []*opsv1.AlertEvent
}

func (r *eventRecorder) send(_ context.Context, ev *opsv1.AlertEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
}

func (r *eventRecorder) kinds() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.events))
	for _, ev := range r.events {
		out = append(out, ev.GetKind())
	}
	return out
}

// 空目标集不装配：nil 返回，ServiceContext 端据此报「探测面未配置」。
func TestServerProbeManager_EmptyTargetsNil(t *testing.T) {
	m, err := NewServerProbeManager(nil, nil, 0, nil)
	require.NoError(t, err)
	assert.Nil(t, m)
	_, ok := m.ProbeOnceByName("any")
	assert.False(t, ok, "nil manager must not panic")
}

// 重名目标整体报错（agent 同口径：重名让窗口事件串台）。
func TestServerProbeManager_DuplicateTargetRejected(t *testing.T) {
	m, err := NewServerProbeManager([]healthprobe.TargetConfig{
		{Name: "dup", Type: healthprobe.TargetTypeTCP, Address: "127.0.0.1:1"},
		{Name: "dup", Type: healthprobe.TargetTypeTCP, Address: "127.0.0.1:1"},
	}, nil, 0, nil)
	require.Error(t, err)
	assert.Nil(t, m)
}

// 一次失败探测（failThreshold=1）产生 probe.window（开窗）与 probe.detail
// （失败明细即时）；事件视角=server、目标与语义随包。
func TestServerProbeManager_WindowAndDetailEventsRouted(t *testing.T) {
	rec := &eventRecorder{}
	m, err := NewServerProbeManager(serverProbeTargets()[:1], rec.send, 0, nil)
	require.NoError(t, err)
	require.NotNil(t, m)

	res, ok := m.ProbeOnceByName("server-db")
	require.True(t, ok)
	assert.False(t, res.OK, "dead tcp target must fail")
	assert.True(t, res.WindowOpen, "failThreshold=1: first failure opens window")

	kinds := rec.kinds()
	assert.Contains(t, kinds, report.ProbeKindWindow)
	assert.Contains(t, kinds, report.ProbeKindDetail)

	rec.mu.Lock()
	var win *opsv1.AlertEvent
	for _, ev := range rec.events {
		if ev.GetKind() == report.ProbeKindWindow {
			win = ev
		}
	}
	rec.mu.Unlock()
	require.NotNil(t, win)
	assert.Equal(t, report.ServerProbeAgentID, win.GetAgentId(), "server viewpoint stamp")
	assert.Equal(t, "server-db", win.GetWindow().GetTarget())
	assert.Equal(t, "unavailable", win.GetWindow().GetWindowKind())
	assert.Equal(t, "readiness", win.GetWindow().GetSemantics(), "default semantics")
}

// 未知目标 ok=false；目标清单顺序同配置声明。
func TestServerProbeManager_ProbeOnceByNameUnknown(t *testing.T) {
	rec := &eventRecorder{}
	m, err := NewServerProbeManager(serverProbeTargets(), rec.send, 0, nil)
	require.NoError(t, err)

	_, ok := m.ProbeOnceByName("no-such-target")
	assert.False(t, ok, "unknown target reported by ok=false")
	assert.Equal(t, []string{"server-db", "platform-api"}, m.Targets())
}

// Start 对 nil 接收者安全。
func TestServerProbeManager_StartNilSafe(t *testing.T) {
	var m *ServerProbeManager
	m.Start(context.Background())
}
