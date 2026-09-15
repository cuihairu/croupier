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

// T2 验收：契约 upsert 后 builtin 模板自动更新（无需手动 regenerate）。
func TestContractUpsertAutoRegeneratesTemplates(t *testing.T) {
	db := setupTestDBFileV9(t)
	require.NoError(t, db.AutoMigrate(&model.ComponentTemplate{}))
	tplModel, calls := withTestTemplateRegen(t, db)
	ctx := context.Background()
	svc := NewContractService(db)

	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g-t2", "e-t2", "agent-1", t2ContractInput("player.get")))
	assert.Equal(t, 1, *calls, "新契约应触发一次模板重建")

	tpl, err := tplModel.FindByKey(ctx, "fn--player.get")
	require.NoError(t, err, "单函数 builtin 模板应自动生成")
	assert.True(t, tpl.Builtin)
}

// T2 验收：schema 未变的重注册不触发重建（心跳重连风暴下不空转）。
func TestContractReRegisterSkipsRegenWhenDigestUnchanged(t *testing.T) {
	db := setupTestDBFileV9(t)
	require.NoError(t, db.AutoMigrate(&model.ComponentTemplate{}))
	_, calls := withTestTemplateRegen(t, db)
	ctx := context.Background()
	svc := NewContractService(db)

	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g-t2", "e-t2", "agent-1", t2ContractInput("player.get")))
	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g-t2", "e-t2", "agent-1", t2ContractInput("player.get")))
	assert.Equal(t, 1, *calls, "digest 未变的重注册不应再次触发")

	// schema 实质变化 → 再次触发。
	changed := t2ContractInput("player.get")
	changed.OutputSchema = `{"type":"object","properties":{"player":{"type":"object"},"gold":{"type":"integer"}}}`
	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g-t2", "e-t2", "agent-1", changed))
	assert.Equal(t, 2, *calls, "契约实质变更应再次触发")
}

// T2 验收：regenerate 失败不阻塞契约重建主流程。
func TestContractUpsertSurvivesRegenFailure(t *testing.T) {
	db := setupTestDBFileV9(t)
	require.NoError(t, db.AutoMigrate(&model.ComponentTemplate{}))
	SetContractTemplateRegenerator(func(ctx context.Context, gameID, env string) error {
		return errors.New("regen boom")
	})
	t.Cleanup(func() { SetContractTemplateRegenerator(nil) })

	svc := NewContractService(db)
	require.NoError(t, svc.RebuildContractFromFunctionMeta(context.Background(), "g-t2", "e-t2", "agent-1", t2ContractInput("player.get")),
		"模板重建失败不应影响契约注册")

	contracts, err := model.NewFunctionContractModel(db).ListByScope(context.Background(), "g-t2", "e-t2")
	require.NoError(t, err)
	assert.Len(t, contracts, 1)
}
