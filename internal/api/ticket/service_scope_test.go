package ticket

import (
	"context"
	"fmt"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 单条工单行级作用域守卫回归（AUD-COV-001 P1：Get/Update/Delete/Transition/
// 评论/bug 关联按 id 直查直改可跨 game/env 越权；列表与创建在 #21 已按
// scope 过滤/归属，此处补齐单条面）。作用域经 WithGameScope 模拟注入。
func ticketScopeCtx(gameID, env string) context.Context {
	return svc.WithGameScope(context.Background(), svc.GameScope{GameID: gameID, Env: env})
}

func newScopeTicketService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db := newTicketTestDB(t)
	return NewService(&svc.ServiceContext{
		TicketModel: model.NewTicketModel(db),
		BugModel:    model.NewBugModel(db),
	}), db
}

func seedScopedTicket(t *testing.T, db *gorm.DB, gameID, env string) uint {
	t.Helper()
	m := model.NewTicketModel(db)
	row := &model.Ticket{
		Title:  "scope-probe",
		Status: dbenum.TicketStatusOpen,
		GameID: gameID,
		Env:    env,
	}
	require.NoError(t, m.Create(context.Background(), row))
	return row.ID
}

func assertPermissionDenied(t *testing.T, err error) *errorx.CodeError {
	t.Helper()
	require.Error(t, err)
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok, "expected *errorx.CodeError, got %T", err)
	assert.Contains(t, codeErr.Message, "无权访问该工单")
	return codeErr
}

func TestTicketGetRejectsForeignScope(t *testing.T) {
	s, db := newScopeTicketService(t)
	id := seedScopedTicket(t, db, "game-a", "prod")

	_, err := s.Get(ticketScopeCtx("game-b", "prod"), &GetRequest{ID: fmt.Sprintf("%d", id)})
	assertPermissionDenied(t, err)

	got, err := s.Get(ticketScopeCtx("game-a", "prod"), &GetRequest{ID: fmt.Sprintf("%d", id)})
	require.NoError(t, err)
	assert.EqualValues(t, id, got.Ticket.Id)
}

func TestTicketUpdateDeleteRejectForeignScope(t *testing.T) {
	s, db := newScopeTicketService(t)
	id := seedScopedTicket(t, db, "game-a", "prod")
	strID := fmt.Sprintf("%d", id)

	_, err := s.Update(ticketScopeCtx("game-b", "prod"), &UpdateRequest{ID: strID, Title: "hijacked"})
	assertPermissionDenied(t, err)

	err = s.Delete(ticketScopeCtx("game-a", "stage"), &DeleteRequest{ID: strID})
	assertPermissionDenied(t, err)

	// 行未被删除/篡改
	got, err := s.Get(ticketScopeCtx("game-a", "prod"), &GetRequest{ID: strID})
	require.NoError(t, err)
	assert.Equal(t, "scope-probe", got.Ticket.Title)
}

func TestTicketTransitionCommentsBugsRejectForeignScope(t *testing.T) {
	s, db := newScopeTicketService(t)
	id := seedScopedTicket(t, db, "game-a", "prod")
	strID := fmt.Sprintf("%d", id)

	_, err := s.Transition(ticketScopeCtx("game-b", "prod"), &TransitionRequest{ID: strID, Status: "resolved"})
	assertPermissionDenied(t, err)

	_, err = s.GetComments(ticketScopeCtx("game-b", "prod"), &GetCommentsRequest{TicketID: strID})
	assertPermissionDenied(t, err)

	_, err = s.CreateComment(ticketScopeCtx("game-b", "prod"), &CreateCommentRequest{TicketID: strID, Content: "sneak"})
	assertPermissionDenied(t, err)

	_, err = s.ListBugs(ticketScopeCtx("game-b", "prod"), id)
	assertPermissionDenied(t, err)

	err = s.LinkBug(ticketScopeCtx("game-b", "prod"), id, &TicketBugLinkRequest{BugID: 1})
	assertPermissionDenied(t, err)

	err = s.UnlinkBug(ticketScopeCtx("game-b", "prod"), id, 1)
	assertPermissionDenied(t, err)
}

func TestTicketTransitionSameScopeStillWorks(t *testing.T) {
	s, db := newScopeTicketService(t)
	id := seedScopedTicket(t, db, "game-a", "prod")

	out, err := s.Transition(ticketScopeCtx("game-a", "prod"), &TransitionRequest{
		ID:     fmt.Sprintf("%d", id),
		Status: "resolved",
	})
	require.NoError(t, err)
	assert.NotNil(t, out.Ticket)
}
