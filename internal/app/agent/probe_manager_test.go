package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/cuihairu/croupier/core/healthprobe"
	"github.com/cuihairu/croupier/core/report"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// recordingSend 收集上行 AlertEvent 正文（并发安全）。
type recordingSend struct {
	mu  sync.Mutex
	buf [][]byte
}

func (r *recordingSend) send(_ context.Context, body []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = append(r.buf, append([]byte(nil), body...))
	return nil
}

func (r *recordingSend) events(t *testing.T) []*opsv1.AlertEvent {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*opsv1.AlertEvent, 0, len(r.buf))
	for _, b := range r.buf {
		ev := &opsv1.AlertEvent{}
		require.NoError(t, proto.Unmarshal(b, ev))
		out = append(out, ev)
	}
	return out
}

// deadTCPTarget 指向闭合端口（RFC 抓取段确定拒绝/超时），保证 tcp Checker 失败。
const deadTCPTarget = "127.0.0.1:1"

func tcpTargets() []healthprobe.TargetConfig {
	return []healthprobe.TargetConfig{
		{Name: "down-a", Category: healthprobe.TargetCategoryGame, Type: healthprobe.TargetTypeTCP, Address: deadTCPTarget, FailThreshold: 1},
		{Name: "down-b", Category: healthprobe.TargetCategoryThirdParty, Type: healthprobe.TargetTypeTCP, Address: deadTCPTarget, FailThreshold: 1},
	}
}

func targetOf(ev *opsv1.AlertEvent) string {
	if ev.Window != nil {
		return ev.Window.Target
	}
	return ev.Detail.Target
}

func TestProbeManagerRejectsInvalidConfigs(t *testing.T) {
	// 「空集」自 R4 起合法（空静态 + 动态可增删的探测面，见
	// TestNewProbeManagerEmptyStaticSetIsDynamicOnly），不再属拒绝面。
	cases := map[string][]healthprobe.TargetConfig{
		"duplicate name": {
			{Name: "dup", Category: healthprobe.TargetCategoryGame, Type: healthprobe.TargetTypeTCP, Address: deadTCPTarget},
			{Name: "dup", Category: healthprobe.TargetCategoryGame, Type: healthprobe.TargetTypeTCP, Address: deadTCPTarget},
		},
		"unknown type":     {{Name: "x", Category: healthprobe.TargetCategoryGame, Type: "carrier-pigeon"}},
		"missing category": {{Name: "x", Type: healthprobe.TargetTypeTCP, Address: deadTCPTarget}},
		"unknown category": {{Name: "x", Category: "external", Type: healthprobe.TargetTypeTCP, Address: deadTCPTarget}},
		"missing address":  {{Name: "x", Category: healthprobe.TargetCategoryGame, Type: healthprobe.TargetTypeTCP}},
	}
	for name, cfgs := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewProbeManager(cfgs, ProbeManagerOptions{AgentID: "agent-1"})
			assert.Error(t, err)
		})
	}
}

func TestProbeManagerUplinksPerTarget(t *testing.T) {
	s := &recordingSend{}
	pm, err := NewProbeManager(tcpTargets(), ProbeManagerOptions{
		AgentID: "agent-1",
		GameID:  "demo",
		Env:     "prod",
		Send:    s.send,
	})
	require.NoError(t, err)
	require.Len(t, pm.Probes(), 2)

	pm.ProbeManagerOnce(context.Background())

	evs := s.events(t)
	require.NotEmpty(t, evs)

	windows := map[string]int{}
	details := map[string]int{}
	for _, ev := range evs {
		assert.Equal(t, "agent-1", ev.AgentId)
		assert.Equal(t, "demo", ev.GameId)
		assert.Equal(t, "prod", ev.Env)
		if targetOf(ev) == "down-a" {
			assert.Equal(t, healthprobe.TargetCategoryGame, ev.TargetCategory)
		}
		if targetOf(ev) == "down-b" {
			assert.Equal(t, healthprobe.TargetCategoryThirdParty, ev.TargetCategory)
		}
		switch ev.Kind {
		case report.ProbeKindWindow:
			windows[ev.Window.Target]++
		case report.ProbeKindDetail:
			details[ev.Detail.Target]++
			assert.NotEmpty(t, ev.Detail.Err, "failure detail must carry the reason, never a bare bool")
		default:
			t.Fatalf("unexpected kind %q", ev.Kind)
		}
	}
	// 每个目标各自开窗 → 各自一条 probe.window，互不串台。
	assert.Equal(t, 1, windows["down-a"])
	assert.Equal(t, 1, windows["down-b"])
	// 异常明细与窗口事件都即时回流（不对称上行：失败不等批量窗口）。
	assert.GreaterOrEqual(t, details["down-a"], 1)
	assert.GreaterOrEqual(t, details["down-b"], 1)

	// 窗口消费者只上行：不产生任何恢复/重启副作用——此处断言开窗数与探针数
	// 一一对应（多一次即意味着有副作用路径）。
	assert.Len(t, pm.currentWindow(), 2)
}

func TestProbeManagerDrainStats(t *testing.T) {
	s := &recordingSend{}
	pm, err := NewProbeManager(tcpTargets(), ProbeManagerOptions{
		AgentID: "agent-1",
		GameID:  "demo",
		Send:    s.send,
	})
	require.NoError(t, err)

	assert.Empty(t, pm.DrainProbeStats(), "no probe yet → no stats row")

	pm.ProbeManagerOnce(context.Background())
	stats := pm.DrainProbeStats()
	require.Len(t, stats, 2, "one row per target")
	byTarget := map[string]*opsv1.ProbeStats{}
	for _, st := range stats {
		byTarget[st.Target] = st
	}
	require.Contains(t, byTarget, "down-a")
	assert.EqualValues(t, 1, byTarget["down-a"].Checked)
	assert.EqualValues(t, 1, byTarget["down-a"].Failed)
	assert.Equal(t, 0.0, byTarget["down-a"].SuccessRate)
	assert.True(t, byTarget["down-a"].WindowOpen)
	assert.NotEmpty(t, byTarget["down-a"].LastReason)
	assert.Equal(t, "demo", byTarget["down-a"].GameId)

	// Drain 已复位：下一轮无新探测不重复上报。
	assert.Empty(t, pm.DrainProbeStats())
}

func TestProbeManagerWithoutSendStillProbes(t *testing.T) {
	// server 未接入时 agent 照跑（M2 可回退语义）。
	pm, err := NewProbeManager([]healthprobe.TargetConfig{
		{Name: "down", Category: healthprobe.TargetCategoryGame, Type: healthprobe.TargetTypeTCP, Address: deadTCPTarget, FailThreshold: 1},
	}, ProbeManagerOptions{AgentID: "agent-1"})
	require.NoError(t, err)
	pm.ProbeManagerOnce(context.Background())
	stats := pm.DrainProbeStats()
	require.Len(t, stats, 1)
	assert.EqualValues(t, 1, stats[0].Failed)
}

// TestProbeManagerCarriesTypeProtocolPhase 静态与动态（R4 下行）两条装配路径
// 都把 probeType/protocol 带进聚合统计（§5.5.3 标注面）；adapter 名只在
// `protocol:` 子路径非空，声明式 steps 路径留空。
func TestProbeManagerCarriesTypeProtocolPhase(t *testing.T) {
	s := &recordingSend{}
	pm, err := NewProbeManager([]healthprobe.TargetConfig{
		{Name: "mc", Category: healthprobe.TargetCategoryGame, Type: healthprobe.TargetTypeProtocol, Address: deadTCPTarget,
			Steps:         []healthprobe.StepSpec{{Name: "handshake", Send: []byte("HELO")}},
			FailThreshold: 1},
	}, ProbeManagerOptions{AgentID: "agent-1", Send: s.send})
	require.NoError(t, err)
	pm.ProbeManagerOnce(context.Background())
	stats := pm.DrainProbeStats()
	require.Len(t, stats, 1)
	assert.Equal(t, "protocol", stats[0].ProbeType)
	assert.Equal(t, healthprobe.TargetCategoryGame, stats[0].TargetCategory)
	assert.Empty(t, stats[0].Protocol, "声明式 steps 未绑定适配器，不造名")
	assert.Equal(t, "dial", stats[0].Phase)

	// 动态目标走同一漏斗：适配器路径带上 protocol 名；协议期望声明位
	//（reachable-unverified 触发）静态/动态两条路径同样透传。
	pm2, err := NewProbeManager(nil, ProbeManagerOptions{AgentID: "agent-2"})
	require.NoError(t, err)
	require.NoError(t, pm2.ApplyTargets([]healthprobe.TargetConfig{
		{Name: "gw", Category: healthprobe.TargetCategoryGame, Type: healthprobe.TargetTypeProtocol, Address: deadTCPTarget,
			Protocol: "minecraft", FailThreshold: 1, Interval: time.Minute},
	}))
	pm2.ProbeManagerOnce(context.Background())
	stats = pm2.DrainProbeStats()
	require.Len(t, stats, 1)
	assert.Equal(t, "gw", stats[0].Target)
	assert.Equal(t, "protocol", stats[0].ProbeType)
	assert.Equal(t, healthprobe.TargetCategoryGame, stats[0].TargetCategory, "动态下行路径同样透传类别")
	assert.Equal(t, "minecraft", stats[0].Protocol)
	assert.Equal(t, "dial", stats[0].Phase)
}

// TestProbeManagerCarriesProtocolExpected 静态与动态装配路径都把协议期望
// 声明位带进聚合统计（§5.5.3 reachable-unverified）：L1 目标声明后面板才
// 能标「可达≠可用」；未声明不带。
func TestProbeManagerCarriesProtocolExpected(t *testing.T) {
	pm, err := NewProbeManager([]healthprobe.TargetConfig{
		{Name: "gw-tcp", Category: healthprobe.TargetCategoryGame, Type: healthprobe.TargetTypeTCP, Address: deadTCPTarget,
			ProtocolExpected: true, FailThreshold: 1},
	}, ProbeManagerOptions{AgentID: "agent-1"})
	require.NoError(t, err)
	pm.ProbeManagerOnce(context.Background())
	stats := pm.DrainProbeStats()
	require.Len(t, stats, 1)
	assert.True(t, stats[0].ProtocolExpected)
	assert.Equal(t, "tcp", stats[0].ProbeType, "声明位不改写实际探测类型")

	pm2, err := NewProbeManager(nil, ProbeManagerOptions{AgentID: "agent-2"})
	require.NoError(t, err)
	require.NoError(t, pm2.ApplyTargets([]healthprobe.TargetConfig{
		{Name: "gw-http", Category: healthprobe.TargetCategoryThirdParty, Type: healthprobe.TargetTypeHTTP, URL: "http://127.0.0.1:1/hz",
			ProtocolExpected: true, FailThreshold: 1, Interval: time.Minute},
	}))
	pm2.ProbeManagerOnce(context.Background())
	stats = pm2.DrainProbeStats()
	require.Len(t, stats, 1)
	assert.True(t, stats[0].ProtocolExpected, "动态（R4 下行）路径同样透传声明位")

	// 未声明的静态目标不带该位（零值 false）。
	pm3, err := NewProbeManager([]healthprobe.TargetConfig{
		{Name: "plain", Category: healthprobe.TargetCategoryGame, Type: healthprobe.TargetTypeTCP, Address: deadTCPTarget,
			FailThreshold: 1},
	}, ProbeManagerOptions{AgentID: "agent-3"})
	require.NoError(t, err)
	pm3.ProbeManagerOnce(context.Background())
	stats = pm3.DrainProbeStats()
	require.Len(t, stats, 1)
	assert.False(t, stats[0].ProtocolExpected)
}

func TestProbeManagerNilSafe(t *testing.T) {
	var pm *ProbeManager
	assert.Nil(t, pm.DrainProbeStats())
	assert.Nil(t, pm.Probes())
	pm.Start(context.Background())
	pm.ProbeManagerOnce(context.Background())
}

func TestProbeManagerStartRespectsContext(t *testing.T) {
	pm, err := NewProbeManager([]healthprobe.TargetConfig{
		{Name: "up", Category: healthprobe.TargetCategoryThirdParty, Type: healthprobe.TargetTypeHTTP, URL: "http://127.0.0.1:1/healthz", Interval: 10 * time.Millisecond},
	}, ProbeManagerOptions{AgentID: "agent-1"})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		pm.Start(ctx)
		close(done)
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ProbeManager.Start did not stop on context cancel")
	}
}
