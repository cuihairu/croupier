package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/function/schemadiff"
	"github.com/cuihairu/croupier/internal/model"
)

// B2：函数契约变更历史写路径回归。

func historyInput(version string, risk spec.RiskLevel, inputSchema string) spec.FunctionContractInput {
	return spec.FunctionContractInput{
		ID:           "player.ban",
		Version:      version,
		Enabled:      true,
		InputSchema:  inputSchema,
		OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
		Resource:     "player",
		Operation:    "ban",
		Capability:   string(spec.CapabilityAction),
		Execution:    string(spec.FunctionExecutionSync),
		Risk:         string(risk),
	}
}

func listHistory(t *testing.T, db *model.FunctionContractVersionModel, ctx context.Context) ([]*model.FunctionContractVersion, int64) {
	t.Helper()
	vers, total, err := db.ListByFunctionPaged(ctx, "g-hist", "dev", "player.ban", 100, 0)
	require.NoError(t, err)
	return vers, total
}

func TestContractVersionHistory_Lifecycle(t *testing.T) {
	db := setupTestDB(t)
	service := NewContractService(db)
	versions := model.NewFunctionContractVersionModel(db)
	ctx := context.Background()

	schemaV1 := `{"type":"object","properties":{"playerId":{"type":"string"},"reason":{"type":"string"}},"required":["playerId"]}`

	// 1) 首次注册 → created 一条，快照可反解为 FunctionSpec
	require.NoError(t, service.RebuildContractFromFunctionMeta(ctx, "g-hist", "dev", "sdk", historyInput("1.0.0", spec.RiskWarning, schemaV1)))
	vers, total := listHistory(t, versions, ctx)
	require.EqualValues(t, 1, total)
	assert.Equal(t, ContractVersionChangeCreated, vers[0].ChangeType)
	assert.EqualValues(t, 1, vers[0].Seq)
	assert.False(t, vers[0].Breaking)
	assert.Empty(t, vers[0].Snapshot, "列表载荷瘦身")
	first, err := versions.FindBySeq(ctx, "g-hist", "dev", "player.ban", 1)
	require.NoError(t, err)
	var snap spec.FunctionSpec
	require.NoError(t, json.Unmarshal(first.Snapshot, &snap))
	assert.Equal(t, "player.ban", snap.ID)
	assert.Equal(t, "1.0.0", snap.Version)

	// 2) 相同内容重注册（字节形态不同、语义相同）→ 不产生历史（与跳过写同判据）
	reformatted := ` {  "type": "object", "properties": { "reason": {"type":"string"}, "playerId": {"type":"string"} }, "required": ["playerId"] }`
	require.NoError(t, service.RebuildContractFromFunctionMeta(ctx, "g-hist", "dev", "sdk", historyInput("1.0.0", spec.RiskWarning, reformatted)))
	_, total = listHistory(t, versions, ctx)
	assert.EqualValues(t, 1, total)

	// 3) risk 抬升 + required 字段删除 → updated + breaking + diff 双字段
	schemaV2 := `{"type":"object","properties":{"playerId":{"type":"string"}},"required":[]}`
	require.NoError(t, service.RebuildContractFromFunctionMeta(ctx, "g-hist", "dev", "sdk", historyInput("1.1.0", spec.RiskDanger, schemaV2)))
	vers, total = listHistory(t, versions, ctx)
	require.EqualValues(t, 2, total)
	updated := vers[0] // 最新在前
	assert.Equal(t, ContractVersionChangeUpdated, updated.ChangeType)
	assert.EqualValues(t, 2, updated.Seq)
	assert.True(t, updated.Breaking)
	var entries []contractFieldChange
	require.NoError(t, json.Unmarshal(updated.Diff, &entries))
	fields := map[string]contractFieldChange{}
	for _, entry := range entries {
		fields[entry.Field] = entry
	}
	require.Contains(t, fields, "risk")
	assert.Equal(t, "warning", fields["risk"].From)
	assert.Equal(t, "danger", fields["risk"].To)
	require.Contains(t, fields, "inputSchema")
	assert.Equal(t, "schema_replaced", fields["inputSchema"].Change)
	breakingFound := false
	for _, finding := range fields["inputSchema"].Findings {
		if finding.Severity == schemadiff.SeverityBreaking {
			breakingFound = true
		}
	}
	assert.True(t, breakingFound, "required 删除必须产 breaking finding 并入 diff")

	// 4) 删除 → removed 一条，快照保留删除前最后一版
	resourceKey, err := service.RemoveFunctionContract(ctx, "g-hist", "dev", "player.ban")
	require.NoError(t, err)
	assert.Equal(t, "player", resourceKey)
	vers, total = listHistory(t, versions, ctx)
	require.EqualValues(t, 3, total)
	assert.Equal(t, ContractVersionChangeRemoved, vers[0].ChangeType)
	removed, err := versions.FindBySeq(ctx, "g-hist", "dev", "player.ban", vers[0].Seq)
	require.NoError(t, err)
	var removedSnap spec.FunctionSpec
	require.NoError(t, json.Unmarshal(removed.Snapshot, &removedSnap))
	assert.Equal(t, "1.1.0", removedSnap.Version)
}

func TestContractVersionHistory_RetentionTrim(t *testing.T) {
	db := setupTestDB(t)
	service := NewContractService(db)
	versions := model.NewFunctionContractVersionModel(db)
	ctx := context.Background()

	for i := 0; i < model.FunctionContractVersionRetention+10; i++ {
		require.NoError(t, service.RebuildContractFromFunctionMeta(ctx, "g-hist", "dev", "sdk",
			historyInput(fmt.Sprintf("1.0.%d", i), spec.RiskSafe, `{"type":"object"}`)))
	}
	vers, total := listHistory(t, versions, ctx)
	assert.EqualValues(t, model.FunctionContractVersionRetention, total)
	require.NotEmpty(t, vers)
	assert.EqualValues(t, model.FunctionContractVersionRetention+10, vers[0].Seq, "最新一条保留")
	assert.EqualValues(t, 11, vers[len(vers)-1].Seq, "第 10 条之前的最老历史被淘汰")
}

func TestContractVersionHistory_ListStripsSnapshot(t *testing.T) {
	db := setupTestDB(t)
	service := NewContractService(db)
	versions := model.NewFunctionContractVersionModel(db)
	ctx := context.Background()

	require.NoError(t, service.RebuildContractFromFunctionMeta(ctx, "g-hist", "dev", "sdk",
		historyInput("1.0.0", spec.RiskSafe, `{"type":"object"}`)))
	vers, _ := listHistory(t, versions, ctx)
	require.Len(t, vers, 1)
	assert.Empty(t, vers[0].Snapshot, "列表载荷瘦身：不带快照")

	row, err := versions.FindBySeq(ctx, "g-hist", "dev", "player.ban", 1)
	require.NoError(t, err)
	assert.NotEmpty(t, row.Snapshot, "单条查询保留完整快照")
}
