package function

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupVersionsLogic(t *testing.T) (*ContractVersionsLogic, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	svcCtx := &svc.ServiceContext{DB: db}
	return NewContractVersionsLogic(context.Background(), svcCtx), db
}

func seedVersions(t *testing.T, db *gorm.DB) {
	t.Helper()
	ctx := context.Background()
	contractSvc := contractsvc.NewContractService(db)
	base := spec.FunctionContractInput{
		ID:           "player.ban",
		Version:      "1.0.0",
		Enabled:      true,
		Resource:     "player",
		Operation:    "ban",
		Capability:   string(spec.CapabilityAction),
		Execution:    string(spec.FunctionExecutionSync),
		Risk:         string(spec.RiskSafe),
		OutputSchema: `{"type":"object"}`,
		InputSchema:  `{"type":"object","properties":{"playerId":{"type":"string"},"reason":{"type":"string"}},"required":["playerId","reason"]}`,
	}
	require.NoError(t, contractSvc.RebuildContractFromFunctionMeta(ctx, "g", "e", "sdk", base))

	// 抬升 risk 并删除一个 required 字段（created→updated）
	base.Version = "1.1.0"
	base.Risk = string(spec.RiskDanger)
	base.InputSchema = `{"type":"object","properties":{"playerId":{"type":"string"}},"required":["playerId"]}`
	require.NoError(t, contractSvc.RebuildContractFromFunctionMeta(ctx, "g", "e", "sdk", base))

	// 再补一条（updated）
	base.Version = "1.2.0"
	require.NoError(t, contractSvc.RebuildContractFromFunctionMeta(ctx, "g", "e", "sdk", base))
}

func TestContractVersionsLogic_List(t *testing.T) {
	logic, db := setupVersionsLogic(t)
	seedVersions(t, db)

	resp, err := logic.List("g", "e", "player.ban", 1, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 3, resp.Total)
	require.Len(t, resp.Items, 3)
	// 最新在前
	assert.EqualValues(t, 3, resp.Items[0].Seq)
	assert.Equal(t, "updated", resp.Items[0].ChangeType)
	assert.Equal(t, "created", resp.Items[2].ChangeType)

	// wire key 断言（lowerCamelCase 契约）
	raw, err := json.Marshal(resp.Items[1])
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &m))
	assert.Equal(t, float64(2), m["seq"])
	assert.Contains(t, m, "createdAt")
	assert.Contains(t, m, "changeType")
}

func TestContractVersionsLogic_ListPaging(t *testing.T) {
	logic, db := setupVersionsLogic(t)
	seedVersions(t, db)

	resp, err := logic.List("g", "e", "player.ban", 2, 2)
	require.NoError(t, err)
	assert.EqualValues(t, 3, resp.Total)
	require.Len(t, resp.Items, 1)
	assert.EqualValues(t, 1, resp.Items[0].Seq) // page=2, size=2 → seq1
}

func TestContractVersionsLogic_DetailReturnsSnapshot(t *testing.T) {
	logic, db := setupVersionsLogic(t)
	seedVersions(t, db)

	detail, err := logic.Detail("g", "e", "player.ban", 1)
	require.NoError(t, err)
	assert.Equal(t, "created", detail.ChangeType)
	require.NotEmpty(t, detail.Snapshot)
	var snapshot spec.FunctionSpec
	require.NoError(t, json.Unmarshal(detail.Snapshot, &snapshot))
	assert.Equal(t, "1.0.0", snapshot.Version)
	assert.Equal(t, spec.RiskSafe, snapshot.Risk)
}

func TestContractVersionsLogic_DiffTwoVersions(t *testing.T) {
	logic, db := setupVersionsLogic(t)
	seedVersions(t, db)

	// seq1(v1.0.0,safe) → seq2(v1.1.0,danger, required 减一 + 删 reason 字段)
	diff, err := logic.Diff("g", "e", "player.ban", 1, 2)
	require.NoError(t, err)
	assert.EqualValues(t, 1, diff.FromSeq)
	assert.EqualValues(t, 2, diff.ToSeq)
	assert.True(t, diff.Breaking, "删除已声明字段应为破坏性")
	byField := map[string]ContractVersionDiffEntry{}
	for _, c := range diff.Changes {
		byField[c.Field] = c
	}
	require.Contains(t, byField, "risk")
	assert.Equal(t, "safe", byField["risk"].From)
	assert.Equal(t, "danger", byField["risk"].To)
	require.Contains(t, byField, "inputSchema")
	assert.Equal(t, "schema_replaced", byField["inputSchema"].Change)
	assert.NotEmpty(t, byField["inputSchema"].Findings)
}

func TestContractVersionsLogic_MissingScopeIsolation(t *testing.T) {
	logic, db := setupVersionsLogic(t)
	seedVersions(t, db)
	// 另一 scope 查不到
	resp, err := logic.List("other", "e", "player.ban", 1, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 0, resp.Total)
}
