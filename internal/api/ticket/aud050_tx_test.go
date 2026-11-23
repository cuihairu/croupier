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

// aud050TxEnv 组装配有 DB 的工单服务：多步写走真事务（AUD-050）。
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

// 带备注的流转两写（备注 + 状态）同事务：第二步（状态更新）失败时，第一
// 步已落库的备注必须随事务回滚——「有备注无状态变更」的半提交中间态不得
// 出现（AUD-050 回归）。
func TestAud050_TransitionNoteRollsBackWithStatus(t *testing.T) {
	s, svcCtx := aud050TxEnv(t)
	ctx := context.Background()

	resp, err := s.Create(ctx, &CreateRequest{Title: "aud050", Content: "c", Category: "bug"})
	require.NoError(t, err)
	idStr := fmt.Sprint(resp.Id)

	// 注入状态更新失败（仅 Update 链，不影响备注 Create）。
	failV9Updates(svcCtx.DB)

	_, err = s.Transition(ctx, &TransitionRequest{ID: idStr, Status: "in_progress", Note: "aud050-note"})
	require.Error(t, err)

	// 回滚面：状态未变、备注未落库。
	ticket, err := svcCtx.TicketModel.FindOne(ctx, uint(resp.Id))
	require.NoError(t, err)
	assert.Equal(t, dbenum.TicketStatusOpen, ticket.Status, "状态更新失败应整体回滚")
	comments, err := svcCtx.TicketModel.ListComments(ctx, uint(resp.Id))
	require.NoError(t, err)
	assert.Empty(t, comments, "已写入的备注应随回滚消失")
}

// 无注入时带备注流转两写整体提交（对照）。
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
