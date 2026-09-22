package openapi

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	dashboardservice "github.com/cuihairu/croupier/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 本文件收敛 CreateSource/UpdateSource/CreateBinding/DeleteBinding、
// createUnboundContractsForSource/removeSupersededUnboundContract、
// unboundFunctionID、RuntimeSources 与 rebuildProposalsForUnboundContracts
// 的残余错误分支。错误注入全部走两类仓库既定手法：
//  1. gorm 回调按表名注错（主回调在 db.Error != nil 时短路，SQL 不真正下发）；
//  2. dashboardservice.SetContractTemplateRegenerator 进程级注入点（模板
//     重建失败仅告警的两处 warn 分支）。

// newCoverageFService 复用包内标准测试装配（不收窄连接池：CreateBinding
// 的 scopedTransaction 持连接期间，事务内契约/提案重建会走根连接池再取
// 连接，MaxOpenConns(1) 会造成自等死锁——goroutine 转储实证）。
func newCoverageFService(t *testing.T) *Service {
	t.Helper()
	return setupOpenAPITestService(t)
}

// coverageFStatementTable 取当前语句的目标表名（Statement.Table 为空时
// 回退已解析 schema 的表名）。
func coverageFStatementTable(tx *gorm.DB) string {
	if tx.Statement == nil {
		return ""
	}
	if tx.Statement.Table != "" {
		return tx.Statement.Table
	}
	if tx.Statement.Schema != nil {
		return tx.Statement.Schema.Table
	}
	return ""
}

// injectCoverageFQueryFailure 目标表的 SELECT 一律失败（快照/查询路径）。
func injectCoverageFQueryFailure(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	require.NoError(t, db.Callback().Query().Before("gorm:Query").Register("coverage_f_fail_query_"+table, func(tx *gorm.DB) {
		if coverageFStatementTable(tx) == table {
			tx.AddError(errors.New("coverageF injected query failure on " + table))
		}
	}))
}

// injectCoverageFCreateFailure 目标表的 INSERT 一律失败（落库路径）。
func injectCoverageFCreateFailure(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	require.NoError(t, db.Callback().Create().Before("gorm:Create").Register("coverage_f_fail_create_"+table, func(tx *gorm.DB) {
		if coverageFStatementTable(tx) == table {
			tx.AddError(errors.New("coverageF injected create failure on " + table))
		}
	}))
}

// injectCoverageFWriteFailure 目标表的软删（DELETE 处理器）与更新（UPDATE
// 处理器）一律失败——FunctionContract 内嵌 gorm.Model，软删走 DELETE
// 处理器；两类都覆盖以防模型形态变化。
func injectCoverageFWriteFailure(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	require.NoError(t, db.Callback().Delete().Before("gorm:Delete").Register("coverage_f_fail_delete_"+table, func(tx *gorm.DB) {
		if coverageFStatementTable(tx) == table {
			tx.AddError(errors.New("coverageF injected delete failure on " + table))
		}
	}))
	require.NoError(t, db.Callback().Update().Before("gorm:Update").Register("coverage_f_fail_update_"+table, func(tx *gorm.DB) {
		if coverageFStatementTable(tx) == table {
			tx.AddError(errors.New("coverageF injected update failure on " + table))
		}
	}))
}

// coverageFPlayerListSpec 单操作标准文档：GET /players，operationId
// player.list，x-resource/x-operation 显式声明（不依赖 REST 分类器推断）。
func coverageFPlayerListSpec(t *testing.T) json.RawMessage {
	t.Helper()
	return rawSpec(t, map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Player API", "version": "1.0.0"},
		"paths": map[string]interface{}{
			"/players": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "player.list",
					"x-resource":  "player",
					"x-operation": "list",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "OK"},
					},
				},
			},
		},
	})
}

// coverageFTwoOpsSpec 双操作文档：在 player.list 之外新增 player.get，
// 供 UpdateSource 场景产生「本次新建 unbound 契约」。
func coverageFTwoOpsSpec(t *testing.T) json.RawMessage {
	t.Helper()
	return rawSpec(t, map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Player API", "version": "1.1.0"},
		"paths": map[string]interface{}{
			"/players": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "player.list",
					"x-resource":  "player",
					"x-operation": "list",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "OK"},
					},
				},
			},
			"/players/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "player.get",
					"x-resource":  "player",
					"x-operation": "get",
					"parameters": []interface{}{
						map[string]interface{}{
							"name":     "id",
							"in":       "path",
							"required": true,
							"schema":   map[string]interface{}{"type": "string"},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "OK"},
					},
				},
			},
		},
	})
}

// unboundFunctionID 归一化：非 [a-z0-9._-] 字符替换为 '-'（命中替换分支）；
// 全部分隔符组成的输入 Trim 后为空（命中空值分支）。
// 「fn- 前缀」分支（out[0] 非 a-z0-9）恒不可达：Trim 的 cutset ".-_" 恰为
// 字符集里全部非字母数字成员，非空结果的首字符必属 [a-z0-9]，见包内函数注释。
func TestUnboundFunctionIDNormalization(t *testing.T) {
	assert.Equal(t, "list-players", unboundFunctionID("List Players"), "空格归一为 '-'")
	assert.Equal(t, "", unboundFunctionID("..."), "全分隔符 Trim 后为空")
	assert.Equal(t, "", unboundFunctionID(" .- "), "分隔符与空格混排 Trim 后为空")
	assert.Equal(t, "abc123", unboundFunctionID("ABC123"), "大写折叠")
	assert.Equal(t, "list.players", unboundFunctionID("List.Players"), "点号保留")
}

// CreateSource：tracker 基线快照失败（内置模板表查询注入错误）→ 上传中止。
func TestCreateSourceTrackerSnapshotFailure(t *testing.T) {
	service := newCoverageFService(t)
	injectCoverageFQueryFailure(t, service.svcCtx.DB, "component_templates")

	_, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{
		Spec: coverageFPlayerListSpec(t),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "snapshot builtin component templates")
}

// UpdateSource：tracker 基线快照失败 → 更新中止（查找/绑定列表均在注入
// 表之前完成，错误必须落在 startUploadPipelineTracker 一行）。
func TestUpdateSourceTrackerSnapshotFailure(t *testing.T) {
	service := newCoverageFService(t)
	ctx := openAPITestContext()
	created, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{Spec: coverageFPlayerListSpec(t)})
	require.NoError(t, err)

	injectCoverageFQueryFailure(t, service.svcCtx.DB, "component_templates")
	_, err = service.UpdateSource(ctx, &OpenAPISourceUpdateRequest{
		SourceID: created.Source.SourceID,
		Spec:     coverageFTwoOpsSpec(t),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "snapshot builtin component templates")
}

// CreateSource：收尾管线里「按资源重建页面提案」失败（page_proposals 写入
// 注错）→ 错误从 rebuildProposalsForUnboundContracts 一路上抛。
func TestCreateSourceFinishPipelineProposalRebuildFailure(t *testing.T) {
	service := newCoverageFService(t)
	injectCoverageFCreateFailure(t, service.svcCtx.DB, "page_proposals")

	_, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{
		Spec: coverageFPlayerListSpec(t),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rebuild unbound page proposals")
}

// UpdateSource：收尾管线里新建契约（player.get）触发提案重建失败 → 错误
// 从 finishUploadPipeline 上抛为整体更新失败。
func TestUpdateSourceFinishPipelineProposalRebuildFailure(t *testing.T) {
	service := newCoverageFService(t)
	ctx := openAPITestContext()
	created, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{Spec: coverageFPlayerListSpec(t)})
	require.NoError(t, err)

	injectCoverageFCreateFailure(t, service.svcCtx.DB, "page_proposals")
	_, err = service.UpdateSource(ctx, &OpenAPISourceUpdateRequest{
		SourceID: created.Source.SourceID,
		Spec:     coverageFTwoOpsSpec(t),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rebuild unbound page proposals")
}

// CreateSource：unbound 契约落库失败（function_contracts 写入注错）→
// 按 operation 包装错误上抛，事务回滚。
func TestCreateSourceUnboundContractWriteFailure(t *testing.T) {
	service := newCoverageFService(t)
	injectCoverageFCreateFailure(t, service.svcCtx.DB, "function_contracts")

	_, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{
		Spec: coverageFPlayerListSpec(t),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create unbound contract for operation player.list")

	// 事务回滚：源行未落库。
	_, err = service.svcCtx.OpenAPISourceModel.FindByScopeAndSourceID(openAPITestContext(), "demo-game", "development", "")
	assert.Error(t, err)
}

// CreateBinding：不同名绑定清理被取代 unbound 契约时删除失败 → 绑定整体
// 失败（事务回滚，removeSupersededUnboundContract 的错误出口）。
func TestCreateBindingSupersededRemovalWriteFailure(t *testing.T) {
	service := newCoverageFService(t)
	// 绑定目标：与 operationId（player.list）不同名的运行时函数。
	require.NoError(t, service.svcCtx.RegistryStore.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-players",
		GameID:   "demo-game",
		Env:      "development",
		LastSeen: time.Now(),
		Functions: map[string]registry.FunctionMeta{
			"players.player.list": {Enabled: true, Version: "1.0.0"},
		},
	}))
	ctx := openAPITestContext()
	created, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{Spec: coverageFPlayerListSpec(t)})
	require.NoError(t, err)

	injectCoverageFWriteFailure(t, service.svcCtx.DB, "function_contracts")
	_, err = service.CreateBinding(ctx, &OpenAPISourceBindingCreateRequest{
		SourceID:    created.Source.SourceID,
		OperationID: "player.list",
		Kind:        "provider",
		FunctionID:  "players.player.list",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remove superseded unbound contract player.list")
}

// UpdateSource 重放：operation 已被手工写入的 provider 绑定取代，重放清理
// 被取代 unbound 契约时删除失败 → 更新整体失败（上传重放清理的错误出口）。
func TestUpdateSourceReplaySupersededRemovalWriteFailure(t *testing.T) {
	service := newCoverageFService(t)
	ctx := openAPITestContext()
	created, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{Spec: coverageFPlayerListSpec(t)})
	require.NoError(t, err)

	// 手工补一条 provider 绑定行（绕过 CreateBinding 的即时清理），让重放
	// 走 boundOperations 取代路径。
	require.NoError(t, service.svcCtx.OpenAPISourceBindingModel.Upsert(ctx, &model.OpenAPISourceBinding{
		GameID:      "demo-game",
		Env:         "development",
		SourceID:    created.Source.SourceID,
		BindingID:   "b-replay",
		OperationID: "player.list",
		Kind:        "provider",
		FunctionID:  "players.player.list",
	}))

	injectCoverageFWriteFailure(t, service.svcCtx.DB, "function_contracts")
	_, err = service.UpdateSource(ctx, &OpenAPISourceUpdateRequest{
		SourceID: created.Source.SourceID,
		Spec:     coverageFPlayerListSpec(t),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remove superseded unbound contract player.list")
}

// removeSupersededUnboundContract 的「非 openapi 来源放行」分支：unbound
// 契约被改成 sdk 来源后，即便同名函数已被 provider 绑定取代也不删。
// 直接单测调用——端到端走 UpdateSource 重放时，事务内的契约重建链会把
// source 改回 openapi，掩盖该分支的真实走向。
func TestRemoveSupersededUnboundContractKeepsNonOpenAPISource(t *testing.T) {
	service := newCoverageFService(t)
	ctx := openAPITestContext()
	_, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{Spec: coverageFPlayerListSpec(t)})
	require.NoError(t, err)

	// 把 unbound 契约改成 sdk 来源 → 不满足「unbound 且 openapi 来源」清理判据。
	require.NoError(t, service.svcCtx.DB.Model(&model.FunctionContract{}).
		Where("game_id = ? AND env = ? AND function_id = ?", "demo-game", "development", "player.list").
		Update("source", "sdk").Error)

	contractService := dashboardservice.NewContractService(service.svcCtx.DB)
	require.NoError(t, service.removeSupersededUnboundContract(ctx, contractService, "demo-game", "development", "player.list"))

	contract, err := model.NewFunctionContractModel(service.svcCtx.DB).
		FindByScopeAndFunctionID(ctx, "demo-game", "development", "player.list")
	require.NoError(t, err, "sdk 来源契约不应被清理")
	assert.Equal(t, "sdk", contract.Source)
}

// removeSupersededUnboundContract 的「契约不存在放行」分支（查询未命中
// 即无事可做）。
func TestRemoveSupersededUnboundContractNoopWhenAbsent(t *testing.T) {
	service := newCoverageFService(t)
	contractService := dashboardservice.NewContractService(service.svcCtx.DB)
	require.NoError(t, service.removeSupersededUnboundContract(
		openAPITestContext(), contractService, "demo-game", "development", "ghost.fn"))
}

// CreateBinding：模板重建器失败仅告警不阻断（提交后的收口语义）。
func TestCreateBindingTemplateRegenFailureWarns(t *testing.T) {
	service := newCoverageFService(t)
	dashboardservice.SetContractTemplateRegenerator(func(ctx context.Context, gameID, env string) error {
		return errors.New("coverageF injected regen failure")
	})
	t.Cleanup(func() { dashboardservice.SetContractTemplateRegenerator(nil) })

	ctx := openAPITestContext()
	created, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{Spec: coverageFPlayerListSpec(t)})
	require.NoError(t, err)

	binding, err := service.CreateBinding(ctx, &OpenAPISourceBindingCreateRequest{
		SourceID:    created.Source.SourceID,
		OperationID: "player.list",
		Kind:        "provider",
		FunctionID:  "player.list",
	})
	require.NoError(t, err, "模板重建失败不应阻断绑定成功路径")
	assert.Equal(t, "player.list", binding.Binding.BindingID)
}

// DeleteBinding：模板重建器失败仅告警不阻断（提交后的收口语义）。
func TestDeleteBindingTemplateRegenFailureWarns(t *testing.T) {
	service := newCoverageFService(t)
	ctx := openAPITestContext()
	created, err := service.CreateSource(ctx, &OpenAPISourceCreateRequest{Spec: coverageFPlayerListSpec(t)})
	require.NoError(t, err)
	binding, err := service.CreateBinding(ctx, &OpenAPISourceBindingCreateRequest{
		SourceID:    created.Source.SourceID,
		OperationID: "player.list",
		Kind:        "provider",
		FunctionID:  "player.list",
	})
	require.NoError(t, err)

	dashboardservice.SetContractTemplateRegenerator(func(ctx context.Context, gameID, env string) error {
		return errors.New("coverageF injected regen failure")
	})
	t.Cleanup(func() { dashboardservice.SetContractTemplateRegenerator(nil) })

	deleted, err := service.DeleteBinding(ctx, &OpenAPISourceBindingDeleteRequest{
		SourceID:  created.Source.SourceID,
		BindingID: binding.Binding.BindingID,
	})
	require.NoError(t, err, "模板重建失败不应阻断解绑成功路径")
	assert.Equal(t, binding.Binding.BindingID, deleted.Binding.BindingID)
}

// RuntimeSources：同 scope 两个 provider 会话按 ProviderID 排序输出。
func TestRuntimeSourcesSortsMultipleProviders(t *testing.T) {
	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read")
	store := service.svcCtx.RegistryStore
	now := time.Now()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-zulu",
		GameID:   "demo-game",
		Env:      "development",
		LastSeen: now,
		Providers: []registry.ProviderSession{{
			ProviderID:  "provider:zulu",
			Version:     "openapi-zulu",
			FunctionIDs: []string{"players.player.list"},
		}},
	}))
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-alpha",
		GameID:   "demo-game",
		Env:      "development",
		LastSeen: now,
		Providers: []registry.ProviderSession{{
			ProviderID:  "provider:alpha",
			Version:     "openapi-alpha",
			FunctionIDs: []string{"players.player.get"},
		}},
	}))

	resp, err := service.RuntimeSources(ctx, &RuntimeSourcesListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 2)
	assert.Equal(t, "provider:alpha", resp.Items[0].ProviderID, "按 ProviderID 稳定排序")
	assert.Equal(t, "provider:zulu", resp.Items[1].ProviderID)
	assert.Equal(t, 2, resp.Total)
}

// createUnboundContractsForSource 的空 operationId 跳过分支（continue）
// 恒不可达：operations 全部来自 parseValidSource → extractSourceOperations，
// 空 operationId 在提取阶段即产出 error 级诊断并整源拒绝，不会进入本函数。
// 同理 unboundFunctionID 的 "fn-" 前缀分支恒不可达（Trim cutset 恰为字符集
// 内全部非字母数字成员，非空结果首字符必属 [a-z0-9]）。以上两分支为防御性
// 代码保留，不删产品分支。
