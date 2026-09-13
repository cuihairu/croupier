package versioning

// 设计债回归测试：contractsForPage 循环内对契约查询错误一律 continue，契约
// 表故障被静默吞成「无契约」且恒返 nil error，调用方（GetChangeChain /
// functionSpecsByID / regenerateStandaloneProposal）的错误检查沦为死代码。
// 已定案修法：ErrRecordNotFound 视为契约缺失的正常业务态跳过；其他错误包装
// 为 "load contract %s" 传播。

import (
	"context"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
