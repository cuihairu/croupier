package page

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// gorm 注入辅助（按 Dest 类型 + 发生序号）
// ---------------------------------------------------------------------------

func pageDestIs[T any](tx *gorm.DB) bool {
	if tx.Statement == nil || tx.Statement.Dest == nil {
		return false
	}
	switch tx.Statement.Dest.(type) {
	case *T, *[]T, *[]*T:
		return true
	}
	return false
}

func injectFinalQueryFailure(t *testing.T, db *gorm.DB, name string, match func(*gorm.DB) bool, occurrence int) {
	t.Helper()
	var n int
	require.NoError(t, db.Callback().Query().Before("gorm:query").
		Register(name, func(tx *gorm.DB) {
			if !match(tx) {
				return
			}
			n++
			if n >= occurrence {
				tx.AddError(errors.New("injected query failure"))
			}
		}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove(name) })
}

func injectFinalCreateFailure(t *testing.T, db *gorm.DB, name string, match func(*gorm.DB) bool) {
	t.Helper()
	require.NoError(t, db.Callback().Create().Before("gorm:create").
		Register(name, func(tx *gorm.DB) {
			if match(tx) {
				tx.AddError(errors.New("injected create failure"))
			}
		}))
	t.Cleanup(func() { _ = db.Callback().Create().Remove(name) })
}

func injectFinalUpdateFailure(t *testing.T, db *gorm.DB, name string, match func(*gorm.DB) bool) {
	t.Helper()
	require.NoError(t, db.Callback().Update().Before("gorm:update").
		Register(name, func(tx *gorm.DB) {
			if match(tx) {
				tx.AddError(errors.New("injected update failure"))
			}
		}))
	t.Cleanup(func() { _ = db.Callback().Update().Remove(name) })
}

func tableIs(tx *gorm.DB, name string) bool {
	return tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == name
}

func isPageSpecDest(tx *gorm.DB) bool { return pageDestIs[model.PageSpec](tx) }
func isPageVersionDest(tx *gorm.DB) bool {
	return pageDestIs[model.PageVersion](tx)
}

// ---------------------------------------------------------------------------
// SaveDraft / RegenerateDraft 错误分支
// ---------------------------------------------------------------------------

// SaveDraft：非法 JSONSchema（RawMessage）使 marshalPageSpec 失败。
func TestFinalSaveDraft_InvalidSchemaMarshal(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	rev := 0
	resp, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "bad-schema-page",
		DraftRevision: &rev,
		Type:          spec.PageTypeResource,
		Title:         map[string]string{"zh-CN": "页面"},
		Category:      spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{RowSchema: spec.JSONSchema("{invalid")},
		},
	})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// SaveDraft 事务内 pageModel.Upsert 失败。
func TestFinalSaveDraft_UpsertError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	injectFinalCreateFailure(t, service.svcCtx.DB, "test.fail.ps.create", isPageSpecDest)

	rev := 0
	resp, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "upsert-fail-page",
		DraftRevision: &rev,
		Type:          spec.PageTypeOperation,
		Title:         map[string]string{"zh-CN": "页面"},
		Category:      spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
		Operation:     testOperationPageSpec(),
		Bindings:      testPageBindings(),
	})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// RegenerateDraft：先保存草稿，再注入 create 失败。
func TestFinalRegenerate_UpsertError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	seedResourceProposal(t, service)

	const (
		gameID = "demo-game"
		env    = "development"
		key    = "resource:player"
	)
	proposal, err := model.NewPageProposalModel(service.svcCtx.DB).FindByScopeAndKey(ctx, gameID, env, key)
	require.NoError(t, err)
	var replacement struct {
		PageKey string `json:"pageKey"`
	}
	require.NoError(t, json.Unmarshal(proposal.PageSpec, &replacement))
	require.NotEmpty(t, replacement.PageKey)

	require.NoError(t, service.svcCtx.DB.Create(&model.PageSpec{
		GameID:          gameID,
		Env:             env,
		PageKey:         replacement.PageKey,
		CategoryKey:     "player",
		SpecJSON:        string(proposal.PageSpec),
		Status:          "draft",
		DraftRevision:   1,
		BaseProposalKey: key,
	}).Error)

	injectFinalUpdateFailure(t, service.svcCtx.DB, "test.fail.ps.update0", isPageSpecDest)

	resp, err := service.RegenerateDraft(ctx, &PageRegenerateRequest{PageKey: replacement.PageKey, DraftRevision: intPtrFinal(1)})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// ---------------------------------------------------------------------------
// Publish 错误分支
// ---------------------------------------------------------------------------

func finalPublishedDraft(t *testing.T, service *Service, ctx context.Context) int {
	t.Helper()
	rev := saveTestPageDraft(t, service, ctx)
	_, err := service.Publish(ctx, &PagePublishRequest{PageKey: "player.manage", DraftRevision: intPtrFinal(rev)})
	require.NoError(t, err)
	return rev
}

// Publish：第二次读取函数契约失败 → buildBindingContracts 找不到函数。
func TestFinalPublish_ContractReloadFailure(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish", "pages:edit")
	rev := saveTestPageDraft(t, service, ctx)

	var contractQueries int
	injectFinalQueryFailure(t, service.svcCtx.DB, "test.fail.fc.query", func(tx *gorm.DB) bool {
		return pageDestIs[model.FunctionContract](tx)
	}, 2)

	// 需要知道第一次函数契约查询发生在哪之前：validatePageSpec 内为 #1，
	// normalizedFunctions(#2) 注错 → buildBindingContracts 报错。
	_ = contractQueries
	resp, err := service.Publish(ctx, &PagePublishRequest{PageKey: "player.manage", DraftRevision: intPtrFinal(rev)})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// Publish 事务内：最新草稿读取失败。
func TestFinalPublish_TxFindFailure(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish", "pages:edit")
	rev := saveTestPageDraft(t, service, ctx)

	var pageSpecQueries int
	name := "test.fail.ps.query"
	require.NoError(t, service.svcCtx.DB.Callback().Query().Before("gorm:query").
		Register(name, func(tx *gorm.DB) {
			if !isPageSpecDest(tx) {
				return
			}
			pageSpecQueries++
			if pageSpecQueries >= 2 {
				tx.AddError(errors.New("injected query failure"))
			}
		}))
	t.Cleanup(func() { _ = service.svcCtx.DB.Callback().Query().Remove(name) })

	resp, err := service.Publish(ctx, &PagePublishRequest{PageKey: "player.manage", DraftRevision: intPtrFinal(rev)})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// Publish 事务内：读取后在同事务内推进 draft_revision → 冲突分支。
func TestFinalPublish_TxRevisionConflict(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish", "pages:edit")
	rev := saveTestPageDraft(t, service, ctx)

	var pageSpecQueries int
	name := "test.bump.ps.revision"
	require.NoError(t, service.svcCtx.DB.Callback().Query().Before("gorm:query").
		Register(name, func(tx *gorm.DB) {
			if !isPageSpecDest(tx) {
				return
			}
			pageSpecQueries++
			if pageSpecQueries == 2 {
				_ = tx.Session(&gorm.Session{NewDB: true}).
					Exec("UPDATE page_specs SET draft_revision = draft_revision + 1").Error
			}
		}))
	t.Cleanup(func() { _ = service.svcCtx.DB.Callback().Query().Remove(name) })

	resp, err := service.Publish(ctx, &PagePublishRequest{PageKey: "player.manage", DraftRevision: intPtrFinal(rev)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "草稿版本冲突")
	assert.Nil(t, resp)
}

// Publish 事务内：DeactivatePage 失败（published 表更新）。
func TestFinalPublish_DeactivateFailure(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish", "pages:edit")
	rev := saveTestPageDraft(t, service, ctx)
	// 先成功发布一次，使 published 记录存在。
	_, err := service.Publish(ctx, &PagePublishRequest{PageKey: "player.manage", DraftRevision: intPtrFinal(rev)})
	require.NoError(t, err)
	// 重新保存草稿形成新版本再发布。
	rev2 := rev
	_, err2 := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "player.manage",
		DraftRevision: &rev2,
		Type:          spec.PageTypeOperation,
		Title:         map[string]string{"zh-CN": "页面"},
		Category:      spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
		Operation:     testOperationPageSpec(),
		Bindings:      testPageBindings(),
	})
	require.NoError(t, err2)

	require.NoError(t, service.svcCtx.DB.Callback().Update().Before("gorm:update").
		Register("test.fail.pub.update", func(tx *gorm.DB) {
			if tableIs(tx, "published_page_specs") {
				tx.AddError(errors.New("injected update failure"))
			}
		}))
	t.Cleanup(func() { _ = service.svcCtx.DB.Callback().Update().Remove("test.fail.pub.update") })

	resp, err := service.Publish(ctx, &PagePublishRequest{PageKey: "player.manage", DraftRevision: intPtrFinal(rev2 + 1)})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// Publish 事务内：pageModel.Upsert（page_specs update）失败。
func TestFinalPublish_PageUpsertFailure(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish", "pages:edit")
	rev := saveTestPageDraft(t, service, ctx)
	injectFinalUpdateFailure(t, service.svcCtx.DB, "test.fail.ps.update2", isPageSpecDest)

	resp, err := service.Publish(ctx, &PagePublishRequest{PageKey: "player.manage", DraftRevision: intPtrFinal(rev)})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// ---------------------------------------------------------------------------
// Unpublish / Rollback 错误分支
// ---------------------------------------------------------------------------

func TestFinalUnpublish_UpsertError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish", "pages:edit")
	finalPublishedDraft(t, service, ctx)
	injectFinalUpdateFailure(t, service.svcCtx.DB, "test.fail.ps.update3", isPageSpecDest)

	resp, err := service.Unpublish(ctx, &PageUnpublishRequest{PageKey: "player.manage"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// Rollback：目标版本的 SpecJSON 中 pageKey 为空 → buildPageSpecJSON 失败。
func TestFinalRollback_BuildSpecError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish", "pages:edit", "pages:rollback")
	rev := saveTestPageDraft(t, service, ctx)

	// 直接把 1 号版本改成 pageKey 为空的合法 JSON。
	broken := `{"pageKey":"","type":"operation","title":{"zh-CN":"t"},"category":{"key":"player","labels":{"zh-CN":"玩家"}},"bindings":[]}`
	require.NoError(t, service.svcCtx.DB.Exec(
		"UPDATE page_versions SET spec_json = ? WHERE game_id = ? AND env = ? AND page_key = ? AND version = 1",
		broken, "demo-game", "development", "player.manage").Error)

	expected := rev
	resp, err := service.Rollback(ctx, &PageRollbackRequest{
		PageKey: "player.manage", VersionID: "1", ExpectedDraftRevision: &expected,
	})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// Rollback：pageModel.Upsert 失败。
func TestFinalRollback_UpsertError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish", "pages:edit", "pages:rollback")
	rev := saveTestPageDraft(t, service, ctx)
	injectFinalUpdateFailure(t, service.svcCtx.DB, "test.fail.ps.update1", isPageSpecDest)

	expected := rev
	resp, err := service.Rollback(ctx, &PageRollbackRequest{
		PageKey: "player.manage", VersionID: "1", ExpectedDraftRevision: &expected,
	})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// Rollback：versionModel.Upsert（page_versions create）失败。
func TestFinalRollback_VersionUpsertError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish", "pages:edit", "pages:rollback")
	rev := saveTestPageDraft(t, service, ctx)
	injectFinalCreateFailure(t, service.svcCtx.DB, "test.fail.pv.create", isPageVersionDest)

	expected := rev
	resp, err := service.Rollback(ctx, &PageRollbackRequest{
		PageKey: "player.manage", VersionID: "1", ExpectedDraftRevision: &expected,
	})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// ---------------------------------------------------------------------------
// applyPageSpecToModel / marshalPageSpec 单元分支
// ---------------------------------------------------------------------------

func TestFinalApplyPageSpecToModel_MarshalError(t *testing.T) {
	p := &model.PageSpec{}
	ps := spec.PageSpec{
		PageKey:  "k",
		Type:     spec.PageTypeResource,
		Title:    spec.LocalizedText{"zh-CN": "标题"},
		Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{RowSchema: spec.JSONSchema("{invalid")},
		},
	}
	err := applyPageSpecToModel(p, ps)
	require.Error(t, err)
}

func TestFinalMarshalPageSpec_InvalidSchema(t *testing.T) {
	_, err := marshalPageSpec(spec.PageSpec{
		PageKey:  "k",
		Type:     spec.PageTypeResource,
		Title:    spec.LocalizedText{"zh-CN": "标题"},
		Category: spec.PageCategorySpec{Key: "player"},
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{RowSchema: spec.JSONSchema("{invalid")},
		},
	})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Bulk / Seed 错误分支
// ---------------------------------------------------------------------------

func TestFinalRebuildAllProposals_CapabilityTableMissing(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("resource_capabilities"))

	resp, err := service.RebuildAllProposals(ctx)
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestFinalBulkPublish_RebuildError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish")
	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("resource_capabilities"))

	resp, err := service.BulkPublish(ctx, &PageBulkRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestFinalBulkPublish_ListProposalsError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish", "pages:edit")
	// Rebuild 之后 ListProposalsByStatus 查询失败：注入 page_proposals 首次查询错误。
	injectFinalQueryFailure(t, service.svcCtx.DB, "test.fail.pp.query", func(tx *gorm.DB) bool {
		return pageDestIs[model.PageProposal](tx)
	}, 1)

	resp, err := service.BulkPublish(ctx, &PageBulkRequest{})
	// 若 Rebuild 内部也读 page_proposals，则错误来自 Rebuild，同样是目标分支之一。
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestFinalBulkUnpublish_ListError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish", "pages:edit")
	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("page_specs"))

	resp, err := service.BulkUnpublish(ctx, &PageBulkRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestFinalBulkUnpublish_UnpublishFailure(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish", "pages:edit")
	finalPublishedDraft(t, service, ctx)
	injectFinalUpdateFailure(t, service.svcCtx.DB, "test.fail.ps.update4", isPageSpecDest)

	resp, err := service.BulkUnpublish(ctx, &PageBulkRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Failed, 1)
	assert.Equal(t, "player.manage", resp.Failed[0]["pageKey"])
}

func TestFinalSeedDemoData_TermUpsertError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish", "pages:edit")
	v9bEnableTermDict(service)
	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("term_dictionary"))

	resp, err := service.SeedDemoData(ctx, &PageSeedDemoRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func intPtrFinal(v int) *int { return &v }
