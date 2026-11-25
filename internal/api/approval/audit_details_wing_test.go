// Coverage patrol round 19 (wt-api): closing out the two wings of the
// details enrichment in recordApprovalAudit in service.go — resultKind
// (331-332) and taskId (334-335): an approval record with a continuation
// task must land resultKind/taskId in the audit log so the detail page
// has structured context when operation-logs filters approval actions by
// kind (the reason wing is already covered by the existing Approve/Reject
// cases). Same-package direct call plus in-memory audit assertions on the
// shape landed in the database.
package approval

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordApprovalAudit_ResultKindAndTaskIDDetails(t *testing.T) {
	auditStore := audit.NewInMemoryAuditStore()
	service := NewService(&svc.ServiceContext{AuditService: audit.NewAuditService(auditStore, nil)})

	record := &approvals.Approval{
		ID:         "appr-r19",
		State:      "approved",
		FunctionID: "player.grant",
		GameID:     "demo-game",
		Env:        "prod",
		Actor:      "requester",
		Reason:     "补发确认",
		ResultKind: "succeeded",
		TaskID:     "task-r19-1",
	}
	service.recordApprovalAudit(context.Background(), audit.EventApprovalApproved, "approver", record)

	records, total, err := auditStore.List(audit.AuditFilter{}, audit.AuditPage{PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	d := records[0].Details
	assert.Equal(t, "appr-r19", d["approvalId"])
	assert.Equal(t, "player.grant", d["functionId"])
	assert.Equal(t, "补发确认", d["reason"], "reason 富化翼（既有口径回归）")
	assert.Equal(t, "succeeded", d["resultKind"], "resultKind 富化翼")
	assert.Equal(t, "task-r19-1", d["taskId"], "taskId 富化翼")
}
