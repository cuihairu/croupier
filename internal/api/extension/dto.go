package extension

type ExtensionCatalogListRequest struct {
	Keyword  string `form:"keyword" json:"keyword"`
	Kind     string `form:"kind" json:"kind"`
	Status   string `form:"status" json:"status"`
	Page     int    `form:"page,default=1" json:"page"`
	PageSize int    `form:"page_size,default=20" json:"page_size"`
}

type ExtensionAgentSyncRequest struct {
	AgentID string `path:"agentId" json:"agent_id"`
}

type ExtensionCatalogItem struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	DisplayName    string   `json:"display_name"`
	Vendor         string   `json:"vendor"`
	Kind           string   `json:"kind"`
	Summary        string   `json:"summary"`
	IconURL        string   `json:"icon_url"`
	Status         string   `json:"status"`
	LatestVersion  string   `json:"latest_version"`
	Installed      bool     `json:"installed"`
	DefaultInstall bool     `json:"default_install"`
	Tags           []string `json:"tags"`
}

type ExtensionReleaseItem struct {
	Version        string `json:"version"`
	ReleaseChannel string `json:"release_channel"`
	MinCoreVersion string `json:"min_core_version"`
	PublishedAt    int64  `json:"published_at"`
	Changelog      string `json:"changelog"`
}

type ExtensionCatalogListResponse struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Total   int64                  `json:"total"`
	Items   []ExtensionCatalogItem `json:"items"`
}

type ExtensionCatalogDetailResponse struct {
	Code         int                    `json:"code"`
	Message      string                 `json:"message"`
	Item         *ExtensionCatalogItem  `json:"item"`
	Releases     []ExtensionReleaseItem `json:"releases"`
	Manifest     map[string]any         `json:"manifest"`
	Capabilities []string               `json:"capabilities"`
}

type ExtensionCatalogReleasesResponse struct {
	Code     int                    `json:"code"`
	Message  string                 `json:"message"`
	Total    int64                  `json:"total"`
	Releases []ExtensionReleaseItem `json:"releases"`
}

type ExtensionInstallRequest struct {
	ExtensionID    string            `json:"extension_id" binding:"required"`
	ReleaseVersion string            `json:"release_version" binding:"required"`
	ScopeType      string            `json:"scope_type" binding:"required"`
	ScopeID        string            `json:"scope_id" binding:"required"`
	TargetType     string            `json:"target_type" binding:"required"`
	TargetID       string            `json:"target_id"`
	Config         map[string]any    `json:"config"`
	SecretRefs     map[string]string `json:"secret_refs"`
}

type ExtensionInstallationListRequest struct {
	ExtensionID string `form:"extension_id" json:"extension_id"`
	ScopeType   string `form:"scope_type" json:"scope_type"`
	ScopeID     string `form:"scope_id" json:"scope_id"`
	TargetType  string `form:"target_type" json:"target_type"`
	TargetID    string `form:"target_id" json:"target_id"`
	Status      string `form:"status" json:"status"`
	Enabled     *bool  `form:"enabled" json:"enabled"`
	Page        int    `form:"page,default=1" json:"page"`
	PageSize    int    `form:"page_size,default=20" json:"page_size"`
}

type ExtensionInstallResponse struct {
	Code           int    `json:"code"`
	Message        string `json:"message"`
	InstallationID uint   `json:"installation_id"`
	Status         string `json:"status"`
}

type ExtensionInstallationItem struct {
	ID              uint   `json:"id"`
	InstallationKey string `json:"installation_key"`
	ExtensionID     string `json:"extension_id"`
	DisplayName     string `json:"display_name"`
	ReleaseVersion  string `json:"release_version"`
	ScopeType       string `json:"scope_type"`
	ScopeID         string `json:"scope_id"`
	TargetType      string `json:"target_type"`
	TargetID        string `json:"target_id"`
	Status          string `json:"status"`
	DesiredState    string `json:"desired_state"`
	Enabled         bool   `json:"enabled"`
	HealthStatus    string `json:"health_status"`
	LastError       string `json:"last_error"`
	UpdatedAt       int64  `json:"updated_at"`
}

type ExtensionInstallationListResponse struct {
	Code    int                         `json:"code"`
	Message string                      `json:"message"`
	Total   int64                       `json:"total"`
	Items   []ExtensionInstallationItem `json:"items"`
}

type ExtensionBindingItem struct {
	BindingType string `json:"binding_type"`
	BindingKey  string `json:"binding_key"`
	TargetRef   string `json:"target_ref"`
	Status      string `json:"status"`
	LastError   string `json:"last_error"`
}

type ExtensionEventItem struct {
	EventType string `json:"event_type"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Payload   string `json:"payload"`
	CreatedBy string `json:"created_by"`
	CreatedAt int64  `json:"created_at"`
}

type ExtensionInstallationDetailResponse struct {
	Code         int                        `json:"code"`
	Message      string                     `json:"message"`
	Installation *ExtensionInstallationItem `json:"installation"`
	ConfigSchema map[string]any             `json:"config_schema"`
	Config       map[string]any             `json:"config"`
	SecretRefs   map[string]string          `json:"secret_refs"`
	Bindings     []ExtensionBindingItem     `json:"bindings"`
	Events       []ExtensionEventItem       `json:"events"`
}

type ExtensionConfigUpdateRequest struct {
	Config     map[string]any    `json:"config"`
	SecretRefs map[string]string `json:"secret_refs"`
}

type ExtensionUpgradeRequest struct {
	ReleaseVersion string `json:"release_version" binding:"required"`
}

type ExtensionActionResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

type ExtensionReconcileResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
	Applied int    `json:"applied"`
	Failed  int    `json:"failed"`
}

type ExtensionConfigSchemaResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Schema  map[string]any `json:"schema"`
}

type ExtensionConfigResponse struct {
	Code       int               `json:"code"`
	Message    string            `json:"message"`
	Config     map[string]any    `json:"config"`
	SecretRefs map[string]string `json:"secret_refs"`
}

type ExtensionTestConnectionResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

type ExtensionCapabilitiesResponse struct {
	Code         int                         `json:"code"`
	Message      string                      `json:"message"`
	Capabilities []string                    `json:"capabilities"`
	Details      []ExtensionCapabilityDetail `json:"details"`
}

type ExtensionCapabilityDetail struct {
	Type       string   `json:"type"`
	Key        string   `json:"key"`
	Capability string   `json:"capability"`
	Provider   string   `json:"provider"`
	Operations []string `json:"operations"`
	Source     string   `json:"source"`
}

type ExtensionPageItem struct {
	Type   string         `json:"type"`
	Key    string         `json:"key"`
	Title  string         `json:"title"`
	Route  string         `json:"route"`
	Icon   string         `json:"icon"`
	Group  string         `json:"group"`
	Order  int            `json:"order"`
	Source string         `json:"source"`
	Schema map[string]any `json:"schema"`
}

type ExtensionPagesResponse struct {
	Code    int                 `json:"code"`
	Message string              `json:"message"`
	Pages   []ExtensionPageItem `json:"pages"`
}

type ExtensionHealthCheckResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Status    string `json:"status"`
	CheckedAt int64  `json:"checked_at"`
}

type ExtensionEventListResponse struct {
	Code    int                  `json:"code"`
	Message string               `json:"message"`
	Total   int64                `json:"total"`
	Items   []ExtensionEventItem `json:"items"`
}

type ExtensionEventListRequest struct {
	Level    string `form:"level" json:"level"`
	Keyword  string `form:"keyword" json:"keyword"`
	Page     int    `form:"page,default=1" json:"page"`
	PageSize int    `form:"page_size,default=20" json:"page_size"`
}

type ExtensionAgentSyncResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Payload interface{} `json:"payload"`
}
