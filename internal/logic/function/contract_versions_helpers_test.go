package function

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

// 读端纯函数：字段级 diff / 快照解码 / 空 schema 判定 / 列表条目裁剪。

func TestDiffFunctionSpecs_FieldLevel(t *testing.T) {
	oldSpec := &spec.FunctionSpec{
		ID:           "player.ban",
		Version:      "1.0.0",
		Enabled:      true,
		Resource:     "player",
		Operation:    "ban",
		Capability:   "action",
		Execution:    "sync",
		Risk:         "safe",
		Permission:   "player:ban",
		TimeoutMs:    1000,
		InputSchema:  spec.JSONSchema(`{"type":"object"}`),
		OutputSchema: spec.JSONSchema(`{"type":"object"}`),
		Summary:      spec.LocalizedText{"zh-CN": "封禁"},
		Tags:         []string{"a"},
	}
	newSpec := *oldSpec
	newSpec.Version = "1.1.0"
	newSpec.Enabled = false
	newSpec.Permission = ""
	newSpec.TimeoutMs = 2000
	newSpec.Risk = "danger"
	newSpec.InputSchema = spec.JSONSchema(`{"type":"string"}`)

	entries := diffFunctionSpecs(oldSpec, &newSpec)
	fields := map[string]ContractVersionDiffEntry{}
	for _, e := range entries {
		fields[e.Field] = e
	}
	require.Contains(t, fields, "version")
	assert.Equal(t, "1.0.0", fields["version"].From)
	assert.Equal(t, "1.1.0", fields["version"].To)
	require.Contains(t, fields, "enabled")
	assert.Equal(t, "true", fields["enabled"].From)
	assert.Equal(t, "false", fields["enabled"].To)
	require.Contains(t, fields, "permission")
	assert.Equal(t, "player:ban", fields["permission"].From)
	assert.Equal(t, "", fields["permission"].To)
	require.Contains(t, fields, "timeoutMs")
	require.Contains(t, fields, "risk")
	assert.Equal(t, "safe", fields["risk"].From)
	assert.Equal(t, "danger", fields["risk"].To)
	require.Contains(t, fields, "inputSchema")
	assert.Equal(t, "schema_replaced", fields["inputSchema"].Change)

	// 相同内容 → 空 diff。
	same := *oldSpec
	assert.Empty(t, diffFunctionSpecs(oldSpec, &same))
}

func TestDiffFunctionSpecs_BothEmptySchemasSkip(t *testing.T) {
	oldSpec := &spec.FunctionSpec{ID: "x", InputSchema: nil}
	newSpec := &spec.FunctionSpec{ID: "x"}
	// nil 与空对象都算 empty → 不出 inputSchema entry。
	for _, e := range diffFunctionSpecs(oldSpec, newSpec) {
		assert.NotEqual(t, "inputSchema", e.Field)
	}
}

func TestIsEmptySchemaJSON(t *testing.T) {
	assert.True(t, isEmptySchemaJSON(nil))
	assert.True(t, isEmptySchemaJSON(json.RawMessage("")))
	assert.True(t, isEmptySchemaJSON(json.RawMessage("  ")))
	assert.True(t, isEmptySchemaJSON(json.RawMessage("null")))
	assert.True(t, isEmptySchemaJSON(json.RawMessage("{}")))
	assert.False(t, isEmptySchemaJSON(json.RawMessage(`{"type":"object"}`)))
	assert.False(t, isEmptySchemaJSON(json.RawMessage(`[]`)))
}

func TestDecodeSnapshotSpec(t *testing.T) {
	// 空快照 → 错误。
	_, err := decodeSnapshotSpec(&model.FunctionContractVersion{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "缺失")

	// 坏 JSON → 解析失败。
	_, err = decodeSnapshotSpec(&model.FunctionContractVersion{Snapshot: model.JSON(`{broken`)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "解析失败")

	// 合法快照。
	row := &model.FunctionContractVersion{Snapshot: model.JSON(`{"id":"player.ban","version":"1.0.0"}`)}
	fn, err := decodeSnapshotSpec(row)
	require.NoError(t, err)
	assert.Equal(t, "player.ban", fn.ID)
	assert.Equal(t, "1.0.0", fn.Version)
}

func TestToContractVersionItem_TrimsDiffAndFormatsTime(t *testing.T) {
	created := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)
	row := &model.FunctionContractVersion{
		Seq:        4,
		Version:    "2.0.0",
		Source:     "sdk",
		ChangeType: "updated",
		Breaking:   true,
		Actor:      "alice",
		Model:      gorm.Model{CreatedAt: created},
		Diff:       model.JSON(`[{"field":"version","from":"1.0.0","to":"2.0.0"}]`),
	}

	item := toContractVersionItem(row)
	assert.EqualValues(t, 4, item.Seq)
	assert.Equal(t, "2.0.0", item.Version)
	assert.Equal(t, "updated", item.ChangeType)
	assert.True(t, item.Breaking)
	assert.Equal(t, "alice", item.Actor)
	assert.Equal(t, "2026-05-06T07:08:09Z", item.CreatedAt)
	require.Len(t, item.Diff, 1)
	assert.Equal(t, "version", item.Diff[0].Field)

	// Diff 坏 JSON → 静默跳过（不阻断列表）。
	row.Diff = model.JSON(`{broken`)
	item = toContractVersionItem(row)
	assert.Empty(t, item.Diff)

	// 无 Diff。
	row.Diff = nil
	item = toContractVersionItem(row)
	assert.Empty(t, item.Diff)
}
