package openapi

import (
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
)

func TestRESTCapabilityClassifier_ClassifyOperation(t *testing.T) {
	classifier := NewRESTCapabilityClassifier()

	tests := []struct {
		name            string
		method          string
		path            string
		hasRequestBody  bool
		responseIsArray bool
		wantCapability  spec.CapabilityKind
		wantConfidence  string
		wantResource    string
	}{
		// Collection query patterns
		{
			name:           "GET /players -> collection_query",
			method:         "GET",
			path:           "/players",
			wantCapability: spec.CapabilityCollectionQuery,
			wantConfidence: "high",
			wantResource:   "players",
		},
		{
			name:           "GET /api/v1/players -> collection_query",
			method:         "GET",
			path:           "/api/v1/players",
			wantCapability: spec.CapabilityCollectionQuery,
			wantConfidence: "high",
			wantResource:   "players",
		},

		// Item query patterns
		{
			name:           "GET /players/{playerId} -> item_query",
			method:         "GET",
			path:           "/players/{playerId}",
			wantCapability: spec.CapabilityItemQuery,
			wantConfidence: "high",
			wantResource:   "players",
		},
		{
			name:           "GET /orders/{orderId} -> item_query",
			method:         "GET",
			path:           "/orders/{orderId}",
			wantCapability: spec.CapabilityItemQuery,
			wantConfidence: "high",
			wantResource:   "orders",
		},

		// Create patterns
		{
			name:           "POST /players -> create",
			method:         "POST",
			path:           "/players",
			hasRequestBody: true,
			wantCapability: spec.CapabilityCreate,
			wantConfidence: "high",
			wantResource:   "players",
		},

		// Update patterns
		{
			name:           "PUT /players/{playerId} -> update",
			method:         "PUT",
			path:           "/players/{playerId}",
			hasRequestBody: true,
			wantCapability: spec.CapabilityUpdate,
			wantConfidence: "high",
			wantResource:   "players",
		},
		{
			name:           "PATCH /players/{playerId} -> update",
			method:         "PATCH",
			path:           "/players/{playerId}",
			hasRequestBody: true,
			wantCapability: spec.CapabilityUpdate,
			wantConfidence: "high",
			wantResource:   "players",
		},

		// Delete patterns
		{
			name:           "DELETE /players/{playerId} -> delete",
			method:         "DELETE",
			path:           "/players/{playerId}",
			wantCapability: spec.CapabilityDelete,
			wantConfidence: "high",
			wantResource:   "players",
		},

		// Action patterns
		{
			name:           "POST /players/{playerId}/ban -> action",
			method:         "POST",
			path:           "/players/{playerId}/ban",
			wantCapability: spec.CapabilityAction,
			wantConfidence: "low",
			wantResource:   "players",
		},
		{
			name:           "GET nested resource path -> action",
			method:         "GET",
			path:           "/players/{playerId}/inventory",
			wantCapability: spec.CapabilityAction,
			wantConfidence: "low",
			wantResource:   "players",
		},
		{
			name:           "DELETE /players -> batch action",
			method:         "DELETE",
			path:           "/players",
			wantCapability: spec.CapabilityAction,
			wantConfidence: "low",
			wantResource:   "players",
		},

		// Edge cases
		{
			name:           "empty path -> action low confidence",
			method:         "GET",
			path:           "",
			wantCapability: spec.CapabilityAction,
			wantConfidence: "low",
			wantResource:   "",
		},
		{
			name:           "UNKNOWN method -> action low confidence",
			method:         "OPTIONS",
			path:           "/players",
			wantCapability: spec.CapabilityAction,
			wantConfidence: "low",
			wantResource:   "players",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.ClassifyOperation(
				tt.method,
				tt.path,
				tt.hasRequestBody,
				tt.responseIsArray,
			)

			assert.Equal(t, tt.wantCapability, result.Capability)
			assert.Equal(t, tt.wantConfidence, result.Confidence)
			assert.Equal(t, tt.wantResource, result.ResourceKey)
		})
	}
}

func TestRESTCapabilityClassifier_DoesNotUseOperationID(t *testing.T) {
	classifier := NewRESTCapabilityClassifier()

	// Verify classifier only uses method/path, never operationId
	// This ensures we don't guess CRUD from operationId like "getPlayer" or "listUsers"
	tests := []struct {
		name           string
		method         string
		path           string
		wantCapability spec.CapabilityKind
	}{
		{
			name:           "GET /data with operationId=getPlayer -> collection_query (not item_query)",
			method:         "GET",
			path:           "/data",
			wantCapability: spec.CapabilityCollectionQuery,
		},
		{
			name:           "POST /data with operationId=updateItem -> create (not update)",
			method:         "POST",
			path:           "/data",
			wantCapability: spec.CapabilityCreate,
		},
		{
			name:           "PUT /data/{id} with operationId=createPlayer -> update (not create)",
			method:         "PUT",
			path:           "/data/{id}",
			wantCapability: spec.CapabilityUpdate,
		},
		{
			name:           "DELETE /data/{id} with operationId=getItem -> delete (not item_query)",
			method:         "DELETE",
			path:           "/data/{id}",
			wantCapability: spec.CapabilityDelete,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.ClassifyOperation(tt.method, tt.path, false, false)
			assert.Equal(t, tt.wantCapability, result.Capability)
		})
	}
}

func TestRESTCapabilityClassifier_ProducesOnlyCapabilitySemantics(t *testing.T) {
	classifier := NewRESTCapabilityClassifier()

	result := classifier.ClassifyOperation("GET", "/players", false, true)

	// Verify result only contains capability semantics
	assert.NotEmpty(t, result.Capability)
	assert.NotEmpty(t, result.ResourceKey)
	assert.NotEmpty(t, result.Confidence)

	// Verify no UI/layout fields
	assert.Nil(t, result.Diagnostics) // No diagnostics for clean classification
}

func TestExtractPathSegments(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"", nil},
		{"/", nil},
		{"/players", []string{"players"}},
		{"/players/{playerId}", []string{"players", "{playerId}"}},
		{"/api/v1/players/{playerId}/stats", []string{"api", "v1", "players", "{playerId}", "stats"}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := extractPathSegments(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsPathParameter(t *testing.T) {
	tests := []struct {
		name    string
		segment string
		want    bool
	}{
		{"path parameter", "{playerId}", true},
		{"plain text", "players", false},
		{"empty", "", false},
		{"open brace only", "{playerId", false},
		{"close brace only", "playerId}", false},
		{"nested braces", "{{id}}", true},
		{"simple param", "{id}", true},
		{"param with spaces", "{ player id }", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPathParameter(tt.segment)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStripAPIVersionPrefix(t *testing.T) {
	tests := []struct {
		name     string
		segments []string
		want     []string
	}{
		{"with api version", []string{"api", "v1", "players"}, []string{"players"}},
		{"with api V2", []string{"api", "V2", "players"}, []string{"players"}},
		{"without api prefix", []string{"players", "stats"}, []string{"players", "stats"}},
		{"api only", []string{"api"}, []string{"api"}},
		{"api with short version", []string{"api", "v", "players"}, []string{"api", "v", "players"}},
		{"empty", nil, nil},
		{"single element", []string{"api"}, []string{"api"}},
		{"two elements", []string{"api", "v1"}, []string{"api", "v1"}},
		{"api with numeric version", []string{"api", "1", "players"}, []string{"api", "1", "players"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripAPIVersionPrefix(tt.segments)
			assert.Equal(t, tt.want, got)
		})
	}
}
