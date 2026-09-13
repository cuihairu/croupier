package page

// 覆盖目标（组 B）：
//  1. handler.SyncSelectors 的 URI/JSON 绑定错误、*PageNotFoundError 404、
//     成功 200 四条路径。
//  2. service.SyncSelectors 的 currentUsername 失败、SpecJSON 解码失败、
//     FunctionVersionStale 透传 Manual、applyPageSpecToModel 失败（空
//     category.key）、buildPageSpecJSONFn 失败、事务内 Find 失败/revision
//     冲突/Upsert 失败（sqlite 触发器 RAISE）。
//  3. publishedContractsForSync 的 FindLatest 失败（DROP TABLE）。
//  4. countManualEntries 的 Input/Output manual 计数。

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	dashboardservice "github.com/cuihairu/croupier/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// handler.SyncSelectors
// ---------------------------------------------------------------------------

// newV11SyncContext 构造带 request context / body 的 gin 测试上下文。
func newV11SyncContext(goCtx context.Context, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pages/x/sync-selectors", reader)
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req.WithContext(goCtx)
	return ctx, rec
}

// URI 绑定错误（缺少 pageKey 参数）→ 400。
func TestV11Handler_SyncSelectors_BindURIError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	handler := NewHandler(service)

	ginCtx, rec := newV11SyncContext(ctx, `{}`)
	ginCtx.Params = nil
	handler.SyncSelectors(ginCtx)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// JSON 绑定错误（非法 JSON body）→ 400。
func TestV11Handler_SyncSelectors_BindJSONError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	handler := NewHandler(service)

	ginCtx, rec := newV11SyncContext(ctx, `{not-json`)
	ginCtx.Params = gin.Params{{Key: "pageKey", Value: "player.manage"}}
	handler.SyncSelectors(ginCtx)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// 页面不存在 → 404（service 返回 *PageNotFoundError）。
func TestV11Handler_SyncSelectors_NotFound(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	handler := NewHandler(service)

	ginCtx, rec := newV11SyncContext(ctx, `{"draftRevision":1,"dryRun":true}`)
	ginCtx.Params = gin.Params{{Key: "pageKey", Value: "ghost-page"}}
	handler.SyncSelectors(ginCtx)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "ghost-page")
}

// 正常 dry-run → 200（response.Success 路径）。
func TestV11Handler_SyncSelectors_Success(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)
	handler := NewHandler(service)

	ginCtx, rec := newV11SyncContext(ctx,
		`{"draftRevision":`+strconv.Itoa(revision)+`,"dryRun":true}`)
	ginCtx.Params = gin.Params{{Key: "pageKey", Value: "player.manage"}}
	handler.SyncSelectors(ginCtx)

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	var resp PageSyncSelectorsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "player.manage", resp.PageKey)
	assert.True(t, resp.DryRun)
	assert.False(t, resp.Applied)
}

// ---------------------------------------------------------------------------
// service.SyncSelectors 错误分支
// ---------------------------------------------------------------------------

// currentUsername 失败（缝隙注入，requirePageEdit 走真实链路先行通过）。
func TestV11SyncSelectors_CurrentUsernameError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)

	injected := errors.New("injected username failure v11")
	withCurrentUsernameError(t, injected)

	resp, err := service.SyncSelectors(ctx, &PageSyncSelectorsRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
		DryRun:        true,
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, injected), "SyncSelectors() error = %v, want injected", err)
}

// SpecJSON 损坏 → pageSpecFromModel 解码失败。
func TestV11SyncSelectors_SpecDecodeError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)

	require.NoError(t, service.svcCtx.DB.Model(&model.PageSpec{}).
		Where("game_id = ? AND env = ? AND page_key = ?", "demo-game", "development", "player.manage").
		Update("spec_json", "{broken").Error)

	resp, err := service.SyncSelectors(ctx, &PageSyncSelectorsRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
		DryRun:        true,
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "decode canonical PageSpec")
}

// apply 路径：synced spec 的 category.key 为空 → applyPageSpecToModel 拒绝。
func TestV11SyncSelectors_ApplyCategoryKeyMissing(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)

	var raw string
	require.NoError(t, service.svcCtx.DB.Model(&model.PageSpec{}).
		Select("spec_json").
		Where("game_id = ? AND env = ? AND page_key = ?", "demo-game", "development", "player.manage").
		Scan(&raw).Error)
	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &doc))
	category, _ := doc["category"].(map[string]any)
	require.NotNil(t, category, "draft spec must contain category")
	category["key"] = ""
	broken, err := json.Marshal(doc)
	require.NoError(t, err)
	require.NoError(t, service.svcCtx.DB.Model(&model.PageSpec{}).
		Where("game_id = ? AND env = ? AND page_key = ?", "demo-game", "development", "player.manage").
		Update("spec_json", string(broken)).Error)

	resp, err := service.SyncSelectors(ctx, &PageSyncSelectorsRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "category.key is required")
}

// apply 路径：buildPageSpecJSONFn 失败（缝隙注入）。
func TestV11SyncSelectors_ApplyBuildSpecJSONError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)

	injected := errors.New("injected buildSpecJSON failure v11")
	orig := buildPageSpecJSONFn
	buildPageSpecJSONFn = func(*model.PageSpec) (string, error) { return "", injected }
	t.Cleanup(func() { buildPageSpecJSONFn = orig })

	resp, err := service.SyncSelectors(ctx, &PageSyncSelectorsRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, injected), "SyncSelectors() error = %v, want injected", err)
}

// 事务内 Find 失败：gorm 查询回调仅在事务内对 page_specs 注入错误
// （findDraft 的事务外 SELECT 不受影响）。
func TestV11SyncSelectors_TxFetchError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)

	injected := errors.New("injected in-tx page_specs fetch failure")
	withV11InTxPageSpecsHook(t, service.svcCtx.DB, func(tx *gorm.DB) {
		_ = tx.AddError(injected)
	})

	resp, err := service.SyncSelectors(ctx, &PageSyncSelectorsRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, injected), "SyncSelectors() error = %v, want injected", err)
}

// 事务内 revision 重查冲突：回调在事务内 SELECT 前先 bump draft_revision，
// 模拟 planner 运行期间的并发保存 → 409 乐观锁。
func TestV11SyncSelectors_TxRevisionConflict(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)

	withV11InTxPageSpecsHook(t, service.svcCtx.DB, func(tx *gorm.DB) {
		if _, err := tx.Statement.ConnPool.ExecContext(
			tx.Statement.Context,
			"UPDATE page_specs SET draft_revision = draft_revision + 1",
		); err != nil {
			_ = tx.AddError(err)
		}
	})

	resp, err := service.SyncSelectors(ctx, &PageSyncSelectorsRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, syncSelectorsConflict(err), "expected 409, got %v", err)
}

// 事务内 Upsert 失败：sqlite 触发器 RAISE 阻断 UPDATE。
func TestV11SyncSelectors_TxUpsertError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)

	require.NoError(t, service.svcCtx.DB.Exec(
		"CREATE TRIGGER v11_block_page_specs_update BEFORE UPDATE ON page_specs "+
			"BEGIN SELECT RAISE(ABORT, 'injected upsert failure v11'); END").Error)

	resp, err := service.SyncSelectors(ctx, &PageSyncSelectorsRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "injected upsert failure v11")
}

// 发布快照读取失败（DROP TABLE）→ publishedContractsForSync 静默回落空 map，
// dry-run 仍成功。
func TestV11SyncSelectors_PublishedContractsLookupFailure(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)

	require.NoError(t, service.svcCtx.DB.Model(&model.PageSpec{}).
		Where("game_id = ? AND env = ? AND page_key = ?", "demo-game", "development", "player.manage").
		Update("published_version", 3).Error)
	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("published_page_specs"))

	resp, err := service.SyncSelectors(ctx, &PageSyncSelectorsRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
		DryRun:        true,
	})
	require.NoError(t, err, "快照表读取失败必须静默回落，不升级成 5xx")
	require.NotNil(t, resp)
	assert.True(t, resp.DryRun)
}

// ---------------------------------------------------------------------------
// 发布链：契约仅 bump version（schema 不变）→ dry-run 报告将
// binding_function_version_stale 以 Manual 透传（不可由 selector 同步修复）。
// ---------------------------------------------------------------------------

func v11GiftMeta(version string) reg.FunctionMeta {
	return reg.FunctionMeta{
		Enabled:      true,
		Version:      version,
		Risk:         "safe",
		InputSchema:  `{"type":"object","properties":{"uid":{"type":"string"},"count":{"type":"integer"}},"required":["uid","count"]}`,
		OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
	}
}

func TestV11SyncSelectors_FunctionVersionStaleManual(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish")
	db := service.svcCtx.DB
	const functionID = "gift.send"
	const proposalKey = "operation:gift.send"

	rebuildFunctionContract(t, db, ctx, functionID, v11GiftMeta("1.0.0"))
	require.NoError(t, dashboardservice.NewContractService(db).RebuildProposalForFunction(ctx, "demo-game", "development", functionID))
	published, err := dashboardservice.NewProposalService(db).AcceptAndPublishProposal(ctx, "demo-game", "development", proposalKey)
	require.NoError(t, err)
	pageKey := published.PageKey
	require.NotEmpty(t, pageKey)

	// 仅 bump version：schema 保持一致，唯一漂移是 function version。
	rebuildFunctionContract(t, db, ctx, functionID, v11GiftMeta("2.0.0"))

	revision := published.DraftRevision
	resp, err := service.SyncSelectors(ctx, &PageSyncSelectorsRequest{
		PageKey:       pageKey,
		DraftRevision: &revision,
		DryRun:        true,
	})
	require.NoError(t, err)

	found := false
	for _, report := range resp.SyncedBindings {
		for _, diag := range report.Manual {
			if diag.Code == "binding_function_version_stale" {
				found = true
				assert.Equal(t, "bindings."+report.BindingID, diag.Field,
					"version stale 诊断必须指回具体 binding")
			}
		}
	}
	assert.True(t, found, "expected binding_function_version_stale in Manual, got %+v", resp.SyncedBindings)
}

// ---------------------------------------------------------------------------
// countManualEntries
// ---------------------------------------------------------------------------

func TestV11CountManualEntries(t *testing.T) {
	reports := []spec.BindingSelectorSyncReport{
		{
			BindingID: "b1",
			Input: []spec.SelectorSyncInputEntry{
				{Target: "/uid", Action: spec.SelectorSyncManual},
				{Target: "/count", Action: spec.SelectorSyncRenamed},
			},
			Output: []spec.SelectorSyncOutputEntry{
				{StateKey: "sel", Action: spec.SelectorSyncManual},
			},
			Manual: []spec.Diagnostic{{Code: "binding_function_missing"}},
		},
		{
			BindingID: "b2",
			Input:     []spec.SelectorSyncInputEntry{{Target: "/x", Action: spec.SelectorSyncRenamed}},
		},
		{BindingID: "b3"},
	}

	// b1: input 1 + output 1 + manual 1 = 3；b2/b3 无 manual。
	assert.Equal(t, 3, countManualEntries(reports))
	assert.Equal(t, 0, countManualEntries(nil))
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// withV11InTxPageSpecsHook 注册仅作用于事务内 page_specs 查询的前置回调
// （ConnPool 为 TxCommitter 即在事务内；ExecContext 复用事务连接，
// 避免 MaxOpenConns(1) 下另开连接死锁）。
func withV11InTxPageSpecsHook(t *testing.T, db *gorm.DB, fn func(tx *gorm.DB)) {
	t.Helper()
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("pagecov_v11:in_tx_hook", func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Schema == nil || tx.Statement.Schema.Table != "page_specs" {
			return
		}
		if _, inTx := tx.Statement.ConnPool.(gorm.TxCommitter); !inTx {
			return
		}
		fn(tx)
	}))
}
