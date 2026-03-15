package platform

// PlatformInfo represents information about a platform
type PlatformInfo struct {
	Name    string   `json:"name"`
	Version string   `json:"version"`
	Enabled bool     `json:"enabled"`
	Methods []string `json:"methods"`
	Source  string   `json:"source,omitempty"` // extension | legacy
}

// CallPlatformRequest represents a request to call a platform
type CallPlatformRequest struct {
	Platform string `json:"platform"` // Platform name, e.g., "quicksdk"
	Method   string `json:"method"`   // API method name
	Request  string `json:"request"`  // Request parameters (JSON string format)
}

// CallPlatformResponse represents the response for calling a platform
type CallPlatformResponse struct {
	Code           int         `json:"code"`
	Message        string      `json:"message"`
	Response       interface{} `json:"response,omitempty"`        // Response from the platform
	Source         string      `json:"source,omitempty"`          // extension | legacy
	Fallback       bool        `json:"fallback,omitempty"`        // true when legacy fallback is used
	FallbackReason string      `json:"fallback_reason,omitempty"` // extension_error
}

// ListPlatformsResponse represents the response for listing platforms
type ListPlatformsResponse struct {
	Code      int            `json:"code"`
	Message   string         `json:"message"`
	Platforms []PlatformInfo `json:"platforms,omitempty"`
}

// ListPlatformMethodsResponse represents the response for listing platform methods
type ListPlatformMethodsResponse struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Methods []string `json:"methods,omitempty"`
	Source  string   `json:"source,omitempty"` // extension | legacy | mixed
}

// ReloadPlatformConfigResponse represents the response for reloading platform config
type ReloadPlatformConfigResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success,omitempty"`
}
