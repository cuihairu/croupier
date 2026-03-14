package rate_limit

// RateLimit represents a rate limit rule
type RateLimit struct {
	Id        string      `json:"id"`
	Name      string      `json:"name"`
	Resource  string      `json:"resource"` // function, api, user
	Limit     int         `json:"limit"`    // 每秒请求数
	Window    int         `json:"window"`   // 时间窗口（秒）
	Action    string      `json:"action"`   // reject, throttle
	Rules     interface{} `json:"rules"`
	Status    int         `json:"status"`
	UpdatedAt string      `json:"updatedAt"`
}

// RateLimitsListRequest represents the request to list rate limits
type RateLimitsListRequest struct {
	Resource string `form:"resource"`
}

// RateLimitsListResponse represents the response with a list of rate limits
type RateLimitsListResponse struct {
	Items []RateLimit `json:"items"`
}

// RateLimitGetRequest represents the request to get a rate limit
type RateLimitGetRequest struct {
	ID string `uri:"id"`
}

// RateLimitGetResponse represents the response with rate limit details
type RateLimitGetResponse struct {
	RateLimit
}

// RateLimitUpsertRequest represents the request to upsert a rate limit
type RateLimitUpsertRequest struct {
	Name     string      `json:"name"`
	Resource string      `json:"resource"`
	Limit    int         `json:"limit"`
	Window   int         `json:"window"`
	Action   string      `json:"action"`
	Rules    interface{} `json:"rules"`
}

// RateLimitUpsertResponse represents the response after upserting a rate limit
type RateLimitUpsertResponse struct {
	RateLimit
}

// RateLimitDeleteRequest represents the request to delete a rate limit
type RateLimitDeleteRequest struct {
	ID string `uri:"id"`
}

// RateLimitPreviewRequest represents the request to preview rate limit impact
type RateLimitPreviewRequest struct {
	Rules interface{} `json:"rules"`
}

// RateLimitPreviewResponse represents the response with rate limit preview
type RateLimitPreviewResponse struct {
	Matches interface{} `json:"matches"`
	Impact  interface{} `json:"impact"`
}

// RateLimitResponse represents the response with rate limit details
type RateLimitResponse struct {
	RateLimit
}
