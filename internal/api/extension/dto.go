package extension

type ExtensionCatalogListRequest struct {
	Keyword  string `form:"keyword" json:"keyword"`
	Kind     string `form:"kind" json:"kind"`
	Status   string `form:"status" json:"status"`
	Page     int    `form:"page,default=1" json:"page"`
	PageSize int    `form:"pageSize,default=20" json:"pageSize"`
}

type ExtensionAgentSyncRequest struct {
	AgentID string `path:"agentId" json:"agentId"`
}

type ExtensionCatalogItem struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	DisplayName    string   `json:"displayName"`
	Vendor         string   `json:"vendor"`
	Kind           string   `json:"kind"`
	Summary        string   `json:"summary"`
	IconURL        string   `json:"iconUrl"`
	Status         string   `json:"status"`
	LatestVersion  string   `json:"latestVersion"`
	Installed      bool     `json:"installed"`
	DefaultInstall bool     `json:"defaultInstall"`
	Tags           []string `json:"tags"`
}

type ExtensionReleaseItem struct {
	Version        string `json:"version"`
	ReleaseChannel string `json:"releaseChannel"`
	MinCoreVersion string `json:"minCoreVersion"`
	PublishedAt    int64  `json:"publishedAt"`
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
	ExtensionID    string            `json:"extensionId" binding:"required"`
	ReleaseVersion string            `json:"releaseVersion" binding:"required"`
	ScopeType      string            `json:"scopeType" binding:"required"`
	ScopeID        string            `json:"scopeId" binding:"required"`
	TargetType     string            `json:"targetType" binding:"required"`
	TargetID       string            `json:"targetId"`
	Config         map[string]any    `json:"config"`
	SecretRefs     map[string]string `json:"secretRefs"`
}

type ExtensionInstallationListRequest struct {
	ExtensionID string `form:"extensionId" json:"extensionId"`
	ScopeType   string `form:"scopeType" json:"scopeType"`
	ScopeID     string `form:"scopeId" json:"scopeId"`
	TargetType  string `form:"targetType" json:"targetType"`
	TargetID    string `form:"targetId" json:"targetId"`
	Status      string `form:"status" json:"status"`
	Enabled     *bool  `form:"enabled" json:"enabled"`
	Page        int    `form:"page,default=1" json:"page"`
	PageSize    int    `form:"pageSize,default=20" json:"pageSize"`
}

type ExtensionInstallResponse struct {
	Code           int    `json:"code"`
	Message        string `json:"message"`
	InstallationID uint   `json:"installationId"`
	Status         string `json:"status"`
}

type ExtensionInstallationItem struct {
	ID              uint   `json:"id"`
	InstallationKey string `json:"installationKey"`
	ExtensionID     string `json:"extensionId"`
	DisplayName     string `json:"displayName"`
	ReleaseVersion  string `json:"releaseVersion"`
	ScopeType       string `json:"scopeType"`
	ScopeID         string `json:"scopeId"`
	TargetType      string `json:"targetType"`
	TargetID        string `json:"targetId"`
	Status          string `json:"status"`
	DesiredState    string `json:"desiredState"`
	Enabled         bool   `json:"enabled"`
	HealthStatus    string `json:"healthStatus"`
	LastError       string `json:"lastError"`
	UpdatedAt       int64  `json:"updatedAt"`
}

type ExtensionInstallationListResponse struct {
	Code    int                         `json:"code"`
	Message string                      `json:"message"`
	Total   int64                       `json:"total"`
	Items   []ExtensionInstallationItem `json:"items"`
}

type ExtensionBindingItem struct {
	BindingType string `json:"bindingType"`
	BindingKey  string `json:"bindingKey"`
	TargetRef   string `json:"targetRef"`
	Status      string `json:"status"`
	LastError   string `json:"lastError"`
}

type ExtensionEventItem struct {
	EventType string `json:"eventType"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Payload   string `json:"payload"`
	CreatedBy string `json:"createdBy"`
	CreatedAt int64  `json:"createdAt"`
}

type ExtensionInstallationDetailResponse struct {
	Code         int                        `json:"code"`
	Message      string                     `json:"message"`
	Installation *ExtensionInstallationItem `json:"installation"`
	ConfigSchema map[string]any             `json:"configSchema"`
	Config       map[string]any             `json:"config"`
	SecretRefs   map[string]string          `json:"secretRefs"`
	Bindings     []ExtensionBindingItem     `json:"bindings"`
	Events       []ExtensionEventItem       `json:"events"`
}

type ExtensionConfigUpdateRequest struct {
	Config     map[string]any    `json:"config"`
	SecretRefs map[string]string `json:"secretRefs"`
}

type ExtensionUpgradeRequest struct {
	ReleaseVersion string `json:"releaseVersion" binding:"required"`
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
	SecretRefs map[string]string `json:"secretRefs"`
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
	Type        string            `json:"type"`
	Key         string            `json:"key"`
	Capability  string            `json:"capability"`
	Provider    string            `json:"provider"`
	Operations  []string          `json:"operations"`
	Permissions map[string]string `json:"permissions,omitempty"`
	ConfigKeys  []string          `json:"configKeys,omitempty"`
	Source      string            `json:"source"`
}

type ExtensionPageItem struct {
	Type               string         `json:"type"`
	Key                string         `json:"key"`
	Title              string         `json:"title"`
	Route              string         `json:"route"`
	Icon               string         `json:"icon"`
	Group              string         `json:"group"`
	Order              int            `json:"order"`
	RequiredPermission string         `json:"requiredPermission,omitempty"`
	Source             string         `json:"source"`
	Schema             map[string]any `json:"schema"`
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
	CheckedAt int64  `json:"checkedAt"`
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
	PageSize int    `form:"pageSize,default=20" json:"pageSize"`
}

type ExtensionAgentSyncResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Payload interface{} `json:"payload"`
}
