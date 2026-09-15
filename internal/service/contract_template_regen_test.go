package service

import (
	"context"
	"errors"
	"testing"

	componentapi "github.com/cuihairu/croupier/internal/api/component"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// withTestTemplateRegen 注入真实组件模块闭包（与 routes.go 装配等价），
// 测试结束恢复未注入态（包级单例，避免污染其他用例）。
func withTestTemplateRegen(t *testing.T, db *gorm.DB) (*model.ComponentTemplateModel, *int) {
	t.Helper()
	tplModel := model.NewComponentTemplateModel(db)
	contractModel := model.NewFunctionContractModel(db)
	compHandler := componentapi.NewHandler(tplModel, contractModel)
	calls := 0
	SetContractTemplateRegenerator(func(ctx context.Context, gameID, env string) error {
		calls++
		contracts, err := contractModel.ListByScope(ctx, gameID, env)
		if err != nil {
			return err
		}
		return compHandler.RegenerateFromContracts(ctx, contracts)
	})
	t.Cleanup(func() { SetContractTemplateRegenerator(nil) })
	return tplModel, &calls
}

func t2ContractInput(id string) spec.FunctionContractInput {
	return spec.FunctionContractInput{
		ID: id, Resource: "player", Capability: "item_query", Execution: "sync", Enabled: true,
		InputSchema:  `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`,
		OutputSchema: `{"type":"object","properties":{"player":{"type":"object"}}}`,
	}
}

// 契约重建本体不再内联触发模板联动（事务内写全局模板表在文件型 sqlite
// 下与事务写锁自死锁，收口挪到各事务边界提交后的
// RegenerateContractTemplates——注册边界的行为见 registry
// store_template_regen_test.go）。
func TestContractRebuildDoesNotRegenTemplatesInline(t *testing.T) {
	db := setupTestDBFileV9(t)
	require.NoError(t, db.AutoMigrate(&model.ComponentTemplate{}))
	_, calls := withTestTemplateRegen(t, db)

	svc := NewContractService(db)
	require.NoError(t, svc.RebuildContractFromFunctionMeta(context.Background(), "g-t2", "e-t2", "agent-1", t2ContractInput("player.get")))
	assert.Equal(t, 0, *calls, "契约落库不应内联触发模板重建（由事务边界提交后收口）")
}

// T2 验收：提交后单次收口——RegenerateContractTemplates 拉当前 scope 契约
// 全量重建，builtin 模板自动生成（无需手动 regenerate 端点）。
func TestRegenerateContractTemplatesBuildsTemplates(t *testing.T) {
	db := setupTestDBFileV9(t)
	require.NoError(t, db.AutoMigrate(&model.ComponentTemplate{}))
	tplModel, calls := withTestTemplateRegen(t, db)
	ctx := context.Background()
	svc := NewContractService(db)

	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g-t2", "e-t2", "agent-1", t2ContractInput("player.get")))
	require.NoError(t, svc.RegenerateContractTemplates(ctx, "g-t2", "e-t2"))
	assert.Equal(t, 1, *calls, "收口入口应触发一次模板重建")

	tpl, err := tplModel.FindByKey(ctx, "fn--player.get")
	require.NoError(t, err, "单函数 builtin 模板应自动生成")
	assert.True(t, tpl.Builtin)
}

// 未注入（单测/裁剪部署）时收口入口 no-op 返回 nil，不 panic。
func TestRegenerateContractTemplatesNoopWithoutInjector(t *testing.T) {
	SetContractTemplateRegenerator(nil)
	assert.NoError(t, RegenerateContractTemplates(context.Background(), "g", "e"))
	assert.NoError(t, NewContractService(nil).RegenerateContractTemplates(context.Background(), "g", "e"))
}

// 收口失败把错误返回给调用方（注册/绑定边界按 T2 语义仅告警不回滚）。
func TestRegenerateContractTemplatesReturnsClosureError(t *testing.T) {
	SetContractTemplateRegenerator(func(ctx context.Context, gameID, env string) error {
		return errors.New("regen boom")
	})
	t.Cleanup(func() { SetContractTemplateRegenerator(nil) })

	err := RegenerateContractTemplates(context.Background(), "g-t2", "e-t2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "regen boom")
}
