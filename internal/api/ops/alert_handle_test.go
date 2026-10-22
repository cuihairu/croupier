package ops

import (
	"context"
	"fmt"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gosqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newAlertHandleService：:memory: sqlite + alerts 表（单连接防多实例漂移）。
func newAlertHandleService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gosqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}))
	return NewService(&svc.ServiceContext{AlertModel: model.NewAlertModel(db)}), db
}

func seedHandleAlert(t *testing.T, db *gorm.DB, alertID string) *model.Alert {
	t.Helper()
	alert := &model.Alert{
		AlertID: alertID,
		Type:    "capture_rule_hit",
		Level:   "critical",
		Message: "reconcile mismatch",
		Source:  "capture",
		Status:  "firing",
	}
	require.NoError(t, db.Create(alert).Error)
	return alert
}

func TestOpsAlertHandleRequiresMatchingScope(t *testing.T) {
	s, db := newAlertHandleService(t)
	alert := seedHandleAlert(t, db, "h-1")
	require.NoError(t, db.Model(alert).Updates(map[string]interface{}{"game_id": "demo", "env": "prod"}).Error)
	ctx := svc.WithGameScope(withUsername(context.Background(), "alice"), svc.GameScope{GameID: "other", Env: "prod"})

	_, err := s.OpsAlertHandle(ctx, "1", &OpsAlertHandleRequest{Action: "acknowledge"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong")

	var row model.Alert
	require.NoError(t, db.First(&row, alert.ID).Error)
	assert.Equal(t, model.AlertHandlePending, row.HandleStatus)
	assert.Empty(t, row.HandledBy)
	assert.Empty(t, row.HandledRemark)

	matching := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})
	resp, err := s.OpsAlertHandle(matching, fmt.Sprintf("%d", alert.ID), &OpsAlertHandleRequest{Action: "acknowledge"})
	require.NoError(t, err)
	assert.Equal(t, model.AlertHandleAcknowledged, resp.HandleStatus)
}

func withUsername(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, "username", name)
}

func TestOpsAlertHandleTransitions(t *testing.T) {
	s, db := newAlertHandleService(t)
	seedHandleAlert(t, db, "h-1")
	ctx := withUsername(context.Background(), "alice")

	// acknowledge：审计三件套一次落库（处理人取认证上下文）。
	resp, err := s.OpsAlertHandle(ctx, "1", &OpsAlertHandleRequest{Action: "acknowledge", Remark: "checked ledger"})
	require.NoError(t, err)
	assert.Equal(t, "acknowledged", resp.HandleStatus)
	assert.Equal(t, "alice", resp.HandledBy)
	assert.NotEmpty(t, resp.HandledAt)

	var row model.Alert
	require.NoError(t, db.First(&row, 1).Error)
	assert.Equal(t, model.AlertHandleAcknowledged, row.HandleStatus)
	assert.Equal(t, "alice", row.HandledBy)
	assert.Equal(t, "checked ledger", row.HandledRemark)
	require.NotNil(t, row.HandledAt)

	// escalate：再流转到不同目标态合法（审计字段覆盖）。
	resp2, err := s.OpsAlertHandle(ctx, "1", &OpsAlertHandleRequest{Action: "escalate", Remark: "needs security review"})
	require.NoError(t, err)
	assert.Equal(t, "escalated", resp2.HandleStatus)
	require.NoError(t, db.First(&row, 1).Error)
	assert.Equal(t, model.AlertHandleEscalated, row.HandleStatus)
}

func TestOpsAlertHandleValidation(t *testing.T) {
	s, db := newAlertHandleService(t)
	seedHandleAlert(t, db, "h-1")
	ctx := context.Background()

	// 非法动作 → 422。
	_, err := s.OpsAlertHandle(ctx, "1", &OpsAlertHandleRequest{Action: "delete"})
	require.Error(t, err)
	assert.Equal(t, 422, httpCode(err))

	// 空请求 → 422。
	_, err = s.OpsAlertHandle(ctx, "1", &OpsAlertHandleRequest{})
	require.Error(t, err)
	assert.Equal(t, 422, httpCode(err))

	// 不存在的告警 → 404（流转不隐式创建）。
	_, err = s.OpsAlertHandle(ctx, "99", &OpsAlertHandleRequest{Action: "acknowledge"})
	require.Error(t, err)
	assert.Equal(t, 404, httpCode(err))

	// 非法 id → 400。
	_, err = s.OpsAlertHandle(ctx, "abc", &OpsAlertHandleRequest{Action: "acknowledge"})
	require.Error(t, err)
	assert.Equal(t, 400, httpCode(err))
	_, err = s.OpsAlertHandle(ctx, "0", &OpsAlertHandleRequest{Action: "acknowledge"})
	require.Error(t, err)
	assert.Equal(t, 400, httpCode(err))
}

func TestOpsAlertsHandleStatusFilter(t *testing.T) {
	s, db := newAlertHandleService(t)
	seedHandleAlert(t, db, "h-1")
	seedHandleAlert(t, db, "h-2")
	ctx := withUsername(context.Background(), "alice")

	_, err := s.OpsAlertHandle(ctx, "2", &OpsAlertHandleRequest{Action: "ignore", Remark: "dup noise"})
	require.NoError(t, err)

	// 全量：工作流字段随行返回（未流转行 = pending）。
	all, err := s.OpsAlerts(ctx, &OpsAlertsRequest{})
	require.NoError(t, err)
	require.Len(t, all.Alerts, 2)
	bySeverity := map[string]OpsAlert{}
	for _, a := range all.Alerts {
		if a.HandleStatus != "" {
			bySeverity[a.HandleStatus] = a
		}
	}
	assert.Equal(t, "alice", bySeverity["ignored"].HandledBy)

	// 过滤：acknowledged 只剩 0 行，ignored 只剩 1 行。
	filtered, err := s.OpsAlerts(ctx, &OpsAlertsRequest{HandleStatus: "ignored"})
	require.NoError(t, err)
	require.Len(t, filtered.Alerts, 1)
	assert.Equal(t, "dup noise", filtered.Alerts[0].HandledRemark)

	none, err := s.OpsAlerts(ctx, &OpsAlertsRequest{HandleStatus: "acknowledged"})
	require.NoError(t, err)
	assert.Empty(t, none.Alerts)
}
