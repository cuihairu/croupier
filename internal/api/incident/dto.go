package incident

import "time"

// ---- Category (incident_categories) DTOs ----

// CategoryAudience is the visibility scope of a category (roles + explicit accounts).
type CategoryAudience struct {
	Roles []string `json:"roles"`
	Users []string `json:"users"`
}

// CategoryDTO is the response body for incident_categories.
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

// CategoryUpsertRequest is the request body for creating/updating a category
// (slug may only be passed on create and is immutable afterwards; pointer fields
// mean partial-update semantics, and name is required on create).
type CategoryUpsertRequest struct {
	Name          *string           `json:"name,omitempty"`
	Slug          string            `json:"slug,omitempty"`
	Sort          *int              `json:"sort,omitempty"`
	Leader        *string           `json:"leader,omitempty"`
	Subcategories *[]string         `json:"subcategories,omitempty"`
	Audience      *CategoryAudience `json:"audience,omitempty"`
	Enabled       *bool             `json:"enabled,omitempty"`
}

// CategoryUsageResponse is the cascade hint counters shown before deleting a category.
type CategoryUsageResponse struct {
	Incidents int64 `json:"incidents"`
	Bugs      int64 `json:"bugs"`
}

// CategoryListResponse is the category list.
type CategoryListResponse struct {
	Items []CategoryDTO `json:"items"`
	Total int64         `json:"total"`
}

// ---- External REST API (incident-reports section 9) ----

// ExternalTokenDTO is the response body for external tokens.
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

// CreateExternalTokenRequest creates a new external token.
type CreateExternalTokenRequest struct {
	Name    string                 `json:"name"`
	Scope   map[string]interface{} `json:"scope,omitempty"`
	Enabled *bool                  `json:"enabled,omitempty"`
}

// UpdateExternalTokenRequest updates external token metadata.
type UpdateExternalTokenRequest struct {
	Name    *string                 `json:"name,omitempty"`
	Scope   *map[string]interface{} `json:"scope,omitempty"`
	Enabled *bool                   `json:"enabled,omitempty"`
}

// ListTokensResponse is the paginated list.
type ListTokensResponse struct {
	Items []ExternalTokenDTO `json:"items"`
	Total int64              `json:"total"`
}

// CreateIncidentFromExternalRequest is the external incident registration request body.
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

// QueryExternalIncidentsRequest holds the external query parameters.
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

// ---- Incident (incidents) DTOs ----

// IncidentDTO is the response body for incidents.
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

// IncidentCreateRequest registers an incident (shared by the panel and the
// automatic-source conversion path; the category is required and must be an enabled row).
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
	// Source is the explicit source (the external API overrides it with "external");
	// when empty it is derived from refType.
	Source string `json:"source,omitempty"`
}

// IncidentUpdateRequest updates an incident (title, category, subcategory, severity,
// attribution, relations and details are all mutable).
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

// IncidentStatusRequest is the status transition request
// (open -> acknowledged -> resolved, resolved may be reopened).
type IncidentStatusRequest struct {
	Status     string     `json:"status"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
}

// IncidentListResponse is the paginated incident list.
type IncidentListResponse struct {
	Items []IncidentDTO `json:"items"`
	Total int64         `json:"total"`
}
