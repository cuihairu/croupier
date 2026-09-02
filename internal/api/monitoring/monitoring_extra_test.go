// 覆盖目标：sendGauge 各类型分支、platformCollector.Collect 的 DB 正常路径、
// checkDatabaseHealth 的 Exec 失败分支、collectRegistryStats 的 nil session 分支、
// summarizeOpsState 的 MQ.Lengths 分支。
package monitoring

import (
	"context"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
)

func TestSendGauge_Types(t *testing.T) {
	desc := prometheus.NewDesc("test_gauge", "help", nil, nil)

	cases := []struct {
		name     string
		value    interface{}
		emitting bool
	}{
		{"int", 3, true},
		{"int64", int64(4), true},
		{"float64", 5.5, true},
		{"unsupported string", "not-a-number", false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ch := make(chan prometheus.Metric, 1)
			sendGauge(ch, desc, tc.value)
			select {
			case m := <-ch:
				assert.True(t, tc.emitting, "should not emit for %v", tc.value)
				pb := &dto.Metric{}
				require.NoError(t, m.Write(pb))
				assert.NotNil(t, pb.GetGauge())
			default:
				assert.False(t, tc.emitting, "should emit for %v", tc.value)
			}
		})
	}
}

func TestPlatformCollector_Describe(t *testing.T) {
	collector := newPlatformCollector(&svc.ServiceContext{})

	descCh := make(chan *prometheus.Desc, 5)
	collector.Describe(descCh)
	close(descCh)
	count := 0
	for range descCh {
		count++
	}
	assert.Equal(t, 5, count)
}

func TestPlatformCollector_CollectWithHealthyDB(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		DB:            db,
		RegistryStore: registry.NewStore(),
		Config: config.Config{
			Database: config.DatabaseConfig{Driver: "sqlite"},
		},
	}
	collector := newPlatformCollector(svcCtx)

	ch := make(chan prometheus.Metric, 8)
	collector.Collect(ch)
	close(ch)

	dbUpDesc := prometheus.NewDesc("croupier_db_up", "Database connectivity (1=up 0=down)", nil, nil).String()
	latencyDesc := prometheus.NewDesc("croupier_db_latency_ms", "Database ping latency in milliseconds", nil, nil).String()

	foundUp, foundLatency := false, false
	for m := range ch {
		pb := &dto.Metric{}
		require.NoError(t, m.Write(pb))
		if pb.GetGauge() == nil {
			continue
		}
		descStr := m.Desc().String()
		if descStr == dbUpDesc {
			foundUp = pb.GetGauge().GetValue() == 1
		}
		if descStr == latencyDesc {
			foundLatency = true
		}
	}
	assert.True(t, foundUp, "db_up should be 1 with healthy DB")
	assert.True(t, foundLatency, "latency metric should be emitted")
}

func TestCheckDatabaseHealth_ExecError(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		DB: db,
		Config: config.Config{
			Database: config.DatabaseConfig{Driver: "sqlite"},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := checkDatabaseHealth(ctx, svcCtx)
	assert.False(t, result["ok"].(bool))
	assert.NotEmpty(t, result["error"])
	assert.Contains(t, result, "latencyMs")
}

func TestCollectRegistryStats_SkipsNilSession(t *testing.T) {
	store := registry.NewStore()
	// 直接向内部 map 注入 nil session，覆盖防御性 continue 分支
	store.Mu().Lock()
	store.AgentsUnsafe()["nil-agent"] = nil
	store.Mu().Unlock()

	stats, snapshots := collectRegistryStats(store)
	assert.True(t, stats["ok"].(bool))
	assert.Equal(t, 0, stats["agentsTotal"])
	assert.Empty(t, snapshots)
}

func TestSummarizeOpsState_MQLengths(t *testing.T) {
	// 使用独立临时目录，避免污染包内 ops_state.json 固件
	opsStore := svc.NewOpsStateStore(t.TempDir())
	_, err := opsStore.Update(func(state *svc.OpsState) {
		state.MQ.Lengths = map[string]int{"events": 3, "payments": 5}
	})
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{OpsStateStore: opsStore}
	result := summarizeOpsState(svcCtx)
	assert.True(t, result["ok"].(bool))
	lengths, ok := result["mqLengths"].(map[string]int)
	require.True(t, ok, "mqLengths should be populated, got %#v", result["mqLengths"])
	assert.Equal(t, 3, lengths["events"])
	assert.Equal(t, 5, lengths["payments"])
	assert.Equal(t, "redis", result["mqType"])
}

func TestUptimeSeconds_Monotonic(t *testing.T) {
	first := uptimeSeconds()
	time.Sleep(10 * time.Millisecond)
	assert.GreaterOrEqual(t, uptimeSeconds(), first)
}
