package approval

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSelfApprovalTestService(allowSelf bool) (*Service, *approvals.MemStore) {
	store := approvals.NewMemStore()
	_, _ = store.Create(&approvals.Approval{
		// 不带 FunctionID：续跑链路提前返回，本组用例只验证审批状态语义
		ID: "ap-self-1", State: "pending", Actor: "alice",
		GameID: "game-1", Env: "prod",
	})
	return NewService(&svc.ServiceContext{
		ApprovalsStore: store,
		Config:         config.Config{Approval: config.ApprovalConfig{AllowSelfApprove: allowSelf}},
	}), store
}

// approvalCtx 带登录态与 game scope（Approve 成功路径会续跑函数，需 scope）。
func approvalCtx(username string) context.Context {
	return svc.WithGameScope(actorCtx(username), svc.GameScope{GameID: "game-1", Env: "prod"})
}

func TestApproveBlocksSelfApproval(t *testing.T) {
	s, store := newSelfApprovalTestService(false)

	resp, err := s.Approve(approvalCtx("alice"), &ApprovalApproveRequest{ID: "ap-self-1"})

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "self_approval_forbidden")
	// 状态保持 pending，审批人与审批时间不得落库
	current, err := store.Get("ap-self-1")
	require.NoError(t, err)
	assert.Equal(t, "pending", current.State)
	assert.Empty(t, current.Approver)
	assert.Nil(t, current.ReviewedAt)
}

func TestRejectBlocksSelfApproval(t *testing.T) {
	s, store := newSelfApprovalTestService(false)

	_, err := s.Reject(actorCtx("alice"), &ApprovalRejectRequest{ID: "ap-self-1", Reason: "不想发了"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "self_approval_forbidden")
	current, err := store.Get("ap-self-1")
	require.NoError(t, err)
	assert.Equal(t, "pending", current.State)
}

func TestApproveAllowsSelfWhenConfigEnabled(t *testing.T) {
	s, store := newSelfApprovalTestService(true)

	resp, err := s.Approve(approvalCtx("alice"), &ApprovalApproveRequest{ID: "ap-self-1"})

	require.NoError(t, err)
	assert.Equal(t, "approved", resp.State)
	current, err := store.Get("ap-self-1")
	require.NoError(t, err)
	assert.Equal(t, "alice", current.Approver)
	require.NotNil(t, current.ReviewedAt)
}

func TestApproveByOtherPersonSucceeds(t *testing.T) {
	s, store := newSelfApprovalTestService(false)

	resp, err := s.Approve(approvalCtx("bob"), &ApprovalApproveRequest{ID: "ap-self-1"})

	require.NoError(t, err)
	assert.Equal(t, "approved", resp.State)
	current, err := store.Get("ap-self-1")
	require.NoError(t, err)
	assert.Equal(t, "bob", current.Approver)
	require.NotNil(t, current.ReviewedAt)
}

func TestRejectByOtherPersonRecordsReasonApproverAndTime(t *testing.T) {
	s, store := newSelfApprovalTestService(false)

	resp, err := s.Reject(actorCtx("bob"), &ApprovalRejectRequest{ID: "ap-self-1", Reason: "参数越权"})

	require.NoError(t, err)
	assert.Equal(t, "rejected", resp.State)
	current, err := store.Get("ap-self-1")
	require.NoError(t, err)
	assert.Equal(t, "参数越权", current.Reason)
	assert.Equal(t, "bob", current.Approver)
	require.NotNil(t, current.ReviewedAt)
}

func TestApproveRequiresLogin(t *testing.T) {
	s, _ := newSelfApprovalTestService(false)

	_, err := s.Approve(context.Background(), &ApprovalApproveRequest{ID: "ap-self-1"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "登录态")
}

func TestBuildApprovalSummaryReviewedByOther(t *testing.T) {
	now := reviewedAtNow()
	cases := []struct {
		name   string
		record approvals.Approval
		other  bool
	}{
		{"unreviewed", approvals.Approval{Actor: "alice", State: "pending"}, false},
		{"self_reviewed", approvals.Approval{Actor: "alice", Approver: "alice", ReviewedAt: &now}, false},
		{"other_reviewed", approvals.Approval{Actor: "alice", Approver: "Bob", ReviewedAt: &now}, true},
		{"system_reviewed", approvals.Approval{Actor: "alice", Approver: "system", ReviewedAt: &now}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			summary := buildApprovalSummary(&tc.record)
			assert.Equal(t, tc.other, summary.ReviewedByOther)
			if tc.record.ReviewedAt != nil {
				assert.Equal(t, tc.record.Approver, summary.Approver)
				assert.NotEmpty(t, summary.ReviewedAt)
			} else {
				assert.Empty(t, summary.ReviewedAt)
			}
		})
	}
}

func reviewedAtNow() time.Time {
	now := time.Now()
	return now
}
