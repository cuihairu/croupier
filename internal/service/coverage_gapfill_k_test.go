package service

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// HasBlockingDiagnostics 包装层：nil 提案不拦截；error 级诊断拦截；
// 仅 warn 不拦截；无效 JSON 不拦截（沿用 hasBlockingDiagnostics 语义）。
func TestHasBlockingDiagnosticsProposalWrapper(t *testing.T) {
	assert.False(t, HasBlockingDiagnostics(nil))

	warnOnly := &model.PageProposal{Diagnostics: []byte(`[{"severity":"warn","code":"x"}]`)}
	assert.False(t, HasBlockingDiagnostics(warnOnly))

	errLevel := &model.PageProposal{Diagnostics: []byte(`[{"severity":"warn"},{"severity":"error"}]`)}
	assert.True(t, HasBlockingDiagnostics(errLevel))

	badJSON := &model.PageProposal{Diagnostics: []byte(`{not-json`)}
	assert.False(t, HasBlockingDiagnostics(badJSON))

	empty := &model.PageProposal{}
	assert.False(t, HasBlockingDiagnostics(empty))
}

// localizedStringFromJSONMap：空 map 空串；zh-CN 优先于 en-US；空白值跳过；
// 任意非空 locale 兜底；非字符串值忽略。
func TestLocalizedStringFromJSONMap(t *testing.T) {
	assert.Equal(t, "", localizedStringFromJSONMap(datatypes.JSONMap{}))

	both := datatypes.JSONMap{"zh-CN": "玩家管理", "en-US": "Players"}
	assert.Equal(t, "玩家管理", localizedStringFromJSONMap(both))

	enOnly := datatypes.JSONMap{"en-US": "Players"}
	assert.Equal(t, "Players", localizedStringFromJSONMap(enOnly))

	// zh-CN 只有空白 → 落到 en-US
	blankZh := datatypes.JSONMap{"zh-CN": "   ", "en-US": "Players"}
	assert.Equal(t, "Players", localizedStringFromJSONMap(blankZh))

	// 非预设 locale：任意非空字符串兜底
	other := datatypes.JSONMap{"ja-JP": "プレイヤー", "count": 3.0}
	assert.Equal(t, "プレイヤー", localizedStringFromJSONMap(other))

	// 全部非字符串 → 空串
	nonString := datatypes.JSONMap{"count": 3.0}
	assert.Equal(t, "", localizedStringFromJSONMap(nonString))
}

// CreateUnboundContract 入口守卫与失败分支：空 functionId 拒绝、契约
// schema 携带表现层字段拒绝、find 查询失败包装错误（不落库）。
func TestCreateUnboundContractGuards(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()
	service := NewContractService(db)

	// 空 functionId
	created, err := service.CreateUnboundContract(ctx, "g-u1", "e-u1", "openapi", unboundMaterialInput("  "))
	require.Error(t, err)
	assert.False(t, created)

	// schema 携带禁止的表现层字段 → rebuildContract 拒绝
	bad := unboundMaterialInput("player.bad")
	bad.InputSchema = `{"type":"object","x-menu":"Players"}`
	created, err = service.CreateUnboundContract(ctx, "g-u1", "e-u1", "openapi", bad)
	require.Error(t, err)
	assert.ErrorContains(t, err, "forbidden presentation field")
	assert.False(t, created)

	// find 查询失败（连接关闭）→ 包装错误
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	created, err = service.CreateUnboundContract(ctx, "g-u1", "e-u1", "openapi", unboundMaterialInput("player.x"))
	require.Error(t, err)
	assert.False(t, created)
}
