package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func jsonUnmarshalV9(raw []byte, v interface{}) error {
	return json.Unmarshal(raw, v)
}

// seedCompositeV9 注册 player.get / order.list 两个契约，供组合页用例复用。
func seedCompositeV9(t *testing.T, svc *ContractService, ctx context.Context) {
	t.Helper()
	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g9", "e9", "agent-1", spec.FunctionContractInput{
		ID: "player.get", Resource: "player", Capability: "item_query", Execution: "sync", Enabled: true,
		InputSchema:  `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`,
		OutputSchema: `{"type":"object","properties":{"player":{"type":"object"},"gold":{"type":"integer"}}}`,
	}))
	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g9", "e9", "agent-1", spec.FunctionContractInput{
		ID: "order.list", Resource: "order", Capability: "collection_query", Execution: "sync", Enabled: true,
		InputSchema:  `{"type":"object","properties":{"playerId":{"type":"string"}},"required":["playerId"]}`,
		OutputSchema: `{"type":"object","properties":{"items":{"type":"array"},"total":{"type":"integer"}}}`,
	}))
}

// CreateCompositeProposal 校验失败分支：空 pageKey、区块数不足、重复 key、契约缺失、
// 全空 FunctionID（无法生成）、以及提案写入失败。
func TestCreateCompositeProposalValidationBranchesV9(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()
	svc := NewContractService(db)
	seedCompositeV9(t, svc, ctx)

	_, err := svc.CreateCompositeProposal(ctx, "g9", "e9", "  ", []CompositeSectionRequest{
		{FunctionID: "player.get"}, {FunctionID: "order.list"},
	})
	assert.ErrorContains(t, err, "pageKey and 2+ sections are required")

	_, err = svc.CreateCompositeProposal(ctx, "g9", "e9", "composite--one", []CompositeSectionRequest{
		{FunctionID: "player.get"},
	})
	assert.ErrorContains(t, err, "pageKey and 2+ sections are required")

	// 空 FunctionID 区块被跳过：仍剩两个有效区块，可正常生成。
	proposal, err := svc.CreateCompositeProposal(ctx, "g9", "e9", "composite--skip-empty", []CompositeSectionRequest{
		{FunctionID: "   "},
		{FunctionID: "player.get", View: "fields", Title: "玩家信息"},
		{FunctionID: "order.list", View: "table", Title: "订单"},
	})
	require.NoError(t, err)
	require.NotNil(t, proposal)
	assert.NotEmpty(t, proposal.ProposalKey)

	// 重复区块 key。
	_, err = svc.CreateCompositeProposal(ctx, "g9", "e9", "composite--dup", []CompositeSectionRequest{
		{Key: "k", FunctionID: "player.get"},
		{Key: "k", FunctionID: "order.list"},
	})
	assert.ErrorContains(t, err, "duplicate section key")

	// 契约不存在。
	_, err = svc.CreateCompositeProposal(ctx, "g9", "e9", "composite--ghost", []CompositeSectionRequest{
		{FunctionID: "player.get"},
		{FunctionID: "ghost.fn"},
	})
	assert.ErrorContains(t, err, "ghost.fn contract not found")

	// 全空 FunctionID → 无有效区块 → 无法生成。
	_, err = svc.CreateCompositeProposal(ctx, "g9", "e9", "composite--empty", []CompositeSectionRequest{
		{FunctionID: " "}, {FunctionID: "  "},
	})
	assert.ErrorContains(t, err, "composite page cannot be generated")
}

// CreateCompositeProposal：行操作/工具栏按钮请求被转换并写入生成的提案。
func TestCreateCompositeProposalWithActionsV9(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()
	svc := NewContractService(db)
	seedCompositeV9(t, svc, ctx)

	proposal, err := svc.CreateCompositeProposal(ctx, "g9", "e9", "composite--actions", []CompositeSectionRequest{
		{FunctionID: "player.get", View: "fields", Title: "玩家信息"},
		{
			FunctionID: "order.list", View: "table", Title: "订单",
			RowActions: []CompositeRowActionRequest{{
				Label:         "查看玩家",
				TargetSection: "player.get",
				Params:        map[string]string{"id": "row.playerId"},
				Chain:         []ActionStepReq{{Kind: "navigate", Target: "player.get", Params: map[string]string{"id": "row.uid"}}},
			}},
			ToolbarActions: []CompositeToolbarActionRequest{{
				Label:  "刷新",
				Danger: true,
				Chain:  []ActionStepReq{{Kind: "reload"}},
			}},
			Events: []EventBindingReq{{
				Event:  "rowClick",
				Action: ActionStepReq{Kind: "openModal", Target: "detail"},
			}},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, proposal)

	var page spec.PageSpec
	require.NoError(t, jsonUnmarshalV9(proposal.PageSpec, &page))
	require.NotNil(t, page.Composite)
	require.NotEmpty(t, page.Composite.Sections)
}

// CreateCompositeProposal：提案写入失败（触发器阻断 INSERT）。
func TestCreateCompositeProposalUpsertErrorV9(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()
	svc := NewContractService(db)
	seedCompositeV9(t, svc, ctx)
	abortWritesV9(t, db, &model.PageProposal{}, "INSERT")

	_, err := svc.CreateCompositeProposal(ctx, "g9", "e9", "composite--fail", []CompositeSectionRequest{
		{FunctionID: "player.get", View: "fields"},
		{FunctionID: "order.list", View: "table"},
	})
	assert.Error(t, err)
}

// 常量表单（static）区块：无契约/绑定，编辑器透传 form.jsonSchema；
// 与绑定区块混合创建 → 发布形状校验通过（static 无 bindingId 合法）。
func TestCreateCompositeProposalWithStaticSectionV9(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()
	svc := NewContractService(db)
	seedCompositeV9(t, svc, ctx)

	_, err := svc.CreateCompositeProposal(ctx, "g9", "e9", "static-missing-form", []CompositeSectionRequest{
		{FunctionID: "player.get"},
		{FunctionID: "order.list"},
		{Key: "consts", Static: true, Title: "常量"},
	})
	assert.ErrorContains(t, err, "requires form.jsonSchema")

	proposal, err := svc.CreateCompositeProposal(ctx, "g9", "e9", "static-mix", []CompositeSectionRequest{
		{FunctionID: "player.get", View: "fields"},
		{FunctionID: "order.list", View: "table", RefreshOn: []string{"consts"}},
		{
			Key:    "consts",
			Title:  "常量筛选",
			Static: true,
			Form: &spec.FormPresentationSpec{
				JSONSchema: spec.JSONSchema(`{"type":"object","properties":{"env":{"type":"string","title":"环境","enum":["prod","stage"]}}}`),
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, proposal)

	var pageSpec spec.PageSpec
	require.NoError(t, jsonUnmarshalV9(proposal.PageSpec, &pageSpec))
	require.NotNil(t, pageSpec.Composite)

	// static 区块：无 bindingId，schema 保留
	var staticSec *spec.CompositeSection
	bindingKeys := map[string]bool{}
	for i := range pageSpec.Composite.Sections {
		sec := &pageSpec.Composite.Sections[i]
		if sec.Static {
			staticSec = sec
		} else {
			bindingKeys[sec.BindingID] = true
		}
	}
	require.NotNil(t, staticSec, "static section should be appended")
	assert.Equal(t, "consts", staticSec.Key)
	assert.Equal(t, "form", staticSec.View)
	assert.Empty(t, staticSec.BindingID)
	require.NotNil(t, staticSec.Form)
	assert.Contains(t, string(staticSec.Form.JSONSchema), "prod")

	// 每个非 static 区块都有独立 binding；static 区块不在 bindings 中
	assert.Len(t, bindingKeys, 2)
	for _, b := range pageSpec.Bindings {
		assert.False(t, staticSec.Key == b.ID, "static section must not produce a binding")
	}

	// 发布形状校验：static 无 bindingId 不再报 binding_missing
	diags := spec.ValidatePublishablePageShape(pageSpec)
	for _, d := range diags {
		assert.NotContains(t, d.Code, "binding_missing", "static section must not trigger binding_missing")
	}
}
