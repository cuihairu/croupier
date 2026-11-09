package registry

import (
	"errors"
	"fmt"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

// UpsertOpenAPI inserts or updates an OpenAPI operation by function ID.
func (s *Store) UpsertOpenAPI(functionID string, operation *openapi3.Operation) error {
	if functionID == "" || operation == nil {
		return fmt.Errorf("function ID and operation are required")
	}
	clone, err := cloneOpenAPIOperation(operation)
	if err != nil {
		return fmt.Errorf("clone operation failed: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.openapiOperations[functionID] = clone
	return nil
}

// ErrOpenAPINotFound 是 GetOpenAPI 对未注册 function ID 的可判别未命中错误
// （区别于 clone 序列化失败等来源故障——调用方须据此区分「未声明」与「故障」，
// 治理链对后者 fail-closed）。
var ErrOpenAPINotFound = errors.New("openapi operation not found")

// GetOpenAPI retrieves an OpenAPI operation by function ID.
// 空 functionID 语义上等同「未注册/未声明」（没有可查询的 OpenAPI 声明），
// 返回可判别的 ErrOpenAPINotFound，调用方据此合法回落（治理链不得把
// 未声明风险当作来源故障 fail-closed）。
func (s *Store) GetOpenAPI(functionID string) (*openapi3.Operation, error) {
	if functionID == "" {
		return nil, fmt.Errorf("%w: function ID is required", ErrOpenAPINotFound)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	op, exists := s.openapiOperations[functionID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrOpenAPINotFound, functionID)
	}

	return cloneOpenAPIOperation(op)
}

// ListOpenAPIOperations returns all OpenAPI operations.
func (s *Store) ListOpenAPIOperations() map[string]*openapi3.Operation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*openapi3.Operation, len(s.openapiOperations))
	for id, op := range s.openapiOperations {
		clone, err := cloneOpenAPIOperation(op)
		if err != nil {
			continue
		}
		result[id] = clone
	}

	return result
}

func cloneOpenAPIOperation(op *openapi3.Operation) (*openapi3.Operation, error) {
	if op == nil {
		return nil, fmt.Errorf("operation is nil")
	}
	b, err := op.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var clone openapi3.Operation
	if err := clone.UnmarshalJSON(b); err != nil {
		return nil, err
	}
	return &clone, nil
}

// UpsertOpenAPIProvider inserts or updates provider capabilities in OpenAPI format.
func (s *Store) UpsertOpenAPIProvider(caps OpenAPIProviderCaps) error {
	if caps.ID == "" {
		return fmt.Errorf("provider ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	caps.UpdatedAt = time.Now()
	s.openapiProviders[caps.ID] = &caps
	return nil
}

// GetOpenAPIProvider retrieves OpenAPI provider capabilities by provider ID.
func (s *Store) GetOpenAPIProvider(providerID string) (*OpenAPIProviderCaps, error) {
	if providerID == "" {
		return nil, fmt.Errorf("provider ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	caps, exists := s.openapiProviders[providerID]
	if !exists {
		return nil, fmt.Errorf("provider not found: %s", providerID)
	}

	return caps, nil
}

// ListOpenAPIProviders returns all OpenAPI providers.
func (s *Store) ListOpenAPIProviders() []*OpenAPIProviderCaps {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*OpenAPIProviderCaps, 0, len(s.openapiProviders))
	for _, caps := range s.openapiProviders {
		result = append(result, caps)
	}

	return result
}

// BuildOpenAPISpec constructs a complete OpenAPI 3.0.3 specification from all registered operations.
func (s *Store) BuildOpenAPISpec() (*openapi3.T, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	doc := &openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title:       "Croupier Functions",
			Description: "Auto-generated OpenAPI specification from registered functions",
			Version:     "1.0.0",
		},
		Paths: openapi3.NewPaths(),
	}

	// Add paths from operations
	for functionID, op := range s.openapiOperations {
		if op == nil {
			continue
		}

		pathItem := &openapi3.PathItem{}
		pathItem.Post = op

		path := fmt.Sprintf("/functions/%s", functionID)
		doc.Paths.Set(path, pathItem)
	}

	return doc, nil
}

// DeleteOpenAPI removes an OpenAPI operation by function ID.
func (s *Store) DeleteOpenAPI(functionID string) error {
	if functionID == "" {
		return fmt.Errorf("function ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.openapiOperations[functionID]; !exists {
		return fmt.Errorf("operation not found: %s", functionID)
	}

	delete(s.openapiOperations, functionID)
	return nil
}

// DeleteOpenAPIProvider removes an OpenAPI provider by provider ID.
func (s *Store) DeleteOpenAPIProvider(providerID string) error {
	if providerID == "" {
		return fmt.Errorf("provider ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.openapiProviders[providerID]; !exists {
		return fmt.Errorf("provider not found: %s", providerID)
	}

	delete(s.openapiProviders, providerID)
	return nil
}
