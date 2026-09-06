// 覆盖目标：Send/Broadcast 的 EncodeData 与 Create 错误、Read 的 MarkRead
// 与二次 FindOne 错误、Stream 的 Recent 错误、Broadcast 受众解析错误与
// 分页第二页、SSE ticker 循环与 sendMessagesEvent 错误分支。
package message

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func injectMessageCallback(t *testing.T, db *gorm.DB, op string, fn func(*gorm.DB)) {
	t.Helper()
	name := "test:fail_" + op
	switch op {
	case "query":
		require.NoError(t, db.Callback().Query().Before("gorm:query").Register(name, fn))
	case "create":
		require.NoError(t, db.Callback().Create().Before("gorm:create").Register(name, fn))
	case "update":
		require.NoError(t, db.Callback().Update().Before("gorm:update").Register(name, fn))
	default:
		t.Fatalf("unsupported op %q", op)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(name) })
}

func TestService_Send_EncodeDataError(t *testing.T) {
	db := newMessageTestDB(t)
	s := NewService(&svc.ServiceContext{MessageModel: model.NewMessageModel(db)})

	_, err := s.Send(context.Background(), &MessageSendRequest{
		To: "u1", Type: "notice", Content: "hi",
		Data: map[string]interface{}{"nan": math.NaN()},
	})
	require.Error(t, err)
}

func TestService_Send_CreateError(t *testing.T) {
	db := newMessageTestDB(t)
	injectMessageCallback(t, db, "create", func(tx *gorm.DB) {
		_ = tx.AddError(errors.New("forced create failure"))
	})
	s := NewService(&svc.ServiceContext{MessageModel: model.NewMessageModel(db)})

	_, err := s.Send(context.Background(), &MessageSendRequest{
		To: "u1", Type: "notice", Content: "hi",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced create failure")
}

func TestService_Read_MarkReadError(t *testing.T) {
	db := newMessageTestDB(t)
	svcCtx := &svc.ServiceContext{MessageModel: model.NewMessageModel(db)}
	s := NewService(svcCtx)

	msg := &model.Message{To: "u1", Type: "notice", Content: "c"}
	require.NoError(t, svcCtx.MessageModel.Create(context.Background(), msg))

	injectMessageCallback(t, db, "update", func(tx *gorm.DB) {
		_ = tx.AddError(errors.New("forced update failure"))
	})
	_, err := s.Read(context.Background(), "u1", &MessageReadRequest{ID: fmt.Sprint(msg.ID)})
	require.Error(t, err)
}

func TestService_Read_SecondFindOneError(t *testing.T) {
	db := newMessageTestDB(t)
	svcCtx := &svc.ServiceContext{MessageModel: model.NewMessageModel(db)}
	s := NewService(svcCtx)

	msg := &model.Message{To: "u1", Type: "notice", Content: "c"}
	require.NoError(t, svcCtx.MessageModel.Create(context.Background(), msg))

	// 第 1 次 query（FindOne）成功，第 2 次（MarkRead 后的回读）失败。
	var queries atomic.Int32
	injectMessageCallback(t, db, "query", func(tx *gorm.DB) {
		if queries.Add(1) >= 2 {
			_ = tx.AddError(errors.New("forced second query failure"))
		}
	})
	_, err := s.Read(context.Background(), "u1", &MessageReadRequest{ID: fmt.Sprint(msg.ID)})
	require.Error(t, err)
}

func TestService_Stream_RecentError(t *testing.T) {
	db := newMessageTestDB(t)
	require.NoError(t, db.Migrator().DropTable(&model.Message{}))
	s := NewService(&svc.ServiceContext{MessageModel: model.NewMessageModel(db)})

	_, err := s.Stream(context.Background(), "u1", &StreamMessagesRequest{})
	require.Error(t, err)
}

func newBroadcastService(db *gorm.DB) *Service {
	return NewService(&svc.ServiceContext{
		MessageModel: model.NewMessageModel(db),
		AdminModel:   model.NewAdminModel(db),
	})
}

func TestService_Broadcast_NilModels(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	_, err := s.Broadcast(context.Background(), &BroadcastRequest{
		Type: "notice", Title: "t", Content: "c",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "消息服务未初始化")
}

func TestService_Broadcast_RoleWithoutUsers(t *testing.T) {
	s := newBroadcastService(newMessageTestDB(t))
	_, err := s.Broadcast(context.Background(), &BroadcastRequest{
		Audience: "role", Role: "ghost-role", Type: "notice", Title: "t", Content: "c",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "收件人列表为空")
}

func TestService_Broadcast_EncodeDataError(t *testing.T) {
	s := newBroadcastService(newMessageTestDB(t))
	_, err := s.Broadcast(context.Background(), &BroadcastRequest{
		Audience: "users", Usernames: []string{"u1"}, Type: "notice", Title: "t", Content: "c",
		Data: map[string]interface{}{"nan": math.NaN()},
	})
	require.Error(t, err)
}

func TestService_Broadcast_CreateError(t *testing.T) {
	db := newMessageTestDB(t)
	svcCtx := &svc.ServiceContext{
		MessageModel: model.NewMessageModel(db),
		AdminModel:   model.NewAdminModel(db),
	}
	require.NoError(t, svcCtx.AdminModel.Create(context.Background(), &model.Admin{Username: "u1"}, "pass-123456"))

	injectMessageCallback(t, db, "create", func(tx *gorm.DB) {
		_ = tx.AddError(errors.New("forced create failure"))
	})
	_, err := NewService(svcCtx).Broadcast(context.Background(), &BroadcastRequest{
		Audience: "users", Usernames: []string{"u1"}, Type: "notice", Title: "t", Content: "c",
	})
	require.Error(t, err)
}

func TestService_Broadcast_AllUsernamesListError(t *testing.T) {
	db := newMessageTestDB(t)
	svcCtx := &svc.ServiceContext{
		MessageModel: model.NewMessageModel(db),
		AdminModel:   model.NewAdminModel(db),
	}
	injectMessageCallback(t, db, "query", func(tx *gorm.DB) {
		_ = tx.AddError(errors.New("forced admin list failure"))
	})
	_, err := NewService(svcCtx).Broadcast(context.Background(), &BroadcastRequest{
		Audience: "all", Type: "notice", Title: "t", Content: "c",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced admin list failure")
}

// seedBulkAdmins 直插 501 个 active admin（绕过 bcrypt），全部挂 role。
func seedBulkAdmins(t *testing.T, db *gorm.DB, roleName string) {
	t.Helper()
	role := &model.Role{Name: roleName}
	require.NoError(t, db.Create(role).Error)

	admins := make([]model.Admin, 501)
	for i := range admins {
		admins[i] = model.Admin{Username: fmt.Sprintf("bulk-%03d", i), Status: 1}
	}
	require.NoError(t, db.CreateInBatches(&admins, 100).Error)

	links := make([]model.AdminRole, len(admins))
	for i := range admins {
		links[i] = model.AdminRole{AdminID: admins[i].ID, RoleID: role.ID}
	}
	require.NoError(t, db.CreateInBatches(&links, 100).Error)
}

func TestService_AllUsernames_SecondPage(t *testing.T) {
	db := newMessageTestDB(t)
	seedBulkAdmins(t, db, "bulk-role")
	s := newBroadcastService(db)

	names, err := s.allUsernames(context.Background())
	require.NoError(t, err)
	assert.Len(t, names, 501, "allUsernames 应翻页聚合全部 501 个用户")
}

func TestService_UsernamesByRole_SecondPageAndError(t *testing.T) {
	db := newMessageTestDB(t)
	seedBulkAdmins(t, db, "bulk-role")
	s := newBroadcastService(db)

	names, err := s.usernamesByRole(context.Background(), "bulk-role")
	require.NoError(t, err)
	assert.Len(t, names, 501, "usernamesByRole 应翻页聚合全部 501 个用户")

	injectMessageCallback(t, db, "query", func(tx *gorm.DB) {
		_ = tx.AddError(errors.New("forced role list failure"))
	})
	_, err = s.usernamesByRole(context.Background(), "bulk-role")
	require.Error(t, err)
}

func TestHandler_Stream_TickerThenShutdown(t *testing.T) {
	db := newMessageTestDB(t)
	svcCtx := &svc.ServiceContext{MessageModel: model.NewMessageModel(db)}
	require.NoError(t, svcCtx.MessageModel.Create(context.Background(), &model.Message{
		To: "streamer", Type: "notice", Content: "tick",
	}))
	h := NewHandler(NewService(svcCtx), config.SSEConfig{UpdateInterval: 1})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, w := newMessageRequest(http.MethodGet, "/messages/stream", "")
	c.Request = c.Request.WithContext(ctx)
	c.Set("username", "streamer")

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.Stream(c)
	}()

	// 等 1 个 ticker 周期（初始事件 + tick 事件），再断开连接。
	time.Sleep(1500 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stream handler did not exit after context cancel")
	}
	assert.GreaterOrEqual(t, strings.Count(w.Body.String(), "event: messages"), 2,
		"初始事件 + 至少一个 tick 事件")
}

func TestSendMessagesEvent_ServiceError(t *testing.T) {
	db := newMessageTestDB(t)
	require.NoError(t, db.Migrator().DropTable(&model.Message{}))
	h := NewHandler(NewService(&svc.ServiceContext{MessageModel: model.NewMessageModel(db)}), config.SSEConfig{})

	c, w := newMessageRequest(http.MethodGet, "/messages/stream", "")
	c.Set("username", "u1")
	h.sendMessagesEvent(c, "u1")

	assert.Empty(t, w.Body.String(), "service 错误时不应写出 SSE 数据")
}

// 未授权路径（无 username）在 Stream 中短路。
func TestHandler_Stream_Unauthorized(t *testing.T) {
	h := newMessageHandler(newMessageTestDB(t))
	c, w := newMessageRequest(http.MethodGet, "/messages/stream", "")
	h.Stream(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
