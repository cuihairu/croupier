package provider

// Provider DTOs - canonical REST contracts for provider API operations.
type ProviderActionRequest struct {
	ID string `uri:"id"`
}

type ProviderDetailRequest struct {
	ID string `uri:"id"`
}

type ProvidersCapabilitiesRequest struct{}

type ProvidersDescriptorsRequest struct{}

type ProvidersResourcesRequest struct {
	ID string `uri:"id"`
}

type ProvidersListRequest struct {
	Page     int `form:"page"`
	PageSize int `form:"pageSize"`
}

// ProviderItem is the canonical provider metadata item returned by list/detail/capabilities.
type ProviderItem map[string]interface{}

// ProvidersListResponse is the canonical REST list response for providers.
type ProvidersListResponse struct {
	Items    []ProviderItem `json:"items"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
}

// ProvidersCapabilitiesResponse is the canonical REST list response for provider capabilities.
type ProvidersCapabilitiesResponse struct {
	Items []ProviderItem `json:"items"`
	Total int            `json:"total"`
}

// ProvidersDescriptorsResponse contains provider OpenAPI manifests keyed by provider id.
type ProvidersDescriptorsResponse struct {
	ProviderManifests map[string]interface{} `json:"providerManifests"`
}

// ProviderDetailResponse returns the canonical detail payload for a single provider.
type ProviderDetailResponse = ProviderItem

// ProvidersResourcesResponse lists resources exported by one or more providers.
type ProvidersResourcesResponse struct {
	Items []map[string]interface{} `json:"items"`
	Total int                      `json:"total"`
}

// ProviderDeleteResponse confirms provider deletion.
type ProviderDeleteResponse struct {
	ID string `json:"id"`
}

// ProviderReloadResponse confirms provider reload and updated timestamp.
type ProviderReloadResponse struct {
	ID        string `json:"id"`
	UpdatedAt string `json:"updatedAt"`
}
