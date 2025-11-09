package rbac

import (
    "net/http"
    "testing"
)

// Test that Casbin policy with keyMatch2 and method wildcard works as expected.
func TestCasbinKeyMatch2(t *testing.T) {
    p, err := LoadCasbinPolicy("../../../configs/rbac.json")
    if err != nil {
        t.Fatalf("load casbin policy: %v", err)
    }
    cp, ok := p.(*CasbinPolicy)
    if !ok {
        t.Skip("Casbin not active (legacy policy in use); skip")
    }
    // developer can GET /api/entities/*
    req, _ := http.NewRequest(http.MethodGet, "/api/entities/123", nil)
    if !cp.CanHTTP("u1", []string{"developer"}, req) {
        t.Fatalf("developer should be allowed to GET /api/entities/*")
    }
    // but cannot PUT /api/entities/*
    req2, _ := http.NewRequest(http.MethodPut, "/api/entities/123", nil)
    if cp.CanHTTP("u1", []string{"developer"}, req2) {
        t.Fatalf("developer should NOT be allowed to PUT /api/entities/*")
    }
    // admin role wildcard
    req3, _ := http.NewRequest(http.MethodDelete, "/api/anything/here", nil)
    if !cp.CanHTTP("admin-user", []string{"admin"}, req3) {
        t.Fatalf("admin should be allowed to * on *")
    }
}
