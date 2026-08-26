package notify_test

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	notifyservice "github.com/cuihairu/croupier/internal/service/notify"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupNotify(t *testing.T) (*notifyservice.Service, *gorm.DB) {
	t.Helper()
	settings.ResetForTest()
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/notify.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	store := model.NewPlatformSettingModel(db)
	layered := settings.InitLayered(context.Background(), &settings.ConfigInput{}, store)
	return notifyservice.New(layered, model.NewMessageModel(db)), db
}

func TestDispatch_InAppDefault(t *testing.T) {
	svc, db := setupNotify(t)

	svc.Dispatch(context.Background(), notifyservice.Event{
		Type:       "approval.created",
		Title:      "新的审批请求",
		Message:    "玩家查询等待审批",
		Recipients: []string{"alice", "bob", "alice", ""},
	})

	var msgs []model.Message
	require.NoError(t, db.Find(&msgs).Error)
	// 去重 + 空接收人剔除。
	assert.Len(t, msgs, 2)
	to := map[string]bool{}
	for _, m := range msgs {
		to[m.To] = true
		assert.Equal(t, "approval.created", m.Type)
	}
	assert.True(t, to["alice"] && to["bob"])
}

func TestDispatch_InAppDisabled(t *testing.T) {
	svc, db := setupNotify(t)
	// 关闭站内信。
	require.NoError(t, model.NewPlatformSettingModel(db).
		Set(context.Background(), settings.KeyNotifyInAppEnabled, jsonFalse, "tester"))
	settings.Current().Reload(context.Background(), model.NewPlatformSettingModel(db))

	svc.Dispatch(context.Background(), notifyservice.Event{Title: "t", Recipients: []string{"alice"}})
	var count int64
	require.NoError(t, db.Model(&model.Message{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestDispatch_NilLayeredAndModel(t *testing.T) {
	// 全 nil 安全：不 panic、无副作用。
	svc := notifyservice.New(nil, nil)
	assert.NotPanics(t, func() {
		svc.Dispatch(context.Background(), notifyservice.Event{Title: "t", Recipients: []string{"a"}})
	})
}

func TestDispatch_ExternalChannelsUnconfigured(t *testing.T) {
	svc, db := setupNotify(t)
	// 未配置任何外部渠道：只写站内信，无 HTTP 报错。
	svc.Dispatch(context.Background(), notifyservice.Event{Title: "t", Recipients: []string{"a"}})
	var count int64
	require.NoError(t, db.Model(&model.Message{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestDispatch_EventTypeFallback(t *testing.T) {
	svc, db := setupNotify(t)
	svc.Dispatch(context.Background(), notifyservice.Event{Title: "t", Recipients: []string{"a"}})
	var msg model.Message
	require.NoError(t, db.First(&msg).Error)
	assert.Equal(t, "system", msg.Type)
}

var jsonFalse = []byte("false")
