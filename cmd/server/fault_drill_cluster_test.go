// ARCH-005 集群故障钻机：双实例注入四类集群故障，逐例记录 拒绝 / 摘流 /
// 恢复 / 副作用次数。
//
//  1. 协调库断连：miniredis SetError 令全部协调命令失败 → 续租连续失败
//     跨过 TTL 窗口 → 就绪面租约不可用 → /readyz 503（摘流）；转发解析
//     owner 失败（拒绝）；协调库恢复后实例以同 ID 重加入（重连）→ 就绪
//     面恢复 → /readyz 200 且转发恢复（恢复）。
//  2. 互联占端口：预占实例互联端口 → 该实例互联监听绑定失败 → 半加入态
//     就绪面 interconnect=false → /readyz 503（摘流）；对端解析到该实例
//     的 owner 行并拨号被拒（拒绝，副作用 0 次）；端口释放后以同 ID
//     重启 → 互联绑定成功 → /readyz 200（恢复）。
//  3. owner 退出：owner 实例 Stop/Resign → 对端发现目录不再含 owner →
//     解析无存活 owner → ErrNoRoute（拒绝，副作用不增长）；owner 以同 ID
//     重启并重新 Claim → 对端重发现 → 转发成功（恢复，副作用恢复增长）。
//  4. 旧 epoch：owner 侧 ServeForwardHandler 收到 CallerEpoch 与本实例
//     当前 epoch 不一致 → NotOwner 拒绝且本地执行不增长（副作用 0 次）；
//     携带正确 epoch 的帧 → 本地执行（恢复，副作用增长）。
//
// 边界诚实：本钻机在真实 startCluster 装配下运行（成员注册/续租/发现/
// 互联 TCP 监听/转发 handler 均为生产代码），Agent 会话层以
// fakeLocalDispatcher 计数——本钻机只断言平台收敛行为（拒绝/摘流/恢复/
// 副作用次数），不断言游戏 exactly-once（与 ARCH-018 同边界）。
package main

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/cuihairu/croupier/internal/api/monitoring"
	"github.com/cuihairu/croupier/internal/cluster"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/svc"
	internaltcp "github.com/cuihairu/croupier/internal/transport/tcp"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/require"
)

// drillClusterInst 是 startCluster 启动的一个集群实例句柄（幂等停止）。
type drillClusterInst struct {
	id     string
	addr   string
	ctx    context.Context
	cancel context.CancelFunc
	lc     *cluster.Lifecycle
	srv    *internaltcp.Server
	svcCtx *svc.ServiceContext

	stopOnce sync.Once
}

// stop 幂等停止：取消循环 + 成员 Resign + 关互联监听。可安全多次调用。
func (d *drillClusterInst) stop() {
	if d == nil {
		return
	}
	d.stopOnce.Do(func() {
		// 先 Resign 再取消循环：Resign 需协调面客户端仍在（生产 root.go 的
		// 停机顺序是先 clusterLifecycle.Stop 再 rootCancel，此处保持一致），
		// 否则 Redis 客户端被 ctx.Done goroutine 提前关闭，成员 key 残留，
		// 对端发现目录仍含本实例（owner 退出用例 stale peer）。
		if d.lc != nil {
			stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			d.lc.Stop(stopCtx)
		}
		if d.srv != nil {
			_ = d.srv.Close()
		}
		d.cancel()
	})
}

// drillClusterSvcCtx 构造可承载协调面 + readyz 的完整服务上下文。
func drillClusterSvcCtx(t *testing.T) *svc.ServiceContext {
	t.Helper()
	sc := newClusterSvcCtx(t)
	sc.OpsStateStore = svc.NewOpsStateStore(t.TempDir())
	return sc
}

// startDrillClusterAt 在指定 advertise 地址启动一个 redis 协调面的集群实例。
// fake 非 nil 时该实例的 owner 侧本地执行用计数替身（仅该实例生效——
// 注入点在 startCluster 同步段读取并随即清除，不泄漏到后续实例）。
func startDrillClusterAt(t *testing.T, mr *miniredis.Miniredis, id, addr string, fake *fakeLocalDispatcher) *drillClusterInst {
	t.Helper()
	sc := drillClusterSvcCtx(t)
	c := &config.Config{}
	c.Cluster = config.ClusterConfig{
		Enabled:           true,
		InstanceID:        id,
		Store:             "redis",
		RedisAddr:         mr.Addr(),
		AdvertiseAddr:     addr,
		HeartbeatInterval: "50ms",
		LeaseTTL:          "2s",
		PeerPollInterval:  "100ms",
	}
	ctx, cancel := context.WithCancel(context.Background())
	if fake != nil {
		clusterInvokerOverride = localInvoker{dispatcher: fake}
		defer func() { clusterInvokerOverride = nil }()
	}
	lc, srv := startCluster(ctx, c, sc)
	require.NotNil(t, lc, "instance %s must join", id)
	inst := &drillClusterInst{
		id: id, addr: addr, ctx: ctx, cancel: cancel,
		lc: lc, srv: srv, svcCtx: sc,
	}
	t.Cleanup(inst.stop)
	return inst
}

func freeDrillAddr(t *testing.T) string {
	t.Helper()
	return "127.0.0.1:" + strconv.Itoa(drillFreePort(t))
}

func readyStatus(t *testing.T, sc *svc.ServiceContext) string {
	t.Helper()
	resp, err := monitoring.NewService(sc).Ready(context.Background(), &monitoring.ReadyRequest{})
	require.NoError(t, err)
	return resp.Status
}

func waitForPeer(t *testing.T, inst *drillClusterInst, peerID string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, p := range inst.lc.Mesh().Peers() {
			if p.InstanceID == peerID {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("instance %s never discovered peer %s", inst.id, peerID)
}

func waitPeerGone(t *testing.T, inst *drillClusterInst, peerID string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		gone := true
		for _, p := range inst.lc.Mesh().Peers() {
			if p.InstanceID == peerID {
				gone = false
				break
			}
		}
		if gone {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("peer %s still present in %s peers after exit", peerID, inst.id)
}

// handshakedDrillCtx 构造已完成互联 hello 握手的上下文（ARCH-003 门禁）。
func handshakedDrillCtx() context.Context {
	ctx := internaltcp.WithConnectionSession(context.Background(), internaltcp.NewConnectionSession())
	internaltcp.ConnectionSessionFromContext(ctx).Set(internaltcp.ConnectionHandshakeKey, true)
	return ctx
}

// --- 故障 1：协调库断连 → 租约摘流 → 重连恢复 ---

func TestDrillCluster_CoordinationDisconnect_DrainRecover(t *testing.T) {
	mr := miniredis.RunT(t)
	fake := &fakeLocalDispatcher{}
	a := startDrillClusterAt(t, mr, "inst-coord-a", freeDrillAddr(t), fake)
	b := startDrillClusterAt(t, mr, "inst-coord-b", freeDrillAddr(t), nil)

	// 基线：两实例就绪面全绿。
	require.Equal(t, "ok", readyStatus(t, a.svcCtx))
	require.Equal(t, "ok", readyStatus(t, b.svcCtx))

	// 注入：协调库命令全部失败（模拟协调库断连）。
	mr.SetError("LOADING coordination store down")

	// 摘流：续租连续失败跨过 TTL 窗口 → 租约不可用 → 两实例 not_ready。
	require.Eventually(t, func() bool {
		return !a.svcCtx.ClusterReadiness.Snapshot().LeaseHealthy &&
			!b.svcCtx.ClusterReadiness.Snapshot().LeaseHealthy
	}, 10*time.Second, 20*time.Millisecond, "lease must drain after coordination disconnect")

	require.Equal(t, "not_ready", readyStatus(t, a.svcCtx), "coord-down instance A must be drained")
	require.Equal(t, "not_ready", readyStatus(t, b.svcCtx), "coord-down instance B must be drained")

	// 拒绝：协调面不可达 → 转发解析 owner 失败（请求被拒，非路由成功）。
	_, err := a.lc.Mesh().Forward(context.Background(), "any-agent", &cluster.ForwardedInvoke{FunctionID: "demo.fn"})
	require.Error(t, err, "forward must be rejected while coordination is down")

	// 恢复：协调库恢复但租约已过期——Redis 成员 key 已 TTL 失效，续租
	// 返回 lease lost，运行期不会自动重新注册。诚实记录：唯一恢复路径是
	// 以同 ID 重启（重连 → 重新注册拿新 epoch）。
	mr.SetError("")
	a.stop()
	b.stop()
	a2 := startDrillClusterAt(t, mr, "inst-coord-a", freeDrillAddr(t), fake)
	b2 := startDrillClusterAt(t, mr, "inst-coord-b", freeDrillAddr(t), nil)
	require.Equal(t, "ok", readyStatus(t, a2.svcCtx), "reconnected instance A must be ready")
	require.Equal(t, "ok", readyStatus(t, b2.svcCtx), "reconnected instance B must be ready")

	// 转发恢复：owner 重连接并重新 Claim → 对端重发现 → 成功，副作用增长。
	require.NoError(t, a2.svcCtx.Cluster.Resolver.ClaimOwner(
		context.Background(), "agent-recover", "demo", "prod", a2.id, a2.lc.Epoch(), nil))
	waitForPeer(t, b2, a2.id)
	res, err := b2.svcCtx.Cluster.Mesh.Forward(context.Background(), "agent-recover", &cluster.ForwardedInvoke{
		FunctionID: "demo.fn", Payload: []byte(`{}`),
	})
	require.NoError(t, err)
	require.True(t, res.OK)
	require.Len(t, fake.invokeAgentIDs, 1, "forward must resume after reconnect")
}

// --- 故障 2：互联占端口 → 半加入态摘流 → 恢复 ---

func TestDrillCluster_InterconnectPortOccupied_DrainRecover(t *testing.T) {
	mr := miniredis.RunT(t)
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	occupied := busy.Addr().String()

	fake := &fakeLocalDispatcher{}
	a := startDrillClusterAt(t, mr, "inst-ic-a", occupied, fake)
	b := startDrillClusterAt(t, mr, "inst-ic-b", freeDrillAddr(t), nil)

	// 摘流：A 的互联端口被占 → 监听绑定失败 → 半加入态（已加入但互联不可用）。
	require.NotNil(t, a.lc, "member still registers even when interconnect port occupied")
	require.Nil(t, a.srv, "interconnect listener must fail to bind on occupied port")
	snap := a.svcCtx.ClusterReadiness.Snapshot()
	require.True(t, snap.Joined)
	require.False(t, snap.InterconnectUp)
	require.False(t, snap.Healthy)
	require.Equal(t, "not_ready", readyStatus(t, a.svcCtx), "port-occupied instance A must be drained")
	require.Equal(t, "ok", readyStatus(t, b.svcCtx), "healthy instance B unaffected")

	// 拒绝：对端 B 解析到 A 的 owner 行，拨号 A 的互联地址——A 监听从未
	// 绑定（端口被占），释放占位后拨号被拒；副作用不得发生。
	require.NoError(t, a.svcCtx.Cluster.Resolver.ClaimOwner(
		context.Background(), "agent-ic", "demo", "prod", a.id, a.lc.Epoch(), nil))
	waitForPeer(t, b, a.id)
	require.NoError(t, busy.Close())
	_, err = b.svcCtx.Cluster.Mesh.Forward(context.Background(), "agent-ic", &cluster.ForwardedInvoke{
		FunctionID: "demo.fn", Payload: []byte(`{}`),
	})
	require.Error(t, err, "forward to drained owner must be rejected")
	require.Empty(t, fake.invokeAgentIDs, "drained owner must not execute side effects")

	// 恢复：端口释放后以同 ID 重启 → 互联监听绑定成功 → readyz ok。
	a.stop()
	a2 := startDrillClusterAt(t, mr, "inst-ic-a", occupied, fake)
	require.NotNil(t, a2.srv, "restart on freed port must bind interconnect")
	require.True(t, a2.svcCtx.ClusterReadiness.Snapshot().InterconnectUp)
	require.Equal(t, "ok", readyStatus(t, a2.svcCtx), "restarted instance must be ready")
}

// --- 故障 3：owner 退出 → 无路由拒绝 → 重连恢复 ---

func TestDrillCluster_OwnerExit_NoRouteThenRecover(t *testing.T) {
	mr := miniredis.RunT(t)
	fake := &fakeLocalDispatcher{}
	a := startDrillClusterAt(t, mr, "inst-own-a", freeDrillAddr(t), fake)
	b := startDrillClusterAt(t, mr, "inst-own-b", freeDrillAddr(t), nil)

	require.NoError(t, a.svcCtx.Cluster.Resolver.ClaimOwner(
		context.Background(), "agent-own", "demo", "prod", a.id, a.lc.Epoch(), nil))
	waitForPeer(t, b, a.id)

	// 基线：owner 存活时转发成功，owner 侧本地执行 1 次。
	res, err := b.svcCtx.Cluster.Mesh.Forward(context.Background(), "agent-own", &cluster.ForwardedInvoke{
		FunctionID: "demo.fn", Payload: []byte(`{}`),
	})
	require.NoError(t, err)
	require.True(t, res.OK)
	require.Len(t, fake.invokeAgentIDs, 1, "owner-side invoke must execute once")

	// owner 退出：Stop 内 Resign 清掉成员记录。
	a.stop()
	waitPeerGone(t, b, a.id)

	// 拒绝：目录无存活 owner → ErrNoRoute；副作用计数不增长。
	_, err = b.svcCtx.Cluster.Mesh.Forward(context.Background(), "agent-own", &cluster.ForwardedInvoke{
		FunctionID: "demo.fn", Payload: []byte(`{}`),
	})
	require.ErrorIs(t, err, cluster.ErrNoRoute, "forward after owner exit must be no-route")
	require.Len(t, fake.invokeAgentIDs, 1, "no side effect while owner is dead")

	// 恢复：owner 以同 ID 重启（epoch 递增）并重新 Claim → 对端重发现 → 成功。
	a2 := startDrillClusterAt(t, mr, "inst-own-a", freeDrillAddr(t), fake)
	require.NoError(t, a2.svcCtx.Cluster.Resolver.ClaimOwner(
		context.Background(), "agent-own", "demo", "prod", a2.id, a2.lc.Epoch(), nil))
	waitForPeer(t, b, a2.id)
	res, err = b.svcCtx.Cluster.Mesh.Forward(context.Background(), "agent-own", &cluster.ForwardedInvoke{
		FunctionID: "demo.fn", Payload: []byte(`{}`),
	})
	require.NoError(t, err)
	require.True(t, res.OK)
	require.Len(t, fake.invokeAgentIDs, 2, "owner-side invoke must resume after reconnect")
}

// --- 故障 4：旧 epoch → fencing 拒绝（副作用 0）→ 恢复 ---

func TestDrillCluster_StaleEpoch_FencingRejectsThenRecovers(t *testing.T) {
	mr := miniredis.RunT(t)
	fake := &fakeLocalDispatcher{}
	a := startDrillClusterAt(t, mr, "inst-stale-a", freeDrillAddr(t), fake)

	handler := newInterconnectHandler(a.lc, localInvoker{dispatcher: fake})
	current := a.lc.Epoch()

	// 旧 epoch（僵尸 owner 视角）：CallerEpoch 落后本实例 → NotOwner 拒绝，
	// 且本地执行（副作用）不得发生。
	staleBody, err := json.Marshal(cluster.ForwardedInvoke{
		Forwarded: true, CallerEpoch: current - 1,
		AgentID: "agent-stale", FunctionID: "demo.fn", Payload: []byte(`{}`),
	})
	require.NoError(t, err)
	fwdBody, err := handler.Handle(handshakedDrillCtx(), protocol.MsgForwardInvokeReq, 1, staleBody)
	require.NoError(t, err)
	var staleRes cluster.ForwardedResult
	require.NoError(t, json.Unmarshal(fwdBody, &staleRes))
	require.True(t, staleRes.NotOwner, "stale epoch must be fenced out")
	require.False(t, staleRes.OK)
	require.Empty(t, fake.invokeAgentIDs, "fenced request must not reach local invoke")

	// 恢复：携带正确 epoch → 本地执行成功，副作用计数增长。
	okBody, err := json.Marshal(cluster.ForwardedInvoke{
		Forwarded: true, CallerEpoch: current,
		AgentID: "agent-stale", FunctionID: "demo.fn", Payload: []byte(`{}`),
	})
	require.NoError(t, err)
	fwdBody, err = handler.Handle(handshakedDrillCtx(), protocol.MsgForwardInvokeReq, 2, okBody)
	require.NoError(t, err)
	var okRes cluster.ForwardedResult
	require.NoError(t, json.Unmarshal(fwdBody, &okRes))
	require.True(t, okRes.OK)
	require.Len(t, fake.invokeAgentIDs, 1, "correct epoch must reach local invoke")
}
