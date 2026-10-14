package agent

import (
	"context"
	"errors"
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

// dbTargets 归属派生的 db 目标声明形态（server 派生前缀 db:、timeout=5s）。
func dbTargets(names ...string) []healthprobe.TargetConfig {
	out := make([]healthprobe.TargetConfig, 0, len(names))
	for _, n := range names {
		out = append(out, healthprobe.TargetConfig{
			Name: "db:" + n, Type: healthprobe.TargetTypeDB,
			Driver: "pgx", DSN: "postgres://ro:ro@127.0.0.1/db",
			Timeout: healthprobe.DefaultDBTimeout,
		})
	}
	return out
}

// NewProbeManager 空静态集 = 动态可增删的探测面（R4 参与语义）。
func TestNewProbeManagerEmptyStaticSetIsDynamicOnly(t *testing.T) {
	pm, err := NewProbeManager(nil, ProbeManagerOptions{AgentID: "agent-1", GameID: "demo", Env: "prod"})
	require.NoError(t, err)
	assert.Empty(t, pm.DynamicTargets())
	assert.NotNil(t, pm.Probes(), "manager must exist so Add can start probes")
}

// 新增目标即装配；缺失目标即卸载（解除归属/源被删）。
func TestApplyTargetsAddsAndRemovesDynamic(t *testing.T) {
	pm, err := NewProbeManager(nil, ProbeManagerOptions{AgentID: "agent-1", Send: (&recordingSend{}).send})
	require.NoError(t, err)

	require.NoError(t, pm.ApplyTargets(dbTargets("orders")))
	assert.Equal(t, []string{"db:orders"}, pm.DynamicTargets())
	require.Len(t, pm.Probes(), 1)
	assert.True(t, pm.IntervalFor("db:orders", time.Now()) > 0)

	require.NoError(t, pm.ApplyTargets(dbTargets("orders", "audit")))
	assert.Equal(t, []string{"db:audit", "db:orders"}, pm.DynamicTargets())
	require.Len(t, pm.Probes(), 2)

	// 空集 = 解除全部归属，必须卸干净。
	require.NoError(t, pm.ApplyTargets(nil))
	assert.Empty(t, pm.DynamicTargets())
	assert.Empty(t, pm.Probes())
}

// 配置未变即保留原探针（窗口状态不清零）；配置变了才重建。
func TestApplyTargetsKeepsProbeWhenConfigUnchanged(t *testing.T) {
	pm, err := NewProbeManager(nil, ProbeManagerOptions{AgentID: "agent-1"})
	require.NoError(t, err)

	require.NoError(t, pm.ApplyTargets(dbTargets("orders")))
	before := pm.Probes()[0]

	require.NoError(t, pm.ApplyTargets(dbTargets("orders")))
	assert.Same(t, before, pm.Probes()[0], "unchanged config must keep the running probe")

	changed := dbTargets("orders")
	changed[0].DSN = "postgres://ro2:ro2@127.0.0.1/db"
	require.NoError(t, pm.ApplyTargets(changed))
	assert.NotSame(t, before, pm.Probes()[0], "changed config must rebuild the probe")
	// 重建后名字与动态记账保持一致（DSN 属探针内部 checker，不外露）。
	assert.Equal(t, []string{"db:orders"}, pm.DynamicTargets())
}

// 与本地 probe.targets[] 撞名 → 整体拒绝（上行路由按名分流，撞名静默串台）。
func TestApplyTargetsRejectsCollisionWithLocalTargets(t *testing.T) {
	pm, err := NewProbeManager(tcpTargets(), ProbeManagerOptions{AgentID: "agent-1"})
	require.NoError(t, err)

	require.NoError(t, pm.ApplyTargets(dbTargets("orders"))) // 无冲突，基线可装
	require.Len(t, pm.Probes(), 3)                           // 2 静态 + 1 动态

	// 动态目标名与本地静态 down-a 相同 → 拒绝且不动现有装配。
	clash := append(dbTargets("orders"), healthprobe.TargetConfig{Name: "down-a", Type: healthprobe.TargetTypeDB, DSN: "x", Driver: "pgx"})
	err = pm.ApplyTargets(clash)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collides")
	assert.Equal(t, []string{"db:orders"}, pm.DynamicTargets())
	require.Len(t, pm.Probes(), 3)
}

// 目标非法（缺 DSN）→ 整体拒绝，不做半装配。
func TestApplyTargetsRejectsInvalidTargetAtomically(t *testing.T) {
	pm, err := NewProbeManager(nil, ProbeManagerOptions{AgentID: "agent-1"})
	require.NoError(t, err)

	bad := dbTargets("orders")
	bad[0].DSN = "" // db checker 必需 dsn + driver
	err = pm.ApplyTargets(bad)
	require.Error(t, err)
	assert.Empty(t, pm.DynamicTargets())
	assert.Empty(t, pm.Probes())

	// 重名目标集同样整体拒绝。
	dup := dbTargets("orders")
	dup = append(dup, dbTargets("orders")[0])
	err = pm.ApplyTargets(dup)
	require.Error(t, err)
	assert.Empty(t, pm.DynamicTargets())
}

// Puller：应用新版本、跳过同版本、坏载荷保持现状、空版本空正文保持现状。
func TestProbeDBTargetsPullerAppliesAndSkips(t *testing.T) {
	pm, err := NewProbeManager(nil, ProbeManagerOptions{AgentID: "agent-1"})
	require.NoError(t, err)

	payload := `[{"name":"db:orders","type":"db","driver":"pgx","dsn":"postgres://ro:ro@127.0.0.1/db","timeout":5000000000}]`
	v := configsync.Versioned[string]{Version: "hash-a", Payload: payload}
	fetch := func(context.Context) (*configsync.Versioned[string], error) { return &v, nil }

	p := NewProbeDBTargetsPuller(pm, fetch, configsync.DefaultInterval, nil)
	changed, err := p.PullOnce(context.Background())
	require.NoError(t, err)
	assert.True(t, changed, "first apply must count as a change")
	assert.Equal(t, "hash-a", p.LastVersion())
	assert.Equal(t, []string{"db:orders"}, pm.DynamicTargets())

	changed, err = p.PullOnce(context.Background()) // 同版本跳过
	require.NoError(t, err)
	assert.False(t, changed)

	p2 := NewProbeDBTargetsPuller(pm, func(context.Context) (*configsync.Versioned[string], error) {
		return &configsync.Versioned[string]{Version: "hash-b", Payload: `[]`}, nil
	}, configsync.DefaultInterval, nil)
	_, err = p2.PullOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "hash-b", p2.LastVersion())
	assert.Empty(t, pm.DynamicTargets(), "empty set must unload dynamic targets")
}

func TestProbeDBTargetsPullerKeepsLastGoodOnError(t *testing.T) {
	pm, err := NewProbeManager(nil, ProbeManagerOptions{AgentID: "agent-1"})
	require.NoError(t, err)
	payload := `[{"name":"db:orders","type":"db","driver":"pgx","dsn":"postgres://ro:ro@127.0.0.1/db"}]`
	p := NewProbeDBTargetsPuller(pm, func(context.Context) (*configsync.Versioned[string], error) {
		return &configsync.Versioned[string]{Version: "hash-a", Payload: payload}, nil
	}, configsync.DefaultInterval, nil)
	_, err = p.PullOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"db:orders"}, pm.DynamicTargets())

	// 坏载荷：版本游标不推进，现有目标集保持不变。
	p2 := NewProbeDBTargetsPuller(pm, func(context.Context) (*configsync.Versioned[string], error) {
		return &configsync.Versioned[string]{Version: "hash-b", Payload: `{`}, nil
	}, configsync.DefaultInterval, nil)
	_, err = p2.PullOnce(context.Background())
	require.Error(t, err)
	assert.Equal(t, []string{"db:orders"}, pm.DynamicTargets())

	// 拉取失败同样保持现状。
	p3 := NewProbeDBTargetsPuller(pm, func(context.Context) (*configsync.Versioned[string], error) {
		return nil, errors.New("upstream down")
	}, configsync.DefaultInterval, nil)
	_, err = p3.PullOnce(context.Background())
	require.Error(t, err)
	assert.Equal(t, []string{"db:orders"}, pm.DynamicTargets())
}

// 空版本 + 空正文 = 服务端不可派生：归一为无下发（保持现状，不算错误）。
func TestProbeDBTargetsPullerEmptyVersionAndPayloadKeepsCurrent(t *testing.T) {
	pm, err := NewProbeManager(nil, ProbeManagerOptions{AgentID: "agent-1"})
	require.NoError(t, err)
	payload := `[{"name":"db:orders","type":"db","driver":"pgx","dsn":"postgres://ro:ro@127.0.0.1/db"}]`
	p := NewProbeDBTargetsPuller(pm, func(context.Context) (*configsync.Versioned[string], error) {
		return &configsync.Versioned[string]{Version: "hash-a", Payload: payload}, nil
	}, configsync.DefaultInterval, nil)
	_, err = p.PullOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"db:orders"}, pm.DynamicTargets())

	p2 := NewProbeDBTargetsPuller(pm, func(context.Context) (*configsync.Versioned[string], error) {
		return &configsync.Versioned[string]{Version: "", Payload: ""}, nil
	}, configsync.DefaultInterval, nil)
	changed, err := p2.PullOnce(context.Background())
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Empty(t, p2.LastVersion(), "server-undervivable response must not advance version")
	assert.Equal(t, []string{"db:orders"}, pm.DynamicTargets())
}

// 空正文（有版本）是协议违约：报错而非静默清空。
func TestProbeDBTargetsPullerRejectsEmptyPayload(t *testing.T) {
	pm, err := NewProbeManager(nil, ProbeManagerOptions{AgentID: "agent-1"})
	require.NoError(t, err)
	p := NewProbeDBTargetsPuller(pm, func(context.Context) (*configsync.Versioned[string], error) {
		return &configsync.Versioned[string]{Version: "hash-x", Payload: "  "}, nil
	}, configsync.DefaultInterval, nil)
	_, err = p.PullOnce(context.Background())
	require.Error(t, err)
	assert.Empty(t, pm.DynamicTargets())
}

// Nil 安全与默认降级（未装配探针面/未注入 fetch）。
func TestProbeDBTargetsPullerNilSafe(t *testing.T) {
	var p *ProbeDBTargetsPuller
	_, err := p.PullOnce(context.Background())
	assert.Error(t, err)
	assert.Empty(t, p.LastVersion())
	p.Start(context.Background()) // 不得 panic

	pm, err := NewProbeManager(nil, ProbeManagerOptions{AgentID: "agent-1"})
	require.NoError(t, err)
	p = NewProbeDBTargetsPuller(pm, nil, configsync.DefaultInterval, nil)
	_, err = p.PullOnce(context.Background())
	require.Error(t, err)

	// fetch seam 未注入时 Start 只回告警不 panic，ctx 取消即退出循环。
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		p.Start(ctx)
		close(done)
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("puller start did not exit after ctx cancel")
	}
}

// UpstreamClient 出口：解码 db 目标响应 + 各错误面。
func TestUpstreamPullProbeDBTargetsDecodesResponse(t *testing.T) {
	resp := &opsv1.ProbeDBTargetsConfigResponse{
		Version:     "hash-1",
		TargetsJson: `[{"name":"db:orders","type":"db","driver":"pgx","dsn":"postgres://ro:ro@127.0.0.1/db"}]`,
	}
	raw, err := proto.Marshal(resp)
	require.NoError(t, err)

	cc := &mockControlClientV9{}
	cc.pullProbeDBTargets = func(context.Context, []byte) ([]byte, error) { return raw, nil }

	c := NewUpstreamClient("mock", "agent-td-decode", agentlocal.NewLocalStore(), nil)
	c.setClient(cc)

	got, err := c.PullProbeDBTargets(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "hash-1", got.GetVersion())
	require.NotEmpty(t, got.GetTargetsJson())

	// fetch seam 视角：返回 Versioned 载荷供 configsync 消费。
	gotv, err := (&App{upstream: c}).fetchProbeDBTargets(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "hash-1", gotv.Version)
	assert.Equal(t, got.GetTargetsJson(), gotv.Payload)
}

func TestUpstreamPullProbeDBTargetsErrors(t *testing.T) {
	c := NewUpstreamClient("mock", "agent-td-err", agentlocal.NewLocalStore(), nil)
	_, err := c.PullProbeDBTargets(context.Background())
	require.Error(t, err, "not connected must error, not panic")

	var nilClient *UpstreamClient
	_, err = nilClient.PullProbeDBTargets(context.Background())
	assert.Error(t, err)

	cc := &mockControlClientV9{}
	cc.pullProbeDBTargets = func(context.Context, []byte) ([]byte, error) { return nil, assert.AnError }
	c.setClient(cc)
	_, err = c.PullProbeDBTargets(context.Background())
	assert.Error(t, err)
}

// tcp/mux 两面 wire 出口：客户端未装配时报错而非 panic。
func TestControlClientsPullProbeDBTargetsNilSafe(t *testing.T) {
	var tcp *tcpControlClient
	_, err := tcp.PullProbeDBTargets(context.Background(), nil)
	assert.Error(t, err)

	var mux *muxControlClient
	_, err = mux.PullProbeDBTargets(context.Background(), nil)
	assert.Error(t, err)
}

// mockControlClientV9 默认出口：空版本 + 空正文 = 服务端不可派生。
func TestMockControlClientPullProbeDBTargetsDefaultEmpty(t *testing.T) {
	cc := &mockControlClientV9{}
	raw, err := cc.PullProbeDBTargets(context.Background(), nil)
	require.NoError(t, err)
	resp := &opsv1.ProbeDBTargetsConfigResponse{}
	require.NoError(t, proto.Unmarshal(raw, resp))
	assert.Empty(t, resp.GetVersion())
	assert.Empty(t, resp.GetTargetsJson())
}
