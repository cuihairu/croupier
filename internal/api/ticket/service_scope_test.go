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

// Regression for the row-level scope guard on a single ticket (AUD-COV-001
// P1: Get/Update/Delete/Transition/comments/bug-link read and mutate
// directly by id and could cross game/env boundaries; list and create were
// already filtered/attributed by scope in #21, so this covers the
// single-row surface). The scope is simulated by injecting it via
// WithGameScope.
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

	// The row was neither deleted nor tampered with
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
