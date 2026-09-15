package service

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T3/D2 验收：执行状态不参与契约 digest——契约内容与绑定态正交，
// 绑定态翻转（T6 自动绑定）不得造成 semantics/proposal digest 与提案
// 版本快照的无谓 churn。computeDigest 直接 json.Marshal 模型行，
// 由 model.FunctionContract.ExecutionState 的 json:"-" 排除（机制锁定）。
func TestExecutionStateExcludedFromContractDigest(t *testing.T) {
	base := &model.FunctionContract{
		GameID:      "g",
		Env:         "e",
		FunctionID:  "player.get",
		Version:     "1",
		ResourceKey: "player",
		Capability:  4,
		Execution:   "sync",
		InputSchema: model.JSON(`{"type":"object"}`),
	}
	bound := *base
	bound.ExecutionState = string(spec.ExecutionStateBound)
	unbound := *base
	unbound.ExecutionState = string(spec.ExecutionStateUnbound)

	assert.Equal(t, computeDigest(&bound), computeDigest(&unbound),
		"仅 ExecutionState 不同的契约 digest 必须一致（json:\"-\" 机制）")
	assert.Equal(t,
		computeDigest([]*model.FunctionContract{&bound}),
		computeDigest([]*model.FunctionContract{&unbound}),
		"semantics/proposal 的 contracts 列表 digest 同样不受影响")

	// 归一层输出（computeDigest(result.Function) 路径）不设 ExecutionState
	//（空串经 omitted 排除）——注册 digest 序列不受 T3 影响；T2 的
	// digest-unchanged 重注册用例在 T3 代码在位时仍绿即为其回归锁。
	fnJSON := toJSON(spec.FunctionSpec{ID: "player.get"})
	assert.NotContains(t, string(fnJSON), "executionState",
		"归一层输出不含 executionState，注册 digest 序列不变")
}

// T3 验收：运行时/SDK 注册写 bound；DB 侧 unbound 行在 schema 未变的
// 重注册下不被覆盖（UpsertContract 跳过写）——翻转是 T6 自动绑定的职责。
func TestContractRegistrationWritesBound(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()
	svc := NewContractService(db)

	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g-t3", "e-t3", "agent-1", t2ContractInput("player.get")))
	contractModel := model.NewFunctionContractModel(db)
	stored, err := contractModel.FindByScopeAndFunctionID(ctx, "g-t3", "e-t3", "player.get")
	require.NoError(t, err)
	assert.Equal(t, string(spec.ExecutionStateBound), stored.ExecutionState,
		"注册即存在可执行后端，新契约必须写 bound")

	// 模拟 T4 产物：行翻转为 unbound 后，重注册按 T6 自动绑定翻转回 bound
	//（ExecutionState 参与 contractSemanticallyEqual——状态变化不被
	//「内容无变化跳过写」吞掉；digest 仍不含状态列，下游零扰动）。
	require.NoError(t, db.Exec("UPDATE function_contracts SET execution_state = 'unbound' WHERE function_id = 'player.get'").Error)
	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g-t3", "e-t3", "agent-1", t2ContractInput("player.get")))
	stored, err = contractModel.FindByScopeAndFunctionID(ctx, "g-t3", "e-t3", "player.get")
	require.NoError(t, err)
	assert.Equal(t, string(spec.ExecutionStateBound), stored.ExecutionState,
		"T6 自动绑定：同 schema 重注册也必须把 unbound 行翻转为 bound")
}

// T3 验收：投影层透传 executionState，存量行空值归一为 bound（迁移
// DEFAULT 的投影层对应物）。
func TestFunctionSpecExecutionStateProjection(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want spec.ExecutionState
	}{
		{"存量空值归一 bound", "", spec.ExecutionStateBound},
		{"unbound 透传", "unbound", spec.ExecutionStateUnbound},
		{"bound 透传", "bound", spec.ExecutionStateBound},
		{"非法值归一 bound", "pending", spec.ExecutionStateBound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FunctionSpecFromContract(&model.FunctionContract{
				FunctionID:     "player.get",
				ExecutionState: tc.raw,
			})
			assert.Equal(t, tc.want, got.ExecutionState)
		})
	}

	// wire 形态（T8/T9 前端消费的 DTO）：投影恒写显式值——bound/unbound
	// 都出现（bound 非空串不受 omitempty 省略）；仅程序化构造的空值省略。
	boundJSON := toJSON(FunctionSpecFromContract(&model.FunctionContract{FunctionID: "f"}))
	assert.Contains(t, string(boundJSON), `"executionState":"bound"`)
	unboundJSON := toJSON(FunctionSpecFromContract(&model.FunctionContract{FunctionID: "f", ExecutionState: "unbound"}))
	assert.Contains(t, string(unboundJSON), `"executionState":"unbound"`)
}
