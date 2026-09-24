package task

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

// 任务详情/取消/SSE 均是敏感操作入口：scope 缺失必须 400，跨 game/env 必须 403。

func TestCurrentTaskScope_MissingScopeIsBadRequest(t *testing.T) {
	_, err := currentTaskScope(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scope")

	// 只有 game 没有 env 也算缺失。
	_, err = currentTaskScope(svc.WithGameScope(context.Background(), svc.GameScope{GameID: "g"}))
	require.Error(t, err)

	scope, err := currentTaskScope(svc.WithGameScope(context.Background(),
		svc.GameScope{GameID: "g1", Env: "prod"}))
	require.NoError(t, err)
	assert.Equal(t, "g1", scope.GameID)
	assert.Equal(t, "prod", scope.Env)
}

func TestRequireTaskScope_CrossScopeForbidden(t *testing.T) {
	run := &model.TaskRun{GameID: "g1", Env: "prod"}

	// 无 scope 注入 → 放行（兼容非 scoped 调用方）。
	assert.NoError(t, requireTaskScope(context.Background(), run))

	// 匹配 → 放行。
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "g1", Env: "prod"})
	assert.NoError(t, requireTaskScope(ctx, run))

	// 跨 game / 跨 env / nil run → 拒绝。
	ctxOtherGame := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "g2", Env: "prod"})
	err := requireTaskScope(ctxOtherGame, run)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权")

	ctxOtherEnv := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "g1", Env: "dev"})
	err = requireTaskScope(ctxOtherEnv, run)
	require.Error(t, err)

	err = requireTaskScope(ctx, nil)
	require.Error(t, err)
}
