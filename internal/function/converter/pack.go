package converter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/getkin/kin-openapi/openapi3"
)

// PackConverter converts Pack manifests to OpenAPI 3.0.3 Operations
type PackConverter struct {
	openapiConverter *OpenAPIConverter
}

// NewPackConverter creates a new Pack converter instance
func NewPackConverter() *PackConverter {
	return &PackConverter{
		openapiConverter: NewOpenAPIConverter(),
	}
}

// PackManifest represents the old Pack manifest format
type PackManifest struct {
	ID        string         `json:"id"`
	Version   string         `json:"version"`
	Name      string         `json:"name"`
	Provider  string         `json:"provider"`
	Functions []PackFunction `json:"functions"`
	Entities  []PackEntity   `json:"entities,omitempty"`
}

// PackFunction represents a function in the Pack manifest
type PackFunction struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Summary     string                 `json:"summary"`
	Description string                 `json:"description"`
	Params      map[string]interface{} `json:"params"`
	Returns     map[string]interface{} `json:"returns"`
	Resource    string                 `json:"resource,omitempty"`
	Risk        string                 `json:"risk,omitempty"`
	Operation   string                 `json:"operation,omitempty"`
}

// PackEntity represents an entity in the Pack manifest
type PackEntity struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Schema     map[string]interface{} `json:"schema"`
	Operations []PackEntityOperation  `json:"operations"`
}

// PackEntityOperation represents an entity operation
type PackEntityOperation struct {
	OP      string                 `json:"op"` // create/read/update/delete/custom
	Name    string                 `json:"name"`
	Params  map[string]interface{} `json:"params"`
	Returns map[string]interface{} `json:"returns"`
	Target  map[string]interface{} `json:"target,omitempty"`
}

// PackToOpenAPI converts a Pack manifest to OpenAPI 3.0.3 operations
func (c *PackConverter) PackToOpenAPI(manifest *PackManifest) (map[string]*openapi3.Operation, error) {
	operations := make(map[string]*openapi3.Operation)

	// Convert standalone functions
	for _, fn := range manifest.Functions {
		op, err := c.convertFunction(fn)
		if err != nil {
			return nil, fmt.Errorf("failed to convert function %s: %w", fn.ID, err)
		}
		operations[fn.ID] = op
	}

	// Convert entity operations
	for _, entity := range manifest.Entities {
		for _, entityOp := range entity.Operations {
			opID := fmt.Sprintf("%s.%s", entity.ID, entityOp.OP)
			op, err := c.convertEntityOperation(entity.ID, entity, entityOp)
			if err != nil {
				return nil, fmt.Errorf("failed to convert entity operation %s: %w", opID, err)
			}
			operations[opID] = op
		}
	}

	return operations, nil
}

// convertFunction converts a Pack function to OpenAPI Operation
func (c *PackConverter) convertFunction(fn PackFunction) (*openapi3.Operation, error) {
	op := &openapi3.Operation{
		OperationID: fn.ID,
		Summary:     fn.Summary,
		Description: fn.Description,
	}

	// Request body
	if fn.Params != nil {
		requestSchema, err := c.jsonSchemaToOpenAPISchema(fn.Params)
		if err != nil {
			return nil, fmt.Errorf("failed to convert params schema: %w", err)
		}

		requestBody := &openapi3.RequestBody{
			Description: "Request payload",
			Required:    true,
			Content: openapi3.Content{
				"application/json": &openapi3.MediaType{
					Schema: requestSchema,
				},
			},
		}
		op.RequestBody = &openapi3.RequestBodyRef{
			Value: requestBody,
		}
	}

	// Response body
	if fn.Returns != nil {
		responseSchema, err := c.jsonSchemaToOpenAPISchema(fn.Returns)
		if err != nil {
			return nil, fmt.Errorf("failed to convert returns schema: %w", err)
		}

		desc := "Response payload"
		response := &openapi3.Response{
			Description: &desc,
			Content: openapi3.Content{
				"application/json": &openapi3.MediaType{
					Schema: responseSchema,
				},
			},
		}

		if op.Responses == nil {
			op.Responses = openapi3.NewResponses()
		}
		op.Responses.Set("200", &openapi3.ResponseRef{Value: response})
	}

	// Capability extensions
	if fn.Resource != "" {
		if op.Extensions == nil {
			op.Extensions = make(map[string]interface{})
		}
		op.Extensions["x-resource"] = fn.Resource
	}

	if fn.Risk != "" {
		if op.Extensions == nil {
			op.Extensions = make(map[string]interface{})
		}
		op.Extensions["x-risk"] = fn.Risk
	}

	if fn.Operation != "" {
		if op.Extensions == nil {
			op.Extensions = make(map[string]interface{})
		}
		op.Extensions["x-operation"] = fn.Operation
	}

	return op, nil
}

// convertEntityOperation converts a Pack entity operation to OpenAPI Operation
func (c *PackConverter) convertEntityOperation(entityID string, entity PackEntity, op PackEntityOperation) (*openapi3.Operation, error) {
	operation := &openapi3.Operation{
		OperationID: fmt.Sprintf("%s.%s", entityID, op.OP),
		Summary:     op.Name,
	}

	// Request body
	if op.Params != nil {
		requestSchema, err := c.jsonSchemaToOpenAPISchema(op.Params)
		if err != nil {
			return nil, fmt.Errorf("failed to convert params schema: %w", err)
		}

		requestBody := &openapi3.RequestBody{
			Description: "Request payload",
			Required:    true,
			Content: openapi3.Content{
				"application/json": &openapi3.MediaType{
					Schema: requestSchema,
				},
			},
		}
		operation.RequestBody = &openapi3.RequestBodyRef{
			Value: requestBody,
		}
	}

	// Response body
	if op.Returns != nil {
		responseSchema, err := c.jsonSchemaToOpenAPISchema(op.Returns)
		if err != nil {
			return nil, fmt.Errorf("failed to convert returns schema: %w", err)
		}

		desc := "Response payload"
		response := &openapi3.Response{
			Description: &desc,
			Content: openapi3.Content{
				"application/json": &openapi3.MediaType{
					Schema: responseSchema,
				},
			},
		}

		if operation.Responses == nil {
			operation.Responses = openapi3.NewResponses()
		}
		operation.Responses.Set("200", &openapi3.ResponseRef{Value: response})
	}

	// Capability extensions
	if operation.Extensions == nil {
		operation.Extensions = make(map[string]interface{})
	}
	operation.Extensions["x-resource"] = entityID
	operation.Extensions["x-operation"] = op.OP

	return operation, nil
}

// jsonSchemaToOpenAPISchema converts a JSON Schema to OpenAPI 3.0.3 Schema
func (c *PackConverter) jsonSchemaToOpenAPISchema(jsonSchema map[string]interface{}) (*openapi3.SchemaRef, error) {
	schemaBytes, err := json.Marshal(jsonSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON schema: %w", err)
	}

	var schema openapi3.Schema
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema: %w", err)
	}

	return &openapi3.SchemaRef{Value: &schema}, nil
}

// LoadPackFromFile loads a Pack manifest from a file
func (c *PackConverter) LoadPackFromFile(filePath string) (*PackManifest, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pack file: %w", err)
	}

	var manifest PackManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pack manifest: %w", err)
	}

	return &manifest, nil
}

// LoadPackFromDir loads a Pack manifest from a directory
func (c *PackConverter) LoadPackFromDir(dirPath string) (*PackManifest, error) {
	manifestPath := filepath.Join(dirPath, "manifest.json")
	return c.LoadPackFromFile(manifestPath)
}
