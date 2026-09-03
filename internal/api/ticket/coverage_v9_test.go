package ticket

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newV9TicketService 构建带全部模型的 service（Bug/Admin/Message 按需注入）。
func newV9TicketService(t *testing.T, withAllModels bool) (*Service, *gorm.DB) {
	t.Helper()
	db := newTicketTestDB(t)
	svcCtx := &svc.ServiceContext{TicketModel: model.NewTicketModel(db)}
	if withAllModels {
		svcCtx.BugModel = model.NewBugModel(db)
		svcCtx.AdminModel = model.NewAdminModel(db)
		svcCtx.MessageModel = model.NewMessageModel(db)
	}
	return NewService(svcCtx), db
}

// createV9Ticket 通过 service 创建一张工单并返回其 ID 字符串。
func createV9Ticket(t *testing.T, s *Service) string {
	t.Helper()
	resp, err := s.Create(context.Background(), &CreateRequest{
		Title: "t", Content: "c", Category: "bug",
	})
	require.NoError(t, err)
	return fmt.Sprint(resp.Ticket.Id)
}

// failV9Updates 令所有 UPDATE 失败。
func failV9Updates(db *gorm.DB) {
	_ = db.Callback().Update().Before("gorm:update").Register("v9_fail_update", func(tx *gorm.DB) {
		_ = tx.AddError(errors.New("forced update failure"))
	})
}

// failV9QueriesOnDest 令 Dest 匹配的查询失败。
func failV9QueriesOnDest(db *gorm.DB, match func(tx *gorm.DB) bool) {
	_ = db.Callback().Query().Before("gorm:query").Register("v9_fail_query", func(tx *gorm.DB) {
		if match(tx) {
			_ = tx.AddError(errors.New("forced query failure"))
		}
	})
}

// failV9QueryAfterUpdate 在第一条 UPDATE 之后令后续查询失败（覆盖更新后回读分支）。
func failV9QueryAfterUpdate(db *gorm.DB) {
	var flagged atomic.Bool
	_ = db.Callback().Update().Before("gorm:update").Register("v9_flag_update", func(tx *gorm.DB) {
		flagged.Store(true)
	})
	_ = db.Callback().Query().Before("gorm:query").Register("v9_fail_after_update", func(tx *gorm.DB) {
		if flagged.Load() {
			_ = tx.AddError(errors.New("forced query failure"))
		}
	})
}

// failV9QueryAfterCreate 在第一条 INSERT 之后令后续查询失败（覆盖创建后回读分支）。
func failV9QueryAfterCreate(db *gorm.DB) {
	var flagged atomic.Bool
	_ = db.Callback().Create().Before("gorm:create").Register("v9_flag_create", func(tx *gorm.DB) {
		flagged.Store(true)
	})
	_ = db.Callback().Query().Before("gorm:query").Register("v9_fail_after_create", func(tx *gorm.DB) {
		if flagged.Load() {
			_ = tx.AddError(errors.New("forced query failure"))
		}
	})
}

func TestHandlerListBadQueryV9(t *testing.T) {
	handler := newTicketHandler(newTicketTestDB(t))

	ctx, rec := newTicketRequest(http.MethodGet, "/api/v1/tickets?page=abc", "")
	handler.List(ctx)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestHandlerListServiceErrorV9(t *testing.T) {
	db := newTicketTestDB(t)
	require.NoError(t, db.Migrator().DropTable("tickets"))
	handler := newTicketHandler(db)

	ctx, rec := newTicketRequest(http.MethodGet, "/api/v1/tickets?page=1&pageSize=10", "")
	handler.List(ctx)
	assert.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
}

func TestHandlerCreateCommentBadJSONV9(t *testing.T) {
	handler := newTicketHandler(newTicketTestDB(t))

	ctx, rec := newTicketRequest(http.MethodPost, "/api/v1/tickets/1/comments", "not-json")
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	handler.CreateComment(ctx)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestHandlerRateBadJSONV9(t *testing.T) {
	handler := newTicketHandler(newTicketTestDB(t))

	ctx, rec := newTicketRequest(http.MethodPost, "/api/v1/tickets/1/rate", "not-json")
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	handler.Rate(ctx)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestHandlerConvertToBugBadJSONV9(t *testing.T) {
	db := newTicketTestDB(t)
	svcCtx := &svc.ServiceContext{
		TicketModel: model.NewTicketModel(db),
		BugModel:    model.NewBugModel(db),
	}
	handler := NewHandler(NewService(svcCtx))

	ctx, rec := newTicketRequest(http.MethodPost, "/api/v1/tickets/1/convert-bug", "not-json")
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	handler.ConvertToBug(ctx)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestServiceListStatusFilterAndErrorV9(t *testing.T) {
	s, db := newV9TicketService(t, false)
	_ = createV9Ticket(t, s)

	// 非法状态过滤值 → -1（model 层 Status<0 表示不过滤，返回全部行）。
	resp, err := s.List(context.Background(), &ListRequest{Page: 1, PageSize: 10, Status: "bogus"})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 1)

	// 合法但无匹配的状态 → 空列表。
	resp, err = s.List(context.Background(), &ListRequest{Page: 1, PageSize: 10, Status: "closed"})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)

	require.NoError(t, db.Migrator().DropTable("tickets"))
	_, err = s.List(context.Background(), &ListRequest{Page: 1, PageSize: 10})
	require.Error(t, err)
}

func TestServiceCreateModelErrorV9(t *testing.T) {
	s, db := newV9TicketService(t, false)
	require.NoError(t, db.Migrator().DropTable("tickets"))

	_, err := s.Create(context.Background(), &CreateRequest{
		Title: "t", Content: "c", Category: "bug",
	})
	require.Error(t, err)
}

func TestServiceGetListCommentsErrorV9(t *testing.T) {
	s, db := newV9TicketService(t, false)
	id := createV9Ticket(t, s)

	require.NoError(t, db.Migrator().DropTable("ticket_comments"))
	_, err := s.Get(context.Background(), &GetRequest{ID: id})
	require.Error(t, err)
}

func TestServiceUpdateAssigneeNotFoundV9(t *testing.T) {
	s, _ := newV9TicketService(t, true)
	id := createV9Ticket(t, s)

	_, err := s.Update(context.Background(), &UpdateRequest{ID: id, Assignee: "ghost"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "处理人账号不存在")
}

func TestServiceUpdateUpdateErrorV9(t *testing.T) {
	s, db := newV9TicketService(t, false)
	id := createV9Ticket(t, s)

	failV9Updates(db)
	_, err := s.Update(context.Background(), &UpdateRequest{ID: id, Title: "new"})
	require.Error(t, err)
}

func TestServiceUpdateFindAfterUpdateErrorV9(t *testing.T) {
	s, db := newV9TicketService(t, false)
	id := createV9Ticket(t, s)

	failV9QueryAfterUpdate(db)
	_, err := s.Update(context.Background(), &UpdateRequest{ID: id, Title: "new"})
	require.Error(t, err)
}

func TestServiceTransitionNoteCommentErrorV9(t *testing.T) {
	s, db := newV9TicketService(t, false)
	id := createV9Ticket(t, s)

	require.NoError(t, db.Migrator().DropTable("ticket_comments"))
	_, err := s.Transition(context.Background(), &TransitionRequest{
		ID: id, Status: "in_progress", Note: "picking up",
	})
	require.Error(t, err)
}

func TestServiceTransitionUpdateErrorV9(t *testing.T) {
	s, db := newV9TicketService(t, false)
	id := createV9Ticket(t, s)

	failV9Updates(db)
	_, err := s.Transition(context.Background(), &TransitionRequest{ID: id, Status: "closed"})
	require.Error(t, err)
}

func TestServiceTransitionFindAfterUpdateErrorV9(t *testing.T) {
	s, db := newV9TicketService(t, false)
	id := createV9Ticket(t, s)

	failV9QueryAfterUpdate(db)
	_, err := s.Transition(context.Background(), &TransitionRequest{ID: id, Status: "closed"})
	require.Error(t, err)
}

func TestServiceGetCommentsErrorV9(t *testing.T) {
	s, db := newV9TicketService(t, false)
	id := createV9Ticket(t, s)

	require.NoError(t, db.Migrator().DropTable("ticket_comments"))
	_, err := s.GetComments(context.Background(), &GetCommentsRequest{TicketID: id})
	require.Error(t, err)
}

func TestServiceCreateCommentErrorsV9(t *testing.T) {
	t.Run("create_comment_fails", func(t *testing.T) {
		s, db := newV9TicketService(t, false)
		id := createV9Ticket(t, s)
		require.NoError(t, db.Migrator().DropTable("ticket_comments"))

		_, err := s.CreateComment(context.Background(), &CreateCommentRequest{
			TicketID: id, Content: "hello",
		})
		require.Error(t, err)
	})

	t.Run("find_after_create_fails", func(t *testing.T) {
		s, db := newV9TicketService(t, false)
		id := createV9Ticket(t, s)
		// CreateComment 无 UPDATE，须在评论 INSERT 之后令查询失败。
		failV9QueryAfterCreate(db)

		_, err := s.CreateComment(context.Background(), &CreateCommentRequest{
			TicketID: id, Content: "hello",
		})
		require.Error(t, err)
	})

	t.Run("list_comments_fails", func(t *testing.T) {
		s, db := newV9TicketService(t, false)
		id := createV9Ticket(t, s)
		// 仅令 ticket_comments 的查询失败，工单查询与评论写入正常。
		failV9QueriesOnDest(db, func(tx *gorm.DB) bool {
			_, ok := tx.Statement.Dest.(*[]model.TicketComment)
			return ok
		})

		_, err := s.CreateComment(context.Background(), &CreateCommentRequest{
			TicketID: id, Content: "hello",
		})
		require.Error(t, err)
	})
}

func TestServiceRateErrorsV9(t *testing.T) {
	s, db := newV9TicketService(t, false)

	t.Run("invalid_id", func(t *testing.T) {
		_, err := s.Rate(context.Background(), &RateRequest{ID: "abc", Rating: 5})
		require.Error(t, err)
	})

	t.Run("not_found", func(t *testing.T) {
		_, err := s.Rate(context.Background(), &RateRequest{ID: "99999", Rating: 5})
		require.Error(t, err)
	})

	t.Run("update_error", func(t *testing.T) {
		id := createV9Ticket(t, s)
		require.NoError(t, db.Model(&model.Ticket{}).Where("id = ?", id).
			Update("status", dbenum.TicketStatusClosed).Error)
		failV9Updates(db)

		_, err := s.Rate(context.Background(), &RateRequest{ID: id, Rating: 5})
		require.Error(t, err)
	})
}

func TestServiceConvertToBugErrorsV9(t *testing.T) {
	s, db := newV9TicketService(t, true)

	t.Run("invalid_id", func(t *testing.T) {
		_, err := s.ConvertToBug(context.Background(), &ConvertToBugRequest{ID: "abc"})
		require.Error(t, err)
	})

	t.Run("create_bug_error", func(t *testing.T) {
		id := createV9Ticket(t, s)
		require.NoError(t, db.Migrator().DropTable("bugs"))

		_, err := s.ConvertToBug(context.Background(), &ConvertToBugRequest{ID: id})
		require.Error(t, err)
	})
}

func TestHelpersEdgeBranchesV9(t *testing.T) {
	// 非法 JSON 的 extra → nil。
	assert.Nil(t, decodeTicketExtra(model.JSON("not-json")))

	// 玩家等级边界收敛。
	assert.Equal(t, 0, sanitizePlayerLevel(-1))
	assert.Equal(t, 0, sanitizePlayerLevel(0))
	assert.Equal(t, 10000, sanitizePlayerLevel(10001))
	assert.Equal(t, 42, sanitizePlayerLevel(42))

	// 无法序列化的 extra → nil。
	assert.Nil(t, encodeTicketExtra(map[string]interface{}{"ch": make(chan int)}))

	// 截断分支。
	assert.Equal(t, "abcd", truncate("abcd", 10))
	assert.Equal(t, "ab…", truncate("abcdef", 3))
}

func TestSendMessageEdgeBranchesV9(t *testing.T) {
	s, _ := newV9TicketService(t, true)
	ctx := context.Background()

	// 收件人为空白 → 直接返回。
	s.sendMessage(ctx, "   ", "ticket.updated", "t", "c", map[string]interface{}{"k": "v"})

	// data 无法编码 → 直接返回。
	s.sendMessage(ctx, "alice", "ticket.updated", "t", "c",
		map[string]interface{}{"ch": make(chan int)})
}
