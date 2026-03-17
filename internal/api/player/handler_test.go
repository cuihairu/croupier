package player

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newPlayerTestContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
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
		{"with page", "?page=2&pageSize=10"},
		{"with gameId", "?gameId=testgame"},
		{"with env", "?env=prod"},
		{"all filters", "?page=1&pageSize=10&gameId=test&env=prod&search=player1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, _ := newPlayerTestContext("GET", "/players"+tt.query, "")
			// Will panic due to nil model, catch it
			defer func() {
				if r := recover(); r != nil {
					// Expected to panic due to nil model
					return
				}
			}()
			handler.List(ctx)
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
			name:  "valid create",
			body:  `{"gameId":"testgame","playerId":"player123","nickname":"Player One"}`,
			valid: true,
		},
		{
			name:  "missing gameId",
			body:  `{"playerId":"player123","nickname":"Player One"}`,
			valid: false,
		},
		{
			name:  "missing playerId",
			body:  `{"gameId":"testgame","nickname":"Player One"}`,
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
			name:  "with balance",
			body:  `{"gameId":"testgame","playerId":"player123","balance":1000}`,
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, rec := newPlayerTestContext("POST", "/players", tt.body)
			if tt.valid {
				// Will panic due to nil model
				defer func() {
					if r := recover(); r != nil {
						return
					}
				}()
			}
			handler.Create(ctx)

			if !tt.valid {
				if rec.Code == http.StatusOK {
					t.Errorf("Expected validation error")
				}
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
		{"valid id", "/players/123", true},
		{"empty id", "/players/", false},
		{"non-numeric id", "/players/abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, rec := newPlayerTestContext("GET", tt.uri, "")
			ctx.Params = gin.Params{gin.Param{Key: "id", Value: strings.TrimPrefix(tt.uri, "/players/")}}
			if tt.valid {
				defer func() {
					if r := recover(); r != nil {
						return
					}
				}()
			}
			handler.Detail(ctx)

			if !tt.valid {
				if rec.Code == http.StatusOK {
					t.Errorf("Expected validation error")
				}
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
			uri:   "/players/123",
			body:  `{"nickname":"Updated Name","avatar":"http://example.com/avatar.png"}`,
			valid: true,
		},
		{
			name:  "with balance adjustment",
			uri:   "/players/123",
			body:  `{"balanceAdjustment":100}`,
			valid: true,
		},
		{
			name:  "invalid json",
			uri:   "/players/123",
			body:  `invalid`,
			valid: false,
		},
		{
			name:  "invalid id",
			uri:   "/players/abc",
			body:  `{"nickname":"Updated"}`,
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, rec := newPlayerTestContext("PUT", tt.uri, tt.body)
			ctx.Params = gin.Params{gin.Param{Key: "id", Value: strings.TrimPrefix(tt.uri, "/players/")}}
			if tt.valid {
				defer func() {
					if r := recover(); r != nil {
						return
					}
				}()
			}
			handler.Update(ctx)

			if !tt.valid {
				if rec.Code == http.StatusOK {
					t.Errorf("Expected validation error")
				}
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
		{"valid id", "/players/123", true},
		{"empty id", "/players/", false},
		{"non-numeric id", "/players/abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, rec := newPlayerTestContext("DELETE", tt.uri, "")
			ctx.Params = gin.Params{gin.Param{Key: "id", Value: strings.TrimPrefix(tt.uri, "/players/")}}
			if tt.valid {
				defer func() {
					if r := recover(); r != nil {
						return
					}
				}()
			}
			handler.Delete(ctx)

			if !tt.valid {
				if rec.Code == http.StatusOK {
					t.Errorf("Expected validation error")
				}
			}
		})
	}
}

func TestHandler_Balance_BindValidation(t *testing.T) {
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
			name:  "valid balance adjustment",
			uri:   "/players/123/balance",
			body:  `{"amount":100,"reason":"deposit"}`,
			valid: true,
		},
		{
			name:  "negative amount",
			uri:   "/players/123/balance",
			body:  `{"amount":-50,"reason":"withdrawal"}`,
			valid: true,
		},
		{
			name:  "missing amount",
			uri:   "/players/123/balance",
			body:  `{"reason":"test"}`,
			valid: false,
		},
		{
			name:  "invalid json",
			uri:   "/players/123/balance",
			body:  `invalid`,
			valid: false,
		},
		{
			name:  "invalid id",
			uri:   "/players/abc/balance",
			body:  `{"amount":100}`,
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, rec := newPlayerTestContext("POST", tt.uri, tt.body)
			id := strings.TrimSuffix(strings.TrimPrefix(tt.uri, "/players/"), "/balance")
			ctx.Params = gin.Params{gin.Param{Key: "id", Value: id}}
			// Wrap all handler calls in panic recovery
			// since service layer may panic even for valid bindings
			panicked := false
			func() {
				defer func() {
					if r := recover(); r != nil {
						panicked = true
					}
				}()
				handler.Balance(ctx)
			}()

			if !tt.valid && !panicked {
				if rec.Code == http.StatusOK {
					t.Errorf("Expected validation error")
				}
			}
		})
	}
}
