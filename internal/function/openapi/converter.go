package openapi

import (
	"encoding/json"
	"fmt"
	"strings"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"github.com/getkin/kin-openapi/openapi3"
)

// Converter converts between FunctionMetadata and OpenAPI 3.0.3 specification.
type Converter struct {
	mapper *SchemaMapper
}

// NewConverter creates a new Converter instance.
func NewConverter() *Converter {
	return &Converter{
		mapper: NewSchemaMapper(),
	}
}

// ImportFromSpecData converts an OpenAPI 3.0.3 specification JSON data to FunctionMetadata list.
// This enables quick registration from third-party API specs.
func (c *Converter) ImportFromSpecData(specData []byte, options *ImportOptions) ([]*functionv1.FunctionMetadata, error) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(specData)
	if err != nil {
		return nil, fmt.Errorf("load OpenAPI spec: %w", err)
	}
	return c.ImportFromSpec(doc, options)
}

// ImportFromSpec converts an OpenAPI 3.0.3 specification to FunctionMetadata list.
// This enables quick registration from third-party API specs.
func (c *Converter) ImportFromSpec(spec *openapi3.T, options *ImportOptions) ([]*functionv1.FunctionMetadata, error) {
	if spec == nil {
		return nil, fmt.Errorf("spec is required")
	}

	var metadatas []*functionv1.FunctionMetadata

	for path, pathItem := range spec.Paths.Map() {
		if pathItem == nil {
			continue
		}

		// Process operations (GET, POST, PUT, DELETE, etc.)
		operations := pathItem.Operations()
		for _, op := range operations {
			if op == nil {
				continue
			}

			metadata, err := c.operationToMetadata(path, op, options)
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

// ExportToSpec converts FunctionMetadata list to OpenAPI 3.0.3 specification.
func (c *Converter) ExportToSpec(metadatas []*functionv1.FunctionMetadata) (*openapi3.T, error) {
	doc := &openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title:       "Croupier Functions",
			Description: "Auto-generated OpenAPI specification from registered functions",
			Version:     "1.0.0",
		},
		Paths: openapi3.NewPaths(),
	}

	for _, metadata := range metadatas {
		if metadata == nil {
			continue
		}

		op := c.metadataToOperation(metadata)
		if op == nil {
			continue
		}

		// Use function ID as path for consistency
		path := fmt.Sprintf("/functions/%s", metadata.Id)
		pathItem := &openapi3.PathItem{
			Post: op,
		}

		doc.Paths.Set(path, pathItem)
	}

	return doc, nil
}

// ImportToMetadata converts a single OpenAPI operation to FunctionMetadata.
func (c *Converter) ImportToMetadata(operationID string, operation *openapi3.Operation) (*functionv1.FunctionMetadata, error) {
	if operation == nil {
		return nil, fmt.Errorf("operation is required")
	}

	return c.operationToMetadata("", operation, nil)
}

// MetadataToOperation converts a single FunctionMetadata to OpenAPI operation.
func (c *Converter) MetadataToOperation(metadata *functionv1.FunctionMetadata) (*openapi3.Operation, error) {
	if metadata == nil {
		return nil, fmt.Errorf("metadata is required")
	}

	op := c.metadataToOperation(metadata)
	if op == nil {
		return nil, fmt.Errorf("failed to convert metadata to operation")
	}

	return op, nil
}

// operationToMetadata converts an OpenAPI operation to FunctionMetadata.
func (c *Converter) operationToMetadata(path string, op *openapi3.Operation, options *ImportOptions) (*functionv1.FunctionMetadata, error) {
	metadata := &functionv1.FunctionMetadata{
		Id:          deriveFunctionID(op, path),
		Name:        deriveName(op),
		Description: op.Description,
		Tags:        op.Tags,
	}

	// Get summary from operation or generate from ID
	if op.Summary != "" {
		metadata.Name = op.Summary
	}

	// Extract input schema from request body
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		for contentType, mt := range op.RequestBody.Value.Content {
			if contentType == "application/json" && mt.Schema != nil {
				inputSchema, err := c.mapper.OpenAPIToJSONSchema(mt.Schema.Value)
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
						outputSchema, err := c.mapper.OpenAPIToJSONSchema(mt.Schema.Value)
						if err == nil {
							metadata.OutputSchema = outputSchema
						}
						break
					}
				}
			}
		}
	}

	// Extract capability extensions. UI/page extensions are rejected at the
	// registration boundary; only executable capability metadata belongs here.
	metadata.Resource = c.extractExtension(op.Extensions, "x-resource")
	riskLevel := c.extractExtension(op.Extensions, "x-risk")
	permission := c.extractExtension(op.Extensions, "x-permission")

	// Set default values
	if riskLevel == "" {
		riskLevel = "medium"
	}

	// Build behavior
	metadata.Behavior = c.deriveBehavior(op, options)

	// Build security
	metadata.Security = &functionv1.FunctionSecurity{
		RiskLevel:        parseRiskLevel(riskLevel),
		Permission:       permission,
		RequiresApproval: riskLevel == "high" || riskLevel == "danger",
		AuditLog:         true,
	}

	return metadata, nil
}

// metadataToOperation converts FunctionMetadata to OpenAPI operation.
func (c *Converter) metadataToOperation(metadata *functionv1.FunctionMetadata) *openapi3.Operation {
	op := &openapi3.Operation{
		OperationID: metadata.Id,
		Summary:     metadata.Name,
		Description: metadata.Description,
		Tags:        metadata.Tags,
		Extensions:  make(map[string]interface{}),
	}

	// Add extensions
	if metadata.Resource != "" {
		op.Extensions["x-resource"] = metadata.Resource
	}

	if metadata.Security != nil {
		riskStr := normalizeRiskLevel(metadata.Security.RiskLevel)
		op.Extensions["x-risk"] = riskStr
		if metadata.Security.Permission != "" {
			op.Extensions["x-permission"] = metadata.Security.Permission
		}
	}

	// Build request body from input schema
	if metadata.InputSchema != "" {
		requestSchema := c.mapper.JSONSchemaToOpenAPI(metadata.InputSchema)
		if requestSchema != nil {
			op.RequestBody = &openapi3.RequestBodyRef{
				Value: &openapi3.RequestBody{
					Description: "Request payload",
					Required:    true,
					Content: openapi3.Content{
						"application/json": &openapi3.MediaType{
							Schema: requestSchema,
						},
					},
				},
			}
		}
	}

	// Build response from output schema
	if metadata.OutputSchema != "" {
		responseSchema := c.mapper.JSONSchemaToOpenAPI(metadata.OutputSchema)
		if responseSchema != nil {
			desc := "Response payload"
			op.Responses = openapi3.NewResponses()
			op.Responses.Set("200", &openapi3.ResponseRef{
				Value: &openapi3.Response{
					Description: &desc,
					Content: openapi3.Content{
						"application/json": &openapi3.MediaType{
							Schema: responseSchema,
						},
					},
				},
			})
		}
	}

	return op
}

// deriveBehavior derives FunctionBehavior from OpenAPI operation.
func (c *Converter) deriveBehavior(op *openapi3.Operation, options *ImportOptions) *functionv1.FunctionBehavior {
	behavior := &functionv1.FunctionBehavior{
		Mode:          functionv1.FunctionBehavior_MODE_QUERY,
		Idempotent:    false,
		TimeoutMs:     30000,
		RouteStrategy: functionv1.FunctionBehavior_ROUTE_STRATEGY_LB,
	}

	// Derive mode from extensions
	if modeStr := c.extractExtension(op.Extensions, "x-mode"); modeStr != "" {
		switch strings.ToLower(modeStr) {
		case "query", "read":
			behavior.Mode = functionv1.FunctionBehavior_MODE_QUERY
		case "command", "write":
			behavior.Mode = functionv1.FunctionBehavior_MODE_COMMAND
		}
	}
	// Note: We can't check HTTP method here because openapi3.Operation
	// doesn't expose its method (it's implicit in the PathItem)

	// Check idempotency
	if idempotentStr := c.extractExtension(op.Extensions, "x-idempotent"); idempotentStr != "" {
		behavior.Idempotent = strings.ToLower(idempotentStr) == "true"
	}

	// Apply options defaults
	if options != nil {
		if options.DefaultTimeoutMs > 0 {
			behavior.TimeoutMs = options.DefaultTimeoutMs
		}
		if options.DefaultRouteStrategy != "" {
			behavior.RouteStrategy = parseRouteStrategy(options.DefaultRouteStrategy)
		}
	}

	return behavior
}

// extractExtension extracts an extension value from the extensions map.
func (c *Converter) extractExtension(extensions map[string]interface{}, key string) string {
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

// deriveFunctionID derives a function ID from operation or path.
func deriveFunctionID(op *openapi3.Operation, path string) string {
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

// deriveName derives a display name from operation.
func deriveName(op *openapi3.Operation) string {
	if op != nil && op.Summary != "" {
		return op.Summary
	}
	if op != nil && op.OperationID != "" {
		// Convert operationId to title case: "player_ban" -> "Player Ban"
		return strings.ToUpper(op.OperationID[:1]) + strings.ReplaceAll(op.OperationID[1:], "_", " ")
	}
	return "Unnamed Function"
}

// parseRiskLevel parses a risk level string to enum.
func parseRiskLevel(level string) functionv1.FunctionSecurity_RiskLevel {
	switch strings.ToLower(level) {
	case "low", "safe":
		return functionv1.FunctionSecurity_RISK_LEVEL_LOW
	case "medium", "moderate":
		return functionv1.FunctionSecurity_RISK_LEVEL_MEDIUM
	case "high":
		return functionv1.FunctionSecurity_RISK_LEVEL_HIGH
	case "danger", "critical":
		return functionv1.FunctionSecurity_RISK_LEVEL_DANGER
	default:
		return functionv1.FunctionSecurity_RISK_LEVEL_MEDIUM
	}
}

// normalizeRiskLevel normalizes a risk level enum to string.
func normalizeRiskLevel(level functionv1.FunctionSecurity_RiskLevel) string {
	switch level {
	case functionv1.FunctionSecurity_RISK_LEVEL_LOW:
		return "low"
	case functionv1.FunctionSecurity_RISK_LEVEL_MEDIUM:
		return "medium"
	case functionv1.FunctionSecurity_RISK_LEVEL_HIGH:
		return "high"
	case functionv1.FunctionSecurity_RISK_LEVEL_DANGER:
		return "danger"
	default:
		return "medium"
	}
}

// parseRouteStrategy parses a route strategy string to enum.
func parseRouteStrategy(strategy string) functionv1.FunctionBehavior_RouteStrategy {
	switch strings.ToLower(strategy) {
	case "lb", "load_balance":
		return functionv1.FunctionBehavior_ROUTE_STRATEGY_LB
	case "broadcast":
		return functionv1.FunctionBehavior_ROUTE_STRATEGY_BROADCAST
	case "targeted":
		return functionv1.FunctionBehavior_ROUTE_STRATEGY_TARGETED
	case "hash", "consistent_hash":
		return functionv1.FunctionBehavior_ROUTE_STRATEGY_HASH
	default:
		return functionv1.FunctionBehavior_ROUTE_STRATEGY_LB
	}
}

// ImportOptions controls the import behavior.
type ImportOptions struct {
	// ContinueOnError continues processing even if some operations fail
	ContinueOnError bool

	// DefaultTimeoutMs is the default timeout for imported functions
	DefaultTimeoutMs int32

	// DefaultRouteStrategy is the default routing strategy
	DefaultRouteStrategy string

	// ResourcePrefix adds a prefix to imported resource keys.
	ResourcePrefix string

	// TagPrefix adds a prefix to all imported tags
	TagPrefix string
}
