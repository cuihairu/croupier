package openapi

import (
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// RESTCapabilityClassifier classifies OpenAPI operations into capability semantics
// based on HTTP method, path pattern, and schema structure.
//
// Classification rules:
//   - GET /{resource} -> collection_query
//   - GET /{resource}/{id} -> item_query
//   - POST /{resource} -> create
//   - PUT/PATCH /{resource}/{id} -> update
//   - DELETE /{resource}/{id} -> delete
//   - Other patterns -> action (low confidence)
//
// The classifier only produces capability semantics, not page layout or UI configuration.
type RESTCapabilityClassifier struct{}

// NewRESTCapabilityClassifier creates a new REST capability classifier.
func NewRESTCapabilityClassifier() *RESTCapabilityClassifier {
	return &RESTCapabilityClassifier{}
}

// ClassificationResult represents the result of classifying an OpenAPI operation.
type ClassificationResult struct {
	// Capability is the classified capability kind.
	Capability spec.CapabilityKind `json:"capability"`

	// ResourceKey is the extracted resource key from the path.
	ResourceKey string `json:"resourceKey"`

	// Confidence indicates how confident the classification is.
	// "high" = standard REST pattern matched
	// "low" = fallback to action
	Confidence string `json:"confidence"`

	// Diagnostics contains any warnings or issues found during classification.
	Diagnostics []spec.Diagnostic `json:"diagnostics,omitempty"`
}

// ClassifyOperation classifies an OpenAPI operation into a capability kind.
//
// Parameters:
//   - method: HTTP method (GET, POST, PUT, PATCH, DELETE)
//   - path: API path (e.g., "/players", "/players/{playerId}")
//   - hasRequestBody: whether the operation has a request body
//   - responseIsArray: whether the response schema is an array or contains an array field
func (c *RESTCapabilityClassifier) ClassifyOperation(
	method string,
	path string,
	hasRequestBody bool,
	responseIsArray bool,
) ClassificationResult {
	method = strings.ToUpper(method)
	path = strings.TrimSuffix(path, "/")

	// Extract path segments
	segments := extractPathSegments(path)
	if len(segments) == 0 {
		return ClassificationResult{
			Capability:  spec.CapabilityAction,
			Confidence:  "low",
			Diagnostics: []spec.Diagnostic{{Code: "empty_path", Severity: spec.SeverityWarning, Message: "Empty API path"}},
		}
	}

	// Check for path parameters (e.g., {playerId})
	hasPathParam := false
	pathParamName := ""
	for _, seg := range segments {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			hasPathParam = true
			pathParamName = strings.TrimPrefix(strings.TrimSuffix(seg, "}"), "{")
			break
		}
	}

	// Extract resource key from first segment
	resourceKey := segments[0]

	// Classify based on method and path pattern
	switch method {
	case "GET":
		if hasPathParam {
			// GET /{resource}/{id} -> item_query
			return ClassificationResult{
				Capability:  spec.CapabilityItemQuery,
				ResourceKey: resourceKey,
				Confidence:  "high",
			}
		}
		// GET /{resource} -> collection_query
		return ClassificationResult{
			Capability:  spec.CapabilityCollectionQuery,
			ResourceKey: resourceKey,
			Confidence:  "high",
		}

	case "POST":
		if hasPathParam {
			// POST /{resource}/{id} -> action (e.g., POST /players/{id}/ban)
			return ClassificationResult{
				Capability:  spec.CapabilityAction,
				ResourceKey: resourceKey,
				Confidence:  "medium",
				Diagnostics: []spec.Diagnostic{{
					Code:     "post_with_path_param",
					Severity: spec.SeverityInfo,
					Message:  "POST with path parameter classified as action: " + pathParamName,
				}},
			}
		}
		// POST /{resource} -> create
		return ClassificationResult{
			Capability:  spec.CapabilityCreate,
			ResourceKey: resourceKey,
			Confidence:  "high",
		}

	case "PUT", "PATCH":
		if hasPathParam {
			// PUT/PATCH /{resource}/{id} -> update
			return ClassificationResult{
				Capability:  spec.CapabilityUpdate,
				ResourceKey: resourceKey,
				Confidence:  "high",
			}
		}
		// PUT/PATCH /{resource} -> action (unusual pattern)
		return ClassificationResult{
			Capability:  spec.CapabilityAction,
			ResourceKey: resourceKey,
			Confidence:  "low",
			Diagnostics: []spec.Diagnostic{{
				Code:     "put_without_path_param",
				Severity: spec.SeverityWarning,
				Message:  "PUT/PATCH without path parameter, classified as action",
			}},
		}

	case "DELETE":
		if hasPathParam {
			// DELETE /{resource}/{id} -> delete
			return ClassificationResult{
				Capability:  spec.CapabilityDelete,
				ResourceKey: resourceKey,
				Confidence:  "high",
			}
		}
		// DELETE /{resource} -> action (batch delete)
		return ClassificationResult{
			Capability:  spec.CapabilityAction,
			ResourceKey: resourceKey,
			Confidence:  "medium",
			Diagnostics: []spec.Diagnostic{{
				Code:     "delete_without_path_param",
				Severity: spec.SeverityInfo,
				Message:  "DELETE without path parameter, classified as batch action",
			}},
		}

	default:
		// Unknown method -> action
		return ClassificationResult{
			Capability:  spec.CapabilityAction,
			ResourceKey: resourceKey,
			Confidence:  "low",
			Diagnostics: []spec.Diagnostic{{
				Code:     "unknown_method",
				Severity: spec.SeverityWarning,
				Message:  "Unknown HTTP method: " + method,
			}},
		}
	}
}

// extractPathSegments extracts non-empty segments from a path.
// e.g., "/players/{playerId}/stats" -> ["players", "{playerId}", "stats"]
func extractPathSegments(path string) []string {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return nil
	}
	parts := strings.Split(path, "/")
	var segments []string
	for _, p := range parts {
		if p != "" {
			segments = append(segments, p)
		}
	}
	return segments
}
