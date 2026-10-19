package agent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/core/healthprobe"
)

// WithProbeConfig 装配面（§12 R2）：配置合法才起装配，非法即报错——观测面
// 配置错了却静默不探，比直接报错危险。
// newProbeTestApp 构造带 upstream 的 App（upstream 只作装配依赖，不拨号）。
func newProbeTestApp(t *testing.T) *App {
	t.Helper()
	return New("127.0.0.1:1", "agent-probe-test")
}

func TestWithProbeConfigAssembles(t *testing.T) {
	app := newProbeTestApp(t)

	require.NoError(t, app.WithProbeConfig(&ProbeConfig{Targets: []healthprobe.TargetConfig{
		{Name: "down", Category: healthprobe.TargetCategoryGame, Type: healthprobe.TargetTypeTCP, Address: deadTCPTarget, FailThreshold: 1},
	}}))
	pm := app.ProbeManager()
	require.NotNil(t, pm)
	assert.Len(t, pm.Probes(), 1)

	// 统计折进 metrics 批量面（正常态聚合面，§13.1）。
	pm.ProbeManagerOnce(context.Background())
	stats := pm.DrainProbeStats()
	require.Len(t, stats, 1)
	assert.EqualValues(t, 1, stats[0].Failed)
}

func TestWithProbeConfigRejectsInvalid(t *testing.T) {
	app := newProbeTestApp(t)

	err := app.WithProbeConfig(&ProbeConfig{Targets: []healthprobe.TargetConfig{
		{Name: "no-address", Category: healthprobe.TargetCategoryGame, Type: healthprobe.TargetTypeTCP},
	}})
	require.Error(t, err)
	assert.Nil(t, app.ProbeManager(), "invalid config must not leave a half-assembled probe face")

	// 重名目标会让上行路由串台，必须拒收。
	err = app.WithProbeConfig(&ProbeConfig{Targets: []healthprobe.TargetConfig{
		{Name: "dup", Category: healthprobe.TargetCategoryGame, Type: healthprobe.TargetTypeTCP, Address: deadTCPTarget},
		{Name: "dup", Category: healthprobe.TargetCategoryGame, Type: healthprobe.TargetTypeTCP, Address: deadTCPTarget},
	}})
	require.Error(t, err)
	assert.Nil(t, app.ProbeManager())
}

func TestWithProbeConfigNilIsNoop(t *testing.T) {
	app := newProbeTestApp(t)
	// 无 probe: 段 = 无探测面（R4 契约不变：nil 配置仍不装配）。
	require.NoError(t, app.WithProbeConfig(nil))
	assert.Nil(t, app.ProbeManager())
	// 有 probe: 段但 targets 为空 = 合法声明（空静态 + 动态可增删，R4 起装配）。
	require.NoError(t, app.WithProbeConfig(&ProbeConfig{}))
	assert.NotNil(t, app.ProbeManager())
	assert.Empty(t, app.ProbeManager().DynamicTargets())

	// 未配置探测时启动路径不 panic。
	app.startProbes(context.Background())

	var nilApp *App
	assert.Nil(t, nilApp.ProbeManager())
	require.NoError(t, nilApp.WithProbeConfig(&ProbeConfig{}))
	nilApp.startProbes(context.Background())
}

func TestWithProbeConfigWithoutUpstream(t *testing.T) {
	app := &App{}
	assert.Error(t, app.WithProbeConfig(&ProbeConfig{Targets: []healthprobe.TargetConfig{
		{Name: "t", Category: healthprobe.TargetCategoryGame, Type: healthprobe.TargetTypeTCP, Address: deadTCPTarget},
	}}))
}

func TestStartProbesStopsOnCancel(t *testing.T) {
	app := newProbeTestApp(t)
	require.NoError(t, app.WithProbeConfig(&ProbeConfig{Targets: []healthprobe.TargetConfig{
		{
			Name: "up", Category: healthprobe.TargetCategoryThirdParty, Type: healthprobe.TargetTypeHTTP,
			URL: "http://127.0.0.1:1/healthz", Interval: 10 * time.Millisecond,
		},
	}}))

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		app.startProbes(ctx)
		// startProbes 自身立即返回（内部起 goroutine）；等一小段确认探针
		// goroutine 也随 ctx 退出：context 取消后不再产生新统计窗口。
		time.Sleep(60 * time.Millisecond)
		cancel()
		time.Sleep(30 * time.Millisecond)
		stats := app.ProbeManager().DrainProbeStats()
		first := len(stats)
		time.Sleep(40 * time.Millisecond)
		assert.Len(t, app.ProbeManager().DrainProbeStats(), 0, "probes must stop on cancel")
		assert.GreaterOrEqual(t, first, 1)
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("probe start/stop did not settle")
	}
}
