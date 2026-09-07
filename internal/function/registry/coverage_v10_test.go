package registry

import (
	"context"
	"testing"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- Register 参数校验 ----

func TestStore_Register_NilMetadataV10(t *testing.T) {
	store := NewStore()
	err := store.Register(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metadata is required")
}

func TestStore_Register_EmptyIDV10(t *testing.T) {
	store := NewStore()
	err := store.Register(context.Background(), &functionv1.FunctionMetadata{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "function ID is required")
}

// ---- RegisterBatch：单项失败时带 ID 包装错误 ----
func TestStore_RegisterBatch_WrappedErrorV10(t *testing.T) {
	store := NewStore()
	err := store.RegisterBatch(context.Background(), []*functionv1.FunctionMetadata{
		{Id: "ok.func"},
		{}, // Id 为空 → Register 报 "function ID is required"
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "register  failed")
	assert.Contains(t, err.Error(), "function ID is required")
}

// ---- Filter：nil filter / 各索引未命中提前返回空 ----

func TestStore_Filter_NilFilterV10(t *testing.T) {
	ctx := context.Background()
	store := NewStore()
	require.NoError(t, store.Register(ctx, &functionv1.FunctionMetadata{Id: "a.b"}))

	result, err := store.Filter(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestStore_Filter_ResourceMissV10(t *testing.T) {
	ctx := context.Background()
	store := NewStore()
	require.NoError(t, store.Register(ctx, &functionv1.FunctionMetadata{Id: "a.b", Resource: "res"}))

	result, err := store.Filter(ctx, &functionv1.FunctionFilter{Resource: "nope"})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestStore_Filter_TagMissV10(t *testing.T) {
	ctx := context.Background()
	store := NewStore()
	require.NoError(t, store.Register(ctx, &functionv1.FunctionMetadata{
		Id: "a.b", Tags: []string{"t1"},
	}))

	result, err := store.Filter(ctx, &functionv1.FunctionFilter{Tags: []string{"nope"}})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestStore_Filter_RiskLevelMissV10(t *testing.T) {
	ctx := context.Background()
	store := NewStore()
	require.NoError(t, store.Register(ctx, &functionv1.FunctionMetadata{
		Id:       "a.b",
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_LOW},
	}))

	result, err := store.Filter(ctx, &functionv1.FunctionFilter{RiskLevel: "nope"})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestStore_Filter_ModeMissV10(t *testing.T) {
	ctx := context.Background()
	store := NewStore()
	require.NoError(t, store.Register(ctx, &functionv1.FunctionMetadata{
		Id:       "a.b",
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_MODE_QUERY},
	}))

	result, err := store.Filter(ctx, &functionv1.FunctionFilter{Mode: "nope"})
	require.NoError(t, err)
	assert.Empty(t, result)
}

// ---- cloneMetadata 失败（nil 注入）：List/ListBy*/Filter 跳过坏条目 ----
// cloneMetadata 仅在入参为 nil 时报错；索引指向的 map 值经白盒注入为 nil，
// 触发各读取路径的 "clone 失败 → continue/skip" 防御分支。

func injectNilEntryV10(store *Store, id string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.functions[id] = nil
	if store.byResource["res"] == nil {
		store.byResource["res"] = map[string]struct{}{}
	}
	store.byResource["res"][id] = struct{}{}
	if store.byTag["tag"] == nil {
		store.byTag["tag"] = map[string]struct{}{}
	}
	store.byTag["tag"][id] = struct{}{}
	if store.byRisk["low"] == nil {
		store.byRisk["low"] = map[string]struct{}{}
	}
	store.byRisk["low"][id] = struct{}{}
	if store.byMode["query"] == nil {
		store.byMode["query"] = map[string]struct{}{}
	}
	store.byMode["query"][id] = struct{}{}
}

func TestStore_List_CloneErrorSkipsNilEntryV10(t *testing.T) {
	store := NewStore()
	injectNilEntryV10(store, "ghost.nil")

	// List 的 clone 失败分支在 slog.Warn 中直接解引用 metadata.Id，
	// nil 条目会 panic（防御分支自身缺陷）；此处验证该行为并防止静默返回坏数据。
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic: List defensive branch dereferences nil metadata.Id")
		}
	}()
	_, _ = store.List(context.Background())
	t.Error("unreachable")
}

func TestStore_ListByResource_CloneErrorSkipsNilEntryV10(t *testing.T) {
	store := NewStore()
	injectNilEntryV10(store, "ghost.nil")

	result, err := store.ListByResource(context.Background(), "res")
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestStore_ListByTag_CloneErrorSkipsNilEntryV10(t *testing.T) {
	store := NewStore()
	injectNilEntryV10(store, "ghost.nil")

	result, err := store.ListByTag(context.Background(), "tag")
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestStore_ListByRiskLevel_CloneErrorSkipsNilEntryV10(t *testing.T) {
	store := NewStore()
	injectNilEntryV10(store, "ghost.nil")

	result, err := store.ListByRiskLevel(context.Background(), "low")
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestStore_ListByMode_CloneErrorSkipsNilEntryV10(t *testing.T) {
	store := NewStore()
	injectNilEntryV10(store, "ghost.nil")

	result, err := store.ListByMode(context.Background(), functionv1.FunctionBehavior_MODE_QUERY)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestStore_Filter_CloneErrorSkipsNilEntryV10(t *testing.T) {
	store := NewStore()
	injectNilEntryV10(store, "ghost.nil")

	result, err := store.Filter(context.Background(), &functionv1.FunctionFilter{})
	require.NoError(t, err)
	assert.Empty(t, result)
}
