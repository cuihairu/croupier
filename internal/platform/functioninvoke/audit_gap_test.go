package functioninvoke

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
)

// gapNotifier counts write-gap alerts. AuditFunctionInvoke/AuditApprovalCreated
// run AFTER the business side effect, so a strict write failure must NOT
// propagate as a 5xx (which would invite a blind retry of an already-executed
// operation) nor be silently discarded: it is surfaced to the operator through
// ReportWriteGap (WARN + rate-limited alert). This is the ARCH-008 D4 caller
// contract.
type gapNotifier struct {
	mu     sync.Mutex
	alerts int
}

func (g *gapNotifier) NotifyAudit(_ context.Context, _ *audit.AuditRecord) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.alerts++
	return nil
}

func (g *gapNotifier) count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.alerts
}

// failingAuditStore rejects every audit write (simulates strict audit outage).
type failingAuditStore struct {
	audit.InMemoryAuditStore
}

func (f *failingAuditStore) Create(_ *audit.AuditRecord) error {
	return errors.New("audit db down")
}

func gapSvc() *audit.AuditService {
	store := &failingAuditStore{InMemoryAuditStore: *audit.NewInMemoryAuditStore()}
	svc := audit.NewAuditService(store, nil)
	return svc
}

func TestAuditFunctionInvoke_WriteGapSurfaced(t *testing.T) {
	svcCtx := &svc.ServiceContext{AuditService: gapSvc()}
	notifier := &gapNotifier{}
	svcCtx.AuditService.SetNotifier(notifier)

	// 副作用已执行（invokeErr 非空代表函数已运行），审计写失败必须呈现缺口。
	AuditFunctionInvoke(context.Background(), svcCtx, "player.grant",
		&model.Admin{Username: "alice"}, []string{"operator"},
		&policy.Policy{DefaultRiskLevel: "high", RequireAudit: true, Source: "default"}, errors.New("boom"), 12)

	assert.Equal(t, 1, notifier.count(), "write gap after side effect must be surfaced")
}

func TestAuditApprovalCreated_WriteGapSurfaced(t *testing.T) {
	svcCtx := &svc.ServiceContext{AuditService: gapSvc()}
	notifier := &gapNotifier{}
	svcCtx.AuditService.SetNotifier(notifier)

	// 审批请求已创建（副作用完成），审计写失败呈现缺口而非 5xx/静默。
	AuditApprovalCreated(context.Background(), svcCtx, "player.grant",
		&model.Admin{Username: "alice"}, []string{"operator"}, "appr-1",
		&policy.Policy{DefaultRiskLevel: "high", ApprovalWorkflow: "single_admin"})

	assert.Equal(t, 1, notifier.count(), "write gap after approval creation must be surfaced")
}
