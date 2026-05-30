package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func assertAdminStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("expected status %d, got %d body=%s", want, rec.Code, rec.Body.String())
	}
}

func assertAdminRejected(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code == http.StatusOK {
		t.Fatalf("expected request to be rejected, got 200 body=%s", rec.Body.String())
	}
}

func newAdminTestContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	return ctx, rec
}

func TestHandler_List_BindValidation(t *testing.T) {

	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name     string
		query    string
		wantCode int
	}{
		{
			name:     "default parameters",
			query:    "",
			wantCode: 0, // Will fail at service layer
		},
		{
			name:     "with page",
			query:    "?page=2",
			wantCode: 0,
		},
		{
			name:     "with pageSize",
			query:    "?pageSize=10",
			wantCode: 0,
		},
		{
			name:     "with search",
			query:    "?search=admin",
			wantCode: 0,
		},
		{
			name:     "with role filter",
			query:    "?role=admin",
			wantCode: 0,
		},
		{
			name:     "with status filter",
			query:    "?status=1",
			wantCode: 0,
		},
		{
			name:     "all filters combined",
			query:    "?page=1&pageSize=10&search=test&role=admin&status=1",
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx, rec := newAdminTestContext("GET", "/admins"+tt.query, "")
			handler.List(ctx)

			if tt.wantCode == 0 {
				if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusUnauthorized && rec.Code != http.StatusOK {
					t.Errorf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
				}
			}
		})
	}
}

func TestHandler_Create_BindValidation(t *testing.T) {

	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name     string
		body     string
		wantCode int
	}{
		{
			name:     "valid create request",
			body:     `{"username":"newadmin","password":"MyPass123","nickname":"New Admin","email":"new@example.com"}`,
			wantCode: 0, // Will fail at service layer
		},
		{
			name:     "missing username",
			body:     `{"password":"MyPass123","nickname":"New Admin"}`,
			wantCode: 400,
		},
		{
			name:     "missing password",
			body:     `{"username":"newadmin","nickname":"New Admin"}`,
			wantCode: 400,
		},
		{
			name:     "empty json",
			body:     `{}`,
			wantCode: 400,
		},
		{
			name:     "invalid json",
			body:     `invalid`,
			wantCode: 400,
		},
		{
			name:     "with roles",
			body:     `{"username":"newadmin","password":"MyPass123","roles":["admin","editor"]}`,
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx, rec := newAdminTestContext("POST", "/admins", tt.body)
			handler.Create(ctx)

			if tt.wantCode != 0 {
				assertAdminRejected(t, rec)
			}
		})
	}
}

func TestHandler_Get_BindValidation(t *testing.T) {

	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name     string
		uri      string
		wantCode int
	}{
		{
			name:     "valid numeric id",
			uri:      "/admins/123",
			wantCode: 0, // Will fail at service layer
		},
		{
			name:     "empty id",
			uri:      "/admins/",
			wantCode: 400,
		},
		{
			name:     "non-numeric id",
			uri:      "/admins/abc",
			wantCode: 400,
		},
		{
			name:     "zero id",
			uri:      "/admins/0",
			wantCode: 400,
		},
		{
			name:     "negative id",
			uri:      "/admins/-1",
			wantCode: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx, rec := newAdminTestContext("GET", tt.uri, "")
			ctx.Params = gin.Params{gin.Param{Key: "id", Value: strings.TrimPrefix(tt.uri, "/admins/")}}
			handler.Get(ctx)

			if tt.wantCode != 0 {
				assertAdminRejected(t, rec)
			}
		})
	}
}

func TestHandler_Update_BindValidation(t *testing.T) {

	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name     string
		uri      string
		body     string
		wantCode int
	}{
		{
			name:     "valid update request",
			uri:      "/admins/123",
			body:     `{"nickname":"Updated Name","email":"updated@example.com","status":1}`,
			wantCode: 0, // Will fail at service layer
		},
		{
			name:     "empty roles array",
			uri:      "/admins/123",
			body:     `{"roles":[]}`,
			wantCode: 0,
		},
		{
			name:     "with roles",
			uri:      "/admins/123",
			body:     `{"roles":["admin","viewer"]}`,
			wantCode: 0,
		},
		{
			name:     "invalid id in uri",
			uri:      "/admins/abc",
			body:     `{"nickname":"Updated"}`,
			wantCode: 400,
		},
		{
			name:     "invalid json",
			uri:      "/admins/123",
			body:     `invalid`,
			wantCode: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx, rec := newAdminTestContext("PUT", tt.uri, tt.body)
			ctx.Params = gin.Params{gin.Param{Key: "id", Value: strings.TrimPrefix(tt.uri, "/admins/")}}
			handler.Update(ctx)

			if tt.wantCode != 0 {
				assertAdminRejected(t, rec)
			}
		})
	}
}

func TestHandler_Delete_BindValidation(t *testing.T) {

	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name     string
		uri      string
		wantCode int
	}{
		{
			name:     "valid numeric id",
			uri:      "/admins/123",
			wantCode: 0, // Will fail at service layer
		},
		{
			name:     "empty id",
			uri:      "/admins/",
			wantCode: 400,
		},
		{
			name:     "non-numeric id",
			uri:      "/admins/abc",
			wantCode: 400,
		},
		{
			name:     "zero id",
			uri:      "/admins/0",
			wantCode: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx, rec := newAdminTestContext("DELETE", tt.uri, "")
			ctx.Params = gin.Params{gin.Param{Key: "id", Value: strings.TrimPrefix(tt.uri, "/admins/")}}
			handler.Delete(ctx)

			if tt.wantCode != 0 {
				assertAdminRejected(t, rec)
			}
		})
	}
}

func TestHandler_PasswordReset_BindValidation(t *testing.T) {

	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name     string
		uri      string
		body     string
		wantCode int
	}{
		{
			name:     "valid password reset",
			uri:      "/admins/123/password-reset",
			body:     `{"newPassword":"newPassword123"}`,
			wantCode: 0, // Will fail at service layer
		},
		{
			name:     "missing new password",
			uri:      "/admins/123/password-reset",
			body:     `{}`,
			wantCode: 400,
		},
		{
			name:     "invalid id",
			uri:      "/admins/abc/password-reset",
			body:     `{"newPassword":"newPassword123"}`,
			wantCode: 400,
		},
		{
			name:     "invalid json",
			uri:      "/admins/123/password-reset",
			body:     `invalid`,
			wantCode: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx, rec := newAdminTestContext("POST", tt.uri, tt.body)
			ctx.Params = gin.Params{gin.Param{Key: "id", Value: strings.TrimPrefix(strings.TrimSuffix(tt.uri, "/password-reset"), "/admins/")}}
			handler.PasswordReset(ctx)

			if tt.wantCode != 0 {
				assertAdminRejected(t, rec)
			}
		})
	}
}

func TestHandler_GetGames_BindValidation(t *testing.T) {

	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name     string
		uri      string
		wantCode int
	}{
		{
			name:     "valid id",
			uri:      "/admins/123/games",
			wantCode: 0, // Will fail at service layer
		},
		{
			name:     "empty id",
			uri:      "/admins//games",
			wantCode: 400,
		},
		{
			name:     "non-numeric id",
			uri:      "/admins/abc/games",
			wantCode: 400,
		},
		{
			name:     "zero id",
			uri:      "/admins/0/games",
			wantCode: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx, rec := newAdminTestContext("GET", tt.uri, "")
			ctx.Params = gin.Params{gin.Param{Key: "id", Value: strings.TrimSuffix(strings.TrimPrefix(tt.uri, "/admins/"), "/games")}}
			handler.GetGames(ctx)

			if tt.wantCode != 0 {
				assertAdminRejected(t, rec)
			}
		})
	}
}

func TestHandler_UpdateGames_BindValidation(t *testing.T) {

	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name     string
		uri      string
		body     string
		wantCode int
	}{
		{
			name:     "valid update",
			uri:      "/admins/123/games",
			body:     `{"games":[{"gameId":"test","envs":["prod","dev"]}]}`,
			wantCode: 0, // Will fail at service layer
		},
		{
			name:     "empty games array",
			uri:      "/admins/123/games",
			body:     `{"games":[]}`,
			wantCode: 0,
		},
		{
			name:     "invalid id",
			uri:      "/admins/abc/games",
			body:     `{"games":[]}`,
			wantCode: 400,
		},
		{
			name:     "invalid json",
			uri:      "/admins/123/games",
			body:     `invalid`,
			wantCode: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx, rec := newAdminTestContext("PUT", tt.uri, tt.body)
			ctx.Params = gin.Params{gin.Param{Key: "id", Value: strings.TrimSuffix(strings.TrimPrefix(tt.uri, "/admins/"), "/games")}}
			handler.UpdateGames(ctx)

			if tt.wantCode != 0 {
				assertAdminRejected(t, rec)
			}
		})
	}
}
