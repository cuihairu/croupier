// 覆盖率巡检第十九轮（wt-api）：service.go recordApprovalAudit 的
// details 富化两翼收口——resultKind（331-332）与 taskId（334-335）：
// 带续跑任务的批准记录须把 resultKind/taskId 写进审计，operation-logs
// 按 kind 过滤审批动作时详情页才拿得到结构化上下文（reason 翼既有
// Approve/Reject 用例已覆盖）。同包直调 + 内存审计断言落库形态。
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
