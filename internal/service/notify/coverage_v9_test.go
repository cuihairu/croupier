// 覆盖目标：dispatchExternal 各渠道投递失败（服务端 500 → sender 报错仅记日志）、
// dispatchInApp 站内信写库失败（数据库已关闭）仅记日志不 panic。
package notify_test

import (
	"context"
	"net/http"
	"testing"

	gsqlite "github.com/glebarez/sqlite"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	notifyservice "github.com/cuihairu/croupier/internal/service/notify"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 企业微信/飞书/通用 webhook 三渠道均返回 500：sender 判定失败，
// Dispatch 只记日志，站内信主流程不受影响。
func TestDispatchExternalChannelFailuresV9(t *testing.T) {
	svc, db := setupNotify(t)
	cs := newCapturedServer(http.StatusInternalServerError)
	defer cs.srv.Close()

	setNotifySetting(t, db, settings.KeyNotifyWecomURL, `"`+cs.srv.URL+`/wecom"`)
	setNotifySetting(t, db, settings.KeyNotifyFeishuURL, `"`+cs.srv.URL+`/feishu"`)
	setNotifySetting(t, db, settings.KeyNotifyFeishuSecret, `"s"`)
	setNotifySetting(t, db, settings.KeyNotifyWebhookURL, `"`+cs.srv.URL+`/hook"`)
	setNotifySetting(t, db, settings.KeyNotifyWebhookSecret, `"whsec"`)

	assert.NotPanics(t, func() {
		svc.Dispatch(context.Background(), notifyservice.Event{
			Title:      "审批待处理",
			Message:    "玩家查询等待审批",
			Recipients: []string{"alice"},
			Priority:   "high",
		})
	})

	// 三渠道各收到一次投递请求（服务端 500，但请求已发出）。
	assert.Len(t, cs.body("/wecom"), 1)
	assert.Len(t, cs.body("/feishu"), 1)
	assert.Len(t, cs.body("/hook"), 1)

	// 站内信不受外部渠道失败影响。
	var count int64
	require.NoError(t, db.Model(&model.Message{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

// 站内信写库失败（连接池已关闭）：仅记 Warn 日志，不 panic。
func TestDispatchInAppCreateFailureV9(t *testing.T) {
	settings.ResetForTest()
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/notify-v9.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	// layered 为 nil：外部渠道跳过，仅站内信路径生效。
	svc := notifyservice.New(nil, model.NewMessageModel(db))
	assert.NotPanics(t, func() {
		svc.Dispatch(context.Background(), notifyservice.Event{
			Type:       "approval.created",
			Title:      "t",
			Message:    "m",
			Recipients: []string{"alice", "bob"},
		})
	})
}
