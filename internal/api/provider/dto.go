package provider

// Provider DTOs - Data Transfer Objects for provider API operations
type ProviderActionRequest struct {
	ID string `uri:"id"`
}

type ProviderDeleteResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ProviderDetailRequest struct {
	ID string `uri:"id"`
}

type ProviderDetailResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ProviderReloadResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ProvidersCapabilitiesRequest struct{}

type ProvidersCapabilitiesResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ProvidersDescriptorsRequest struct{}

type ProvidersDescriptorsResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ProvidersEntitiesRequest struct {
	ID string `uri:"id"`
}

type ProvidersEntitiesResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ProvidersListRequest struct {
	Page     int `form:"page"`
	PageSize int `form:"pageSize"`
}

type ProvidersListResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
