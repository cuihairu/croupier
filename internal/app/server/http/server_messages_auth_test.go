package httpserver

import (
    "bytes"
    "net/http"
    "net/http/httptest"
    "testing"

    jwt "github.com/cuihairu/croupier/internal/security/token"
)

// Messages authorization: GET requires JWT; POST requires messages:send/users:manage/admin
func TestMessagesAuth(t *testing.T) {
    s := &Server{}
    s.jwtMgr = jwt.NewManager("test")
    r := s.ginEngine()

    // 1) GET without token -> 401
    req1 := httptest.NewRequest(http.MethodGet, "/api/messages", nil)
    w1 := httptest.NewRecorder()
    r.ServeHTTP(w1, req1)
    if w1.Code != http.StatusUnauthorized { t.Fatalf("GET messages no token expected 401, got %d", w1.Code) }

    // 2) GET with token (viewer) -> 503 (no repo) or 404/200; assert not 401
    tokViewer, _ := s.jwtMgr.Sign("v1", []string{"viewer"}, 0)
    req2 := httptest.NewRequest(http.MethodGet, "/api/messages", nil)
    req2.Header.Set("Authorization", "Bearer "+tokViewer)
    w2 := httptest.NewRecorder()
    r = s.ginEngine()
    r.ServeHTTP(w2, req2)
    if w2.Code == http.StatusUnauthorized { t.Fatalf("GET messages with token should not be 401") }

    // 3) POST send message without permission -> 403
    req3 := httptest.NewRequest(http.MethodPost, "/api/messages", bytes.NewBufferString(`{"to":"u2","title":"hi"}`))
    req3.Header.Set("Authorization", "Bearer "+tokViewer)
    req3.Header.Set("Content-Type", "application/json")
    w3 := httptest.NewRecorder()
    r = s.ginEngine()
    r.ServeHTTP(w3, req3)
    if w3.Code != http.StatusForbidden { t.Fatalf("POST messages viewer expected 403, got %d", w3.Code) }

    // 4) admin role (via token roles) should pass RBAC check -> not 403; may be 503 due to missing repos
    tokAdmin, _ := s.jwtMgr.Sign("admin", []string{"admin"}, 0)
    req4 := httptest.NewRequest(http.MethodPost, "/api/messages", bytes.NewBufferString(`{"to":"u2","title":"hi"}`))
    req4.Header.Set("Authorization", "Bearer "+tokAdmin)
    req4.Header.Set("Content-Type", "application/json")
    w4 := httptest.NewRecorder()
    r = s.ginEngine()
    r.ServeHTTP(w4, req4)
    if w4.Code == http.StatusForbidden { t.Fatalf("POST messages admin should not be 403") }
}

