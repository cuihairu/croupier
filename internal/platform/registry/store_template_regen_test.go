package registry

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// regenCountingService 是模板收口语义的观测 fake：契约物化全部 no-op，
// 仅统计 RegenerateContractTemplates 调用次数。
type regenCountingService struct {
	regenCalls int
	regenErr   error
	regenScope seenScope
}

func (r *regenCountingService) RebuildContractFromFunctionMeta(context.Context, string, string, string, spec.FunctionContractInput) error {
	return nil
}

func (r *regenCountingService) RemoveFunctionContract(context.Context, string, string, string) (string, error) {
	return "", nil
}

func (r *regenCountingService) RebuildResourceCapability(context.Context, string, string, string) error {
	return nil
}

func (r *regenCountingService) RebuildProposalsForResource(context.Context, string, string, string) error {
	return nil
}

func (r *regenCountingService) RebuildProposalForFunction(context.Context, string, string, string) error {
	return nil
}

func (r *regenCountingService) RegenerateContractTemplates(ctx context.Context, gameID, env string) error {
	r.regenCalls++
	if seen, ok := ctx.Value(registryTestScopeKey{}).(seenScope); ok {
		r.regenScope = seen
	}
	return r.regenErr
}

func regenTestSession(functions map[string]FunctionMeta) *AgentSession {
	return &AgentSession{
		AgentID:   "agent-regen",
		GameID:    "demo-game",
		Env:       "development",
		Functions: functions,
	}
}

// T2 收口（注册边界）：带函数快照的注册在写入成功后单次触发模板重建；
// 函数集未变的重注册（心跳/重连风暴）不重复触发。
func TestUpsertAgentRegensTemplatesOncePerSnapshotChange(t *testing.T) {
	store := NewStore()
	counter := &regenCountingService{}
	store.SetContractService(counter)
	store.SetScopeContextResolver(func(gameID, env string) context.Context {
		return context.WithValue(context.Background(), registryTestScopeKey{}, seenScope{gameID: gameID, env: env})
	})

	require.NoError(t, store.UpsertAgent(regenTestSession(map[string]FunctionMeta{
		"player.query": {Enabled: true, Version: "1", Resource: "player", Capability: "collection_query", Execution: "sync"},
	})))
	assert.Equal(t, 1, counter.regenCalls, "首次注册应触发一次模板收口")
	assert.Equal(t, "demo-game", counter.regenScope.gameID)
	assert.Equal(t, "development", counter.regenScope.env)

	// 函数集与元数据完全一致的重注册：不重复触发。
	require.NoError(t, store.UpsertAgent(regenTestSession(map[string]FunctionMeta{
		"player.query": {Enabled: true, Version: "1", Resource: "player", Capability: "collection_query", Execution: "sync"},
	})))
	assert.Equal(t, 1, counter.regenCalls, "快照未变的重注册不应触发收口")

	// 元数据变化（版本升级）：再次触发。
	require.NoError(t, store.UpsertAgent(regenTestSession(map[string]FunctionMeta{
		"player.query": {Enabled: true, Version: "2", Resource: "player", Capability: "collection_query", Execution: "sync"},
	})))
	assert.Equal(t, 2, counter.regenCalls, "快照变化的重注册应再次触发收口")

	// 函数移除：触发。
	require.NoError(t, store.UpsertAgent(regenTestSession(map[string]FunctionMeta{})))
	assert.Equal(t, 3, counter.regenCalls, "函数移除应触发收口")
}

// 心跳兼容注册（Functions == nil，不带快照）：不触发收口。
func TestUpsertAgentSkipsRegenWithoutFunctionSnapshot(t *testing.T) {
	store := NewStore()
	counter := &regenCountingService{}
	store.SetContractService(counter)

	require.NoError(t, store.UpsertAgent(&AgentSession{
		AgentID: "agent-hb", GameID: "demo-game", Env: "development",
	}))
	assert.Zero(t, counter.regenCalls, "无函数快照的注册不应触发模板收口")
}

// T2 容错：收口失败不阻断注册结果（已提交的会话照常可见，手动
// regenerate 仍是兜底）。
func TestUpsertAgentSurvivesRegenFailure(t *testing.T) {
	store := NewStore()
	counter := &regenCountingService{regenErr: errors.New("regen boom")}
	store.SetContractService(counter)

	require.NoError(t, store.UpsertAgent(regenTestSession(map[string]FunctionMeta{
		"player.query": {Enabled: true, Version: "1", Resource: "player", Capability: "collection_query", Execution: "sync"},
	})), "模板收口失败不应影响注册结果")
	require.NotNil(t, store.AgentsUnsafe()["agent-regen"])
	assert.Equal(t, 1, counter.regenCalls)
}
