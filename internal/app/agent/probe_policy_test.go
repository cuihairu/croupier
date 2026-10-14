package agent

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/cuihairu/croupier/core/configsync"
	"github.com/cuihairu/croupier/core/healthprobe"
	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// policyTargets 双目标：down-a/down-b（闭合端口，确定失败），各带本地间隔。
func policyTargets() []healthprobe.TargetConfig {
	return []healthprobe.TargetConfig{
		{Name: "down-a", Type: healthprobe.TargetTypeTCP, Address: deadTCPTarget,
			FailThreshold: 1, Interval: 20 * time.Second},
		{Name: "down-b", Type: healthprobe.TargetTypeTCP, Address: deadTCPTarget,
			FailThreshold: 1, Interval: 25 * time.Second},
	}
}

func newPolicyTestManager(t *testing.T) *ProbeManager {
	t.Helper()
	pm, err := NewProbeManager(policyTargets(), ProbeManagerOptions{
		AgentID: "agent-1",
		GameID:  "demo",
		Env:     "prod",
		Send:    (&recordingSend{}).send,
	})
	require.NoError(t, err)
	return pm
}

// 未下发策略时回落本地 probe.targets 声明的间隔（不写死 agent 默认）。
func TestProbeManagerIntervalFallsBackToLocalConfig(t *testing.T) {
	pm := newPolicyTestManager(t)
	now := time.Now()
	assert.Nil(t, pm.Policy())
	assert.Equal(t, 20*time.Second, pm.IntervalFor("down-a", now))
	assert.Equal(t, 25*time.Second, pm.IntervalFor("down-b", now))
}

// 策略热更即换快照：基础间隔 + 按目标覆盖，未覆盖者仍取策略基础间隔。
func TestProbeManagerPolicyOverridesLocalConfig(t *testing.T) {
	pm := newPolicyTestManager(t)
	policy, err := healthprobe.ParseProbePolicy(
		`{"interval":"10s","targets":[{"target":"down-b","interval":"3s"}]}`)
	require.NoError(t, err)

	pm.SetPolicy(policy)
	now := time.Now()
	assert.Equal(t, 10*time.Second, pm.IntervalFor("down-a", now))
	assert.Equal(t, 3*time.Second, pm.IntervalFor("down-b", now))
	assert.Equal(t, 10*time.Second, pm.Policy().BaseInterval())
}

// 清除策略（nil）回落到本地配置间隔——「撤销下发」有确定语义。
func TestProbeManagerClearPolicyRestoresLocalInterval(t *testing.T) {
	pm := newPolicyTestManager(t)
	policy, err := healthprobe.ParseProbePolicy(`{"interval":"4s"}`)
	require.NoError(t, err)
	pm.SetPolicy(policy)
	require.Equal(t, 4*time.Second, pm.IntervalFor("down-a", time.Now()))

	pm.SetPolicy(nil)
	assert.Nil(t, pm.Policy())
	assert.Equal(t, 20*time.Second, pm.IntervalFor("down-a", time.Now()))
}

// 异常态提频：开窗目标间隔减半并有 5s 下限；恢复后回原频率；其他目标不受影响。
func TestProbeManagerBoostWhileWindowOpen(t *testing.T) {
	pm := newPolicyTestManager(t)
	policy, err := healthprobe.ParseProbePolicy(`{"interval":"30s"}`)
	require.NoError(t, err)
	pm.SetPolicy(policy)

	now := time.Now()
	require.Equal(t, 30*time.Second, pm.IntervalFor("down-a", now))

	// down-a 探一次失败（FailThreshold=1）→ 窗口打开 → 提频 15s；down-b 未探仍 30s。
	probes := pm.Probes()
	require.Len(t, probes, 2)
	probes[0].ProbeOnce(context.Background())

	assert.Equal(t, 15*time.Second, pm.IntervalFor("down-a", now))
	assert.Equal(t, 30*time.Second, pm.IntervalFor("down-b", now))
}

// 提频下限：策略间隔 6s 时减半到 3s 被 5s 下限托住（绝不慢于原值）。
func TestProbeManagerBoostRespectsFloor(t *testing.T) {
	pm := newPolicyTestManager(t)
	policy, err := healthprobe.ParseProbePolicy(`{"interval":"6s"}`)
	require.NoError(t, err)
	pm.SetPolicy(policy)
	pm.Probes()[0].ProbeOnce(context.Background())

	assert.Equal(t, 5*time.Second, pm.IntervalFor("down-a", time.Now()))
}

// autoBoost=false 时开窗不提速（策略显式关）。
func TestProbeManagerBoostCanBeDisabled(t *testing.T) {
	pm := newPolicyTestManager(t)
	policy, err := healthprobe.ParseProbePolicy(`{"interval":"30s","autoBoost":false}`)
	require.NoError(t, err)
	pm.SetPolicy(policy)
	pm.Probes()[0].ProbeOnce(context.Background())

	assert.Equal(t, 30*time.Second, pm.IntervalFor("down-a", time.Now()))
}

// fetchSeq 依次返回预置拉取结果（末条重复复用），记录调用次数。
type fetchSeq struct {
	calls int
	resp  []*configsync.Versioned[string]
	errs  []error
}

func (f *fetchSeq) fetch(context.Context) (*configsync.Versioned[string], error) {
	i := f.calls
	f.calls++
	if i >= len(f.resp) {
		i = len(f.resp) - 1
	}
	if i < len(f.errs) && f.errs[i] != nil {
		return nil, f.errs[i]
	}
	return f.resp[i], nil
}

// 下发即生效；版本未变则跳过（省叶轮，不重复生效）。
func TestProbePolicyPullerAppliesAndSkipsSameVersion(t *testing.T) {
	pm := newPolicyTestManager(t)
	f := &fetchSeq{resp: []*configsync.Versioned[string]{
		{Version: "7", Payload: `{"interval":"10s"}`},
	}}
	p := NewProbePolicyPuller(pm, f.fetch, configsync.DefaultInterval, slog.New(slog.DiscardHandler))

	applied, err := p.PullOnce(context.Background())
	require.NoError(t, err)
	assert.True(t, applied)
	assert.Equal(t, 10*time.Second, pm.IntervalFor("down-a", time.Now()))
	assert.Equal(t, "7", p.LastVersion())

	applied, err = p.PullOnce(context.Background())
	require.NoError(t, err)
	assert.False(t, applied, "same version must not re-apply")
	assert.Equal(t, 10*time.Second, pm.IntervalFor("down-a", time.Now()))
}

// 新版本热换快照。
func TestProbePolicyPullerAppliesNewVersion(t *testing.T) {
	pm := newPolicyTestManager(t)
	f := &fetchSeq{resp: []*configsync.Versioned[string]{
		{Version: "7", Payload: `{"interval":"10s"}`},
		{Version: "8", Payload: `{"interval":"15s"}`},
	}}
	p := NewProbePolicyPuller(pm, f.fetch, configsync.DefaultInterval, slog.New(slog.DiscardHandler))

	_, err := p.PullOnce(context.Background())
	require.NoError(t, err)
	applied, err := p.PullOnce(context.Background())
	require.NoError(t, err)
	assert.True(t, applied)
	assert.Equal(t, 15*time.Second, pm.IntervalFor("down-a", time.Now()))
}

// 坏策略不生效、版本游标不推进：保持最后已知策略，下轮重试。
func TestProbePolicyPullerKeepsLastGoodOnBadPolicy(t *testing.T) {
	pm := newPolicyTestManager(t)
	good, err := healthprobe.ParseProbePolicy(`{"interval":"10s"}`)
	require.NoError(t, err)
	pm.SetPolicy(good)

	f := &fetchSeq{resp: []*configsync.Versioned[string]{
		{Version: "9", Payload: `{"interval":"not-a-duration"}`},
	}}
	p := NewProbePolicyPuller(pm, f.fetch, configsync.DefaultInterval, slog.New(slog.DiscardHandler))

	applied, err := p.PullOnce(context.Background())
	require.Error(t, err)
	assert.False(t, applied)
	assert.Equal(t, 10*time.Second, pm.IntervalFor("down-a", time.Now()), "last good policy kept")
	assert.Empty(t, p.LastVersion(), "version cursor must not advance on failed apply")
}

// 服务端未发布（空版本空内容）保持现状，不算错误、不清空已有策略。
func TestProbePolicyPullerEmptyPayloadKeepsCurrent(t *testing.T) {
	pm := newPolicyTestManager(t)
	good, err := healthprobe.ParseProbePolicy(`{"interval":"10s"}`)
	require.NoError(t, err)
	pm.SetPolicy(good)

	f := &fetchSeq{resp: []*configsync.Versioned[string]{{Version: "", Payload: ""}}}
	p := NewProbePolicyPuller(pm, f.fetch, configsync.DefaultInterval, slog.New(slog.DiscardHandler))

	applied, err := p.PullOnce(context.Background())
	require.NoError(t, err)
	assert.False(t, applied)
	assert.Equal(t, 10*time.Second, pm.IntervalFor("down-a", time.Now()))
}

// 拉取失败（通道未连/读失败）向上传播并保持现状。
func TestProbePolicyPullerFetchErrorPropagates(t *testing.T) {
	pm := newPolicyTestManager(t)
	f := &fetchSeq{
		resp: []*configsync.Versioned[string]{{Version: "1", Payload: `{"interval":"10s"}`}},
		errs: []error{assert.AnError},
	}
	p := NewProbePolicyPuller(pm, f.fetch, configsync.DefaultInterval, slog.New(slog.DiscardHandler))

	applied, err := p.PullOnce(context.Background())
	require.Error(t, err)
	assert.False(t, applied)
	assert.Equal(t, 20*time.Second, pm.IntervalFor("down-a", time.Now()))
}

// 装配不全（无探针面/无拉取 seam）时空转不 panic：PullOnce 报错、Start 直接返回。
func TestProbePolicyPullerNilSafe(t *testing.T) {
	pm := newPolicyTestManager(t)

	var nilPuller *ProbePolicyPuller
	nilPuller.Start(context.Background())
	_, err := nilPuller.PullOnce(context.Background())
	assert.Error(t, err)

	// 有探针面无 fetch seam。
	noFetch := NewProbePolicyPuller(pm, nil, 0, nil)
	_, err = noFetch.PullOnce(context.Background())
	assert.Error(t, err)
	noFetch.Start(context.Background())

	// 有 fetch seam 无探针面：拉取正常返回，但无处生效（不 panic、不静默生效）。
	orphan := NewProbePolicyPuller(nil, func(context.Context) (*configsync.Versioned[string], error) {
		return &configsync.Versioned[string]{Version: "3", Payload: `{"interval":"10s"}`}, nil
	}, configsync.DefaultInterval, nil)
	applied, err := orphan.PullOnce(context.Background())
	require.NoError(t, err)
	assert.False(t, applied)
}

// Start 拉起轮询循环：策略在首轮即生效（不等一个周期）。
func TestProbePolicyPullerStartAppliesImmediately(t *testing.T) {
	pm := newPolicyTestManager(t)
	f := &fetchSeq{resp: []*configsync.Versioned[string]{
		{Version: "1", Payload: `{"interval":"9s"}`},
	}}
	p := NewProbePolicyPuller(pm, f.fetch, 20*time.Millisecond, slog.New(slog.DiscardHandler))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	require.Eventually(t, func() bool {
		return pm.IntervalFor("down-a", time.Now()) == 9*time.Second
	}, 2*time.Second, 10*time.Millisecond)
}

// 下行 wire：agent 侧 PullProbePolicy 发出 AgentId 并解码响应。
func TestUpstreamPullProbePolicyDecodesResponse(t *testing.T) {
	resp := &opsv1.ProbePolicyConfigResponse{Version: "12", PolicyJson: `{"interval":"10s"}`}
	raw, err := proto.Marshal(resp)
	require.NoError(t, err)

	cc := &mockControlClientV9{}
	cc.pullProbePolicy = func(context.Context, []byte) ([]byte, error) { return raw, nil }

	c := NewUpstreamClient("mock", "agent-pp-decode", agentlocal.NewLocalStore(), nil)
	c.setClient(cc)

	got, err := c.PullProbePolicy(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "12", got.GetVersion())
	assert.Equal(t, `{"interval":"10s"}`, got.GetPolicyJson())
}

func TestUpstreamPullProbePolicyErrors(t *testing.T) {
	c := NewUpstreamClient("mock", "agent-pp-err", agentlocal.NewLocalStore(), nil)
	_, err := c.PullProbePolicy(context.Background())
	require.Error(t, err, "not connected must error, not panic")

	var nilClient *UpstreamClient
	_, err = nilClient.PullProbePolicy(context.Background())
	assert.Error(t, err)

	cc := &mockControlClientV9{}
	cc.pullProbePolicy = func(context.Context, []byte) ([]byte, error) { return nil, assert.AnError }
	c.setClient(cc)
	_, err = c.PullProbePolicy(context.Background())
	assert.Error(t, err)
}

// tcp/mux 两面 wire 出口：客户端未装配时报错而非 panic。
func TestControlClientsPullProbePolicyNilSafe(t *testing.T) {
	var tcp *tcpControlClient
	_, err := tcp.PullProbePolicy(context.Background(), nil)
	assert.Error(t, err)

	var mux *muxControlClient
	_, err = mux.PullProbePolicy(context.Background(), nil)
	assert.Error(t, err)
}
