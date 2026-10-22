package function

// #34：注册警告过滤下拉聚合（GET /api/v1/functions/warnings/filter-options）。

import (
	"context"
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedRegistrationWarning(t *testing.T, store *reg.Store, key, agentID, functionID, code string) {
	t.Helper()
	store.UpsertRegistrationWarning(context.Background(), reg.FunctionRegistrationWarning{
		Key: key, GameID: "demo-game", Env: "development",
		AgentID: agentID, FunctionID: functionID, Code: code,
		Message: "test warning", Count: 1,
	})
}

func TestFunctionWarningFilterOptions_AggregatesDistinctFunctionsAndAgents(t *testing.T) {
	store := reg.NewStore()
	seedRegistrationWarning(t, store, "w-1", "agent-1", "fn.a", "invalid_version")
	seedRegistrationWarning(t, store, "w-2", "agent-1", "fn.b", "invalid_version")
	seedRegistrationWarning(t, store, "w-3", "agent-2", "fn.b", "stale_contract")
	// scope 外不进聚合
	store.UpsertRegistrationWarning(context.Background(), reg.FunctionRegistrationWarning{
		Key: "w-out", GameID: "other", Env: "x",
		AgentID: "agent-9", FunctionID: "fn.z", Code: "c", Count: 1,
	})

	svcCtx := &svc.ServiceContext{RegistryStore: store}
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
	resp, err := functionWarningFilterOptions(ctx, svcCtx)
	require.NoError(t, err)

	// 条数降序、同数按 value 字典序
	require.Len(t, resp.Functions, 2)
	assert.Equal(t, "fn.b", resp.Functions[0].Value)
	assert.Equal(t, int64(2), resp.Functions[0].Count)
	assert.Equal(t, "fn.a", resp.Functions[1].Value)

	require.Len(t, resp.Agents, 2)
	assert.Equal(t, "agent-1", resp.Agents[0].Value)
	assert.Equal(t, int64(2), resp.Agents[0].Count)
	assert.Equal(t, "agent-2", resp.Agents[1].Value)
}

func TestFunctionWarningFilterOptions_NilStoreReturnsEmpty(t *testing.T) {
	resp, err := functionWarningFilterOptions(context.Background(), &svc.ServiceContext{})
	require.NoError(t, err)
	assert.Empty(t, resp.Functions)
	assert.Empty(t, resp.Agents)
}

func TestFunctionWarningFilterOptions_HandlerOK(t *testing.T) {
	store := reg.NewStore()
	seedRegistrationWarning(t, store, "w-1", "agent-1", "fn.a", "invalid_version")
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	handler := NewHandler(&Service{svcCtx: svcCtx})
	assert.NotNil(t, handler)
	resp, err := (&Service{svcCtx: svcCtx}).FunctionWarningFilterOptions(
		svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"}))
	require.NoError(t, err)
	require.Len(t, resp.Functions, 1)
	assert.Equal(t, "fn.a", resp.Functions[0].Value)
}
