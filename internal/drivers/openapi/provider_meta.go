package openapi

import (
	"encoding/json"
)

// IsEnabled returns whether the provider is enabled.
func (p *Provider) IsEnabled() bool {
	return p.config.Enabled
}

// SupportedMethods returns the list of supported methods.
func (p *Provider) SupportedMethods() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.methods
}

// GetMethodDetails returns the complete details for all methods.
// This is used by Agent to register functions with full metadata.
func (p *Provider) GetMethodDetails() map[string]*MethodDetails {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]*MethodDetails, len(p.methodMap))
	for name, method := range p.methodMap {
		result[name] = &MethodDetails{
			Name:         name,
			OperationID:  method.OperationID,
			Summary:      method.Summary,
			Description:  method.Description,
			Tags:         method.Tags,
			Deprecated:   method.Deprecated,
			Parameters:   method.Parameters,
			Resource:     method.Resource,
			Risk:         method.Risk,
			Operation:    method.Operation,
			Capability:   method.Capability,
			Execution:    method.Execution,
			Enabled:      method.Enabled,
			Permission:   method.Permission,
			Version:      method.Version,
			InputSchema:  method.InputSchema,
			OutputSchema: method.OutputSchema,
		}
	}
	return result
}

// InfoVersion returns the document-level info.version of the first parsed
// OpenAPI spec (raw value; validity must be checked by the caller).
func (p *Provider) InfoVersion() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.infoVersion
}

// GetOpenAPIDoc returns the raw OpenAPI document JSON.
// This can be used to store the complete OpenAPI specification for later retrieval.
func (p *Provider) GetOpenAPIDoc() json.RawMessage {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.openapiDoc
}
