package approval

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ARCH-001 D2/D4 服务层：重复批准（双击/重试）必须返回 409
// approval_already_reviewed，且不产生第二次续跑/状态翻转。首个请求
// 已成功续跑（ResultKind=sync），第二个请求不得再触发执行。
func TestApproveCov_DuplicateApproveConflicts(t *testing.T) {
	s, ctx := covContinuationEnv(t, "")

	// 第一个请求：正常批准 + 续跑成功。
	first, err := s.Approve(ctx, &ApprovalApproveRequest{ID: "ap-cov"})
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, "approved", first.State)
	assert.True(t, first.Continuation)
	assert.Equal(t, "sync", first.ResultKind)

	// 第二个请求（同一操作员双击）：409 conflict，不续跑、不改状态。
	_, err = s.Approve(ctx, &ApprovalApproveRequest{ID: "ap-cov"})
	require.Error(t, err)
	var ce *errorx.CodeError
	require.True(t, errors.As(err, &ce), "期望 errorx.CodeError，得到 %T", err)
	assert.Equal(t, 409, ce.Code)
	assert.Equal(t, "approval_already_reviewed", ce.ErrorCode())
	assert.Equal(t, "approved", ce.Details["state"])
	assert.Equal(t, "approver", ce.Details["approver"])
}

// 重复拒绝同样 409，且不覆盖首个拒绝的 reason/approver。
func TestApproveCov_DuplicateRejectConflicts(t *testing.T) {
	s, ctx := covContinuationEnv(t, "")
	ctx = context.WithValue(ctx, "username", "reviewer2")

	first, err := s.Reject(ctx, &ApprovalRejectRequest{ID: "ap-cov", Reason: "证据不足"})
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, "rejected", first.State)

	_, err = s.Reject(ctx, &ApprovalRejectRequest{ID: "ap-cov", Reason: "再拒一次"})
	require.Error(t, err)
	var ce *errorx.CodeError
	require.True(t, errors.As(err, &ce), "期望 errorx.CodeError，得到 %T", err)
	assert.Equal(t, 409, ce.Code)
	assert.Equal(t, "approval_already_reviewed", ce.ErrorCode())
	assert.Equal(t, "rejected", ce.Details["state"])

	got, err := s.svcCtx.ApprovalsStore.Get("ap-cov")
	require.NoError(t, err)
	assert.Equal(t, "证据不足", got.Reason, "首个 reason 保持不变")
}

// 批准后拒绝（竞争）：拒绝请求看到 approved 终态 → 409，终态仍 approved。
func TestApproveCov_RejectAfterApproveConflicts(t *testing.T) {
	s, ctx := covContinuationEnv(t, "")

	_, err := s.Approve(ctx, &ApprovalApproveRequest{ID: "ap-cov"})
	require.NoError(t, err)

	_, err = s.Reject(context.WithValue(ctx, "username", "reviewer2"),
		&ApprovalRejectRequest{ID: "ap-cov", Reason: "迟到的拒绝"})
	require.Error(t, err)
	var ce *errorx.CodeError
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, 409, ce.Code)
	assert.Equal(t, "approval_already_reviewed", ce.ErrorCode())
	assert.Equal(t, "approved", ce.Details["state"])
}
