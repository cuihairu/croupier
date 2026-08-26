package tool

type Tool struct {
	Id          int64  `json:"id"`
	Name        string `json:"name"`
	Url         string `json:"url"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category"`
	Icon        string `json:"icon,omitempty"`
	GameId      string `json:"gameId,omitempty"`
	Env         string `json:"env,omitempty"`
	Enabled     bool   `json:"enabled"`
	Sort        int    `json:"sort"`
	CreatedBy   string `json:"createdBy,omitempty"`
	UpdatedAt   string `json:"updatedAt"`
}

type ToolListRequest struct {
	GameID string `form:"gameId,optional"`
	Env    string `form:"env,optional"`
}

type ToolListResponse struct {
	Items []Tool `json:"items"`
}

type ToolCreateRequest struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description,optional"`
	Category    string `json:"category,optional"`
	Icon        string `json:"icon,optional"`
	GameID      string `json:"gameId,optional"`
	Env         string `json:"env,optional"`
	Sort        int    `json:"sort,optional"`
}

type ToolCreateResponse struct {
	Tool
}

type ToolUpdateRequest struct {
	ID          string  `uri:"id"`
	Name        string  `json:"name,optional"`
	URL         *string `json:"url,optional"`
	Description *string `json:"description,optional"`
	Category    *string `json:"category,optional"`
	Icon        *string `json:"icon,optional"`
	Sort        *int    `json:"sort,optional"`
	Enabled     *bool   `json:"enabled,optional"`
	GameID      *string `json:"gameId,optional"`
	Env         *string `json:"env,optional"`
}

type ToolUpdateResponse struct {
	Tool
}

type ToolDeleteRequest struct {
	ID string `uri:"id"`
}
