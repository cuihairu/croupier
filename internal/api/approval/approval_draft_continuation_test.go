// 覆盖目标：ensurePageApprovalStillFresh 的 draft 分支——草稿预览执行产生的
// 审批在续跑时的新鲜度校验。草稿没有可回读的发布快照，故按「契约仍在且版本
// 未变」判定：审批期间函数契约重新注册（版本变化或函数消失）即拒绝续跑。
package approval

import (
	"context"
	"fmt"
	"strings"
	"testing"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/svc"
)

// newDraftApprovalEnv 构造草稿审批续跑的最小环境：仅函数契约表（草稿无快照）。
// version 为空则不写入契约行（模拟函数消失）。
func newDraftApprovalEnv(t *testing.T, version string) *svc.ServiceContext {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	if version != "" {
		require.NoError(t, db.Create(&model.FunctionContract{
			GameID: pageEnvGame, Env: pageEnvEnv, FunctionID: "player.query", Version: version,
			Enabled: true, Capability: dbenum.CapabilityAction, Execution: "sync",
			InputSchema: model.JSON(`{"type":"object"}`),
		}).Error)
	}
	return &svc.ServiceContext{DB: db, ApprovalsStore: approvals.NewMemStore()}
}

func draftApprovalRecord() *approvals.Approval {
	return &approvals.Approval{
		ID: "ap-draft", State: "pending", FunctionID: "player.query",
		GameID: pageEnvGame, Env: pageEnvEnv, Actor: "tester",
		Payload: []byte(`{"keyword":"alice"}`),
		Metadata: map[string]string{
			"pageSnapshotGovernance": "draft",
			"page_key":               "draft--players",
			"binding_id":             "player.list",
			"snapshot_function_ver":  "1.0.0",
		},
	}
}

func TestContinueApprovedFunction_DraftGovernance(t *testing.T) {
	ctx := context.Background()

	t.Run("fresh contract passes then invoke rejected without scope", func(t *testing.T) {
		s := NewService(newDraftApprovalEnv(t, "1.0.0"))
		// 草稿契约版本与审批快照一致 → 校验通过 → 走到 FunctionInvoke →
		// ctx 无 scope 报错，证明校验未拦。
		_, err := s.continueApprovedFunction(ctx, draftApprovalRecord())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "scope")
	})

	t.Run("contract version changed rejects continuation", func(t *testing.T) {
		s := NewService(newDraftApprovalEnv(t, "2.0.0"))
		_, err := s.continueApprovedFunction(ctx, draftApprovalRecord())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "contract version 2.0.0 != snapshot 1.0.0")
	})

	t.Run("function no longer registered rejects continuation", func(t *testing.T) {
		s := NewService(newDraftApprovalEnv(t, ""))
		_, err := s.continueApprovedFunction(ctx, draftApprovalRecord())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no longer registered")
	})

	t.Run("incomplete metadata rejects", func(t *testing.T) {
		s := NewService(newDraftApprovalEnv(t, "1.0.0"))
		rec := draftApprovalRecord()
		rec.FunctionID = "" // 无函数标识 → 只能靠 metadata 兜底，也缺失
		// 直连校验函数：continueApprovedFunction 对空 FunctionID 提前返回 nil。
		err := s.ensurePageApprovalStillFresh(ctx, rec, rec.Metadata)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "draft page approval metadata is incomplete")
	})

	t.Run("contract database unavailable rejects", func(t *testing.T) {
		s := NewService(&svc.ServiceContext{DB: nil, ApprovalsStore: approvals.NewMemStore()})
		_, err := s.continueApprovedFunction(ctx, draftApprovalRecord())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "function contract database unavailable")
	})
}

// ensurePageApprovalStillFresh 只按 pageSnapshotGovernance 分流；draft 标记的
// 大小写与空白不敏感。
func TestEnsurePageApprovalStillFresh_GovernanceDispatch(t *testing.T) {
	s := NewService(newDraftApprovalEnv(t, "1.0.0"))
	rec := draftApprovalRecord()
	rec.Metadata["pageSnapshotGovernance"] = " DRAFT "
	err := s.ensurePageApprovalStillFresh(context.Background(), rec, rec.Metadata)
	require.NoError(t, err)

	// 非页面治理（无标记）不校验，直接通过。
	plain := &approvals.Approval{FunctionID: "player.query", GameID: pageEnvGame, Env: pageEnvEnv}
	require.NoError(t, s.ensurePageApprovalStillFresh(context.Background(), plain, map[string]string{}))
}
