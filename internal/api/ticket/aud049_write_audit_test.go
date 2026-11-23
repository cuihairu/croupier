package ticket

import (
	"context"
	"fmt"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aud049TicketCtx 携带操作者用户名 + 匹配 scope 的成功路径上下文。
func aud049TicketCtx(gameID, env, username string) context.Context {
	ctx := ticketScopeCtx(gameID, env)
	return context.WithValue(ctx, "username", username)
}

// aud049TicketEnv 组装带审计内存存储的工单服务（AUD-049），附一条 bug 行
// 供关联用例使用。
func aud049TicketEnv(t *testing.T) (*Service, *audit.InMemoryAuditStore, uint) {
	t.Helper()
	db := newTicketTestDB(t)
	bug := &model.Bug{Title: "aud049-bug"}
	require.NoError(t, db.Create(bug).Error)
	store := audit.NewInMemoryAuditStore()
	svcCtx := &svc.ServiceContext{
		TicketModel:  model.NewTicketModel(db),
		BugModel:     model.NewBugModel(db),
		AuditService: audit.NewAuditService(store, nil),
	}
	return NewService(svcCtx), store, bug.ID
}

// aud049TicketRecords 取同事件类型的全部留痕。注意 InMemoryAuditStore.List
// 底层是 map（遍历顺序随机），断言一律按 outcome 子集过滤，不依赖顺序。
func aud049TicketRecords(t *testing.T, store *audit.InMemoryAuditStore, event audit.AuditEventType) []*audit.AuditRecord {
	t.Helper()
	records, _, err := store.List(audit.AuditFilter{EventType: []audit.AuditEventType{event}}, audit.AuditPage{Page: 1, PageSize: 50})
	require.NoError(t, err)
	return records
}

func aud049ByOutcome(records []*audit.AuditRecord, outcome string) []*audit.AuditRecord {
	var out []*audit.AuditRecord
	for _, r := range records {
		if r.Outcome == outcome {
			out = append(out, r)
		}
	}
	return out
}

// 创建/更新/删除/流转/评论/bug 关联成功留痕；跨 scope 的写操作落 denied
// 且与 success 共用同一事件类型（差异只在 outcome）。
func TestAud049_TicketWriteAudit(t *testing.T) {
	s, store, bugID := aud049TicketEnv(t)
	okCtx := aud049TicketCtx("game-a", "prod", "aud049er")
	foreign := ticketScopeCtx("game-b", "prod")

	resp, err := s.Create(okCtx, &CreateRequest{
		Title: "aud049-ticket", Content: "c", Category: "bug",
		GameId: "game-a", Env: "prod", Assignee: "",
	})
	require.NoError(t, err)
	idStr := fmt.Sprint(resp.Id)

	creates := aud049TicketRecords(t, store, audit.EventTicketCreate)
	require.Len(t, creates, 1)
	assert.Equal(t, "success", creates[0].Outcome)
	assert.Equal(t, idStr, creates[0].Resource.ID)
	assert.Equal(t, "ticket", creates[0].Resource.Type)
	assert.Equal(t, "aud049er", creates[0].Actor.ID, "成功留痕应携带操作者用户名")
	assert.Equal(t, "aud049-ticket", creates[0].Details["title"])
	assert.Equal(t, "game-a", creates[0].Details["gameId"])

	// 跨 scope 更新/流转/评论 → denied。
	_, err = s.Update(foreign, &UpdateRequest{ID: idStr, Title: "越权改"})
	require.Error(t, err)
	_, err = s.Transition(foreign, &TransitionRequest{ID: idStr, Status: "in_progress"})
	require.Error(t, err)
	_, err = s.CreateComment(foreign, &CreateCommentRequest{TicketID: idStr, Content: "sneak"})
	require.Error(t, err)

	// 同 scope：更新与流转成功，success 流转的 "to" 记录落库后的枚举状态。
	_, err = s.Update(okCtx, &UpdateRequest{ID: idStr, Title: "aud049-renamed"})
	require.NoError(t, err)
	_, err = s.Transition(okCtx, &TransitionRequest{ID: idStr, Status: "in_progress"})
	require.NoError(t, err)
	_, err = s.CreateComment(okCtx, &CreateCommentRequest{TicketID: idStr, Content: "handled"})
	require.NoError(t, err)

	updates := aud049ByOutcome(aud049TicketRecords(t, store, audit.EventTicketUpdate), "denied")
	require.Len(t, updates, 1)
	updates = aud049ByOutcome(aud049TicketRecords(t, store, audit.EventTicketUpdate), "success")
	require.Len(t, updates, 1)

	transitions := aud049TicketRecords(t, store, audit.EventTicketTransition)
	require.Len(t, transitions, 2)
	deniedTransitions := aud049ByOutcome(transitions, "denied")
	require.Len(t, deniedTransitions, 1)
	assert.Equal(t, "in_progress", deniedTransitions[0].Details["to"], "denied 的 to 记录请求目标状态")
	okTransitions := aud049ByOutcome(transitions, "success")
	require.Len(t, okTransitions, 1)
	assert.Equal(t, dbenum.TicketStatusInProgress, okTransitions[0].Details["to"], "success 的 to 记录落库后的枚举状态")

	comments := aud049TicketRecords(t, store, audit.EventTicketComment)
	require.Len(t, comments, 2)
	assert.Len(t, aud049ByOutcome(comments, "denied"), 1)
	assert.Len(t, aud049ByOutcome(comments, "success"), 1)

	err = s.LinkBug(okCtx, uint(resp.Id), &TicketBugLinkRequest{BugID: bugID})
	require.NoError(t, err)
	err = s.UnlinkBug(okCtx, uint(resp.Id), bugID)
	require.NoError(t, err)
	links := aud049TicketRecords(t, store, audit.EventTicketBugLink)
	require.Len(t, links, 1)
	assert.Equal(t, "success", links[0].Outcome)
	assert.Equal(t, bugID, links[0].Details["bugId"])
	unlinks := aud049TicketRecords(t, store, audit.EventTicketBugUnlink)
	require.Len(t, unlinks, 1)
	assert.Equal(t, "success", unlinks[0].Outcome)

	err = s.Delete(okCtx, &DeleteRequest{ID: idStr})
	require.NoError(t, err)
	deletes := aud049TicketRecords(t, store, audit.EventTicketDelete)
	require.Len(t, deletes, 1)
	assert.Equal(t, "success", deletes[0].Outcome)
}
