package versioning

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Shared fixtures
// ---------------------------------------------------------------------------

func extraOperationPage(pageKey string) spec.PageSpec {
	return spec.PageSpec{
		PageKey:  pageKey,
		Type:     spec.PageTypeOperation,
		Title:    spec.LocalizedText{"zh-CN": "标题A"},
		Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
		Bindings: []spec.PageFunctionBinding{{
			ID:         "run",
			FunctionID: "player.ban",
			Usage:      spec.BindingUsageAction,
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
	}
}

func corruptProposalVersionJSON(t *testing.T, db *gorm.DB, version int) {
	t.Helper()
	require.NoError(t, db.Exec(
		"UPDATE page_proposal_versions SET proposal = ? WHERE version = ?",
		"{corrupted", version).Error)
}

func seedExtraContract(t *testing.T, db *gorm.DB, functionID string) {
	t.Helper()
	require.NoError(t, model.NewFunctionContractModel(db).UpsertContract(context.Background(),
		&model.FunctionContract{
			GameID:      "demo-game",
			Env:         "development",
			FunctionID:  functionID,
			Version:     "1.0.0",
			Enabled:     true,
			ResourceKey: "player",
			Capability:  "action",
			UpdatedAt:   time.Now(),
		}))
}

// ---------------------------------------------------------------------------
// GetChangeChain error branches
// ---------------------------------------------------------------------------

func TestExtra_GetChangeChain_ProposalVersionsListError(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db)

	page := extraOperationPage("operation--player.ban")
	seedExtraContract(t, db, "player.ban")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))

	// Seed a proposal bound to the page so GetChangeChain resolves it.
	proposalModel := model.NewPageProposalModel(db)
	require.NoError(t, proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID: "demo-game", Env: "development",
		ProposalKey: "operation:player.ban", PageKey: page.PageKey,
		Status: "pending", PageSpec: jsonValue(page), UpdatedBy: "tester",
	}))

	require.NoError(t, db.Exec("DROP TABLE page_proposal_versions").Error)

	_, err := svc.GetChangeChain(ctx, &GetChangeChainRequest{
		GameID: "demo-game", Env: "development", PageKey: "operation--player.ban",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list proposal versions")
}

func TestExtra_GetChangeChain_PageVersionsListError(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db)

	page := extraOperationPage("operation--player.ban")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))

	require.NoError(t, db.Exec("DROP TABLE page_versions").Error)

	_, err := svc.GetChangeChain(ctx, &GetChangeChainRequest{
		GameID: "demo-game", Env: "development", PageKey: "operation--player.ban",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list page versions")
}

// ---------------------------------------------------------------------------
// Diff branches
// ---------------------------------------------------------------------------

func TestExtra_Diff_WithFullSemanticsAndProposal(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db)

	require.NoError(t, model.NewCapabilitySemanticsModel(db).UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID:            "demo-game",
		Env:               "development",
		ResourceKey:       "player",
		IdentityField:     "player_id",
		CollectionQueryID: 11,
		CreateID:          21,
		UpdateID:          22,
		DeleteID:          23,
		Version:           3,
	}))

	page := spec.PageSpec{
		PageKey:     "resource--player",
		Type:        spec.PageTypeResource,
		ResourceKey: "player",
		Title:       spec.LocalizedText{"zh-CN": "Player"},
		Category:    spec.PageCategorySpec{Key: "player"},
		Bindings: []spec.PageFunctionBinding{{
			ID:         "query",
			FunctionID: "player.list",
			Usage:      spec.BindingUsageQuery,
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
	}
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))

	proposalModel := model.NewPageProposalModel(db)
	require.NoError(t, proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID:      "demo-game",
		Env:         "development",
		ProposalKey: "resource:player",
		PageKey:     "resource--player",
		PageType:    string(spec.PageTypeResource),
		Quality:     "good",
		Status:      "pending",
		PageSpec:    jsonValue(page),
		UpdatedBy:   "tester",
	}))

	resp, err := svc.Diff(ctx, &DiffRequest{
		GameID: "demo-game", Env: "development", PageKey: "resource--player",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	paths := make(map[string]bool, len(resp.Changes))
	for _, change := range resp.Changes {
		paths[change.Path] = true
	}
	for _, want := range []string{
		"identityField", "collectionQueryId",
		"lifecycle.create", "lifecycle.update", "lifecycle.delete",
		"proposal.quality",
	} {
		assert.True(t, paths[want], "expected diff path %s in %v", want, paths)
	}
}

// ---------------------------------------------------------------------------
// Merge branches
// ---------------------------------------------------------------------------

func seedMergeScenario(
	t *testing.T,
	db *gorm.DB,
	baseTitle, draftTitle, latestTitle spec.LocalizedText,
) spec.PageSpec {
	t.Helper()
	base := extraOperationPage("operation--player.ban")
	base.Title = baseTitle
	draft := base
	draft.Title = draftTitle
	latest := base
	latest.Title = latestTitle
	seedVersioningMergeFixture(t, db, base, draft, latest)
	return draft
}

func setBaseProposalVersion(t *testing.T, db *gorm.DB, pageKey string, version int) {
	t.Helper()
	require.NoError(t, db.Exec(
		"UPDATE page_specs SET base_proposal_version = ? WHERE page_key = ?",
		version, pageKey).Error)
}

func TestExtra_Merge_ProposalLookupErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("missing latest proposal", func(t *testing.T) {
		db := setupTestDB(t)
		svc := NewService(db)
		draft := extraOperationPage("operation--player.ban")
		require.NoError(t, createVersioningTestPage(db, "demo-game", "development", draft))

		pageModel := model.NewPageSpecModel(db)
		record, err := pageModel.FindByScopeAndPageKey(ctx, "demo-game", "development", draft.PageKey)
		require.NoError(t, err)
		record.BaseProposalKey = "operation:player.ban"
		record.BaseProposalVersion = 1
		require.NoError(t, pageModel.Upsert(ctx, record))

		_, err = svc.Merge(ctx, &MergeRequest{
			GameID: "demo-game", Env: "development", PageKey: draft.PageKey,
			ExpectedDraftRevision: record.DraftRevision, Strategy: MergeStrategyAuto,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "latest proposal not found")
	})

	t.Run("proposal page key mismatch", func(t *testing.T) {
		db := setupTestDB(t)
		svc := NewService(db)
		draft := extraOperationPage("operation--player.ban")
		require.NoError(t, createVersioningTestPage(db, "demo-game", "development", draft))

		require.NoError(t, model.NewPageProposalModel(db).UpsertProposal(ctx, &model.PageProposal{
			GameID: "demo-game", Env: "development",
			ProposalKey: "operation:player.ban", PageKey: "operation--other.page",
			Status: "pending", PageSpec: jsonValue(draft), UpdatedBy: "tester",
		}))

		pageModel := model.NewPageSpecModel(db)
		record, err := pageModel.FindByScopeAndPageKey(ctx, "demo-game", "development", draft.PageKey)
		require.NoError(t, err)
		record.BaseProposalKey = "operation:player.ban"
		record.BaseProposalVersion = 1
		require.NoError(t, pageModel.Upsert(ctx, record))

		_, err = svc.Merge(ctx, &MergeRequest{
			GameID: "demo-game", Env: "development", PageKey: draft.PageKey,
			ExpectedDraftRevision: record.DraftRevision, Strategy: MergeStrategyAuto,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not match the requested page")
	})

	t.Run("missing latest snapshot", func(t *testing.T) {
		db := setupTestDB(t)
		svc := NewService(db)
		draft := extraOperationPage("operation--player.ban")
		require.NoError(t, createVersioningTestPage(db, "demo-game", "development", draft))

		require.NoError(t, model.NewPageProposalModel(db).UpsertProposal(ctx, &model.PageProposal{
			GameID: "demo-game", Env: "development",
			ProposalKey: "operation:player.ban", PageKey: draft.PageKey,
			Status: "pending", PageSpec: jsonValue(draft), UpdatedBy: "tester",
		}))

		pageModel := model.NewPageSpecModel(db)
		record, err := pageModel.FindByScopeAndPageKey(ctx, "demo-game", "development", draft.PageKey)
		require.NoError(t, err)
		record.BaseProposalKey = "operation:player.ban"
		record.BaseProposalVersion = 1
		require.NoError(t, pageModel.Upsert(ctx, record))

		_, err = svc.Merge(ctx, &MergeRequest{
			GameID: "demo-game", Env: "development", PageKey: draft.PageKey,
			ExpectedDraftRevision: record.DraftRevision, Strategy: MergeStrategyAuto,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "latest proposal snapshot not found")
	})

	t.Run("missing base snapshot", func(t *testing.T) {
		db := setupTestDB(t)
		draft := seedMergeScenario(t, db,
			spec.LocalizedText{"zh-CN": "标题A"},
			spec.LocalizedText{"zh-CN": "标题A"},
			spec.LocalizedText{"zh-CN": "标题B"})
		setBaseProposalVersion(t, db, draft.PageKey, 9)

		_, err := NewService(db).Merge(context.Background(), &MergeRequest{
			GameID: "demo-game", Env: "development", PageKey: draft.PageKey,
			ExpectedDraftRevision: 1, Strategy: MergeStrategyAuto,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "base proposal snapshot not found")
	})

	t.Run("corrupt base snapshot", func(t *testing.T) {
		db := setupTestDB(t)
		draft := seedMergeScenario(t, db,
			spec.LocalizedText{"zh-CN": "标题A"},
			spec.LocalizedText{"zh-CN": "标题A"},
			spec.LocalizedText{"zh-CN": "标题B"})
		corruptProposalVersionJSON(t, db, 1)

		_, err := NewService(db).Merge(context.Background(), &MergeRequest{
			GameID: "demo-game", Env: "development", PageKey: draft.PageKey,
			ExpectedDraftRevision: 1, Strategy: MergeStrategyAuto,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode proposal snapshot")
	})
}

func TestExtra_Merge_AutoStrategyAppliesSafeChanges(t *testing.T) {
	db := setupTestDB(t)
	draft := seedMergeScenario(t, db,
		spec.LocalizedText{"zh-CN": "标题A"},
		spec.LocalizedText{"zh-CN": "标题A"}, // draft untouched
		spec.LocalizedText{"zh-CN": "标题B"}) // latest advanced

	resp, err := NewService(db).Merge(context.Background(), &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: draft.PageKey,
		ExpectedDraftRevision: 1, Strategy: MergeStrategyAuto,
		Reason: "auto merge display text",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Merged)
	assert.Equal(t, 0, resp.Conflicts)
	assert.Equal(t, 2, resp.DraftRevision)
	require.Len(t, resp.AutoMergeItems, 1)
	assert.Equal(t, "title", resp.AutoMergeItems[0].Field)
}

// seedBindingsScenario builds a fixture whose three-way merge produces a
// single "bindings" conflict (both draft and latest changed them).
func seedBindingsConflictScenario(t *testing.T, db *gorm.DB, draftBindingID, latestBindingID string) spec.PageSpec {
	t.Helper()
	base := extraOperationPage("operation--player.ban")

	draft := base
	draft.Bindings = append(draft.Bindings, spec.PageFunctionBinding{
		ID: draftBindingID, FunctionID: "player.kick", Usage: spec.BindingUsageAction,
	})
	latest := base
	latest.Bindings = append(latest.Bindings, spec.PageFunctionBinding{
		ID: latestBindingID, FunctionID: "player.mute", Usage: spec.BindingUsageAction,
	})

	seedVersioningMergeFixture(t, db, base, draft, latest)
	return draft
}

func TestExtra_Merge_AutoStrategyWithConflictsOnlyReturnsEarly(t *testing.T) {
	db := setupTestDB(t)
	draft := seedBindingsConflictScenario(t, db, "extra-draft", "extra-latest")

	resp, err := NewService(db).Merge(context.Background(), &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: draft.PageKey,
		ExpectedDraftRevision: 1, Strategy: MergeStrategyAuto,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Merged)
	assert.Len(t, resp.ConflictItems, 1)
	assert.Equal(t, 1, resp.DraftRevision, "no revision should be written for conflicts only")

	preview, err := NewService(db).Merge(context.Background(), &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: draft.PageKey,
		ExpectedDraftRevision: 1, Strategy: MergeStrategyAuto, DryRun: true,
	})
	require.NoError(t, err)
	assert.Empty(t, preview.AutoMergeItems)
	require.Len(t, preview.ConflictItems, 1)
}

func TestExtra_Merge_ManualResolutionAppliesCustomValue(t *testing.T) {
	db := setupTestDB(t)
	draft := seedBindingsConflictScenario(t, db, "extra-draft", "extra-latest")

	resolvedBindings := `[{"id":"run","functionId":"player.ban","usage":"action"}]`
	resp, err := NewService(db).Merge(context.Background(), &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: draft.PageKey,
		ExpectedDraftRevision: 1, Strategy: MergeStrategyManual,
		Reason: "keep canonical binding",
		Conflicts: []ConflictResolution{{
			Path:  "bindings",
			Value: json.RawMessage(resolvedBindings),
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Merged)
	assert.Equal(t, 0, resp.Conflicts)
	assert.Equal(t, 2, resp.DraftRevision)

	record, err := model.NewPageSpecModel(db).
		FindByScopeAndPageKey(context.Background(), "demo-game", "development", draft.PageKey)
	require.NoError(t, err)
	saved, err := pageSpecFromModel(record)
	require.NoError(t, err)
	require.Len(t, saved.Bindings, 1)
	assert.Equal(t, "run", saved.Bindings[0].ID)
}
