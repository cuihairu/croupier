package approval

import (
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BUG-024 回归：审批终态必须写入哈希链审计（audit_records），且 actor_id
// 是审批人而非申请人。审批详情页「查看审计（批准/拒绝）」按
// actor=<审批人>&kind=approval_approve/reject 跳转 operation-logs，
// 源头没有该行或 actor 记成申请人，跳过去必然查无此人/张冠李戴。
func newAuditChainTestService(t *testing.T) (*Service, *audit.AuditService) {
	t.Helper()
	auditSvc := audit.NewAuditService(audit.NewInMemoryAuditStore(), nil)
	store := approvals.NewMemStore()
	_, err := store.Create(&approvals.Approval{
		// 不带 FunctionID：续跑链路提前返回，本组用例只验证审计语义
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

	// 前端「查看审计（批准）」的查询形态：actor=审批人 + kind=approval_approve
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
	// 申请人 / 审批人两个身份都要可读，且互不混淆
	assert.Equal(t, "gm01", rec.Details["actor"])
	assert.Equal(t, "reviewer01", rec.Details["operator"])
	assert.Equal(t, "reviewer01", rec.Actor.ID)

	// 修复前实况：audit_records 从无 approval.approved 行；即便有人写，
	// 也绝不允许把 actor 记成申请人（跳到申请人名下查 approve 是错误数据）
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

// 审计是旁路：AuditService 缺席时审批主流程不受影响。
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
