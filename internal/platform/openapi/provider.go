// Package openapi provides a generic provider for OpenAPI/Swagger based services.
// This allows quick integration of any HTTP service that provides OpenAPI documentation.
package openapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/platform/provider"
	"github.com/cuihairu/croupier/internal/platform/ratelimit"
	"gopkg.in/yaml.v3"
)

// Duration is a custom duration type that supports parsing from both
// human-readable strings (like "10s", "1m", "500ms") and integers (nanoseconds).
type Duration time.Duration

// UnmarshalJSON implements json.Unmarshaler for Duration.
func (d *Duration) UnmarshalJSON(data []byte) error {
	// Try string first (e.g., "10s", "1m", "500ms")
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		parsed, err := time.ParseDuration(str)
		if err != nil {
			return fmt.Errorf("invalid duration string %q: %w", str, err)
		}
		*d = Duration(parsed)
		return nil
	}

	// Try integer (nanoseconds)
	var ns int64
	if err := json.Unmarshal(data, &ns); err == nil {
		*d = Duration(ns)
		return nil
	}

	// Try float (seconds, for convenience)
	var secs float64
	if err := json.Unmarshal(data, &secs); err == nil {
		*d = Duration(time.Duration(secs * float64(time.Second)))
		return nil
	}

	return fmt.Errorf("duration must be a string (e.g., \"10s\") or number, got: %s", string(data))
}

// Duration returns the time.Duration value.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// Provider implements the provider.Provider interface for OpenAPI based services.
// It provides a generic way to call any HTTP API with configurable authentication,
// parameter mapping, and response transformation.
type Provider struct {
	config        provider.ProviderConfig
	openapiConfig *Config
	httpClient    *http.Client
	rateLimiter   ratelimit.Limiter
	methods       []string
	methodMap     map[string]*APIMethod // method name -> API definition
	openapiDoc    json.RawMessage       // Raw OpenAPI document JSON
	mu            sync.RWMutex
}

// Config holds the OpenAPI provider specific configuration.
type Config struct {
	// BaseURL is the base URL of the OpenAPI service (e.g., "http://api.example.com")
	BaseURL string `yaml:"base_url" json:"base_url"`

	// Auth specifies the authentication method
	Auth *AuthConfig `yaml:"auth" json:"auth"`

	// Methods defines the API methods available through this provider
	// If empty, will attempt to discover from OpenAPI spec
	Methods []APIMethod `yaml:"methods" json:"methods"`

	// OpenAPISpec is the URL or local path to a single OpenAPI/Swagger specification
	// If provided, the provider will auto-discover available methods
	OpenAPISpec string `yaml:"openapi_spec" json:"openapi_spec"`

	// OpenAPISpecs is a list of URLs or local paths to multiple OpenAPI specifications
	// If provided, the provider will merge all specs and auto-discover methods
	// Takes precedence over OpenAPISpec if both are set
	OpenAPISpecs []string `yaml:"openapi_specs" json:"openapi_specs"`

	// Timeout for HTTP requests (supports "10s", "1m", "500ms" or seconds as number)
	Timeout Duration `yaml:"timeout" json:"timeout"`

	// RetryCount specifies how many times to retry on failure
	RetryCount int `yaml:"retry_count" json:"retry_count"`

	// Headers are default headers to include in all requests
	Headers map[string]string `yaml:"headers" json:"headers"`

	// Transform allows response transformation
	Transform *TransformConfig `yaml:"transform" json:"transform"`
}

// AuthConfig defines authentication methods.
type AuthConfig struct {
	// Type can be: "none", "bearer", "basic", "api_key", "custom"
	Type string `yaml:"type" json:"type"`

	// Bearer token for "bearer" type
	Token string `yaml:"token" json:"token"`

	// Username for "basic" type
	Username string `yaml:"username" json:"username"`

	// Password for "basic" type
	Password string `yaml:"password" json:"password"`

	// APIKey configuration for "api_key" type
	APIKey *APIKeyAuth `yaml:"api_key" json:"api_key"`

	// Custom header configuration for "custom" type
	CustomHeaders map[string]string `yaml:"custom_headers" json:"custom_headers"`
}

// APIKeyAuth defines API key authentication.
type APIKeyAuth struct {
	// Name is the header or query parameter name (e.g., "X-API-Key")
	Name string `yaml:"name" json:"name"`

	// Value is the API key value
	Value string `yaml:"value" json:"value"`

	// In specifies where to put the key: "header" or "query"
	In string `yaml:"in" json:"in"`
}

// APIMethod defines a single API method that can be called.
// It closely follows OpenAPI 3.0 operation object structure.
type APIMethod struct {
	// Name is the method name used for calling (e.g., "get_role", "ban_user")
	// Falls back to operationId or generated from path if not set
	Name string `yaml:"name" json:"name"`

	// OperationId from OpenAPI spec (unique identifier for the operation)
	OperationID string `yaml:"operation_id" json:"operation_id"`

	// Summary is a short summary of what the operation does (from OpenAPI)
	Summary string `yaml:"summary" json:"summary"`

	// Description is a verbose explanation of the operation behavior (from OpenAPI)
	Description string `yaml:"description" json:"description"`

	// Path is the API path (e.g., "/api/role/{role_id}")
	Path string `yaml:"path" json:"path"`

	// Method is the HTTP method (GET, POST, PUT, DELETE, etc.)
	Method string `yaml:"method" json:"method"`

	// Tags for categorization (from OpenAPI tags)
	// Operations may be grouped by tags for UI organization
	Tags []string `yaml:"tags" json:"tags"`

	// Parameters defines how to map request parameters
	Parameters []ParameterMapping `yaml:"parameters" json:"parameters"`

	// RequestBody defines how to map the request body
	RequestBody *RequestBodyMapping `yaml:"request_body" json:"request_body"`

	// ResponseMapping defines how to transform the response
	ResponseMapping *ResponseMapping `yaml:"response_mapping" json:"response_mapping"`

	// Deprecated marks this operation as deprecated (from OpenAPI)
	Deprecated bool `yaml:"deprecated" json:"deprecated"`

	// OpenAPI 3.0.3 Extension fields (x-*)
	Resource   string `yaml:"x-resource" json:"x-resource"`     // x-resource: business resource/capability key
	Risk       string `yaml:"x-risk" json:"x-risk"`             // x-risk: risk level
	Operation  string `yaml:"x-operation" json:"x-operation"`   // x-operation: business action key
	Capability string `yaml:"x-capability" json:"x-capability"` // x-capability: lifecycle capability
	Execution  string `yaml:"x-execution" json:"x-execution"`   // x-execution: sync/task
	Enabled    bool   `yaml:"x-enabled" json:"x-enabled"`       // x-enabled: whether this function is enabled
	Permission string `yaml:"x-permission" json:"x-permission"` // x-permission: optional permission identifier
}

// ParameterMapping defines how to map a parameter.
// Follows OpenAPI 3.0 parameter object structure.
type ParameterMapping struct {
	// Name is the parameter name in the API request
	Name string `yaml:"name" json:"name"`

	// In specifies where the parameter goes: "path", "query", "header", "cookie"
	In string `yaml:"in" json:"in"`

	// Description of the parameter (from OpenAPI)
	Description string `yaml:"description" json:"description"`

	// From specifies the field name in the input JSON (e.g., "role_id")
	// If empty, uses Name
	From string `yaml:"from" json:"from"`

	// Required indicates if this parameter is required (from OpenAPI)
	Required bool `yaml:"required" json:"required"`

	// Deprecated marks this parameter as deprecated (from OpenAPI)
	Deprecated bool `yaml:"deprecated" json:"deprecated"`

	// Default value if not provided (from OpenAPI)
	Default interface{} `yaml:"default" json:"default"`

	// Type of the parameter: "string", "number", "integer", "boolean", "array", "object"
	Type string `yaml:"type" json:"type"`

	// Format of the parameter type: e.g., "int32", "int64", "float", "double", "date-time"
	Format string `yaml:"format" json:"format"`

	// Enum allowed values for this parameter (from OpenAPI)
	Enum []interface{} `yaml:"enum" json:"enum"`
}

// RequestBodyMapping defines how to map the request body.
type RequestBodyMapping struct {
	// Type specifies the body format: "json", "form", "text"
	Type string `yaml:"type" json:"type"`

	// Template is a Go template for building the body
	// Example: {"user_id": "{{ .user_id }}", "reason": "{{ .reason }}"}
	Template string `yaml:"template" json:"template"`

	// Fields defines field mappings (used when type is "json")
	Fields map[string]string `yaml:"fields" json:"fields"`
}

// ResponseMapping defines how to transform the response.
type ResponseMapping struct {
	// ExtractPath is a JSON path to extract from response (e.g., "data.items")
	ExtractPath string `yaml:"extract_path" json:"extract_path"`

	// Wrap indicates whether to wrap the response in a standard envelope
	Wrap bool `yaml:"wrap" json:"wrap"`

	// SuccessField is the field that indicates success (e.g., "code == 0")
	SuccessField string `yaml:"success_field" json:"success_field"`

	// SuccessValue is the value that indicates success
	SuccessValue interface{} `yaml:"success_value" json:"success_value"`
}

// TransformConfig defines global response transformation.
type TransformConfig struct {
	// SuccessField specifies the field that indicates success
	SuccessField string `yaml:"success_field" json:"success_field"`

	// SuccessValue is the value that indicates success
	SuccessValue interface{} `yaml:"success_value" json:"success_value"`

	// DataField specifies the field containing the actual data
	DataField string `yaml:"data_field" json:"data_field"`

	// ErrorField specifies the field containing error message
	ErrorField string `yaml:"error_field" json:"error_field"`
}

// MethodDetails contains the metadata for a method extracted from OpenAPI.
// This is used when registering functions to the LocalStore.
type MethodDetails struct {
	Name        string
	OperationID string
	Summary     string
	Description string
	Tags        []string
	Deprecated  bool
	Parameters  []ParameterMapping

	// OpenAPI 3.0.3 Extension fields (x-*)
	Resource   string // x-resource
	Risk       string // x-risk
	Operation  string // x-operation
	Capability string // x-capability
	Execution  string // x-execution
	Enabled    bool   // x-enabled
	Permission string // x-permission
}

// NewProvider creates a new OpenAPI provider.
func NewProvider() *Provider {
	return &Provider{
		methodMap: make(map[string]*APIMethod),
	}
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "openapi"
}

// Init initializes the provider with configuration.
func (p *Provider) Init(ctx context.Context, config provider.ProviderConfig) error {
	p.config = config
	p.openapiConfig = &Config{}

	// Extract OpenAPI config from provider config
	if err := p.extractConfig(config.Config); err != nil {
		return fmt.Errorf("failed to extract config: %w", err)
	}

	// Initialize HTTP client
	timeout := p.openapiConfig.Timeout.Duration()
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	p.httpClient = &http.Client{
		Timeout: timeout,
	}

	// Initialize rate limiter if configured
	if config.RateLimit != nil {
		p.rateLimiter = ratelimit.NewTokenBucket(
			config.RateLimit.RequestsPerMinute,
			config.RateLimit.BurstSize,
		)
	}

	// Build method map
	if err := p.buildMethodMap(); err != nil {
		return fmt.Errorf("failed to build method map: %w", err)
	}

	// Auto-discover methods if OpenAPI spec(s) are provided
	if len(p.openapiConfig.Methods) == 0 {
		// Use multiple specs if available
		if len(p.openapiConfig.OpenAPISpecs) > 0 {
			for _, specURL := range p.openapiConfig.OpenAPISpecs {
				if err := p.discoverMethodsFromSpec(ctx, specURL); err != nil {
					return fmt.Errorf("failed to discover methods from %s: %w", specURL, err)
				}
			}
		} else if p.openapiConfig.OpenAPISpec != "" {
			// Fallback to single spec for backward compatibility
			if err := p.discoverMethodsFromSpec(ctx, p.openapiConfig.OpenAPISpec); err != nil {
				return fmt.Errorf("failed to discover methods: %w", err)
			}
		}
	}

	return nil
}

// extractConfig extracts OpenAPI config from the generic provider config.
func (p *Provider) extractConfig(config map[string]interface{}) error {
	// Marshal and unmarshal to convert map to struct
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, p.openapiConfig)
}

// buildMethodMap builds the method name to API definition mapping.
func (p *Provider) buildMethodMap() error {
	p.methods = make([]string, 0, len(p.openapiConfig.Methods))
	p.methodMap = make(map[string]*APIMethod)

	for i := range p.openapiConfig.Methods {
		method := &p.openapiConfig.Methods[i]
		if method.Name == "" {
			continue
		}
		p.methodMap[method.Name] = method
		p.methods = append(p.methods, method.Name)
	}

	return nil
}

// discoverMethodsFromSpec auto-discovers methods from a single OpenAPI specification.
// Supports both HTTP URLs and local file paths (YAML or JSON format).
func (p *Provider) discoverMethodsFromSpec(ctx context.Context, specURL string) error {
	var spec []byte
	var err error

	if strings.HasPrefix(specURL, "http://") || strings.HasPrefix(specURL, "https://") {
		// HTTP URL
		req, err := http.NewRequestWithContext(ctx, "GET", specURL, nil)
		if err != nil {
			return err
		}

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to fetch OpenAPI spec: %s", resp.Status)
		}

		spec, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
	} else {
		// Local file path (supports both JSON and YAML)
		spec, err = os.ReadFile(specURL)
		if err != nil {
			return fmt.Errorf("failed to read OpenAPI spec file: %w", err)
		}

		// Convert YAML to JSON if needed
		if !looksLikeJSON(spec) {
			spec, err = yamlToJSON(spec)
			if err != nil {
				return fmt.Errorf("failed to convert YAML to JSON: %w", err)
			}
		}
	}

	// Parse OpenAPI spec and generate method definitions
	return p.parseOpenAPISpec(spec)
}

// looksLikeJSON checks if the data appears to be JSON format.
func looksLikeJSON(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return false
	}
	return trimmed[0] == '{' || trimmed[0] == '['
}

// yamlToJSON converts YAML bytes to JSON bytes.
func yamlToJSON(yamlData []byte) ([]byte, error) {
	var yamlObj interface{}
	if err := yaml.Unmarshal(yamlData, &yamlObj); err != nil {
		return nil, err
	}
	return json.Marshal(yamlObj)
}

// parseOpenAPISpec parses OpenAPI/Swagger specification.
func (p *Provider) parseOpenAPISpec(spec []byte) error {
	// Store raw OpenAPI document for later retrieval
	if p.openapiDoc == nil {
		p.openapiDoc = spec
	} else {
		// Merge with existing doc (if multiple specs)
		var existing map[string]interface{}
		var newDoc map[string]interface{}
		if err := json.Unmarshal(p.openapiDoc, &existing); err != nil {
			return err
		}
		if err := json.Unmarshal(spec, &newDoc); err != nil {
			return err
		}

		// Merge paths
		if existingPaths, ok := existing["paths"].(map[string]interface{}); ok {
			if newPaths, ok := newDoc["paths"].(map[string]interface{}); ok {
				for k, v := range newPaths {
					existingPaths[k] = v
				}
			}
		}

		merged, err := json.Marshal(existing)
		if err != nil {
			return err
		}
		p.openapiDoc = merged
	}

	var openapi map[string]interface{}
	if err := json.Unmarshal(spec, &openapi); err != nil {
		return err
	}

	// Support both OpenAPI 3.0 and Swagger 2.0
	paths := make(map[string]interface{})
	if v, ok := openapi["paths"].(map[string]interface{}); ok {
		// OpenAPI 3.0
		paths = v
	} else if v, ok := openapi["apis"].(map[string]interface{}); ok {
		// Swagger 2.0 (sometimes uses "apis")
		paths = v
	}

	for apiPath, pathItem := range paths {
		pathObj, ok := pathItem.(map[string]interface{})
		if !ok {
			continue
		}

		// Iterate over HTTP methods
		for httpMethod, methodItem := range pathObj {
			httpMethod = strings.ToUpper(httpMethod)
			if httpMethod == "PARAMETERS" || httpMethod == "$REF" {
				continue
			}

			methodObj, ok := methodItem.(map[string]interface{})
			if !ok {
				continue
			}

			// Extract operation ID or generate from path
			var methodName string
			if opID, ok := methodObj["operationId"].(string); ok {
				methodName = opID
			} else {
				methodName = pathToMethodName(apiPath, httpMethod)
			}

			// Extract description
			desc := ""
			if summary, ok := methodObj["summary"].(string); ok {
				desc = summary
			} else if description, ok := methodObj["description"].(string); ok {
				desc = description
			}

			// Extract tags
			tags := p.extractTags(methodObj)

			// Extract operationId
			operationID, _ := methodObj["operationId"].(string)

			// Extract summary (prefer summary over description for short text)
			summary, _ := methodObj["summary"].(string)

			// Extract deprecated flag
			deprecated, _ := methodObj["deprecated"].(bool)

			// Extract OpenAPI extension fields (x-*)
			resource, _ := methodObj["x-resource"].(string)
			risk, _ := methodObj["x-risk"].(string)
			operation, _ := methodObj["x-operation"].(string)
			capability, _ := methodObj["x-capability"].(string)
			execution, _ := methodObj["x-execution"].(string)
			enabled, _ := methodObj["x-enabled"].(bool)
			permission, _ := methodObj["x-permission"].(string)

			// Create APIMethod
			apiMethod := &APIMethod{
				Name:        methodName,
				OperationID: operationID,
				Summary:     summary,
				Description: desc,
				Path:        apiPath,
				Method:      httpMethod,
				Tags:        tags,
				Deprecated:  deprecated,
				Parameters:  p.extractParameters(methodObj),
				Resource:    resource,
				Risk:        risk,
				Operation:   operation,
				Capability:  capability,
				Execution:   execution,
				Enabled:     enabled,
				Permission:  permission,
			}

			p.methodMap[methodName] = apiMethod
			p.methods = append(p.methods, methodName)
		}
	}

	return nil
}

// extractTags extracts tags from OpenAPI method definition.
func (p *Provider) extractTags(methodObj map[string]interface{}) []string {
	tagList, ok := methodObj["tags"].([]interface{})
	if !ok {
		return nil
	}
	tags := make([]string, 0, len(tagList))
	for _, t := range tagList {
		if tag, ok := t.(string); ok {
			tags = append(tags, tag)
		}
	}
	return tags
}

// extractParameters extracts parameters from OpenAPI method definition.
func (p *Provider) extractParameters(methodObj map[string]interface{}) []ParameterMapping {
	params := make([]ParameterMapping, 0)

	paramList, ok := methodObj["parameters"].([]interface{})
	if !ok {
		return params
	}

	for _, param := range paramList {
		pObj, ok := param.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract basic parameter info
		name, _ := pObj["name"].(string)
		in, _ := pObj["in"].(string) // "path", "query", "header", "cookie"
		required, _ := pObj["required"].(bool)
		desc, _ := pObj["description"].(string)
		deprecated, _ := pObj["deprecated"].(bool)

		// Build parameter mapping
		paramMapping := ParameterMapping{
			Name:        name,
			In:          in,
			Description: desc,
			From:        name, // Default to using same name
			Required:    required,
			Deprecated:  deprecated,
		}

		// Extract default value
		if defaultValue, ok := pObj["default"]; ok {
			paramMapping.Default = defaultValue
		}

		// Extract schema information (type, format, enum)
		// In OpenAPI 3.0, parameter schema is in the "schema" field
		if schema, ok := pObj["schema"].(map[string]interface{}); ok {
			if typ, ok := schema["type"].(string); ok {
				paramMapping.Type = typ
			}
			if format, ok := schema["format"].(string); ok {
				paramMapping.Format = format
			}
			if enum, ok := schema["enum"].([]interface{}); ok {
				paramMapping.Enum = enum
			}
		}

		params = append(params, paramMapping)
	}

	return params
}

// pathToMethodName converts a path like "/api/role/{id}" to method name "getRole".
func pathToMethodName(apiPath, method string) string {
	// Remove path parameters
	cleanPath := strings.ReplaceAll(apiPath, "}", "")
	cleanPath = strings.ReplaceAll(cleanPath, "{", "")

	// Split by /
	segments := strings.Split(cleanPath, "/")
	segments = filterEmpty(segments)

	// Build method name: {method}{pathSegments}
	var name strings.Builder
	name.WriteString(strings.ToLower(method))
	for _, s := range segments {
		if s != "api" && s != "v1" && s != "v2" {
			name.WriteString(strings.Title(s))
		}
	}

	return name.String()
}

func filterEmpty(ss []string) []string {
	var result []string
	for _, s := range ss {
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

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
			Name:        name,
			OperationID: method.OperationID,
			Summary:     method.Summary,
			Description: method.Description,
			Tags:        method.Tags,
			Deprecated:  method.Deprecated,
			Parameters:  method.Parameters,
			Resource:    method.Resource,
			Risk:        method.Risk,
			Operation:   method.Operation,
			Capability:  method.Capability,
			Execution:   method.Execution,
			Enabled:     method.Enabled,
			Permission:  method.Permission,
		}
	}
	return result
}

// GetOpenAPIDoc returns the raw OpenAPI document JSON.
// This can be used to store the complete OpenAPI specification for later retrieval.
func (p *Provider) GetOpenAPIDoc() json.RawMessage {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.openapiDoc
}

// Call invokes an API method.
func (p *Provider) Call(ctx context.Context, method string, request []byte) ([]byte, error) {
	// Check rate limit
	if p.rateLimiter != nil {
		if err := p.rateLimiter.Wait(ctx); err != nil {
			return nil, err
		}
	}

	p.mu.RLock()
	apiMethod, exists := p.methodMap[method]
	p.mu.RUnlock()

	if !exists {
		return nil, &provider.MethodNotSupportedError{Provider: p.Name(), Method: method}
	}

	if !p.IsEnabled() {
		return nil, &provider.ProviderDisabledError{Name: p.Name()}
	}

	// Parse request
	var reqData map[string]interface{}
	if len(request) > 0 {
		if err := json.Unmarshal(request, &reqData); err != nil {
			return nil, fmt.Errorf("invalid request JSON: %w", err)
		}
	}

	// Build HTTP request
	httpReq, err := p.buildRequest(ctx, apiMethod, reqData)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	// Add authentication
	p.addAuth(httpReq)

	// Add default headers
	for k, v := range p.openapiConfig.Headers {
		httpReq.Header.Set(k, v)
	}

	// Execute request with retry
	var resp *http.Response
	for i := 0; i <= p.openapiConfig.RetryCount; i++ {
		resp, err = p.httpClient.Do(httpReq)
		if err == nil && resp.StatusCode < 500 {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		if i < p.openapiConfig.RetryCount {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(i+1) * time.Second):
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error: %s: %s", resp.Status, string(body))
	}

	// Transform response if configured
	return p.transformResponse(body)
}

// buildRequest builds an HTTP request from the API method and request data.
func (p *Provider) buildRequest(ctx context.Context, apiMethod *APIMethod, reqData map[string]interface{}) (*http.Request, error) {
	// Build URL with path and query parameters
	url := p.buildURL(apiMethod, reqData)

	var body io.Reader
	var contentType string

	// Build request body if needed
	if apiMethod.Method == "POST" || apiMethod.Method == "PUT" || apiMethod.Method == "PATCH" {
		if apiMethod.RequestBody != nil {
			bodyBytes, ct := p.buildBody(apiMethod.RequestBody, reqData)
			body = bytes.NewReader(bodyBytes)
			contentType = ct
		} else if len(reqData) > 0 {
			data, _ := json.Marshal(reqData)
			body = bytes.NewReader(data)
			contentType = "application/json"
		}
	}

	req, err := http.NewRequestWithContext(ctx, apiMethod.Method, url, body)
	if err != nil {
		return nil, err
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	// Add parameters as headers if configured
	for _, param := range apiMethod.Parameters {
		if param.In == "header" {
			val := p.getParamValue(reqData, param)
			if val != "" {
				req.Header.Set(param.Name, val)
			}
		}
	}

	return req, nil
}

// buildURL builds the full URL with path parameters and query string.
func (p *Provider) buildURL(apiMethod *APIMethod, reqData map[string]interface{}) string {
	baseURL := strings.TrimSuffix(p.openapiConfig.BaseURL, "/")
	apiPath := apiMethod.Path

	// Replace path parameters
	for _, param := range apiMethod.Parameters {
		if param.In == "path" {
			val := p.getParamValue(reqData, param)
			placeholder := "{" + param.Name + "}"
			apiPath = strings.ReplaceAll(apiPath, placeholder, val)
		}
	}

	url := baseURL + apiPath

	// Add query parameters
	queryParts := []string{}
	for _, param := range apiMethod.Parameters {
		if param.In == "query" {
			val := p.getParamValue(reqData, param)
			if val != "" {
				queryParts = append(queryParts, fmt.Sprintf("%s=%s", param.Name, val))
			}
		}
	}

	if len(queryParts) > 0 {
		url += "?" + strings.Join(queryParts, "&")
	}

	return url
}

// buildBody builds the request body.
func (p *Provider) buildBody(bodyConfig *RequestBodyMapping, reqData map[string]interface{}) ([]byte, string) {
	if bodyConfig.Type == "json" || bodyConfig.Type == "" {
		if bodyConfig.Template != "" {
			// Use template
			result := p.applyTemplate(bodyConfig.Template, reqData)
			return []byte(result), "application/json"
		}
		// Use field mapping
		result := make(map[string]interface{})
		for dstField, srcField := range bodyConfig.Fields {
			if val, ok := reqData[srcField]; ok {
				result[dstField] = val
			}
		}
		data, _ := json.Marshal(result)
		return data, "application/json"
	}
	return nil, ""
}

// applyTemplate applies a simple template with {{ .field }} placeholders.
func (p *Provider) applyTemplate(template string, data map[string]interface{}) string {
	result := template
	for key, val := range data {
		placeholder := fmt.Sprintf("{{ .%s }}", key)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", val))
	}
	return result
}

// getParamValue gets a parameter value from request data.
func (p *Provider) getParamValue(reqData map[string]interface{}, param ParameterMapping) string {
	from := param.From
	if from == "" {
		from = param.Name
	}

	if val, ok := reqData[from]; ok && val != nil {
		return fmt.Sprintf("%v", val)
	}

	if param.Default != nil {
		return fmt.Sprintf("%v", param.Default)
	}

	return ""
}

// addAuth adds authentication to the request.
func (p *Provider) addAuth(req *http.Request) {
	if p.openapiConfig.Auth == nil {
		return
	}

	switch p.openapiConfig.Auth.Type {
	case "bearer":
		if p.openapiConfig.Auth.Token != "" {
			req.Header.Set("Authorization", "Bearer "+p.openapiConfig.Auth.Token)
		}
	case "basic":
		if p.openapiConfig.Auth.Username != "" || p.openapiConfig.Auth.Password != "" {
			req.SetBasicAuth(p.openapiConfig.Auth.Username, p.openapiConfig.Auth.Password)
		}
	case "api_key":
		if p.openapiConfig.Auth.APIKey != nil {
			if p.openapiConfig.Auth.APIKey.In == "header" {
				req.Header.Set(p.openapiConfig.Auth.APIKey.Name, p.openapiConfig.Auth.APIKey.Value)
			}
		}
	case "custom":
		for k, v := range p.openapiConfig.Auth.CustomHeaders {
			req.Header.Set(k, v)
		}
	}
}

// transformResponse transforms the response based on configuration.
func (p *Provider) transformResponse(body []byte) ([]byte, error) {
	if p.openapiConfig.Transform == nil {
		return body, nil
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return body, nil // Return as-is if not JSON
	}

	transformed := make(map[string]interface{})

	// Check success field
	if p.openapiConfig.Transform.SuccessField != "" {
		if successVal, ok := resp[p.openapiConfig.Transform.SuccessField]; ok {
			transformed["success"] = successVal == p.openapiConfig.Transform.SuccessValue
		}
	}

	// Extract data field
	if p.openapiConfig.Transform.DataField != "" {
		if dataVal, ok := resp[p.openapiConfig.Transform.DataField]; ok {
			transformed["data"] = dataVal
		}
	} else {
		transformed["data"] = resp
	}

	// Extract error field
	if p.openapiConfig.Transform.ErrorField != "" {
		if errVal, ok := resp[p.openapiConfig.Transform.ErrorField]; ok {
			transformed["error"] = errVal
		}
	}

	return json.Marshal(transformed)
}

// Close closes the provider.
func (p *Provider) Close() error {
	if p.httpClient != nil {
		p.httpClient.CloseIdleConnections()
	}
	return nil
}
