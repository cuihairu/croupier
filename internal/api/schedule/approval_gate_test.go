package schedule

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	policymgr "github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func scheduleApprovalCtx(t *testing.T, db *gorm.DB) (*Service, *svc.ServiceContext, context.Context) {
	t.Helper()
	svcCtx := scheduleAdminSvcCtx(t, db, "tester")
	svcCtx.ApprovalsStore = approvals.NewMemStore()
	ctx := svc.WithGameScope(context.WithValue(context.Background(), "username", "tester"), svc.GameScope{GameID: "demo", Env: "prod"})
	return NewService(svcCtx), svcCtx, ctx
}

// ARCH-024：schedule Create 经统一治理闸门——政策要求审批时创建为 paused +
// 审批请求（approvalTarget=schedule），批准续跑后才激活。
func TestScheduleCreate_RequireApprovalCreatesPausedSchedule(t *testing.T) {
	name := "schedule_ap_create"
	db, err := gorm.Open(gsqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	s, svcCtx, ctx := scheduleApprovalCtx(t, db)
	require.NoError(t, svcCtx.PolicyManager.SetOverride(context.Background(), "fn.reset", &policymgr.Policy{
		FunctionID: "fn.reset", RequireApproval: true, ApprovalWorkflow: "single_admin",
	}))

	resp, err := s.Create(ctx, &CreateRequest{
		Name: "每日重置", CronExpr: "*/5 * * * *", FunctionID: "fn.reset",
		GameID: "demo", Env: "prod", Payload: []byte(`{"k":1}`), MaxFailedRuns: 3,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.ApprovalRequired)
	assert.NotEmpty(t, resp.ApprovalID)
	assert.Equal(t, model.ScheduleStatusPaused, resp.Item.Status)

	got, err := svcCtx.ApprovalsStore.Get(resp.ApprovalID)
	require.NoError(t, err)
	assert.Equal(t, "pending", got.State)
	assert.Equal(t, "schedule", got.Metadata["approvalTarget"])
	assert.Equal(t, "fn.reset", got.FunctionID)
}

// ARCH-024：TriggerNow 经统一治理闸门——政策要求审批时创建审批请求而非派发。
func TestScheduleTriggerNow_RequireApprovalCreatesApproval(t *testing.T) {
	name := "schedule_ap_trigger"
	db, err := gorm.Open(gsqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	s, svcCtx, ctx := scheduleApprovalCtx(t, db)
	require.NoError(t, svcCtx.PolicyManager.SetOverride(context.Background(), "fn.reset", &policymgr.Policy{
		FunctionID: "fn.reset", RequireApproval: true,
	}))

	now := time.Now()
	created, err := svcCtx.TaskScheduleModel.Create(ctx, model.CreateScheduleInput{
		Name: "已激活调度", CronExpr: "0 3 * * *", GameID: "demo", Env: "prod",
		FunctionID: "fn.reset", Payload: model.JSON(`{"k":1}`), Actor: "tester",
	})
	require.NoError(t, err)
	require.NoError(t, svcCtx.TaskScheduleModel.UpdateSchedule(ctx, created.ID, map[string]interface{}{"next_triggered_at": now}))

	resp, err := s.TriggerNow(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.ApprovalRequired)
	assert.NotEmpty(t, resp.ApprovalID)
	assert.Empty(t, resp.TaskRunID, "审批要求时不得派发任务")
}

// ARCH-024：政策不要求审批时 TriggerNow 照常派发（此处 Dispatcher 空注册表 →
// 路由错误，但证明未走审批分支）。
func TestScheduleTriggerNow_NoApprovalDispatches(t *testing.T) {
	name := "schedule_ap_trigger_no"
	db, err := gorm.Open(gsqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	s, svcCtx, ctx := scheduleApprovalCtx(t, db)

	now := time.Now()
	created, err := svcCtx.TaskScheduleModel.Create(ctx, model.CreateScheduleInput{
		Name: "已激活调度", CronExpr: "0 3 * * *", GameID: "demo", Env: "prod",
		FunctionID: "fn.reset", Payload: model.JSON(`{"k":1}`), Actor: "tester",
	})
	require.NoError(t, err)
	require.NoError(t, svcCtx.TaskScheduleModel.UpdateSchedule(ctx, created.ID, map[string]interface{}{"next_triggered_at": now}))

	_, err = s.TriggerNow(ctx, created.ID)
	require.Error(t, err, "无审批要求时派发：空注册表无 agent → 路由错误")
}
