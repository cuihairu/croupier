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
	// 提案重建观测（卡点 1：移出事务后仍按计划逐资源/逐函数触发）。
	resourceProposalCalls []string
	functionProposalCalls []string
	proposalErr           error
}

func (r *regenCountingService) RebuildContractFromFunctionMeta(context.Context, string, string, string, spec.FunctionContractInput) error {
	return nil
}

func (r *regenCountingService) RemoveFunctionContract(_ context.Context, _, _, functionID string) (string, error) {
	if functionID == "mail.query" {
		return "mail", nil
	}
	return "", nil
}

func (r *regenCountingService) RebuildResourceCapability(context.Context, string, string, string) error {
	return nil
}

func (r *regenCountingService) RebuildProposalsForResource(_ context.Context, _, _, resourceKey string) error {
	r.resourceProposalCalls = append(r.resourceProposalCalls, resourceKey)
	return r.proposalErr
}

func (r *regenCountingService) RebuildProposalForFunction(_ context.Context, _, _, functionID string) error {
	r.functionProposalCalls = append(r.functionProposalCalls, functionID)
	return r.proposalErr
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
// regenerate 仍是兜底），且失败写入 registrationWarnings（UI 可见）。
func TestUpsertAgentSurvivesRegenFailure(t *testing.T) {
	store := NewStore()
	counter := &regenCountingService{regenErr: errors.New("regen boom")}
	store.SetContractService(counter)

	require.NoError(t, store.UpsertAgent(regenTestSession(map[string]FunctionMeta{
		"player.query": {Enabled: true, Version: "1", Resource: "player", Capability: "collection_query", Execution: "sync"},
	})), "模板收口失败不应影响注册结果")
	require.NotNil(t, store.AgentsUnsafe()["agent-regen"])
	assert.Equal(t, 1, counter.regenCalls)

	warnings := store.ListRegistrationWarnings(RegistrationWarningFilter{Code: WarningCodeTemplateRegenFailed})
	require.Len(t, warnings, 1, "模板重建失败应产生 template_regen_failed 告警")
	assert.Equal(t, "regen boom", warnings[0].Message)
	assert.Equal(t, "agent-regen", warnings[0].AgentID)
	assert.Equal(t, 1, warnings[0].Count)

	// 快照变化的重注册再次失败：同一告警按 message 去重、Count 递增。
	require.NoError(t, store.UpsertAgent(regenTestSession(map[string]FunctionMeta{
		"player.query": {Enabled: true, Version: "2", Resource: "player", Capability: "collection_query", Execution: "sync"},
	})))
	assert.Equal(t, 2, counter.regenCalls)
	warnings = store.ListRegistrationWarnings(RegistrationWarningFilter{Code: WarningCodeTemplateRegenFailed})
	require.Len(t, warnings, 1)
	assert.Equal(t, 2, warnings[0].Count, "重复失败应递增 Count 而非新增条目")
}

// 卡点 1（提案移出注册事务）：提交后按计划逐资源/逐函数触发提案重建；
// 同内容重注册与原事务内行为一致地全量幂等重算（提案无 diff 门控，
// 心跳风暴成本由幂等 upsert 承接）；失败只告警不回滚。
func TestUpsertAgentRebuildsProposalsAfterCommit(t *testing.T) {
	store := NewStore()
	counter := &regenCountingService{}
	store.SetContractService(counter)

	session := func(version string) *AgentSession {
		return &AgentSession{
			AgentID: "agent-proposal",
			GameID:  "demo-game",
			Env:     "development",
			Functions: map[string]FunctionMeta{
				"mail.query": {Enabled: true, Version: version, Resource: "mail", Capability: "collection_query", Execution: "sync"},
				"mail.send":  {Enabled: true, Version: version},
			},
		}
	}

	require.NoError(t, store.UpsertAgent(session("1")))
	assert.Equal(t, []string{"mail"}, counter.resourceProposalCalls, "资源维度提案重建应在提交后触发")
	assert.Equal(t, []string{"mail.send"}, counter.functionProposalCalls, "standalone 提案重建应在提交后触发")

	// 同内容重注册（心跳风暴）：提案重建无 diff 门控——materializeAgent
	// 全量收集，与原事务内行为一致地幂等重算（调用递增、结果幂等）。
	require.NoError(t, store.UpsertAgent(session("1")))
	assert.Len(t, counter.resourceProposalCalls, 2)
	assert.Len(t, counter.functionProposalCalls, 2)

	// 提案重建失败：注册仍成功，失败降级为 proposal_rebuild_failed 告警。
	counter.proposalErr = errors.New("proposal db down")
	require.NoError(t, store.UpsertAgent(session("2")))
	require.NotNil(t, store.AgentsUnsafe()["agent-proposal"])
	warnings := store.ListRegistrationWarnings(RegistrationWarningFilter{Code: WarningCodeProposalRebuildFailed})
	require.Len(t, warnings, 2, "资源与 standalone 两条失败各产生一条告警")
	assert.Contains(t, warnings[0].Message+warnings[1].Message, "resource mail")
	assert.Contains(t, warnings[0].Message+warnings[1].Message, "function mail.send")

	// Removed：函数下线后 RemoveFunctionContract 返回的 resourceKey 进入
	// 计划，资源维度提案重建（清理路径）在提交后仍被触发；standalone
	// 函数已随事务内契约级联清理，不再进入计划。
	counter.proposalErr = nil
	require.NoError(t, store.UpsertAgent(&AgentSession{
		AgentID: "agent-proposal", GameID: "demo-game", Env: "development",
		Functions: map[string]FunctionMeta{},
	}))
	assert.Equal(t, []string{"mail", "mail", "mail", "mail"}, counter.resourceProposalCalls, "Removed 后资源维度应再次触发（清理路径）")
	assert.Len(t, counter.functionProposalCalls, 3, "standalone 已级联清理，不再触发函数维度重建")
}
