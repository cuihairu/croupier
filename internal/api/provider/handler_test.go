package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

func newProviderTestContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	return ctx, rec
}

func TestBindProviderRequestUsesQueryForGet(t *testing.T) {
	t.Parallel()

	ctx, _ := newProviderTestContext(http.MethodGet, "/api/v1/providers?page=2&pageSize=10", "")
	var req ProvidersListRequest
	if err := bindProviderRequest(ctx, &req); err != nil {
		t.Fatalf("bindProviderRequest() error = %v", err)
	}
	if req.Page != 2 {
		t.Fatalf("expected page=2, got %d", req.Page)
	}
	if req.PageSize != 10 {
		t.Fatalf("expected pageSize=10, got %d", req.PageSize)
	}
}

func TestProviderHandlersReturnServiceErrors(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	cases := []struct {
		name   string
		target string
		fn     func(*gin.Context)
	}{
		{name: "List", target: "/api/v1/providers", fn: h.List},
		{name: "Capabilities", target: "/api/v1/providers/capabilities", fn: h.Capabilities},
		{name: "Descriptors", target: "/api/v1/providers/descriptors", fn: h.Descriptors},
		{name: "Detail", target: "/api/v1/providers/", fn: h.Detail},
		{name: "Entities", target: "/api/v1/providers/entities", fn: h.Entities},
		{name: "Delete", target: "/api/v1/providers/delete", fn: h.Delete},
		{name: "Reload", target: "/api/v1/providers/reload", fn: h.Reload},
		{name: "AliasGet", target: "/api/v1/providers/get", fn: h.Get},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, rec := newProviderTestContext(http.MethodGet, tc.target, "")
			tc.fn(ctx)
			if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
				t.Fatalf("unexpected status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}
