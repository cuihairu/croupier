package model

// #23/#33：过滤下拉服务端聚合（TaskRun.OperatorOptions / ExecutionLog.OperatorOptions）
// 与 /tasks actor 过滤下推的 model 层用例。

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskRunModel_OperatorOptionsAndActorFilter(t *testing.T) {
	db := setupAllModelsDB(t)
	rm := NewTaskRunModel(db)
	ctx := context.Background()

	seed := []*TaskRun{
		{TaskID: "t-1", FunctionID: "fn.a", GameID: "demo", Env: "prod", Status: "succeeded", Actor: "alice"},
		{TaskID: "t-2", FunctionID: "fn.a", GameID: "demo", Env: "prod", Status: "succeeded", Actor: "alice"},
		{TaskID: "t-3", FunctionID: "fn.b", GameID: "demo", Env: "prod", Status: "failed", Actor: "bob"},
		// 空 actor（系统触发）：不是可过滤选项，聚合必须排除
		{TaskID: "t-4", FunctionID: "fn.b", GameID: "demo", Env: "prod", Status: "succeeded"},
		// scope 外：不进聚合
		{TaskID: "t-5", FunctionID: "fn.a", GameID: "other", Env: "prod", Status: "succeeded", Actor: "carol"},
	}
	for _, item := range seed {
		require.NoError(t, rm.Create(ctx, item))
	}

	rows, err := rm.OperatorOptions(ctx, ListTasksOptions{GameID: "demo", Env: "prod"})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	// 条数降序：alice(2) 在前
	assert.Equal(t, "alice", rows[0].Name)
	assert.Equal(t, int64(2), rows[0].Count)
	assert.Equal(t, "bob", rows[1].Name)
	assert.Equal(t, int64(1), rows[1].Count)

	// 同过滤维度联动：按函数过滤后聚合只剩 alice
	rows, err = rm.OperatorOptions(ctx, ListTasksOptions{GameID: "demo", Env: "prod", FunctionID: "fn.a"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "alice", rows[0].Name)

	// List 的 Actor 过滤真实下推（此前 /tasks 的 actor 参数被服务端丢弃）
	items, total, err := rm.List(ctx, ListTasksOptions{GameID: "demo", Env: "prod", Actor: "alice"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, items, 2)
	for _, item := range items {
		assert.Equal(t, "alice", item.Actor)
	}
}

func TestExecutionLogModel_OperatorOptions(t *testing.T) {
	db := setupAllModelsDB(t)
	m := NewExecutionLogModel(db)
	ctx := context.Background()

	seed := []ExecutionLog{
		{GameID: "demo", Env: "prod", Source: "invoke", FunctionID: "fn.a", Actor: "alice", Status: "ok"},
		{GameID: "demo", Env: "prod", Source: "invoke", FunctionID: "fn.b", Actor: "alice", Status: "ok"},
		{GameID: "demo", Env: "prod", Source: "invoke", FunctionID: "fn.b", Actor: "bob", Status: "error"},
		// 空 actor：排除；scope 外：排除
		{GameID: "demo", Env: "prod", Source: "invoke", FunctionID: "fn.c", Status: "ok"},
		{GameID: "elsewhere", Env: "prod", Source: "invoke", FunctionID: "fn.a", Actor: "carol", Status: "ok"},
	}
	for i := range seed {
		require.NoError(t, m.Create(ctx, &seed[i]))
	}

	rows, err := m.OperatorOptions(ctx, ExecutionLogListOptions{GameID: "demo", Env: "prod"})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "alice", rows[0].Name)
	assert.Equal(t, int64(2), rows[0].Count)
	assert.Equal(t, "bob", rows[1].Name)

	// 维度联动：按状态过滤后只剩 bob（该维度下 alice 无 error 留痕）
	rows, err = m.OperatorOptions(ctx, ExecutionLogListOptions{GameID: "demo", Env: "prod", Status: "error"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "bob", rows[0].Name)
}
