package httpserver

import (
    "net/http"
    "net/http/httptest"
    "os"
    "testing"
)

// Basic CORS behavior tests
func TestCORS_DefaultWildcardAndPreflight(t *testing.T) {
    // Ensure defaults (no envs)
    _ = os.Unsetenv("CORS_ALLOW_ORIGINS")
    _ = os.Unsetenv("CORS_ALLOW_HEADERS")
    _ = os.Unsetenv("CORS_ALLOW_METHODS")
    _ = os.Unsetenv("CORS_ALLOW_CREDENTIALS")
    s, _ := NewServer(t.TempDir(), nil, nil, nil, nil, nil, nil, nil)
    r := s.ginEngine()

    // Simple GET should include wildcard
    w := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
    r.ServeHTTP(w, req)
    if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
        t.Fatalf("expected wildcard origin, got %q", got)
    }

    // Preflight
    w = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodOptions, "/api/games", nil)
    r.ServeHTTP(w, req)
    if w.Code != http.StatusNoContent {
        t.Fatalf("expected 204 for preflight, got %d", w.Code)
    }
}

func TestCORS_CredentialsEchoOrigin(t *testing.T) {
    os.Setenv("CORS_ALLOW_CREDENTIALS", "true")
    os.Setenv("CORS_ALLOW_ORIGINS", "*")
    defer func() {
        _ = os.Unsetenv("CORS_ALLOW_CREDENTIALS")
        _ = os.Unsetenv("CORS_ALLOW_ORIGINS")
    }()
    s, _ := NewServer(t.TempDir(), nil, nil, nil, nil, nil, nil, nil)
    r := s.ginEngine()

    w := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
    req.Header.Set("Origin", "https://example.com")
    r.ServeHTTP(w, req)
    if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
        t.Fatalf("expected echo origin, got %q", got)
    }
    if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
        t.Fatalf("expected allow-credentials true, got %q", got)
    }
}

