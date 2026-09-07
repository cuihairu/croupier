package service

// 本文件只做覆盖率补洞（gap-fill）：针对 contract_service.go /
// proposal_service.go 中尚未覆盖的可达分支补充用例。不修改产品代码。

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// --- helpers ---------------------------------------------------------------

// matchTable 在 gorm:query / gorm:create 等回调触发点解析语句目标表：
// Query 链中 Statement.Table 可能尚未解析（Scan/Model 延迟 parse）。
func matchTable(tx *gorm.DB, table string) bool {
	if tx.Statement == nil {
		return false
	}
	if tx.Statement.Table == "" && tx.Statement.Model != nil {
		_ = tx.Statement.Parse(tx.Statement.Model)
	}
	return tx.Statement.Table == table
}

// injectWriteFailCallback 让指定表的 gorm 写入（create/update）失败，读放行。
func injectWriteFailCallback(db *gorm.DB, table string) (remove func()) {
	name := "test.failwrite." + table
	fail := func(tx *gorm.DB) {
		if matchTable(tx, table) {
			_ = tx.AddError(errors.New("injected failure for " + table))
		}
	}
	_ = db.Callback().Create().Before("gorm:create").Register(name, fail)
	_ = db.Callback().Update().Before("gorm:update").Register(name, fail)
	return func() {
		_ = db.Callback().Create().Remove(name)
		_ = db.Callback().Update().Remove(name)
	}
}

// injectFailCallback 让指定表的 gorm 语句（增删改查）失败。
func injectFailCallback(db *gorm.DB, table string) (remove func()) {
	name := "test.fail." + table
	fail := func(tx *gorm.DB) {
		if matchTable(tx, table) {
			_ = tx.AddError(errors.New("injected failure for " + table))
		}
	}
	_ = db.Callback().Create().Before("gorm:create").Register(name, fail)
	_ = db.Callback().Update().Before("gorm:update").Register(name, fail)
	// gorm 的 Scan 走 Row 回调链（非 Query），Find 走 Query 链：两处都拦。
	_ = db.Callback().Query().Before("gorm:query").Register(name, fail)
	_ = db.Callback().Row().Before("gorm:row").Register(name, fail)
	_ = db.Callback().Delete().Before("gorm:delete").Register(name, fail)
	return func() {
		_ = db.Callback().Create().Remove(name)
		_ = db.Callback().Update().Remove(name)
		_ = db.Callback().Query().Remove(name)
		_ = db.Callback().Row().Remove(name)
		_ = db.Callback().Delete().Remove(name)
	}
}

func mustPersistProposal(t *testing.T, svc *ProposalService, page spec.PageSpec, quality string) {
	t.Helper()
	pageJSON, err := json.Marshal(page)
	require.NoError(t, err)
	require.NoError(t, svc.proposalModel.UpsertProposal(context.Background(), &model.PageProposal{
		GameID:      "demo-game",
		Env:         "development",
		ProposalKey: "key-" + page.PageKey,
		PageKey:     page.PageKey,
		PageType:    string(page.Type),
		Quality:     quality,
		Status:      dbenum.ProposalStatusPending,
		PageSpec:    pageJSON,
	}))
}

// setupPublishableProposal 建立可发布的 operation 提案（action binding +
// 启用的 mail.send 契约），返回 pageKey。
func setupPublishableProposal(t *testing.T, db *gorm.DB, svc *ProposalService, pageKey string) spec.PageSpec {
	t.Helper()
	ctx := proposalTestContext()
	contract := &model.FunctionContract{
		GameID: "demo-game", Env: "development",
		FunctionID: "mail.send", Version: "1.0.0", Enabled: true,
		Capability:  dbenum.CapabilityAction,
		Execution:   "sync",
		ResourceKey: "mail",
		InputSchema: model.JSON(`{"type":"object"}`),
	}
	require.NoError(t, model.NewFunctionContractModel(db).UpsertContract(ctx, contract))

	page := spec.PageSpec{
		PageKey:     pageKey,
		Type:        spec.PageTypeOperation,
		ResourceKey: "mail",
		Title:       spec.LocalizedText{"zh-CN": "发送邮件"},
		Category: spec.PageCategorySpec{
			Key: "mail", Labels: spec.LocalizedText{"zh-CN": "邮件"},
		},
		Operation: &spec.OperationPageSpec{
			Form: &spec.FormPresentationSpec{JSONSchema: spec.JSONSchema(`{"type":"object"}`)},
		},
		Bindings: []spec.PageFunctionBinding{{
			ID:         "main",
			FunctionID: "mail.send",
			Usage:      spec.BindingUsageAction,
			Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
	}
	mustPersistProposal(t, svc, page, "ready")
	return page
}

// --- contract_service.go ---------------------------------------------------

// L971-975: RebuildProposalsForResource digest 变化 + UpsertSemantics 失败。
func TestRebuildProposalsForResource_UpsertSemanticsError(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewContractService(db)

	// 先正常 rebuild 一次，建立 semantics。
	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", FunctionMetaInput{
		ID: "player.list", Version: "1.0.0", Enabled: true,
		Resource: "player", Operation: "list",
		Capability: "collection_query", Execution: "sync",
		InputSchema:  `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`,
		OutputSchema: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"}}}}}}`,
	}))
	require.NoError(t, svc.RebuildResourceCapability(ctx, "demo-game", "development", "player"))
	require.NoError(t, svc.RebuildProposalsForResource(ctx, "demo-game", "development", "player"))

	// 修改契约使 digest 变化，然后注入 capability_semantics 写失败。
	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", FunctionMetaInput{
		ID: "player.list", Version: "1.1.0", Enabled: true,
		Resource: "player", Operation: "list",
		Capability: "collection_query", Execution: "sync",
		InputSchema:  `{"type":"object","properties":{"id":{"type":"string"},"q":{"type":"string"}},"required":["id"]}`,
		OutputSchema: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"q":{"type":"string"}}}}}}`,
	}))

	remove := injectWriteFailCallback(db, "capability_semantics")
	defer remove()
	err := svc.RebuildProposalsForResource(ctx, "demo-game", "development", "player")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update semantics digest")
}

// L1082-1084: GenerateResourcePageProposal 返回 !ok（CollectionQueryID=0）
// 且 removeResourceProposal 失败。
func TestUpsertResourceProposal_GenerateNotOKRemoveError(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewContractService(db)

	remove := injectFailCallback(db, "page_proposals")
	defer remove()
	sem := &model.CapabilitySemantics{
		GameID: "demo-game", Env: "development", ResourceKey: "player",
		IdentityField: "id",
	}
	_, err := svc.upsertResourceProposal(ctx, "demo-game", "development", sem, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete resource proposal")
}

// L1085: GenerateResourcePageProposal 返回 !ok 时清空 resource proposal。
func TestUpsertResourceProposal_GenerateNotOK(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewContractService(db)

	sem := &model.CapabilitySemantics{
		GameID: "demo-game", Env: "development", ResourceKey: "player",
		IdentityField: "id",
	}
	consumed, err := svc.upsertResourceProposal(ctx, "demo-game", "development", sem, nil)
	require.NoError(t, err)
	assert.Empty(t, consumed)
}

// 说明：upsertResourceProposal 的 ShouldBlockProposal 分支
// (contract_service.go L1087-1094) 论证不可达——
// GenerateResourcePageProposal 的诊断 code 集合封闭于
// {resource_semantics_missing, identity_missing, collection_query_missing,
// semantic_conflicts_invalid, semantic_conflict_unresolved,
// resource_identity_column_missing, resource_action_*,
// json_schema_generation_subset_*}，而 ShouldBlockProposal 仅对
// function_id_missing / function_disabled 返回 true，二者不相交。

// L1118/L1124: upsertStandaloneProposals 跳过 nil 契约与空 FunctionID。
func TestUpsertStandaloneProposals_SkipsNilAndEmptyContracts(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewContractService(db)

	err := svc.upsertStandaloneProposals(ctx, "demo-game", "development",
		[]*model.FunctionContract{nil, {GameID: "demo-game", Env: "development", FunctionID: "  "}},
		nil, nil)
	require.NoError(t, err)
}

// L1141-1145: standalone 生成被 error 诊断 block（函数禁用）。
func TestUpsertStandaloneProposals_BlockedByDisabledFunction(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewContractService(db)

	contract := &model.FunctionContract{
		GameID: "demo-game", Env: "development",
		FunctionID: "mail.send", Version: "1.0.0", Enabled: false,
		Capability:  dbenum.CapabilityAction,
		Execution:   "sync",
		ResourceKey: "mail",
		InputSchema: model.JSON(`{"type":"object"}`),
	}
	require.NoError(t, model.NewFunctionContractModel(db).UpsertContract(ctx, contract))

	err := svc.upsertStandaloneProposals(ctx, "demo-game", "development", []*model.FunctionContract{contract}, nil, nil)
	require.NoError(t, err)

	issues, err := model.NewBlockedProposalIssueModel(db).ListByScopeAndResourceKey(ctx, "demo-game", "development", "mail")
	require.NoError(t, err)
	require.NotEmpty(t, issues)
	assert.Equal(t, "mail.send", issues[0].FunctionID)
}

// L1142-1144: standalone block 分支 upsertBlockedIssue 失败。
func TestUpsertStandaloneProposals_BlockedUpsertIssueError(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewContractService(db)

	contract := &model.FunctionContract{
		GameID: "demo-game", Env: "development",
		FunctionID: "mail.send", Version: "1.0.0", Enabled: false,
		Capability:  dbenum.CapabilityAction,
		Execution:   "sync",
		ResourceKey: "mail",
		InputSchema: model.JSON(`{"type":"object"}`),
	}
	require.NoError(t, model.NewFunctionContractModel(db).UpsertContract(ctx, contract))

	remove := injectFailCallback(db, "blocked_proposal_issues")
	defer remove()
	err := svc.upsertStandaloneProposals(ctx, "demo-game", "development", []*model.FunctionContract{contract}, nil, nil)
	require.Error(t, err)
}

// L1183: removeStandaloneProposalsForFunction 删除失败。
func TestRemoveStandaloneProposalsForFunction_DeleteError(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewContractService(db)

	remove := injectFailCallback(db, "page_proposals")
	defer remove()
	err := svc.removeStandaloneProposalsForFunction(ctx, "demo-game", "development", "mail.send")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete standalone proposal")
}

// L1065-1067: RebuildProposalForFunction 中 upsertGeneratedProposal 失败。
func TestRebuildProposalForFunction_UpsertProposalError(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewContractService(db)

	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", FunctionMetaInput{
		ID: "mail.send", Version: "1.0.0", Enabled: true,
		Resource: "mail", Operation: "send",
		Capability: "action", Execution: "sync",
		InputSchema: `{"type":"object","properties":{"to":{"type":"string"}}}`,
	}))

	remove := injectFailCallback(db, "page_proposals")
	defer remove()
	err := svc.RebuildProposalForFunction(ctx, "demo-game", "development", "mail.send")
	require.Error(t, err)
}

// L1477-1479 / L1480-1482: 组合页静态区块 key 缺失 / 重复。
func TestCreateCompositeProposal_StaticSectionKeyErrors(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewContractService(db)

	_, err := svc.CreateCompositeProposal(ctx, "demo-game", "development", "composite--x", []CompositeSectionRequest{
		{Static: true, Form: &spec.FormPresentationSpec{JSONSchema: spec.JSONSchema(`{"type":"object"}`)}},
		{Static: true, Key: "b", Form: &spec.FormPresentationSpec{JSONSchema: spec.JSONSchema(`{"type":"object"}`)}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "static section requires key")

	_, err = svc.CreateCompositeProposal(ctx, "demo-game", "development", "composite--x", []CompositeSectionRequest{
		{Static: true, Key: "a", Form: &spec.FormPresentationSpec{JSONSchema: spec.JSONSchema(`{"type":"object"}`)}},
		{Static: true, Key: "a", Form: &spec.FormPresentationSpec{JSONSchema: spec.JSONSchema(`{"type":"object"}`)}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate section key")
}

// --- proposal_service.go ---------------------------------------------------

// L315-317: AcceptProposal 事务内回写提案状态失败。
func TestAcceptProposal_UpdateProposalStatusError(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)

	page := testProposalPageSpec("resource--player-acc4")
	mustPersistProposal(t, svc, page, "ready")

	remove := injectWriteFailCallback(db, "page_proposals")
	defer remove()
	err := svc.AcceptProposal(ctx, "demo-game", "development", "key-"+page.PageKey)
	require.Error(t, err)
}

// L282-284: AcceptProposal 事务内查找现有 page draft 失败。
func TestAcceptProposal_FindExistingDraftError(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)

	page := testProposalPageSpec("resource--player-acc1")
	mustPersistProposal(t, svc, page, "ready")

	remove := injectFailCallback(db, "page_specs")
	defer remove()
	err := svc.AcceptProposal(ctx, "demo-game", "development", "key-"+page.PageKey)
	require.Error(t, err)
}

// L533-535: createProposalVersionSnapshot 中 json.Marshal(proposal) 失败
// （Diagnostics 为无效 JSON 落库；hasBlockingDiagnostics 解析失败不拦截）。
func TestAcceptProposal_SnapshotMarshalError(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)

	page := testProposalPageSpec("resource--player-acc2")
	pageJSON, err := json.Marshal(page)
	require.NoError(t, err)
	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID:      "demo-game",
		Env:         "development",
		ProposalKey: "key-" + page.PageKey,
		PageKey:     page.PageKey,
		PageType:    string(page.Type),
		Quality:     "ready",
		Status:      dbenum.ProposalStatusPending,
		PageSpec:    pageJSON,
		Diagnostics: model.JSON(`not-json`),
	}))

	err = svc.AcceptProposal(ctx, "demo-game", "development", "key-"+page.PageKey)
	require.Error(t, err)
}

// L328-330: AcceptProposal 写 page version 失败。
func TestAcceptProposal_UpsertVersionError(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)

	page := testProposalPageSpec("resource--player-acc3")
	mustPersistProposal(t, svc, page, "ready")

	remove := injectFailCallback(db, "page_versions")
	defer remove()
	err := svc.AcceptProposal(ctx, "demo-game", "development", "key-"+page.PageKey)
	require.Error(t, err)
}

// 说明：AcceptAndPublishProposal 的 buildBindingContracts 失败分支
// (proposal_service.go L363-365) 论证不可达——
// validateDirectPublishPageSpec 前置运行 CollectBindingSelectorIssues，
// 其对缺失/禁用函数分别报 "bound function contract does not exist" /
// "bound function is disabled"，任何能让 buildBindingContracts 失败的
// 输入都会先被该校验拒绝（422），不会进入事务前的快照构建。

// L388-390: AcceptAndPublishProposal 事务内查找 page draft 失败。
func TestAcceptAndPublishProposal_FindDraftError(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)

	page := setupPublishableProposal(t, db, svc, "operation--mail-pub1")

	remove := injectFailCallback(db, "page_specs")
	defer remove()
	_, err := svc.AcceptAndPublishProposal(ctx, "demo-game", "development", "key-"+page.PageKey)
	require.Error(t, err)
}

// L395-397: AcceptAndPublishProposal GetNextVersion 失败。
func TestAcceptAndPublishProposal_NextVersionError(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)

	page := setupPublishableProposal(t, db, svc, "operation--mail-pub2")

	remove := injectFailCallback(db, "page_versions")
	defer remove()
	_, err := svc.AcceptAndPublishProposal(ctx, "demo-game", "development", "key-"+page.PageKey)
	require.Error(t, err)
}

// L402-404: AcceptAndPublishProposal 提案快照失败。
func TestAcceptAndPublishProposal_SnapshotError(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)

	page := setupPublishableProposal(t, db, svc, "operation--mail-pub3")

	remove := injectFailCallback(db, "page_proposal_versions")
	defer remove()
	_, err := svc.AcceptAndPublishProposal(ctx, "demo-game", "development", "key-"+page.PageKey)
	require.Error(t, err)
}

// L432-434: AcceptAndPublishProposal DeactivatePage 失败。
func TestAcceptAndPublishProposal_DeactivateError(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)

	page := setupPublishableProposal(t, db, svc, "operation--mail-pub4")

	remove := injectWriteFailCallback(db, "published_page_specs")
	defer remove()
	_, err := svc.AcceptAndPublishProposal(ctx, "demo-game", "development", "key-"+page.PageKey)
	require.Error(t, err)
}

// L451-453: AcceptAndPublishProposal 发布快照 Create 失败。
func TestAcceptAndPublishProposal_PublishedCreateError(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)

	page := setupPublishableProposal(t, db, svc, "operation--mail-pub5")

	// DeactivatePage 是 UPDATE；Create 是 INSERT：只对 create 失败。
	name := "test.fail.published.create"
	_ = db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "published_page_specs" {
			_ = tx.AddError(errors.New("injected published create failure"))
		}
	})
	defer func() { _ = db.Callback().Create().Remove(name) }()
	_, err := svc.AcceptAndPublishProposal(ctx, "demo-game", "development", "key-"+page.PageKey)
	require.Error(t, err)
}

// L454-456: AcceptAndPublishProposal page draft upsert 失败。
func TestAcceptAndPublishProposal_DraftUpsertError(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)

	page := setupPublishableProposal(t, db, svc, "operation--mail-pub6")

	// 查询放行，仅对 page_specs 的写入失败。
	name := "test.fail.pages.create"
	_ = db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "page_specs" {
			_ = tx.AddError(errors.New("injected page_specs create failure"))
		}
	})
	defer func() { _ = db.Callback().Create().Remove(name) }()
	_, err := svc.AcceptAndPublishProposal(ctx, "demo-game", "development", "key-"+page.PageKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create page draft from proposal")
}

// L467-469: AcceptAndPublishProposal 写发布版本失败。
func TestAcceptAndPublishProposal_VersionUpsertError(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewProposalService(db)

	page := setupPublishableProposal(t, db, svc, "operation--mail-pub7")

	// GetNextVersion 是 SELECT（放行），仅对 page_versions 的写入失败。
	name := "test.fail.pageversions.create"
	_ = db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "page_versions" {
			_ = tx.AddError(errors.New("injected page_versions create failure"))
		}
	})
	defer func() { _ = db.Callback().Create().Remove(name) }()
	_, err := svc.AcceptAndPublishProposal(ctx, "demo-game", "development", "key-"+page.PageKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create page published version from proposal")
}

// L1272-1274: upsertGeneratedProposal 中 json.Marshal(PageSpec) 失败
// （generated.PageSpec 携带无效 JSONSchema RawMessage）。
func TestUpsertGeneratedProposal_MarshalError(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewContractService(db)

	generated := spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey: "operation--bad",
			Type:    spec.PageTypeOperation,
			Operation: &spec.OperationPageSpec{
				Form: &spec.FormPresentationSpec{JSONSchema: spec.JSONSchema(`{invalid`)},
			},
		},
	}
	err := svc.upsertGeneratedProposal(ctx, "demo-game", "development",
		"operation--bad", nil, nil, generated)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal generated page spec")
}

// L652-654: listContractChanges 中 publishedContractChanges 失败（非缺表错误）。
func TestListContractChanges_PublishedQueryError(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewProposalService(db)

	remove := injectFailCallback(db, "published_page_specs")
	defer remove()
	_, err := svc.listContractChanges(ctx, "demo-game", "development", "")
	require.Error(t, err)
}

// L656-658: listContractChanges 中 draftContractChanges 失败。
func TestListContractChanges_DraftQueryError(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewProposalService(db)

	remove := injectFailCallback(db, "page_specs")
	defer remove()
	_, err := svc.listContractChanges(ctx, "demo-game", "development", "")
	require.Error(t, err)
}

// L664-666: listContractChanges 排序中 Kind 不同、时间相同的分支。
func TestListContractChanges_SortKindTieBreaker(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewProposalService(db)

	now := "2024-01-01 00:00:00+00:00"
	publishedAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	// published stale：绑定函数不存在。
	pub := &model.PublishedPageSpec{
		GameID: "demo-game", Env: "development", PageKey: "resource--pub", Version: 1,
		SpecJSON:    `{"pageKey":"resource--pub","type":"operation","bindings":[{"id":"b1","functionId":"gone.fn","usage":"action","execution":{"mode":"sync"}}]}`,
		Active:      true,
		PublishedAt: publishedAt,
		PublishedBy: "tester",
	}
	require.NoError(t, model.NewPublishedPageSpecModel(db).Create(ctx, pub))

	// draft stale：绑定函数不存在。
	draft := &model.PageSpec{
		GameID: "demo-game", Env: "development", PageKey: "resource--draft",
		Type: "operation", Status: "draft", DraftRevision: 1,
		SpecJSON: `{"pageKey":"resource--draft","type":"operation","bindings":[{"id":"b1","functionId":"gone.fn2","usage":"action","execution":{"mode":"sync"}}]}`,
	}
	require.NoError(t, draft.SetTitle(map[string]string{"zh-CN": "草稿"}))
	require.NoError(t, model.NewPageSpecModel(db).Upsert(ctx, draft))

	// 强制两个 UpdatedAt/PublishedAt 相同。
	require.NoError(t, db.Exec("UPDATE published_page_specs SET published_at = ? WHERE page_key = ?", now, "resource--pub").Error)
	require.NoError(t, db.Exec("UPDATE page_specs SET updated_at = ? WHERE page_key = ?", now, "resource--draft").Error)

	changes, err := svc.listContractChanges(ctx, "demo-game", "development", "")
	require.NoError(t, err)
	require.Len(t, changes, 2)
	// draft < published（字母序），时间相同时按 Kind 排序。
	assert.Equal(t, "draft", changes[0].Kind)
	assert.Equal(t, "published", changes[1].Kind)
}
