package utils

// 本文件补齐 function_helpers.go EnsureFunctionEnabled（E2 执行门禁）的
// 包内语句覆盖。该函数被 internal/api/function 的测试间接执行过，但
// 覆盖率按包归属计算，必须在 utils 包内自证全部分支：
// nil 防御、无物化行放行（NotFound 归一）、非 NotFound 错误透传、
// disabled 拦截（稳定错误码 function_disabled）、enabled 放行。

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

// setupEnsureEnabledClosedContext 建库后立即关闭连接池：FindByFunctionID
// 返回的是底层连接错误而非 ErrRecordNotFound，用于覆盖「非 NotFound
// 错误原样透传」分支——门禁不能把基础设施故障吞成放行。
func setupEnsureEnabledClosedContext(t *testing.T) *svc.ServiceContext {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	return &svc.ServiceContext{DB: db, FunctionModel: model.NewFunctionModel(db)}
}

// TestEnsureFunctionEnabled_NilGuards 覆盖两个 nil 防御分支：svcCtx 为
// nil（零值依赖注入）与 FunctionModel 未初始化都不得 panic，按放行处理
// ——门禁依赖的写路径（setFunctionEnabled）同样触达不到这些形态。
func TestEnsureFunctionEnabled_NilGuards(t *testing.T) {
	assert.NoError(t, EnsureFunctionEnabled(context.Background(), nil, "fn.1"))
	assert.NoError(t, EnsureFunctionEnabled(context.Background(), &svc.ServiceContext{}, "fn.1"))
}

// TestEnsureFunctionEnabled_NotMaterialized 覆盖 NotFound 归一分支：
// functions 表无行（契约未物化）时门禁不拦——disable 写路径同样写不了
// 不存在的行，拦截语义与禁用入口保持同一张真值表。
func TestEnsureFunctionEnabled_NotMaterialized(t *testing.T) {
	svcCtx := setupInvokePermContextV9(t)
	assert.NoError(t, EnsureFunctionEnabled(context.Background(), svcCtx, "fn.never-registered"))
}

// TestEnsureFunctionEnabled_Disabled 覆盖禁用拦截分支：status=disabled
// 必须返回稳定错误码 function_disabled 的 409，错误码与页面生成器的
// 同名诊断对齐同一语义词汇表，前端据此给出「去函数目录重新启用」引导。
func TestEnsureFunctionEnabled_Disabled(t *testing.T) {
	svcCtx := setupInvokePermContextV9(t)
	require.NoError(t, svcCtx.DB.Create(&model.Function{
		FunctionID: "fn.off",
		Name:       "off",
		Status:     model.StatusDisabled,
	}).Error)

	err := EnsureFunctionEnabled(context.Background(), svcCtx, "fn.off")
	require.Error(t, err)
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok, "禁用拦截必须返回 CodeError 供错误码分支")
	assert.Equal(t, 409, codeErr.Code)
	assert.Equal(t, "function_disabled", codeErr.StableCode)
}

// TestEnsureFunctionEnabled_Enabled 覆盖放行分支：status=enabled 不拦。
func TestEnsureFunctionEnabled_Enabled(t *testing.T) {
	svcCtx := setupInvokePermContextV9(t)
	require.NoError(t, svcCtx.DB.Create(&model.Function{
		FunctionID: "fn.on",
		Name:       "on",
		Status:     model.StatusEnabled,
	}).Error)
	assert.NoError(t, EnsureFunctionEnabled(context.Background(), svcCtx, "fn.on"))
}

// TestEnsureFunctionEnabled_QueryError 覆盖非 NotFound 错误透传分支。
func TestEnsureFunctionEnabled_QueryError(t *testing.T) {
	svcCtx := setupEnsureEnabledClosedContext(t)
	err := EnsureFunctionEnabled(context.Background(), svcCtx, "fn.any")
	assert.Error(t, err)
}
