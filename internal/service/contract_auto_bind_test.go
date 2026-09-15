package service

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unboundMaterialInput 模拟 T4 上传管线产物：OpenAPI 富分类（resource/
// capability/risk/permission/approval）+ 文档推导 schema。
func unboundMaterialInput(id string) spec.FunctionContractInput {
	return spec.FunctionContractInput{
		ID: id, Resource: "player", Capability: "item_query", Execution: "sync",
		Summary:           "查询玩家",
		InputSchema:       `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`,
		OutputSchema:      `{"type":"object","properties":{"player":{"type":"object"}}}`,
		Risk:              "warning",
		Permission:        "player:read",
		ApprovalRequired:  true,
		ApprovalPolicyKey: "two_person",
	}
}

// sdkRegistrationInput 模拟运行时注册：仅函数名 + 真实 schema，无分类标注。
func sdkRegistrationInput(id string) spec.FunctionContractInput {
	return spec.FunctionContractInput{
		ID:           id,
		Version:      "1.0.0",
		InputSchema:  `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`,
		OutputSchema: `{"type":"object","properties":{"player":{"type":"object"}}}`,
	}
}

// T6/D3 验收：unbound → 注册同名函数 → 自动 bound，且上传物料的富分类
// （resource/capability/risk/permission/approval）不被稀疏的运行时注册
// 覆盖丢失；审计事件 openapi_source.binding_auto 落库。
func TestAutoBindUnboundContractOnRuntimeRegistration(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()
	auditStore := audit.NewInMemoryAuditStore()
	svc := NewContractService(db).WithAuditService(audit.NewAuditService(auditStore, nil))

	created, err := svc.CreateUnboundContract(ctx, "g-t6", "e-t6", "openapi", unboundMaterialInput("player.get"))
	require.NoError(t, err)
	require.True(t, created, "首传应新建 unbound 契约")

	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g-t6", "e-t6", "sdk", sdkRegistrationInput("player.get")))

	contractModel := model.NewFunctionContractModel(db)
	stored, err := contractModel.FindByScopeAndFunctionID(ctx, "g-t6", "e-t6", "player.get")
	require.NoError(t, err)
	assert.Equal(t, string(spec.ExecutionStateBound), stored.ExecutionState, "运行时注册同名函数自动置 bound")
	// 分类回补：SDK 未声明的字段保留 OpenAPI 物料值。
	assert.Equal(t, "player", stored.ResourceKey)
	assert.Equal(t, "item_query", stored.Capability.String())
	assert.Equal(t, "warning", stored.Risk.String())
	assert.Equal(t, "player:read", stored.Permission)
	assert.True(t, stored.Approval["required"] == true, "审批要求不因运行时未声明而丢失")
	assert.Equal(t, "two_person", stored.Approval["policyKey"])

	records, total, err := auditStore.List(audit.AuditFilter{EventType: []audit.AuditEventType{audit.EventOpenAPISourceBindingAuto}}, audit.AuditPage{PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total, "自动绑定必须产出 binding_auto 审计事件")
	assert.Equal(t, "player.get", records[0].Resource.ID)
	assert.Equal(t, "openapi", records[0].Details["materialSource"], "审计标注物料来源")

	// 幂等：再次重注册不重复写 binding_auto（已 bound，非翻转）。
	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g-t6", "e-t6", "sdk", sdkRegistrationInput("player.get")))
	_, total, err = auditStore.List(audit.AuditFilter{EventType: []audit.AuditEventType{audit.EventOpenAPISourceBindingAuto}}, audit.AuditPage{PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, total, "非 unbound→bound 翻转的重注册不重复审计")
}

// T6 验收：functionId 不匹配不误绑——注册其他函数不触碰既有 unbound 行，
// 未匹配上传物料的新注册照常写 bound。
func TestAutoBindDoesNotMatchDifferentFunctionID(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()
	auditStore := audit.NewInMemoryAuditStore()
	svc := NewContractService(db).WithAuditService(audit.NewAuditService(auditStore, nil))

	created, err := svc.CreateUnboundContract(ctx, "g-t6", "e-t6", "openapi", unboundMaterialInput("player.get"))
	require.NoError(t, err)
	require.True(t, created)

	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g-t6", "e-t6", "sdk", sdkRegistrationInput("order.list")))

	contractModel := model.NewFunctionContractModel(db)
	unbound, err := contractModel.FindByScopeAndFunctionID(ctx, "g-t6", "e-t6", "player.get")
	require.NoError(t, err)
	assert.Equal(t, string(spec.ExecutionStateUnbound), unbound.ExecutionState, "注册其他函数不得误绑 unbound 行")

	bound, err := contractModel.FindByScopeAndFunctionID(ctx, "g-t6", "e-t6", "order.list")
	require.NoError(t, err)
	assert.Equal(t, string(spec.ExecutionStateBound), bound.ExecutionState, "未匹配物料的注册照常写 bound")

	_, total, err := auditStore.List(audit.AuditFilter{EventType: []audit.AuditEventType{audit.EventOpenAPISourceBindingAuto}}, audit.AuditPage{PageSize: 10})
	require.NoError(t, err)
	assert.Zero(t, total, "无匹配翻转时不写 binding_auto")
}

// T6 验收：运行时声明的字段优先于上传物料（回补只填空缺，不反向覆盖）。
func TestAutoBindKeepsRuntimeDeclaredFields(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()
	svc := NewContractService(db)

	created, err := svc.CreateUnboundContract(ctx, "g-t6", "e-t6", "openapi", unboundMaterialInput("player.get"))
	require.NoError(t, err)
	require.True(t, created)

	runtimeInput := sdkRegistrationInput("player.get")
	runtimeInput.Resource = "account"
	runtimeInput.Risk = "danger"
	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g-t6", "e-t6", "sdk", runtimeInput))

	stored, err := model.NewFunctionContractModel(db).FindByScopeAndFunctionID(ctx, "g-t6", "e-t6", "player.get")
	require.NoError(t, err)
	assert.Equal(t, "account", stored.ResourceKey, "运行时显式声明优先")
	assert.Equal(t, "danger", stored.Risk.String())
	assert.Equal(t, "item_query", stored.Capability.String(), "未声明的仍由物料回补")
}
