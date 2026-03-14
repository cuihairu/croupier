package openapi

// Descriptor represents a function descriptor
type Descriptor struct {
	Id          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Category    string      `json:"category"`
	Schema      interface{} `json:"schema"`
}

// DescriptorsRequest is the request to get descriptors
type DescriptorsRequest struct {
	Type   string `form:"type"`
	GameId string `form:"gameId"`
}

// DescriptorsResponse is the response with descriptors
type DescriptorsResponse struct {
	Items []Descriptor `json:"items"`
}

// OpenAPIDocumentRequest is the request to get aggregated OpenAPI document
type OpenAPIDocumentRequest struct{}

// OpenAPIDocumentResponse is the response with aggregated OpenAPI document
type OpenAPIDocumentResponse struct {
	Spec interface{} `json:"spec"` // OpenAPI 3.0.3 Document
}

// OpenAPIImportRequest is the request to import OpenAPI spec
type OpenAPIImportRequest struct {
	Spec interface{} `json:"spec" binding:"required"` // OpenAPI 3.0.3 Document
}

// OpenAPIImportResponse is the response from OpenAPI import
type OpenAPIImportResponse struct {
	Imported int      `json:"imported"`
	Failed   []string `json:"failed"`
}

// OpenAPISpecRequest is the request to get function OpenAPI spec
type OpenAPISpecRequest struct {
	ID string `uri:"id" binding:"required"`
}

// OpenAPISpecResponse is the response with function OpenAPI spec
type OpenAPISpecResponse struct {
	Spec interface{} `json:"spec"` // OpenAPI 3.0.3 Operation Object
}

// GetSpecRequest is the request to get function OpenAPI spec
// Deprecated: Use OpenAPISpecRequest instead
type GetSpecRequest = OpenAPISpecRequest

// GetSpecResponse is the response with function OpenAPI spec
// Deprecated: Use OpenAPISpecResponse instead
type GetSpecResponse = OpenAPISpecResponse

// ImportRequest is the request to import OpenAPI spec
// Deprecated: Use OpenAPIImportRequest instead
type ImportRequest = OpenAPIImportRequest

// ImportResponse is the response from OpenAPI import
// Deprecated: Use OpenAPIImportResponse instead
type ImportResponse = OpenAPIImportResponse

// EntityFunctionsRequest is the request to get entity functions
type EntityFunctionsRequest struct {
	ID string `uri:"id" binding:"required"`
}

// EntityFunction represents a function associated with an entity
type EntityFunction struct {
	ID        string `json:"id"`
	Operation string `json:"operation"`
	Name      string `json:"name"`
}

// EntityFunctionsResponse is the response with entity functions
type EntityFunctionsResponse struct {
	Items []EntityFunction `json:"items"`
}

// GetDocumentRequest is the request to get aggregated OpenAPI document
// Deprecated: Use OpenAPIDocumentRequest instead
type GetDocumentRequest = OpenAPIDocumentRequest

// GetDocumentResponse is the response with aggregated OpenAPI document
// Deprecated: Use OpenAPIDocumentResponse instead
type GetDocumentResponse = OpenAPIDocumentResponse
