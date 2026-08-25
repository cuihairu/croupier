package ticket

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newNotifyEnv(t *testing.T) (*Service, *model.TicketModel, *model.MessageModel, *model.AdminModel) {
	t.Helper()
	db := newTicketTestDB(t)
	ticketModel := model.NewTicketModel(db)
	messageModel := model.NewMessageModel(db)
	adminModel := model.NewAdminModel(db)
	svcCtx := &svc.ServiceContext{
		TicketModel:  ticketModel,
		MessageModel: messageModel,
		AdminModel:   adminModel,
	}
	return NewService(svcCtx), ticketModel, messageModel, adminModel
}

func countMessages(t *testing.T, mm *model.MessageModel, to, msgType string) int64 {
	t.Helper()
	list, total, err := mm.List(context.Background(), model.ListMessagesOptions{
		To:   to,
		Type: msgType,
	})
	require.NoError(t, err)
	_ = list
	return total
}

func createAdmin(t *testing.T, am *model.AdminModel, username string) {
	t.Helper()
	admin := &model.Admin{Username: username, Nickname: username, Status: 1}
	require.NoError(t, am.Create(context.Background(), admin, "password123"))
}

func withActor(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, "username", username)
}

func TestNotifyAssignee_SendsMessage(t *testing.T) {
	svcTicket, _, mm, am := newNotifyEnv(t)
	createAdmin(t, am, "alice")
	createAdmin(t, am, "bob")

	ticket := &model.Ticket{Title: "t", Category: "payment", Status: dbenum.TicketStatusOpen, Assignee: "alice"}
	require.NoError(t, svcTicket.svcCtx.TicketModel.Create(context.Background(), ticket))

	svcTicket.notifyAssignee(withActor(context.Background(), "bob"), ticket, "alice", "bob")

	assert.EqualValues(t, 1, countMessages(t, mm, "alice", "ticket.assigned"))
	assert.EqualValues(t, 0, countMessages(t, mm, "bob", "ticket.assigned"))
}

func TestNotifyAssignee_SkipsSelfAssignment(t *testing.T) {
	svcTicket, _, mm, _ := newNotifyEnv(t)

	ticket := &model.Ticket{Title: "t", Category: "payment", Status: dbenum.TicketStatusOpen, Assignee: "alice"}
	require.NoError(t, svcTicket.svcCtx.TicketModel.Create(context.Background(), ticket))

	svcTicket.notifyAssignee(withActor(context.Background(), "alice"), ticket, "alice", "alice")
	assert.EqualValues(t, 0, countMessages(t, mm, "alice", "ticket.assigned"))
}

func TestNotifyTicketEvent_SkipsOperatorAndEmptyAssignee(t *testing.T) {
	svcTicket, _, mm, _ := newNotifyEnv(t)

	assigned := &model.Ticket{Title: "a", Category: "c", Status: dbenum.TicketStatusInProgress, Assignee: "alice"}
	require.NoError(t, svcTicket.svcCtx.TicketModel.Create(context.Background(), assigned))
	svcTicket.notifyTicketEvent(withActor(context.Background(), "alice"), assigned, "alice", "title", "body")
	assert.EqualValues(t, 0, countMessages(t, mm, "alice", "ticket.updated"))

	svcTicket.notifyTicketEvent(withActor(context.Background(), "bob"), assigned, "bob", "title", "body")
	assert.EqualValues(t, 1, countMessages(t, mm, "alice", "ticket.updated"))

	unassigned := &model.Ticket{Title: "u", Category: "c", Status: dbenum.TicketStatusOpen}
	require.NoError(t, svcTicket.svcCtx.TicketModel.Create(context.Background(), unassigned))
	svcTicket.notifyTicketEvent(withActor(context.Background(), "bob"), unassigned, "bob", "title", "body")
	// alice keeps her single earlier message; the unassigned ticket adds none.
	assert.EqualValues(t, 1, countMessages(t, mm, "alice", "ticket.updated"))
	assert.EqualValues(t, 1, countMessages(t, mm, "alice", "ticket.updated")+countMessages(t, mm, "bob", "ticket.updated"))
}

func TestValidateAssignee(t *testing.T) {
	svcTicket, _, _, am := newNotifyEnv(t)
	createAdmin(t, am, "alice")
	ctx := context.Background()

	got, err := svcTicket.validateAssignee(ctx, "  alice  ")
	require.NoError(t, err)
	assert.Equal(t, "alice", got)

	got, err = svcTicket.validateAssignee(ctx, "")
	require.NoError(t, err)
	assert.Empty(t, got)

	_, err = svcTicket.validateAssignee(ctx, "ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "处理人账号不存在")
}

func TestCreate_WithAssigneeValidatesAndNotifies(t *testing.T) {
	svcTicket, _, mm, am := newNotifyEnv(t)
	createAdmin(t, am, "alice")

	ctx := withActor(context.Background(), "bob")
	resp, err := svcTicket.Create(ctx, &CreateRequest{
		Title: "help", Content: "stuck", Category: "payment",
		Assignee: "alice",
	})
	require.NoError(t, err)
	assert.Equal(t, "alice", resp.Assignee)
	assert.EqualValues(t, 1, countMessages(t, mm, "alice", "ticket.assigned"))

	_, err = svcTicket.Create(ctx, &CreateRequest{
		Title: "help2", Content: "stuck", Category: "payment",
		Assignee: "ghost",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "处理人账号不存在")
}

func TestUpdate_AssigneeChangeNotifiesOnce(t *testing.T) {
	svcTicket, tm, mm, am := newNotifyEnv(t)
	createAdmin(t, am, "alice")
	createAdmin(t, am, "carol")

	ticket := &model.Ticket{Title: "t", Category: "payment", Status: dbenum.TicketStatusOpen}
	require.NoError(t, tm.Create(context.Background(), ticket))

	ctx := withActor(context.Background(), "bob")
	_, err := svcTicket.Update(ctx, &UpdateRequest{ID: fmt.Sprintf("%d", ticket.ID), Assignee: "alice"})
	require.NoError(t, err)
	assert.EqualValues(t, 1, countMessages(t, mm, "alice", "ticket.assigned"))

	// Re-assign to the same person: no duplicate notification.
	_, err = svcTicket.Update(ctx, &UpdateRequest{ID: fmt.Sprintf("%d", ticket.ID), Assignee: "alice"})
	require.NoError(t, err)
	assert.EqualValues(t, 1, countMessages(t, mm, "alice", "ticket.assigned"))

	// Changing assignee notifies the new one.
	_, err = svcTicket.Update(ctx, &UpdateRequest{ID: fmt.Sprintf("%d", ticket.ID), Assignee: "carol"})
	require.NoError(t, err)
	assert.EqualValues(t, 1, countMessages(t, mm, "carol", "ticket.assigned"))
}

func TestTransition_NotifiesAssignee(t *testing.T) {
	svcTicket, tm, mm, _ := newNotifyEnv(t)
	ticket := &model.Ticket{Title: "t", Category: "payment", Status: dbenum.TicketStatusOpen, Assignee: "alice"}
	require.NoError(t, tm.Create(context.Background(), ticket))

	ctx := withActor(context.Background(), "bob")
	_, err := svcTicket.Transition(ctx, &TransitionRequest{
		ID: fmt.Sprintf("%d", ticket.ID), Status: "in_progress", Note: "开始处理",
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, countMessages(t, mm, "alice", "ticket.updated"))
}

func TestCreateComment_NotifiesAssigneeNotAuthor(t *testing.T) {
	svcTicket, tm, mm, _ := newNotifyEnv(t)
	ticket := &model.Ticket{Title: "t", Category: "payment", Status: dbenum.TicketStatusOpen, Assignee: "alice"}
	require.NoError(t, tm.Create(context.Background(), ticket))

	ctx := withActor(context.Background(), "bob")
	_, err := svcTicket.CreateComment(ctx, &CreateCommentRequest{
		TicketID: fmt.Sprintf("%d", ticket.ID), Content: "已联系玩家",
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, countMessages(t, mm, "alice", "ticket.updated"))
	assert.EqualValues(t, 0, countMessages(t, mm, "bob", "ticket.updated"))
}

func TestTruncateContent(t *testing.T) {
	assert.Equal(t, "abc", truncateContent("  abc  ", 10))
	assert.Equal(t, "ab...", truncateContent("abcdef", 2))
	long := make([]rune, 200)
	for i := range long {
		long[i] = '中'
	}
	got := truncateContent(string(long), 100)
	assert.Equal(t, 103, len([]rune(got)))
	assert.True(t, strings.HasSuffix(got, "..."))
}
