package function

import (
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// R53: closes out the two degraded wings of the ContractVersions lazy
// backfill surface (landed in 9c50e3d).
//
// (1) handler.go:35-36 backfill write fails -> slog.Warn degradation, the
//   response is untouched (source-comment contract: "history is derived audit
//   data; a backfill failure degrades to a log line and does not affect the
//   list response");
// (2) handler.go:39-41 the re-query after a successful backfill
//   (created=true) fails -> the error is passed through.
//
// Injection technique (established in this repo): this package's setupTestDB
// gives every case its own `:memory:` database (not a shared singleton), so
// sqlite trigger injection is inherently isolated per case with no
// cross-case poisoning surface — unlike packages that share a
// cache=shared database and need a unique scope for isolation.
// (1) write block: BEFORE INSERT TRIGGER + RAISE(ABORT) (same as the svc C
// batch);
// (2) read blow-up: an AFTER INSERT TRIGGER appends a poison row whose seq
//   is non-numeric TEXT — the backfill INSERT itself succeeds (created=true),
//   and the List Find that follows immediately scanning that TEXT into an
//   int64 column yields a scan error, producing the "write succeeds -> the
//   very next read fails" in-request read-failure shape (the RAISE family
//   cannot reach the read side; the poison row is its read-side equivalent).

func TestHandler_ContractVersions_BackfillWriteFailureDegrades(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := setupTestServiceContext(t)
	seedContractWithoutHistory(t, svcCtx)
	require.NoError(t, svcCtx.DB.Exec(
		`CREATE TRIGGER r53_block_version_insert BEFORE INSERT ON function_contract_versions `+
			`BEGIN SELECT RAISE(ABORT, 'r53 injected write failure'); END;`).Error)

	// The backfill write is blocked -> degraded to a warn only; the response
	// is still the empty list from the first query (200, not 500).
	total, items := listVersionsViaHandler(t, NewHandler(NewService(svcCtx)), "inventory.consume")
	require.EqualValues(t, 0, total)
	require.Empty(t, items)
}

func TestHandler_ContractVersions_RequeryFailureAfterBackfill(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := setupTestServiceContext(t)
	seedContractWithoutHistory(t, svcCtx)
	// After the backfill INSERT succeeds, the trigger appends a poison row
	// with seq='x': Count is unaffected, and Find scanning TEXT 'x' into the
	// int64 Seq yields a scan error -> the re-query error passes through as 500.
	require.NoError(t, svcCtx.DB.Exec(
		`CREATE TRIGGER r53_poison_after_insert AFTER INSERT ON function_contract_versions `+
			`BEGIN INSERT INTO function_contract_versions `+
			`(game_id, env, function_id, seq, version, change_type, actor, snapshot, created_at, updated_at) `+
			`VALUES ('g', 'e', 'inventory.consume', 'x', '0.0.0', 'created', 'r53-poison', '{}', `+
			`datetime('now'), datetime('now')); END;`).Error)

	h := NewHandler(NewService(svcCtx))
	ctx, rec := newFunctionTestContext(http.MethodGet,
		"/api/v1/functions/inventory.consume/contract-versions?page=1&pageSize=10", "")
	ctx.Params = gin.Params{{Key: "id", Value: "inventory.consume"}}
	ctx.Request = ctx.Request.WithContext(
		svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: "g", Env: "e"}))
	h.ContractVersions(ctx)

	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
}
