package function

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
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
