package page

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SUI-032 验收：命名展示变体（SUI-031 协议）随草稿与发布快照写入现有表
//（page_specs / published_page_specs 的 SpecJSON blob），草稿回读、旧版本
// 快照回读与「服务器重启后重查」数据一致。表结构零变更：变体位于表单
// 节点内部，随 SpecJSON blob 经既有 typed 序列化链落库；发布前校验、
// 编辑器与渲染解析分别在 SUI-033/034/035 落地。

func banFormWithVariants() *spec.FormPresentationSpec {
	form := spec.DefaultFormPresentation(spec.JSONSchema(`{
		"type":"object",
		"properties":{"playerId":{"type":"string"},"reason":{"type":"string"},"keyword":{"type":"string"}},
		"required":["playerId","reason"]
	}`))
	form.Variants = []spec.FormVariantSpec{
		{
			Key:   "standard",
			Title: spec.LocalizedText{"zh-CN": "标准", "en-US": "Standard"},
			Fields: []spec.FormVariantFieldSpec{
				{Key: "playerId", Width: 12},
			},
		},
		{
			Key:    "compact",
			Title:  spec.LocalizedText{"zh-CN": "紧凑", "en-US": "Compact"},
			Layout: spec.FormLayoutGrid,
			Fields: []spec.FormVariantFieldSpec{
				{Key: "playerId", Width: 6},
				{Key: "reason", Widget: spec.FormWidgetTextArea, Width: 6},
			},
			SubmitButton: &spec.FormButtonSpec{Text: spec.LocalizedText{"zh-CN": "封禁"}, Type: "primary"},
		},
	}
	form.DefaultVariant = "compact"
	return form
}

func saveBanDraftRevision(t *testing.T, service *Service, ctx context.Context, revision int, form *spec.FormPresentationSpec) int {
	t.Helper()
	req := &PageSaveRequest{
		PageKey:       "player.ban",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "封禁玩家", "en-US": "Ban Player"},
		Category:      spec.PageCategorySpec{Key: "player"},
		Operation: &spec.OperationPageSpec{
			Form: form,
			ResultView: &spec.ResultViewSpec{
				SuccessMessage: spec.LocalizedText{"zh-CN": "操作成功"},
				ErrorMessage:   spec.LocalizedText{"zh-CN": "操作失败"},
			},
		},
		Bindings: testPageBindings(),
	}
	resp, err := service.SaveDraft(ctx, req)
	require.NoError(t, err)
	return resp.DraftRevision
}

func parsePageSpecJSON(t *testing.T, raw string) spec.PageSpec {
	t.Helper()
	var pageSpec spec.PageSpec
	require.NoError(t, json.Unmarshal([]byte(raw), &pageSpec))
	return pageSpec
}

func TestFormVariantsDraftAndSnapshotRoundtrip(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read", "pages:publish")
	gameID, env := "demo-game", "development"

	// SUI-033 契约新鲜度：发布时变体字段键会对照 player.action 的当前
	// input schema 复解析——把测试契约对齐到表单 fixture 的字段集
	//（playerId/reason/keyword），保证本测试的差异来源只在存储链。
	rebuildFunctionContract(t, service.svcCtx.DB, ctx, "player.action", reg.FunctionMeta{
		Enabled:      true,
		Version:      "1.0.0",
		Risk:         "safe",
		Permission:   "player:action",
		Resource:     "player",
		Operation:    "action",
		InputSchema:  `{"type":"object","properties":{"playerId":{"type":"string"},"reason":{"type":"string"},"keyword":{"type":"string"}}}`,
		OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
	})

	// v1：无变体的旧形态（SUI-031 之前发布的页面沿用原呈现），先发布留快照。
	legacyForm := spec.DefaultFormPresentation(spec.JSONSchema(`{
		"type":"object",
		"properties":{"playerId":{"type":"string"},"reason":{"type":"string"},"keyword":{"type":"string"}},
		"required":["playerId","reason"]
	}`))
	rev1 := saveBanDraftRevision(t, service, ctx, 0, legacyForm)
	_, err := service.Publish(ctx, &PagePublishRequest{PageKey: "player.ban", DraftRevision: &rev1})
	require.NoError(t, err)

	// v2：同一表单携带命名变体 + 默认项。
	rev2 := saveBanDraftRevision(t, service, ctx, rev1, banFormWithVariants())
	require.Greater(t, rev2, rev1)

	// 草稿回读：变体与默认 key 原样。
	draft, err := service.GetDraft(ctx, &PageDraftRequest{PageKey: "player.ban"})
	require.NoError(t, err)
	require.NotNil(t, draft.Operation)
	require.NotNil(t, draft.Operation.Form)
	require.Len(t, draft.Operation.Form.Variants, 2)
	assert.Equal(t, "compact", draft.Operation.Form.DefaultVariant)
	assert.Equal(t, "standard", draft.Operation.Form.Variants[0].Key)
	assert.Equal(t, spec.FormLayoutGrid, draft.Operation.Form.Variants[1].Layout)
	assert.Equal(t, spec.FormWidgetTextArea, draft.Operation.Form.Variants[1].Fields[1].Widget)
	require.NotNil(t, draft.Operation.Form.Variants[1].SubmitButton)
	assert.Equal(t, spec.LocalizedText{"zh-CN": "封禁"}, draft.Operation.Form.Variants[1].SubmitButton.Text)

	// 发布 v2（DeactivatePage 置 v1 快照 active=false，行保留）。
	_, err = service.Publish(ctx, &PagePublishRequest{PageKey: "player.ban", DraftRevision: &rev2})
	require.NoError(t, err)

	// 发布快照回读（现有表 published_page_specs，零新表）。
	publishedModel := model.NewPublishedPageSpecModel(service.svcCtx.DB)
	v2row, err := publishedModel.FindByScopePageKeyAndVersion(ctx, gameID, env, "player.ban", rev2)
	require.NoError(t, err)
	v2spec := parsePageSpecJSON(t, v2row.SpecJSON)
	require.NotNil(t, v2spec.Operation)
	require.NotNil(t, v2spec.Operation.Form)
	assert.Equal(t, draft.Operation.Form.Variants, v2spec.Operation.Form.Variants)
	assert.Equal(t, "compact", v2spec.Operation.Form.DefaultVariant)

	// 旧版本（v1）快照回读：无 variants/defaultVariant 键，逐字稳定。
	v1row, err := publishedModel.FindByScopePageKeyAndVersion(ctx, gameID, env, "player.ban", rev1)
	require.NoError(t, err)
	v1spec := parsePageSpecJSON(t, v1row.SpecJSON)
	require.NotNil(t, v1spec.Operation)
	require.NotNil(t, v1spec.Operation.Form)
	assert.Empty(t, v1spec.Operation.Form.Variants)
	assert.Empty(t, v1spec.Operation.Form.DefaultVariant)
	assert.NotContains(t, v1row.SpecJSON, `"variants"`)
	assert.NotContains(t, v1row.SpecJSON, `"defaultVariant"`)

	// 服务器重启模拟：全新模型实例（模型无状态，状态在 DB）重新反序列
	// 化草稿行，与发布快照内容一致。
	freshPageModel := model.NewPageSpecModel(service.svcCtx.DB)
	freshDraft, err := freshPageModel.FindByScopeAndPageKey(ctx, gameID, env, "player.ban")
	require.NoError(t, err)
	restartSpec := parsePageSpecJSON(t, freshDraft.SpecJSON)
	require.NotNil(t, restartSpec.Operation)
	require.NotNil(t, restartSpec.Operation.Form)
	assert.Equal(t, v2spec.Operation.Form.Variants, restartSpec.Operation.Form.Variants)
	assert.Equal(t, v2spec.Operation.Form.DefaultVariant, restartSpec.Operation.Form.DefaultVariant)
}
