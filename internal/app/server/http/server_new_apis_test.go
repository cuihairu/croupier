package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	auditchain "github.com/cuihairu/croupier/internal/audit/chain"
	pack "github.com/cuihairu/croupier/internal/pack"
	registry "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/security/rbac"
	jwt "github.com/cuihairu/croupier/internal/security/token"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
)

// Mock components for testing new APIs
type mockFunctionInvoker struct{}

func (mockFunctionInvoker) Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
	return &functionv1.InvokeResponse{Payload: []byte(`{"result":"success"}`)}, nil
}

func (mockFunctionInvoker) StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) {
	return &functionv1.StartJobResponse{JobId: "job-123"}, nil
}

func (mockFunctionInvoker) StreamJob(ctx context.Context, req *functionv1.JobStreamRequest) (functionv1.FunctionService_StreamJobClient, error) {
	return nil, nil
}

func (mockFunctionInvoker) CancelJob(ctx context.Context, req *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error) {
	return &functionv1.StartJobResponse{JobId: "job-123"}, nil
}

func setupTestServer(t *testing.T) (*Server, string) {
	dir := t.TempDir()

	// Setup minimal audit writer
	_ = os.MkdirAll("logs", 0o755)
	aw, err := auditchain.NewWriter("logs/audit.log")
	if err != nil {
		t.Fatalf("audit writer: %v", err)
	}

	// Create registry store with mock data
	store := registry.NewStore()

	// Add mock agent data
	store.UpsertAgent(&registry.AgentSession{
		AgentID:  "test-agent-1",
		GameID:   "test-game",
		Env:      "development",
		RPCAddr:  "127.0.0.1:19090",
		Version:  "1.0.0",
		Functions: map[string]registry.FunctionMeta{
			"test-func": {
				Enabled: true,
			},
		},
		ExpireAt: time.Now().Add(time.Hour),
	})

	// Add mock provider capabilities
	store.UpsertProviderCaps(registry.ProviderCaps{
		ID:       "test-provider",
		Version:  "1.0.0",
		Lang:     "go",
		SDK:      "go-sdk",
		Manifest: []byte(`{"functions":[{"id":"provider-func","version":"1.0.0"}]}`),
	})

	// Create server with all dependencies
	srv, err := NewServer(dir, new(mockFunctionInvoker), aw, nil, store, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Setup component manager
	srv.componentMgr = pack.NewComponentManager(dir)
	_ = srv.componentMgr.LoadRegistry()

	// Setup RBAC
	p, err := rbac.LoadCasbinPolicy("../../../../configs/rbac.json")
	if err == nil {
		srv.rbac = p
	}
	srv.jwtMgr = jwt.NewManager("test-secret")

	return srv, dir
}

// Test Function Management APIs
func TestFunctionManagementAPIs(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create JWT tokens for different roles
	adminToken, _ := srv.jwtMgr.Sign("admin", []string{"admin"}, 0)
	developerToken, _ := srv.jwtMgr.Sign("dev", []string{"developer"}, 0)
	viewerToken, _ := srv.jwtMgr.Sign("viewer", []string{"viewer"}, 0)

	tests := []struct {
		name       string
		method     string
		path       string
		token      string
		body       string
		expectCode int
	}{
		// GET /api/functions - List all functions
		{
			name:       "List functions as admin",
			method:     "GET",
			path:       "/api/functions",
			token:      adminToken,
			expectCode: 200,
		},
		{
			name:       "List functions as developer",
			method:     "GET",
			path:       "/api/functions",
			token:      developerToken,
			expectCode: 200,
		},
		{
			name:       "List functions without auth",
			method:     "GET",
			path:       "/api/functions",
			token:      "",
			expectCode: 401,
		},

		// GET /api/functions/:id - Get specific function
		{
			name:       "Get specific function as admin",
			method:     "GET",
			path:       "/api/functions/test-func",
			token:      adminToken,
			expectCode: 200,
		},
		{
			name:       "Get nonexistent function",
			method:     "GET",
			path:       "/api/functions/nonexistent",
			token:      adminToken,
			expectCode: 404,
		},

		// PATCH /api/functions/:id/enable - Enable function
		{
			name:       "Enable function as admin",
			method:     "PATCH",
			path:       "/api/functions/test-func/enable",
			token:      adminToken,
			expectCode: 200,
		},
		{
			name:       "Enable function as viewer (should fail)",
			method:     "PATCH",
			path:       "/api/functions/test-func/enable",
			token:      viewerToken,
			expectCode: 403,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != "" {
				body = bytes.NewBufferString(tt.body)
			} else {
				body = nil
			}

			req := httptest.NewRequest(tt.method, tt.path, body)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			rr := httptest.NewRecorder()
			srv.ginEngine().ServeHTTP(rr, req)

			if rr.Code != tt.expectCode {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectCode, rr.Code, rr.Body.String())
			}

			// For successful GET requests, verify response structure
			if tt.expectCode == 200 && tt.method == "GET" {
				var response map[string]interface{}
				if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
					t.Errorf("Failed to parse JSON response: %v", err)
				}
			}
		})
	}
}

// Test Provider Management APIs
func TestProviderManagementAPIs(t *testing.T) {
	srv, _ := setupTestServer(t)

	adminToken, _ := srv.jwtMgr.Sign("admin", []string{"admin"}, 0)
	viewerToken, _ := srv.jwtMgr.Sign("viewer", []string{"viewer"}, 0)

	tests := []struct {
		name       string
		method     string
		path       string
		token      string
		expectCode int
	}{
		// GET /api/providers - List providers
		{
			name:       "List providers as admin",
			method:     "GET",
			path:       "/api/providers",
			token:      adminToken,
			expectCode: 200,
		},
		{
			name:       "List providers without auth",
			method:     "GET",
			path:       "/api/providers",
			token:      "",
			expectCode: 401,
		},

		// GET /api/providers/:id - Get specific provider
		{
			name:       "Get specific provider as admin",
			method:     "GET",
			path:       "/api/providers/test-provider",
			token:      adminToken,
			expectCode: 200,
		},
		{
			name:       "Get nonexistent provider",
			method:     "GET",
			path:       "/api/providers/nonexistent",
			token:      adminToken,
			expectCode: 404,
		},

		// DELETE /api/providers/:id - Delete provider
		{
			name:       "Delete provider as admin",
			method:     "DELETE",
			path:       "/api/providers/test-provider",
			token:      adminToken,
			expectCode: 200,
		},
		{
			name:       "Delete provider as viewer (should fail)",
			method:     "DELETE",
			path:       "/api/providers/test-provider",
			token:      viewerToken,
			expectCode: 403,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			rr := httptest.NewRecorder()
			srv.ginEngine().ServeHTTP(rr, req)

			if rr.Code != tt.expectCode {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectCode, rr.Code, rr.Body.String())
			}
		})
	}
}

// Test Schema Management APIs
func TestSchemaManagementAPIs(t *testing.T) {
	srv, _ := setupTestServer(t)

	adminToken, _ := srv.jwtMgr.Sign("admin", []string{"admin"}, 0)
	developerToken, _ := srv.jwtMgr.Sign("dev", []string{"developer"}, 0)

	// Sample schema for testing
	testSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
			"age": map[string]interface{}{
				"type": "integer",
				"minimum": 0,
			},
		},
		"required": []string{"name"},
	}
	schemaJSON, _ := json.Marshal(testSchema)

	tests := []struct {
		name       string
		method     string
		path       string
		token      string
		body       string
		expectCode int
	}{
		// POST /api/schemas - Create schema
		{
			name:       "Create schema as admin",
			method:     "POST",
			path:       "/api/schemas",
			token:      adminToken,
			body:       string(schemaJSON),
			expectCode: 201,
		},
		{
			name:       "Create schema as developer",
			method:     "POST",
			path:       "/api/schemas",
			token:      developerToken,
			body:       string(schemaJSON),
			expectCode: 201,
		},
		{
			name:       "Create schema without auth",
			method:     "POST",
			path:       "/api/schemas",
			token:      "",
			body:       string(schemaJSON),
			expectCode: 401,
		},
		{
			name:       "Create invalid schema",
			method:     "POST",
			path:       "/api/schemas",
			token:      adminToken,
			body:       `{"invalid":"schema"}`,
			expectCode: 400,
		},

		// GET /api/schemas - List schemas
		{
			name:       "List schemas as admin",
			method:     "GET",
			path:       "/api/schemas",
			token:      adminToken,
			expectCode: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != "" {
				body = bytes.NewBufferString(tt.body)
			} else {
				body = nil
			}

			req := httptest.NewRequest(tt.method, tt.path, body)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			rr := httptest.NewRecorder()
			srv.ginEngine().ServeHTTP(rr, req)

			if rr.Code != tt.expectCode {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectCode, rr.Code, rr.Body.String())
			}
		})
	}
}

// Test X-Render Integration APIs
func TestXRenderAPIs(t *testing.T) {
	srv, _ := setupTestServer(t)

	adminToken, _ := srv.jwtMgr.Sign("admin", []string{"admin"}, 0)
	developerToken, _ := srv.jwtMgr.Sign("dev", []string{"developer"}, 0)

	tests := []struct {
		name       string
		method     string
		path       string
		token      string
		expectCode int
	}{
		// GET /api/x-render/components - List available components
		{
			name:       "List x-render components as admin",
			method:     "GET",
			path:       "/api/x-render/components",
			token:      adminToken,
			expectCode: 200,
		},
		{
			name:       "List x-render components as developer",
			method:     "GET",
			path:       "/api/x-render/components",
			token:      developerToken,
			expectCode: 200,
		},
		{
			name:       "List x-render components without auth",
			method:     "GET",
			path:       "/api/x-render/components",
			token:      "",
			expectCode: 401,
		},

		// GET /api/x-render/templates - List available templates
		{
			name:       "List x-render templates as admin",
			method:     "GET",
			path:       "/api/x-render/templates",
			token:      adminToken,
			expectCode: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, (*bytes.Buffer)(nil))
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			rr := httptest.NewRecorder()
			srv.ginEngine().ServeHTTP(rr, req)

			if rr.Code != tt.expectCode {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectCode, rr.Code, rr.Body.String())
			}

			// For successful GET requests, verify response structure
			if tt.expectCode == 200 && tt.method == "GET" {
				var response interface{}
				if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
					t.Errorf("Failed to parse JSON response: %v", err)
				}
			}
		})
	}
}

// Test Component Details Enhancement APIs
func TestComponentDetailsAPIs(t *testing.T) {
	srv, _ := setupTestServer(t)

	adminToken, _ := srv.jwtMgr.Sign("admin", []string{"admin"}, 0)
	techLeadToken, _ := srv.jwtMgr.Sign("techlead", []string{"tech_lead"}, 0)

	tests := []struct {
		name       string
		method     string
		path       string
		token      string
		body       string
		expectCode int
	}{
		// GET /api/components/:id - Get component details
		{
			name:       "Get component details as tech lead",
			method:     "GET",
			path:       "/api/components/test-component",
			token:      techLeadToken,
			expectCode: 404, // Expected since we don't have actual components
		},
		{
			name:       "Get component details as admin",
			method:     "GET",
			path:       "/api/components/test-component",
			token:      adminToken,
			expectCode: 404, // Expected since we don't have actual components
		},

		// PATCH /api/components/:id - Update component config
		{
			name:       "Update component config as admin",
			method:     "PATCH",
			path:       "/api/components/test-component",
			token:      adminToken,
			body:       `{"displayConfig":{"enabled":true}}`,
			expectCode: 404, // Expected since we don't have actual components
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body *bytes.Buffer = nil
			if tt.body != "" {
				body = bytes.NewBufferString(tt.body)
			}

			req := httptest.NewRequest(tt.method, tt.path, body)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			rr := httptest.NewRecorder()
			srv.ginEngine().ServeHTTP(rr, req)

			if rr.Code != tt.expectCode {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectCode, rr.Code, rr.Body.String())
			}
		})
	}
}

// Test RBAC permissions for all new endpoints
func TestNewAPIsRBACPermissions(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Skip if RBAC not properly configured
	if srv.rbac == nil {
		t.Skip("RBAC not configured, skipping authorization tests")
	}

	// Test different role permissions
	roles := map[string][]string{
		"admin":     {"admin"},
		"tech_lead": {"tech_lead"},
		"developer": {"developer"},
		"viewer":    {"viewer"},
	}

	// Define expected permissions for each role and endpoint
	permissionTests := []struct {
		endpoint string
		method   string
		roles    map[string]bool // role -> should_have_access
	}{
		{"/api/functions", "GET", map[string]bool{
			"admin": true, "tech_lead": true, "developer": true, "viewer": true,
		}},
		{"/api/functions/test-func", "GET", map[string]bool{
			"admin": true, "tech_lead": true, "developer": true, "viewer": false,
		}},
		{"/api/functions/test-func/enable", "PATCH", map[string]bool{
			"admin": true, "tech_lead": false, "developer": false, "viewer": false,
		}},
		{"/api/providers", "GET", map[string]bool{
			"admin": true, "tech_lead": true, "developer": false, "viewer": false,
		}},
		{"/api/providers/test", "DELETE", map[string]bool{
			"admin": true, "tech_lead": false, "developer": false, "viewer": false,
		}},
		{"/api/schemas", "POST", map[string]bool{
			"admin": true, "tech_lead": false, "developer": true, "viewer": false,
		}},
		{"/api/x-render/components", "GET", map[string]bool{
			"admin": true, "tech_lead": true, "developer": true, "viewer": false,
		}},
	}

	for _, test := range permissionTests {
		for roleName, roleList := range roles {
			shouldHaveAccess := test.roles[roleName]

			t.Run(roleName+"_"+test.method+"_"+test.endpoint, func(t *testing.T) {
				token, _ := srv.jwtMgr.Sign("user", roleList, 0)

				req := httptest.NewRequest(test.method, test.endpoint, nil)
				req.Header.Set("Authorization", "Bearer "+token)
				if test.method == "POST" || test.method == "PATCH" {
					req.Header.Set("Content-Type", "application/json")
				}

				rr := httptest.NewRecorder()
				srv.ginEngine().ServeHTTP(rr, req)

				if shouldHaveAccess {
					// Should not be 403 Forbidden
					if rr.Code == 403 {
						t.Errorf("Role %s should have access to %s %s, but got 403", roleName, test.method, test.endpoint)
					}
				} else {
					// Should be 403 Forbidden
					if rr.Code != 403 {
						t.Errorf("Role %s should NOT have access to %s %s, expected 403 but got %d", roleName, test.method, test.endpoint, rr.Code)
					}
				}
			})
		}
	}
}

// Test response formats and data structure
func TestNewAPIsResponseFormats(t *testing.T) {
	srv, _ := setupTestServer(t)

	adminToken, _ := srv.jwtMgr.Sign("admin", []string{"admin"}, 0)

	tests := []struct {
		name             string
		endpoint         string
		expectedFields   []string
		responseIsArray  bool
	}{
		{
			name:            "Functions list response format",
			endpoint:        "/api/functions",
			expectedFields:  []string{"functions"},
			responseIsArray: false,
		},
		{
			name:            "Providers list response format",
			endpoint:        "/api/providers",
			expectedFields:  []string{"providers"},
			responseIsArray: false,
		},
		{
			name:            "X-Render components response format",
			endpoint:        "/api/x-render/components",
			expectedFields:  []string{"components"},
			responseIsArray: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", test.endpoint, nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)

			rr := httptest.NewRecorder()
			srv.ginEngine().ServeHTTP(rr, req)

			if rr.Code != 200 {
				t.Errorf("Expected status 200, got %d", rr.Code)
				return
			}

			var response map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Errorf("Failed to parse JSON response: %v", err)
				return
			}

			// Verify expected fields exist
			for _, field := range test.expectedFields {
				if _, exists := response[field]; !exists {
					t.Errorf("Expected field '%s' not found in response", field)
				}
			}
		})
	}
}
