package approval

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ARCH-001 D2/D4 service layer: a duplicate approval (double-click/retry)
// must return 409 approval_already_reviewed without a second continuation
// or a state flip. The first request has already continued successfully
// (ResultKind=sync), so the second request must not trigger execution.
func TestApproveCov_DuplicateApproveConflicts(t *testing.T) {
	s, ctx := covContinuationEnv(t, "")

	// First request: normal approve + successful continuation.
	first, err := s.Approve(ctx, &ApprovalApproveRequest{ID: "ap-cov"})
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, "approved", first.State)
	assert.True(t, first.Continuation)
	assert.Equal(t, "sync", first.ResultKind)

	// Second request (same operator double-click): 409 conflict, no
	// continuation and no state change.
	_, err = s.Approve(ctx, &ApprovalApproveRequest{ID: "ap-cov"})
	require.Error(t, err)
	var ce *errorx.CodeError
	require.True(t, errors.As(err, &ce), "期望 errorx.CodeError，得到 %T", err)
	assert.Equal(t, 409, ce.Code)
	assert.Equal(t, "approval_already_reviewed", ce.ErrorCode())
	assert.Equal(t, "approved", ce.Details["state"])
	assert.Equal(t, "approver", ce.Details["approver"])
}

// A duplicate reject is likewise 409 and does not overwrite the reason or
// approver of the first rejection.
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

// Reject after approve (race): the reject request sees the approved
// terminal state → 409, and the terminal state stays approved.
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
