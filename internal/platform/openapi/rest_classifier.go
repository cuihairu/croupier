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

	// Only classify unambiguous REST collection/item paths. Version prefixes do
	// not identify a resource and nested paths remain actions for review.
	segments := stripAPIVersionPrefix(extractPathSegments(path))
	if len(segments) == 0 {
		return ClassificationResult{
			Capability:  spec.CapabilityAction,
			Confidence:  "low",
			Diagnostics: []spec.Diagnostic{{Code: "empty_path", Severity: spec.SeverityWarning, Message: "Empty API path"}},
		}
	}

	// The first non-version segment is the resource key. Only /resource and
	// /resource/{id} are lifecycle candidates; deeper paths are actions.
	resourceKey := segments[0]
	isCollection := len(segments) == 1
	isItem := len(segments) == 2 && isPathParameter(segments[1])

	// Classify based on method and path pattern
	switch method {
	case "GET":
		if isItem {
			return ClassificationResult{
				Capability:  spec.CapabilityItemQuery,
				ResourceKey: resourceKey,
				Confidence:  "high",
			}
		}
		if isCollection {
			return ClassificationResult{Capability: spec.CapabilityCollectionQuery, ResourceKey: resourceKey, Confidence: "high"}
		}

	case "POST":
		if isCollection {
			return ClassificationResult{Capability: spec.CapabilityCreate, ResourceKey: resourceKey, Confidence: "high"}
		}

	case "PUT", "PATCH":
		if isItem {
			return ClassificationResult{
				Capability:  spec.CapabilityUpdate,
				ResourceKey: resourceKey,
				Confidence:  "high",
			}
		}

	case "DELETE":
		if isItem {
			return ClassificationResult{
				Capability:  spec.CapabilityDelete,
				ResourceKey: resourceKey,
				Confidence:  "high",
			}
		}
	}

	return ClassificationResult{
		Capability:  spec.CapabilityAction,
		ResourceKey: resourceKey,
		Confidence:  "low",
		Diagnostics: []spec.Diagnostic{{
			Code:     "rest_shape_ambiguous",
			Severity: spec.SeverityWarning,
			Message:  "method/path is not an unambiguous REST lifecycle operation",
		}},
	}
}

func isPathParameter(segment string) bool {
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")
}

func stripAPIVersionPrefix(segments []string) []string {
	if len(segments) >= 3 && strings.EqualFold(segments[0], "api") && len(segments[1]) > 1 &&
		(segments[1][0] == 'v' || segments[1][0] == 'V') {
		return segments[2:]
	}
	return segments
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
