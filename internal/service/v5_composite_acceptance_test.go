package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

// TestV5CompositeProposalRoundTrip T5.7 验收：V5 特有的 wire 形态（inputAssignments
// 多段 page_state 路径、row.x 行操作参数、事件 params 表达式原文）通过
// CreateCompositeProposal → AcceptAndPublishProposal 完整往返，并在
// PublishedPageSpec 中保持不变——服务端不解释表达式、只透传编译产物（设计 D2）。
func TestV5CompositeProposalRoundTrip(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/v5.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.FunctionContract{},
		&model.CapabilitySemantics{},
		&model.ResourceCapability{},
		&model.CapabilitySemanticVersion{},
		&model.PageProposal{},
		&model.PageProposalVersion{},
		&model.BlockedProposalIssue{},
		&model.PageSpec{},
		&model.PublishedPageSpec{},
		&model.PageVersion{},
		&model.TermDictionary{},
		&model.Alert{},
	))

	svc := NewContractService(db)
	proposalSvc := NewProposalService(db)
	ctx := context.Background()

	// 种契约：player.list（表格）+ mail.send（操作表单）
	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "demo", "prod", "sdk", spec.FunctionContractInput{
		ID: "player.list", Resource: "player", Capability: "collection_query", Execution: "sync", Enabled: true,
		InputSchema:  `{"type":"object","properties":{"keyword":{"type":"string"}}}`,
		OutputSchema: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"uid":{"type":"string"},"nickname":{"type":"string"}}}},"total":{"type":"integer"}}}`,
	}))
	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "demo", "prod", "sdk", spec.FunctionContractInput{
		ID: "mail.send", Resource: "mail", Capability: "action", Execution: "sync", Enabled: true,
		InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"},"title":{"type":"string"},"keyword":{"type":"string"},"total":{"type":"integer"}},"required":["playerId"]}`,
	}))

	// 创建组合页提案——包含 V5 wire 形态
	v5Sections := []CompositeSectionRequest{
		{
			Key:        "playerListTable",
			FunctionID: "player.list",
			View:       "table",
			Title:      "玩家列表",
			Span:       16,
			AutoRun:    true,
			RowActions: []CompositeRowActionRequest{
				{
					Label:         "发邮件",
					TargetSection: "mailSendModal",
					Params: map[string]string{
						"playerId": "row.uid",      // {{row.uid}} 编译产物
						"nickname": "row.nickname", // {{row.nickname}} 编译产物
					},
				},
			},
			Events: []EventBindingReq{
				{
					Event: "rowSelected",
					Action: ActionStepReq{
						Kind:   "runBinding",
						Target: "mailSendForm",
						Params: map[string]string{
							"nickname": "{{playerListTable.selectedRow.nickname}}", // V5 表达式原文
						},
					},
				},
			},
		},
		{
			Key:        "filterForm",
			FunctionID: "player.list",
			View:       "form",
			Title:      "筛选",
			Span:       8,
			Static:     true,
			Form: &spec.FormPresentationSpec{
				JSONSchema: spec.JSONSchema(`{"type":"object","properties":{"keyword":{"type":"string"}}}`),
			},
		},
		{
			Key:        "mailSendForm",
			FunctionID: "mail.send",
			View:       "form",
			Title:      "发邮件",
			Display:    "dialog",
			Group:      "mailSendModal",
			InputAssignments: []CompositeInputAssignmentRequest{
				// V5 page_state 多段路径（§6 编译规则）：selectedRow / values / data 分支
				{Target: "/playerId", Kind: "page_state", Key: "playerListTable", Path: "/selectedRow/uid"},
				{Target: "/keyword", Kind: "page_state", Key: "filterForm", Path: "/values/keyword"},
				{Target: "/total", Kind: "page_state", Key: "playerListTable", Path: "/data/total"},
				{Target: "/title", Kind: "literal", Value: json.RawMessage(`"固定标题"`)},
			},
		},
	}

	proposal, err := svc.CreateCompositeProposal(ctx, "demo", "prod", "composite--v5-player", v5Sections, nil)
	require.NoError(t, err, "CreateCompositeProposal must accept V5 wire format")
	require.NotNil(t, proposal)

	// 接受并发布
	result, err := proposalSvc.AcceptAndPublishProposal(ctx, "demo", "prod", proposal.ProposalKey)
	require.NoError(t, err, "AcceptAndPublishProposal must not reject V5 wire format")
	assert.NotEqual(t, 0, result.PublishedVersion, "published version should be set")

	// 读取发布快照，验证 V5 wire 形态完整保留
	published, err := model.NewPublishedPageSpecModel(db).
		FindLatestByScopeAndPageKey(ctx, "demo", "prod", proposal.PageKey)
	require.NoError(t, err, "published snapshot should exist")

	var pageSpec spec.PageSpec
	require.NoError(t, json.Unmarshal([]byte(published.SpecJSON), &pageSpec))
	require.NotNil(t, pageSpec.Composite, "composite spec should be present")

	secMap := map[string]spec.CompositeSection{}
	for _, s := range pageSpec.Composite.Sections {
		secMap[s.Key] = s
	}
	bindingByID := map[string]spec.PageFunctionBinding{}
	for _, b := range pageSpec.Bindings {
		bindingByID[b.ID] = b
	}

	// ---- inputAssignments：多段 page_state 路径保留在绑定 selectors 上 ----
	form := secMap["mailSendForm"]
	require.NotEmpty(t, form.BindingID, "dialog form section must reference a binding")
	formBinding := bindingByID[form.BindingID]
	require.NotNil(t, formBinding.Selectors, "mail.send binding must carry selectors")
	require.Len(t, formBinding.Selectors.Input.Assignments, 4, "4 explicit assignments expected")

	byTarget := map[string]spec.InputAssignment{}
	for _, a := range formBinding.Selectors.Input.Assignments {
		byTarget[a.Target] = a
	}
	assert.Equal(t, "playerListTable", byTarget["/playerId"].Source.Key)
	assert.Equal(t, "/selectedRow/uid", byTarget["/playerId"].Source.Path)
	assert.Equal(t, spec.SourcePageState, byTarget["/playerId"].Source.Kind)

	assert.Equal(t, "filterForm", byTarget["/keyword"].Source.Key)
	assert.Equal(t, "/values/keyword", byTarget["/keyword"].Source.Path)

	assert.Equal(t, "/data/total", byTarget["/total"].Source.Path)
	assert.Equal(t, "literal", string(byTarget["/title"].Source.Kind))
	assert.JSONEq(t, `"固定标题"`, string(byTarget["/title"].Source.Value))

	// ---- rowActions：row.x 参数保留 ----
	table := secMap["playerListTable"]
	require.NotNil(t, table.Table, "table section must have table spec")
	require.Len(t, table.Table.RowActions, 1)
	ra := table.Table.RowActions[0]
	assert.Equal(t, "row.uid", ra.Params["playerId"], "rowAction playerId preserves row.uid")
	assert.Equal(t, "row.nickname", ra.Params["nickname"], "rowAction nickname preserves row.nickname")

	// ---- 事件绑定：表达式 params 原文保留 ----
	require.Len(t, table.Events, 1)
	ev := table.Events[0]
	assert.Equal(t, "rowSelected", ev.Event)
	assert.Equal(t, "{{playerListTable.selectedRow.nickname}}", ev.Action.Params["nickname"],
		"event params preserve expression string verbatim")

	// ---- sections 顺序：static 区块（filterForm 输入在中间）按输入位置保留，
	// accept-and-publish 全链路后发布快照顺序与请求一致 ----
	order := make([]string, 0, len(pageSpec.Composite.Sections))
	for _, s := range pageSpec.Composite.Sections {
		order = append(order, s.Key)
	}
	assert.Equal(t, []string{"playerListTable", "filterForm", "mailSendForm"}, order,
		"static section must stay at its input position after accept-and-publish")
}
