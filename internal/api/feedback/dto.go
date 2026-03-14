package feedback

// Feedback represents user feedback
type Feedback struct {
	Id        int64  `json:"id"`
	PlayerId  string `json:"playerId"`
	Contact   string `json:"contact"`
	Content   string `json:"content"`
	Category  string `json:"category"`
	Priority  string `json:"priority"`
	Status    string `json:"status"`
	Rating    int    `json:"rating"` // 1-5星
	Attach    string `json:"attach"` // 附件URL
	GameId    string `json:"gameId"`
	Env       string `json:"env"`
	Reply     string `json:"reply"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// FeedbackStats represents feedback statistics
type FeedbackStats struct {
	Total        int            `json:"total"`
	ByCategory   map[string]int `json:"byCategory"`
	ByStatus     map[string]int `json:"byStatus"`
	AvgRating    float64        `json:"avgRating"`
	ResponseRate float64        `json:"responseRate"`
}

// FeedbackListRequest represents the request to list feedbacks
type FeedbackListRequest struct {
	Page     int    `form:"page,optional,default=1"`
	PageSize int    `form:"pageSize,optional,default=20"`
	Status   string `form:"status"`
	Category string `form:"category"`
	GameId   string `form:"gameId"`
}

// FeedbackListResponse represents the response with a list of feedbacks
type FeedbackListResponse struct {
	Items []Feedback `json:"items"`
	Total int64      `json:"total"`
	Page  int        `json:"page"`
	Size  int        `json:"pageSize"`
}

// FeedbackCreateRequest represents the request to create feedback
type FeedbackCreateRequest struct {
	PlayerId string `json:"playerId"`
	Contact  string `json:"contact"`
	Content  string `json:"content"`
	Category string `json:"category"`
	Rating   int    `json:"rating"`
	Attach   string `json:"attach"`
	GameId   string `json:"gameId"`
	Env      string `json:"env"`
}

// FeedbackCreateResponse represents the response after creating feedback
type FeedbackCreateResponse struct {
	Feedback
}

// FeedbackUpdateRequest represents the request to update feedback
type FeedbackUpdateRequest struct {
	ID       string `uri:"id"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
	Reply    string `json:"reply"`
}

// FeedbackUpdateResponse represents the response after updating feedback
type FeedbackUpdateResponse struct {
	Feedback
}

// FeedbackDeleteRequest represents the request to delete feedback
type FeedbackDeleteRequest struct {
	ID string `uri:"id"`
}

// FeedbackStatsRequest represents the request to get feedback stats
type FeedbackStatsRequest struct {
	GameId string `form:"gameId"`
	Days   int    `form:"days,optional,default=7"`
}

// FeedbackStatsResponse represents the response with feedback stats
type FeedbackStatsResponse struct {
	FeedbackStats
}

// FeedbackDetailResponse represents the response with feedback details
type FeedbackDetailResponse struct {
	Feedback
}

// FeatureAdoption represents feature adoption metrics
type FeatureAdoption struct {
	Feature      string  `json:"feature"`
	Users        int     `json:"users"`
	AdoptionRate float64 `json:"adoptionRate"`
	Frequency    float64 `json:"frequency"`
}
