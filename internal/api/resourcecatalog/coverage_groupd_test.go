package resourcecatalog

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// semanticSourceDigest: contract JSON column bytes corrupted (written directly,
// bypassing dbtype.JSON.Value) -> json.Marshal goes through
// dbtype.JSON.MarshalJSON (RawMessage compact validation) and fails -> the digest
// falls back to an empty string (treated as no source descriptor).
func TestSemanticSourceDigestInvalidContractJSONColumn(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	seedV9Capability(t, db, "player")
	seedV9Contract(t, db, "player.list", dbenum.CapabilityCollectionQuery,
		`{"type":"object","properties":{"page":{"type":"integer"}}}`, `{"type":"object"}`)

	contractModel := model.NewFunctionContractModel(db)

	// Healthy data: the digest is a 64-character hex SHA-256.
	digest := semanticSourceDigest(ctx, contractModel, "g1", "e1", "player")
	require.NotEmpty(t, digest)
	assert.Len(t, digest, 64)

	// Corrupt the input_schema column bytes: Scan does not validate the raw text; the
	// error only surfaces at Marshal time.
	require.NoError(t, db.Exec(
		"UPDATE function_contracts SET input_schema = ? WHERE game_id = ? AND env = ? AND resource_key = ?",
		"{not-json", "g1", "e1", "player").Error)
	assert.Empty(t, semanticSourceDigest(ctx, contractModel, "g1", "e1", "player"))
}

// capabilitySemanticsJSON: invalid JSON bytes inside the semantics payload ->
// Marshal fails and nil is returned.
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
