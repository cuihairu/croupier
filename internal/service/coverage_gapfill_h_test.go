package service

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AcceptProposal：草稿落库（page_specs 写入）被触发器阻断 →
// "create page draft from proposal" 错误分支。
func TestAcceptProposal_DraftUpsertBlockedV9H(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := newPublishAuthorizedContext(t, db, "gapfill_publisher")
	svc := NewProposalService(db)
	svc.SetProposalDraftPublisher(acceptAndPublishFallback{db: db})

	require.NoError(t, db.Create(&model.FunctionContract{
		GameID: "demo-game", Env: "development", FunctionID: "player.query", Enabled: true, Version: "1.0.0",
	}).Error)

	proposal, err := buildOperationProposal("op--upsert-blocked", "player.query", nil)
	require.NoError(t, err)
	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, proposal))

	// 只阻断 page_specs 的写入：提案快照（page_proposal_versions）仍可写，
	// 失败点精确落在 pageModel.Upsert。
	abortWritesV9(t, db, &model.PageSpec{}, "INSERT", "UPDATE")

	_, err = svc.AcceptProposal(ctx, "demo-game", "development", proposal.ProposalKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create page draft from proposal")
}

// AcceptAndPublishProposal：规范化与快照契约共用 TRIM 精确查询，
// 带空白 function_id 的契约可命中同一行，不再因读写归一漂移失败。
func TestAcceptAndPublish_BuildBindingContractsMissV9H(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := newPublishAuthorizedContext(t, db, "gapfill_contract_publisher")
	svc := NewProposalService(db)
	svc.SetProposalDraftPublisher(acceptAndPublishFallback{db: db})

	require.NoError(t, db.Create(&model.FunctionContract{
		GameID: "demo-game", Env: "development", FunctionID: " player.query", Enabled: true, Version: "1.0.0",
	}).Error)

	proposal, err := buildOperationProposal("op--ws-fid", "player.query", nil)
	require.NoError(t, err)
	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, proposal))

	_, err = svc.AcceptAndPublishProposal(ctx, "demo-game", "development", proposal.ProposalKey)
	require.NoError(t, err)
}

// CreateCompositeProposal：panic → recover 显式转为错误（nil proposalModel
// 触发确定性空指针，验证 defer recover 不让 500 空响应逃逸）。
func TestCreateCompositeProposal_PanicRecoveredV9H(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()

	require.NoError(t, db.Create(&model.FunctionContract{
		GameID: "demo-game", Env: "development", FunctionID: "player.list", Enabled: true, Version: "1.0.0",
		Capability: dbenum.CapabilityCollectionQuery,
	}).Error)

	// proposalModel 置空：生成链走完后 upsertGeneratedProposal 首个
	// proposalModel 调用即空指针 panic，由函数顶部 recover 捕获。
	svc := NewContractService(db)
	svc.proposalModel = nil

	_, err := svc.CreateCompositeProposal(ctx, "demo-game", "development", "composite--panic", []CompositeSectionRequest{
		{Key: "s1", FunctionID: "player.list", View: "table"},
		{Key: "s2", FunctionID: "player.list", View: "fields"},
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal error creating composite page")
}
