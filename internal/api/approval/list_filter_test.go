package approval

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newApprovalListTestService(t *testing.T) *Service {
	t.Helper()
	return NewService(&svc.ServiceContext{ApprovalsStore: approvals.NewMemStore()})
}

func mustCreateApproval(t *testing.T, s *Service, id, actor, functionID, state string) {
	t.Helper()
	_, err := s.svcCtx.ApprovalsStore.Create(&approvals.Approval{
		ID:         id,
		Actor:      actor,
		FunctionID: functionID,
		GameID:     "game-1",
		Env:        "prod",
		State:      state,
	})
	require.NoError(t, err)
}

func actorCtx(username string) context.Context {
	return context.WithValue(context.Background(), "username", username)
}

func TestListFiltersByActor(t *testing.T) {
	s := newApprovalListTestService(t)
	mustCreateApproval(t, s, "ap-1", "alice", "mail.send", "pending")
	mustCreateApproval(t, s, "ap-2", "bob", "mail.send", "pending")
	mustCreateApproval(t, s, "ap-3", "alice", "player.ban", "approved")

	resp, err := s.List(actorCtx("admin"), &ApprovalsListRequest{Actor: "alice"})
	require.NoError(t, err)
	assert.Len(t, resp.Approvals, 2)
	for _, item := range resp.Approvals {
		assert.Equal(t, "alice", item.Actor)
	}
}

func TestListFiltersByFunctionIDAndStatus(t *testing.T) {
	s := newApprovalListTestService(t)
	mustCreateApproval(t, s, "ap-1", "alice", "mail.send", "pending")
	mustCreateApproval(t, s, "ap-2", "bob", "mail.send", "approved")
	mustCreateApproval(t, s, "ap-3", "alice", "player.ban", "pending")

	resp, err := s.List(actorCtx("admin"), &ApprovalsListRequest{
		FunctionID: "mail.send",
		Status:     "pending",
	})
	require.NoError(t, err)
	assert.Len(t, resp.Approvals, 1)
	assert.Equal(t, "ap-1", resp.Approvals[0].ID)
}

func TestListMineOverridesActorAndFiltersByCurrentUser(t *testing.T) {
	s := newApprovalListTestService(t)
	mustCreateApproval(t, s, "ap-1", "alice", "mail.send", "pending")
	mustCreateApproval(t, s, "ap-2", "bob", "mail.send", "pending")

	// mine=true 时忽略请求中的 actor，强制按当前登录用户过滤
	resp, err := s.List(actorCtx("bob"), &ApprovalsListRequest{Mine: true, Actor: "alice"})
	require.NoError(t, err)
	require.Len(t, resp.Approvals, 1)
	assert.Equal(t, "ap-2", resp.Approvals[0].ID)
	assert.Equal(t, "bob", resp.Approvals[0].Actor)
}

func TestListMineWithoutUsernameReturnsEmpty(t *testing.T) {
	s := newApprovalListTestService(t)
	mustCreateApproval(t, s, "ap-1", "alice", "mail.send", "pending")

	resp, err := s.List(context.Background(), &ApprovalsListRequest{Mine: true})
	require.NoError(t, err)
	assert.Empty(t, resp.Approvals)
	assert.Equal(t, int64(0), resp.Total)
}

func TestCurrentApprovalActor(t *testing.T) {
	assert.Empty(t, currentApprovalActor(context.Background()))
	assert.Empty(t, currentApprovalActor(context.WithValue(context.Background(), "username", "  ")))
	assert.Equal(t, "alice", currentApprovalActor(actorCtx(" alice ")))
}
