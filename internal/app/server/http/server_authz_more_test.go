package httpserver

import (
    "bytes"
    "mime/multipart"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/cuihairu/croupier/internal/security/rbac"
    jwt "github.com/cuihairu/croupier/internal/security/token"
)

func loadCasbin(t *testing.T) *rbac.CasbinPolicy {
    t.Helper()
    p, err := rbac.LoadCasbinPolicy("../../../../configs/rbac.json")
    if err != nil { t.Fatalf("load casbin: %v", err) }
    cp, ok := p.(*rbac.CasbinPolicy)
    if !ok { t.Skip("casbin not active; skip") }
    return cp
}

// viewer should be forbidden on assignments; gm allowed
func TestCasbinAuthZ_Assignments(t *testing.T) {
    s := &Server{}
    s.rbac = loadCasbin(t)
    s.jwtMgr = jwt.NewManager("test")
    r := s.ginEngine()

    tokViewer, _ := s.jwtMgr.Sign("v1", []string{"viewer"}, 0)
    req1 := httptest.NewRequest(http.MethodGet, "/api/assignments", nil)
    req1.Header.Set("Authorization", "Bearer "+tokViewer)
    w1 := httptest.NewRecorder()
    r.ServeHTTP(w1, req1)
    if w1.Code != http.StatusForbidden { t.Fatalf("viewer GET /api/assignments expected 403, got %d", w1.Code) }

    tokGM, _ := s.jwtMgr.Sign("gm1", []string{"gm"}, 0)
    req2 := httptest.NewRequest(http.MethodGet, "/api/assignments", nil)
    req2.Header.Set("Authorization", "Bearer "+tokGM)
    w2 := httptest.NewRecorder()
    r = s.ginEngine()
    r.ServeHTTP(w2, req2)
    if w2.Code != http.StatusOK { t.Fatalf("gm GET /api/assignments expected 200, got %d", w2.Code) }
}

// viewer forbidden for upload; tech_lead allowed by RBAC (route may return 400/503 depending on storage)
func TestCasbinAuthZ_Upload(t *testing.T) {
    s := &Server{}
    s.rbac = loadCasbin(t)
    s.jwtMgr = jwt.NewManager("test")
    r := s.ginEngine()

    // Build a small multipart body
    var buf bytes.Buffer
    mw := multipart.NewWriter(&buf)
    fw, _ := mw.CreateFormFile("file", "a.txt")
    _, _ = fw.Write([]byte("hello"))
    _ = mw.Close()

    tokViewer, _ := s.jwtMgr.Sign("v1", []string{"viewer"}, 0)
    req1 := httptest.NewRequest(http.MethodPost, "/api/upload", &buf)
    req1.Header.Set("Authorization", "Bearer "+tokViewer)
    req1.Header.Set("Content-Type", mw.FormDataContentType())
    w1 := httptest.NewRecorder()
    r.ServeHTTP(w1, req1)
    if w1.Code != http.StatusForbidden { t.Fatalf("viewer POST /api/upload expected 403, got %d", w1.Code) }

    tokLead, _ := s.jwtMgr.Sign("lead", []string{"tech_lead"}, 0)
    req2 := httptest.NewRequest(http.MethodPost, "/api/upload", &buf)
    req2.Header.Set("Authorization", "Bearer "+tokLead)
    req2.Header.Set("Content-Type", mw.FormDataContentType())
    w2 := httptest.NewRecorder()
    r = s.ginEngine()
    r.ServeHTTP(w2, req2)
    if w2.Code == http.StatusForbidden { t.Fatalf("tech_lead POST /api/upload should not be 403") }
}

