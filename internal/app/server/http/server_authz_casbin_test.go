package httpserver

import (
    "bytes"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/cuihairu/croupier/internal/security/rbac"
    jwt "github.com/cuihairu/croupier/internal/security/token"
    pack "github.com/cuihairu/croupier/internal/pack"
)

// Test ginAuthZ with Casbin policy: path wildcard and method enforcement
func TestCasbinAuthZ_Entities(t *testing.T) {
    s := &Server{}
    // Load casbin model/policy from repo configs (path relative to this test file)
    p, err := rbac.LoadCasbinPolicy("../../../../configs/rbac.json")
    if err != nil {
        t.Fatalf("load casbin policy: %v", err)
    }
    if _, ok := p.(*rbac.CasbinPolicy); !ok {
        t.Skip("Casbin not active; skip")
    }
    s.rbac = p
    s.jwtMgr = jwt.NewManager("test-secret")
    r := s.ginEngine()

    // 1) No token -> 401
    req1 := httptest.NewRequest(http.MethodGet, "/api/entities", nil)
    w1 := httptest.NewRecorder()
    r.ServeHTTP(w1, req1)
    if w1.Code != http.StatusUnauthorized {
        t.Fatalf("expected 401, got %d", w1.Code)
    }

    // 2) developer can GET /api/entities -> 200
    tokDev, _ := s.jwtMgr.Sign("devuser", []string{"developer"}, 0)
    req2 := httptest.NewRequest(http.MethodGet, "/api/entities", nil)
    req2.Header.Set("Authorization", "Bearer "+tokDev)
    w2 := httptest.NewRecorder()
    r = s.ginEngine()
    r.ServeHTTP(w2, req2)
    if w2.Code != http.StatusOK {
        t.Fatalf("developer GET /api/entities expected 200, got %d", w2.Code)
    }

    // 3) viewer cannot POST /api/entities -> 403 (RBAC)
    tokViewer, _ := s.jwtMgr.Sign("viewer1", []string{"viewer"}, 0)
    req3 := httptest.NewRequest(http.MethodPost, "/api/entities", bytes.NewBufferString("{}"))
    req3.Header.Set("Authorization", "Bearer "+tokViewer)
    req3.Header.Set("Content-Type", "application/json")
    w3 := httptest.NewRecorder()
    r = s.ginEngine()
    r.ServeHTTP(w3, req3)
    if w3.Code != http.StatusForbidden {
        t.Fatalf("viewer POST /api/entities expected 403, got %d", w3.Code)
    }

    // 4) admin wildcard allows POST; route may return 400 for payload, but must not be 403
    tokAdmin, _ := s.jwtMgr.Sign("admin", []string{"admin"}, 0)
    req4 := httptest.NewRequest(http.MethodPost, "/api/entities", bytes.NewBufferString("{}"))
    req4.Header.Set("Authorization", "Bearer "+tokAdmin)
    req4.Header.Set("Content-Type", "application/json")
    w4 := httptest.NewRecorder()
    r = s.ginEngine()
    r.ServeHTTP(w4, req4)
    if w4.Code == http.StatusForbidden {
        t.Fatalf("admin POST /api/entities should not be 403 (got 403)")
    }
}

// Additional authz tests across endpoints/methods
func TestCasbinAuthZ_More(t *testing.T) {
    s := &Server{}
    p, err := rbac.LoadCasbinPolicy("../../../../configs/rbac.json")
    if err != nil { t.Fatalf("load casbin policy: %v", err) }
    if _, ok := p.(*rbac.CasbinPolicy); !ok { t.Skip("Casbin not active; skip") }
    s.rbac = p
    s.jwtMgr = jwt.NewManager("test-secret")
    // prepare component manager for /api/components route
    s.componentMgr = pack.NewComponentManager("data")
    _ = s.componentMgr.LoadRegistry()
    r := s.ginEngine()

    // components GET: developer forbidden, tech_lead allowed
    tokDev, _ := s.jwtMgr.Sign("dev", []string{"developer"}, 0)
    reqC1 := httptest.NewRequest(http.MethodGet, "/api/components", nil)
    reqC1.Header.Set("Authorization", "Bearer "+tokDev)
    wC1 := httptest.NewRecorder()
    r.ServeHTTP(wC1, reqC1)
    if wC1.Code != http.StatusForbidden { t.Fatalf("developer GET /api/components expected 403, got %d", wC1.Code) }

    tokLead, _ := s.jwtMgr.Sign("lead", []string{"tech_lead"}, 0)
    reqC2 := httptest.NewRequest(http.MethodGet, "/api/components", nil)
    reqC2.Header.Set("Authorization", "Bearer "+tokLead)
    wC2 := httptest.NewRecorder()
    r = s.ginEngine()
    r.ServeHTTP(wC2, reqC2)
    if wC2.Code != http.StatusOK { t.Fatalf("tech_lead GET /api/components expected 200, got %d", wC2.Code) }

    // games GET: producer allowed, viewer forbidden
    tokProd, _ := s.jwtMgr.Sign("pm", []string{"producer"}, 0)
    reqG1 := httptest.NewRequest(http.MethodGet, "/api/games", nil)
    reqG1.Header.Set("Authorization", "Bearer "+tokProd)
    wG1 := httptest.NewRecorder()
    r = s.ginEngine()
    r.ServeHTTP(wG1, reqG1)
    if wG1.Code != http.StatusOK { t.Fatalf("producer GET /api/games expected 200, got %d", wG1.Code) }

    tokViewer, _ := s.jwtMgr.Sign("viewer1", []string{"viewer"}, 0)
    reqG2 := httptest.NewRequest(http.MethodGet, "/api/games", nil)
    reqG2.Header.Set("Authorization", "Bearer "+tokViewer)
    wG2 := httptest.NewRecorder()
    r = s.ginEngine()
    r.ServeHTTP(wG2, reqG2)
    if wG2.Code != http.StatusForbidden { t.Fatalf("viewer GET /api/games expected 403, got %d", wG2.Code) }

    // entities id GET: developer allowed by wildcard; expect not 403 (likely 404 if entity missing)
    reqE := httptest.NewRequest(http.MethodGet, "/api/entities/some-id", nil)
    reqE.Header.Set("Authorization", "Bearer "+tokDev)
    wE := httptest.NewRecorder()
    r = s.ginEngine()
    r.ServeHTTP(wE, reqE)
    if wE.Code == http.StatusForbidden { t.Fatalf("developer GET /api/entities/:id should not be 403") }
}
