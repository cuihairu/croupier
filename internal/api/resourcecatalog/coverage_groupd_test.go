package resourcecatalog

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// semanticSourceDigest：契约 JSON 列字节损坏（绕过 dbtype.JSON.Value 直写）
// → json.Marshal 经 dbtype.JSON.MarshalJSON（RawMessage compact 校验）失败
// → 摘要回落空串（视为无源描述符）。
func TestSemanticSourceDigestInvalidContractJSONColumn(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	seedV9Capability(t, db, "player")
	seedV9Contract(t, db, "player.list", dbenum.CapabilityCollectionQuery,
		`{"type":"object","properties":{"page":{"type":"integer"}}}`, `{"type":"object"}`)

	contractModel := model.NewFunctionContractModel(db)

	// 正常数据：摘要为 64 位十六进制 SHA-256。
	digest := semanticSourceDigest(ctx, contractModel, "g1", "e1", "player")
	require.NotEmpty(t, digest)
	assert.Len(t, digest, 64)

	// 破坏 input_schema 列字节：Scan 不校验原文，Marshal 时才报错。
	require.NoError(t, db.Exec(
		"UPDATE function_contracts SET input_schema = ? WHERE game_id = ? AND env = ? AND resource_key = ?",
		"{not-json", "g1", "e1", "player").Error)
	assert.Empty(t, semanticSourceDigest(ctx, contractModel, "g1", "e1", "player"))
}

// capabilitySemanticsJSON：语义负载含无效 JSON 字节 → Marshal 失败返回 nil。
func TestCapabilitySemanticsJSONInvalidPayloadBytes(t *testing.T) {
	valid := capabilitySemanticsJSON(&model.CapabilitySemantics{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Actions: model.JSON(`[{"functionId":"player.grant"}]`),
	})
	require.NotNil(t, valid)

	invalid := capabilitySemanticsJSON(&model.CapabilitySemantics{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Reports: model.JSON(`{not-json`),
	})
	assert.Nil(t, invalid)
}
