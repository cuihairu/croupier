package message

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newGapfixService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	require.NoError(t, db.AutoMigrate(&model.Admin{}, &model.Role{}, &model.AdminRole{}))
	adminModel := model.NewAdminModel(db)
	require.NoError(t, adminModel.Create(context.Background(), &model.Admin{Username: "boss", Nickname: "Boss"}, "x"))
	return NewService(&svc.ServiceContext{MessageModel: model.NewMessageModel(db), AdminModel: adminModel})
}

// Data 不可 JSON 序列化 → EncodeData 失败 → Broadcast 报 400 语义错误。
func TestBroadcastUnserializableData(t *testing.T) {
	s := newGapfixService(t)
	_, err := s.Broadcast(context.Background(), &BroadcastRequest{
		Audience: "all",
		Type:     "system",
		Title:    "停服通知",
		Content:  "今晚维护",
		Data:     make(chan int),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "序列化消息数据失败")
}

// SSE 事件序列化失败（注入缝隙）→ 静默跳过本帧，不写响应不 panic。
func TestSendMessagesEventMarshalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(&svc.ServiceContext{}), config.SSEConfig{})
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/messages/stream", nil)

	orig := marshalMessagesEvent
	marshalMessagesEvent = func(v interface{}) ([]byte, error) { return nil, errors.New("boom") }
	t.Cleanup(func() { marshalMessagesEvent = orig })

	h.sendMessagesEvent(c, "alice")
	assert.Empty(t, rec.Body.String())
}
