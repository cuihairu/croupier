// Package openapi provides a generic provider for OpenAPI/Swagger based services.
package openapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/provider"
)

func TestNewProvider(t *testing.T) {
	p := NewProvider()
	if p == nil {
		t.Fatal("NewProvider() returned nil")
	}
	if p.methodMap == nil {
		t.Error("methodMap should be initialized")
	}
}

func TestProviderName(t *testing.T) {
	p := NewProvider()
	if name := p.Name(); name != "openapi" {
		t.Errorf("Name() = %q, want %q", name, "openapi")
	}
}

func TestProviderInit(t *testing.T) {
	tests := []struct {
		name    string
		config  provider.ProviderConfig
		wantErr bool
	}{
		{
			name: "basic config",
			config: provider.ProviderConfig{
				Enabled: true,
				Type:    "openapi",
				Config: map[string]interface{}{
					"base_url": "http://example.com",
					"methods": []interface{}{
						map[string]interface{}{
							"name":   "get_user",
							"path":   "/api/user",
							"method": "GET",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "config with auth",
			config: provider.ProviderConfig{
				Enabled: true,
				Type:    "openapi",
				Config: map[string]interface{}{
					"base_url": "http://example.com",
					"auth": map[string]interface{}{
						"type":  "bearer",
						"token": "test-token",
					},
					"methods": []interface{}{
						map[string]interface{}{
							"name":   "get_user",
							"path":   "/api/user",
							"method": "GET",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "config with timeout",
			config: provider.ProviderConfig{
				Enabled: true,
				Type:    "openapi",
				Config: map[string]interface{}{
					"base_url": "http://example.com",
					"timeout":  "10s", // Human-readable duration string
					"methods": []interface{}{
						map[string]interface{}{
							"name":   "test",
							"path":   "/test",
							"method": "GET",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "config with rate limit",
			config: provider.ProviderConfig{
				Enabled: true,
				Type:    "openapi",
				Config: map[string]interface{}{
					"base_url": "http://example.com",
				},
				RateLimit: &provider.RateLimitConfig{
					RequestsPerMinute: 60,
					BurstSize:         10,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProvider()
			err := p.Init(context.Background(), tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProviderIsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{"enabled", true, true},
		{"disabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProvider()
			_ = p.Init(context.Background(), provider.ProviderConfig{
				Enabled: tt.enabled,
				Type:    "openapi",
				Config: map[string]interface{}{
					"base_url": "http://example.com",
				},
			})
			if got := p.IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProviderSupportedMethods(t *testing.T) {
	p := NewProvider()
	_ = p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config: map[string]interface{}{
			"base_url": "http://example.com",
			"methods": []interface{}{
				map[string]interface{}{"name": "method1", "path": "/m1", "method": "GET"},
				map[string]interface{}{"name": "method2", "path": "/m2", "method": "POST"},
				map[string]interface{}{"name": "method3", "path": "/m3", "method": "PUT"},
			},
		},
	})

	methods := p.SupportedMethods()
	if len(methods) != 3 {
		t.Errorf("SupportedMethods() returned %d methods, want 3", len(methods))
	}

	expected := map[string]bool{"method1": true, "method2": true, "method3": true}
	for _, m := range methods {
		if !expected[m] {
			t.Errorf("unexpected method: %s", m)
		}
	}
}

func TestProviderGetMethodDetails(t *testing.T) {
	p := NewProvider()
	_ = p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config: map[string]interface{}{
			"base_url": "http://example.com",
			"methods": []interface{}{
				map[string]interface{}{
					"name":         "get_user",
					"operation_id": "getUser",
					"summary":      "Get user info",
					"description":  "Get user information by ID",
					"tags":         []interface{}{"user", "info"},
					"deprecated":   false,
					"path":         "/api/user",
					"method":       "GET",
				},
			},
		},
	})

	details := p.GetMethodDetails()
	if len(details) != 1 {
		t.Fatalf("GetMethodDetails() returned %d details, want 1", len(details))
	}

	d, ok := details["get_user"]
	if !ok {
		t.Fatal("method 'get_user' not found in details")
	}

	if d.Name != "get_user" {
		t.Errorf("Name = %q, want %q", d.Name, "get_user")
	}
	if d.OperationID != "getUser" {
		t.Errorf("OperationID = %q, want %q", d.OperationID, "getUser")
	}
	if d.Summary != "Get user info" {
		t.Errorf("Summary = %q, want %q", d.Summary, "Get user info")
	}
}

func TestProviderCall(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":   "123",
				"name": "Test User",
			})
		case "/api/user/123":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":   "123",
				"name": "User 123",
			})
		case "/api/error":
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	p := NewProvider()
	_ = p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config: map[string]interface{}{
			"base_url": server.URL,
			"methods": []interface{}{
				map[string]interface{}{
					"name":   "get_user",
					"path":   "/api/user",
					"method": "GET",
				},
				map[string]interface{}{
					"name":   "get_user_by_id",
					"path":   "/api/user/{user_id}",
					"method": "GET",
					"parameters": []interface{}{
						map[string]interface{}{
							"name":     "user_id",
							"in":       "path",
							"required": true,
						},
					},
				},
				map[string]interface{}{
					"name":   "error_method",
					"path":   "/api/error",
					"method": "GET",
				},
			},
		},
	})

	tests := []struct {
		name    string
		method  string
		request []byte
		wantErr bool
	}{
		{
			name:    "simple GET",
			method:  "get_user",
			request: nil,
			wantErr: false,
		},
		{
			name:    "GET with path param",
			method:  "get_user_by_id",
			request: []byte(`{"user_id": "123"}`),
			wantErr: false,
		},
		{
			name:    "method not found",
			method:  "unknown_method",
			request: nil,
			wantErr: true,
		},
		{
			name:    "server error",
			method:  "error_method",
			request: nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := p.Call(context.Background(), tt.method, tt.request)
			if (err != nil) != tt.wantErr {
				t.Errorf("Call() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && resp == nil {
				t.Error("Call() returned nil response")
			}
		})
	}
}

func TestProviderCallWithAuth(t *testing.T) {
	tests := []struct {
		name       string
		authConfig map[string]interface{}
		checkAuth  func(*http.Request) bool
	}{
		{
			name: "bearer auth",
			authConfig: map[string]interface{}{
				"type":  "bearer",
				"token": "test-token-123",
			},
			checkAuth: func(r *http.Request) bool {
				return r.Header.Get("Authorization") == "Bearer test-token-123"
			},
		},
		{
			name: "basic auth",
			authConfig: map[string]interface{}{
				"type":     "basic",
				"username": "user",
				"password": "pass",
			},
			checkAuth: func(r *http.Request) bool {
				user, pass, ok := r.BasicAuth()
				return ok && user == "user" && pass == "pass"
			},
		},
		{
			name: "api key header",
			authConfig: map[string]interface{}{
				"type": "api_key",
				"api_key": map[string]interface{}{
					"name":  "X-API-Key",
					"value": "secret-key",
					"in":    "header",
				},
			},
			checkAuth: func(r *http.Request) bool {
				return r.Header.Get("X-API-Key") == "secret-key"
			},
		},
		{
			name: "custom headers",
			authConfig: map[string]interface{}{
				"type": "custom",
				"custom_headers": map[string]interface{}{
					"X-Custom-Auth": "custom-value",
				},
			},
			checkAuth: func(r *http.Request) bool {
				return r.Header.Get("X-Custom-Auth") == "custom-value"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authChecked := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authChecked = tt.checkAuth(r)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			}))
			defer server.Close()

			p := NewProvider()
			_ = p.Init(context.Background(), provider.ProviderConfig{
				Enabled: true,
				Type:    "openapi",
				Config: map[string]interface{}{
					"base_url": server.URL,
					"auth":     tt.authConfig,
					"methods": []interface{}{
						map[string]interface{}{
							"name":   "test",
							"path":   "/test",
							"method": "GET",
						},
					},
				},
			})

			_, err := p.Call(context.Background(), "test", nil)
			if err != nil {
				t.Errorf("Call() error = %v", err)
			}
			if !authChecked {
				t.Error("auth was not properly applied")
			}
		})
	}
}

func TestProviderCallWithRequestBody(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	tests := []struct {
		name        string
		requestBody map[string]interface{}
		request     map[string]interface{}
		wantBody    map[string]interface{}
	}{
		{
			name: "field mapping",
			requestBody: map[string]interface{}{
				"type": "json",
				"fields": map[string]interface{}{
					"user_id": "player_id",
					"reason":  "ban_reason",
				},
			},
			request: map[string]interface{}{
				"player_id":  "123",
				"ban_reason": "cheating",
			},
			wantBody: map[string]interface{}{
				"user_id": "123",
				"reason":  "cheating",
			},
		},
		{
			name: "template",
			requestBody: map[string]interface{}{
				"type":     "json",
				"template": `{"id": "{{ .player_id }}", "action": "ban"}`,
			},
			request: map[string]interface{}{
				"player_id": "456",
			},
			wantBody: map[string]interface{}{
				"id":     "456",
				"action": "ban",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receivedBody = nil

			p := NewProvider()
			_ = p.Init(context.Background(), provider.ProviderConfig{
				Enabled: true,
				Type:    "openapi",
				Config: map[string]interface{}{
					"base_url": server.URL,
					"methods": []interface{}{
						map[string]interface{}{
							"name":         "test",
							"path":         "/test",
							"method":       "POST",
							"request_body": tt.requestBody,
						},
					},
				},
			})

			reqBytes, _ := json.Marshal(tt.request)
			_, err := p.Call(context.Background(), "test", reqBytes)
			if err != nil {
				t.Errorf("Call() error = %v", err)
				return
			}

			for k, v := range tt.wantBody {
				if receivedBody[k] != v {
					t.Errorf("body[%q] = %v, want %v", k, receivedBody[k], v)
				}
			}
		})
	}
}

func TestProviderCallWithResponseTransform(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    0,
			"message": "success",
			"data": map[string]interface{}{
				"id":   "123",
				"name": "test",
			},
		})
	}))
	defer server.Close()

	p := NewProvider()
	_ = p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config: map[string]interface{}{
			"base_url": server.URL,
			"transform": map[string]interface{}{
				"success_field": "code",
				"success_value": float64(0),
				"data_field":    "data",
				"error_field":   "message",
			},
			"methods": []interface{}{
				map[string]interface{}{
					"name":   "test",
					"path":   "/test",
					"method": "GET",
				},
			},
		},
	})

	resp, err := p.Call(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if result["success"] != true {
		t.Errorf("success = %v, want true", result["success"])
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data field not found or wrong type")
	}
	if data["id"] != "123" {
		t.Errorf("data.id = %v, want '123'", data["id"])
	}
}

func TestProviderCallDisabled(t *testing.T) {
	p := NewProvider()
	_ = p.Init(context.Background(), provider.ProviderConfig{
		Enabled: false,
		Type:    "openapi",
		Config: map[string]interface{}{
			"base_url": "http://example.com",
			"methods": []interface{}{
				map[string]interface{}{
					"name":   "test",
					"path":   "/test",
					"method": "GET",
				},
			},
		},
	})

	_, err := p.Call(context.Background(), "test", nil)
	if err == nil {
		t.Error("Call() should return error for disabled provider")
	}

	if _, ok := err.(*provider.ProviderDisabledError); !ok {
		t.Errorf("error type = %T, want *provider.ProviderDisabledError", err)
	}
}

func TestProviderClose(t *testing.T) {
	p := NewProvider()
	_ = p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config: map[string]interface{}{
			"base_url": "http://example.com",
		},
	})

	if err := p.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestBuildURL(t *testing.T) {
	p := NewProvider()
	_ = p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config: map[string]interface{}{
			"base_url": "http://example.com",
		},
	})

	tests := []struct {
		name    string
		method  *APIMethod
		reqData map[string]interface{}
		want    string
	}{
		{
			name: "simple path",
			method: &APIMethod{
				Path:   "/api/users",
				Method: "GET",
			},
			reqData: nil,
			want:    "http://example.com/api/users",
		},
		{
			name: "path with parameter",
			method: &APIMethod{
				Path:   "/api/users/{user_id}",
				Method: "GET",
				Parameters: []ParameterMapping{
					{Name: "user_id", In: "path"},
				},
			},
			reqData: map[string]interface{}{"user_id": "123"},
			want:    "http://example.com/api/users/123",
		},
		{
			name: "query parameters",
			method: &APIMethod{
				Path:   "/api/users",
				Method: "GET",
				Parameters: []ParameterMapping{
					{Name: "page", In: "query"},
					{Name: "limit", In: "query"},
				},
			},
			reqData: map[string]interface{}{"page": "1", "limit": "10"},
			want:    "http://example.com/api/users?page=1&limit=10",
		},
		{
			name: "mixed parameters",
			method: &APIMethod{
				Path:   "/api/users/{user_id}/posts",
				Method: "GET",
				Parameters: []ParameterMapping{
					{Name: "user_id", In: "path"},
					{Name: "page", In: "query"},
				},
			},
			reqData: map[string]interface{}{"user_id": "456", "page": "2"},
			want:    "http://example.com/api/users/456/posts?page=2",
		},
		{
			name: "parameter with from mapping",
			method: &APIMethod{
				Path:   "/api/users/{id}",
				Method: "GET",
				Parameters: []ParameterMapping{
					{Name: "id", In: "path", From: "user_id"},
				},
			},
			reqData: map[string]interface{}{"user_id": "789"},
			want:    "http://example.com/api/users/789",
		},
		{
			name: "parameter with default",
			method: &APIMethod{
				Path:   "/api/users",
				Method: "GET",
				Parameters: []ParameterMapping{
					{Name: "limit", In: "query", Default: "20"},
				},
			},
			reqData: map[string]interface{}{},
			want:    "http://example.com/api/users?limit=20",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.buildURL(tt.method, tt.reqData)
			if got != tt.want {
				t.Errorf("buildURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPathToMethodName(t *testing.T) {
	tests := []struct {
		path   string
		method string
		want   string
	}{
		{"/api/users", "GET", "getUsers"},
		{"/api/users/{id}", "GET", "getUsersId"},
		{"/api/v1/players", "POST", "postPlayers"},
		{"/v2/items/{item_id}/details", "PUT", "putItemsItem_idDetails"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := pathToMethodName(tt.path, tt.method)
			if got != tt.want {
				t.Errorf("pathToMethodName(%q, %q) = %q, want %q", tt.path, tt.method, got, tt.want)
			}
		})
	}
}

func TestParseOpenAPISpec(t *testing.T) {
	spec := `{
		"openapi": "3.0.3",
		"paths": {
			"/api/users": {
				"get": {
					"operationId": "listUsers",
					"summary": "List all users",
					"description": "Returns a list of users",
					"tags": ["users"],
					"deprecated": false,
					"parameters": [
						{
							"name": "page",
							"in": "query",
							"required": false,
							"schema": {"type": "integer", "default": 1}
						}
					]
				},
				"post": {
					"operationId": "createUser",
					"summary": "Create a user",
					"tags": ["users"]
				}
			},
			"/api/users/{id}": {
				"get": {
					"operationId": "getUser",
					"summary": "Get user by ID",
					"tags": ["users"],
					"parameters": [
						{
							"name": "id",
							"in": "path",
							"required": true,
							"schema": {"type": "string"}
						}
					]
				}
			}
		}
	}`

	p := NewProvider()
	p.openapiConfig = &Config{BaseURL: "http://example.com"}
	p.methodMap = make(map[string]*APIMethod)

	err := p.parseOpenAPISpec([]byte(spec))
	if err != nil {
		t.Fatalf("parseOpenAPISpec() error = %v", err)
	}

	if len(p.methods) != 3 {
		t.Errorf("expected 3 methods, got %d", len(p.methods))
	}

	// Check listUsers
	if m, ok := p.methodMap["listUsers"]; ok {
		if m.Summary != "List all users" {
			t.Errorf("listUsers.Summary = %q, want %q", m.Summary, "List all users")
		}
		if len(m.Tags) != 1 || m.Tags[0] != "users" {
			t.Errorf("listUsers.Tags = %v, want [users]", m.Tags)
		}
		if len(m.Parameters) != 1 {
			t.Errorf("listUsers.Parameters count = %d, want 1", len(m.Parameters))
		}
	} else {
		t.Error("method 'listUsers' not found")
	}

	// Check getUser
	if m, ok := p.methodMap["getUser"]; ok {
		if m.Path != "/api/users/{id}" {
			t.Errorf("getUser.Path = %q, want %q", m.Path, "/api/users/{id}")
		}
		if len(m.Parameters) != 1 || m.Parameters[0].Name != "id" {
			t.Errorf("getUser.Parameters incorrect")
		}
	} else {
		t.Error("method 'getUser' not found")
	}
}

func TestProviderCallWithContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	p := NewProvider()
	_ = p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config: map[string]interface{}{
			"base_url": server.URL,
			"methods": []interface{}{
				map[string]interface{}{
					"name":   "slow",
					"path":   "/slow",
					"method": "GET",
				},
			},
		},
	})

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := p.Call(ctx, "slow", nil)
	if err == nil {
		t.Error("Call() should return error for cancelled context")
	}
}

func TestFilterEmpty(t *testing.T) {
	tests := []struct {
		input []string
		want  []string
	}{
		{[]string{"a", "", "b", "", "c"}, []string{"a", "b", "c"}},
		{[]string{"", "", ""}, nil},
		{[]string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{nil, nil},
	}

	for _, tt := range tests {
		got := filterEmpty(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("filterEmpty(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
