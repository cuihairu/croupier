// 补齐 ops 包剩余可覆盖分支：registry agents map 含 nil 会话时各列表函数
// 的防御性 continue（AgentService.List / opsAgentsList / opsMetrics /
// opsFunctions / opsServices / listNodes）。
package ops

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newGhostNilSessionStore 构造一个包含 nil 会话条目的 registry store，
// 用于触发各列表函数的 nil 守卫（调用方必须持锁，此处按约定加写锁注入）。
func newGhostNilSessionStore(t *testing.T) *registry.Store {
	t.Helper()
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID:   "live-agent",
		GameID:    "g1",
		Env:       "prod",
		Addr:      "h:1",
		Labels:    map[string]string{},
		Functions: map[string]registry.FunctionMeta{},
		LastSeen:  time.Now(),
		ExpireAt:  time.Now().Add(time.Minute),
	}))
	store.Mu().Lock()
	store.AgentsUnsafe()["ghost-nil"] = nil
	store.Mu().Unlock()
	return store
}

func TestAgentServiceListSkipsNilSession(t *testing.T) {
	t.Parallel()

	store := newGhostNilSessionStore(t)
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	agents, err := NewAgentService(svcCtx).List(context.Background(), "", "", "")
	require.NoError(t, err)
	require.Len(t, agents, 1)
	assert.Equal(t, "live-agent", agents[0].AgentID)
}

func TestOpsAgentsListSkipsNilSession(t *testing.T) {
	t.Parallel()

	store := newGhostNilSessionStore(t)
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	resp, err := opsAgentsList(context.Background(), svcCtx, &OpsAgentsListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Agents, 1)
	assert.Equal(t, "live-agent", resp.Agents[0].AgentID)
}

func TestOpsMetricsSkipsNilSession(t *testing.T) {
	t.Parallel()

	store := newGhostNilSessionStore(t)
	svcCtx := &svc.ServiceContext{RegistryStore: store, MetricsStore: registry.NewMetricsStore()}
	resp, err := opsMetrics(context.Background(), svcCtx, &OpsMetricsRequest{})
	require.NoError(t, err)
	// nil 会话被跳过，不产生指标行；不因 nil panic 即为通过
	assert.NotNil(t, resp)
}

func TestOpsFunctionsSkipsNilSession(t *testing.T) {
	t.Parallel()

	store := newGhostNilSessionStore(t)
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	resp, err := opsFunctions(context.Background(), svcCtx, &OpsFunctionsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Functions, 0)
}

func TestOpsServicesSkipsNilSession(t *testing.T) {
	t.Parallel()

	store := newGhostNilSessionStore(t)
	svcCtx := &svc.ServiceContext{RegistryStore: store, StartTime: time.Now()}
	resp, err := opsServices(context.Background(), svcCtx, &OpsServicesRequest{})
	require.NoError(t, err)
	// 1 server 自身 + 1 个活跃 agent（nil 会话被跳过）
	require.Len(t, resp.Services, 2)
}

func TestListNodesSkipsNilSession(t *testing.T) {
	t.Parallel()

	store := newGhostNilSessionStore(t)
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	nodes := listNodes(context.Background(), svcCtx, "", "", "")
	require.NotEmpty(t, nodes)
	for _, n := range nodes {
		assert.NotEmpty(t, n.Id)
	}
}
