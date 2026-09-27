package task

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 分配闸门（OPEN-ISSUES #36 / BUG-032）：/tasks Start 与 functionInvoke
// 共用 EnsureFunctionAssigned，scope 存在分配记录时未分配函数不能借异步
// 任务入口绕过白名单（403 function_not_assigned）——与 E2 禁用拦截
// （service_disabled_test.go）同位同理由。
func writeTaskAssignments(t *testing.T, svcCtx *svc.ServiceContext, data string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "assignments.json")
	require.NoError(t, os.WriteFile(path, []byte(data), 0o644))
	svcCtx.Config.Registry.AssignmentsPath = path
}

func TestServiceStartV9_UnassignedFunctionDenied(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	writeTaskAssignments(t, svcCtx, `{"test-game|test-env":["player.ban"]}`)
	rt := &fakeRuntimeV9{findFn: &model.FunctionContract{}}
	taskSvc := &Service{svcCtx: svcCtx, runtime: rt}
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "test-game", Env: "test-env"})

	_, err := taskSvc.Start(ctx, &StartRequest{FunctionID: "player.kick"})
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, 403, codeErr.Code)
	assert.Equal(t, "function_not_assigned", codeErr.ErrorCode())
	assert.Nil(t, rt.lastReq, "未分配函数不得派发任务")
}

// 列表内函数通过闸门（链路继续走到登录用户校验——ctx 未带 username，
// 以「未找到登录用户」证明闸门放行而非拒绝）。
func TestServiceStartV9_AssignedFunctionGatePasses(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	writeTaskAssignments(t, svcCtx, `{"test-game|test-env":["player.ban"]}`)
	taskSvc := &Service{svcCtx: svcCtx, runtime: &fakeRuntimeV9{findFn: &model.FunctionContract{}}}
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "test-game", Env: "test-env"})

	_, err := taskSvc.Start(ctx, &StartRequest{FunctionID: "player.ban"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未找到登录用户")
}
