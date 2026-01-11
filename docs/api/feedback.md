### 1. "获取反馈列表"

1. route definition

- Url: /api/v1/feedback
- Method: GET
- Request: `FeedbackListRequest`
- Response: `FeedbackListResponse`

2. request definition



```golang
type FeedbackListRequest struct {
	Page int `form:"page,optional,default=1"`
	PageSize int `form:"pageSize,optional,default=20"`
	Status string `form:"status,optional"`
	Category string `form:"category,optional"`
	GameId string `form:"gameId,optional"`
}
```


3. response definition



```golang
type FeedbackListResponse struct {
	Items []Feedback `json:"items"`
	Total int64 `json:"total"`
	Page int `json:"page"`
	Size int `json:"pageSize"`
}
```

### 2. "创建反馈"

1. route definition

- Url: /api/v1/feedback
- Method: POST
- Request: `FeedbackCreateRequest`
- Response: `FeedbackDetailResponse`

2. request definition



```golang
type FeedbackCreateRequest struct {
	PlayerId string `json:"playerId,optional"`
	Contact string `json:"contact"`
	Content string `json:"content"`
	Category string `json:"category"`
	Rating int `json:"rating,optional"`
	Attach string `json:"attach,optional"`
	GameId string `json:"gameId,optional"`
	Env string `json:"env,optional"`
}
```


3. response definition



```golang
type FeedbackDetailResponse struct {
	Id int64 `json:"id"`
	PlayerId string `json:"playerId"`
	Contact string `json:"contact"`
	Content string `json:"content"`
	Category string `json:"category"`
	Priority string `json:"priority"`
	Status string `json:"status"`
	Rating int `json:"rating"` // 1-5星
	Attach string `json:"attach"` // 附件URL
	GameId string `json:"gameId"`
	Env string `json:"env"`
	Reply string `json:"reply"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type Feedback struct {
	Id int64 `json:"id"`
	PlayerId string `json:"playerId"`
	Contact string `json:"contact"`
	Content string `json:"content"`
	Category string `json:"category"`
	Priority string `json:"priority"`
	Status string `json:"status"`
	Rating int `json:"rating"` // 1-5星
	Attach string `json:"attach"` // 附件URL
	GameId string `json:"gameId"`
	Env string `json:"env"`
	Reply string `json:"reply"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 3. "更新反馈"

1. route definition

- Url: /api/v1/feedback/:id
- Method: PUT
- Request: `FeedbackUpdateRequest`
- Response: `FeedbackDetailResponse`

2. request definition



```golang
type FeedbackUpdateRequest struct {
	ID string `path:"id"`
	Status string `json:"status,optional"`
	Priority string `json:"priority,optional"`
	Reply string `json:"reply,optional"`
}
```


3. response definition



```golang
type FeedbackDetailResponse struct {
	Id int64 `json:"id"`
	PlayerId string `json:"playerId"`
	Contact string `json:"contact"`
	Content string `json:"content"`
	Category string `json:"category"`
	Priority string `json:"priority"`
	Status string `json:"status"`
	Rating int `json:"rating"` // 1-5星
	Attach string `json:"attach"` // 附件URL
	GameId string `json:"gameId"`
	Env string `json:"env"`
	Reply string `json:"reply"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type Feedback struct {
	Id int64 `json:"id"`
	PlayerId string `json:"playerId"`
	Contact string `json:"contact"`
	Content string `json:"content"`
	Category string `json:"category"`
	Priority string `json:"priority"`
	Status string `json:"status"`
	Rating int `json:"rating"` // 1-5星
	Attach string `json:"attach"` // 附件URL
	GameId string `json:"gameId"`
	Env string `json:"env"`
	Reply string `json:"reply"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 4. "删除反馈"

1. route definition

- Url: /api/v1/feedback/:id
- Method: DELETE
- Request: `FeedbackDeleteRequest`
- Response: `-`

2. request definition



```golang
type FeedbackDeleteRequest struct {
	ID string `path:"id"`
}
```


3. response definition


### 5. "获取反馈统计"

1. route definition

- Url: /api/v1/feedback/stats
- Method: GET
- Request: `FeedbackStatsRequest`
- Response: `FeedbackStatsResponse`

2. request definition



```golang
type FeedbackStatsRequest struct {
	GameId string `form:"gameId,optional"`
	Days int `form:"days,optional,default=7"`
}
```


3. response definition



```golang
type FeedbackStatsResponse struct {
	Total int `json:"total"`
	ByCategory map[string]int `json:"byCategory"`
	ByStatus map[string]int `json:"byStatus"`
	AvgRating float64 `json:"avgRating"`
	ResponseRate float64 `json:"responseRate"`
}

type FeedbackStats struct {
	Total int `json:"total"`
	ByCategory map[string]int `json:"byCategory"`
	ByStatus map[string]int `json:"byStatus"`
	AvgRating float64 `json:"avgRating"`
	ResponseRate float64 `json:"responseRate"`
}
```

