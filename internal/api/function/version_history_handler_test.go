package function

// #26 函数历史版本索引（GET /api/v1/functions/version-history）handler 单测：
// scope 校验、semver 降序聚合（数值序非字典序、不可解析版本殿后、空白版本
// 剔除、scope 隔离）、DB 未初始化降级、30s TTL 缓存命中语义。
// 版本行经 AppendVersion 播种（与生产注册链同一 seq 分配逻辑）。

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
	// b 函数排后（函数按字典序）；a 函数版本混插，含不可解析与纯空白
	seedContractVersion(t, svcCtx, gameID, "b.fn", "1.0.0")
	seedContractVersion(t, svcCtx, gameID, "a.fn", "1.0.0")
	seedContractVersion(t, svcCtx, gameID, "a.fn", "1.9.0")
	seedContractVersion(t, svcCtx, gameID, "a.fn", "1.10.0")
	seedContractVersion(t, svcCtx, gameID, "a.fn", "nightly-x")
	// 纯空白版本：SQL 过滤不掉（version <> ''），由 handler TrimSpace 剔除
	seedContractVersion(t, svcCtx, gameID, "a.fn", " ")
	// 其他 scope 的行不得混入
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
	// semver 数值降序：1.10.0 > 1.9.0（字典序会把 1.9.0 排前）；不可解析殿后
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

	// 缓存窗口（30s TTL）内新增的版本不改变响应——陈旧可接受（见实现注释）
	seedContractVersion(t, svcCtx, gameID, "a.fn", "2.0.0")
	ctx2, rec2 := newVersionIndexRequest(gameID)
	h.VersionHistoryIndex(ctx2)
	items := decodeVersionIndex(t, rec2.Body.Bytes())
	if len(items) != 1 || len(items[0].Versions) != 1 {
		t.Fatalf("expected cached stale payload, got %+v", items)
	}

	// 绕过缓存直查则能看到新版本（证明陈旧来自缓存而非查询缺陷）
	fresh, err := loadVersionIndex(context.Background(), svcCtx, gameID, "e")
	if err != nil {
		t.Fatalf("loadVersionIndex: %v", err)
	}
	if len(fresh) != 1 || len(fresh[0].Versions) != 2 {
		t.Fatalf("expected 2 versions uncached, got %+v", fresh)
	}
}
