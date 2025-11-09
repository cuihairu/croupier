package httpserver

import (
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "testing"
)

// /api/packs/export: when s.packsExportRequireAuth=false, no auth required; when true, RBAC enforced
func TestPacksExportAuthToggle(t *testing.T) {
    s := &Server{}
    // prepare packDir with content
    dir := t.TempDir()
    s.packDir = dir
    _ = os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"name":"demo"}`), 0o644)
    _ = os.MkdirAll(filepath.Join(dir, "descriptors"), 0o755)
    _ = os.WriteFile(filepath.Join(dir, "descriptors", "a.json"), []byte(`{"id":"a"}`), 0o644)
    r := s.ginEngine()

    // toggle off -> 200 without token
    s.packsExportRequireAuth = false
    req1 := httptest.NewRequest(http.MethodGet, "/api/packs/export", nil)
    w1 := httptest.NewRecorder()
    r.ServeHTTP(w1, req1)
    if w1.Code != http.StatusOK { t.Fatalf("export without auth expected 200, got %d", w1.Code) }

    // toggle on -> requires auth; without token unified middleware returns 401
    s.packsExportRequireAuth = true
    req2 := httptest.NewRequest(http.MethodGet, "/api/packs/export", nil)
    w2 := httptest.NewRecorder()
    r = s.ginEngine()
    r.ServeHTTP(w2, req2)
    if w2.Code != http.StatusUnauthorized { t.Fatalf("export with auth expected 401 without token, got %d", w2.Code) }
}

