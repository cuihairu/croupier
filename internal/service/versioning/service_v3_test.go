package versioning

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// RollbackDraft success path
// ---------------------------------------------------------------------------

func TestVersioningService_RollbackDraft_Success(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	page := spec.PageSpec{
		PageKey:     "resource--player",
		Type:        spec.PageTypeResource,
		ResourceKey: "player",
		Title:       spec.LocalizedText{"zh-CN": "玩家"},
		Category:    spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
	}
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))
	require.NoError(t, createVersioningTestPageVersion(db, "demo-game", "development", page, 1, "initial"))

	oldPage := page
	oldPage.Title = spec.LocalizedText{"zh-CN": "旧版玩家"}
	require.NoError(t, createVersioningTestPageVersion(db, "demo-game", "development", oldPage, 2, "old draft"))

	resp, err := service.RollbackDraft(ctx, &RollbackRequest{
		GameID:                "demo-game",
		Env:                   "development",
		PageKey:               "resource--player",
		ExpectedDraftRevision: 1,
		Version:               2,
		Reason:                "test rollback",
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Contains(t, resp.Message, "rolled back")
	assert.Equal(t, "resource--player", resp.PageKey)
	assert.True(t, resp.DraftRevision > 1, "DraftRevision should be incremented")
}

func TestVersioningService_RollbackDraft_ZeroVersion(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.RollbackDraft(context.Background(), &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: "resource--player",
		ExpectedDraftRevision: 1, Version: 0,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version is required")
}

func TestVersioningService_RollbackDraft_ZeroRevision(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.RollbackDraft(context.Background(), &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: "resource--player",
		ExpectedDraftRevision: 0, Version: 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expectedDraftRevision is required")
}

func TestVersioningService_RollbackDraft_EmptyPageKey(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.RollbackDraft(context.Background(), &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: "  ",
		ExpectedDraftRevision: 1, Version: 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pageKey is required")
}

func TestVersioningService_RollbackDraft_VersionNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", spec.PageSpec{
		PageKey: "resource--player", Type: spec.PageTypeResource,
		Title:    spec.LocalizedText{"zh-CN": "玩家"},
		Category: spec.PageCategorySpec{Key: "player"},
	}))
	_, err := service.RollbackDraft(context.Background(), &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: "resource--player",
		ExpectedDraftRevision: 1, Version: 999,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page version not found")
}

// ---------------------------------------------------------------------------
// RollbackPublish success path
// ---------------------------------------------------------------------------

func TestVersioningService_RollbackPublish_Success(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Use unique game_id to avoid UNIQUE constraint conflicts across tests.
	const testGame = "rollback_publish_test"
	page := spec.PageSpec{
		PageKey: "resource--player", Type: spec.PageTypeResource,
		Title:    spec.LocalizedText{"zh-CN": "玩家"},
		Category: spec.PageCategorySpec{Key: "player"},
	}
	require.NoError(t, createVersioningTestPage(db, testGame, "development", page))
	// Seed page_versions so GetNextVersion returns 2 (not 1).
	require.NoError(t, createVersioningTestPageVersion(db, testGame, "development", page, 1, "initial"))

	publishedJSON, err := marshalPageSpec(page)
	require.NoError(t, err)
	require.NoError(t, model.NewPublishedPageSpecModel(db).Create(ctx, &model.PublishedPageSpec{
		GameID: testGame, Env: "development", PageKey: "resource--player",
		Version: 1, SpecJSON: publishedJSON,
		Active: true, PublishedAt: time.Now(), PublishedBy: "tester",
	}))

	resp, err := service.RollbackPublish(ctx, &RollbackRequest{
		GameID: testGame, Env: "development", PageKey: "resource--player",
		ExpectedDraftRevision: 1, Version: 1, Reason: "test rollback publish",
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Message, "rolled back")
	assert.Equal(t, 2, resp.Version)
}

func TestVersioningService_RollbackPublish_ZeroVersion(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.RollbackPublish(context.Background(), &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: "resource--player",
		ExpectedDraftRevision: 1, Version: 0,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version is required")
}

func TestVersioningService_RollbackPublish_ZeroRevision(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.RollbackPublish(context.Background(), &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: "resource--player",
		ExpectedDraftRevision: 0, Version: 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expectedDraftRevision is required")
}

func TestVersioningService_RollbackPublish_EmptyPageKey(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.RollbackPublish(context.Background(), &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: "",
		ExpectedDraftRevision: 1, Version: 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pageKey is required")
}

func TestVersioningService_RollbackPublish_VersionNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", spec.PageSpec{
		PageKey: "resource--player", Type: spec.PageTypeResource,
		Title:    spec.LocalizedText{"zh-CN": "玩家"},
		Category: spec.PageCategorySpec{Key: "player"},
	}))
	_, err := service.RollbackPublish(context.Background(), &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: "resource--player",
		ExpectedDraftRevision: 1, Version: 999,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ---------------------------------------------------------------------------
// Republish success path
// ---------------------------------------------------------------------------

func TestVersioningService_Republish_Success(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Use unique game_id to avoid UNIQUE constraint conflicts across tests.
	const testGame = "republish_test"
	contractModel := model.NewFunctionContractModel(db)
	require.NoError(t, contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: testGame, Env: "development", FunctionID: "player.list",
		Version: "1.0.0", Enabled: true, UpdatedAt: time.Now(),
	}))

	page := spec.PageSpec{
		PageKey: "resource--player", Type: spec.PageTypeResource,
		Title:    spec.LocalizedText{"zh-CN": "玩家"},
		Category: spec.PageCategorySpec{Key: "player"},
		Bindings: []spec.PageFunctionBinding{{
			ID: "query", FunctionID: "player.list", Usage: spec.BindingUsageQuery,
			Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
	}
	require.NoError(t, createVersioningTestPage(db, testGame, "development", page))
	// Seed page_versions so GetNextVersion returns 2 (not 1).
	require.NoError(t, createVersioningTestPageVersion(db, testGame, "development", page, 1, "initial"))

	// Seed an initial published spec so Republish creates version 2.
	publishedJSON, err := marshalPageSpec(page)
	require.NoError(t, err)
	require.NoError(t, model.NewPublishedPageSpecModel(db).Create(ctx, &model.PublishedPageSpec{
		GameID: testGame, Env: "development", PageKey: "resource--player",
		Version: 1, SpecJSON: publishedJSON,
		Active: true, PublishedAt: time.Now(), PublishedBy: "tester",
	}))

	resp, err := service.Republish(ctx, &RepublishRequest{
		GameID: testGame, Env: "development", PageKey: "resource--player",
		Reason: "test republish",
	})
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Version)
	assert.Contains(t, resp.Message, "republished")
}

func TestVersioningService_Republish_EmptyPageKey(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.Republish(context.Background(), &RepublishRequest{
		GameID: "demo-game", Env: "development", PageKey: "  ",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pageKey is required")
}

func TestVersioningService_Republish_PageNotFound(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.Republish(context.Background(), &RepublishRequest{
		GameID: "demo-game", Env: "development", PageKey: "nonexistent",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestVersioningService_Republish_BindingWithEmptyFunctionID(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", spec.PageSpec{
		PageKey: "resource--player", Type: spec.PageTypeResource,
		Title:    spec.LocalizedText{"zh-CN": "玩家"},
		Category: spec.PageCategorySpec{Key: "player"},
		Bindings: []spec.PageFunctionBinding{{ID: "query", Usage: spec.BindingUsageQuery}},
	}))
	_, err := service.Republish(context.Background(), &RepublishRequest{
		GameID: "demo-game", Env: "development", PageKey: "resource--player",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "functionId is required")
}

func TestVersioningService_Republish_ContractNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", spec.PageSpec{
		PageKey: "resource--player", Type: spec.PageTypeResource,
		Title:    spec.LocalizedText{"zh-CN": "玩家"},
		Category: spec.PageCategorySpec{Key: "player"},
		Bindings: []spec.PageFunctionBinding{{ID: "query", FunctionID: "nonexistent.func", Usage: spec.BindingUsageQuery}},
	}))
	_, err := service.Republish(context.Background(), &RepublishRequest{
		GameID: "demo-game", Env: "development", PageKey: "resource--player",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestVersioningService_Republish_DisabledContract(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)
	require.NoError(t, model.NewFunctionContractModel(db).UpsertContract(ctx, &model.FunctionContract{
		GameID: "demo-game", Env: "development", FunctionID: "disabled.func",
		Version: "1.0.0", Enabled: false, UpdatedAt: time.Now(),
	}))
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", spec.PageSpec{
		PageKey: "resource--player", Type: spec.PageTypeResource,
		Title:    spec.LocalizedText{"zh-CN": "玩家"},
		Category: spec.PageCategorySpec{Key: "player"},
		Bindings: []spec.PageFunctionBinding{{ID: "query", FunctionID: "disabled.func", Usage: spec.BindingUsageQuery}},
	}))
	_, err := service.Republish(context.Background(), &RepublishRequest{
		GameID: "demo-game", Env: "development", PageKey: "resource--player",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

// ---------------------------------------------------------------------------
// Merge: various strategies and error paths
// ---------------------------------------------------------------------------

func TestVersioningService_Merge_UnknownStrategy(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.Merge(context.Background(), &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: "resource--player",
		Strategy: "unknown",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown merge strategy")
}

func TestVersioningService_Merge_AcceptStrategy(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.Merge(context.Background(), &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: "resource--player",
		Strategy: MergeStrategyAccept,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestVersioningService_Merge_EmptyPageKey(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.Merge(context.Background(), &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: "",
		Strategy: MergeStrategyAuto,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pageKey is required")
}

func TestVersioningService_Merge_Auto_NoDraftRevision(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.Merge(context.Background(), &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: "resource--player",
		Strategy: MergeStrategyAuto, ExpectedDraftRevision: 0,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expectedDraftRevision is required")
}

func TestVersioningService_Merge_Auto_NoBaseProposal(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", spec.PageSpec{
		PageKey: "resource--player", Type: spec.PageTypeResource,
		Title:    spec.LocalizedText{"zh-CN": "玩家"},
		Category: spec.PageCategorySpec{Key: "player"},
	}))
	_, err := service.Merge(context.Background(), &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: "resource--player",
		Strategy: MergeStrategyAuto, ExpectedDraftRevision: 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no base proposal snapshot")
}

// ---------------------------------------------------------------------------
// GetChangeChain: with semantics and proposal
// ---------------------------------------------------------------------------

func TestVersioningService_GetChangeChain_WithSemanticsAndProposal(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	contractModel := model.NewFunctionContractModel(db)
	require.NoError(t, contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "demo-game", Env: "development", FunctionID: "player.list",
		Version: "1.0.0", Enabled: true, UpdatedAt: time.Now(), UpdatedBy: "admin",
	}))

	semanticsModel := model.NewCapabilitySemanticsModel(db)
	require.NoError(t, semanticsModel.UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID: "demo-game", Env: "development", ResourceKey: "player",
		Version: 3, UpdatedAt: time.Now(), UpdatedBy: "admin",
	}))

	page := spec.PageSpec{
		PageKey: "resource--player", Type: spec.PageTypeResource, ResourceKey: "player",
		Title:    spec.LocalizedText{"zh-CN": "玩家"},
		Category: spec.PageCategorySpec{Key: "player"},
		Bindings: []spec.PageFunctionBinding{{
			ID: "query", FunctionID: "player.list", Usage: spec.BindingUsageQuery,
			Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
	}
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))

	proposalModel := model.NewPageProposalModel(db)
	pageJSON, _ := json.Marshal(page)
	require.NoError(t, proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "demo-game", Env: "development",
		ProposalKey: "operation:player.list", PageKey: "resource--player",
		PageType: "resource", ResourceKey: "player", Quality: "good",
		PageSpec: model.JSON(pageJSON), Status: dbenum.ProposalStatusPending,
		UpdatedAt: time.Now(), UpdatedBy: "admin",
	}))

	require.NoError(t, createVersioningTestPageVersion(db, "demo-game", "development", page, 1, "initial"))

	chain, err := service.GetChangeChain(ctx, &GetChangeChainRequest{
		GameID: "demo-game", Env: "development", PageKey: "resource--player",
	})
	require.NoError(t, err)
	assert.NotNil(t, chain)
	assert.True(t, chain.Current.SemanticVersion > 0 || len(chain.Items) > 0, "chain should have data")
}

func TestVersioningService_GetChangeChain_EmptyPageKey(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.GetChangeChain(context.Background(), &GetChangeChainRequest{
		GameID: "demo-game", Env: "development", PageKey: "  ",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pageKey is required")
}

func TestVersioningService_GetChangeChain_PageNotFound(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.GetChangeChain(context.Background(), &GetChangeChainRequest{
		GameID: "demo-game", Env: "development", PageKey: "nonexistent",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ---------------------------------------------------------------------------
// Diff: error paths
// ---------------------------------------------------------------------------

func TestVersioningService_Diff_EmptyPageKey(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.Diff(context.Background(), &DiffRequest{
		GameID: "demo-game", Env: "development", PageKey: "  ",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pageKey is required")
}

func TestVersioningService_Diff_PageNotFound(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.Diff(context.Background(), &DiffRequest{
		GameID: "demo-game", Env: "development", PageKey: "nonexistent",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ---------------------------------------------------------------------------
// samePageSpec
// ---------------------------------------------------------------------------

func TestSamePageSpecV4(t *testing.T) {
	assert.True(t, samePageSpec(spec.PageSpec{}, spec.PageSpec{}))
	s1 := spec.PageSpec{PageKey: "test", Type: spec.PageTypeOperation}
	s2 := spec.PageSpec{PageKey: "test", Type: spec.PageTypeOperation}
	assert.True(t, samePageSpec(s1, s2))
	assert.False(t, samePageSpec(spec.PageSpec{PageKey: "a"}, spec.PageSpec{PageKey: "b"}))
}

// ---------------------------------------------------------------------------
// normalizePageSpec: more fields
// ---------------------------------------------------------------------------

func TestNormalizePageSpec_MoreFields(t *testing.T) {
	page := spec.PageSpec{
		PageKey: "  test  ", ResourceKey: "  player  ", Icon: "  icon  ",
		Title:       spec.LocalizedText{"zh": " 玩家 ", "en": " Player "},
		Description: spec.LocalizedText{"zh_cn": " 描述 "},
		Category:    spec.PageCategorySpec{Key: "  cat  ", Labels: spec.LocalizedText{"en": " Cat "}, Order: 3},
		Bindings:    []spec.PageFunctionBinding{{ID: "  run  ", FunctionID: "  func  "}},
	}
	n := normalizePageSpec(page)
	assert.Equal(t, "test", n.PageKey)
	assert.Equal(t, "player", n.ResourceKey)
	assert.Equal(t, "icon", n.Icon)
	assert.Equal(t, "玩家", n.Title["zh-CN"])
	assert.Equal(t, "Player", n.Title["en-US"])
	assert.Equal(t, "描述", n.Description["zh-CN"])
	assert.Equal(t, "cat", n.Category.Key)
	assert.Equal(t, "Cat", n.Category.Labels["en-US"])
	assert.Equal(t, "run", n.Bindings[0].ID)
	assert.Equal(t, "func", n.Bindings[0].FunctionID)
}

// ---------------------------------------------------------------------------
// bindingContractChanges: with published contracts
// ---------------------------------------------------------------------------

func TestVersioningService_BindingContractChanges_WithPublished(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Use unique game_id to avoid UNIQUE constraint conflicts across tests.
	const testGame = "binding_changes_with_pub_test"
	require.NoError(t, model.NewFunctionContractModel(db).UpsertContract(ctx, &model.FunctionContract{
		GameID: testGame, Env: "development", FunctionID: "player.list",
		Version: "1.0.0", Enabled: true,
		InputSchema:  model.JSON(`{"type":"object"}`),
		OutputSchema: model.JSON(`{"type":"object"}`),
		Risk:         dbenum.RiskSafe, Permission: "admin:all", UpdatedAt: time.Now(),
	}))

	publishedJSON, _ := json.Marshal(spec.PageSpec{PageKey: "resource--player", Type: spec.PageTypeResource})
	require.NoError(t, model.NewPublishedPageSpecModel(db).Create(ctx, &model.PublishedPageSpec{
		GameID: testGame, Env: "development", PageKey: "resource--player",
		Version: 1, SpecJSON: string(publishedJSON),
		BindingContractsJSON: `[{"bindingId":"query","functionId":"player.list","inputSchemaDigest":"old","outputSchemaDigest":"old","risk":"high","permission":"ops:read"}]`,
		Active:               true, PublishedAt: time.Now(),
	}))

	changes := service.bindingContractChanges(ctx, testGame, "development", "resource--player", spec.PageSpec{
		PageKey: "resource--player", Type: spec.PageTypeResource,
		Bindings: []spec.PageFunctionBinding{{ID: "query", FunctionID: "player.list"}},
	})
	assert.True(t, len(changes) >= 2, "expected at least 2 changes, got %d", len(changes))
}

func TestVersioningService_BindingContractChanges_FunctionRemoved(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Use unique game_id to avoid UNIQUE constraint conflicts across tests.
	const testGame = "binding_changes_func_removed_test"
	publishedJSON, _ := json.Marshal(spec.PageSpec{PageKey: "resource--player", Type: spec.PageTypeResource})
	require.NoError(t, model.NewPublishedPageSpecModel(db).Create(ctx, &model.PublishedPageSpec{
		GameID: testGame, Env: "development", PageKey: "resource--player",
		Version: 1, SpecJSON: string(publishedJSON),
		BindingContractsJSON: `[{"bindingId":"query","functionId":"deleted.func","inputSchemaDigest":"","outputSchemaDigest":"","risk":"low","permission":""}]`,
		Active:               true, PublishedAt: time.Now(),
	}))

	changes := service.bindingContractChanges(ctx, testGame, "development", "resource--player", spec.PageSpec{
		PageKey: "resource--player", Type: spec.PageTypeResource,
		Bindings: []spec.PageFunctionBinding{{ID: "query", FunctionID: "deleted.func"}},
	})
	require.Len(t, changes, 1)
	assert.Equal(t, "removed", changes[0].ChangeType)
}

// ---------------------------------------------------------------------------
// draftBindingContractChanges
// ---------------------------------------------------------------------------

func TestVersioningService_DraftBindingContractChanges(t *testing.T) {
	service := NewService(setupTestDB(t))
	changes := service.draftBindingContractChanges(context.Background(), "demo-game", "development", spec.PageSpec{
		Bindings: []spec.PageFunctionBinding{
			{ID: "query", FunctionID: "nonexistent.func"},
			{ID: "", FunctionID: ""},
			{ID: "action", FunctionID: "another.missing"},
		},
	})
	require.Len(t, changes, 2)
	assert.Equal(t, "removed", changes[0].ChangeType)
}

// ---------------------------------------------------------------------------
// buildBindingContracts: error paths
// ---------------------------------------------------------------------------

func TestVersioningService_BuildBindingContracts_EmptyFunctionID(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.buildBindingContracts(context.Background(), "demo-game", "development", []spec.PageFunctionBinding{
		{ID: "run", FunctionID: ""},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "functionId is required")
}

func TestVersioningService_BuildBindingContracts_ContractNotFound(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.buildBindingContracts(context.Background(), "demo-game", "development", []spec.PageFunctionBinding{
		{ID: "run", FunctionID: "nonexistent.func"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestVersioningService_BuildBindingContracts_DisabledContract(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)
	require.NoError(t, model.NewFunctionContractModel(db).UpsertContract(ctx, &model.FunctionContract{
		GameID: "demo-game", Env: "development", FunctionID: "disabled.func",
		Version: "1.0.0", Enabled: false, UpdatedAt: time.Now(),
	}))
	_, err := service.buildBindingContracts(ctx, "demo-game", "development", []spec.PageFunctionBinding{
		{ID: "run", FunctionID: "disabled.func"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

// ---------------------------------------------------------------------------
// mainContractForStandalonePage: error paths
// ---------------------------------------------------------------------------

func TestVersioningService_MainContractForStandalonePage_ResourceType(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.mainContractForStandalonePage(context.Background(), "demo-game", "development",
		spec.PageSpec{Type: spec.PageTypeResource})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resource pages must be regenerated")
}

func TestVersioningService_MainContractForStandalonePage_NoBinding(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.mainContractForStandalonePage(context.Background(), "demo-game", "development",
		spec.PageSpec{Type: spec.PageTypeOperation, Bindings: []spec.PageFunctionBinding{}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no main executable binding")
}

func TestVersioningService_MainContractForStandalonePage_QueryOnly(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.mainContractForStandalonePage(context.Background(), "demo-game", "development",
		spec.PageSpec{Type: spec.PageTypeOperation, Bindings: []spec.PageFunctionBinding{
			{ID: "q", FunctionID: "f", Usage: spec.BindingUsageQuery},
		}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no main executable binding")
}

func TestVersioningService_MainContractForStandalonePage_ContractNotFound(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.mainContractForStandalonePage(context.Background(), "demo-game", "development",
		spec.PageSpec{Type: spec.PageTypeOperation, Bindings: []spec.PageFunctionBinding{
			{ID: "run", FunctionID: "nonexistent.func", Usage: spec.BindingUsageAction},
		}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ---------------------------------------------------------------------------
// mergeMessage, diffSummary, manualMergeMessage, mergePreviewMessage
// ---------------------------------------------------------------------------

func TestMergeMessage_AllBranches(t *testing.T) {
	assert.Equal(t, "no contract changes require merge", mergeMessage(0, 0, false))
	assert.Contains(t, mergeMessage(0, 0, true), "no contract changes")
	assert.Contains(t, mergeMessage(2, 0, false), "found 2 safe changes")
	assert.Contains(t, mergeMessage(0, 3, false), "3 conflicts")
	assert.Contains(t, mergeMessage(3, 0, true), "auto-merged 3 safe changes")
	assert.Contains(t, mergeMessage(3, 2, true), "auto-merged 3 safe changes")
	assert.Contains(t, mergeMessage(3, 2, true), "2 conflicts still require manual review")
}

func TestDiffSummary_AllBranches(t *testing.T) {
	assert.Equal(t, "found no changes", diffSummary(0, 0, 0))
	s := diffSummary(5, 2, 3)
	assert.Contains(t, s, "5 changes")
	assert.Contains(t, s, "2 safe merge")
	assert.Contains(t, s, "3 conflicts")
}

func TestManualMergeMessage_AllBranches(t *testing.T) {
	assert.Equal(t, "accepted latest proposal snapshot", manualMergeMessage(0, 0))
	assert.Contains(t, manualMergeMessage(0, 3), "resolved 3 conflicts")
	assert.Contains(t, manualMergeMessage(2, 3), "auto-merged 2")
	assert.Contains(t, manualMergeMessage(2, 3), "resolved 3 conflicts")
}

func TestMergePreviewMessage_AllBranches(t *testing.T) {
	assert.Equal(t, "no contract changes require merge", mergePreviewMessage(0, 0, false))
	assert.Equal(t, "latest proposal snapshot can be accepted without page content changes", mergePreviewMessage(0, 0, true))
	assert.Contains(t, mergePreviewMessage(3, 0, false), "3 safe changes")
	assert.Contains(t, mergePreviewMessage(3, 2, false), "3 safe changes")
	assert.Contains(t, mergePreviewMessage(3, 2, false), "2 conflicts")
}

// ---------------------------------------------------------------------------
// proposalForPage
// ---------------------------------------------------------------------------

func TestVersioningService_ProposalForPage_NilPage(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.proposalForPage(context.Background(), "demo-game", "development", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestVersioningService_ProposalForPage_WithBaseProposalKey(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)
	require.NoError(t, model.NewPageProposalModel(db).UpsertProposal(ctx, &model.PageProposal{
		GameID: "demo-game", Env: "development",
		ProposalKey: "operation:player.list", PageKey: "resource--player",
		PageSpec: model.JSON(`{"pageKey":"test"}`),
		Status:   dbenum.ProposalStatusPending, UpdatedAt: time.Now(),
	}))
	proposal, err := service.proposalForPage(ctx, "demo-game", "development", &model.PageSpec{
		GameID: "demo-game", Env: "development", PageKey: "resource--player",
		BaseProposalKey: "operation:player.list",
	})
	require.NoError(t, err)
	assert.Equal(t, "operation:player.list", proposal.ProposalKey)
}

// ---------------------------------------------------------------------------
// jsonValue / computeDigest error paths
// ---------------------------------------------------------------------------

func TestJsonValue_UnmarshalableValue(t *testing.T) {
	ch := make(chan int)
	result := jsonValue(ch)
	assert.Equal(t, "null", string(result))
}

func TestComputeDigest_UnmarshalableValue(t *testing.T) {
	ch := make(chan int)
	assert.Equal(t, "", computeDigest(ch))
}

// ---------------------------------------------------------------------------
// applyPageSpecToModel
// ---------------------------------------------------------------------------

func TestApplyPageSpecToModel_MoreFields(t *testing.T) {
	page := &model.PageSpec{}
	err := applyPageSpecToModel(page, spec.PageSpec{
		PageKey: "test-page", Type: spec.PageTypeOperation, ResourceKey: "player",
		Icon: "icon-name", Order: 5,
		Title:    spec.LocalizedText{"zh-CN": "测试"},
		Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}, Order: 3},
		Bindings: []spec.PageFunctionBinding{{ID: "run", FunctionID: "player.ban"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "test-page", page.PageKey)
	assert.Equal(t, "operation", page.Type)
	assert.Equal(t, "player", page.ResourceKey)
	assert.Equal(t, "icon-name", page.Icon)
	assert.Equal(t, 5, page.Order)
	assert.Equal(t, "player", page.CategoryKey)
	assert.Equal(t, 3, page.CategoryOrder)
}

// ---------------------------------------------------------------------------
// pageSpecFromModel: error paths
// ---------------------------------------------------------------------------

func TestPageSpecFromModel_NilPage(t *testing.T) {
	_, err := pageSpecFromModel(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPageSpecFromModel_EmptySpecJSON(t *testing.T) {
	_, err := pageSpecFromModel(&model.PageSpec{SpecJSON: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPageSpecFromModel_InvalidJSON(t *testing.T) {
	_, err := pageSpecFromModel(&model.PageSpec{SpecJSON: "invalid json"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}

// ---------------------------------------------------------------------------
// mergePreviewForPage
// ---------------------------------------------------------------------------

func TestVersioningService_MergePreviewForPage_NilPage(t *testing.T) {
	service := NewService(setupTestDB(t))
	_, err := service.mergePreviewForPage(context.Background(), "demo-game", "development", "resource--player", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestVersioningService_MergePreviewForPage_NoBaseProposal(t *testing.T) {
	service := NewService(setupTestDB(t))
	result, err := service.mergePreviewForPage(context.Background(), "demo-game", "development", "resource--player",
		&model.PageSpec{GameID: "demo-game", Env: "development", PageKey: "resource--player"}, nil)
	require.NoError(t, err)
	assert.Empty(t, result.AutoMerge)
}

// ---------------------------------------------------------------------------
// proposalKeyForPage
// ---------------------------------------------------------------------------

func TestProposalKeyForPage_Whitespace(t *testing.T) {
	assert.Equal(t, "", proposalKeyForPage(spec.PageTypeOperation, "  "))
	assert.Equal(t, "", proposalKeyForPage(spec.PageTypeOperation, ""))
}

// ---------------------------------------------------------------------------
// createProposalVersionSnapshot error paths
// ---------------------------------------------------------------------------

func TestCreateProposalVersionSnapshot_NilProposal(t *testing.T) {
	_, err := createProposalVersionSnapshot(context.Background(), model.NewPageProposalVersionModel(setupTestDB(t)), nil, "reason", "actor")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be persisted")
}

func TestCreateProposalVersionSnapshot_ZeroIDProposal(t *testing.T) {
	proposal := &model.PageProposal{}
	proposal.ID = 0
	_, err := createProposalVersionSnapshot(context.Background(), model.NewPageProposalVersionModel(setupTestDB(t)), proposal, "reason", "actor")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be persisted")
}

// ---------------------------------------------------------------------------
// applyConflictField: more cases
// ---------------------------------------------------------------------------

func TestApplyConflictField_Title(t *testing.T) {
	// "title" is not in the switch cases for applyConflictField (it's for auto-merge)
	// Test that it returns an error for unsupported fields
	page := spec.PageSpec{}
	err := applyConflictField(&page, "title", json.RawMessage(`{"zh-CN":"新标题"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestApplyConflictField_Description(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "description", json.RawMessage(`{"zh-CN":"新描述"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestApplyConflictField_Order(t *testing.T) {
	page := spec.PageSpec{}
	err := applyConflictField(&page, "order", json.RawMessage(`10`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestApplyConflictField_ReportDataset(t *testing.T) {
	page := spec.PageSpec{}
	require.NoError(t, applyConflictField(&page, "report.dataset", json.RawMessage(`{"columns":[]}`)))
	assert.NotNil(t, page.Report)
	assert.NotNil(t, page.Report.Dataset)
}

func TestApplyConflictField_ReportTable(t *testing.T) {
	page := spec.PageSpec{}
	require.NoError(t, applyConflictField(&page, "report.table", json.RawMessage(`{"identityKey":"id"}`)))
	assert.NotNil(t, page.Report)
	assert.NotNil(t, page.Report.Table)
}

func TestApplyConflictField_ReportTable_NilReport(t *testing.T) {
	page := spec.PageSpec{}
	require.NoError(t, applyConflictField(&page, "report.table", json.RawMessage(`{"identityKey":"id"}`)))
	assert.NotNil(t, page.Report)
	assert.NotNil(t, page.Report.Table)
}

// ---------------------------------------------------------------------------
// normalizeLocalizedText: more cases
// ---------------------------------------------------------------------------

func TestNormalizeLocalizedText_AllWhitespace(t *testing.T) {
	input := map[string]string{"zh-CN": "  ", "en-US": "  "}
	result := normalizeLocalizedText(input)
	assert.Nil(t, result)
}
