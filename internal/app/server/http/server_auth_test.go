package httpserver

import (
	"github.com/cuihairu/croupier/internal/security/rbac"
	jwt "github.com/cuihairu/croupier/internal/security/token"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Test that /api/registry is protected by require() and returns JSON errors
func TestRegistryAuthz(t *testing.T) {
	s := &Server{}
	r := s.ginEngine()

	// 1) No token -> 401
	req := httptest.NewRequest(http.MethodGet, "/api/registry", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	// 2) With token but no permission -> 403
	s.jwtMgr = jwt.NewManager("test-secret")
	s.rbac = rbac.NewPolicy() // deny by default
	tok, _ := s.jwtMgr.Sign("u1", []string{"guest"}, 0)
	req2 := httptest.NewRequest(http.MethodGet, "/api/registry", nil)
	req2.Header.Set("Authorization", "Bearer "+tok)
	w2 := httptest.NewRecorder()
	r = s.ginEngine() // rebuild to include middleware with jwtMgr
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w2.Code)
	}

	// 3) Grant permission -> 200
	p := s.rbac.(*rbac.Policy)
	p.Grant("user:u1", "registry:read")
	req3 := httptest.NewRequest(http.MethodGet, "/api/registry", nil)
	req3.Header.Set("Authorization", "Bearer "+tok)
	w3 := httptest.NewRecorder()
	r = s.ginEngine()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w3.Code)
	}
}
