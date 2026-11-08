package batchop

// RegistrySelector 前置守卫（nil 接收者 / 空 store / 缺 gameId/env）与
// Execute 的 selectors 解码失败落 failed 分支：此前无用例触达。

import (
	"context"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistrySelector_GuardErrors(t *testing.T) {
	ctx := context.Background()
	var nilSel *RegistrySelector
	if _, err := nilSel.Select(ctx, "g", "e", nil, nil); err == nil || !strings.Contains(err.Error(), "registry store unavailable") {
		t.Fatalf("nil selector err = %v, want registry store unavailable", err)
	}
	empty := &RegistrySelector{}
	if _, err := empty.Select(ctx, "g", "e", nil, nil); err == nil || !strings.Contains(err.Error(), "registry store unavailable") {
		t.Fatalf("nil store err = %v, want registry store unavailable", err)
	}
	withStore := NewRegistrySelector(reg.NewStore())
	if _, err := withStore.Select(ctx, "", "", nil, nil); err == nil || !strings.Contains(err.Error(), "gameId and env required") {
		t.Fatalf("empty scope err = %v, want gameId and env required", err)
	}
}

func TestExecute_UndecodableSelectorsMarksFailed(t *testing.T) {
	store := newFakeStore()
	o := New(&fakeSelector{agents: []string{"agent-1"}}, &fakeExecutor{}, store, nil)
	op, err := o.Create(context.Background(), &CreateRequest{
		GameID: "g", Env: "e", Kind: "exec", AgentIDs: []string{"agent-1"},
		Exec: &ExecPayload{Command: "echo"},
	})
	require.NoError(t, err)

	// 圈选标签存成非法 JSON：解码失败必须把批次落 failed 并回传错误。
	store.ops[op.BatchID].LabelsJSON = "not-json"
	_, err = o.Execute(context.Background(), "g", "e", op.BatchID, false)
	require.Error(t, err)
	// 解码失败必须把批次落 failed（fakeStore 未建模 results_json，只断言状态）。
	assert.Equal(t, model.BatchOpStatusFailed, store.ops[op.BatchID].Status)
}
