package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
)

// TestNormalizeToSecond tests time normalization
func TestNormalizeToSecond(t *testing.T) {
	tests := []struct {
		name string
		t    time.Time
		want time.Time
	}{
		{
			name: "truncate to second",
			t:    time.Date(2024, 1, 1, 12, 34, 56, 123456789, time.UTC),
			want: time.Date(2024, 1, 1, 12, 34, 56, 0, time.UTC),
		},
		{
			name: "already truncated",
			t:    time.Date(2024, 1, 1, 12, 34, 56, 0, time.UTC),
			want: time.Date(2024, 1, 1, 12, 34, 56, 0, time.UTC),
		},
		{
			name: "with timezone",
			t:    time.Date(2024, 1, 1, 12, 34, 56, 500000000, time.FixedZone("UTC+8", 8*3600)),
			want: time.Date(2024, 1, 1, 4, 34, 56, 0, time.UTC), // UTC after conversion
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeToSecond(tt.t)
			if !got.Equal(tt.want) {
				t.Errorf("normalizeToSecond() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetUsernameFromContext tests extracting username from context
func TestGetUsernameFromContext(t *testing.T) {
	tests := []struct {
		name     string
		username string
		want     string
	}{
		{
			name:     "username in context",
			username: "testuser",
			want:     "testuser",
		},
		{
			name:     "empty username",
			username: "",
			want:     "",
		},
		{
			name:     "no username in context",
			username: "",
			want:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.username != "" {
				ctx = context.WithValue(ctx, "username", tt.username)
			}
			got := GetUsernameFromContext(ctx)
			if got != tt.want {
				t.Errorf("GetUsernameFromContext() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetRolesFromContext tests extracting roles from context
func TestGetRolesFromContext(t *testing.T) {
	tests := []struct {
		name  string
		roles []string
		want  []string
	}{
		{
			name:  "roles in context",
			roles: []string{"admin", "user"},
			want:  []string{"admin", "user"},
		},
		{
			name:  "empty roles",
			roles: []string{},
			want:  []string{},
		},
		{
			name:  "no roles in context",
			roles: nil,
			want:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.roles != nil {
				ctx = context.WithValue(ctx, "roles", tt.roles)
			}
			got := GetRolesFromContext(ctx)
			if tt.want == nil {
				if got != nil {
					t.Errorf("GetRolesFromContext() = %v, want nil", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("GetRolesFromContext() length = %d, want %d", len(got), len(tt.want))
				return
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("GetRolesFromContext()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestShouldBypass tests bypass logic for auth middleware
func TestShouldBypass(t *testing.T) {
	m := &AuthMiddleware{
		allowPaths: map[string]struct{}{
			"/api/v1/auth/login":         {},
			"/api/v1/monitoring/health":  {},
			"/api/v1/monitoring/healthz": {},
		},
		allowPref: []string{
			"/api/v1/auth/login",
		},
		publicReadPrefixes: []string{
			"/api/v1/configs",
		},
	}

	tests := []struct {
		name   string
		method string
		path   string
		want   bool
	}{
		{
			name:   "exact allow path",
			method: "POST",
			path:   "/api/v1/auth/login",
			want:   true,
		},
		{
			name:   "health endpoint",
			method: "GET",
			path:   "/api/v1/monitoring/health",
			want:   true,
		},
		{
			name:   "allow prefix",
			method: "POST",
			path:   "/api/v1/auth/login/callback",
			want:   true,
		},
		{
			name:   "GET on public read prefix",
			method: "GET",
			path:   "/api/v1/configs/public",
			want:   true,
		},
		{
			name:   "POST on public read prefix (not allowed)",
			method: "POST",
			path:   "/api/v1/configs/public",
			want:   false,
		},
		{
			name:   "protected path",
			method: "GET",
			path:   "/api/v1/admin/users",
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, tt.path, nil)
			got := m.shouldBypass(r)
			if got != tt.want {
				t.Errorf("shouldBypass() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNewAuthMiddleware tests creating auth middleware
func TestNewAuthMiddleware(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	m := NewAuthMiddleware(svcCtx)

	if m == nil {
		t.Fatal("NewAuthMiddleware() returned nil")
	}
	if m.svcCtx != svcCtx {
		t.Errorf("NewAuthMiddleware().svcCtx = %v, want %v", m.svcCtx, svcCtx)
	}
	if len(m.allowPaths) == 0 {
		t.Error("NewAuthMiddleware().allowPaths is empty")
	}
	if len(m.allowPref) == 0 {
		t.Error("NewAuthMiddleware().allowPref is empty")
	}
	if len(m.publicReadPrefixes) == 0 {
		t.Error("NewAuthMiddleware().publicReadPrefixes is empty")
	}
}

// TestNewDBHealth tests creating DBHealth
func TestNewDBHealth(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	h := NewDBHealth(svcCtx)

	if h == nil {
		t.Fatal("NewDBHealth() returned nil")
	}
	if h.svcCtx != svcCtx {
		t.Errorf("NewDBHealth().svcCtx = %v, want %v", h.svcCtx, svcCtx)
	}
}

// TestDBHealth_Check_NilContext tests check with nil service context
func TestDBHealth_Check_NilContext(t *testing.T) {
	h := &DBHealth{}
	ctx := context.Background()

	err := h.Check(ctx)
	if err == nil {
		t.Error("Check() expected error for nil svcCtx, got nil")
	}
}

// TestDBHealth_Check_NilAdminModel tests check with nil AdminModel
func TestDBHealth_Check_NilAdminModel(t *testing.T) {
	h := &DBHealth{
		svcCtx: &svc.ServiceContext{AdminModel: nil},
	}
	ctx := context.Background()

	err := h.Check(ctx)
	if err == nil {
		t.Error("Check() expected error for nil AdminModel, got nil")
	}
}

// TestRequirePermission tests requiring permission middleware
func TestRequirePermission(t *testing.T) {
	permission := "admin:write"
	middleware := RequirePermission(permission)

	if middleware == nil {
		t.Fatal("RequirePermission() returned nil")
	}

	// Create a test handler
	nextCalled := false
	next := func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	}

	// Wrap with middleware
	handler := middleware(next)

	// Test with unauthenticated request
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
	handler(w, r)

	if nextCalled {
		t.Error("RequirePermission() called next handler for unauthenticated request")
	}

	// Test with authenticated request
	nextCalled = false
	r = httptest.NewRequest("GET", "/api/v1/admin/users", nil)
	ctx := context.WithValue(r.Context(), "username", "testuser")
	r = r.WithContext(ctx)
	w = httptest.NewRecorder()
	handler(w, r)

	if !nextCalled {
		t.Error("RequirePermission() did not call next handler for authenticated request")
	}
}

// TestAuthMiddleware_Handle_Bypass tests bypass logic
func TestAuthMiddleware_Handle_Bypass(t *testing.T) {
	m := NewAuthMiddleware(&svc.ServiceContext{})

	nextCalled := false
	next := func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	}

	handler := m.Handle(next)

	tests := []struct {
		name string
		path string
	}{
		{
			name: "login endpoint",
			path: "/api/v1/auth/login",
		},
		{
			name: "health endpoint",
			path: "/api/v1/monitoring/health",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled = false
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", tt.path, nil)
			handler(w, r)

			if !nextCalled {
				t.Errorf("Handle() did not call next for bypass path %s", tt.path)
			}
		})
	}
}

// TestAuthMiddleware_Handle_MissingAuth tests missing authorization
func TestAuthMiddleware_Handle_MissingAuth(t *testing.T) {
	m := NewAuthMiddleware(&svc.ServiceContext{})

	next := func(w http.ResponseWriter, r *http.Request) {
		t.Error("next should not be called for missing auth")
	}

	handler := m.Handle(next)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
	handler(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Handle() status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// TestAuthMiddleware_Handle_InvalidAuthHeader tests invalid authorization header
func TestAuthMiddleware_Handle_InvalidAuthHeader(t *testing.T) {
	m := NewAuthMiddleware(&svc.ServiceContext{})

	next := func(w http.ResponseWriter, r *http.Request) {
		t.Error("next should not be called for invalid auth")
	}

	handler := m.Handle(next)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
	r.Header.Set("Authorization", "InvalidFormat token123")
	handler(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Handle() status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}
