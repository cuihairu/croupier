package function

// #26 sorting/dedup edges and the storage error path of the function history
// version index (the existing cases already cover scope validation, semver
// descending order, uninitialized DB, and the 30s TTL cache):
//  1) the dedupeVersions duplicate branch -- GROUP BY(function_id, version)
//     cannot catch equal values differing by whitespace ('1.0.0' and ' 1.0.0'
//     are two rows); they must converge after TrimSpace;
//  2) the "both unparseable" sort branch -- after parseable versions sort
//     first, the remaining versions order lexicographically;
//  3) a ListDistinctVersions storage error (missing table) -> a unified error
//     object instead of a silent empty set.

import (
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Equal versions differing by whitespace converge into one option.
func TestHandler_VersionHistoryIndex_DedupesWhitespaceVariants(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	const gameID = "vh-dedupe"
	svcCtx := setupTestServiceContext(t)
	seedContractVersion(t, svcCtx, gameID, "a.fn", "1.0.0")
	// Whitespace variant of the same version: two distinct rows at the SQL
	// layer; the handler must dedupe them
	seedContractVersion(t, svcCtx, gameID, "a.fn", " 1.0.0")

	h := NewHandler(NewService(svcCtx))
	ctx, rec := newVersionIndexRequest(gameID)
	h.VersionHistoryIndex(ctx)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	items := decodeVersionIndex(t, rec.Body.Bytes())
	require.Len(t, items, 1)
	assert.Equal(t, []string{"1.0.0"}, items[0].Versions)
}

// Unparseable versions sort after parseable ones, lexicographically (this
// branch only triggers when both are unparseable).
func TestHandler_VersionHistoryIndex_UnparseableSortedLexicographically(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	const gameID = "vh-unparseable"
	svcCtx := setupTestServiceContext(t)
	seedContractVersion(t, svcCtx, gameID, "a.fn", "nightly-x")
	seedContractVersion(t, svcCtx, gameID, "a.fn", "alpha")
	seedContractVersion(t, svcCtx, gameID, "a.fn", "1.0.0")

	h := NewHandler(NewService(svcCtx))
	ctx, rec := newVersionIndexRequest(gameID)
	h.VersionHistoryIndex(ctx)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	items := decodeVersionIndex(t, rec.Body.Bytes())
	require.Len(t, items, 1)
	assert.Equal(t, []string{"1.0.0", "alpha", "nightly-x"}, items[0].Versions)
}

// A missing version table (migrations not run) -> 500; it must not silently
// return an empty dropdown (an empty dropdown would be misread as "no history
// versions").
func TestHandler_VersionHistoryIndex_StoreError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := setupTestServiceContext(t)
	require.NoError(t, svcCtx.DB.Migrator().DropTable(&model.FunctionContractVersion{}))

	h := NewHandler(NewService(svcCtx))
	ctx, rec := newVersionIndexRequest("vh-store-err")
	h.VersionHistoryIndex(ctx)

	assert.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"error"`)
}
