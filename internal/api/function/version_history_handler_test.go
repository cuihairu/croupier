package function

// #26 handler unit tests for the function history version index
// (GET /api/v1/functions/version-history): scope validation, semver descending
// aggregation (numeric order rather than lexicographic, unparseable versions
// last, blank versions stripped, scope isolation), degraded behavior with an
// uninitialized DB, and the 30s TTL cache hit semantics.
// Version rows are seeded through AppendVersion (the same seq allocation logic
// as the production registration chain).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

func seedContractVersion(t *testing.T, svcCtx *svc.ServiceContext, gameID, functionID, version string) {
	t.Helper()
	row := &model.FunctionContractVersion{
		GameID:     gameID,
		Env:        "e",
		FunctionID: functionID,
		Version:    version,
		Source:     "sdk",
		ChangeType: "updated",
		Snapshot:   contractVersionSnapshot(t, version, `{"type":"object"}`),
	}
	if err := model.NewFunctionContractVersionModel(svcCtx.DB).AppendVersion(context.Background(), row); err != nil {
		t.Fatalf("seed %s %s: %v", functionID, version, err)
	}
}

func newVersionIndexRequest(gameID string) (*gin.Context, *httptest.ResponseRecorder) {
	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/version-history", "")
	if gameID != "" {
		ctx.Request = ctx.Request.WithContext(
			svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: gameID, Env: "e"}))
	}
	return ctx, rec
}

func decodeVersionIndex(t *testing.T, body []byte) []functionVersionIndexItem {
	t.Helper()
	var resp functionVersionIndexResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, string(body))
	}
	return resp.Items
}

func TestHandler_VersionHistoryIndex_ListsSemverDesc(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	const gameID = "vh-sort"
	svcCtx := setupTestServiceContext(t)
	// The b function sorts later (functions are lexicographic); the a
	// function's versions are interleaved, including an unparseable and a
	// blank one
	seedContractVersion(t, svcCtx, gameID, "b.fn", "1.0.0")
	seedContractVersion(t, svcCtx, gameID, "a.fn", "1.0.0")
	seedContractVersion(t, svcCtx, gameID, "a.fn", "1.9.0")
	seedContractVersion(t, svcCtx, gameID, "a.fn", "1.10.0")
	seedContractVersion(t, svcCtx, gameID, "a.fn", "nightly-x")
	// Pure-whitespace version: not filtered out by SQL (version <> ''),
	// stripped by the handler's TrimSpace
	seedContractVersion(t, svcCtx, gameID, "a.fn", " ")
	// Rows from other scopes must not leak in
	seedContractVersion(t, svcCtx, "vh-other", "a.fn", "9.9.9")

	h := NewHandler(NewService(svcCtx))
	ctx, rec := newVersionIndexRequest(gameID)
	h.VersionHistoryIndex(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	items := decodeVersionIndex(t, rec.Body.Bytes())
	if len(items) != 2 {
		t.Fatalf("expected 2 functions, got %+v", items)
	}
	if items[0].FunctionID != "a.fn" || items[1].FunctionID != "b.fn" {
		t.Fatalf("expected lexicographic function order, got %v, %v", items[0].FunctionID, items[1].FunctionID)
	}
	// semver numeric descending: 1.10.0 > 1.9.0 (lexicographic order would put
	// 1.9.0 first); unparseable versions go last
	want := []string{"1.10.0", "1.9.0", "1.0.0", "nightly-x"}
	if len(items[0].Versions) != len(want) {
		t.Fatalf("expected versions %v, got %v", want, items[0].Versions)
	}
	for i, v := range want {
		if items[0].Versions[i] != v {
			t.Fatalf("version[%d] = %q, want %q (full %v)", i, items[0].Versions[i], v, items[0].Versions)
		}
	}
}

func TestHandler_VersionHistoryIndex_MissingScope(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newVersionIndexRequest("")

	h.VersionHistoryIndex(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_VersionHistoryIndex_DBNotInitialized(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newVersionIndexRequest("vh-nodb")

	h.VersionHistoryIndex(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with empty index, got %d body=%s", rec.Code, rec.Body.String())
	}
	if items := decodeVersionIndex(t, rec.Body.Bytes()); len(items) != 0 {
		t.Fatalf("expected empty index, got %+v", items)
	}
}

func TestHandler_VersionHistoryIndex_TTLCache(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	const gameID = "vh-cache"
	svcCtx := setupTestServiceContext(t)
	seedContractVersion(t, svcCtx, gameID, "a.fn", "1.0.0")
	h := NewHandler(NewService(svcCtx))

	ctx, rec := newVersionIndexRequest(gameID)
	h.VersionHistoryIndex(ctx)
	if items := decodeVersionIndex(t, rec.Body.Bytes()); len(items) != 1 || len(items[0].Versions) != 1 {
		t.Fatalf("first call expected single version, got %+v", items)
	}

	// A version appended inside the cache window (30s TTL) does not change the
	// response -- staleness is acceptable (see the implementation comment)
	seedContractVersion(t, svcCtx, gameID, "a.fn", "2.0.0")
	ctx2, rec2 := newVersionIndexRequest(gameID)
	h.VersionHistoryIndex(ctx2)
	items := decodeVersionIndex(t, rec2.Body.Bytes())
	if len(items) != 1 || len(items[0].Versions) != 1 {
		t.Fatalf("expected cached stale payload, got %+v", items)
	}

	// A query that bypasses the cache does see the new version (proving the
	// staleness comes from the cache, not a query defect)
	fresh, err := loadVersionIndex(context.Background(), svcCtx, gameID, "e")
	if err != nil {
		t.Fatalf("loadVersionIndex: %v", err)
	}
	if len(fresh) != 1 || len(fresh[0].Versions) != 2 {
		t.Fatalf("expected 2 versions uncached, got %+v", fresh)
	}
}
