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

var _ = gorm.ErrRecordNotFound

// ---------------------------------------------------------------------------
// persistMergedDraft / createProposalVersionSnapshot direct units
// ---------------------------------------------------------------------------

func TestExtra_PersistMergedDraft_GuardBranches(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db)

	draft := extraOperationPage("operation--guard")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", draft))
	pageModel := model.NewPageSpecModel(db)
	current, err := pageModel.FindByScopeAndPageKey(ctx, "demo-game", "development", draft.PageKey)
	require.NoError(t, err)

	_, err = svc.persistMergedDraft(ctx, persistMergedDraftParams{Current: current})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "merge request is required")

	_, err = svc.persistMergedDraft(ctx, persistMergedDraftParams{Request: &MergeRequest{}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page draft not found")

	current.BaseProposalKey = "operation:player.ban"
	current.BaseProposalVersion = 2
	revision, err := svc.persistMergedDraft(ctx, persistMergedDraftParams{
		Request:             &MergeRequest{GameID: "demo-game", Env: "development", ExpectedDraftRevision: current.DraftRevision},
		PageKey:             draft.PageKey,
		Current:             current,
		MergedPage:          mustPageSpecFromModel(current),
		ProposalKey:         "operation:player.ban",
		NextBaseProposalVer: 2,
	})
	require.NoError(t, err)
	assert.Equal(t, current.DraftRevision, revision)
}

func TestExtra_CreateProposalVersionSnapshot_UnpersistedProposal(t *testing.T) {
	_, err := createProposalVersionSnapshot(
		context.Background(), nil, &model.PageProposal{}, "reason", "tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "proposal must be persisted before snapshot")
}

// ---------------------------------------------------------------------------
// RegenerateProposal branches
// ---------------------------------------------------------------------------

func TestExtra_RegenerateProposal_EmptyFunctionBinding(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db)

	page := extraOperationPage("operation--orphan")
	page.Bindings = []spec.PageFunctionBinding{{ID: "run", Usage: spec.BindingUsageAction}}
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))

	_, err := svc.RegenerateProposal(ctx, &RegenerateProposalRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no main executable binding")
}

func TestExtra_RegenerateProposal_MissingContract(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db)

	page := extraOperationPage("operation--ghost")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))

	_, err := svc.RegenerateProposal(ctx, &RegenerateProposalRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "main page function contract not found")
}

func TestExtra_RegenerateProposal_UpsertFailsWhenProposalsTableDropped(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	page := extraOperationPage("operation--player.ban")
	seedExtraContract(t, db, "player.ban")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))

	require.NoError(t, db.Exec("DROP TABLE page_proposals").Error)

	_, err := NewService(db).RegenerateProposal(ctx, &RegenerateProposalRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upsert page proposal")
}

func TestExtra_RegenerateProposal_SnapshotFailsWhenVersionsTableDropped(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	page := extraOperationPage("operation--player.ban")
	seedExtraContract(t, db, "player.ban")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))

	require.NoError(t, db.Exec("DROP TABLE page_proposal_versions").Error)

	_, err := NewService(db).RegenerateProposal(ctx, &RegenerateProposalRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "snapshot page proposal")
}

// ---------------------------------------------------------------------------
// RollbackDraft branches
// ---------------------------------------------------------------------------

func seedRollbackFixture(t *testing.T, db *gorm.DB, pageKey string) spec.PageSpec {
	t.Helper()
	page := extraOperationPage(pageKey)
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))
	require.NoError(t, createVersioningTestPageVersion(db, "demo-game", "development", page, 1, "v1"))
	return page
}

func TestExtra_RollbackDraft_TargetVersionMissing(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db)

	page := seedRollbackFixture(t, db, "operation--rb1")

	_, err := svc.RollbackDraft(ctx, &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
		ExpectedDraftRevision: 1, Version: 42,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page version not found")
}

func TestExtra_RollbackDraft_CorruptTargetSnapshot(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db)

	page := seedRollbackFixture(t, db, "operation--rb2")
	require.NoError(t, db.Exec(
		"UPDATE page_versions SET spec_json = '{bad' WHERE page_key = ? AND version = 1",
		page.PageKey).Error)

	_, err := svc.RollbackDraft(ctx, &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
		ExpectedDraftRevision: 1, Version: 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode page version")
}

func TestExtra_RollbackDraft_DraftRowDeleted(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db)

	page := seedRollbackFixture(t, db, "operation--rb3")
	require.NoError(t, db.Exec("DELETE FROM page_specs WHERE page_key = ?", page.PageKey).Error)

	_, err := svc.RollbackDraft(ctx, &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
		ExpectedDraftRevision: 1, Version: 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page draft not found")
}

func TestExtra_RollbackDraft_RevisionConflictAndListError(t *testing.T) {
	t.Run("revision conflict inside transaction", func(t *testing.T) {
		db := setupTestDB(t)
		ctx := context.Background()
		svc := NewService(db)

		page := seedRollbackFixture(t, db, "operation--rb4")
		_, err := svc.RollbackDraft(ctx, &RollbackRequest{
			GameID: "demo-game", Env: "development", PageKey: page.PageKey,
			ExpectedDraftRevision: 99, Version: 1,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "revision conflict")
	})

	t.Run("version listing fails when table dropped", func(t *testing.T) {
		db := setupTestDB(t)
		ctx := context.Background()

		page := seedRollbackFixture(t, db, "operation--rb5")
		require.NoError(t, db.Exec("DROP TABLE page_versions").Error)

		_, err := NewService(db).RollbackDraft(ctx, &RollbackRequest{
			GameID: "demo-game", Env: "development", PageKey: page.PageKey,
			ExpectedDraftRevision: 1, Version: 1,
		})
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// RollbackPublish branches
// ---------------------------------------------------------------------------

func seedPublishedFixture(t *testing.T, db *gorm.DB, pageKey string) spec.PageSpec {
	t.Helper()
	page := seedRollbackFixture(t, db, pageKey)

	raw, err := marshalPageSpec(page)
	require.NoError(t, err)
	publishedModel := model.NewPublishedPageSpecModel(db)
	require.NoError(t, publishedModel.Create(context.Background(), &model.PublishedPageSpec{
		GameID:                "demo-game",
		Env:                   "development",
		PageKey:               pageKey,
		Version:               1,
		SpecJSON:              raw,
		BindingContractsJSON:  "[]",
		RendererSchemaVersion: rendererSchemaVersion,
		BaseProposalKey:       "",
		BaseProposalVersion:   0,
		FunctionDigest:        "digest",
		GeneratorVersion:      "test",
		Active:                true,
		PublishedBy:           "tester",
	}))
	return page
}

func TestExtra_RollbackPublish_TargetVersionMissing(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db)

	page := seedRollbackFixture(t, db, "operation--rp0")

	_, err := svc.RollbackPublish(ctx, &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
		ExpectedDraftRevision: 1, Version: 7,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "published page version not found")
}

func TestExtra_RollbackPublish_CorruptSnapshot(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db)

	page := seedPublishedFixture(t, db, "operation--rp1")
	require.NoError(t, db.Exec(
		"UPDATE published_page_specs SET spec_json = '{bad' WHERE page_key = ? AND version = 1",
		page.PageKey).Error)

	_, err := svc.RollbackPublish(ctx, &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
		ExpectedDraftRevision: 1, Version: 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode published page spec")
}

func TestExtra_RollbackPublish_DraftRowDeleted(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db)

	page := seedPublishedFixture(t, db, "operation--rp2")
	require.NoError(t, db.Exec("DELETE FROM page_specs WHERE page_key = ?", page.PageKey).Error)

	_, err := svc.RollbackPublish(ctx, &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
		ExpectedDraftRevision: 1, Version: 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page draft not found")
}

func TestExtra_RollbackPublish_RevisionConflict(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db)

	page := seedPublishedFixture(t, db, "operation--rp3")

	_, err := svc.RollbackPublish(ctx, &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
		ExpectedDraftRevision: 99, Version: 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "revision conflict")
}

func TestExtra_RollbackPublish_SuccessRestoresPublishedState(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db)

	page := seedPublishedFixture(t, db, "operation--rp4")

	resp, err := svc.RollbackPublish(ctx, &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
		ExpectedDraftRevision: 1, Version: 1,
		Reason: "restore previous release",
	})
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Version)
	assert.Equal(t, 2, resp.DraftRevision)

	record, err := model.NewPageSpecModel(db).
		FindByScopeAndPageKey(ctx, "demo-game", "development", page.PageKey)
	require.NoError(t, err)
	assert.Equal(t, "published", record.Status)
	assert.True(t, record.PublishedActive)
}

// ---------------------------------------------------------------------------
// Republish branches
// ---------------------------------------------------------------------------

func TestExtra_Republish_InvalidStoredSpec(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db)

	page := extraOperationPage("operation--pub1")
	seedExtraContract(t, db, "player.ban")
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))
	require.NoError(t, db.Exec(
		"UPDATE page_specs SET spec_json = '' WHERE page_key = ?", page.PageKey).Error)

	_, err := svc.Republish(ctx, &RepublishRequest{
		GameID: "demo-game", Env: "development", PageKey: page.PageKey,
	})
	require.Error(t, err)
}

func TestExtra_BuildBindingContracts_Errors(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db)

	seedExtraContract(t, db, "player.ban")
	disabled := extraOperationPage("operation--disabled")
	_ = disabled

	t.Run("empty function id", func(t *testing.T) {
		_, err := svc.buildBindingContracts(ctx, "demo-game", "development",
			[]spec.PageFunctionBinding{{ID: "run"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "binding.functionId is required")
	})

	t.Run("missing contract", func(t *testing.T) {
		_, err := svc.buildBindingContracts(ctx, "demo-game", "development",
			[]spec.PageFunctionBinding{{ID: "run", FunctionID: "ghost.fn"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bound function contract does not exist")
	})

	t.Run("disabled contract", func(t *testing.T) {
		require.NoError(t, model.NewFunctionContractModel(db).UpsertContract(ctx,
			&model.FunctionContract{
				GameID: "demo-game", Env: "development", FunctionID: "off.fn",
				Version: "1.0.0", Enabled: false, UpdatedAt: time.Now(),
			}))
		_, err := svc.buildBindingContracts(ctx, "demo-game", "development",
			[]spec.PageFunctionBinding{{ID: "run", FunctionID: "off.fn"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bound function is disabled")
	})

	t.Run("sorts snapshots by binding id", func(t *testing.T) {
		out, err := svc.buildBindingContracts(ctx, "demo-game", "development",
			[]spec.PageFunctionBinding{
				{ID: "zz", FunctionID: "player.ban"},
				{ID: "aa", FunctionID: "player.ban"},
			})
		require.NoError(t, err)
		require.Len(t, out, 2)
		assert.Equal(t, "aa", out[0].BindingID)
		assert.Equal(t, "zz", out[1].BindingID)
	})
}

// ---------------------------------------------------------------------------
// applyConflictField / applyFormFieldConflictValue tables
// ---------------------------------------------------------------------------

func TestExtra_ApplyConflictField_TableDriven(t *testing.T) {
	cases := []struct {
		name      string
		field     string
		raw       string
		wantErr   bool
		errSubstr string
		verify    func(t *testing.T, page spec.PageSpec)
	}{
		{"type ok", "type", `"task"`, false, "", func(t *testing.T, p spec.PageSpec) {
			assert.Equal(t, spec.PageTypeTask, p.Type)
		}},
		{"type bad", "type", `123`, true, "decode manual merge field type", nil},
		{"resourceKey ok", "resourceKey", `"pet"`, false, "", func(t *testing.T, p spec.PageSpec) {
			assert.Equal(t, "pet", p.ResourceKey)
		}},
		{"category.key bad", "category.key", `[]`, true, "decode manual merge field category.key", nil},
		{"bindings ok", "bindings", `[{"id":"a","functionId":"f"}]`, false, "", func(t *testing.T, p spec.PageSpec) {
			require.Len(t, p.Bindings, 1)
			assert.Equal(t, "a", p.Bindings[0].ID)
		}},
		{"bindings bad", "bindings", `{}`, true, "decode manual merge field bindings", nil},
		{"listView identityKey", "resource.listView.identityKey", `"id"`, false, "", nil},
		{"listView rowSchema", "resource.listView.rowSchema", `{"type":"object"}`, false, "", nil},
		{"listView defaultSort ok", "resource.listView.defaultSort", `{"field":"a","order":"asc"}`, false, "", nil},
		{"listView defaultSort bad", "resource.listView.defaultSort", `42`, true, "decode manual merge field resource.listView.defaultSort", nil},
		{"listView filters ok", "resource.listView.filters", `[{"key":"s","type":"select"}]`, false, "", nil},
		{"listView filters bad", "resource.listView.filters", `"x"`, true, "decode manual merge field resource.listView.filters", nil},
		{"listView pagination ok", "resource.listView.pagination", `{"pageSize":20}`, false, "", nil},
		{"listView pagination bad", "resource.listView.pagination", `"x"`, true, "decode manual merge field resource.listView.pagination", nil},
		{"rowActions ok", "resource.listView.rowActions", `[{"key":"edit","title":{"zh-CN":"编辑"}}]`, false, "", nil},
		{"batchActions bad", "resource.listView.batchActions", `"x"`, true, "decode manual merge field resource.listView.batchActions", nil},
		{"toolbarActions ok", "resource.listView.toolbarActions", `[]`, false, "", nil},
		{"detail actions bad", "resource.detailView.actions", `"x"`, true, "decode manual merge field resource.detailView.actions", nil},
		{"resource actions ok", "resource.actions", `[]`, false, "", nil},
		{"createForm ok", "resource.createForm", `{"fields":[]}`, false, "", nil},
		{"updateForm bad", "resource.updateForm", `42`, true, "decode manual merge field resource.updateForm", nil},
		{"deleteAction ok", "resource.deleteAction", `{"title":{"zh-CN":"删除"}}`, false, "", nil},
		{"deleteAction bad", "resource.deleteAction", `42`, true, "decode manual merge field resource.deleteAction", nil},
		{"operation jsonSchema", "operation.form.jsonSchema", `{"type":"object"}`, false, "", nil},
		{"operation confirm bad", "operation.confirm", `42`, true, "decode manual merge field operation.confirm", nil},
		{"operation resultView bad", "operation.resultView", `42`, true, "decode manual merge field operation.resultView", nil},
		{"task jsonSchema", "task.form.jsonSchema", `{}`, false, "", nil},
		{"task taskView bad", "task.taskView", `42`, true, "decode manual merge field task.taskView", nil},
		{"task resultView bad", "task.resultView", `42`, true, "decode manual merge field task.resultView", nil},
		{"report query jsonSchema", "report.queryForm.jsonSchema", `{}`, false, "", nil},
		{"report dataset bad", "report.dataset", `42`, true, "decode manual merge field report.dataset", nil},
		{"report table bad", "report.table", `42`, true, "decode manual merge field report.table", nil},
		{"unsupported field", "totally.unknown", `{}`, true, "unsupported manual merge field", nil},
		{"indexed out of range", "operation.form.fields[5].label", `"x"`, true, "manual merge field is out of range", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page := spec.PageSpec{PageKey: "p"}
			err := applyConflictField(&page, tc.field, json.RawMessage(tc.raw))
			if tc.wantErr {
				require.Error(t, err)
				if tc.errSubstr != "" {
					assert.Contains(t, err.Error(), tc.errSubstr)
				}
				return
			}
			require.NoError(t, err)
			if tc.verify != nil {
				tc.verify(t, page)
			}
		})
	}
}

func TestExtra_ApplyFormFieldConflictValue_Leaves(t *testing.T) {
	baseFields := func() []spec.FormFieldSpec {
		return []spec.FormFieldSpec{{Key: "reason"}}
	}

	cases := []struct {
		name    string
		leaf    string
		raw     string
		wantErr bool
		verify  func(t *testing.T, field spec.FormFieldSpec)
	}{
		{"key ok", "key", `"memo"`, false, func(t *testing.T, f spec.FormFieldSpec) {
			assert.Equal(t, "memo", f.Key)
		}},
		{"visibleWhen ok", "visibleWhen", `{"field":"x","op":"eq","value":1}`, false, func(t *testing.T, f spec.FormFieldSpec) {
			require.NotNil(t, f.VisibleWhen)
		}},
		{"visibleWhen bad", "visibleWhen", `42`, true, nil},
		{"required ok", "required", `true`, false, func(t *testing.T, f spec.FormFieldSpec) {
			require.NotNil(t, f.Required)
			assert.True(t, *f.Required)
		}},
		{"required bad", "required", `"yes"`, true, nil},
		{"defaultValue", "defaultValue", `"abc"`, false, func(t *testing.T, f spec.FormFieldSpec) {
			assert.JSONEq(t, `"abc"`, string(f.DefaultValue))
		}},
		{"disabled ok", "disabled", `true`, false, func(t *testing.T, f spec.FormFieldSpec) {
			require.NotNil(t, f.Disabled)
			assert.True(t, *f.Disabled)
		}},
		{"widgetProps ok", "widgetProps", `{"rows":4}`, false, nil},
		{"widgetProps bad", "widgetProps", `"x"`, true, nil},
		{"validationRules ok", "validationRules", `[{"type":"max","value":10}]`, false, nil},
		{"validationRules bad", "validationRules", `"x"`, true, nil},
		{"unknown leaf", "colour", `"red"`, true, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fields := baseFields()
			full := "operation.form.fields[0]." + tc.leaf
			err := applyFormFieldConflictValue(&fields[0], tc.leaf, json.RawMessage(tc.raw), full)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), full)
				return
			}
			require.NoError(t, err)
			if tc.verify != nil {
				tc.verify(t, fields[0])
			}
		})
	}
}
