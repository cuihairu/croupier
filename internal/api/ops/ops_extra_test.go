// 覆盖目标：opsAgentsList 集群分支（归属表合并/scope 过滤/读取失败回落）、
// opsMetrics 的 MetricsStore 聚合路径、opsConfig 的 env 回落分支。
package ops

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/cluster"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpsAgentsList_ClusterOwners_MergeAndScopeFilter(t *testing.T) {
	now := time.Now()
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "a-local", GameID: "g1", Env: "prod", Addr: "h:1", Version: "v1",
		Labels:    map[string]string{"k": "v"},
		Functions: map[string]registry.FunctionMeta{"fn.a": {}},
		LastSeen:  now, ExpireAt: now.Add(time.Minute),
	}))

	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Cluster: &svc.ClusterRuntime{
			ListAgentOwners: func(context.Context) ([]cluster.AgentOwnerRecord, error) {
				return []cluster.AgentOwnerRecord{
					{AgentID: "a-local", InstanceID: "self", GameID: "g1", Env: "prod", LastSeenAt: now},
					{AgentID: "a-remote", InstanceID: "peer", GameID: "g2", Env: "prod", LastSeenAt: now},
				}, nil
			},
		},
	}

	// 全量：local 合并 connected 信息，remote 标 ownerInstance
	resp, err := opsAgentsList(context.Background(), svcCtx, &OpsAgentsListRequest{})
	require.NoError(t, err)
	byID := map[string]OpsAgentInfo{}
	for _, a := range resp.Agents {
		byID[a.AgentID] = a
	}
	require.Len(t, byID, 2)
	assert.True(t, byID["a-local"].Connected)
	assert.Equal(t, "v1", byID["a-local"].Version)
	assert.False(t, byID["a-remote"].Connected)
	assert.Equal(t, "peer", byID["a-remote"].OwnerInstance)

	// scope 过滤：只看 g2
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "g2"})
	resp2, err := opsAgentsList(ctx, svcCtx, &OpsAgentsListRequest{})
	require.NoError(t, err)
	require.Len(t, resp2.Agents, 1)
	assert.Equal(t, "a-remote", resp2.Agents[0].AgentID)
}

func TestOpsAgentsList_ClusterOwnersError_FallsBackToLocal(t *testing.T) {
	now := time.Now()
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "a1", GameID: "g1", Env: "prod", Addr: "h:1",
		Labels: map[string]string{}, Functions: map[string]registry.FunctionMeta{},
		LastSeen: now, ExpireAt: now.Add(time.Minute),
	}))
	svcCtx := &svc.ServiceContext{
		RegistryStore: store,
		Cluster: &svc.ClusterRuntime{
			ListAgentOwners: func(context.Context) ([]cluster.AgentOwnerRecord, error) {
				return nil, errors.New("owner table down")
			},
		},
	}

	resp, err := opsAgentsList(context.Background(), svcCtx, &OpsAgentsListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Agents, 1)
	assert.Equal(t, "a1", resp.Agents[0].AgentID)
}

func TestOpsMetrics_AggregatesLatestReports(t *testing.T) {
	now := time.Now()
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "m1", GameID: "g1", Env: "prod", Addr: "h:1",
		Labels: map[string]string{}, Functions: map[string]registry.FunctionMeta{},
		LastSeen: now, ExpireAt: now.Add(time.Minute),
	}))
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "m2", GameID: "g1", Env: "dev", Addr: "h:2",
		Labels: map[string]string{}, Functions: map[string]registry.FunctionMeta{},
		LastSeen: now, ExpireAt: now.Add(time.Minute),
	}))

	metricsStore := registry.NewMetricsStore()
	metricsStore.Add("m1", &opsv1.MetricsReport{
		AgentId: "m1",
		Cpu:     &opsv1.CpuMetrics{UsagePercent: 42.5, Cores: 4, Load_1M: 1.2, Load_5M: 1.1, Load_15M: 1.0, PerCore: []float64{40, 45}},
		Memory:  &opsv1.MemoryMetrics{TotalBytes: 100, UsedBytes: 50, AvailableBytes: 50, UsagePercent: 50, SwapTotal: 1, SwapUsed: 0},
		Disks:   []*opsv1.DiskMetrics{{MountPoint: "/", Device: "/dev/sda1", FsType: "ext4", TotalBytes: 10, UsedBytes: 5, AvailableBytes: 5, UsagePercent: 50}},
		Networks: []*opsv1.NetworkMetrics{
			{Interface: "eth0", BytesSent: 1, BytesRecv: 2, PacketsSent: 3, PacketsRecv: 4},
		},
	})

	svcCtx := &svc.ServiceContext{RegistryStore: store, MetricsStore: metricsStore}

	// env 过滤只留 m1；m2 无上报也会被跳过
	resp, err := opsMetrics(context.Background(), svcCtx, &OpsMetricsRequest{Env: "prod"})
	require.NoError(t, err)
	require.Len(t, resp.Metrics, 1)
	m := resp.Metrics[0]
	assert.Equal(t, "m1", m.AgentID)
	assert.Equal(t, 42.5, m.CPU.UsagePercent)
	assert.Len(t, m.Disks, 1)
	assert.Len(t, m.Networks, 1)

	// nil 各组件的空返回分支
	empty, err := opsMetrics(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.Empty(t, empty.Metrics)
	empty2, err := opsMetrics(context.Background(), &svc.ServiceContext{RegistryStore: store}, nil)
	require.NoError(t, err)
	assert.Empty(t, empty2.Metrics)
}

func TestOpsConfig_EnvFallback(t *testing.T) {
	t.Setenv("CROUPIER_ALERTMANAGER_URL", "http://am.example")
	t.Setenv("CROUPIER_GRAFANA_EXPLORE_URL", "http://grafana.example")
	t.Setenv("CROUPIER_JAEGER_URL", "http://jaeger.example")

	resp, err := opsConfig(context.Background(), &svc.ServiceContext{}, &OpsConfigRequest{})
	require.NoError(t, err)
	assert.Equal(t, "http://am.example", resp.AlertmanagerURL)
	assert.Equal(t, "http://grafana.example", resp.GrafanaExploreURL)
	assert.Equal(t, "http://jaeger.example", resp.JaegerURL)
}

func TestOpsConfig_NilSvcCtx_EnvOnly(t *testing.T) {
	t.Setenv("CROUPIER_JAEGER_URL", "http://j2")
	resp, err := opsConfig(context.Background(), nil, &OpsConfigRequest{})
	require.NoError(t, err)
	assert.Equal(t, "http://j2", resp.JaegerURL)
}
