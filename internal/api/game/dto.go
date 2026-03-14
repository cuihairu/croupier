package game

// GameCreateRequest represents the request to create a game
type GameCreateRequest struct {
	Name        string `json:"name"`
	AliasName   string `json:"aliasName"`
	Description string `json:"description"`
	Config      string `json:"config"`
}

// GameCreateResponse represents the response after creating a game
type GameCreateResponse struct {
	Game GameInfo `json:"game"`
}

// GameDeleteRequest represents the request to delete a game
type GameDeleteRequest struct {
	ID string `uri:"id"`
}

// GameDeleteResponse represents the response after deleting a game
type GameDeleteResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// GameDetailRequest represents the request to get game details
type GameDetailRequest struct {
	ID string `uri:"id"`
}

// GameDetailResponse represents the response with game details
type GameDetailResponse struct {
	Game GameInfo `json:"game"`
}

// GameEnvAddRequest represents the request to add a game environment
type GameEnvAddRequest struct {
	ID   string `uri:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// GameEnvAddResponse represents the response after adding a game environment
type GameEnvAddResponse struct {
	Envs []GameEnvItem `json:"envs"`
}

// GameEnvDeleteRequest represents the request to delete a game environment
type GameEnvDeleteRequest struct {
	ID    string `uri:"id"`
	EnvID string `uri:"envId"`
}

// GameEnvDeleteResponse represents the response after deleting a game environment
type GameEnvDeleteResponse struct {
	Envs []GameEnvItem `json:"envs"`
}

// GameEnvItem represents a game environment
type GameEnvItem struct {
	Env         string `json:"env"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

// GameEnvUpdateRequest represents the request to update a game environment
type GameEnvUpdateRequest struct {
	ID    string `uri:"id"`
	EnvID string `uri:"envId"`
	Name  string `json:"name"`
	Type  string `json:"type"`
}

// GameEnvUpdateResponse represents the response after updating a game environment
type GameEnvUpdateResponse struct {
	Envs []GameEnvItem `json:"envs"`
}

// GameEnvsData represents game environments data
type GameEnvsData struct {
	Envs []GameEnvItem `json:"envs"`
}

// GameEnvsListRequest represents the request to list game environments
type GameEnvsListRequest struct {
	ID string `uri:"id"`
}

// GameEnvsListResponse represents the response with game environments
type GameEnvsListResponse struct {
	Envs []GameEnvItem `json:"envs"`
}

// GameInfo represents game information
type GameInfo struct {
	ID          uint          `json:"id"`
	Name        string        `json:"name"`
	Icon        string        `json:"icon"`
	Description string        `json:"description"`
	Enabled     bool          `json:"enabled"`
	AliasName   string        `json:"aliasName"`
	Homepage    string        `json:"homepage"`
	Status      string        `json:"status"`
	GameType    string        `json:"gameType"`
	GenreCode   string        `json:"genreCode"`
	Color       string        `json:"color"`
	Envs        []GameEnvItem `json:"envs"`
	CreatedAt   string        `json:"createdAt"`
	UpdatedAt   string        `json:"updatedAt"`
}

// GameUpdateRequest represents the request to update a game
type GameUpdateRequest struct {
	ID          string `uri:"id"`
	Name        string `json:"name"`
	AliasName   string `json:"aliasName"`
	Description string `json:"description"`
	Config      string `json:"config"`
	Status      string `json:"status"`
}

// GameUpdateResponse represents the response after updating a game
type GameUpdateResponse struct {
	Game GameInfo `json:"game"`
}

// GamesData represents games data
type GamesData struct {
	Games []GameInfo `json:"games"`
	Total int        `json:"total"`
}

// GamesListRequest represents the request to list games
type GamesListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Status   string `form:"status"`
}

// GamesListResponse represents the response with a list of games
type GamesListResponse struct {
	Games []GameInfo `json:"games"`
	Total int        `json:"total"`
	Page  int        `json:"page"`
	Size  int        `json:"size"`
}

// Type aliases for backward compatibility with internal/types package
//
// These aliases allow code that imports types from internal/types to work
// with types from this package instead. This is useful when moving types
// from the centralized types package to more domain-specific locations.

type (
	// Deprecated: Use GameCreateRequest directly
	GameCreateReq = GameCreateRequest
	// Deprecated: Use GameCreateResponse directly
	GameCreateResp = GameCreateResponse
	// Deprecated: Use GameDeleteRequest directly
	GameDeleteReq = GameDeleteRequest
	// Deprecated: Use GameDeleteResponse directly
	GameDeleteResp = GameDeleteResponse
	// Deprecated: Use GameDetailRequest directly
	GameDetailReq = GameDetailRequest
	// Deprecated: Use GameDetailResponse directly
	GameDetailResp = GameDetailResponse
	// Deprecated: Use GameEnvAddRequest directly
	GameEnvAddReq = GameEnvAddRequest
	// Deprecated: Use GameEnvAddResponse directly
	GameEnvAddResp = GameEnvAddResponse
	// Deprecated: Use GameEnvDeleteRequest directly
	GameEnvDeleteReq = GameEnvDeleteRequest
	// Deprecated: Use GameEnvDeleteResponse directly
	GameEnvDeleteResp = GameEnvDeleteResponse
	// Deprecated: Use GameEnvUpdateRequest directly
	GameEnvUpdateReq = GameEnvUpdateRequest
	// Deprecated: Use GameEnvUpdateResponse directly
	GameEnvUpdateResp = GameEnvUpdateResponse
	// Deprecated: Use GameEnvsListRequest directly
	GameEnvsListReq = GameEnvsListRequest
	// Deprecated: Use GameEnvsListResponse directly
	GameEnvsListResp = GameEnvsListResponse
	// Deprecated: Use GameUpdateRequest directly
	GameUpdateReq = GameUpdateRequest
	// Deprecated: Use GameUpdateResponse directly
	GameUpdateResp = GameUpdateResponse
	// Deprecated: Use GamesListRequest directly
	GamesListReq = GamesListRequest
	// Deprecated: Use GamesListResponse directly
	GamesListResp = GamesListResponse
)
