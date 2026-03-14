package ticket

type Comment struct {
	Id        int64  `json:"id"`
	Content   string `json:"content"`
	Author    string `json:"author"`
	CreatedAt string `json:"createdAt"`
}

type Ticket struct {
	Id        int64    `json:"id"`
	Title     string   `json:"title"`
	Content   string   `json:"content"`
	Category  string   `json:"category"`
	Priority  string   `json:"priority"`
	Status    string   `json:"status"`
	Assignee  string   `json:"assignee"`
	Tags      []string `json:"tags"`
	PlayerId  string   `json:"playerId"`
	Contact   string   `json:"contact"`
	GameId    string   `json:"gameId"`
	Env       string   `json:"env"`
	Source    string   `json:"source"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
}

type TicketCommentCreateRequest struct {
	TicketID string `uri:"ticketId"`
	Content  string `json:"content"`
}

type TicketCommentsRequest struct {
	TicketID string `uri:"ticketId"`
}

type TicketCommentsResponse struct {
	Items []Comment `json:"items"`
}

type TicketCreateRequest struct {
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Category string   `json:"category"`
	Priority string   `json:"priority,optional,default=medium"`
	Tags     []string `json:"tags"`
	PlayerId string   `json:"playerId"`
	Contact  string   `json:"contact"`
	GameId   string   `json:"gameId"`
	Env      string   `json:"env"`
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
	Status   string `form:"status"`
	Category string `form:"category"`
	Priority string `form:"priority"`
	Assignee string `form:"assignee"`
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
