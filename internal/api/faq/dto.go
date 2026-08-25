package faq

// FAQ represents a frequently asked question
type FAQ struct {
	Id       int64    `json:"id"`
	Question string   `json:"question"`
	Answer   string   `json:"answer"`
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
	Visible  bool     `json:"visible"`
	Sort     int      `json:"sort"`
	Views    int      `json:"views"`
	Slug     string   `json:"slug,omitempty"`
	Summary  string   `json:"summary,omitempty"`
	// Vote counters feed the content governance queue (helpful ratio).
	HelpfulCount   int    `json:"helpfulCount"`
	UnhelpfulCount int    `json:"unhelpfulCount"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

// FAQCategory represents a FAQ category
type FAQCategory struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// FAQListRequest represents the request to list FAQs
type FAQListRequest struct {
	Page     int    `form:"page,optional,default=1"`
	PageSize int    `form:"pageSize,optional,default=20"`
	Category string `form:"category"`
	Keyword  string `form:"keyword"`
	Tag      string `form:"tag"`
	Visible  *bool  `form:"visible"`
	// orderBy=helpful sorts by vote activity first (governance view).
	OrderBy string `form:"orderBy"`
}

// FAQListResponse represents the response with a list of FAQs
type FAQListResponse struct {
	Items []FAQ `json:"items"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"pageSize"`
}

// FAQCreateRequest represents the request to create an FAQ
type FAQCreateRequest struct {
	Question string   `json:"question"`
	Answer   string   `json:"answer"`
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
	Visible  bool     `json:"visible,optional,default=true"`
	Sort     int      `json:"sort,optional,default=0"`
	Slug     string   `json:"slug,optional"`
	Summary  string   `json:"summary,optional"`
}

// FAQCreateResponse represents the response after creating an FAQ
type FAQCreateResponse struct {
	FAQ
}

// FAQUpdateRequest represents the request to update an FAQ
type FAQUpdateRequest struct {
	ID       string   `uri:"id"`
	Question string   `json:"question"`
	Answer   string   `json:"answer"`
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
	Visible  *bool    `json:"visible"`
	Sort     *int     `json:"sort"`
	Slug     *string  `json:"slug"`
	Summary  *string  `json:"summary"`
}

// FAQUpdateResponse represents the response after updating an FAQ
type FAQUpdateResponse struct {
	FAQ
}

// FAQDeleteRequest represents the request to delete an FAQ
type FAQDeleteRequest struct {
	ID string `uri:"id"`
}

// FAQVoteRequest records player feedback on an FAQ entry (helpful or not).
type FAQVoteRequest struct {
	ID      string `uri:"id"`
	Helpful bool   `json:"helpful"`
}

// FAQVoteResponse returns the updated counters.
type FAQVoteResponse struct {
	HelpfulCount   int `json:"helpfulCount"`
	UnhelpfulCount int `json:"unhelpfulCount"`
}

// FAQDetailResponse represents the response with FAQ details
type FAQDetailResponse struct {
	FAQ
}

// FAQCategoriesRequest represents the request to get FAQ categories
type FAQCategoriesRequest struct{}

// FAQCategoriesResponse represents the response with FAQ categories
type FAQCategoriesResponse struct {
	Items []FAQCategory `json:"items"`
}
