package versioning

// 设计债回归测试：contractsForPage 循环内对契约查询错误一律 continue，契约
// 表故障被静默吞成「无契约」且恒返 nil error，调用方（GetChangeChain /
// functionSpecsByID / regenerateStandaloneProposal）的错误检查沦为死代码。
// 已定案修法：ErrRecordNotFound 视为契约缺失的正常业务态跳过；其他错误包装
// 为 "load contract %s" 传播。

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 契约查询报错（非 NotFound，这里用 DropTable 注入表故障）时错误必须传播，
// 且错误信息定位到具体的 functionID。
func TestContractsForPage_QueryErrorPropagates(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	require.NoError(t, db.Migrator().DropTable("function_contracts"))

	pageSpec := spec.PageSpec{
		Bindings: []spec.PageFunctionBinding{
			{ID: "b1", FunctionID: "fn1"},
		},
	}
	contracts, err := svc.contractsForPage(context.Background(), "game1", "prod", pageSpec)
	require.Error(t, err)
	assert.Nil(t, contracts)
	assert.Contains(t, err.Error(), "load contract fn1")
	assert.NotContains(t, strings.ToLower(err.Error()), "not found", "表故障不得被误判为契约缺失")
}

// 契约缺失（gorm.ErrRecordNotFound）是正常业务态：跳过该 binding，不报错。
func TestContractsForPage_NotFoundSkipsSilently(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)

	pageSpec := spec.PageSpec{
		Bindings: []spec.PageFunctionBinding{
			{ID: "b1", FunctionID: "nonexistent"},
		},
	}
	contracts, err := svc.contractsForPage(context.Background(), "game1", "prod", pageSpec)
	require.NoError(t, err)
	assert.Empty(t, contracts)
}

// GetChangeChain：页面存在但契约表故障 → "list contracts" 包装传播，
// 不得静默返回空契约链（否则表故障被掩盖成「无函数变更」）。
func TestGetChangeChain_ContractQueryErrorPropagates(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)

	page := spec.PageSpec{
		PageKey: "operation--player.ban",
		Type:    spec.PageTypeOperation,
		Bindings: []spec.PageFunctionBinding{
			{ID: "run", FunctionID: "player.ban", Usage: spec.BindingUsageAction},
		},
	}
	require.NoError(t, createVersioningTestPage(db, "demo-game", "development", page))
	// GetChangeChain 先查 page 后查契约，page 表保留、仅契约表故障。
	require.NoError(t, db.Migrator().DropTable("function_contracts"))

	chain, err := svc.GetChangeChain(context.Background(), &GetChangeChainRequest{
		GameID:  "demo-game",
		Env:     "development",
		PageKey: page.PageKey,
	})
	require.Error(t, err)
	assert.Nil(t, chain)
	assert.Contains(t, err.Error(), "list contracts")
	assert.Contains(t, err.Error(), "load contract player.ban")
}

// functionSpecsByID：contractsForPage 的错误原样传播到调用方。
func TestFunctionSpecsByID_ContractQueryErrorPropagates(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	require.NoError(t, db.Migrator().DropTable("function_contracts"))

	pageSpec := spec.PageSpec{
		Bindings: []spec.PageFunctionBinding{
			{ID: "b1", FunctionID: "fn1"},
		},
	}
	specs, err := svc.functionSpecsByID(context.Background(), "game1", "prod", pageSpec)
	require.Error(t, err)
	assert.Nil(t, specs)
	assert.Contains(t, err.Error(), "load contract fn1")
}

// regenerateStandaloneProposal：主契约查询成功后，functionSpecsByID 内的
// 契约查询故障必须中断重新生成——用 Before("gorm:query") 按 functionID
// 定点注错，保证主契约（player.ban）可查而次要 binding（boom-fn）故障。
func TestRegenerateStandaloneProposal_ContractQueryErrorAborts(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	ctx := context.Background()

	require.NoError(t, model.NewFunctionContractModel(db).UpsertContract(ctx, &model.FunctionContract{
		GameID:      "demo-game",
		Env:         "development",
		FunctionID:  "player.ban",
		Version:     "1.0.0",
		Enabled:     true,
		ResourceKey: "player",
		Capability:  dbenum.CapabilityAction,
		UpdatedAt:   time.Now(),
	}))
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("test:contract_boom", func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != "function_contracts" {
			return
		}
		// Statement.Vars 在 gorm:query 的 BuildQuerySQL 阶段才填充，故必须
		// 用 After 钩子按参数定点覆盖错误（Before 阶段 Vars 恒空）。
		for _, v := range tx.Statement.Vars {
			if s, ok := v.(string); ok && s == "boom-fn" {
				tx.Error = errors.New("injected contract query failure")
				return
			}
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove("test:contract_boom") })

	pageSpec := spec.PageSpec{
		PageKey: "operation--player.ban",
		Type:    spec.PageTypeOperation,
		Bindings: []spec.PageFunctionBinding{
			{ID: "run", FunctionID: "player.ban", Usage: spec.BindingUsageAction},
			{ID: "row", FunctionID: "boom-fn", Usage: spec.BindingUsageQuery},
		},
	}
	err := svc.regenerateStandaloneProposal(ctx, "demo-game", "development", pageSpec, &model.PageSpec{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected contract query failure")
}
