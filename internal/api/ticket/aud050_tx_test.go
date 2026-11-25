package ticket

import (
	"context"
	"fmt"
	"testing"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aud050TxEnv assembles a DB-backed ticket service: multi-step writes go
// through a real transaction (AUD-050).
func aud050TxEnv(t *testing.T) (*Service, *svc.ServiceContext) {
	t.Helper()
	db := newTicketTestDB(t)
	svcCtx := &svc.ServiceContext{
		DB:          db,
		TicketModel: model.NewTicketModel(db),
		BugModel:    model.NewBugModel(db),
	}
	return NewService(svcCtx), svcCtx
}

// A transition with a note is a two-write operation (note + status) inside
// one transaction: when the second step (status update) fails, the note
// already landed in the database by the first step must roll back with the
// transaction — the half-committed "note without status change" state must
// never appear (AUD-050 regression).
func TestAud050_TransitionNoteRollsBackWithStatus(t *testing.T) {
	s, svcCtx := aud050TxEnv(t)
	ctx := context.Background()

	resp, err := s.Create(ctx, &CreateRequest{Title: "aud050", Content: "c", Category: "bug"})
	require.NoError(t, err)
	idStr := fmt.Sprint(resp.Id)

	// Inject a status-update failure (only the Update chain; note Create is unaffected).
	failV9Updates(svcCtx.DB)

	_, err = s.Transition(ctx, &TransitionRequest{ID: idStr, Status: "in_progress", Note: "aud050-note"})
	require.Error(t, err)

	// Rollback surface: status unchanged, note not landed in the database.
	ticket, err := svcCtx.TicketModel.FindOne(ctx, uint(resp.Id))
	require.NoError(t, err)
	assert.Equal(t, dbenum.TicketStatusOpen, ticket.Status, "状态更新失败应整体回滚")
	comments, err := svcCtx.TicketModel.ListComments(ctx, uint(resp.Id))
	require.NoError(t, err)
	assert.Empty(t, comments, "已写入的备注应随回滚消失")
}

// Without injection the two writes of a note-carrying transition commit as a
// whole (control case).
func TestAud050_TransitionNoteCommitsAtomically(t *testing.T) {
	s, svcCtx := aud050TxEnv(t)
	ctx := context.Background()

	resp, err := s.Create(ctx, &CreateRequest{Title: "aud050-ok", Content: "c", Category: "bug"})
	require.NoError(t, err)
	idStr := fmt.Sprint(resp.Id)

	_, err = s.Transition(ctx, &TransitionRequest{ID: idStr, Status: "in_progress", Note: "picked up"})
	require.NoError(t, err)

	ticket, err := svcCtx.TicketModel.FindOne(ctx, uint(resp.Id))
	require.NoError(t, err)
	assert.Equal(t, dbenum.TicketStatusInProgress, ticket.Status)
	comments, err := svcCtx.TicketModel.ListComments(ctx, uint(resp.Id))
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assert.Contains(t, comments[0].Content, "picked up")
}
