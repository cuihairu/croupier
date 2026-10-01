package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/model"
)

// R53：BackfillInitialContractVersion（9c50e3d 落地的存量契约惰性回填）
// 四处错误翼收口——上游 contract_versions_backfill_test.go 只盖了主链与
// ErrRecordNotFound 跳过翼（ghost.function），本文件补齐：
//  1. :201-203 nil/零值 service 守卫（回填是列表路径衍生的 best-effort，
//     零值接收者静默跳过而非 panic）；
//  2. :205-207 版本历史 ListByFunctionPaged 错误包装翼；
//  3. :215-217 契约查找非 NotFound 错误包装翼（缺表注入：版本表完好且
//     为空 → total=0 通过，function_contracts 缺表 → First 报错而非
//     ErrRecordNotFound，精确停在目标分支）；
//  4. :219-221 快照 Marshal 失败包装翼——库里直写校验层造不出的形态：
//     db.Create 绕过 UpsertContract 直塞非法 JSON 到 input_schema
//     （dbtype.JSON.Value 原样透传、sqlite 列不校验），而
//     normalizeJSONSchema（contract_projection.go:36）对非引号开头
//     原文透传，json.Marshal 对含非法 RawMessage 的结构体在 compact
//     阶段报错——存量坏行不炸不吞、错误带函数名透传。
//
// 注入口径（仓内既有技法）：本包 setupTestDB 为每用例独立 `:memory:`
// 库，DropTable 缺表注入（contract_coverage_v9_test.go:521 同款）天然
// 按用例隔离；scope 仍取 "g-r53" 唯一值（派发要求的隔离卫生）。

func TestBackfillInitialContractVersion_NilServiceGuard(t *testing.T) {
	var s *ContractService
	created, err := s.BackfillInitialContractVersion(context.Background(), "g-r53", "dev", "player.ban")
	require.NoError(t, err)
	require.False(t, created)
}

func TestBackfillInitialContractVersion_ListErrorWing(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.Migrator().DropTable(&model.FunctionContractVersion{}))
	s := NewContractService(db)

	created, err := s.BackfillInitialContractVersion(context.Background(), "g-r53", "dev", "player.ban")
	require.ErrorContains(t, err, "list contract versions player.ban")
	require.False(t, created)
}

func TestBackfillInitialContractVersion_FindErrorWing(t *testing.T) {
	// 版本表完好且为空（total=0 通过）→ 契约表缺表 → First 报错，
	// 与 ErrRecordNotFound 跳过翼（既有 ghost 用例）分叉在本分支。
	db := setupTestDB(t)
	require.NoError(t, db.Migrator().DropTable(&model.FunctionContract{}))
	s := NewContractService(db)

	created, err := s.BackfillInitialContractVersion(context.Background(), "g-r53", "dev", "player.ban")
	require.ErrorContains(t, err, "find function contract player.ban")
	require.False(t, created)
}

func TestBackfillInitialContractVersion_MarshalErrorWing(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.Create(&model.FunctionContract{
		GameID:      "g-r53",
		Env:         "dev",
		FunctionID:  "player.ban",
		Version:     "1.0.0",
		Enabled:     true,
		InputSchema: model.JSON(`{not-json`),
	}).Error)
	s := NewContractService(db)

	created, err := s.BackfillInitialContractVersion(context.Background(), "g-r53", "dev", "player.ban")
	require.ErrorContains(t, err, "marshal contract snapshot player.ban")
	require.False(t, created)
}
