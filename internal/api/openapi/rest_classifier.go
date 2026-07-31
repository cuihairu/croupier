package openapi

import (
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// classifyRESTCapability infers capability from HTTP method and path patterns.
// This is used when x-capability extension is not provided.
// Returns empty string if no reliable classification can be made.
func classifyRESTCapability(method, path string) spec.CapabilityKind {
	method = strings.ToUpper(method)
	path = strings.TrimRight(path, "/")

	// Extract path parameters
	hasPathParam := strings.Contains(path, "{")

	// Get base resource path (without path params)
	basePath := path
	if idx := strings.Index(path, "{"); idx > 0 {
		basePath = strings.TrimRight(path[:idx], "/")
	}

	switch method {
	case "GET":
		if hasPathParam {
			// GET /players/{id} → item_query
			return spec.CapabilityItemQuery
		}
		// GET /players → collection_query
		if basePath != "" {
			return spec.CapabilityCollectionQuery
		}
	case "POST":
		// POST /players → create
		if !hasPathParam && basePath != "" {
			return spec.CapabilityCreate
		}
		// POST /players/{id}/actions → action (needs more context)
		if hasPathParam && strings.Contains(path, "/actions") {
			return spec.CapabilityAction
		}
	case "PUT":
		// PUT /players/{id} → update
		if hasPathParam {
			return spec.CapabilityUpdate
		}
	case "PATCH":
		// PATCH /players/{id} → update
		if hasPathParam {
			return spec.CapabilityUpdate
		}
	case "DELETE":
		// DELETE /players/{id} → delete
		if hasPathParam {
			return spec.CapabilityDelete
		}
	}

	return ""
}

// inferIdentityFieldFromPath extracts the identity field name from a path template.
// e.g., "/players/{player_id}" → "player_id"
// e.g., "/api/v1/orders/{orderId}" → "orderId"
func inferIdentityFieldFromPath(path string) string {
	start := strings.LastIndex(path, "{")
	end := strings.LastIndex(path, "}")
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return path[start+1 : end]
}

// inferResourceFromPath extracts the resource key from a path.
// e.g., "/players" → "players"
// e.g., "/api/v1/players/{id}" → "players"
// e.g., "/player-actions" → "player-actions"
func inferResourceFromPath(path string) string {
	// Remove trailing slash
	path = strings.TrimRight(path, "/")

	// Remove path parameters
	if idx := strings.Index(path, "{"); idx > 0 {
		path = path[:idx]
		path = strings.TrimRight(path, "/")
	}

	// Get last segment
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return ""
	}

	// Return last non-empty segment
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return ""
}

// restClassificationDiagnostic creates a diagnostic for REST classification.
func restClassificationDiagnostic(method, path string, capability spec.CapabilityKind) spec.Diagnostic {
	return spec.Diagnostic{
		Code:     "rest_capability_inferred",
		Severity: spec.SeverityInfo,
		Message:  "capability inferred from REST method/path: " + string(capability),
		Field:    "x-capability",
	}
}
