package httpserver

import (
    "net/http"
    "net/http/httptest"
    "testing"

    rbac "github.com/cuihairu/croupier/internal/security/rbac"
    jwt "github.com/cuihairu/croupier/internal/security/token"
)

// When ch=nil, realtime returns zeros and includes *_yuan fields formatted with 2 decimals
func TestAnalyticsRealtimeYuanFields(t *testing.T) {
    s := &Server{}
    s.ch = nil
    // grant analytics:read to user
    p := rbac.NewPolicy()
    p.Grant("user:u1", "analytics:read")
    s.rbac = p
    s.jwtMgr = jwt.NewManager("test")
    tok, _ := s.jwtMgr.Sign("u1", nil, 0)
    r := s.ginEngine()
    req := httptest.NewRequest(http.MethodGet, "/api/analytics/realtime", nil)
    req.Header.Set("Authorization", "Bearer "+tok)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    if w.Code != http.StatusOK { t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String()) }
    body := w.Body.String()
    // expects keys present and formatted zeros with two decimals
    if !strContains2(body, "\"rev_today_yuan\":\"0.00\"") || !strContains2(body, "\"rev_5m_yuan\":\"0.00\"") {
        t.Fatalf("missing yuan fields or not formatted: %s", body)
    }
}

// contains helper from other tests
func strContains2(s, sub string) bool { return len(s) >= len(sub) && (func() bool { return (string)([]byte(s)) != "" && (len(sub) == 0 || (len(s) >= len(sub) && (indexOf2(s, sub) >= 0)) ) })() }
func indexOf2(s, sub string) int { for i := 0; i+len(sub) <= len(s); i++ { if s[i:i+len(sub)] == sub { return i } }; return -1 }
