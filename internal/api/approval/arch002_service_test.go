package approval

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// claimCountingStore 包装真实 MemStore，统计 ClaimDispatch 调用——断言重复
// 批准绝不再进入领取（否则会二次续跑）。
type claimCountingStore struct {
	*approvals.MemStore
	claims   int
	complete int
}

func (s *claimCountingStore) ClaimDispatch(id, fingerprint string) (*approvals.Approval, error) {
	s.claims++
	return s.MemStore.ClaimDispatch(id, fingerprint)
}

func (s *claimCountingStore) CompleteDispatch(id string, token int64, resultKind, taskID string, result []byte) (*approvals.Approval, error) {
	s.complete++
	return s.MemStore.CompleteDispatch(id, token, resultKind, taskID, result)
}

func newArch002Service(t *testing.T) (*Service, *claimCountingStore) {
	t.Helper()
	store := &claimCountingStore{MemStore: approvals.NewMemStore()}
	_, err := store.Create(&approvals.Approval{
		ID: "ap-arch002", State: "pending", FunctionID: "", Actor: "tester",
	})
	require.NoError(t, err)
	s := NewService(&svc.ServiceContext{ApprovalsStore: store})
	return s, store
}

// ARCH-002 D7 服务层：正常批准恰好一次领取、恰好一次结果回写。
func TestArch002_ServiceApproveClaimsAndCompletesOnce(t *testing.T) {
	s, store := newArch002Service(t)
	ctx := context.WithValue(context.Background(), "username", "admin")

	resp, err := s.Approve(ctx, &ApprovalApproveRequest{ID: "ap-arch002"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "approved", resp.State)
	assert.Equal(t, 1, store.claims, "exactly one dispatch claim")
	assert.Equal(t, 1, store.complete, "exactly one result write-back")

	got, err := store.Get("ap-arch002")
	require.NoError(t, err)
	assert.Equal(t, "dispatched", got.ClaimStatus)
	assert.True(t, got.ClaimToken > 0, "claim token monotonic")
}

// ARCH-002 D7 服务层：重复批准（记录已终态）→ 409，绝不再次领取/续跑。
func TestArch002_ServiceDuplicateApproveConflictsNoSecondClaim(t *testing.T) {
	s, store := newArch002Service(t)
	ctx := context.WithValue(context.Background(), "username", "admin")

	_, err := s.Approve(ctx, &ApprovalApproveRequest{ID: "ap-arch002"})
	require.NoError(t, err)
	before := store.claims

	_, err = s.Approve(ctx, &ApprovalApproveRequest{ID: "ap-arch002"})
	require.Error(t, err)
	var ce *errorx.CodeError
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, 409, ce.Code)
	assert.Equal(t, "approval_already_reviewed", ce.ErrorCode())
	assert.Equal(t, before, store.claims, "duplicate approve must not claim again")
}
