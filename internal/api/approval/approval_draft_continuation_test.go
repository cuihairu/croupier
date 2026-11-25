// Coverage targets: the draft branch of ensurePageApprovalStillFresh -- the
// freshness check at continuation time for approvals created by draft preview
// execution. A draft has no published snapshot to read back, so freshness is
// decided by "the contract is still present and its version is unchanged": if
// the function contract was re-registered while the approval was pending
// (version change or the function gone), continuation is refused.
package approval

import (
	"context"
	"fmt"
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

// newDraftApprovalEnv builds the minimal environment for draft approval
// continuation: only the function contract table (a draft has no snapshot).
// An empty version writes no contract row (simulating the function being gone).
func newDraftApprovalEnv(t *testing.T, version string) *svc.ServiceContext {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:approval_draft_%d?mode=memory&cache=shared", covSharedDBSeq.Add(1))), &gorm.Config{})
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
		// The draft contract version matches the approval snapshot -> the check
		// passes -> reaches FunctionInvoke -> the missing-scope error from the
		// ctx proves the check did not block it.
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
		rec.FunctionID = "" // no function identifier -> only the metadata fallback, which is also missing
		// Call the validation function directly: continueApprovedFunction
		// returns nil early for an empty FunctionID.
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

// ensurePageApprovalStillFresh dispatches solely on pageSnapshotGovernance;
// the draft marker is insensitive to case and surrounding whitespace.
func TestEnsurePageApprovalStillFresh_GovernanceDispatch(t *testing.T) {
	s := NewService(newDraftApprovalEnv(t, "1.0.0"))
	rec := draftApprovalRecord()
	rec.Metadata["pageSnapshotGovernance"] = " DRAFT "
	err := s.ensurePageApprovalStillFresh(context.Background(), rec, rec.Metadata)
	require.NoError(t, err)

	// Not page-governed (no marker): no validation, straight through.
	plain := &approvals.Approval{FunctionID: "player.query", GameID: pageEnvGame, Env: pageEnvEnv}
	require.NoError(t, s.ensurePageApprovalStillFresh(context.Background(), plain, map[string]string{}))
}
