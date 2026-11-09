// Package openapi provides a generic provider for OpenAPI/Swagger based services.
// This allows quick integration of any HTTP service that provides OpenAPI documentation.
//
// 本包是 #66 Provider 插件批二 P1 迁入的 driver 实现（原 internal/platform/openapi，
// mapping §2.3「作为驱动复用，不再当业务模块」）；Provider 接口实现由
// internal/platform/provider 的 driver 契约蓝本收窄而来，driver 注册表见
// internal/drivers。
package openapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/platform/provider"
	"github.com/cuihairu/croupier/internal/platform/ratelimit"
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
	infoVersion   string                // info.version of the first parsed document (may be non-semver)
	// outputValidators 按 method 缓存响应 schema 的编译产物（能力矩阵 #4）。
	outputValidators sync.Map
	mu               sync.RWMutex
}

// Config holds the OpenAPI provider specific configuration.
type Config struct {
	// BaseURL is the base URL of the OpenAPI service (e.g., "http://api.example.com")
	BaseURL string `yaml:"base_url" json:"baseUrl"`

	// Auth specifies the authentication method
	Auth *AuthConfig `yaml:"auth" json:"auth"`

	// Methods defines the API methods available through this provider
	// If empty, will attempt to discover from OpenAPI spec
	Methods []APIMethod `yaml:"methods" json:"methods"`

	// OpenAPISpec is the URL or local path to a single OpenAPI/Swagger specification
	// If provided, the provider will auto-discover available methods
	OpenAPISpec string `yaml:"openapi_spec" json:"openapiSpec"`

	// OpenAPISpecs is a list of URLs or local paths to multiple OpenAPI specifications
	// If provided, the provider will merge all specs and auto-discover methods
	// Takes precedence over OpenAPISpec if both are set
	OpenAPISpecs []string `yaml:"openapi_specs" json:"openapiSpecs"`

	// Timeout for HTTP requests (supports "10s", "1m", "500ms" or seconds as number)
	Timeout Duration `yaml:"timeout" json:"timeout"`

	// RetryCount specifies how many times to retry on failure
	RetryCount int `yaml:"retry_count" json:"retryCount"`

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
	APIKey *APIKeyAuth `yaml:"api_key" json:"apiKey"`

	// Custom header configuration for "custom" type
	CustomHeaders map[string]string `yaml:"custom_headers" json:"customHeaders"`
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
	OperationID string `yaml:"operation_id" json:"operationId"`

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
	RequestBody *RequestBodyMapping `yaml:"request_body" json:"requestBody"`

	// ResponseMapping defines how to transform the response
	ResponseMapping *ResponseMapping `yaml:"response_mapping" json:"responseMapping"`

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
	Version    string `yaml:"x-version" json:"x-version"`       // x-version: per-operation semver override (falls back to batch default)

	// InputSchema is the JSON Schema (JSON text) derived from the operation's
	// parameters and application/json request body (see extractInputSchema).
	// Derived from the spec; methods declared only in config stay empty.
	InputSchema string `yaml:"input_schema" json:"inputSchema"`

	// OutputSchema is the JSON Schema (JSON text) of the operation's success
	// (2xx) application/json response (see extractOutputSchema); empty when
	// the spec declares no JSON success response — response validation is
	// skipped for such methods.
	OutputSchema string `yaml:"output_schema" json:"outputSchema"`
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
	ExtractPath string `yaml:"extract_path" json:"extractPath"`

	// Wrap indicates whether to wrap the response in a standard envelope
	Wrap bool `yaml:"wrap" json:"wrap"`

	// SuccessField is the field that indicates success (e.g., "code == 0")
	SuccessField string `yaml:"success_field" json:"successField"`

	// SuccessValue is the value that indicates success
	SuccessValue interface{} `yaml:"success_value" json:"successValue"`
}

// TransformConfig defines global response transformation.
type TransformConfig struct {
	// SuccessField specifies the field that indicates success
	SuccessField string `yaml:"success_field" json:"successField"`

	// SuccessValue is the value that indicates success
	SuccessValue interface{} `yaml:"success_value" json:"successValue"`

	// DataField specifies the field containing the actual data
	DataField string `yaml:"data_field" json:"dataField"`

	// ErrorField specifies the field containing error message
	ErrorField string `yaml:"error_field" json:"errorField"`
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
	Version    string // x-version (per-operation semver override; may be empty)

	// InputSchema is the JSON Schema (JSON text) derived from the operation's
	// parameters and application/json request body (see extractInputSchema).
	InputSchema string

	// OutputSchema is the JSON Schema (JSON text) of the operation's success
	// response (see extractOutputSchema); empty when absent from the spec.
	OutputSchema string
}

// NewProvider creates a new OpenAPI provider.
func NewProvider() *Provider {
	return &Provider{
		methodMap: make(map[string]*APIMethod),
	}
}
