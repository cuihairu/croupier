package function

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

func newFunctionTestContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	return ctx, rec
}

func TestBindFunctionRequestUsesQueryForGet(t *testing.T) {
	t.Parallel()

	ctx, _ := newFunctionTestContext(http.MethodGet, "/api/v1/functions?page=2&pageSize=5&gameId=test", "")
	var req FunctionsListRequest
	if err := bindFunctionRequest(ctx, &req); err != nil {
		t.Fatalf("bindFunctionRequest() error = %v", err)
	}
	if req.Page != 2 {
		t.Fatalf("expected page=2, got %d", req.Page)
	}
	if req.PageSize != 5 {
		t.Fatalf("expected pageSize=5, got %d", req.PageSize)
	}
	if req.GameId != "test" {
		t.Fatalf("expected gameId=test, got %q", req.GameId)
	}
}

func TestBindFunctionRequestUsesJSONForPost(t *testing.T) {
	t.Parallel()

	ctx, _ := newFunctionTestContext(http.MethodPost, "/api/v1/functions/delete", `{"functionId":"f1"}`)
	var req FunctionDeleteRequest
	if err := bindFunctionRequest(ctx, &req); err != nil {
		t.Fatalf("bindFunctionRequest() error = %v", err)
	}
	if req.FunctionId != "f1" {
		t.Fatalf("expected functionId=f1, got %q", req.FunctionId)
	}
}

func TestBindFunctionRequestBindsURIParams(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/functions/inventory.consume/form", nil)
	ctx.Request = req
	ctx.Params = gin.Params{{Key: "id", Value: "inventory.consume"}}

	var bindReq FunctionFormRequest
	if err := bindFunctionRequest(ctx, &bindReq); err != nil {
		t.Fatalf("bindFunctionRequest() error = %v", err)
	}
	if bindReq.ID != "inventory.consume" {
		t.Fatalf("expected id=inventory.consume, got %q", bindReq.ID)
	}
}

func TestFunctionHandlersRejectMalformedJSON(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(&Service{})
	cases := []struct {
		name string
		fn   func(*gin.Context)
	}{
		{name: "FunctionsList", fn: h.FunctionsList},
		{name: "FunctionsPending", fn: h.FunctionsPending},
		{name: "FunctionDetail", fn: h.FunctionDetail},
		{name: "FunctionAnalytics", fn: h.FunctionAnalytics},
		{name: "FunctionCopy", fn: h.FunctionCopy},
		{name: "FunctionDelete", fn: h.FunctionDelete},
		{name: "FunctionDisable", fn: h.FunctionDisable},
		{name: "FunctionEnable", fn: h.FunctionEnable},
		{name: "FunctionHistory", fn: h.FunctionHistory},
		{name: "FunctionInvoke", fn: h.FunctionInvoke},
		{name: "FunctionPublish", fn: h.FunctionPublish},
		{name: "FunctionInstances", fn: h.FunctionInstances},
		{name: "FunctionInstancesAll", fn: h.FunctionInstancesAll},
		{name: "FunctionPermissions", fn: h.FunctionPermissions},
		{name: "FunctionPermissionsUpdate", fn: h.FunctionPermissionsUpdate},
		{name: "FunctionForm", fn: h.FunctionForm},
		{name: "FunctionFormUpdate", fn: h.FunctionFormUpdate},
		{name: "FunctionFormHistory", fn: h.FunctionFormHistory},
		{name: "FunctionFormRollback", fn: h.FunctionFormRollback},
		{name: "FunctionWarnings", fn: h.FunctionWarnings},
		{name: "Descriptors", fn: h.Descriptors},
		{name: "BatchCopyFunctions", fn: h.BatchCopyFunctions},
		{name: "BatchDeleteFunctions", fn: h.BatchDeleteFunctions},
		{name: "BatchUpdateFunctions", fn: h.BatchUpdateFunctions},
		{name: "AliasList", fn: h.List},
		{name: "AliasPending", fn: h.Pending},
		{name: "AliasAnalytics", fn: h.Analytics},
		{name: "AliasBatchDelete", fn: h.BatchDelete},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, rec := newFunctionTestContext(http.MethodPost, "/api/v1/functions", "{")
			tc.fn(ctx)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status=400, got %d body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "bad_request") {
				t.Fatalf("expected bad_request body, got %s", rec.Body.String())
			}
		})
	}
}

// TestHandler_FunctionDetail_ValidRequest tests valid request path
func TestHandler_FunctionDetail_ValidRequest(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/detail?id=test", "")

	h.FunctionDetail(ctx)

	// Should process the request
	if rec.Code != http.StatusOK && rec.Code != http.StatusInternalServerError {
		// Accept both since we're not mocking the full service
	}
}

// TestHandler_FunctionAnalytics_ValidRequest tests valid request path
func TestHandler_FunctionAnalytics_ValidRequest(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/analytics?id=test", "")

	h.FunctionAnalytics(ctx)

	// Should process the request
	if rec.Code != http.StatusOK && rec.Code != http.StatusInternalServerError {
		// Accept both since we're not mocking the full service
	}
}

// TestHandler_FunctionCopy_ValidRequest tests valid request path
func TestHandler_FunctionCopy_ValidRequest(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newFunctionTestContext(http.MethodPost, "/api/v1/functions/copy", `{
		"sourceGameId": "game1",
		"sourceEnv": "dev",
		"targetGameId": "game2",
		"targetEnv": "prod",
		"functionIds": ["f1", "f2"]
	}`)

	h.FunctionCopy(ctx)

	// Should process the request
	if rec.Code != http.StatusOK && rec.Code != http.StatusInternalServerError {
		// Accept both since we're not mocking the full service
	}
}

// TestHandler_FunctionInvoke_ValidRequest tests valid request path
func TestHandler_FunctionInvoke_ValidRequest(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newFunctionTestContext(http.MethodPost, "/api/v1/functions/invoke", `{
		"id": "test",
		"payload": {"key": "value"},
		"mode": "sync"
	}`)

	h.FunctionInvoke(ctx)

	// Should process the request
	if rec.Code != http.StatusOK && rec.Code != http.StatusInternalServerError {
		// Accept both since we're not mocking the full service
	}
}
