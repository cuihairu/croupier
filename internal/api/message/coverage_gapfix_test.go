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

// Data that cannot be JSON serialized -> EncodeData fails -> Broadcast reports
// a 400-class semantic error.
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

// SSE event serialization fails (via the injection seam) -> the frame is
// skipped silently, with no response write and no panic.
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
