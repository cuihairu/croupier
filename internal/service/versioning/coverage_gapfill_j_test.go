package versioning

// 覆盖率补洞（gap-fill）：只针对 service.go 中尚未覆盖的可达分支。
// 不修改产品代码与既有测试。

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// injectVersioningFail 让指定表的 gorm 语句失败（Query/Row 链都要拦：
// gorm 的 Scan 走 Row 链而非 Query 链）。
func injectVersioningFail(db *gorm.DB, table string) (remove func()) {
	removeQ := injectVersioningFailQuery(db, table)
	removeW := injectVersioningFailWrite(db, table)
	return func() {
		removeQ()
		removeW()
	}
}

func injectVersioningFailQuery(db *gorm.DB, table string) (remove func()) {
	name := "zz.failq." + table
	fail := func(tx *gorm.DB) {
		if versioningMatchTable(tx, table) {
			_ = tx.AddError(errors.New("injected failure for " + table))
		}
	}
	_ = db.Callback().Query().Before("gorm:query").Register(name, fail)
	_ = db.Callback().Row().Before("gorm:row").Register(name, fail)
	return func() {
		_ = db.Callback().Query().Remove(name)
		_ = db.Callback().Row().Remove(name)
	}
}

// 只拦 Row 链（gorm Scan）：Find（Query 链）放行。
func injectVersioningFailRowOnly(db *gorm.DB, table string) (remove func()) {
	name := "zz.failrow." + table
	fail := func(tx *gorm.DB) {
		if versioningMatchTable(tx, table) {
			_ = tx.AddError(errors.New("injected row failure for " + table))
		}
	}
	_ = db.Callback().Row().Before("gorm:row").Register(name, fail)
	return func() {
		_ = db.Callback().Row().Remove(name)
	}
}

func injectVersioningFailWrite(db *gorm.DB, table string) (remove func()) {
	name := "zz.failw." + table
	fail := func(tx *gorm.DB) {
		if versioningMatchTable(tx, table) {
			_ = tx.AddError(errors.New("injected write failure for " + table))
		}
	}
	_ = db.Callback().Create().Before("gorm:create").Register(name, fail)
	_ = db.Callback().Update().Before("gorm:update").Register(name, fail)
	return func() {
		_ = db.Callback().Create().Remove(name)
		_ = db.Callback().Update().Remove(name)
	}
}

func versioningMatchTable(tx *gorm.DB, table string) bool {
	if tx.Statement == nil {
		return false
	}
	if tx.Statement.Table == "" && tx.Statement.Model != nil {
		_ = tx.Statement.Parse(tx.Statement.Model)
	}
	return tx.Statement.Table == table
}

// 不可达论证（L130/L670/L731，contractsForPage 错误传播）：
// contractsForPage 对 FindByScopeAndFunctionID 的任意错误 continue 容错
// （service.go L626-628），唯一 return 路径的 error 恒为 nil，因此
// GetChangeChain / functionSpecsByID / regenerateStandaloneProposal 中
// 对其错误的包装分支不可达。L700（proposalKey 为空）同样不可达：
// mainContract 经非空 functionID 查得，proposalKeyForPage 对非空
// functionID 恒返回非空 key。

// L739: upsertGeneratedProposal 中 json.Marshal(PageSpec) 失败
// （generated.PageSpec 携带无效 JSONSchema RawMessage）。
func TestUpsertGeneratedProposal_MarshalError(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db)

	generated := spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey: "operation--bad",
			Type:    spec.PageTypeOperation,
			Operation: &spec.OperationPageSpec{
				Form: &spec.FormPresentationSpec{JSONSchema: spec.JSONSchema(`{invalid`)},
			},
		},
	}
	err := svc.upsertGeneratedProposal(ctx, "demo-game", "development", "operation--bad", nil, generated)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal generated page spec")
}

// L784: createProposalVersionSnapshot 中 json.Marshal(proposal) 失败
// （已落库提案的 PageSpec 为无效 JSON RawMessage）。
func TestCreateProposalVersionSnapshot_MarshalError(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	proposal := &model.PageProposal{
		GameID: "demo-game", Env: "development",
		ProposalKey: "operation:bad", PageKey: "operation--bad",
		PageType: "operation", Quality: "basic",
		Status:   dbenum.ProposalStatusPending,
		PageSpec: model.JSON(`not-json`),
	}
	require.NoError(t, model.NewPageProposalModel(db).UpsertProposal(ctx, proposal))

	_, err := createProposalVersionSnapshot(ctx, model.NewPageProposalVersionModel(db), proposal, "boom", "tester")
	require.Error(t, err)
}

// L795: createProposalVersionSnapshot 中 CreateVersion 失败。
func TestCreateProposalVersionSnapshot_CreateError(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	proposal := &model.PageProposal{
		GameID: "demo-game", Env: "development",
		ProposalKey: "operation:err", PageKey: "operation--err",
		PageType: "operation", Quality: "basic",
		Status:   dbenum.ProposalStatusPending,
		PageSpec: jsonValue(spec.PageSpec{PageKey: "operation--err"}),
	}
	require.NoError(t, model.NewPageProposalModel(db).UpsertProposal(ctx, proposal))

	remove := injectVersioningFailWrite(db, "page_proposal_versions")
	defer remove()
	_, err := createProposalVersionSnapshot(ctx, model.NewPageProposalVersionModel(db), proposal, "boom", "tester")
	require.Error(t, err)
}

// L1581: RollbackDraft 事务内 GetNextVersion（page_versions）失败。
func TestRollbackDraft_NextVersionError(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db)

	page := spec.PageSpec{
		PageKey:     "resource--player",
		Type:        spec.PageTypeResource,
		ResourceKey: "player",
		Title:       spec.LocalizedText{"zh-CN": "玩家"},
		Category:    spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
		Bindings: []spec.PageFunctionBinding{{
			ID: "query", FunctionID: "player.list", Usage: spec.BindingUsageQuery,
			Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		}},
	}
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))
	require.NoError(t, createVersioningTestPageVersion(db, "demo-game", "development", page, 3, "v3"))

	// findPageVersion 走 Find（Query 链）放行；事务内 GetNextVersion 走
	// Scan（Row 链）被拦。
	remove := injectVersioningFailRowOnly(db, "page_versions")
	defer remove()
	_, err := svc.RollbackDraft(ctx, &RollbackRequest{
		GameID: "demo-game", Env: "development", PageKey: "resource--player",
		Version: 3, ExpectedDraftRevision: 1,
	})
	require.Error(t, err)
}
