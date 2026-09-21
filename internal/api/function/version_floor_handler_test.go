package function

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
)

// 函数级最低 SDK 版本门槛 REST 面：GET/PUT/DELETE + scope/参数校验。
// 测试上下文无认证信息（匿名），准入面与 descriptors 读路径一致放行。

func withFloorScope(ctx *gin.Context) {
	ctx.Request = ctx.Request.WithContext(
		svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: "g", Env: "e"}))
}

func TestHandler_VersionFloor_GetEmpty(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/player.ban/version-floor", "")
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	withFloorScope(ctx)

	h.VersionFloorGet(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp versionFloorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}
	if resp.FunctionID != "player.ban" || resp.MinVersion != "" {
		t.Fatalf("expected empty floor echo, got %+v", resp)
	}
}

func TestHandler_VersionFloor_PutThenGetThenDelete(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))

	put, putRec := newFunctionTestContext(http.MethodPut, "/api/v1/functions/player.ban/version-floor", `{"minVersion":"0.3.0"}`)
	put.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	withFloorScope(put)
	h.VersionFloorPut(put)
	if putRec.Code != http.StatusOK {
		t.Fatalf("put expected 200, got %d body=%s", putRec.Code, putRec.Body.String())
	}

	get, getRec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/player.ban/version-floor", "")
	get.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	withFloorScope(get)
	h.VersionFloorGet(get)
	var resp versionFloorResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.MinVersion != "0.3.0" {
		t.Fatalf("expected floor 0.3.0, got %q", resp.MinVersion)
	}

	del, delRec := newFunctionTestContext(http.MethodDelete, "/api/v1/functions/player.ban/version-floor", "")
	del.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	withFloorScope(del)
	h.VersionFloorDelete(del)
	if delRec.Code != http.StatusOK {
		t.Fatalf("delete expected 200, got %d body=%s", delRec.Code, delRec.Body.String())
	}

	get2, get2Rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/player.ban/version-floor", "")
	get2.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	withFloorScope(get2)
	h.VersionFloorGet(get2)
	var resp2 versionFloorResponse
	if err := json.Unmarshal(get2Rec.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp2.MinVersion != "" {
		t.Fatalf("expected cleared floor, got %q", resp2.MinVersion)
	}
}

func TestHandler_VersionFloor_PutUnparseableRejected(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newFunctionTestContext(http.MethodPut, "/api/v1/functions/player.ban/version-floor", `{"minVersion":"unknown"}`)
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	withFloorScope(ctx)

	h.VersionFloorPut(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "parseable") {
		t.Fatalf("expected parseable error, got %s", rec.Body.String())
	}
}

func TestHandler_VersionFloor_MissingScope(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/player.ban/version-floor", "")
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}

	h.VersionFloorGet(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "X-Game-ID") {
		t.Fatalf("expected scope error, got %s", rec.Body.String())
	}
}

// ---- 批量面：GET /version-floors + POST /version-floor/batch ----

func TestHandler_VersionFloors_ListEmpty(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/version-floors", "")
	withFloorScope(ctx)

	h.VersionFloorsList(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp versionFloorsListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}
	if len(resp.Floors) != 0 {
		t.Fatalf("expected empty floors, got %v", resp.Floors)
	}
}

func TestHandler_VersionFloors_ListAfterSet(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	store := h.service.SvcCtx().RegistryStore
	if err := store.SetFunctionVersionFloor("g", "e", "player.ban", "0.3.0", "tester"); err != nil {
		t.Fatalf("seed floor a: %v", err)
	}
	if err := store.SetFunctionVersionFloor("g", "e", "mail.send", "1.0.0", "tester"); err != nil {
		t.Fatalf("seed floor b: %v", err)
	}

	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/version-floors", "")
	withFloorScope(ctx)
	h.VersionFloorsList(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp versionFloorsListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Floors["player.ban"] != "0.3.0" || resp.Floors["mail.send"] != "1.0.0" {
		t.Fatalf("expected both floors, got %v", resp.Floors)
	}
}

func TestHandler_VersionFloors_ListMissingScope(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/version-floors", "")

	h.VersionFloorsList(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "X-Game-ID") {
		t.Fatalf("expected scope error, got %s", rec.Body.String())
	}
}

func TestHandler_VersionFloorBatch_Set(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newFunctionTestContext(http.MethodPost, "/api/v1/functions/version-floor/batch",
		`{"functionIds":["player.ban","mail.send"],"minVersion":"0.3.0"}`)
	withFloorScope(ctx)
	h.VersionFloorBatch(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp versionFloorBatchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}
	if resp.Updated != 2 || len(resp.Failed) != 0 || resp.MinVersion != "0.3.0" {
		t.Fatalf("expected updated=2 failed=0 minVersion=0.3.0, got %+v", resp)
	}
	// 逐函数回读验证落库
	for id, want := range map[string]string{"player.ban": "0.3.0", "mail.send": "0.3.0"} {
		if got := h.service.SvcCtx().RegistryStore.GetFunctionVersionFloor("g", "e", id); got != want {
			t.Fatalf("floor %s: expected %q, got %q", id, want, got)
		}
	}
}

func TestHandler_VersionFloorBatch_Clear(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	store := h.service.SvcCtx().RegistryStore
	if err := store.SetFunctionVersionFloor("g", "e", "player.ban", "0.3.0", "tester"); err != nil {
		t.Fatalf("seed floor: %v", err)
	}

	ctx, rec := newFunctionTestContext(http.MethodPost, "/api/v1/functions/version-floor/batch",
		`{"functionIds":["player.ban"],"minVersion":""}`)
	withFloorScope(ctx)
	h.VersionFloorBatch(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp versionFloorBatchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Updated != 1 || len(resp.Failed) != 0 {
		t.Fatalf("expected updated=1 failed=0, got %+v", resp)
	}
	if got := store.GetFunctionVersionFloor("g", "e", "player.ban"); got != "" {
		t.Fatalf("expected cleared floor, got %q", got)
	}
}

func TestHandler_VersionFloorBatch_UnparseableRejected(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newFunctionTestContext(http.MethodPost, "/api/v1/functions/version-floor/batch",
		`{"functionIds":["player.ban"],"minVersion":"unknown"}`)
	withFloorScope(ctx)
	h.VersionFloorBatch(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "parseable") {
		t.Fatalf("expected parseable error, got %s", rec.Body.String())
	}
	// 零写入：统一值非法时不得留下任何门槛
	if got := h.service.SvcCtx().RegistryStore.GetFunctionVersionFloor("g", "e", "player.ban"); got != "" {
		t.Fatalf("expected zero writes, got floor %q", got)
	}
}

func TestHandler_VersionFloorBatch_MissingFunctionIds(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	for name, body := range map[string]string{
		"missing": `{}`,
		"empty":   `{"functionIds":[]}`,
		"blank":   `{"functionIds":["  ",""]}`,
	} {
		ctx, rec := newFunctionTestContext(http.MethodPost, "/api/v1/functions/version-floor/batch", body)
		withFloorScope(ctx)
		h.VersionFloorBatch(ctx)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d body=%s", name, rec.Code, rec.Body.String())
		}
	}
}

func TestHandler_VersionFloorBatch_MissingScope(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newFunctionTestContext(http.MethodPost, "/api/v1/functions/version-floor/batch",
		`{"functionIds":["player.ban"],"minVersion":"0.3.0"}`)

	h.VersionFloorBatch(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "X-Game-ID") {
		t.Fatalf("expected scope error, got %s", rec.Body.String())
	}
}

func TestHandler_VersionFloorBatch_TrimsAndDedupes(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newFunctionTestContext(http.MethodPost, "/api/v1/functions/version-floor/batch",
		`{"functionIds":[" player.ban ","player.ban","mail.send"],"minVersion":"0.3.0"}`)
	withFloorScope(ctx)
	h.VersionFloorBatch(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp versionFloorBatchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Updated != 2 {
		t.Fatalf("expected deduped updated=2, got %+v", resp)
	}
}

// setupFloorDBForHandler 迁出带 function_version_floors 的 sqlite 内存库
// （镜像 registry 包 setupFnFloorDB；跨包不能复用其测试 helper）。
func setupFloorDBForHandler(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.FunctionVersionFloor{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// TestHandler_VersionFloorBatch_PartialFailure：底层连接被关后逐函数写
// 失败进 failed（部分成功语义；首次批量在连接存活时成功落库）。
func TestHandler_VersionFloorBatch_PartialFailure(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	floorDB := setupFloorDBForHandler(t)
	sqlDB, err := floorDB.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}

	h := NewHandler(NewService(&svc.ServiceContext{RegistryStore: registry.NewStoreWithDB(floorDB)}))

	// 先在连接可用时成功写两个函数
	ctx, rec := newFunctionTestContext(http.MethodPost, "/api/v1/functions/version-floor/batch",
		`{"functionIds":["player.ban","mail.send"],"minVersion":"0.3.0"}`)
	withFloorScope(ctx)
	h.VersionFloorBatch(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp versionFloorBatchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Updated != 2 || len(resp.Failed) != 0 {
		t.Fatalf("expected updated=2 failed=0, got %+v", resp)
	}
	_ = sqlDB.Close()

	// 再跑一次批量：连接已关 → 两个函数都进 failed
	ctx2, rec2 := newFunctionTestContext(http.MethodPost, "/api/v1/functions/version-floor/batch",
		`{"functionIds":["player.ban","mail.send"],"minVersion":"1.0.0"}`)
	withFloorScope(ctx2)
	h.VersionFloorBatch(ctx2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec2.Code, rec2.Body.String())
	}
	var resp2 versionFloorBatchResponse
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp2.Updated != 0 || len(resp2.Failed) != 2 {
		t.Fatalf("expected updated=0 failed=2, got %+v", resp2)
	}
}
