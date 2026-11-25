package approval

import (
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BUG-024 regression: an approval's terminal state must be written to the
// hash-chained audit (audit_records), with actor_id set to the approver
// rather than the applicant. The approval detail page's "view audit
// (approve/reject)" links to operation-logs with actor=<approver>&
// kind=approval_approve/reject, so a missing source row or an actor recorded
// as the applicant makes that jump show nobody or the wrong person.
func newAuditChainTestService(t *testing.T) (*Service, *audit.AuditService) {
	t.Helper()
	auditSvc := audit.NewAuditService(audit.NewInMemoryAuditStore(), nil)
	store := approvals.NewMemStore()
	_, err := store.Create(&approvals.Approval{
		// No FunctionID: the continuation path returns early, so this group of
		// cases only verifies audit semantics
		ID: "ap-audit-1", State: "pending", Actor: "gm01",
		GameID: "game-1", Env: "prod",
	})
	require.NoError(t, err)
	return NewService(&svc.ServiceContext{
		ApprovalsStore: store,
		AuditService:   auditSvc,
	}), auditSvc
}

func TestApproveWritesAuditChainWithApproverAsActor(t *testing.T) {
	s, auditSvc := newAuditChainTestService(t)

	_, err := s.Approve(approvalCtx("reviewer01"), &ApprovalApproveRequest{ID: "ap-audit-1"})
	require.NoError(t, err)

	// The query shape behind the frontend's "view audit (approve)" link:
	// actor=approver + kind=approval_approve
	items, total, err := auditSvc.List(audit.AuditFilter{
		EventType:   []audit.AuditEventType{audit.EventApprovalApproved},
		ActorID:     "reviewer01",
		GameID:      "game-1",
		Environment: "prod",
	}, audit.AuditPage{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total, "审批动作必须落审计链且 actor=审批人")
	rec := items[0]
	assert.Equal(t, "approval", rec.Resource.Type)
	assert.Equal(t, "ap-audit-1", rec.Resource.ID)
	// Both identities, applicant and approver, must be readable without being
	// confused with each other
	assert.Equal(t, "gm01", rec.Details["actor"])
	assert.Equal(t, "reviewer01", rec.Details["operator"])
	assert.Equal(t, "reviewer01", rec.Actor.ID)

	// Pre-fix reality: audit_records never had an approval.approved row; and
	// even if someone wrote one, the actor must never be recorded as the
	// applicant (jumping to an approve row under the applicant is bad data)
	_, applicantTotal, err := auditSvc.List(audit.AuditFilter{
		EventType: []audit.AuditEventType{audit.EventApprovalApproved},
		ActorID:   "gm01",
	}, audit.AuditPage{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, 0, applicantTotal, "申请人名下不得出现 approval.approved 行")
}

func TestRejectWritesAuditChainWithReason(t *testing.T) {
	s, auditSvc := newAuditChainTestService(t)

	resp, err := s.Reject(approvalCtx("reviewer02"), &ApprovalRejectRequest{ID: "ap-audit-1", Reason: "证据不足"})
	require.NoError(t, err)
	assert.Equal(t, "rejected", resp.State)

	items, total, err := auditSvc.List(audit.AuditFilter{
		EventType:   []audit.AuditEventType{audit.EventApprovalRejected},
		ActorID:     "reviewer02",
		GameID:      "game-1",
		Environment: "prod",
	}, audit.AuditPage{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	rec := items[0]
	assert.Equal(t, "ap-audit-1", rec.Resource.ID)
	assert.Equal(t, "gm01", rec.Details["actor"])
	assert.Equal(t, "证据不足", rec.Details["reason"])
	assert.Equal(t, "reviewer02", rec.Actor.ID)
}

// Auditing is a side channel: without an AuditService the approval main flow
// is unaffected.
func TestApproveToleratesMissingAuditService(t *testing.T) {
	store := approvals.NewMemStore()
	_, err := store.Create(&approvals.Approval{
		ID: "ap-audit-2", State: "pending", Actor: "gm01",
		GameID: "game-1", Env: "prod",
	})
	require.NoError(t, err)
	s := NewService(&svc.ServiceContext{ApprovalsStore: store})

	resp, err := s.Approve(approvalCtx("reviewer01"), &ApprovalApproveRequest{ID: "ap-audit-2"})

	require.NoError(t, err)
	assert.Equal(t, "approved", resp.State)
}
