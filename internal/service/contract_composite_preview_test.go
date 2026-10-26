package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

// previewSections 组合页预览用例的统一输入：详情 + 表格两区块，表格依赖详情
// 的 page_state 联动（与生产提案用例同形态）。
func previewSections() []CompositeSectionRequest {
	return []CompositeSectionRequest{
		{FunctionID: "player.get", View: "fields", Title: "玩家信息"},
		{FunctionID: "order.list", View: "table", Title: "订单", RefreshOn: []string{"player.get"}},
	}
}

// TestBuildCompositePageSpec_NoWrite：预览生成链只读——不产生提案行、
// 不产生提案版本快照，与「保存→发布」路径的写入副作用完全隔离。
func TestBuildCompositePageSpec_NoWrite(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()
	svc := NewContractService(db)
	seedCompositeV9(t, svc, ctx)

	require.NoError(t, db.AutoMigrate(&model.PageProposal{}, &model.PageProposalVersion{}))

	count := func(table string) int64 {
		var n int64
		require.NoError(t, db.Table(table).Count(&n).Error)
		return n
	}
	proposalsBefore, versionsBefore := count("page_proposals"), count("page_proposal_versions")

	generated, contracts, err := svc.BuildCompositePageSpec(ctx, "g9", "e9", "preview--no-write", previewSections(), nil)
	require.NoError(t, err)
	require.Len(t, contracts, 2, "预览路径返回参与生成的契约，供持久化段复用")

	assert.Equal(t, spec.PageTypeComposite, generated.Type)
	require.NotNil(t, generated.Composite, "预览产物必须是可渲染的 composite 区块集")
	require.Len(t, generated.Composite.Sections, 2)
	assert.NotEmpty(t, generated.PageSpec.Bindings, "预览产物必须带绑定分配，客户端无法自行拼装")
	assert.NotEmpty(t, generated.Quality)

	assert.Equal(t, proposalsBefore, count("page_proposals"), "预览不得写入提案")
	assert.Equal(t, versionsBefore, count("page_proposal_versions"), "预览不得写入提案版本快照")
}

// TestBuildCompositePageSpec_SameAsPersistedProposal：预览产物与保存提案的
// PageSpec 逐字段一致——编辑器预览消费的 spec 与发布页来自同一份生成链，
// 不出现「预览一套结构、发布另一套结构」的漂移。
func TestBuildCompositePageSpec_SameAsPersistedProposal(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()
	svc := NewContractService(db)
	seedCompositeV9(t, svc, ctx)

	const pageKey = "preview--equivalence"
	generated, _, err := svc.BuildCompositePageSpec(ctx, "g9", "e9", pageKey, previewSections(), nil)
	require.NoError(t, err)

	proposal, err := svc.CreateCompositeProposal(ctx, "g9", "e9", pageKey, previewSections(), nil)
	require.NoError(t, err)
	require.NotNil(t, proposal)

	require.JSONEq(t, string(mustJSON(t, generated.PageSpec)), string(proposal.PageSpec))
	assert.Equal(t, generated.Quality, spec.GeneratedPageQuality(proposal.Quality))
	assert.Len(t, generated.Diagnostics, 0)
}

// TestBuildCompositePageSpec_Errors：预览路径继承生成链的全部输入校验，
// 不另设一套宽松规则。
func TestBuildCompositePageSpec_Errors(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()
	svc := NewContractService(db)
	seedCompositeV9(t, svc, ctx)

	_, _, err := svc.BuildCompositePageSpec(ctx, "g9", "e9", "  ", previewSections(), nil)
	require.ErrorContains(t, err, "pageKey is required")

	_, _, err = svc.BuildCompositePageSpec(ctx, "g9", "e9", "preview--missing-fn", []CompositeSectionRequest{
		{FunctionID: "player.get"}, {FunctionID: "no.such.fn"},
	}, nil)
	require.ErrorContains(t, err, "contract not found")

	_, _, err = svc.BuildCompositePageSpec(ctx, "g9", "e9", "preview--dup-key", []CompositeSectionRequest{
		{Key: "same", FunctionID: "player.get"}, {Key: "same", FunctionID: "order.list"},
	}, nil)
	require.ErrorContains(t, err, "duplicate section key")

	_, _, err = svc.BuildCompositePageSpec(ctx, "g9", "e9", "preview--static", []CompositeSectionRequest{
		{Static: true, Key: "s"}, // 缺 form.jsonSchema
	}, nil)
	require.ErrorContains(t, err, "requires form.jsonSchema")
}

// TestBuildCompositePageSpec_StaticAndTemplateSnapshot：预览路径与提案路径
// 共用 static 区块交错与模板快照透传——常量表单不经生成器，模板快照只作来源
// 标记，两者在预览产物中保持同一位置与同一内容。
func TestBuildCompositePageSpec_StaticAndTemplateSnapshot(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()
	svc := NewContractService(db)
	seedCompositeV9(t, svc, ctx)

	sections := []CompositeSectionRequest{
		{Static: true, Key: "filter", Form: &spec.FormPresentationSpec{JSONSchema: spec.JSONSchema(
			`{"type":"object","properties":{"keyword":{"type":"string"}}}`)}},
		{Key: "orders", FunctionID: "order.list", View: "table", Title: "订单"},
	}
	templates := []spec.ComponentTemplateUsage{{Key: "tpl--table", Digest: "d1"}}

	generated, _, err := svc.BuildCompositePageSpec(ctx, "g9", "e9", "preview--static", sections, templates)
	require.NoError(t, err)
	require.Len(t, generated.Composite.Sections, 2)
	assert.Equal(t, "filter", generated.Composite.Sections[0].Key, "static 区块保持输入流原位置")
	assert.True(t, generated.Composite.Sections[0].Static)
	assert.Equal(t, "orders", generated.Composite.Sections[1].Key)
	require.Len(t, generated.ComponentTemplates, 1)
	assert.Equal(t, "tpl--table", generated.ComponentTemplates[0].Key)
}

func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}
