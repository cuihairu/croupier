package approval

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ARCH-024：批准「调度激活审批」（approvalTarget=schedule）→ 激活 paused 调度、
// 写入批准快照（approvalBypass=approved）并重算 next_triggered_at；cron 触发以
// 快照作为站立授权。
func TestApproveSchedule_ActivatesSchedule(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:ap_sched_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)
	approver := &model.Admin{Username: "approver", Nickname: "Approver", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), approver, "pw"))
	role := &model.Role{Name: "operator"}
	require.NoError(t, roleModel.Create(context.Background(), role))
	require.NoError(t, adminModel.AssignRole(context.Background(), approver.ID, role.ID))

	svcCtx := &svc.ServiceContext{
		DB:                db,
		AdminModel:        adminModel,
		RoleModel:         roleModel,
		TaskScheduleModel: model.NewTaskScheduleModel(db),
		ApprovalsStore:    approvals.NewMemStore(),
	}

	// paused 调度（政策要求审批时创建的形态），metadata 携带 approvalId 引用。
	created, err := svcCtx.TaskScheduleModel.Create(context.Background(), model.CreateScheduleInput{
		Name: "高危调度", CronExpr: "0 4 * * *", GameID: "covgame", Env: "prod",
		FunctionID: "cov.echo", Payload: model.JSON(`{"x":1}`), Actor: "tester",
		Status: model.ScheduleStatusPaused,
	})
	require.NoError(t, err)
	_, err = svcCtx.ApprovalsStore.Create(&approvals.Approval{
		ID: "ap-sched", State: "pending", FunctionID: "cov.echo",
		GameID: "covgame", Env: "prod", Actor: "tester",
		Payload: []byte(`{"x":1}`),
		Metadata: map[string]string{
			"approvalTarget": "schedule",
			"scheduleId":     fmt.Sprintf("%d", created.ID),
		},
	})
	require.NoError(t, err)

	s := NewService(svcCtx)
	ctx := context.WithValue(context.Background(), "username", "approver")
	resp, err := s.Approve(ctx, &ApprovalApproveRequest{ID: "ap-sched"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "approved", resp.State)
	assert.True(t, resp.Continuation)
	assert.Equal(t, "schedule", resp.ResultKind)

	// 调度被激活 + 批准快照写入 + next_triggered_at 重算。
	var sch model.TaskSchedule
	require.NoError(t, db.Where("id = ?", created.ID).First(&sch).Error)
	assert.Equal(t, model.ScheduleStatusActive, sch.Status)
	assert.NotNil(t, sch.NextTriggeredAt)
	assert.True(t, sch.NextTriggeredAt.After(time.Now()))

	var meta map[string]string
	require.NoError(t, json.Unmarshal(sch.Metadata, &meta))
	assert.Equal(t, "approved", meta["approvalBypass"])
	assert.Equal(t, "ap-sched", meta["approvalId"])
	assert.Equal(t, "tester", meta["approvalActor"])
}
