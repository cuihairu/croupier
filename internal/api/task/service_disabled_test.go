package task

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// E2 禁用拦截：/tasks Start 与 functionInvoke 共用 EnsureFunctionEnabled，
// 禁用函数不能借异步任务入口绕过（409 function_disabled）。
func TestServiceStartV9_DisabledFunctionBlocked(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	require.NoError(t, svcCtx.FunctionModel.Create(context.Background(), &model.Function{
		FunctionID: "player.ban",
		GameID:     "test-game",
		Status:     model.StatusDisabled,
	}))
	rt := &fakeRuntimeV9{findFn: &model.FunctionContract{}}
	taskSvc := &Service{svcCtx: svcCtx, runtime: rt}
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "test-game", Env: "test-env"})

	_, err := taskSvc.Start(ctx, &StartRequest{FunctionID: "player.ban"})
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, 409, codeErr.Code)
	assert.Equal(t, "function_disabled", codeErr.ErrorCode())
	assert.Nil(t, rt.lastReq, "禁用函数不得派发任务")
}

// 无 functions 物化行时守卫直通（与禁用写路径同一张真值表），不拦正常启动。
func TestServiceStartV9_UnmaterializedFunctionGatePasses(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	// functions 表无行：守卫不拦，链路走到 LoadCurrentAdmin 才失败
	// （ctx 未带 username），证明守卫直通而非被禁用拦截。
	taskSvc := &Service{svcCtx: svcCtx, runtime: &fakeRuntimeV9{findFn: &model.FunctionContract{}}}
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "test-game", Env: "test-env"})

	_, err := taskSvc.Start(ctx, &StartRequest{FunctionID: "player.ban"})
	require.Error(t, err)
	var codeErr *errorx.CodeError
	assert.False(t, errors.As(err, &codeErr) && codeErr.ErrorCode() == "function_disabled")
	assert.Contains(t, err.Error(), "未找到登录用户")
}
