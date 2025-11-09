package httpserver

import (
    "bytes"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "testing"

    "github.com/cuihairu/croupier/internal/security/rbac"
    jwt "github.com/cuihairu/croupier/internal/security/token"
)

// /api/packs/import requires packs:reload via require(); viewer denied, admin allowed (not 403).
func TestPacksImportRBAC(t *testing.T) {
    s := &Server{}
    // Use legacy policy for require() checks
    pol := rbac.NewPolicy()
    s.rbac = pol
    s.jwtMgr = jwt.NewManager("test")
    r := s.ginEngine()

    // viewer -> 403
    tokViewer, _ := s.jwtMgr.Sign("v1", []string{"viewer"}, 0)
    req1 := httptest.NewRequest(http.MethodPost, "/api/packs/import", bytes.NewBufferString(""))
    req1.Header.Set("Authorization", "Bearer "+tokViewer)
    w1 := httptest.NewRecorder()
    r.ServeHTTP(w1, req1)
    if w1.Code != http.StatusForbidden { t.Fatalf("viewer import expected 403, got %d", w1.Code) }

    // admin granted packs:reload -> not 403
    pol.Grant("user:admin", "packs:reload")
    tokAdmin, _ := s.jwtMgr.Sign("admin", nil, 0)
    req2 := httptest.NewRequest(http.MethodPost, "/api/packs/import", bytes.NewBufferString(""))
    req2.Header.Set("Authorization", "Bearer "+tokAdmin)
    w2 := httptest.NewRecorder()
    r = s.ginEngine()
    r.ServeHTTP(w2, req2)
    if w2.Code == http.StatusForbidden { t.Fatalf("admin import should not be 403") }
}

// /api/packs/list guarded by Casbin; viewer 403, tech_lead 200
func TestPacksListRBAC(t *testing.T) {
    s := &Server{}
    // Casbin
    p, err := rbac.LoadCasbinPolicy("../../../../configs/rbac.json")
    if err != nil { t.Fatalf("load casbin: %v", err) }
    if _, ok := p.(*rbac.CasbinPolicy); !ok { t.Skip("casbin not active") }
    s.rbac = p
    s.jwtMgr = jwt.NewManager("test")
    // prepare packDir with manifest
    dir := t.TempDir()
    s.packDir = dir
    _ = os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"name":"demo"}`), 0o644)
    _ = os.MkdirAll(filepath.Join(dir, "descriptors"), 0o755)
    _ = os.MkdirAll(filepath.Join(dir, "ui"), 0o755)

    r := s.ginEngine()
    // viewer denied
    tokViewer, _ := s.jwtMgr.Sign("v1", []string{"viewer"}, 0)
    req1 := httptest.NewRequest(http.MethodGet, "/api/packs/list", nil)
    req1.Header.Set("Authorization", "Bearer "+tokViewer)
    w1 := httptest.NewRecorder()
    r.ServeHTTP(w1, req1)
    if w1.Code != http.StatusForbidden { t.Fatalf("viewer list expected 403, got %d", w1.Code) }
    // tech_lead allowed
    tokLead, _ := s.jwtMgr.Sign("lead", []string{"tech_lead"}, 0)
    req2 := httptest.NewRequest(http.MethodGet, "/api/packs/list", nil)
    req2.Header.Set("Authorization", "Bearer "+tokLead)
    w2 := httptest.NewRecorder()
    r = s.ginEngine()
    r.ServeHTTP(w2, req2)
    if w2.Code != http.StatusOK { t.Fatalf("tech_lead list expected 200, got %d (%s)", w2.Code, w2.Body.String()) }
}
