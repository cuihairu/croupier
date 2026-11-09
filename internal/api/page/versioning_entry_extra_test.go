package page

// service.go 2050-2290 版本化/入口发布补测：PublishVersioningDraft 的
// internal-only 拦截臂、ValidateVersioningRollbackTarget 的可达臂、
// publishEntryCore 的 caller 分派/作用域/actor/req 错误臂、
// RepublishDraftFromEntry 的完整错误面与成功路径、
// PublishExistingDraftForProposal 成功路径、pagePublishSource 的形态臂。
//
// 诚实跳过：
//   - PublishVersioningDraft 成功臂（2073-2077）：公开面恒不置 InternalCaller，
//     publishEntryCore 对 versioning.republish 强制 internal-only（2155-2157），
//     公开调用必走错误返回，成功路径仅内部调用方可达，无测试入口。
//   - ValidateVersioningRollbackTarget 的 buildBindingContracts 错误臂
//     （2095-2097）与 pageJSONMarshal 错误臂（2098-2100）：buildBindingContracts
//     失败仅当绑定引用不存在函数，而 validateBinding 先以
//     binding_function_not_found 诊断使其落入 diags>0 分支（两者互斥）；
//     contracts 为可序列化结构体，marshal 恒成功。
//   - publishEntryCore 的 empty VersionRollback case（2159）无语句体，
//     fallthrough 到作用域解析即被后续分支覆盖，无独立可测行为。

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// noScopeCtx：无 GameScope/username 的裸上下文（requireScope/currentUsername 失败臂）。
func noScopeCtx() context.Context { return context.Background() }

// scopeOnlyCtx：有 scope 无 username（currentUsername 失败臂）。
func scopeOnlyCtx() context.Context {
	return svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
}

// PublishVersioningDraft 的 internal-only 拦截（2058-2072 + 2155-2157）。
func TestPublishVersioningDraft_InternalOnly(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	rev := saveTestPageDraft(t, service, ctx)

	_, err := service.PublishVersioningDraft(ctx, "demo-game", "development", "page_tester", "player.manage", rev, "test reason")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `entry "versioning.republish" is internal-only`)
}

// ValidateVersioningRollbackTarget：成功、目标版本不存在、验证失败三臂。
func TestValidateVersioningRollbackTarget_Arms(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	db := service.svcCtx.DB
	rev := saveTestPageDraft(t, service, ctx)
	_, err := service.Publish(ctx, &PagePublishRequest{PageKey: "player.manage", DraftRevision: &rev})
	require.NoError(t, err)

	// 成功臂（2081-2086, 2095, 2101）
	require.NoError(t, service.ValidateVersioningRollbackTarget(ctx, "demo-game", "development", "player.manage", rev))

	// 目标版本不存在 → FindByScopePageKeyAndVersion 错误（2083-2085）
	err = service.ValidateVersioningRollbackTarget(ctx, "demo-game", "development", "player.manage", 999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "published page version not found")

	// 验证失败臂（2091-2093）：删除函数契约后绑定引用不存在函数 →
	// binding_function_not_found 诊断 → diags>0 → 验证错误
	require.NoError(t, db.Where("game_id = ? AND env = ?", "demo-game", "development").
		Delete(&model.FunctionContract{}).Error)
	err = service.ValidateVersioningRollbackTarget(ctx, "demo-game", "development", "player.manage", rev)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page validation failed")
}

// publishEntryCore 的 caller 分派错误臂。
func TestPublishFromEntry_CallerArms(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	rev := saveTestPageDraft(t, service, ctx)
	req := &PagePublishRequest{PageKey: "player.manage", DraftRevision: &rev}

	// ProposalAccept + InternalCaller → 禁止绕过（2148-2150）
	_, err := service.PublishFromEntry(ctx, pagePublishEntry{Caller: pagePublishCallerProposalAccept, InternalCaller: true}, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "proposal accept cannot bypass")

	// AutoPublish + 非 internal → internal-only（2155-2157）
	_, err = service.PublishFromEntry(ctx, pagePublishEntry{Caller: pagePublishCallerAutoPublish}, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is internal-only")

	// 未知 caller（2160-2161）
	_, err = service.PublishFromEntry(ctx, pagePublishEntry{Caller: "bogus.caller"}, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown publish entry")

	// API + 全字段 → 成功（2154, 2166-2179, publishCore）
	resp, err := service.PublishFromEntry(ctx, pagePublishEntry{
		Caller: pagePublishCallerAPI, GameID: "demo-game", Env: "development", Actor: "page_tester",
	}, req)
	require.NoError(t, err)
	assert.True(t, resp.Published)
}

// publishEntryCore 的作用域/actor/req 错误臂。
func TestPublishFromEntry_ScopeActorReqArms(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	rev := 1
	req := &PagePublishRequest{PageKey: "player.manage", DraftRevision: &rev}

	// 空 GameID/Env + 无 scope → requireScope 失败（2164-2167）
	_, err := service.PublishFromEntry(noScopeCtx(), pagePublishEntry{Caller: pagePublishCallerVersionRollback}, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Game-ID is required")

	// 空 Actor + scope 无 username → currentUsername 失败（2171-2174）
	_, err = service.PublishFromEntry(scopeOnlyCtx(), pagePublishEntry{
		Caller: pagePublishCallerVersionRollback, GameID: "demo-game", Env: "development",
	}, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未找到登录用户")

	// req nil → bad request（2177-2179）
	_, err = service.PublishFromEntry(scopeOnlyCtx(), pagePublishEntry{Caller: pagePublishCallerAPI, GameID: "demo-game", Env: "development", Actor: "a"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "publish request is required")
}

// RepublishDraftFromEntry 的完整错误面与成功路径。
func TestRepublishDraftFromEntry_Arms(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	db := service.svcCtx.DB
	rev := saveTestPageDraft(t, service, ctx)

	// req nil（2192-2194）
	_, err := service.RepublishDraftFromEntry(ctx, pagePublishEntry{Caller: pagePublishCallerRepublish}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "republish request is required")

	// pageKey 空（2201-2203）
	_, err = service.RepublishDraftFromEntry(ctx, pagePublishEntry{Caller: pagePublishCallerRepublish, GameID: "demo-game", Env: "development", Actor: "a"},
		&RepublishDraftRequest{PageKey: "   "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pageKey is required")

	// 空 GameID/Env + 无 scope → requireScope 失败（2206-2208）
	_, err = service.RepublishDraftFromEntry(noScopeCtx(), pagePublishEntry{Caller: pagePublishCallerRepublish},
		&RepublishDraftRequest{PageKey: "player.manage"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Game-ID is required")

	// 空 Actor + scope 无 username → currentUsername 失败（2212-2214）
	_, err = service.RepublishDraftFromEntry(scopeOnlyCtx(), pagePublishEntry{Caller: pagePublishCallerRepublish, GameID: "demo-game", Env: "development"},
		&RepublishDraftRequest{PageKey: "player.manage"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未找到登录用户")

	// caller 不允许重发布（2220-2221）
	_, err = service.RepublishDraftFromEntry(ctx, pagePublishEntry{Caller: pagePublishCallerAPI, GameID: "demo-game", Env: "development", Actor: "a"},
		&RepublishDraftRequest{PageKey: "player.manage"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot republish a draft")

	// FindByScopeAndPageKey 失败 → 页面不存在（2225-2227）
	_, err = service.RepublishDraftFromEntry(ctx, pagePublishEntry{Caller: pagePublishCallerRepublish, GameID: "demo-game", Env: "development", Actor: "a"},
		&RepublishDraftRequest{PageKey: "no.such.page"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page not found: no.such.page")

	// 草稿版本冲突（2228-2233）
	stale := 5
	_, err = service.RepublishDraftFromEntry(ctx, pagePublishEntry{Caller: pagePublishCallerRepublish, GameID: "demo-game", Env: "development", Actor: "a"},
		&RepublishDraftRequest{PageKey: "player.manage", ExpectedDraftRevision: &stale})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "草稿版本冲突")

	// publishCore 失败（2238-2240）：删除契约 → 绑定引用不存在函数 → 校验失败
	require.NoError(t, db.Where("game_id = ? AND env = ?", "demo-game", "development").
		Delete(&model.FunctionContract{}).Error)
	_, err = service.RepublishDraftFromEntry(ctx, pagePublishEntry{Caller: pagePublishCallerRepublish, GameID: "demo-game", Env: "development", Actor: "a"},
		&RepublishDraftRequest{PageKey: "player.manage", ExpectedDraftRevision: &rev})
	require.Error(t, err)

	// 成功 + 审计（2241-2247）：新环境（契约齐备）重发布
	service2, ctx2, auditStore2 := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	rev2 := saveTestPageDraft(t, service2, ctx2)
	resp, err := service2.RepublishDraftFromEntry(ctx2, pagePublishEntry{Caller: pagePublishCallerRepublish, GameID: "demo-game", Env: "development", Actor: "page_tester"},
		&RepublishDraftRequest{PageKey: "player.manage", ExpectedDraftRevision: &rev2})
	require.NoError(t, err)
	assert.True(t, resp.Published)
	records, total, err := auditStore2.List(audit.AuditFilter{EventType: []audit.AuditEventType{audit.EventPagePublish}}, audit.AuditPage{PageSize: 10})
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, 1)
	found := false
	for _, r := range records {
		if r.Details["action"] == "republish_draft" {
			found = true
			break
		}
	}
	assert.True(t, found, "republish 须记录独立审计（publishCore 基底 + 本方法明细）")
}

// RepublishDraftFromEntry 的 RequiresPublish 权限失败臂（2196-2198）。
func TestRepublishDraftFromEntry_RequiresPublishPermission(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit") // 无 pages:publish
	saveTestPageDraft(t, service, ctx)

	_, err := service.RepublishDraftFromEntry(ctx, pagePublishEntry{
		Caller: pagePublishCallerRepublish, RequiresPublish: true, GameID: "demo-game", Env: "development", Actor: "a",
	}, &RepublishDraftRequest{PageKey: "player.manage"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权发布页面")
}

// PublishExistingDraftForProposal 成功路径（2155 通过 + 2250-2263 审计）。
func TestPublishExistingDraftForProposal_Success(t *testing.T) {
	service, ctx, auditStore := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	rev := saveTestPageDraft(t, service, ctx)

	resp, err := service.PublishExistingDraftForProposal(ctx, "demo-game", "development", "page_tester", "player.manage", rev, "proposal-1")
	require.NoError(t, err)
	assert.True(t, resp.Published)
	assert.Equal(t, rev, resp.PublishedVersion)

	records, total, err := auditStore.List(audit.AuditFilter{EventType: []audit.AuditEventType{audit.EventPagePublish}}, audit.AuditPage{PageSize: 10})
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, 1)
	found := false
	for _, r := range records {
		if r.Details["action"] == "auto_publish_composite" && r.Details["proposal"] == "proposal-1" {
			found = true
			break
		}
	}
	assert.True(t, found, "auto publish 须记录独立审计（publishCore 基底 + 本方法明细）")
}

// pagePublishSource 的 nil receiver / nil page / 无提案 key / 未找到提案 /
// 找到提案填摘要 五形态（2273-2288）。
func TestPagePublishSource_Arms(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	db := service.svcCtx.DB

	// nil receiver
	var nilS *Service
	assert.Equal(t, pagePublishSource{}, nilS.pagePublishSource(ctx, "demo-game", "development", &model.PageSpec{}))

	// nil page
	assert.Equal(t, pagePublishSource{}, service.pagePublishSource(ctx, "demo-game", "development", nil))

	// BaseProposalKey 空 → 早退（2277-2280）
	src := service.pagePublishSource(ctx, "demo-game", "development", &model.PageSpec{PageKey: "p"})
	assert.Empty(t, src.BaseProposalKey)

	// BaseProposalKey 有值但提案不存在 → 原样返回（2282-2283）
	src = service.pagePublishSource(ctx, "demo-game", "development", &model.PageSpec{BaseProposalKey: "no-such"})
	assert.Equal(t, "no-such", src.BaseProposalKey)
	assert.Empty(t, src.FunctionDigest)

	// 提案存在 → 填摘要（2285-2288）
	require.NoError(t, db.Create(&model.PageProposal{
		GameID: "demo-game", Env: "development", ProposalKey: "proposal-9", PageKey: "player.manage",
		FunctionDigest: "fd-1", SemanticsDigest: "sd-1", GeneratorVersion: "gv-1",
		Title:       datatypes.JSONMap{"zh-CN": "玩家管理"},
		PageSpec:    model.JSON(`{}`),
		Diagnostics: model.JSON(`[]`),
		Status:      dbenum.ProposalStatusPending,
		UpdatedBy:   "tester",
	}).Error)
	src = service.pagePublishSource(ctx, "demo-game", "development", &model.PageSpec{BaseProposalKey: "proposal-9", BaseProposalVersion: 1})
	assert.Equal(t, "fd-1", src.FunctionDigest)
	assert.Equal(t, "sd-1", src.SemanticsDigest)
	assert.Equal(t, "gv-1", src.GeneratorVersion)
	assert.Equal(t, "proposal-9", src.BaseProposalKey)
}
