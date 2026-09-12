// 覆盖目标：schedule service.SetStatus(active) 中 m.SetStatus 状态更新
// 失败分支（service.go:188-190）。
//
// SetStatus(active) 依次执行两次 UPDATE：先重算 next_triggered_at、再落
// status。现有用例（schedule_extra_test.go 的 active_second_update_fails）
// 的计数起点包含创建时的 UPDATE，实际命中的是重算那次；本文件在创建完成
// 之后再注册“第 2 次起失败”的回调，使重算成功、状态更新失败。
package schedule

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestService_SetStatus_ActiveStatusUpdateFailsGroupE(t *testing.T) {
	name := fmt.Sprintf("schedule_groupE_%d", atomic.AddUint64(&dbSeq, 1))
	db, err := gorm.Open(gsqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	m := model.NewTaskScheduleModel(db)
	s := NewService(&svc.ServiceContext{TaskScheduleModel: m})
	ctx := context.WithValue(context.Background(), "username", "tester")

	created, err := m.Create(ctx, model.CreateScheduleInput{
		Name: "groupE", CronExpr: "*/5 * * * *", GameID: "demo", Env: "prod",
		FunctionID: "fn.reset", Actor: "tester",
	})
	require.NoError(t, err)

	var calls int32
	require.NoError(t, db.Callback().Update().Before("gorm:update").
		Register("groupE_fail_second_update", func(tx *gorm.DB) {
			if atomic.AddInt32(&calls, 1) >= 2 {
				_ = tx.AddError(errors.New("status update boom"))
			}
		}))
	t.Cleanup(func() { _ = db.Callback().Update().Remove("groupE_fail_second_update") })

	item, err := s.SetStatus(ctx, created.ID, model.ScheduleStatusActive)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status update boom")
	assert.Equal(t, ScheduleItem{}, item)
}
