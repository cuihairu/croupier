package incident

import "time"

// ---- 类别（incident_categories）DTO ----

// CategoryAudience 类别可见范围（角色 + 显式账号）。
type CategoryAudience struct {
	Roles []string `json:"roles"`
	Users []string `json:"users"`
}

// CategoryDTO 是 incident_categories 的响应体。
type CategoryDTO struct {
	ID            uint              `json:"id"`
	Name          string            `json:"name"`
	Slug          string            `json:"slug"`
	Sort          int               `json:"sort"`
	Leader        string            `json:"leader"`
	Subcategories []string          `json:"subcategories"`
	Audience      *CategoryAudience `json:"audience,omitempty"`
	Enabled       bool              `json:"enabled"`
	Builtin       bool              `json:"builtin"`
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
}

// CategoryUpsertRequest 创建/更新类别的请求体（slug 仅创建可传，建后不可改；
// 指针字段=部分更新语义，create 时 name 必填）。
type CategoryUpsertRequest struct {
	Name          *string           `json:"name,omitempty"`
	Slug          string            `json:"slug,omitempty"`
	Sort          *int              `json:"sort,omitempty"`
	Leader        *string           `json:"leader,omitempty"`
	Subcategories *[]string         `json:"subcategories,omitempty"`
	Audience      *CategoryAudience `json:"audience,omitempty"`
	Enabled       *bool             `json:"enabled,omitempty"`
}

// CategoryUsageResponse 删除类别前的级联提示计数。
type CategoryUsageResponse struct {
	Incidents int64 `json:"incidents"`
	Bugs      int64 `json:"bugs"`
}

// CategoryListResponse 类别列表。
type CategoryListResponse struct {
	Items []CategoryDTO `json:"items"`
	Total int64         `json:"total"`
}

// ---- 对外 REST API（incident-reports §9）----

// ExternalTokenDTO 外部令牌响应体。
type ExternalTokenDTO struct {
	ID         uint                   `json:"id"`
	Name       string                 `json:"name"`
	Scope      map[string]interface{} `json:"scope,omitempty"`
	Enabled    bool                   `json:"enabled"`
	LastUsedAt *time.Time             `json:"lastUsedAt,omitempty"`
	CreatedBy  string                 `json:"createdBy,omitempty"`
	CreatedAt  time.Time              `json:"createdAt"`
	UpdatedAt  time.Time              `json:"updatedAt"`
	PlainText  string                 `json:"plainText,omitempty"`
}

// CreateExternalTokenRequest 新建外部令牌。
type CreateExternalTokenRequest struct {
	Name    string                 `json:"name"`
	Scope   map[string]interface{} `json:"scope,omitempty"`
	Enabled *bool                  `json:"enabled,omitempty"`
}

// UpdateExternalTokenRequest 更新外部令牌元数据。
type UpdateExternalTokenRequest struct {
	Name    *string                 `json:"name,omitempty"`
	Scope   *map[string]interface{} `json:"scope,omitempty"`
	Enabled *bool                   `json:"enabled,omitempty"`
}

// ListTokensResponse 分页列表。
type ListTokensResponse struct {
	Items []ExternalTokenDTO `json:"items"`
	Total int64              `json:"total"`
}

// CreateIncidentFromExternalRequest 外部登记事故请求体。
type CreateIncidentFromExternalRequest struct {
	Title           string                 `json:"title"`
	CategoryID      uint                   `json:"categoryId"`
	Subcategory     string                 `json:"subcategory,omitempty"`
	Severity        string                 `json:"severity,omitempty"`
	DetectedAt      *time.Time             `json:"detectedAt,omitempty"`
	ResponsibleType string                 `json:"responsibleType,omitempty"`
	ResponsibleID   string                 `json:"responsibleId,omitempty"`
	GameID          string                 `json:"gameId,omitempty"`
	Env             string                 `json:"env,omitempty"`
	ExecLogIDs      []int64                `json:"execLogIds,omitempty"`
	RefType         string                 `json:"refType,omitempty"`
	RefID           string                 `json:"refId,omitempty"`
	Details         map[string]interface{} `json:"details,omitempty"`
	Source          string                 `json:"source,omitempty"`
}

// QueryExternalIncidentsRequest 外部查询参数。
type QueryExternalIncidentsRequest struct {
	CategoryID  *uint      `json:"categoryId,omitempty"`
	Subcategory *string    `json:"subcategory,omitempty"`
	Status      *string    `json:"status,omitempty"`
	Severity    *string    `json:"severity,omitempty"`
	Source      *string    `json:"source,omitempty"`
	From        *time.Time `json:"from,omitempty"`
	To          *time.Time `json:"to,omitempty"`
	Page        *int       `json:"page,omitempty"`
	PageSize    *int       `json:"pageSize,omitempty"`
}

// ---- 事故（incidents）DTO ----

// IncidentDTO 是 incidents 的响应体。
type IncidentDTO struct {
	ID              uint                   `json:"id"`
	IncidentKey     string                 `json:"incidentKey,omitempty"`
	Title           string                 `json:"title"`
	CategoryID      uint                   `json:"categoryId"`
	CategorySlug    string                 `json:"categorySlug,omitempty"`
	CategoryName    string                 `json:"categoryName,omitempty"`
	Subcategory     string                 `json:"subcategory,omitempty"`
	Severity        string                 `json:"severity"`
	Status          string                 `json:"status"`
	Source          string                 `json:"source"`
	ResponsibleType string                 `json:"responsibleType,omitempty"`
	ResponsibleID   string                 `json:"responsibleId,omitempty"`
	DetectedAt      time.Time              `json:"detectedAt"`
	ResolvedAt      *time.Time             `json:"resolvedAt,omitempty"`
	GameID          string                 `json:"gameId,omitempty"`
	Env             string                 `json:"env,omitempty"`
	ExecLogIDs      []int64                `json:"execLogIds,omitempty"`
	RefType         string                 `json:"refType,omitempty"`
	RefID           string                 `json:"refId,omitempty"`
	Details         map[string]interface{} `json:"details,omitempty"`
	CreatedBy       string                 `json:"createdBy,omitempty"`
	CreatedAt       time.Time              `json:"createdAt"`
	UpdatedAt       time.Time              `json:"updatedAt"`
}

// IncidentCreateRequest 登记事故（面板/自动源转换共用；类别必选且须启用行）。
type IncidentCreateRequest struct {
	Title           string                 `json:"title"`
	CategoryID      uint                   `json:"categoryId"`
	Subcategory     string                 `json:"subcategory,omitempty"`
	Severity        string                 `json:"severity"`
	DetectedAt      *time.Time             `json:"detectedAt,omitempty"`
	ResponsibleType string                 `json:"responsibleType,omitempty"`
	ResponsibleID   string                 `json:"responsibleId,omitempty"`
	GameID          string                 `json:"gameId,omitempty"`
	Env             string                 `json:"env,omitempty"`
	ExecLogIDs      []int64                `json:"execLogIds,omitempty"`
	RefType         string                 `json:"refType,omitempty"`
	RefID           string                 `json:"refId,omitempty"`
	Details         map[string]interface{} `json:"details,omitempty"`
	IncidentKey     string                 `json:"incidentKey,omitempty"`
	// Source 显式来源（外部 API 批次覆写 external）；空时按 refType 推导。
	Source string `json:"source,omitempty"`
}

// IncidentUpdateRequest 更新事故（可改标题/类别/子类/严重度/归因/关联/明细）。
type IncidentUpdateRequest struct {
	Title           *string                `json:"title,omitempty"`
	CategoryID      *uint                  `json:"categoryId,omitempty"`
	Subcategory     *string                `json:"subcategory,omitempty"`
	Severity        *string                `json:"severity,omitempty"`
	ResponsibleType *string                `json:"responsibleType,omitempty"`
	ResponsibleID   *string                `json:"responsibleId,omitempty"`
	GameID          *string                `json:"gameId,omitempty"`
	Env             *string                `json:"env,omitempty"`
	ExecLogIDs      *[]int64               `json:"execLogIds,omitempty"`
	RefType         *string                `json:"refType,omitempty"`
	RefID           *string                `json:"refId,omitempty"`
	Details         map[string]interface{} `json:"details,omitempty"`
}

// IncidentStatusRequest 状态流转请求（open→acknowledged→resolved，resolved 可重开）。
type IncidentStatusRequest struct {
	Status     string     `json:"status"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
}

// IncidentListResponse 事故分页列表。
type IncidentListResponse struct {
	Items []IncidentDTO `json:"items"`
	Total int64         `json:"total"`
}
