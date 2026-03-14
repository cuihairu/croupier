package config

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

func newConfigTestContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	return ctx, rec
}

func TestBindConfigRequestUsesQueryForGet(t *testing.T) {
	t.Parallel()

	ctx, _ := newConfigTestContext(http.MethodGet, "/api/v1/config/versions?key=workspace:player&version=2", "")
	var req GetVersionRequest
	if err := bindConfigRequest(ctx, &req); err != nil {
		t.Fatalf("bindConfigRequest() error = %v", err)
	}
	if req.Key != "workspace:player" {
		t.Fatalf("expected key=workspace:player, got %q", req.Key)
	}
	if req.Version != 2 {
		t.Fatalf("expected version=2, got %d", req.Version)
	}
}

func TestBindConfigRequestUsesJSONForPost(t *testing.T) {
	t.Parallel()

	ctx, _ := newConfigTestContext(http.MethodPost, "/api/v1/config", `{"key":"workspace:player","value":"{}"}`)
	var req UpsertRequest
	if err := bindConfigRequest(ctx, &req); err != nil {
		t.Fatalf("bindConfigRequest() error = %v", err)
	}
	if req.Key != "workspace:player" {
		t.Fatalf("expected key=workspace:player, got %q", req.Key)
	}
}

func TestConfigHandlers(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))

	t.Run("UpsertRejectsMalformedJSON", func(t *testing.T) {
		t.Parallel()
		ctx, rec := newConfigTestContext(http.MethodPost, "/api/v1/config", "{")
		h.Upsert(ctx)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"code":400`) {
			t.Fatalf("expected wrapped 400 response, got status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("ListVersionsValidatesEmptyKey", func(t *testing.T) {
		t.Parallel()
		ctx, rec := newConfigTestContext(http.MethodGet, "/api/v1/config/versions", "")
		h.ListVersions(ctx)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"code":500`) {
			t.Fatalf("expected wrapped 500 response, got status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("GetVersionValidatesEmptyKey", func(t *testing.T) {
		t.Parallel()
		ctx, rec := newConfigTestContext(http.MethodGet, "/api/v1/config/version", "")
		h.GetVersion(ctx)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"code":500`) {
			t.Fatalf("expected wrapped 500 response, got status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}
