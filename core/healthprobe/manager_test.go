package healthprobe

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// httpTarget 恒失败目标（127.0.0.1:1 立即拒绝，阈值 1）：Checker 失败即计数，
// 结果回调可断言探针确实在跑，无需真实连通依赖。短间隔保证生命周期断言在
// 亚秒级完成。
func httpTarget(name string) TargetConfig {
	return TargetConfig{
		Name: name, Type: TargetTypeHTTP,
		URL: "http://127.0.0.1:1/unreachable", Timeout: 100 * time.Millisecond,
		FailThreshold: 1, Interval: 20 * time.Millisecond,
	}
}

func TestManagerAddRunsImmediatelyAfterStart(t *testing.T) {
	var probes atomic.Int32
	m := NewDynamicManager("", "", func(WindowEvent) {}, func(Result) { probes.Add(1) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		m.Start(ctx)
		close(done)
	}()

	// Start 后 Add：探针立即起跑（首个 ProbeOnce 先于下一间隔）。
	require.NoError(t, m.Add(httpTarget("dyn-a")))
	require.Eventually(t, func() bool { return probes.Load() >= 1 },
		2*time.Second, 10*time.Millisecond, "added probe must run its first check")

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("manager start did not exit after ctx cancel")
	}
}

func TestManagerAddBeforeStartPickedUpAtStart(t *testing.T) {
	var probes atomic.Int32
	m := NewDynamicManager("", "", nil, func(Result) { probes.Add(1) })
	require.NoError(t, m.Add(httpTarget("early-a")))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		m.Start(ctx)
		close(done)
	}()
	require.Eventually(t, func() bool { return probes.Load() >= 1 },
		2*time.Second, 10*time.Millisecond, "pre-Start item must be picked up by Start")

	cancel()
	<-done
}

func TestManagerRemoveStopsProbe(t *testing.T) {
	var probes atomic.Int32
	m := NewDynamicManager("", "", nil, func(Result) { probes.Add(1) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Start(ctx)
	require.NoError(t, m.Add(httpTarget("stop-a")))
	require.Eventually(t, func() bool { return probes.Load() >= 1 },
		2*time.Second, 10*time.Millisecond)

	require.NoError(t, m.Remove("stop-a"))
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, len(m.Probes()), "removed probe must leave the manager")

	before := probes.Load()
	time.Sleep(80 * time.Millisecond)
	assert.Equal(t, before, probes.Load(), "removed probe must stop probing")
}

func TestManagerRemoveUnknownTargetErrors(t *testing.T) {
	m := NewDynamicManager("", "", nil, nil)
	err := m.Remove("ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManagerAddDuplicateRejected(t *testing.T) {
	m := NewDynamicManager("", "", nil, nil)
	require.NoError(t, m.Add(httpTarget("dup-a")))
	err := m.Add(httpTarget("dup-a"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate target name")
	assert.Equal(t, 1, len(m.Probes()))
}

func TestManagerAddInvalidTargetRejected(t *testing.T) {
	m := NewDynamicManager("", "", nil, nil)
	err := m.Add(TargetConfig{Name: "bad", Type: TargetTypeTCP})
	require.Error(t, err)
	assert.Equal(t, 0, len(m.Probes()), "failed add must not leave a half item")

	err = m.Add(TargetConfig{Name: "  ", Type: TargetTypeTCP, Address: "x:1"})
	require.Error(t, err)
}

func TestManagerAddEmptyNameRejected(t *testing.T) {
	m := NewDynamicManager("", "", nil, nil)
	err := m.Add(TargetConfig{Type: TargetTypeHTTP, URL: "http://x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}
