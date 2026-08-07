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
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&model.FunctionContract{},
		&model.ResourceCapability{},
		&model.CapabilitySemantics{},
		&model.CapabilitySemanticVersion{},
		&model.PageProposal{},
		&model.PageProposalVersion{},
		&model.PageSpec{},
		&model.PublishedPageSpec{},
		&model.PageVersion{},
	)
	require.NoError(t, err)

	return db
}

func createVersioningTestPage(db *gorm.DB, gameID, env string, page spec.PageSpec) error {
	raw, err := json.Marshal(page)
	if err != nil {
		return err
	}
	modelPage := &model.PageSpec{
		GameID:        gameID,
		Env:           env,
		PageKey:       page.PageKey,
		Type:          string(page.Type),
		ResourceKey:   page.ResourceKey,
		CategoryKey:   page.Category.Key,
		CategoryOrder: page.Category.Order,
		Order:         page.Order,
		SpecJSON:      string(raw),
		Status:        "draft",
		DraftRevision: 1,
		UpdatedAt:     time.Now(),
	}
	if err := modelPage.SetTitle(page.Title); err != nil {
		return err
	}
	if err := modelPage.SetCategoryLabels(page.Category.Labels); err != nil {
		return err
	}
	return model.NewPageSpecModel(db).Upsert(context.Background(), modelPage)
}

func createVersioningTestPageVersion(
	db *gorm.DB,
	gameID, env string,
	page spec.PageSpec,
	version int,
	message string,
) error {
	raw, err := marshalPageSpec(page)
	if err != nil {
		return err
	}
	return model.NewPageVersionModel(db).UpsertByScopePageKeyVersion(context.Background(), &model.PageVersion{
		GameID:    gameID,
		Env:       env,
		PageKey:   page.PageKey,
		Version:   version,
		SpecJSON:  raw,
		Status:    "draft",
		Message:   message,
		CreatedBy: "tester",
		CreatedAt: time.Now(),
	})
}

func seedVersioningMergeFixture(
	t *testing.T,
	db *gorm.DB,
	basePage spec.PageSpec,
	draftPage spec.PageSpec,
	latestPage spec.PageSpec,
) {
	t.Helper()

	ctx := context.Background()
	proposalModel := model.NewPageProposalModel(db)
	proposalVersionModel := model.NewPageProposalVersionModel(db)
	pageModel := model.NewPageSpecModel(db)

	proposalKey := proposalKeyForPage(basePage.Type, basePage.Bindings[0].FunctionID)
	proposal := &model.PageProposal{
		GameID:           "demo-game",
		Env:              "development",
		ProposalKey:      proposalKey,
		PageKey:          basePage.PageKey,
		PageType:         string(basePage.Type),
		ResourceKey:      basePage.ResourceKey,
		Quality:          "basic",
		GeneratorVersion: "test",
		FunctionDigest:   "digest-v1",
		Title:            localizedTextToJSONMap(basePage.Title),
		Description:      localizedTextToJSONMap(basePage.Description),
		CategoryKey:      basePage.Category.Key,
		PageSpec:         jsonValue(basePage),
		Status:           "pending",
		UpdatedAt:        time.Now(),
		UpdatedBy:        "tester",
	}
	require.NoError(t, proposalModel.UpsertProposal(ctx, proposal))
	_, err := createProposalVersionSnapshot(ctx, proposalVersionModel, proposal, "seed base proposal", "tester")
	require.NoError(t, err)

	proposal.FunctionDigest = "digest-v2"
	proposal.Title = localizedTextToJSONMap(latestPage.Title)
	proposal.Description = localizedTextToJSONMap(latestPage.Description)
	proposal.CategoryKey = latestPage.Category.Key
	proposal.PageSpec = jsonValue(latestPage)
	proposal.UpdatedAt = time.Now().Add(time.Second)
	proposal.UpdatedBy = "tester"
	require.NoError(t, proposalModel.UpsertProposal(ctx, proposal))
	_, err = createProposalVersionSnapshot(ctx, proposalVersionModel, proposal, "seed latest proposal", "tester")
	require.NoError(t, err)

	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", draftPage))
	pageRecord, err := pageModel.FindByScopeAndPageKey(ctx, "demo-game", "development", draftPage.PageKey)
	require.NoError(t, err)
	pageRecord.BaseProposalKey = proposalKey
	pageRecord.BaseProposalVersion = 1
	pageRecord.DraftRevision = 1
	pageRecord.UpdatedBy = "tester"
	pageRecord.UpdatedAt = time.Now()
	require.NoError(t, pageModel.Upsert(ctx, pageRecord))
	require.NoError(t, createVersioningTestPageVersion(db, "demo-game", "development", draftPage, 1, "seed draft"))
}

func boolPtr(value bool) *bool {
	return &value
}

func TestVersioningService_GetChangeChain(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Create some contracts
	contractModel := model.NewFunctionContractModel(db)
	err := contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID:      "demo-game",
		Env:         "development",
		FunctionID:  "player.list",
		Version:     "1.0.0",
		Enabled:     true,
		ResourceKey: "player",
		Capability:  "collection_query",
		UpdatedAt:   time.Now(),
	})
	require.NoError(t, err)
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", spec.PageSpec{
		PageKey:     "resource--player",
		Type:        spec.PageTypeResource,
		ResourceKey: "player",
		Title:       spec.LocalizedText{"zh-CN": "Player"},
		Category:    spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "Player"}},
		Bindings: []spec.PageFunctionBinding{{
			ID:         "query",
			FunctionID: "player.list",
			Usage:      spec.BindingUsageQuery,
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
	}))

	// Get change chain
	chain, err := service.GetChangeChain(ctx, &GetChangeChainRequest{
		GameID:  "demo-game",
		Env:     "development",
		PageKey: "resource--player",
	})
	require.NoError(t, err)
	assert.NotNil(t, chain)
	assert.Equal(t, "resource--player", chain.PageKey)
	assert.Equal(t, "player", chain.ResourceKey)
	assert.Len(t, chain.Items, 1)
}

func TestVersioningService_Diff(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Create semantics
	semanticsModel := model.NewCapabilitySemanticsModel(db)
	err := semanticsModel.UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID:        "demo-game",
		Env:           "development",
		ResourceKey:   "player",
		IdentityField: "player_id",
	})
	require.NoError(t, err)
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", spec.PageSpec{
		PageKey:     "resource--player",
		Type:        spec.PageTypeResource,
		ResourceKey: "player",
		Title:       spec.LocalizedText{"zh-CN": "Player"},
		Category:    spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "Player"}},
	}))

	// Get diff
	diff, err := service.Diff(ctx, &DiffRequest{
		GameID:      "demo-game",
		Env:         "development",
		PageKey:     "resource--player",
		FromVersion: 1,
		ToVersion:   1,
	})
	require.NoError(t, err)
	assert.NotNil(t, diff)
	assert.Contains(t, diff.Summary, "changes")
}

func TestVersioningService_MergeReject(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Test reject merge (should work without any data)
	result, err := service.Merge(ctx, &MergeRequest{
		GameID:   "demo-game",
		Env:      "development",
		PageKey:  "resource--player",
		Strategy: MergeStrategyReject,
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "all changes rejected", result.Message)
}

func TestVersioningService_MergeAutoNoDraft(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Test auto merge without draft should fail
	_, err := service.Merge(ctx, &MergeRequest{
		GameID:   "demo-game",
		Env:      "development",
		PageKey:  "resource--player",
		Strategy: MergeStrategyAuto,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "page draft not found")
}

func TestVersioningService_MergeManualRequiresExactConflictResolutions(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	basePage := spec.PageSpec{
		PageKey:  "operation--player.ban",
		Type:     spec.PageTypeOperation,
		Title:    spec.LocalizedText{"zh-CN": "封禁玩家"},
		Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
		Bindings: []spec.PageFunctionBinding{{
			ID:         "run",
			FunctionID: "player.ban",
			Usage:      spec.BindingUsageAction,
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
		Operation: &spec.OperationPageSpec{
			Form: &spec.FormPresentationSpec{
				JSONSchema: spec.JSONSchema(`{"type":"object","properties":{"playerId":{"type":"string"}}}`),
				Fields: []spec.FormFieldSpec{{
					Key:      "playerId",
					Label:    spec.LocalizedText{"zh-CN": "玩家ID"},
					Required: boolPtr(true),
				}},
			},
			Confirm: &spec.ConfirmActionSpec{
				Title:       spec.LocalizedText{"zh-CN": "确认封禁"},
				ConfirmText: spec.LocalizedText{"zh-CN": "确认"},
				BindingID:   "run",
			},
		},
	}
	draftPage := basePage
	draftPage.Operation = &spec.OperationPageSpec{
		Form: basePage.Operation.Form,
		Confirm: &spec.ConfirmActionSpec{
			Title:       spec.LocalizedText{"zh-CN": "人工确认封禁"},
			ConfirmText: spec.LocalizedText{"zh-CN": "确认"},
			BindingID:   "run",
		},
	}
	latestPage := basePage
	latestPage.Title = spec.LocalizedText{"zh-CN": "封禁玩家操作"}
	latestPage.Operation = &spec.OperationPageSpec{
		Form: basePage.Operation.Form,
		Confirm: &spec.ConfirmActionSpec{
			Title:       spec.LocalizedText{"zh-CN": "封禁前确认"},
			ConfirmText: spec.LocalizedText{"zh-CN": "确认"},
			BindingID:   "run",
		},
	}
	seedVersioningMergeFixture(t, db, basePage, draftPage, latestPage)

	_, err := service.Merge(ctx, &MergeRequest{
		GameID:   "demo-game",
		Env:      "development",
		PageKey:  "operation--player.ban",
		Strategy: MergeStrategyManual,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "manual merge must resolve exactly the current conflicts")
	assert.Contains(t, err.Error(), "operation.confirm")
}

func TestVersioningService_MergeManualDryRunReturnsPreview(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	basePage := spec.PageSpec{
		PageKey:  "operation--player.ban",
		Type:     spec.PageTypeOperation,
		Title:    spec.LocalizedText{"zh-CN": "封禁玩家"},
		Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
		Bindings: []spec.PageFunctionBinding{{
			ID:         "run",
			FunctionID: "player.ban",
			Usage:      spec.BindingUsageAction,
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
		Operation: &spec.OperationPageSpec{
			Form: &spec.FormPresentationSpec{
				JSONSchema: spec.JSONSchema(`{"type":"object","properties":{"playerId":{"type":"string"}}}`),
				Fields: []spec.FormFieldSpec{{
					Key:      "playerId",
					Label:    spec.LocalizedText{"zh-CN": "玩家ID"},
					Required: boolPtr(true),
				}},
			},
			Confirm: &spec.ConfirmActionSpec{
				Title:       spec.LocalizedText{"zh-CN": "确认封禁"},
				ConfirmText: spec.LocalizedText{"zh-CN": "确认"},
				BindingID:   "run",
			},
		},
	}
	draftPage := basePage
	draftPage.Operation = &spec.OperationPageSpec{
		Form: basePage.Operation.Form,
		Confirm: &spec.ConfirmActionSpec{
			Title:       spec.LocalizedText{"zh-CN": "人工确认封禁"},
			ConfirmText: spec.LocalizedText{"zh-CN": "确认"},
			BindingID:   "run",
		},
	}
	latestPage := basePage
	latestPage.Title = spec.LocalizedText{"zh-CN": "封禁玩家操作"}
	latestPage.Operation = &spec.OperationPageSpec{
		Form: basePage.Operation.Form,
		Confirm: &spec.ConfirmActionSpec{
			Title:       spec.LocalizedText{"zh-CN": "封禁前确认"},
			ConfirmText: spec.LocalizedText{"zh-CN": "确认"},
			BindingID:   "run",
		},
	}
	seedVersioningMergeFixture(t, db, basePage, draftPage, latestPage)

	result, err := service.Merge(ctx, &MergeRequest{
		GameID:   "demo-game",
		Env:      "development",
		PageKey:  "operation--player.ban",
		Strategy: MergeStrategyManual,
		DryRun:   true,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Merged)
	assert.Equal(t, 1, result.Conflicts)
	require.Len(t, result.AutoMergeItems, 1)
	require.Len(t, result.ConflictItems, 1)
	assert.Equal(t, "operation.confirm", result.ConflictItems[0].Field)

	pageRecord, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(ctx, "demo-game", "development", "operation--player.ban")
	require.NoError(t, err)
	assert.Equal(t, 1, pageRecord.DraftRevision)
	assert.Equal(t, 1, pageRecord.BaseProposalVersion)
}

func TestVersioningService_MergeManualAppliesAutoAndConflictResolutions(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	basePage := spec.PageSpec{
		PageKey:  "operation--player.ban",
		Type:     spec.PageTypeOperation,
		Title:    spec.LocalizedText{"zh-CN": "封禁玩家"},
		Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
		Bindings: []spec.PageFunctionBinding{{
			ID:         "run",
			FunctionID: "player.ban",
			Usage:      spec.BindingUsageAction,
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
		Operation: &spec.OperationPageSpec{
			Form: &spec.FormPresentationSpec{
				JSONSchema: spec.JSONSchema(`{"type":"object","properties":{"playerId":{"type":"string"}}}`),
				Fields: []spec.FormFieldSpec{{
					Key:      "playerId",
					Label:    spec.LocalizedText{"zh-CN": "玩家ID"},
					Required: boolPtr(true),
				}},
			},
			Confirm: &spec.ConfirmActionSpec{
				Title:       spec.LocalizedText{"zh-CN": "确认封禁"},
				ConfirmText: spec.LocalizedText{"zh-CN": "确认"},
				BindingID:   "run",
			},
		},
	}
	draftPage := basePage
	draftPage.Operation = &spec.OperationPageSpec{
		Form: basePage.Operation.Form,
		Confirm: &spec.ConfirmActionSpec{
			Title:       spec.LocalizedText{"zh-CN": "人工确认封禁"},
			ConfirmText: spec.LocalizedText{"zh-CN": "确认"},
			BindingID:   "run",
		},
	}
	latestPage := basePage
	latestPage.Title = spec.LocalizedText{"zh-CN": "封禁玩家操作"}
	latestPage.Operation = &spec.OperationPageSpec{
		Form: basePage.Operation.Form,
		Confirm: &spec.ConfirmActionSpec{
			Title:       spec.LocalizedText{"zh-CN": "封禁前确认"},
			ConfirmText: spec.LocalizedText{"zh-CN": "确认"},
			BindingID:   "run",
		},
	}
	seedVersioningMergeFixture(t, db, basePage, draftPage, latestPage)

	result, err := service.Merge(ctx, &MergeRequest{
		GameID:   "demo-game",
		Env:      "development",
		PageKey:  "operation--player.ban",
		Strategy: MergeStrategyManual,
		Conflicts: []ConflictResolution{{
			Path:      "operation.confirm",
			AcceptNew: true,
		}},
		Reason: "manual review",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, result.Conflicts)
	assert.Equal(t, 2, result.DraftRevision)
	assert.Contains(t, result.Message, "resolved 1 conflicts")

	pageRecord, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(ctx, "demo-game", "development", "operation--player.ban")
	require.NoError(t, err)
	assert.Equal(t, 2, pageRecord.DraftRevision)
	assert.Equal(t, 2, pageRecord.BaseProposalVersion)

	pageSpec, err := pageSpecFromModel(pageRecord)
	require.NoError(t, err)
	assert.Equal(t, latestPage.Title["zh-CN"], pageSpec.Title["zh-CN"])
	require.NotNil(t, pageSpec.Operation)
	require.NotNil(t, pageSpec.Operation.Confirm)
	assert.Equal(t, latestPage.Operation.Confirm.Title["zh-CN"], pageSpec.Operation.Confirm.Title["zh-CN"])

	versions, err := model.NewPageVersionModel(db).ListByScopeAndPageKey(ctx, "demo-game", "development", "operation--player.ban")
	require.NoError(t, err)
	require.Len(t, versions, 2)
	assert.Equal(t, 2, versions[0].Version)
	assert.Equal(t, "manual review", versions[0].Message)
}

func TestVersioningService_RollbackDraftNoData(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Rollback draft without data should fail
	_, err := service.RollbackDraft(ctx, &RollbackRequest{
		GameID:  "demo-game",
		Env:     "development",
		PageKey: "resource--player",
		Version: 1,
		Reason:  "test rollback",
	})
	assert.Error(t, err)
}

func TestVersioningService_RollbackPublishNoData(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Rollback publish without data should fail
	_, err := service.RollbackPublish(ctx, &RollbackRequest{
		GameID:  "demo-game",
		Env:     "development",
		PageKey: "resource--player",
		Version: 1,
		Reason:  "test rollback",
	})
	assert.Error(t, err)
}

func TestVersioningService_RegenerateProposal(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Create semantics and contracts first
	semanticsModel := model.NewCapabilitySemanticsModel(db)
	err := semanticsModel.UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID:      "demo-game",
		Env:         "development",
		ResourceKey: "player",
	})
	require.NoError(t, err)

	contractModel := model.NewFunctionContractModel(db)
	err = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID:      "demo-game",
		Env:         "development",
		FunctionID:  "player.list",
		Version:     "1.0.0",
		Enabled:     true,
		ResourceKey: "player",
		Capability:  "collection_query",
		UpdatedAt:   time.Now(),
	})
	require.NoError(t, err)
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", spec.PageSpec{
		PageKey:     "resource--player",
		Type:        spec.PageTypeResource,
		ResourceKey: "player",
		Title:       spec.LocalizedText{"zh-CN": "Player"},
		Category:    spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "Player"}},
		Bindings: []spec.PageFunctionBinding{{
			ID:         "query",
			FunctionID: "player.list",
			Usage:      spec.BindingUsageQuery,
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
	}))

	// Regenerate proposal
	result, err := service.RegenerateProposal(ctx, &RegenerateProposalRequest{
		GameID:  "demo-game",
		Env:     "development",
		PageKey: "resource--player",
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Message, "regenerated")
}

func TestVersioningService_RepublishNoDraft(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db)

	// Republish without draft should fail
	_, err := service.Republish(ctx, &RepublishRequest{
		GameID:  "demo-game",
		Env:     "development",
		PageKey: "resource--player",
		Reason:  "test republish",
	})
	assert.Error(t, err)
}

func TestProposalKeyForPage(t *testing.T) {
	assert.Equal(t, "operation:mail.send", proposalKeyForPage(spec.PageTypeOperation, "mail.send"))
	assert.Equal(t, "task:reward.batch", proposalKeyForPage(spec.PageTypeTask, "reward.batch"))
	assert.Equal(t, "report:analytics.retention", proposalKeyForPage(spec.PageTypeReport, "analytics.retention"))
	assert.Empty(t, proposalKeyForPage(spec.PageTypeOperation, ""))
}

func TestMergeMessage(t *testing.T) {
	msg := mergeMessage(0, 0, false)
	assert.Equal(t, "no contract changes require merge", msg)

	msg2 := mergeMessage(1, 0, false)
	assert.Contains(t, msg2, "found 1 safe changes")

	msg3 := mergeMessage(0, 2, false)
	assert.Contains(t, msg3, "2 conflicts")

	msg4 := mergeMessage(1, 0, true)
	assert.Contains(t, msg4, "auto-merged 1")

	msg5 := mergeMessage(1, 1, true)
	assert.Contains(t, msg5, "auto-merged 1")
	assert.Contains(t, msg5, "1 conflicts")
}

func TestJsonString(t *testing.T) {
	assert.Equal(t, json.RawMessage(`"hello"`), jsonString("hello"))
	assert.Equal(t, json.RawMessage(`""`), jsonString(""))
	assert.Equal(t, json.RawMessage(`"test"`), jsonString("test"))
}

func TestFirstNonEmpty(t *testing.T) {
	assert.Equal(t, "a", firstNonEmpty("a", "b", "c"))
	assert.Equal(t, "b", firstNonEmpty("", "b", "c"))
	assert.Equal(t, "c", firstNonEmpty("", "", "c"))
	assert.Empty(t, firstNonEmpty("", "", ""))
}

func TestSamePageSpec(t *testing.T) {
	// Test empty specs
	spec1 := spec.PageSpec{}
	spec2 := spec.PageSpec{}
	assert.True(t, samePageSpec(spec1, spec2))

	// Test with same page key
	spec3 := spec.PageSpec{PageKey: "test"}
	spec4 := spec.PageSpec{PageKey: "test"}
	assert.True(t, samePageSpec(spec3, spec4))

	// Test with different page key
	spec5 := spec.PageSpec{PageKey: "test1"}
	spec6 := spec.PageSpec{PageKey: "test2"}
	assert.False(t, samePageSpec(spec5, spec6))
}

func TestDigestRaw(t *testing.T) {
	digest1 := digestRaw([]byte("test"))
	digest2 := digestRaw([]byte("test"))
	assert.Equal(t, digest1, digest2)

	digest3 := digestRaw([]byte("different"))
	assert.NotEqual(t, digest1, digest3)
}

func TestPageSpecFromProposalSnapshot(t *testing.T) {
	tests := []struct {
		merged    int
		conflicts int
		changed   bool
		expected  string
	}{
		{0, 0, false, "no contract changes require merge"},
		{1, 0, false, "found 1 safe changes and 0 conflicts; no draft change written"},
		{0, 2, false, "found 0 safe changes and 2 conflicts; no draft change written"},
		{1, 0, true, "auto-merged 1 safe changes"},
		{1, 1, true, "auto-merged 1 safe changes; 1 conflicts still require manual review"},
	}

	for _, tt := range tests {
		result := mergeMessage(tt.merged, tt.conflicts, tt.changed)
		assert.Equal(t, tt.expected, result)
	}
}

func TestDigestRawV3(t *testing.T) {
	// Test same input
	d1 := digestRaw([]byte("test"))
	d2 := digestRaw([]byte("test"))
	assert.Equal(t, d1, d2)

	// Test different input
	d3 := digestRaw([]byte("different"))
	assert.NotEqual(t, d1, d3)
}

func TestFirstNonEmptyV3(t *testing.T) {
	assert.Equal(t, "a", firstNonEmpty("a", "b", "c"))
	assert.Equal(t, "b", firstNonEmpty("", "b", "c"))
	assert.Equal(t, "c", firstNonEmpty("", "", "c"))
	assert.Empty(t, firstNonEmpty("", "", ""))
}

func TestSamePageSpecV3(t *testing.T) {
	// Test empty specs
	spec1 := spec.PageSpec{}
	spec2 := spec.PageSpec{}
	assert.True(t, samePageSpec(spec1, spec2))

	// Test same page key
	spec3 := spec.PageSpec{PageKey: "test"}
	spec4 := spec.PageSpec{PageKey: "test"}
	assert.True(t, samePageSpec(spec3, spec4))

	// Test different page key
	spec5 := spec.PageSpec{PageKey: "test1"}
	spec6 := spec.PageSpec{PageKey: "test2"}
	assert.False(t, samePageSpec(spec5, spec6))
}

func TestNewService(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	assert.NotNil(t, service)
	assert.NotNil(t, service.db)
	assert.NotNil(t, service.contractModel)
	assert.NotNil(t, service.semanticsModel)
	assert.NotNil(t, service.proposalModel)
	assert.NotNil(t, service.proposalVersionModel)
}
