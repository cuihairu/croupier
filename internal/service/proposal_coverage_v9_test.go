package service

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ginParamsV9(key, value string) gin.Params {
	return gin.Params{{Key: key, Value: value}}
}

// ListProposalDTOs：非法 PageSpec → DTO 转换错误；页面已物化 → PageExists。
func TestListProposalDTOsBranchesV9(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()
	svc := NewProposalService(db)

	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "g9", Env: "e9", ProposalKey: "bad:spec",
		PageKey: "op--bad", PageType: "operation", Quality: "ready",
		Status:   dbenum.ProposalStatusPending,
		PageSpec: model.JSON(`{"pageKey":`),
	}))
	_, err := svc.ListProposalDTOs(ctx, "g9", "e9", ProposalListFilter{})
	assert.Error(t, err)

	page := testProposalPageSpec("resource--player")
	pageJSON, err := json.Marshal(page)
	require.NoError(t, err)
	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "g9", Env: "e9", ProposalKey: "resource:player",
		PageKey: "resource--player", PageType: "operation", ResourceKey: "player",
		Quality: "ready", Status: dbenum.ProposalStatusPending, PageSpec: pageJSON,
	}))
	require.NoError(t, model.NewPageSpecModel(db).Upsert(ctx, &model.PageSpec{
		GameID: "g9", Env: "e9", PageKey: "resource--player", Type: "operation", Status: "draft",
	}))

	dtos, err := svc.ListProposalDTOs(ctx, "g9", "e9", ProposalListFilter{Status: "pending", ResourceKey: "player"})
	require.NoError(t, err)
	require.Len(t, dtos, 1)
	assert.True(t, dtos[0].PageExists)
}

// Inbox：blocked issue 查询失败与契约列表查询失败分别向上返回错误。
func TestInboxErrorBranchesV9(t *testing.T) {
	ctx := context.Background()

	t.Run("blocked issue query error", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewProposalService(db)
		stubTableV9(t, db, &model.BlockedProposalIssue{})
		_, err := svc.Inbox(ctx, "g9", "e9", ProposalListFilter{})
		assert.Error(t, err)
	})

	t.Run("contract change query error", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewProposalService(db)
		stubTableV9(t, db, &model.FunctionContract{})
		_, err := svc.Inbox(ctx, "g9", "e9", ProposalListFilter{})
		assert.Error(t, err)
	})
}

// AcceptProposal：PageSpec 校验失败（缺 zh-CN 标题/绑定）返回验证错误。
func TestAcceptProposalValidationFailedV9(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)

	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "demo-game", Env: "development", ProposalKey: "resource:invalid",
		PageKey: "op--invalid", PageType: "operation", Quality: "ready",
		Status:   dbenum.ProposalStatusPending,
		PageSpec: model.JSON(`{"pageKey":"op--invalid","type":"operation"}`),
	}))
	err := svc.AcceptProposal(ctx, "demo-game", "development", "resource:invalid")
	assert.ErrorContains(t, err, "validation")
}

// AcceptAndPublishProposal：非法 PageSpec JSON → 解析错误。
func TestAcceptAndPublishInvalidPageSpecV9(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()
	svc := NewProposalService(db)

	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "g9", Env: "e9", ProposalKey: "op:broken",
		PageKey: "op--broken", PageType: "operation", Quality: "ready",
		Status:   dbenum.ProposalStatusPending,
		PageSpec: model.JSON(`{"pageKey":`),
	}))
	_, err := svc.AcceptAndPublishProposal(ctx, "g9", "e9", "op:broken")
	assert.ErrorContains(t, err, "invalid JSON")
}

// listContractChanges 及其子查询的错误分支。
func TestListContractChangesErrorBranchesV9(t *testing.T) {
	ctx := context.Background()

	t.Run("function specs error", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewProposalService(db)
		stubTableV9(t, db, &model.FunctionContract{})
		_, err := svc.listContractChanges(ctx, "g9", "e9", "")
		assert.Error(t, err)
	})

	t.Run("published query error", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewProposalService(db)
		stubTableV9(t, db, &model.PublishedPageSpec{})
		_, err := svc.publishedContractChanges(ctx, "g9", "e9", "", nil)
		assert.Error(t, err)
	})

	t.Run("draft query error", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewProposalService(db)
		stubTableV9(t, db, &model.PageSpec{})
		_, err := svc.draftContractChanges(ctx, "g9", "e9", "", nil, nil)
		assert.Error(t, err)
	})
}

// listContractChanges：排序 comparator（时间相同→Kind→PageKey）与
// 无 stale 诊断的 published/draft 跳过分支。
func TestListContractChangesSortAndSkipsV9(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()
	svc := NewProposalService(db)
	stale := time.Time{}

	// stale published（绑定缺契约快照）。
	require.NoError(t, db.Create(&model.PublishedPageSpec{
		GameID: "g9", Env: "e9", PageKey: "pb", Version: 1, Active: true, PublishedAt: stale,
		SpecJSON: `{"pageKey":"pb","type":"operation","bindings":[{"id":"x","functionId":"ghost.fn"}]}`,
	}).Error)
	// fresh published（无绑定）→ continue。
	require.NoError(t, db.Create(&model.PublishedPageSpec{
		GameID: "g9", Env: "e9", PageKey: "pfresh", Version: 1, Active: true, PublishedAt: stale,
		SpecJSON: `{"pageKey":"pfresh","type":"operation","bindings":[]}`,
	}).Error)
	// stale drafts（绑定指向不存在的函数）。
	require.NoError(t, db.Create(&model.PageSpec{
		GameID: "g9", Env: "e9", PageKey: "pa", Type: "operation", Status: "draft",
		SpecJSON: `{"pageKey":"pa","type":"operation","bindings":[{"id":"x","functionId":"ghost.fn"}]}`,
	}).Error)
	require.NoError(t, db.Create(&model.PageSpec{
		GameID: "g9", Env: "e9", PageKey: "pz", Type: "operation", Status: "draft",
		SpecJSON: `{"pageKey":"pz","type":"operation","bindings":[{"id":"x","functionId":"ghost.fn"}]}`,
	}).Error)
	// fresh draft（无绑定）→ continue。
	require.NoError(t, db.Create(&model.PageSpec{
		GameID: "g9", Env: "e9", PageKey: "pd", Type: "operation", Status: "draft",
		SpecJSON: `{"pageKey":"pd","type":"operation","bindings":[]}`,
	}).Error)

	changes, err := svc.listContractChanges(ctx, "g9", "e9", "")
	require.NoError(t, err)
	require.Len(t, changes, 3)
	// 时间相同：draft < published（Kind），draft 之间按 PageKey 升序。
	assert.Equal(t, "pa", changes[0].PageKey)
	assert.Equal(t, "draft", changes[0].Kind)
	assert.Equal(t, "pz", changes[1].PageKey)
	assert.Equal(t, "pb", changes[2].PageKey)
	assert.Equal(t, "published", changes[2].Kind)
}

// validateDirectPublishPageSpec：契约查询失败；query 绑定缺 output selectors。
func TestValidateDirectPublishPageSpecBranchesV9(t *testing.T) {
	ctx := context.Background()

	t.Run("function specs error", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewProposalService(db)
		stubTableV9(t, db, &model.FunctionContract{})
		err := svc.validateDirectPublishPageSpec(ctx, "g9", "e9", &model.PageProposal{PageKey: "op--x"}, testProposalPageSpec("op--x"))
		assert.Error(t, err)
	})

	t.Run("missing output selectors", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewProposalService(db)
		require.NoError(t, model.NewFunctionContractModel(db).UpsertContract(ctx, &model.FunctionContract{
			GameID: "g9", Env: "e9", FunctionID: "player.query", Version: "1.0.0", Enabled: true,
			Capability:   dbenum.CapabilityCollectionQuery,
			Execution:    "sync",
			InputSchema:  model.JSON(`{"type":"object","properties":{"q":{"type":"string"}}}`),
			OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array"}}}`),
		}))
		page := spec.PageSpec{
			PageKey: "resource--player", Type: spec.PageTypeResource, ResourceKey: "player",
			Title:    spec.LocalizedText{"zh-CN": "玩家"},
			Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
			Resource: &spec.ResourcePageSpec{},
			Bindings: []spec.PageFunctionBinding{{
				ID: "query", FunctionID: "player.query", Usage: spec.BindingUsageQuery,
				Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
				Selectors: &spec.BindingSelectors{},
			}},
		}
		err := svc.validateDirectPublishPageSpec(ctx, "g9", "e9", &model.PageProposal{PageKey: "resource--player"}, page)
		assert.ErrorContains(t, err, "selectors.output")
	})
}

// validateCategoryLabelConflict：publishedModel 为 nil 跳过、查询失败、
// 同 PageKey 跳过、异类目跳过、同目目同文案不冲突。
func TestValidateCategoryLabelConflictBranchesV9(t *testing.T) {
	ctx := context.Background()
	page := spec.PageSpec{
		PageKey:  "target",
		Category: spec.PageCategorySpec{Key: "cat", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
	}

	t.Run("nil published model", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewProposalService(db)
		svc.publishedModel = nil
		assert.NoError(t, svc.validateCategoryLabelConflict(ctx, "g9", "e9", page))
	})

	t.Run("published query error", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewProposalService(db)
		stubTableV9(t, db, &model.PublishedPageSpec{})
		err := svc.validateCategoryLabelConflict(ctx, "g9", "e9", page)
		assert.ErrorContains(t, err, "list published pages")
	})

	t.Run("same page key and other categories skipped", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewProposalService(db)
		seed := []model.PublishedPageSpec{
			{ // 同 PageKey → continue。
				GameID: "g9", Env: "e9", PageKey: "target", Version: 1, Active: true,
				SpecJSON: `{"pageKey":"target","category":{"key":"cat","labels":{"zh-CN":"别的"}}}`,
			},
			{ // 异类目 → continue。
				GameID: "g9", Env: "e9", PageKey: "other", Version: 1, Active: true,
				SpecJSON: `{"pageKey":"other","category":{"key":"othercat","labels":{"zh-CN":"别的"}}}`,
			},
			{ // 同类目同文案 → 不冲突。
				GameID: "g9", Env: "e9", PageKey: "another", Version: 1, Active: true,
				SpecJSON: `{"pageKey":"another","category":{"key":"cat","labels":{"zh-CN":"玩家"}}}`,
			},
		}
		for i := range seed {
			require.NoError(t, db.Create(&seed[i]).Error)
		}
		assert.NoError(t, svc.validateCategoryLabelConflict(ctx, "g9", "e9", page))
	})
}

// Handler 成功路径：GetContract / GetProposal / Accept / AcceptAndPublish / Reject。
func TestHandlerSuccessPathsV9(t *testing.T) {
	ctx := proposalTestContext()

	t.Run("contract handler get success", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewContractService(db)
		registerPlayerListV9(t, svc, ctx, "demo-game", "development", "player", "player.list")
		handler := NewContractHandler(svc)

		c, w := newHandlerTestContext(http.MethodGet, "/api/contracts/player.list")
		c.Params = ginParamsV9("functionId", "player.list")
		c.Request = c.Request.WithContext(ctx)
		handler.GetContract(c)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("proposal handler flows", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewProposalService(db)
		handler := NewProposalHandler(svc)

		// GetProposal 成功。
		page := testProposalPageSpec("resource--player")
		pageJSON, err := json.Marshal(page)
		require.NoError(t, err)
		require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
			GameID: "demo-game", Env: "development", ProposalKey: "resource:player",
			PageKey: "resource--player", PageType: "operation", ResourceKey: "player",
			Quality: "ready", Status: dbenum.ProposalStatusPending, PageSpec: pageJSON,
		}))
		c, w := newHandlerTestContext(http.MethodGet, "/api/proposals/resource:player")
		c.Params = ginParamsV9("proposalKey", "resource:player")
		c.Request = c.Request.WithContext(ctx)
		handler.GetProposal(c)
		assert.Equal(t, http.StatusOK, w.Code)

		// AcceptProposal 成功。
		c, w = newHandlerTestContext(http.MethodPost, "/api/proposals/resource:player/accept")
		c.Params = ginParamsV9("proposalKey", "resource:player")
		c.Request = c.Request.WithContext(ctx)
		handler.AcceptProposal(c)
		assert.Equal(t, http.StatusOK, w.Code)

		// RejectProposal 成功（新提案）。
		require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
			GameID: "demo-game", Env: "development", ProposalKey: "resource:reject",
			PageKey: "resource--reject", PageType: "operation", ResourceKey: "player",
			Quality: "ready", Status: dbenum.ProposalStatusPending, PageSpec: pageJSON,
		}))
		c, w = newHandlerTestContext(http.MethodPost, "/api/proposals/resource:reject/reject")
		c.Params = ginParamsV9("proposalKey", "resource:reject")
		c.Request = c.Request.WithContext(ctx)
		handler.RejectProposal(c)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("accept and publish success", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewProposalService(db)
		handler := NewProposalHandler(svc)

		require.NoError(t, model.NewFunctionContractModel(db).UpsertContract(ctx, &model.FunctionContract{
			GameID: "demo-game", Env: "development", FunctionID: "mail.send", Version: "2.3.4", Enabled: true,
			Capability:   dbenum.CapabilityAction,
			Execution:    "sync",
			InputSchema:  model.JSON(`{"type":"object"}`),
			OutputSchema: model.JSON(`{"type":"object"}`),
		}))
		page := spec.PageSpec{
			PageKey: "operation--mail.send", Type: spec.PageTypeOperation,
			Title:     spec.LocalizedText{"zh-CN": "发送邮件"},
			Category:  spec.PageCategorySpec{Key: "mail", Labels: spec.LocalizedText{"zh-CN": "邮件"}},
			Operation: &spec.OperationPageSpec{Form: &spec.FormPresentationSpec{JSONSchema: spec.JSONSchema(`{"type":"object"}`)}},
			Bindings: []spec.PageFunctionBinding{{
				ID: "main", FunctionID: "mail.send", Usage: spec.BindingUsageAction,
				Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
			}},
		}
		pageJSON, err := json.Marshal(page)
		require.NoError(t, err)
		require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
			GameID: "demo-game", Env: "development", ProposalKey: "operation:mail.send",
			PageKey: page.PageKey, PageType: string(page.Type), Quality: "basic",
			Status: dbenum.ProposalStatusPending, PageSpec: pageJSON,
		}))

		c, w := newHandlerTestContext(http.MethodPost, "/api/proposals/operation:mail.send/accept-and-publish")
		c.Params = ginParamsV9("proposalKey", "operation:mail.send")
		c.Request = c.Request.WithContext(ctx)
		handler.AcceptAndPublishProposal(c)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// ExportAllPages：published 页查询失败分支。
func TestExportAllPagesPublishedErrorV9(t *testing.T) {
	db := setupTestDBFileV9(t)
	svc := NewDataExportService(db)
	require.NoError(t, db.Migrator().DropTable(&model.PublishedPageSpec{}))
	_, err := svc.ExportAllPages(context.Background(), "g9", "e9")
	assert.ErrorContains(t, err, "export published pages")
}
