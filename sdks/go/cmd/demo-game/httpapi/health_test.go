package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/sdks/go/cmd/demo-game/httpapi"
)

func TestHealthzAlwaysOK(t *testing.T) {
	rec := httptest.NewRecorder()
	httpapi.Healthz(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got == "" {
		t.Fatal("healthz returned empty body")
	}
}

func TestReadyzFlipsWithFlag(t *testing.T) {
	var ready httpapi.Ready
	handler := httpapi.Readyz(&ready)

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz before SetReady = %d, want 503", rec.Code)
	}

	ready.SetReady(true)
	rec = httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("readyz after SetReady = %d, want 200", rec.Code)
	}

	ready.SetReady(false)
	rec = httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz during shutdown = %d, want 503", rec.Code)
	}
}
