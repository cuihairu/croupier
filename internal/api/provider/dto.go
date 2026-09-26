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

// SdkVersionCount 单个 SDK 版本的实例计数。
type SdkVersionCount struct {
	Version string `json:"version"`
	Count   int    `json:"count"`
}

// SdkLanguageStats 按 SDK 语言聚合的版本分布。
type SdkLanguageStats struct {
	Language string            `json:"language"`
	Count    int               `json:"count"`
	Versions []SdkVersionCount `json:"versions"`
}

// SdkInstanceItem 在线 SDK 实例明细。
type SdkInstanceItem struct {
	ProviderID   string            `json:"providerId"`
	AgentID      string            `json:"agentId"`
	GameID       string            `json:"gameId"`
	Env          string            `json:"env"`
	ServiceAddr  string            `json:"serviceAddr,omitempty"`
	SdkName      string            `json:"sdkName,omitempty"`
	SdkLanguage  string            `json:"sdkLanguage"`
	SdkVersion   string            `json:"sdkVersion"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	LastSeenUnix int64             `json:"lastSeenUnix"`
}

// SdkStatsResponse GET /api/v1/providers/sdk-stats 的响应：
// 在线 provider 实例的 SDK 语言/版本分布。
type SdkStatsResponse struct {
	TotalInstances int                `json:"totalInstances"`
	Languages      []SdkLanguageStats `json:"languages"`
	Instances      []SdkInstanceItem  `json:"instances"`
}

// SdkStatsRequest 查询参数：metaKey/metaValue 对用户实例元数据做子串过滤
// （大小写不敏感；value 条件同时匹配键名与 k=v 整对，详见 matchMetadata；
// 在线会话是内存态，线性过滤即可，无需数据库索引）。
type SdkStatsRequest struct {
	MetaKey   string `form:"metaKey,optional"`
	MetaValue string `form:"metaValue,optional"`
}
