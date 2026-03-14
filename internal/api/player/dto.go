package player

// Player represents a player entity in the system.
type Player struct {
	Id        int64  `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	GameId    string `json:"gameId"`
	Status    int    `json:"status"`  // 1:active 0:banned 2:suspended
	Balance   int64  `json:"balance"` // 游戏货币
	Level     int    `json:"level"`
	Vip       int    `json:"vip"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// PlayerBalanceRequest represents a request to update player balance.
type PlayerBalanceRequest struct {
	ID     string `uri:"id"`
	Amount int64  `json:"amount"`
	Reason string `json:"reason"`
}

// PlayerBalanceResponse represents the response after updating player balance.
type PlayerBalanceResponse struct {
	Player
}

// PlayerCreateRequest represents a request to create a new player.
type PlayerCreateRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	GameId   string `json:"gameId"`
}

// PlayerCreateResponse represents the response after creating a player.
type PlayerCreateResponse struct {
	Player
}

// PlayerDeleteRequest represents a request to delete a player.
type PlayerDeleteRequest struct {
	ID string `uri:"id"`
}

// PlayerDetailRequest represents a request to get player details.
type PlayerDetailRequest struct {
	ID string `uri:"id"`
}

// PlayerDetailResponse represents the response with player details.
type PlayerDetailResponse struct {
	Player
}

// PlayerUpdateRequest represents a request to update player information.
type PlayerUpdateRequest struct {
	ID       string `uri:"id"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Status   int    `json:"status"`
	Level    int    `json:"level"`
	Vip      int    `json:"vip"`
}

// PlayerUpdateResponse represents the response after updating a player.
type PlayerUpdateResponse struct {
	Player
}

// PlayersListRequest represents a request to list players with filtering.
type PlayersListRequest struct {
	Page     int    `form:"page,optional,default=1"`
	PageSize int    `form:"pageSize,optional,default=20"`
	GameId   string `form:"gameId"`
	Search   string `form:"search"`
	Status   int    `form:"status"`
	Level    int    `form:"level"`
	Vip      int    `form:"vip"`
}

// PlayersListResponse represents the paginated response of players list.
type PlayersListResponse struct {
	Items []Player `json:"items"`
	Total int64    `json:"total"`
	Page  int      `json:"page"`
	Size  int      `json:"pageSize"`
}

// Type aliases for backward compatibility with internal/types package
// These aliases allow gradual migration from types.Player to player.Player
type (
	// PlayerAlias is an alias for Player for backward compatibility
	PlayerAlias = Player

	// PlayerBalanceRequestAlias is an alias for PlayerBalanceRequest
	PlayerBalanceRequestAlias = PlayerBalanceRequest

	// PlayerBalanceResponseAlias is an alias for PlayerBalanceResponse
	PlayerBalanceResponseAlias = PlayerBalanceResponse

	// PlayerCreateRequestAlias is an alias for PlayerCreateRequest
	PlayerCreateRequestAlias = PlayerCreateRequest

	// PlayerCreateResponseAlias is an alias for PlayerCreateResponse
	PlayerCreateResponseAlias = PlayerCreateResponse

	// PlayerDeleteRequestAlias is an alias for PlayerDeleteRequest
	PlayerDeleteRequestAlias = PlayerDeleteRequest

	// PlayerDetailRequestAlias is an alias for PlayerDetailRequest
	PlayerDetailRequestAlias = PlayerDetailRequest

	// PlayerDetailResponseAlias is an alias for PlayerDetailResponse
	PlayerDetailResponseAlias = PlayerDetailResponse

	// PlayerUpdateRequestAlias is an alias for PlayerUpdateRequest
	PlayerUpdateRequestAlias = PlayerUpdateRequest

	// PlayerUpdateResponseAlias is an alias for PlayerUpdateResponse
	PlayerUpdateResponseAlias = PlayerUpdateResponse

	// PlayersListRequestAlias is an alias for PlayersListRequest
	PlayersListRequestAlias = PlayersListRequest

	// PlayersListResponseAlias is an alias for PlayersListResponse
	PlayersListResponseAlias = PlayersListResponse
)
