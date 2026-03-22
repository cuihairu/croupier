package config

// UpsertRequest represents the legacy request to create or update a config.
// Source of truth for new HTTP clients should be SaveConfigRequest.
type UpsertRequest struct {
	Key   string `json:"key"`   // config key
	Value string `json:"value"` // config value
}

// ConfigVersion represents a single config version.
// Canonical HTTP JSON contract should use camelCase keys.
type ConfigVersion struct {
	Key       string `json:"key"`
	Version   int    `json:"version"`
	CreatedBy string `json:"createdBy"`
	CreatedAt string `json:"createdAt"`
	GameID    string `json:"gameId"`
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
	Key       string `json:"key"`
	Version   int    `json:"version"`
	CreatedBy string `json:"createdBy"`
	CreatedAt string `json:"createdAt"`
	GameID    string `json:"gameId"`
	Env       string `json:"env"`
	Format    string `json:"format"`
	Message   string `json:"message"`
	Value     string `json:"value"`
}

// ListVersionsResponse represents the response with config versions list
type ListVersionsResponse struct {
	Key      string              `json:"key"`
	Total    int                 `json:"total"`
	Versions []ConfigVersionItem `json:"versions"`
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

// ListConfigsRequest defines the filters for GET /api/v1/configs.
type ListConfigsRequest struct {
	GameID string `form:"gameId"`
	Env    string `form:"env"`
	Format string `form:"format"`
	IDLike string `form:"idLike"`
}

// ConfigItem represents the latest version summary for a config key.
type ConfigItem struct {
	ID             string `json:"id"`
	Format         string `json:"format"`
	GameID         string `json:"gameId"`
	Env            string `json:"env"`
	LatestVersion  int    `json:"latestVersion"`
	UpdatedAt      string `json:"updatedAt"`
	LastMessage    string `json:"lastMessage"`
	LastModifiedBy string `json:"lastModifiedBy"`
}

// ListConfigsResponse returns the latest version summary for each config key.
type ListConfigsResponse struct {
	Items []ConfigItem `json:"items"`
}

// GetConfigRequest defines the resource identity for GET /api/v1/configs/:id.
type GetConfigRequest struct {
	ID string `uri:"id"`
}

// GetConfigResponse returns the latest editable content for a config key.
type GetConfigResponse struct {
	ID      string `json:"id"`
	Format  string `json:"format"`
	Content string `json:"content"`
	Version int    `json:"version"`
	GameID  string `json:"gameId"`
	Env     string `json:"env"`
}

// SaveConfigRequest defines the canonical write contract for PUT /api/v1/configs/:id.
type SaveConfigRequest struct {
	Format      string `json:"format"`
	Content     string `json:"content"`
	Message     string `json:"message"`
	BaseVersion int    `json:"baseVersion"`
	GameID      string `json:"gameId"`
	Env         string `json:"env"`
}

// SaveConfigResponse returns the newly created version metadata.
type SaveConfigResponse struct {
	Version int `json:"version"`
}

// ValidateConfigRequest defines the payload for POST /api/v1/configs/:id/validate.
type ValidateConfigRequest struct {
	Format  string `json:"format"`
	Content string `json:"content"`
}

// ValidateConfigResponse returns parse validation results only.
type ValidateConfigResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors"`
}
