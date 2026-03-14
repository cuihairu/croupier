package config

// UpsertRequest represents the request to create or update a config
type UpsertRequest struct {
	Key   string `json:"key"`   // config key
	Value string `json:"value"` // config value
}

// ConfigVersion represents a single config version
type ConfigVersion struct {
	Key       string `json:"key"`
	Version   int    `json:"version"`
	CreatedBy string `json:"createdBy"`
	CreatedAt string `json:"createdAt"`
	GameID    string `json:"game_id"`
	Env       string `json:"env"`
	Format    string `json:"format"`
	Message   string `json:"message"`
	Value     string `json:"value,omitempty"`
}

// UpsertResponse represents the response for config upsert
type UpsertResponse struct {
	Version ConfigVersion `json:"version"`
}

// ListVersionsRequest represents the request to list config versions
type ListVersionsRequest struct {
	Key string `form:"key"` // config key
}

// ConfigVersionItem represents a simplified config version for list view
type ConfigVersionItem struct {
	Key          string `json:"key"`
	Version      int    `json:"version"`
	CreatedBy    string `json:"createdBy"`
	CreatedAt    string `json:"createdAt"`
	GameID       string `json:"game_id"`
	Env          string `json:"env"`
	Format       string `json:"format"`
	Message      string `json:"message"`
	Value        string `json:"value"`
}

// ListVersionsResponse represents the response with config versions list
type ListVersionsResponse struct {
	Key      string                `json:"key"`
	Total    int                   `json:"total"`
	Versions []ConfigVersionItem   `json:"versions"`
}

// GetVersionRequest represents the request to get a specific config version
type GetVersionRequest struct {
	Key     string `form:"key"`     // config key
	Version int    `form:"version"` // version number
}

// GetVersionResponse represents the response for a specific config version
type GetVersionResponse struct {
	Version ConfigVersion `json:"version"`
}

// ConfigUpsertRequest represents the request to create or update a config
type ConfigUpsertRequest struct {
	Key   string `json:"key"`   // 配置键
	Value string `json:"value"` // 配置值
}

// ConfigUpsertResponse represents the response for config upsert
type ConfigUpsertResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ConfigVersionDetailRequest represents the request to get a specific config version
type ConfigVersionDetailRequest struct {
	Key     string `form:"key"`     // 配置键
	Version int    `form:"version"` // 版本号
}

// ConfigVersionDetailResponse represents the response for a specific config version
type ConfigVersionDetailResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ConfigVersionsRequest represents the request to list config versions
type ConfigVersionsRequest struct {
	Key string `form:"key"` // 配置键
}

// ConfigVersionsResponse represents the response with config versions list
type ConfigVersionsResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
