package ticket

type Comment struct {
	Id        int64  `json:"id"`
	Content   string `json:"content"`
	Author    string `json:"author"`
	CreatedAt string `json:"createdAt"`
}

type Ticket struct {
	Id       int64    `json:"id"`
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Category string   `json:"category"`
	Priority string   `json:"priority"`
	Status   string   `json:"status"`
	Assignee string   `json:"assignee"`
	Tags     []string `json:"tags"`
	PlayerId string   `json:"playerId"`
	Contact  string   `json:"contact"`
	GameId   string   `json:"gameId"`
	Env      string   `json:"env"`
	Source   string   `json:"source"`
	// 玩家上下文（game-support P1；见 docs/research/game-support-systems.md）
	ServerId    string                 `json:"serverId,omitempty"`
	PlayerLevel int                    `json:"playerLevel,omitempty"`
	DeviceOS    string                 `json:"deviceOs,omitempty"`
	DeviceModel string                 `json:"deviceModel,omitempty"`
	Language    string                 `json:"language,omitempty"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
	// CSAT 满意度评分（1-5，0=未评；仅已解决/已关闭后可提交）
	Rating    int    `json:"rating,omitempty"`
	RatedBy   string `json:"ratedBy,omitempty"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type TicketCommentCreateRequest struct {
	TicketID string `uri:"id"`
	Content  string `json:"content"`
}

type TicketCommentsRequest struct {
	TicketID string `uri:"id"`
}

type TicketCommentsResponse struct {
	Items []Comment `json:"items"`
}

type TicketCreateRequest struct {
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Category string   `json:"category"`
	Priority string   `json:"priority"`
	Tags     []string `json:"tags"`
	PlayerId string   `json:"playerId"`
	Contact  string   `json:"contact"`
	GameId   string   `json:"gameId"`
	Env      string   `json:"env"`
	Assignee string   `json:"assignee"`
	// 玩家上下文（game-support P1）
	ServerId    string                 `json:"serverId,optional"`
	PlayerLevel int                    `json:"playerLevel,optional"`
	DeviceOS    string                 `json:"deviceOs,optional"`
	DeviceModel string                 `json:"deviceModel,optional"`
	Language    string                 `json:"language,optional"`
	Extra       map[string]interface{} `json:"extra,optional"`
}

type TicketDeleteRequest struct {
	ID string `uri:"id"`
}

type TicketDetailRequest struct {
	ID string `uri:"id"`
}

type TicketDetailResponse struct {
	Ticket
	Comments []Comment `json:"comments,omitempty"`
}

type TicketTransitionRequest struct {
	ID     string `uri:"id"`
	Status string `json:"status"` // open, in_progress, resolved, closed
	Note   string `json:"note"`
}

type TicketUpdateRequest struct {
	ID       string   `uri:"id"`
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Category string   `json:"category"`
	Priority string   `json:"priority"`
	Assignee string   `json:"assignee"`
	Tags     []string `json:"tags"`
}

type TicketsListRequest struct {
	Page     int    `form:"page,optional,default=1"`
	PageSize int    `form:"pageSize,optional,default=20"`
	Query    string `form:"q"`
	Status   string `form:"status"`
	Category string `form:"category"`
	Priority string `form:"priority"`
	Assignee string `form:"assignee"`
	GameID   string `form:"gameId"`
	Env      string `form:"env"`
}

type TicketsListResponse struct {
	Items []Ticket `json:"items"`
	Total int64    `json:"total"`
	Page  int      `json:"page"`
	Size  int      `json:"pageSize"`
}

// Type aliases for service layer compatibility
type ListRequest = TicketsListRequest
type ListResponse = TicketsListResponse
type CreateRequest = TicketCreateRequest
type CreateResponse = TicketDetailResponse
type GetRequest = TicketDetailRequest
type GetResponse = TicketDetailResponse
type UpdateRequest = TicketUpdateRequest
type UpdateResponse = TicketDetailResponse
type DeleteRequest = TicketDeleteRequest
type DeleteResponse = TicketDetailResponse
type TransitionRequest = TicketTransitionRequest
type TransitionResponse = TicketDetailResponse
type GetCommentsRequest = TicketCommentsRequest
type GetCommentsResponse = TicketCommentsResponse
type CreateCommentRequest = TicketCommentCreateRequest
type CreateCommentResponse = TicketCommentsResponse

// RateRequest submits the satisfaction rating after a ticket closes
// (game-support P2 CSAT).
type RateRequest struct {
	ID     string `uri:"id"`
	Rating int    `json:"rating"` // 1-5
}

// RateResponse confirms the stored rating.
type RateResponse struct {
	TicketID int64 `json:"ticketId"`
	Rating   int   `json:"rating"`
}
