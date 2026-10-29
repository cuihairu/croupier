package main

import (
	"context"
	"net"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ARCH-004：cluster.enabled 启动期降级路径必须把降级事实写进就绪面，
// 供 /readyz 判定与 /healthz 集群组件观测。

// db 存储但 DB 缺失 → 降级事实入就绪面（未加入 + 降级原因）。
func TestStartCluster_ReadinessDegradeNoCoordination(t *testing.T) {
	svcCtx := newClusterSvcCtx(t)
	svcCtx.DB = nil

	c := &config.Config{}
	c.Cluster = config.ClusterConfig{Enabled: true, InstanceID: "server-no-db", Store: "db"}

	lc, srv := startCluster(context.Background(), c, svcCtx)
	assert.Nil(t, lc)
	assert.Nil(t, srv)
	require.NotNil(t, svcCtx.ClusterReadiness)
	snap := svcCtx.ClusterReadiness.Snapshot()
	assert.False(t, snap.Joined)
	assert.Contains(t, snap.Degraded, "db unavailable")
}

// advertiseAddr 为空 → cluster.Start 拒绝 → 降级事实入就绪面。
func TestStartCluster_ReadinessEmptyAdvertiseAddr(t *testing.T) {
	svcCtx := newClusterSvcCtx(t)
	c := &config.Config{}
	c.Cluster = config.ClusterConfig{Enabled: true, InstanceID: "server-no-addr", Store: "db"}

	lc, srv := startCluster(context.Background(), c, svcCtx)
	assert.Nil(t, lc)
	assert.Nil(t, srv)
	require.NotNil(t, svcCtx.ClusterReadiness)
	snap := svcCtx.ClusterReadiness.Snapshot()
	assert.False(t, snap.Joined)
	assert.Contains(t, snap.Degraded, "advertiseAddr")
}

// 互联监听端口被占 → 成员已加入但互联不可用（半加入态，/readyz 503）。
func TestStartCluster_ReadinessInterconnectDown(t *testing.T) {
	svcCtx := newClusterSvcCtx(t)
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = busy.Close() })

	c := &config.Config{}
	c.Cluster = config.ClusterConfig{
		Enabled:       true,
		InstanceID:    "server-ic-down",
		Store:         "db",
		AdvertiseAddr: busy.Addr().String(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	lc, srv := startCluster(ctx, c, svcCtx)
	require.NotNil(t, lc)
	assert.Nil(t, srv)
	require.NotNil(t, svcCtx.ClusterReadiness)
	snap := svcCtx.ClusterReadiness.Snapshot()
	assert.True(t, snap.Joined, "member registered")
	assert.False(t, snap.InterconnectUp, "listener failed to bind")
	assert.Contains(t, snap.InterconnectErr, "in use")
	assert.False(t, snap.Healthy)

	stopCluster(t, lc, nil)
}

// db 存储正常路径：成员注册 + 互联监听绑定 → 就绪面全绿。
func TestStartCluster_ReadinessHealthy(t *testing.T) {
	svcCtx := newClusterSvcCtx(t)
	c := &config.Config{}
	c.Cluster = config.ClusterConfig{
		Enabled:       true,
		InstanceID:    "server-ready",
		Store:         "db",
		AdvertiseAddr: "127.0.0.1:0",
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	lc, srv := startCluster(ctx, c, svcCtx)
	require.NotNil(t, lc)
	require.NotNil(t, srv)
	require.NotNil(t, svcCtx.ClusterReadiness)

	snap := svcCtx.ClusterReadiness.Snapshot()
	assert.True(t, snap.Joined)
	assert.True(t, snap.InterconnectUp)
	assert.True(t, snap.Healthy)

	stopCluster(t, lc, srv)
}

// 租约窗口与心跳/TTL 一致性：就绪面以 startCluster 归一化后的配置构造。
func TestStartCluster_ReadinessWindowFromConfig(t *testing.T) {
	svcCtx := newClusterSvcCtx(t)
	c := &config.Config{}
	c.Cluster = config.ClusterConfig{
		Enabled:           true,
		InstanceID:        "server-window",
		Store:             "db",
		AdvertiseAddr:     "127.0.0.1:0",
		HeartbeatInterval: "1s",
		LeaseTTL:          "2s",
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	lc, srv := startCluster(ctx, c, svcCtx)
	require.NotNil(t, lc)
	require.NotNil(t, srv)

	// 归一化配置 1s/2s：连续失败 2 次（2s >= 2s 窗口）判租约不可用。
	svcCtx.ClusterReadiness.RenewFail(assert.AnError)
	assert.True(t, svcCtx.ClusterReadiness.Snapshot().LeaseHealthy)
	svcCtx.ClusterReadiness.RenewFail(assert.AnError)
	assert.False(t, svcCtx.ClusterReadiness.Snapshot().LeaseHealthy)

	stopCluster(t, lc, srv)
}
