// Package function provides OpenAPI import functionality for SDK users.
package function

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// RegisterFromOpenAPI registers functions from an OpenAPI 3.0.3 specification.
// It parses the spec, converts operations to FunctionMetadata, and registers them.
// The handlerFunc is called for each function ID to get the corresponding handler.
func (r *Registry) RegisterFromOpenAPI(specData []byte, options *ImportOptions, handlerFunc func(operationID string) Handler) error {
	// Load OpenAPI spec
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(specData)
	if err != nil {
		return fmt.Errorf("load OpenAPI spec failed: %w", err)
	}

	if err := doc.Validate(loader.Context); err != nil {
		return fmt.Errorf("validate OpenAPI spec failed: %w", err)
	}

	// Convert operations to metadata
	metadatas, err := r.openAPIToMetadata(doc, options)
	if err != nil {
		return fmt.Errorf("convert OpenAPI to metadata failed: %w", err)
	}

	// Register each metadata
	for _, metadata := range metadatas {
		handler := handlerFunc(metadata.ID)
		if handler == nil {
			if options != nil && options.ContinueOnError {
				continue
			}
			return fmt.Errorf("no handler provided for function: %s", metadata.ID)
		}

		if err := r.Register(metadata, handler); err != nil {
			if options != nil && options.ContinueOnError {
				r.logger.Warn("Failed to register function", "id", metadata.ID, "error", err)
				continue
			}
			return fmt.Errorf("register function %s failed: %w", metadata.ID, err)
		}
	}

	return nil
}

// RegisterFromOpenAPIWithHandlers registers functions from an OpenAPI 3.0.3 specification
// using a map of function IDs to handlers.
func (r *Registry) RegisterFromOpenAPIWithHandlers(specData []byte, options *ImportOptions, handlers map[string]Handler) error {
	return r.RegisterFromOpenAPI(specData, options, func(operationID string) Handler {
		return handlers[operationID]
	})
}

// openAPIToMetadata converts an OpenAPI 3.0.3 spec to FunctionMetadata list.
func (r *Registry) openAPIToMetadata(doc *openapi3.T, options *ImportOptions) ([]*FunctionMetadata, error) {
	var metadatas []*FunctionMetadata

	for path, pathItem := range doc.Paths.Map() {
		if pathItem == nil {
			continue
		}

		// Process operations (GET, POST, PUT, DELETE, etc.)
		operations := pathItem.Operations()
		for _, op := range operations {
			if op == nil {
				continue
			}

			metadata, err := r.operationToMetadata(path, op, options)
			if err != nil {
				if options != nil && options.ContinueOnError {
					continue
				}
				return nil, fmt.Errorf("convert operation %s failed: %w", op.OperationID, err)
			}

			metadatas = append(metadatas, metadata)
		}
	}

	return metadatas, nil
}

// operationToMetadata converts a single OpenAPI operation to FunctionMetadata.
func (r *Registry) operationToMetadata(path string, op *openapi3.Operation, options *ImportOptions) (*FunctionMetadata, error) {
	metadata := &FunctionMetadata{
		ID:          deriveOperationID(op, path),
		Name:        deriveOperationName(op),
		Description: op.Description,
		Tags:        op.Tags,
		Behavior: &FunctionBehavior{
			Mode:          ModeQuery,
			Idempotent:    false,
			TimeoutMs:     30000,
			RouteStrategy: RouteLB,
		},
		Risk: &FunctionRisk{
			Level: RiskMedium,
		},
	}

	// Get summary from operation or generate from ID
	if op.Summary != "" {
		metadata.Name = op.Summary
	}

	// Extract input schema from request body
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		for contentType, mt := range op.RequestBody.Value.Content {
			if contentType == "application/json" && mt.Schema != nil {
				inputSchema, err := schemaToJSONSchema(mt.Schema.Value)
				if err == nil {
					metadata.InputSchema = inputSchema
				}
				break
			}
		}
	}

	// Extract output schema from responses
	if op.Responses != nil {
		for code, respRef := range op.Responses.Map() {
			if code == "200" && respRef != nil && respRef.Value != nil {
				for contentType, mt := range respRef.Value.Content {
					if contentType == "application/json" && mt.Schema != nil {
						outputSchema, err := schemaToJSONSchema(mt.Schema.Value)
						if err == nil {
							metadata.OutputSchema = outputSchema
						}
						break
					}
				}
			}
		}
	}

	// Extract Croupier capability extensions.
	metadata.Resource = extractExtension(op.Extensions, "x-resource")
	metadata.Operation = extractExtension(op.Extensions, "x-operation")
	metadata.Permission = extractExtension(op.Extensions, "x-permission")
	riskLevel := extractExtension(op.Extensions, "x-risk")

	// Apply options
	if options != nil {
		if options.ResourcePrefix != "" && metadata.Resource != "" {
			metadata.Resource = options.ResourcePrefix + "." + metadata.Resource
		}
		if options.TagPrefix != "" {
			tags := make([]string, 0, len(metadata.Tags))
			for _, tag := range metadata.Tags {
				tags = append(tags, options.TagPrefix+tag)
			}
			metadata.Tags = tags
		}
		if options.DefaultTimeoutMs > 0 {
			metadata.Behavior.TimeoutMs = options.DefaultTimeoutMs
		}
	}

	// Set risk level from extension
	if riskLevel != "" {
		metadata.Risk.Level = parseRiskLevel(riskLevel)
	}

	return metadata, nil
}

// deriveOperationID derives a function ID from operation or path.
func deriveOperationID(op *openapi3.Operation, path string) string {
	if op != nil && op.OperationID != "" {
		return op.OperationID
	}

	// Generate from path: /api/players/{id} -> api.players.get
	if path != "" {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) > 0 {
			return strings.Join(parts, ".")
		}
	}

	return "unknown.function"
}

// deriveOperationName derives a display name from operation.
func deriveOperationName(op *openapi3.Operation) string {
	if op != nil && op.Summary != "" {
		return op.Summary
	}
	if op != nil && op.OperationID != "" {
		// Convert operationId to title case: "player_ban" -> "Player Ban"
		return toTitleCase(op.OperationID)
	}
	return "Unnamed Function"
}

// toTitleCase converts a string to title case.
func toTitleCase(s string) string {
	if s == "" {
		return ""
	}
	words := strings.Split(s, "_")
	for i, word := range words {
		if word != "" {
			if len(word) > 0 {
				words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
			}
		}
	}
	return strings.Join(words, " ")
}

// extractExtension extracts an extension value from the extensions map.
func extractExtension(extensions map[string]interface{}, key string) string {
	if extensions == nil {
		return ""
	}

	val, exists := extensions[key]
	if !exists {
		return ""
	}

	switch v := val.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case bool:
		return fmt.Sprintf("%t", v)
	case float64:
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// parseRiskLevel parses a risk level string to enum.
func parseRiskLevel(level string) RiskLevel {
	switch strings.ToLower(level) {
	case "low", "safe":
		return RiskLow
	case "medium", "moderate":
		return RiskMedium
	case "high":
		return RiskHigh
	case "danger", "critical":
		return RiskDanger
	default:
		return RiskMedium
	}
}

// schemaToJSONSchema converts an OpenAPI schema to JSON Schema string.
func schemaToJSONSchema(schema *openapi3.Schema) (string, error) {
	if schema == nil {
		return "{}", nil
	}

	// Build a simple JSON Schema representation
	result := map[string]interface{}{}

	if schema.Type != nil && len(*schema.Type) > 0 {
		result["type"] = (*schema.Type)[0]
	}

	if schema.Description != "" {
		result["description"] = schema.Description
	}

	if len(schema.Properties) > 0 {
		props := make(map[string]interface{})
		for name, ref := range schema.Properties {
			if ref != nil && ref.Value != nil {
				props[name] = map[string]interface{}{
					"type": schemaTypeToString(ref.Value),
				}
				if ref.Value.Description != "" {
					props[name].(map[string]interface{})["description"] = ref.Value.Description
				}
			}
		}
		result["properties"] = props
	}

	if schema.Required != nil && len(schema.Required) > 0 {
		result["required"] = schema.Required
	}

	// Convert to JSON
	bytes, err := json.Marshal(result)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

// schemaTypeToString converts OpenAPI schema type to string.
func schemaTypeToString(schema *openapi3.Schema) string {
	if schema == nil {
		return "object"
	}
	if schema.Type != nil && len(*schema.Type) > 0 {
		return (*schema.Type)[0]
	}
	return "object"
}
