package page

import (
	"context"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SUI-033 验收：发布前验证变体与当前契约——
//  1. 变体引用的契约字段被删除/改名后，发布与版本化回滚预检必须以
//     form_variant_contract_stale 定位到受影响变体（草稿 schema 冻结在
//     旧形态，只有对照当前契约才能发现漂移）；
//  2. 保存/预览路径（publish=false）不跑契约新鲜度——草稿允许滞后，
//     重新生成才是变体同步的正规路径（stale-draft 模型边界）。
//
// 变体 key 冲突 / defaultVariant 不可解析 / 非法覆盖键的定位错误在
// spec 包单测覆盖（form_validation_variants_test.go），此处走服务级
// 全链路（SaveDraft → Publish → 回滚预检）。

const banContractSchemaWithKeyword = `{"type":"object","properties":{"playerId":{"type":"string"},"reason":{"type":"string"},"keyword":{"type":"string"}},"required":["playerId","reason"]}`

const banContractSchemaWithoutKeyword = `{"type":"object","properties":{"playerId":{"type":"string"},"reason":{"type":"string"}},"required":["playerId","reason"]}`

// staleVariantBanForm 的 compact 变体引用 keyword——契约删除该字段后
// 该变体即过期（受影响变体名 = compact）。
func staleVariantBanForm() *spec.FormPresentationSpec {
	form := spec.DefaultFormPresentation(spec.JSONSchema(banContractSchemaWithKeyword))
	form.Fields = []spec.FormFieldSpec{
		{Key: "playerId", Widget: spec.FormWidgetInput},
		{Key: "reason", Widget: spec.FormWidgetTextArea},
	}
	form.Variants = []spec.FormVariantSpec{
		{Key: "standard", Title: spec.LocalizedText{"zh-CN": "标准"}},
		{
			Key:    "compact",
			Title:  spec.LocalizedText{"zh-CN": "紧凑"},
			Layout: spec.FormLayoutGrid,
			Fields: []spec.FormVariantFieldSpec{
				{Key: "playerId", Width: 6},
				{Key: "reason", Widget: spec.FormWidgetTextArea, Width: 6},
				{Key: "keyword", Widget: spec.FormWidgetInput, Width: 12},
			},
		},
	}
	form.DefaultVariant = "compact"
	return form
}

// staleVariantBanBindings 的 selector 覆盖契约全部 required 字段
// （playerId/reason，改名前后都在契约里），保证发布/回滚预检失败时唯一
// 差异来源是变体契约新鲜度而非 selector。
func staleVariantBanBindings() []spec.PageFunctionBinding {
	return []spec.PageFunctionBinding{
		{
			ID:         "player.action",
			FunctionID: "player.action",
			Usage:      spec.BindingUsageAction,
			Selectors: &spec.BindingSelectors{
				Input: spec.SelectorAST{Assignments: []spec.InputAssignment{
					{
						Target: "/playerId",
						Source: spec.ValueSource{Kind: spec.SourceForm, Path: "/playerId"},
					},
					{
						Target: "/reason",
						Source: spec.ValueSource{Kind: spec.SourceForm, Path: "/reason"},
					},
				}},
			},
			Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
		},
	}
}

func rebuildBanContract(t *testing.T, service *Service, ctx context.Context, version string, inputSchema string) {
	t.Helper()
	rebuildFunctionContract(t, service.svcCtx.DB, ctx, "player.action", reg.FunctionMeta{
		Enabled:      true,
		Version:      version,
		Risk:         "safe",
		Permission:   "player:action",
		Resource:     "player",
		Operation:    "action",
		InputSchema:  inputSchema,
		OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
	})
}

func saveStaleVariantDraft(t *testing.T, service *Service, ctx context.Context, revision int) int {
	t.Helper()
	req := &PageSaveRequest{
		PageKey:       "player.ban",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "封禁玩家", "en-US": "Ban Player"},
		Category:      spec.PageCategorySpec{Key: "player"},
		Operation: &spec.OperationPageSpec{
			Form: staleVariantBanForm(),
			ResultView: &spec.ResultViewSpec{
				SuccessMessage: spec.LocalizedText{"zh-CN": "操作成功"},
				ErrorMessage:   spec.LocalizedText{"zh-CN": "操作失败"},
			},
		},
		Bindings: staleVariantBanBindings(),
	}
	resp, err := service.SaveDraft(ctx, req)
	require.NoError(t, err)
	return resp.DraftRevision
}

func TestPublishRejectsStaleVariantAgainstCurrentContract(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read", "pages:publish")
	gameID, env := "demo-game", "development"

	// 契约仍含 keyword：保存 + 发布全绿。
	rebuildBanContract(t, service, ctx, "1.0.0", banContractSchemaWithKeyword)
	rev := saveStaleVariantDraft(t, service, ctx, 0)
	_, err := service.Publish(ctx, &PagePublishRequest{PageKey: "player.ban", DraftRevision: &rev})
	require.NoError(t, err)

	// 契约删除 keyword（改名场景同形态）：草稿 schema 冻结旧形态，
	// 保存/预览不报（stale-draft 模型），发布必须定位受影响变体。
	rebuildBanContract(t, service, ctx, "1.1.0", banContractSchemaWithoutKeyword)

	draft, err := service.GetDraft(ctx, &PageDraftRequest{PageKey: "player.ban"})
	require.NoError(t, err)
	saveDiags := service.ValidatePageSpecInScope(ctx, gameID, env, draft.PageSpec, false)
	for _, diag := range saveDiags {
		assert.NotEqual(t, "form_variant_contract_stale", diag.Code,
			"save/preview must not enforce contract freshness")
	}

	publishDiags := service.ValidatePageSpecInScope(ctx, gameID, env, draft.PageSpec, true)
	found := false
	for _, diag := range publishDiags {
		if diag.Code == "form_variant_contract_stale" {
			found = true
			assert.Equal(t, "operation.form.variants[1].fields[2].key", diag.Field)
			assert.Contains(t, diag.Message, `variant "compact"`)
			assert.Contains(t, diag.Message, "keyword")
			assert.Equal(t, "player.action", diag.FunctionID)
		}
	}
	require.True(t, found, "publish must expose the affected variant, got %#v", publishDiags)

	_, err = service.Publish(ctx, &PagePublishRequest{PageKey: "player.ban", DraftRevision: &rev})
	require.Error(t, err)
	// details map 按 field→message 组织：定位路径 + 受影响变体名都在。
	assert.Contains(t, err.Error(), "operation.form.variants[1].fields[2].key")
	assert.Contains(t, err.Error(), `variant "compact"`)
}

func TestVersioningRollbackPreflightRejectsStaleVariant(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read", "pages:publish")
	gameID, env := "demo-game", "development"

	// 发布带变体的 v1（契约含 keyword，回滚目标快照合法）。
	rebuildBanContract(t, service, ctx, "1.0.0", banContractSchemaWithKeyword)
	rev := saveStaleVariantDraft(t, service, ctx, 0)
	_, err := service.Publish(ctx, &PagePublishRequest{PageKey: "player.ban", DraftRevision: &rev})
	require.NoError(t, err)

	// 回滚预检在契约未变时通过。
	require.NoError(t, service.ValidateVersioningRollbackTarget(ctx, gameID, env, "player.ban", rev))

	// 契约删除 keyword 后：快照里的变体指向已删除字段，回滚预检必须
	// 拒绝并暴露受影响变体——否则回滚会复活过期呈现。
	rebuildBanContract(t, service, ctx, "1.1.0", banContractSchemaWithoutKeyword)
	err = service.ValidateVersioningRollbackTarget(ctx, gameID, env, "player.ban", rev)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "operation.form.variants[1].fields[2].key")
	assert.True(t, strings.Contains(err.Error(), `variant "compact"`),
		"rollback preflight must name the affected variant, got %v", err)

	// 契约恢复 keyword（字段改名回原形）后同一快照重新可回滚。
	rebuildBanContract(t, service, ctx, "1.2.0", banContractSchemaWithKeyword)
	require.NoError(t, service.ValidateVersioningRollbackTarget(ctx, gameID, env, "player.ban", rev))
}
