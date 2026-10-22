package openapi

// FunctionProviderType represents the type of function provider (e.g., OpenAPI, Proto, Pack).
type FunctionProviderType int

const (
	FunctionProviderTypeUnknown FunctionProviderType = iota
	FunctionProviderTypeOpenAPI
	FunctionProviderTypeProto
	FunctionProviderTypePack
)

// FunctionProvider represents a function provider with metadata and functions.
// This is used for converting between different function definition formats.
// Note: This is different from provider.Provider which is for third-party platform integrations.
type FunctionProvider struct {
	Name      string
	Type      FunctionProviderType
	Version   string
	Functions map[string]*FunctionDefinition
}

// FunctionDefinition represents a single function with all its metadata.
type FunctionDefinition struct {
	ID           string
	Summary      string
	Description  string
	Resource     string
	Risk         string
	Operation    string
	InputSchema  interface{}
	OutputSchema interface{}
}

// FunctionProviderTypeFromString converts a string to FunctionProviderType.
func FunctionProviderTypeFromString(s string) FunctionProviderType {
	switch s {
	case "openapi", "OpenAPI":
		return FunctionProviderTypeOpenAPI
	case "proto", "Proto":
		return FunctionProviderTypeProto
	case "pack", "Pack":
		return FunctionProviderTypePack
	default:
		return FunctionProviderTypeUnknown
	}
}

// String returns the string representation of FunctionProviderType.
func (t FunctionProviderType) String() string {
	switch t {
	case FunctionProviderTypeOpenAPI:
		return "openapi"
	case FunctionProviderTypeProto:
		return "proto"
	case FunctionProviderTypePack:
		return "pack"
	default:
		return "unknown"
	}
}
