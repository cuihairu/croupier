package task

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	policymgr "github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ARCH-024：/tasks Start 经统一治理闸门，政策要求审批时创建审批请求而非
// 直接派发（高危函数不能经 /tasks 绕过审批）。
func TestServiceStart_RequireApprovalCreatesApproval(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svcCtx.ApprovalsStore = approvals.NewMemStore()
	createTestFunction(t, svcCtx.DB, "player.ban", "Ban Player")
	require.NoError(t, svcCtx.PolicyManager.SetOverride(context.Background(), "player.ban", &policymgr.Policy{
		FunctionID: "player.ban", RequireApproval: true, ApprovalWorkflow: "single_admin",
	}))

	ctx := svc.WithGameScope(seedAdminWithRole(t, svcCtx.DB, "root", "admin"), svc.GameScope{GameID: "test-game", Env: "test-env"})
	rt := &fakeRuntimeV9{findFn: &model.FunctionContract{}}
	taskSvc := &Service{svcCtx: svcCtx, runtime: rt}

	resp, err := taskSvc.Start(ctx, &StartRequest{FunctionID: "player.ban", Params: map[string]interface{}{"id": 1}})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.ApprovalRequired)
	assert.NotEmpty(t, resp.ApprovalID)
	assert.Empty(t, resp.TaskID, "审批要求时不得派发任务")

	// 未派发：runtime 未收到 StartTask；审批记录落库。
	assert.Nil(t, rt.lastReq, "政策要求审批时不得调用 StartTask")
	got, err := svcCtx.ApprovalsStore.Get(resp.ApprovalID)
	require.NoError(t, err)
	assert.Equal(t, "pending", got.State)
	assert.Equal(t, "player.ban", got.FunctionID)
	assert.Equal(t, "task", got.Metadata["approvalTarget"])
	assert.Equal(t, "root", got.Actor)
}

// ARCH-024：政策不要求审批时照常派发（无审批记录）。
func TestServiceStart_NoApprovalDispatches(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svcCtx.ApprovalsStore = approvals.NewMemStore()
	createTestFunction(t, svcCtx.DB, "player.ban", "Ban Player")

	ctx := svc.WithGameScope(seedAdminWithRole(t, svcCtx.DB, "root", "admin"), svc.GameScope{GameID: "test-game", Env: "test-env"})
	rt := &fakeRuntimeV9{findFn: &model.FunctionContract{}}
	taskSvc := &Service{svcCtx: svcCtx, runtime: rt}

	resp, err := taskSvc.Start(ctx, &StartRequest{FunctionID: "player.ban", Params: map[string]interface{}{"id": 1}})
	require.NoError(t, err)
	assert.False(t, resp.ApprovalRequired)
	assert.Empty(t, resp.ApprovalID)
	require.NotNil(t, rt.lastReq, "无审批要求时必须派发")
	assert.Equal(t, "fake-task-v9", resp.TaskID)
}
