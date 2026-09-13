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
	Description string `json:"description"`
	Category    string `json:"category"`
	Icon        string `json:"icon"`
	GameID      string `json:"gameId"`
	Env         string `json:"env"`
	Sort        int    `json:"sort"`
}

type ToolCreateResponse struct {
	Tool
}

type ToolUpdateRequest struct {
	ID          string  `uri:"id"`
	Name        string  `json:"name"`
	URL         *string `json:"url"`
	Description *string `json:"description"`
	Category    *string `json:"category"`
	Icon        *string `json:"icon"`
	Sort        *int    `json:"sort"`
	Enabled     *bool   `json:"enabled"`
	GameID      *string `json:"gameId"`
	Env         *string `json:"env"`
}

type ToolUpdateResponse struct {
	Tool
}

type ToolDeleteRequest struct {
	ID string `uri:"id"`
}
