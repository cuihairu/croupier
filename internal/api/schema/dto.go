package schema

// SchemaItem represents a schema item in the system
type SchemaItem struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Schema    interface{} `json:"schema"`
	UIConfig  interface{} `json:"uiConfig,omitempty"`
	CreatedAt string      `json:"createdAt"`
	UpdatedAt string      `json:"updatedAt"`
}

type SchemaCreateRequest struct {
	Name   string      `json:"name"`
	Schema interface{} `json:"schema"`
}

type SchemaCreateResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type SchemaDeleteRequest struct {
	ID string `uri:"id"`
}

type SchemaDeleteResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type SchemaDetailRequest struct {
	ID string `uri:"id"`
}

type SchemaDetailResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type SchemaRawValidateRequest struct {
	Schema interface{} `json:"schema"`
	Data   interface{} `json:"data"`
}

type SchemaRawValidateResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type SchemaUIConfigRequest struct {
	ID string `uri:"id"`
}

type SchemaUIConfigResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type SchemaUIConfigUpdateRequest struct {
	ID     string      `uri:"id"`
	Config interface{} `json:"config"`
}

type SchemaUIConfigUpdateResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type SchemaUpdateRequest struct {
	ID     string      `uri:"id"`
	Schema interface{} `json:"schema"`
}

type SchemaUpdateResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type SchemaValidateRequest struct {
	ID   string      `uri:"id"`
	Data interface{} `json:"data"`
}

type SchemaValidateResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type SchemasListRequest struct {
	Page     int `form:"page"`
	PageSize int `form:"pageSize"`
}

type SchemasListResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Service types for schema operations

// ListRequest is the request to list schemas
type ListRequest struct {
	Page     int `form:"page"`
	PageSize int `form:"pageSize"`
}

// ListResponse is the response with schema list
type ListResponse struct {
	Items []SchemaItem `json:"items"`
	Total int          `json:"total"`
}

// CreateRequest is the request to create a schema
type CreateRequest struct {
	Name   string      `json:"name"`
	Schema interface{} `json:"schema"`
}

// CreateResponse is the response from creating a schema
type CreateResponse struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Schema    interface{} `json:"schema"`
	UIConfig  interface{} `json:"uiConfig,omitempty"`
	CreatedAt string      `json:"createdAt"`
	UpdatedAt string      `json:"updatedAt"`
}

// GetRequest is the request to get a schema
type GetRequest struct {
	ID string `uri:"id"`
}

// GetResponse is the response with schema details
type GetResponse struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Schema    interface{} `json:"schema"`
	UIConfig  interface{} `json:"uiConfig,omitempty"`
	CreatedAt string      `json:"createdAt"`
	UpdatedAt string      `json:"updatedAt"`
}

// UpdateRequest is the request to update a schema
type UpdateRequest struct {
	ID     string      `uri:"id"`
	Schema interface{} `json:"schema"`
}

// UpdateResponse is the response from updating a schema
type UpdateResponse struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Schema    interface{} `json:"schema"`
	UIConfig  interface{} `json:"uiConfig,omitempty"`
	CreatedAt string      `json:"createdAt"`
	UpdatedAt string      `json:"updatedAt"`
}

// DeleteRequest is the request to delete a schema
type DeleteRequest struct {
	ID string `uri:"id"`
}

// ValidateRequest is the request to validate data against a schema
type ValidateRequest struct {
	ID   string      `uri:"id"`
	Data interface{} `json:"data"`
}

// ValidateResponse is the response from validation
type ValidateResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// RawValidateRequest is the request to validate data against a raw schema
type RawValidateRequest struct {
	Schema interface{} `json:"schema"`
	Data   interface{} `json:"data"`
}

// RawValidateResponse is the response from raw validation
type RawValidateResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// GetUIConfigRequest is the request to get UI config for a schema
type GetUIConfigRequest struct {
	ID string `uri:"id"`
}

// GetUIConfigResponse is the response with UI config
type GetUIConfigResponse struct {
	ID       string      `json:"id"`
	UIConfig interface{} `json:"uiConfig,omitempty"`
}

// UpdateUIConfigRequest is the request to update UI config for a schema
type UpdateUIConfigRequest struct {
	ID     string      `uri:"id"`
	Config interface{} `json:"config"`
}

// UpdateUIConfigResponse is the response from updating UI config
type UpdateUIConfigResponse struct {
	ID       string      `json:"id"`
	UIConfig interface{} `json:"uiConfig,omitempty"`
}
