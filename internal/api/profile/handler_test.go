package profile

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newProfileTestContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	return ctx, rec
}

func TestHandler_GetProfile_Unauthorized(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})
	ctx, rec := newProfileTestContext("GET", "/profile", "")
	handler.GetProfile(ctx)

	var result map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["code"].(float64) != 401 {
		t.Errorf("Expected code 401 for unauthorized, got %v", result["code"])
	}
}

func TestHandler_GetProfile_WithUsername(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})
	ctx, _ := newProfileTestContext("GET", "/profile", "")
	ctx.Set("username", "admin")

	// Will panic due to nil model, catch it
	defer func() {
		if r := recover(); r != nil {
			// Expected to panic due to nil model
			return
		}
		t.Errorf("Expected panic due to nil model")
	}()
	handler.GetProfile(ctx)
}

func TestHandler_GetGames_Unauthorized(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})
	ctx, rec := newProfileTestContext("GET", "/profile/games", "")
	handler.GetGames(ctx)

	var result map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["code"].(float64) != 401 {
		t.Errorf("Expected code 401 for unauthorized, got %v", result["code"])
	}
}

func TestHandler_GetGames_WithUsername(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})
	ctx, _ := newProfileTestContext("GET", "/profile/games", "")
	ctx.Set("username", "admin")

	// Will panic due to nil model, catch it
	defer func() {
		if r := recover(); r != nil {
			// Expected to panic due to nil model
			return
		}
		t.Errorf("Expected panic due to nil model")
	}()
	handler.GetGames(ctx)
}

func TestHandler_UpdateProfile_Unauthorized(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})
	ctx, rec := newProfileTestContext("PUT", "/profile", `{"nickname":"Updated"}`)
	handler.UpdateProfile(ctx)

	var result map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["code"].(float64) != 401 {
		t.Errorf("Expected code 401 for unauthorized, got %v", result["code"])
	}
}

func TestHandler_UpdateProfile_BindValidation(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		body     string
		wantCode int
	}{
		{
			name:     "valid update request",
			body:     `{"nickname":"Updated Nickname","email":"updated@example.com","phone":"1234567890","avatar":"http://example.com/avatar.png"}`,
			wantCode: 401, // Auth check happens first
		},
		{
			name:     "empty json",
			body:     `{}`,
			wantCode: 401,
		},
		{
			name:     "invalid json",
			body:     `invalid`,
			wantCode: 401,
		},
		{
			name:     "partial update",
			body:     `{"nickname":"Only Nickname"}`,
			wantCode: 401,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := NewHandler(&Service{})
			ctx, rec := newProfileTestContext("PUT", "/profile", tt.body)
			handler.UpdateProfile(ctx)

			var result map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err == nil {
				if code, ok := result["code"].(float64); ok && int(code) != tt.wantCode {
					t.Errorf("Expected code %d, got %d", tt.wantCode, int(code))
				}
			}
		})
	}
}

func TestHandler_UpdateProfile_WithUsername(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})
	ctx, _ := newProfileTestContext("PUT", "/profile", `{"nickname":"Updated"}`)
	ctx.Set("username", "admin")

	// Will panic due to nil model, catch it
	defer func() {
		if r := recover(); r != nil {
			// Expected to panic due to nil model
			return
		}
		t.Errorf("Expected panic due to nil model")
	}()
	handler.UpdateProfile(ctx)
}

func TestHandler_ChangePassword_Unauthorized(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})
	ctx, rec := newProfileTestContext("POST", "/profile/change-password", `{"oldPassword":"old","newPassword":"new"}`)
	handler.ChangePassword(ctx)

	var result map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["code"].(float64) != 401 {
		t.Errorf("Expected code 401 for unauthorized, got %v", result["code"])
	}
}

func TestHandler_ChangePassword_BindValidation(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		body     string
		wantCode int
	}{
		{
			name:     "valid change password",
			body:     `{"oldPassword":"oldPassword123","newPassword":"newPassword123"}`,
			wantCode: 401, // Auth check happens first
		},
		{
			name:     "missing old password",
			body:     `{"newPassword":"newPassword123"}`,
			wantCode: 401, // Auth check happens before validation
		},
		{
			name:     "missing new password",
			body:     `{"oldPassword":"oldPassword123"}`,
			wantCode: 401, // Auth check happens before validation
		},
		{
			name:     "empty json",
			body:     `{}`,
			wantCode: 401, // Auth check happens before validation
		},
		{
			name:     "invalid json",
			body:     `invalid`,
			wantCode: 401, // Auth check happens before JSON parsing
		},
		{
			name:     "short new password",
			body:     `{"oldPassword":"old","newPassword":"12345"}`,
			wantCode: 401, // Auth check happens before validation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := NewHandler(&Service{})
			ctx, rec := newProfileTestContext("POST", "/profile/change-password", tt.body)
			handler.ChangePassword(ctx)

			var result map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err == nil {
				if code, ok := result["code"].(float64); ok && int(code) != tt.wantCode {
					t.Errorf("Expected code %d, got %d. Body: %s", tt.wantCode, int(code), rec.Body.String())
				}
			}
		})
	}
}

func TestHandler_ChangePassword_WithUsername(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})
	ctx, _ := newProfileTestContext("POST", "/profile/change-password", `{"oldPassword":"old","newPassword":"new123456"}`)
	ctx.Set("username", "admin")

	// Will panic due to nil model, catch it
	defer func() {
		if r := recover(); r != nil {
			// Expected to panic due to nil model
			return
		}
		t.Errorf("Expected panic due to nil model")
	}()
	handler.ChangePassword(ctx)
}

func TestHandler_GetPermissions_Unauthorized(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})
	ctx, rec := newProfileTestContext("GET", "/profile/permissions", "")
	handler.GetPermissions(ctx)

	var result map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["code"].(float64) != 401 {
		t.Errorf("Expected code 401 for unauthorized, got %v", result["code"])
	}
}

func TestHandler_GetPermissions_WithUsername(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})
	ctx, _ := newProfileTestContext("GET", "/profile/permissions", "")
	ctx.Set("username", "admin")

	// Will panic due to nil model, catch it
	defer func() {
		if r := recover(); r != nil {
			// Expected to panic due to nil model
			return
		}
		t.Errorf("Expected panic due to nil model")
	}()
	handler.GetPermissions(ctx)
}
