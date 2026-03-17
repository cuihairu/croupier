package game

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newGameTestContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	return ctx, rec
}

func TestHandler_List_BindValidation(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name  string
		query string
	}{
		{"default", ""},
		{"with page", "?page=2"},
		{"with pageSize", "?pageSize=10"},
		{"with search", "?search=test"},
		{"all filters", "?page=1&pageSize=10&search=test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, _ := newGameTestContext("GET", "/games"+tt.query, "")
			handler.List(ctx)
			// Binding should succeed, service layer will fail
		})
	}
}

func TestHandler_Create_BindValidation(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name  string
		body  string
		valid bool
	}{
		{
			name:  "valid create request",
			body:  `{"name":"testgame","aliasName":"Test Game","color":"#8c8c8c"}`,
			valid: true,
		},
		{
			name:  "missing name",
			body:  `{"aliasName":"Test Game"}`,
			valid: false,
		},
		{
			name:  "empty json",
			body:  `{}`,
			valid: false,
		},
		{
			name:  "invalid json",
			body:  `invalid`,
			valid: false,
		},
		{
			name:  "with envs",
			body:  `{"name":"testgame","envs":[{"env":"prod"},{"env":"dev"}]}`,
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, rec := newGameTestContext("POST", "/games", tt.body)
			handler.Create(ctx)

			if !tt.valid && rec.Code == http.StatusOK {
				t.Errorf("Expected validation error for invalid request")
			}
		})
	}
}

func TestHandler_Detail_BindValidation(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name  string
		uri   string
		valid bool
	}{
		{"valid id", "/games/testgame", true},
		{"empty id", "/games/", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, rec := newGameTestContext("GET", tt.uri, "")
			ctx.Params = gin.Params{gin.Param{Key: "gameId", Value: strings.TrimPrefix(tt.uri, "/games/")}}
			handler.Detail(ctx)

			if !tt.valid && rec.Code == http.StatusOK {
				t.Errorf("Expected validation error")
			}
		})
	}
}

func TestHandler_Update_BindValidation(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name  string
		uri   string
		body  string
		valid bool
	}{
		{
			name:  "valid update",
			uri:   "/games/testgame",
			body:  `{"aliasName":"Updated Name","color":"#ffffff"}`,
			valid: true,
		},
		{
			name:  "invalid json",
			uri:   "/games/testgame",
			body:  `invalid`,
			valid: false,
		},
		{
			name:  "with envs",
			uri:   "/games/testgame",
			body:  `{"envs":[{"env":"prod"}]}`,
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, rec := newGameTestContext("PUT", tt.uri, tt.body)
			ctx.Params = gin.Params{gin.Param{Key: "gameId", Value: strings.TrimPrefix(tt.uri, "/games/")}}
			handler.Update(ctx)

			if !tt.valid && rec.Code == http.StatusOK {
				t.Errorf("Expected validation error")
			}
		})
	}
}

func TestHandler_Delete_BindValidation(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name  string
		uri   string
		valid bool
	}{
		{"valid id", "/games/testgame", true},
		{"empty id", "/games/", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, rec := newGameTestContext("DELETE", tt.uri, "")
			ctx.Params = gin.Params{gin.Param{Key: "gameId", Value: strings.TrimPrefix(tt.uri, "/games/")}}
			handler.Delete(ctx)

			if !tt.valid && rec.Code == http.StatusOK {
				t.Errorf("Expected validation error")
			}
		})
	}
}

func TestHandler_EnvsList_BindValidation(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name  string
		uri   string
		valid bool
	}{
		{"valid id", "/games/testgame/envs", true},
		{"empty id", "/games//envs", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, rec := newGameTestContext("GET", tt.uri, "")
			gameId := strings.TrimPrefix(strings.TrimSuffix(tt.uri, "/envs"), "/games/")
			ctx.Params = gin.Params{gin.Param{Key: "gameId", Value: gameId}}
			handler.EnvsList(ctx)

			if !tt.valid && rec.Code == http.StatusOK {
				t.Errorf("Expected validation error")
			}
		})
	}
}

func TestHandler_EnvAdd_BindValidation(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name  string
		uri   string
		body  string
		valid bool
	}{
		{
			name:  "valid add",
			uri:   "/games/testgame/envs",
			body:  `{"env":"staging","color":"#ff0000"}`,
			valid: true,
		},
		{
			name:  "missing env",
			uri:   "/games/testgame/envs",
			body:  `{"color":"#ff0000"}`,
			valid: false,
		},
		{
			name:  "invalid json",
			uri:   "/games/testgame/envs",
			body:  `invalid`,
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, rec := newGameTestContext("POST", tt.uri, tt.body)
			ctx.Params = gin.Params{gin.Param{Key: "gameId", Value: "testgame"}}
			handler.EnvAdd(ctx)

			if !tt.valid && rec.Code == http.StatusOK {
				t.Errorf("Expected validation error")
			}
		})
	}
}

func TestHandler_EnvUpdate_BindValidation(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name  string
		uri   string
		body  string
		valid bool
	}{
		{
			name:  "valid update",
			uri:   "/games/testgame/envs/prod",
			body:  `{"color":"#00ff00"}`,
			valid: true,
		},
		{
			name:  "invalid json",
			uri:   "/games/testgame/envs/prod",
			body:  `invalid`,
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, rec := newGameTestContext("PUT", tt.uri, tt.body)
			parts := strings.Split(strings.TrimPrefix(tt.uri, "/games/"), "/")
			ctx.Params = gin.Params{
				gin.Param{Key: "gameId", Value: parts[0]},
				gin.Param{Key: "env", Value: parts[1]},
			}
			handler.EnvUpdate(ctx)

			if !tt.valid && rec.Code == http.StatusOK {
				t.Errorf("Expected validation error")
			}
		})
	}
}

func TestHandler_EnvDelete_BindValidation(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&Service{})

	tests := []struct {
		name  string
		uri   string
		valid bool
	}{
		{"valid", "/games/testgame/envs/prod", true},
		{"missing env", "/games/testgame/envs/", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, rec := newGameTestContext("DELETE", tt.uri, "")
			parts := strings.Split(strings.TrimPrefix(tt.uri, "/games/"), "/")
			if len(parts) >= 2 {
				ctx.Params = gin.Params{
					gin.Param{Key: "gameId", Value: parts[0]},
					gin.Param{Key: "env", Value: parts[1]},
				}
			}
			handler.EnvDelete(ctx)

			if !tt.valid && rec.Code == http.StatusOK {
				t.Errorf("Expected validation error")
			}
		})
	}
}
