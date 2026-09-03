package versioning

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// mergePreviewForPage error branches (reached directly; Diff swallows errors)
// ---------------------------------------------------------------------------

func TestV9_MergePreviewForPage_BaseSnapshotCorrupt(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db)

	draft := seedMergeScenario(t, db,
		spec.LocalizedText{"zh-CN": "标题A"},
		spec.LocalizedText{"zh-CN": "标题A"},
		spec.LocalizedText{"zh-CN": "标题B"})
	corruptProposalVersionJSON(t, db, 1)

	page, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(ctx, "demo-game", "development", draft.PageKey)
	require.NoError(t, err)
	proposal, err := model.NewPageProposalModel(db).FindByScopeAndKey(ctx, "demo-game", "development", "operation:player.ban")
	require.NoError(t, err)

	_, err = svc.mergePreviewForPage(ctx, "demo-game", "development", draft.PageKey, page, proposal)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode proposal snapshot")
}

func TestV9_MergePreviewForPage_LatestProposalSpecCorrupt(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db)

	draft := seedMergeScenario(t, db,
		spec.LocalizedText{"zh-CN": "标题A"},
		spec.LocalizedText{"zh-CN": "标题A"},
		spec.LocalizedText{"zh-CN": "标题B"})
	require.NoError(t, db.Exec("UPDATE page_proposals SET page_spec = '{bad' WHERE proposal_key = ?",
		"operation:player.ban").Error)

	page, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(ctx, "demo-game", "development", draft.PageKey)
	require.NoError(t, err)
	proposal, err := model.NewPageProposalModel(db).FindByScopeAndKey(ctx, "demo-game", "development", "operation:player.ban")
	require.NoError(t, err)

	_, err = svc.mergePreviewForPage(ctx, "demo-game", "development", draft.PageKey, page, proposal)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode proposal PageSpec")
}

// ---------------------------------------------------------------------------
// Merge decode / resolution / persist error branches
// ---------------------------------------------------------------------------

func TestV9_Merge_DraftSpecCorrupt(t *testing.T) {
	db := setupTestDB(t)
	draft := seedMergeScenario(t, db,
		spec.LocalizedText{"zh-CN": "标题A"},
		spec.LocalizedText{"zh-CN": "标题A"},
		spec.LocalizedText{"zh-CN": "标题B"})
	require.NoError(t, db.Exec("UPDATE page_specs SET spec_json = '{bad' WHERE page_key = ?",
		draft.PageKey).Error)

	_, err := NewService(db).Merge(context.Background(), &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: draft.PageKey,
		ExpectedDraftRevision: 1, Strategy: MergeStrategyAuto,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode page spec")
}

func TestV9_Merge_LatestProposalSpecCorrupt(t *testing.T) {
	db := setupTestDB(t)
	draft := seedMergeScenario(t, db,
		spec.LocalizedText{"zh-CN": "标题A"},
		spec.LocalizedText{"zh-CN": "标题A"},
		spec.LocalizedText{"zh-CN": "标题B"})
	require.NoError(t, db.Exec("UPDATE page_proposals SET page_spec = '{bad' WHERE proposal_key = ?",
		"operation:player.ban").Error)

	_, err := NewService(db).Merge(context.Background(), &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: draft.PageKey,
		ExpectedDraftRevision: 1, Strategy: MergeStrategyAuto,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode proposal PageSpec")
}

func TestV9_Merge_ManualResolutionInvalidCustomValue(t *testing.T) {
	db := setupTestDB(t)
	draft := seedBindingsConflictScenario(t, db, "extra-draft", "extra-latest")

	_, err := NewService(db).Merge(context.Background(), &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: draft.PageKey,
		ExpectedDraftRevision: 1, Strategy: MergeStrategyManual,
		Conflicts: []ConflictResolution{{
			Path:      "bindings",
			AcceptNew: false,
			Value:     json.RawMessage(`[123]`),
		}},
	})
	require.Error(t, err)
}

func TestV9_Merge_ManualPersistFails(t *testing.T) {
	db := setupTestDB(t)
	draft := seedBindingsConflictScenario(t, db, "extra-draft2", "extra-latest2")
	require.NoError(t, db.Exec("DROP TABLE page_versions").Error)

	_, err := NewService(db).Merge(context.Background(), &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: draft.PageKey,
		ExpectedDraftRevision: 1, Strategy: MergeStrategyManual,
		Conflicts: []ConflictResolution{{Path: "bindings", AcceptNew: false}},
	})
	require.Error(t, err)
}

func TestV9_Merge_AcceptSnapshotPersistFails(t *testing.T) {
	db := setupTestDB(t)
	base := extraOperationPage("operation--accept-snap")
	latest := base
	latest.Operation = &spec.OperationPageSpec{Form: &spec.FormPresentationSpec{
		JSONSchema: spec.JSONSchema(`{"type":"object"}`),
	}}
	seedVersioningMergeFixture(t, db, base, base, latest)
	require.NoError(t, db.Exec("DROP TABLE page_versions").Error)

	_, err := NewService(db).Merge(context.Background(), &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: base.PageKey,
		ExpectedDraftRevision: 1, Strategy: MergeStrategyAuto,
	})
	require.Error(t, err)
}

func TestV9_Merge_AutoNoOpAutomergeWithConflicts(t *testing.T) {
	db := setupTestDB(t)
	base := extraOperationPage("operation--noop")
	base.Title = spec.LocalizedText{"zh-CN": "标题基线"}

	draft := base
	draft.Title = spec.LocalizedText{"zh-CN": "标题草稿"}
	draft.Bindings = append(draft.Bindings, spec.PageFunctionBinding{
		ID: "extra-draft", FunctionID: "player.kick", Usage: spec.BindingUsageAction,
	})

	latest := base
	latest.Title = spec.LocalizedText{"zh-CN": "标题最新"}
	latest.Bindings = append(latest.Bindings, spec.PageFunctionBinding{
		ID: "extra-latest", FunctionID: "player.mute", Usage: spec.BindingUsageAction,
	})

	seedVersioningMergeFixture(t, db, base, draft, latest)

	resp, err := NewService(db).Merge(context.Background(), &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: base.PageKey,
		ExpectedDraftRevision: 1, Strategy: MergeStrategyAuto,
	})
	require.NoError(t, err)
	// Auto-merge item keeps the draft value (no-op) and a bindings conflict
	// exists, so no new revision is persisted.
	assert.Equal(t, 1, resp.Merged)
	assert.Equal(t, 1, resp.Conflicts)
	assert.Equal(t, 1, resp.DraftRevision)
}

func TestV9_Merge_AutoIndexedColumnsApplied(t *testing.T) {
	db := setupTestDB(t)
	withColumns := func(page spec.PageSpec, title string) spec.PageSpec {
		page.Resource = &spec.ResourcePageSpec{ListView: &spec.ListViewSpec{
			Columns: []spec.ColumnSpec{{Title: spec.LocalizedText{"zh-CN": title}}},
		}}
		return page
	}
	base := withColumns(extraOperationPage("operation--columns"), "列A")
	seedVersioningMergeFixture(t, db, base, base, withColumns(base, "列B"))

	resp, err := NewService(db).Merge(context.Background(), &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: base.PageKey,
		ExpectedDraftRevision: 1, Strategy: MergeStrategyAuto,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resp.Merged, 1)
	assert.Equal(t, 0, resp.Conflicts)

	page, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(context.Background(),
		"demo-game", "development", base.PageKey)
	require.NoError(t, err)
	merged := mustPageSpecFromModel(page)
	require.NotNil(t, merged.Resource)
	require.NotNil(t, merged.Resource.ListView)
	require.Len(t, merged.Resource.ListView.Columns, 1)
	assert.Equal(t, "列B", merged.Resource.ListView.Columns[0].Title["zh-CN"])
}

func TestV9_Merge_AutoPersistFails(t *testing.T) {
	db := setupTestDB(t)
	draft := seedMergeScenario(t, db,
		spec.LocalizedText{"zh-CN": "标题A"},
		spec.LocalizedText{"zh-CN": "标题A"},
		spec.LocalizedText{"zh-CN": "标题B"})
	require.NoError(t, db.Exec("DROP TABLE page_versions").Error)

	_, err := NewService(db).Merge(context.Background(), &MergeRequest{
		GameID: "demo-game", Env: "development", PageKey: draft.PageKey,
		ExpectedDraftRevision: 1, Strategy: MergeStrategyAuto,
	})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// createProposalVersionSnapshot / persistMergedDraft units
// ---------------------------------------------------------------------------

func TestV9_CreateProposalVersionSnapshot_GetNextVersionError(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	proposal := &model.PageProposal{
		GameID: "demo-game", Env: "development",
		ProposalKey: "operation:player.ban", PageKey: "operation--snap",
		Status: dbenum.ProposalStatusPending, UpdatedBy: "tester",
	}
	require.NoError(t, model.NewPageProposalModel(db).UpsertProposal(ctx, proposal))
	require.NotZero(t, proposal.ID)
	require.NoError(t, db.Exec("DROP TABLE page_proposal_versions").Error)

	_, err := createProposalVersionSnapshot(ctx, model.NewPageProposalVersionModel(db), proposal, "reason", "tester")
	require.Error(t, err)
}

func TestV9_PersistMergedDraft_FetchFails(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	draft := extraOperationPage("operation--pm-fetch")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", draft))
	require.NoError(t, db.Exec("DROP TABLE page_specs").Error)

	_, err := NewService(db).persistMergedDraft(ctx, persistMergedDraftParams{
		Request:       &MergeRequest{GameID: "demo-game", Env: "development", ExpectedDraftRevision: 1},
		PageKey:       draft.PageKey,
		Current:       &model.PageSpec{PageKey: draft.PageKey},
		MergedPage:    draft,
		ProposalKey:   "operation:player.ban",
		ForceRevision: true,
	})
	require.Error(t, err)
}

func TestV9_PersistMergedDraft_RevisionConflict(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	draft := extraOperationPage("operation--pm-conflict")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", draft))
	current, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(ctx, "demo-game", "development", draft.PageKey)
	require.NoError(t, err)

	_, err = NewService(db).persistMergedDraft(ctx, persistMergedDraftParams{
		Request:       &MergeRequest{GameID: "demo-game", Env: "development", ExpectedDraftRevision: 99},
		PageKey:       draft.PageKey,
		Current:       current,
		MergedPage:    draft,
		ProposalKey:   "operation:player.ban",
		ForceRevision: true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "revision conflict")
}

func TestV9_PersistMergedDraft_GetNextVersionFails(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	draft := extraOperationPage("operation--pm-next")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", draft))
	current, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(ctx, "demo-game", "development", draft.PageKey)
	require.NoError(t, err)
	require.NoError(t, db.Exec("DROP TABLE page_versions").Error)

	_, err = NewService(db).persistMergedDraft(ctx, persistMergedDraftParams{
		Request:       &MergeRequest{GameID: "demo-game", Env: "development", ExpectedDraftRevision: current.DraftRevision},
		PageKey:       draft.PageKey,
		Current:       current,
		MergedPage:    draft,
		ProposalKey:   "operation:player.ban",
		ForceRevision: true,
	})
	require.Error(t, err)
}

func TestV9_PersistMergedDraft_ApplySpecFails(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	draft := extraOperationPage("operation--pm-apply")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", draft))
	current, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(ctx, "demo-game", "development", draft.PageKey)
	require.NoError(t, err)

	broken := draft
	broken.Operation = &spec.OperationPageSpec{Form: &spec.FormPresentationSpec{
		JSONSchema: spec.JSONSchema("{invalid"),
	}}

	_, err = NewService(db).persistMergedDraft(ctx, persistMergedDraftParams{
		Request:       &MergeRequest{GameID: "demo-game", Env: "development", ExpectedDraftRevision: current.DraftRevision},
		PageKey:       draft.PageKey,
		Current:       current,
		MergedPage:    broken,
		ProposalKey:   "operation:player.ban",
		ForceRevision: true,
	})
	require.Error(t, err)
}

func TestV9_PersistMergedDraft_UpsertFails(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	draft := extraOperationPage("operation--pm-upsert")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", draft))
	current, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(ctx, "demo-game", "development", draft.PageKey)
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TRIGGER v9_no_page_update BEFORE UPDATE ON page_specs BEGIN SELECT RAISE(ABORT, 'blocked'); END").Error)
	require.NoError(t, db.Exec("CREATE TRIGGER v9_no_page_insert BEFORE INSERT ON page_specs BEGIN SELECT RAISE(ABORT, 'blocked'); END").Error)

	_, err = NewService(db).persistMergedDraft(ctx, persistMergedDraftParams{
		Request:       &MergeRequest{GameID: "demo-game", Env: "development", ExpectedDraftRevision: current.DraftRevision},
		PageKey:       draft.PageKey,
		Current:       current,
		MergedPage:    draft,
		ProposalKey:   "operation:player.ban",
		ForceRevision: true,
	})
	require.Error(t, err)
}

func TestV9_PersistMergedDraft_NoRevisionEarlyReturn(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	draft := extraOperationPage("operation--pm-early")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", draft))
	current, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(ctx, "demo-game", "development", draft.PageKey)
	require.NoError(t, err)
	current.BaseProposalKey = ""
	require.NoError(t, model.NewPageSpecModel(db).Upsert(ctx, current))

	revision, err := NewService(db).persistMergedDraft(ctx, persistMergedDraftParams{
		Request:             &MergeRequest{GameID: "demo-game", Env: "development", ExpectedDraftRevision: current.DraftRevision},
		PageKey:             draft.PageKey,
		Current:             current,
		MergedPage:          mustPageSpecFromModel(current),
		ProposalKey:         "operation:player.ban",
		NextBaseProposalVer: 3,
	})
	require.NoError(t, err)
	assert.Equal(t, current.DraftRevision, revision)

	page, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(ctx, "demo-game", "development", draft.PageKey)
	require.NoError(t, err)
	assert.Equal(t, "operation:player.ban", page.BaseProposalKey)
	assert.Equal(t, 3, page.BaseProposalVersion)
}

// ---------------------------------------------------------------------------
// applyConflictField / indexed conflict leaves
// ---------------------------------------------------------------------------

func TestV9_ApplyConflictField_CreateFormDecodeError(t *testing.T) {
	page := spec.PageSpec{PageKey: "operation--cf", Type: spec.PageTypeResource}
	err := applyConflictField(&page, "resource.createForm", json.RawMessage(`[1,2,3]`))
	require.Error(t, err)
}

func TestV9_ApplyIndexedConflictField_TaskFormOutOfRange(t *testing.T) {
	page := spec.PageSpec{PageKey: "task--oob", Type: spec.PageTypeTask}
	page.Task = &spec.TaskPageSpec{Form: &spec.FormPresentationSpec{
		Fields: []spec.FormFieldSpec{{Key: "reason"}},
	}}
	handled, err := applyIndexedConflictField(&page, "task.form.fields[3].key", json.RawMessage(`"k"`))
	require.Error(t, err)
	assert.True(t, handled)
}

func TestV9_ApplyIndexedConflictField_ReportQueryFormOutOfRange(t *testing.T) {
	page := spec.PageSpec{PageKey: "report--oob", Type: spec.PageTypeReport}
	page.Report = &spec.ReportPageSpec{QueryForm: &spec.FormPresentationSpec{
		Fields: []spec.FormFieldSpec{{Key: "date"}},
	}}
	handled, err := applyIndexedConflictField(&page, "report.queryForm.fields[3].key", json.RawMessage(`"k"`))
	require.Error(t, err)
	assert.True(t, handled)
}

func TestV9_ApplyFormFieldConflictValue_DisabledDecodeError(t *testing.T) {
	field := spec.FormFieldSpec{Key: "reason"}
	err := applyFormFieldConflictValue(&field, "disabled", json.RawMessage(`"yes"`), "operation.form.fields[0].disabled")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// marshal error branches (invalid raw JSONSchema payloads)
// ---------------------------------------------------------------------------

func brokenSchemaPageV9() spec.PageSpec {
	page := extraOperationPage("operation--broken")
	page.Operation = &spec.OperationPageSpec{Form: &spec.FormPresentationSpec{
		JSONSchema: spec.JSONSchema("{invalid"),
	}}
	return page
}

func TestV9_SamePageSpec_MarshalErrorLeft(t *testing.T) {
	assert.False(t, samePageSpec(brokenSchemaPageV9(), extraOperationPage("operation--other")))
}

func TestV9_SamePageSpec_MarshalErrorRight(t *testing.T) {
	assert.False(t, samePageSpec(extraOperationPage("operation--other"), brokenSchemaPageV9()))
}

func TestV9_MarshalPageSpec_Error(t *testing.T) {
	_, err := marshalPageSpec(brokenSchemaPageV9())
	require.Error(t, err)
}

func TestV9_ApplyPageSpecToModel_Error(t *testing.T) {
	page := &model.PageSpec{PageKey: "operation--broken"}
	err := applyPageSpecToModel(page, brokenSchemaPageV9())
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// RollbackDraft / RollbackPublish error branches
// ---------------------------------------------------------------------------

func TestV9_RollbackDraft_PageFetchFails(t *testing.T) {
	db := setupTestDB(t)
	page := seedRollbackFixture(t, db, "operation--rd-fetch")
	require.NoError(t, db.Exec("DROP TABLE page_specs").Error)

	_, err := NewService(db).RollbackDraft(context.Background(), &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
		ExpectedDraftRevision: 1, Version: 1,
	})
	require.Error(t, err)
}

func TestV9_RollbackDraft_UpsertFails(t *testing.T) {
	db := setupTestDB(t)
	page := seedRollbackFixture(t, db, "operation--rd-upsert")
	require.NoError(t, db.Exec("CREATE TRIGGER v9_rd_no_update BEFORE UPDATE ON page_specs BEGIN SELECT RAISE(ABORT, 'blocked'); END").Error)
	require.NoError(t, db.Exec("CREATE TRIGGER v9_rd_no_insert BEFORE INSERT ON page_specs BEGIN SELECT RAISE(ABORT, 'blocked'); END").Error)

	_, err := NewService(db).RollbackDraft(context.Background(), &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
		ExpectedDraftRevision: 1, Version: 1,
	})
	require.Error(t, err)
}

func TestV9_RollbackPublish_GetNextVersionFails(t *testing.T) {
	db := setupTestDB(t)
	page := seedPublishedFixture(t, db, "operation--rp-next")
	require.NoError(t, db.Exec("DROP TABLE page_versions").Error)

	_, err := NewService(db).RollbackPublish(context.Background(), &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
		ExpectedDraftRevision: 1, Version: 1,
	})
	require.Error(t, err)
}

func TestV9_RollbackPublish_PageFetchFails(t *testing.T) {
	db := setupTestDB(t)
	page := seedPublishedFixture(t, db, "operation--rp-fetch")
	require.NoError(t, db.Exec("DROP TABLE page_specs").Error)

	_, err := NewService(db).RollbackPublish(context.Background(), &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
		ExpectedDraftRevision: 1, Version: 1,
	})
	require.Error(t, err)
}

func TestV9_RollbackPublish_DeactivateFails(t *testing.T) {
	db := setupTestDB(t)
	page := seedPublishedFixture(t, db, "operation--rp-deact")
	require.NoError(t, db.Exec("CREATE TRIGGER v9_rp_no_pub_update BEFORE UPDATE ON published_page_specs BEGIN SELECT RAISE(ABORT, 'blocked'); END").Error)

	_, err := NewService(db).RollbackPublish(context.Background(), &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
		ExpectedDraftRevision: 1, Version: 1,
	})
	require.Error(t, err)
}

func TestV9_RollbackPublish_PublishedCreateFails(t *testing.T) {
	db := setupTestDB(t)
	page := seedPublishedFixture(t, db, "operation--rp-create")
	require.NoError(t, db.Exec("CREATE TRIGGER v9_rp_no_pub_insert BEFORE INSERT ON published_page_specs BEGIN SELECT RAISE(ABORT, 'blocked'); END").Error)

	_, err := NewService(db).RollbackPublish(context.Background(), &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
		ExpectedDraftRevision: 1, Version: 1,
	})
	require.Error(t, err)
}

func TestV9_RollbackPublish_PageUpsertFails(t *testing.T) {
	db := setupTestDB(t)
	page := seedPublishedFixture(t, db, "operation--rp-upsert")
	require.NoError(t, db.Exec("CREATE TRIGGER v9_rp_no_page_update BEFORE UPDATE ON page_specs BEGIN SELECT RAISE(ABORT, 'blocked'); END").Error)
	require.NoError(t, db.Exec("CREATE TRIGGER v9_rp_no_page_insert BEFORE INSERT ON page_specs BEGIN SELECT RAISE(ABORT, 'blocked'); END").Error)

	_, err := NewService(db).RollbackPublish(context.Background(), &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
		ExpectedDraftRevision: 1, Version: 1,
	})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// RegenerateProposal / Republish error branches
// ---------------------------------------------------------------------------

func TestV9_RegenerateProposal_EmptyPageKey(t *testing.T) {
	db := setupTestDB(t)
	_, err := NewService(db).RegenerateProposal(context.Background(), &RegenerateProposalRequest{
		GameID: "demo-game", Env: "development", PageKey: "   ",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pageKey is required")
}

func TestV9_RegenerateProposal_ResourceRebuildFails(t *testing.T) {
	db := setupTestDB(t)
	page := extraOperationPage("resource--player")
	page.Type = spec.PageTypeResource
	page.ResourceKey = "player"
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))
	require.NoError(t, db.Exec("DROP TABLE capability_semantics").Error)

	_, err := NewService(db).RegenerateProposal(context.Background(), &RegenerateProposalRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "regenerate proposal")
}

func TestV9_Republish_GetNextVersionFails(t *testing.T) {
	db := setupTestDB(t)
	page := extraOperationPage("operation--pub-next")
	seedExtraContract(t, db, "player.ban")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))
	require.NoError(t, db.Exec("DROP TABLE page_versions").Error)

	_, err := NewService(db).Republish(context.Background(), &RepublishRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
	})
	require.Error(t, err)
}

func TestV9_Republish_DeactivateFails(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	page := extraOperationPage("operation--pub-deact")
	seedExtraContract(t, db, "player.ban")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))
	raw, err := marshalPageSpec(page)
	require.NoError(t, err)
	require.NoError(t, model.NewPublishedPageSpecModel(db).Create(ctx, &model.PublishedPageSpec{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
		Version: 1, SpecJSON: raw, Active: true, PublishedBy: "tester",
	}))
	require.NoError(t, db.Exec("CREATE TRIGGER v9_pub_no_update BEFORE UPDATE ON published_page_specs BEGIN SELECT RAISE(ABORT, 'blocked'); END").Error)

	_, err = NewService(db).Republish(ctx, &RepublishRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
	})
	require.Error(t, err)
}

func TestV9_Republish_PublishedCreateFails(t *testing.T) {
	db := setupTestDB(t)
	page := extraOperationPage("operation--pub-create")
	seedExtraContract(t, db, "player.ban")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))
	require.NoError(t, db.Exec("CREATE TRIGGER v9_pub_no_insert BEFORE INSERT ON published_page_specs BEGIN SELECT RAISE(ABORT, 'blocked'); END").Error)

	_, err := NewService(db).Republish(context.Background(), &RepublishRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
	})
	require.Error(t, err)
}

func TestV9_Republish_PageUpsertFails(t *testing.T) {
	db := setupTestDB(t)
	page := extraOperationPage("operation--pub-upsert")
	seedExtraContract(t, db, "player.ban")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))
	require.NoError(t, db.Exec("CREATE TRIGGER v9_pub_no_page_update BEFORE UPDATE ON page_specs BEGIN SELECT RAISE(ABORT, 'blocked'); END").Error)
	require.NoError(t, db.Exec("CREATE TRIGGER v9_pub_no_page_insert BEFORE INSERT ON page_specs BEGIN SELECT RAISE(ABORT, 'blocked'); END").Error)

	_, err := NewService(db).Republish(context.Background(), &RepublishRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
	})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// GetChangeChain: proposal version current-state + published version items
// ---------------------------------------------------------------------------

func TestV9_GetChangeChain_ProposalVersionAndPublishedItem(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	page := extraOperationPage("operation--chain")
	seedExtraContract(t, db, "player.ban")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))
	require.NoError(t, createVersioningTestPageVersion(db, "demo-game", "development", page, 1, "seed draft"))
	require.NoError(t, db.Exec("UPDATE page_versions SET status = 'published' WHERE page_key = ?", page.PageKey).Error)

	proposal := &model.PageProposal{
		GameID: "demo-game", Env: "development",
		ProposalKey: "operation:player.ban", PageKey: page.PageKey,
		Status: dbenum.ProposalStatusPending, UpdatedBy: "tester",
	}
	require.NoError(t, model.NewPageProposalModel(db).UpsertProposal(ctx, proposal))
	require.NotZero(t, proposal.ID)
	_, err := createProposalVersionSnapshot(ctx, model.NewPageProposalVersionModel(db), proposal, "seed", "tester")
	require.NoError(t, err)

	chain, err := NewService(db).GetChangeChain(ctx, &GetChangeChainRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, chain.Current.ProposalVersion)

	hasPublish := false
	for _, item := range chain.Items {
		if item.Type == ChangeTypePublish {
			hasPublish = true
		}
	}
	assert.True(t, hasPublish)
}

// ---------------------------------------------------------------------------
// DeletePage / CreateCompositePage service branches
// ---------------------------------------------------------------------------

func TestV9_DeletePage_EmptyPageKey(t *testing.T) {
	db := setupTestDB(t)
	err := NewService(db).DeletePage(context.Background(), "demo-game", "development", "  ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pageKey is required")
}

func TestV9_DeletePage_DraftDeleteFails(t *testing.T) {
	db := setupTestDB(t)
	page := extraOperationPage("operation--del1")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))
	require.NoError(t, db.Exec("DROP TABLE page_specs").Error)

	err := NewService(db).DeletePage(context.Background(), "demo-game", "development", page.PageKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete page draft")
}

func TestV9_DeletePage_PublishedDeleteFails(t *testing.T) {
	db := setupTestDB(t)
	page := extraOperationPage("operation--del2")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))
	require.NoError(t, db.Exec("DROP TABLE published_page_specs").Error)

	err := NewService(db).DeletePage(context.Background(), "demo-game", "development", page.PageKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete published page")
}

func TestV9_DeletePage_ProposalDeleteFails(t *testing.T) {
	db := setupTestDB(t)
	page := extraOperationPage("operation--del3")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))
	require.NoError(t, db.Exec("DROP TABLE page_proposals").Error)

	err := NewService(db).DeletePage(context.Background(), "demo-game", "development", page.PageKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete page proposals")
}

func TestV9_DeletePage_Success(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	page := extraOperationPage("operation--del4")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))
	require.NoError(t, createVersioningTestPageVersion(db, "demo-game", "development", page, 1, "seed"))
	raw, err := marshalPageSpec(page)
	require.NoError(t, err)
	require.NoError(t, model.NewPublishedPageSpecModel(db).Create(ctx, &model.PublishedPageSpec{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
		Version: 1, SpecJSON: raw, Active: true,
	}))
	require.NoError(t, model.NewPageProposalModel(db).UpsertProposal(ctx, &model.PageProposal{
		GameID: "demo-game", Env: "development",
		ProposalKey: "operation:player.ban", PageKey: page.PageKey,
		Status: dbenum.ProposalStatusPending, UpdatedBy: "tester",
	}))

	require.NoError(t, NewService(db).DeletePage(ctx, "demo-game", "development", page.PageKey))

	_, err = model.NewPageSpecModel(db).FindByScopeAndPageKey(ctx, "demo-game", "development", page.PageKey)
	require.Error(t, err)
}

func TestV9_CreateCompositePage_ValidationError(t *testing.T) {
	db := setupTestDB(t)
	_, err := NewService(db).CreateCompositePage(context.Background(),
		"demo-game", "development", "composite--one", nil)
	require.Error(t, err)
}
