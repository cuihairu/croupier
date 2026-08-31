package ops

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/lbstats"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 覆盖 GetClusterInfo（单实例/集群 nil 分支）与 LBStatsQuery（未配置/正常路径）。

func TestGetClusterInfo_StandaloneAndNil(t *testing.T) {
	s := NewService(&svc.ServiceContext{})

	// 单实例（Cluster=nil）：standalone 条目
	resp, err := s.GetClusterInfo(context.Background())
	require.NoError(t, err)
	assert.False(t, resp.Enabled)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "standalone", resp.Items[0].InstanceID)
	assert.True(t, resp.Items[0].Self)
	assert.Equal(t, 1, resp.Total)
	assert.Equal(t, 1, resp.AliveCount)

	// nil service 保险分支
	var nilSvc *Service
	_, err = nilSvc.GetClusterInfo(context.Background())
	assert.Error(t, err)
}

func TestGetClusterInfo_ClusterEnabled(t *testing.T) {
	s := NewService(&svc.ServiceContext{
		Cluster: &svc.ClusterRuntime{
			InstanceID: "inst-1",
			LBStats:    lbstats.NewLBStatsService("http://prom:9090"),
		},
	})
	resp, err := s.GetClusterInfo(context.Background())
	require.NoError(t, err)
	assert.True(t, resp.Enabled)
	assert.Equal(t, "inst-1", resp.Self)
	require.NotNil(t, resp.LbStats)
	assert.True(t, resp.LbStats.Enabled)
	assert.Equal(t, "/api/v1/ops/cluster/lb-stats", resp.LbStats.QueryURL)
}

func TestLBStatsQuery_Paths(t *testing.T) {
	// 未配置：报错
	s := NewService(&svc.ServiceContext{})
	_, err := s.LBStatsQuery(context.Background(), "haproxy_up")
	assert.Error(t, err)

	// 已配置 mock Prometheus：正常返回 + 白名单拒绝
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer prom.Close()
	s2 := NewService(&svc.ServiceContext{
		Cluster: &svc.ClusterRuntime{LBStats: lbstats.NewLBStatsService(prom.URL)},
	})
	res, err := s2.LBStatsQuery(context.Background(), "haproxy_backend_current_sessions")
	require.NoError(t, err)
	assert.NotNil(t, res)

	_, err = s2.LBStatsQuery(context.Background(), "go_gc_duration_seconds")
	assert.Error(t, err)
}
