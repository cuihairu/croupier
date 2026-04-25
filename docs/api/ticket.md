# 工单 API

### 1. "获取工单列表"

1. route definition

- Url: /api/v1/tickets
- Method: GET
- Request: `TicketsListRequest`
- Response: `TicketsListResponse`

2. request definition



```go
type TicketsListRequest struct {
	Page int `form:"page,optional,default=1"`
	PageSize int `form:"pageSize,optional,default=20"`
	Status string `form:"status,optional"`
	Category string `form:"category,optional"`
	Priority string `form:"priority,optional"`
	Assignee string `form:"assignee,optional"`
}
```


3. response definition



```go
type TicketsListResponse struct {
	Items []Ticket `json:"items"`
	Total int64 `json:"total"`
	Page int `json:"page"`
	Size int `json:"pageSize"`
}
```

### 2. "创建工单"

1. route definition

- Url: /api/v1/tickets
- Method: POST
- Request: `TicketCreateRequest`
- Response: `TicketDetailResponse`

2. request definition



```go
type TicketCreateRequest struct {
	Title string `json:"title"`
	Content string `json:"content"`
	Category string `json:"category"`
	Priority string `json:"priority,optional,default=medium"`
	Tags []string `json:"tags,optional"`
	PlayerId string `json:"playerId,optional"`
	Contact string `json:"contact,optional"`
	GameId string `json:"gameId,optional"`
	Env string `json:"env,optional"`
}
```


3. response definition



```go
type TicketDetailResponse struct {
	Id int64 `json:"id"`
	Title string `json:"title"`
	Content string `json:"content"`
	Category string `json:"category"`
	Priority string `json:"priority"`
	Status string `json:"status"`
	Assignee string `json:"assignee"`
	Tags []string `json:"tags"`
	PlayerId string `json:"playerId"`
	Contact string `json:"contact"`
	GameId string `json:"gameId"`
	Env string `json:"env"`
	Source string `json:"source"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	Comments []Comment `json:"comments,omitempty"`
}

type Ticket struct {
	Id int64 `json:"id"`
	Title string `json:"title"`
	Content string `json:"content"`
	Category string `json:"category"`
	Priority string `json:"priority"`
	Status string `json:"status"`
	Assignee string `json:"assignee"`
	Tags []string `json:"tags"`
	PlayerId string `json:"playerId"`
	Contact string `json:"contact"`
	GameId string `json:"gameId"`
	Env string `json:"env"`
	Source string `json:"source"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 3. "获取工单详情"

1. route definition

- Url: /api/v1/tickets/:id
- Method: GET
- Request: `TicketDetailRequest`
- Response: `TicketDetailResponse`

2. request definition



```go
type TicketDetailRequest struct {
	ID string `path:"id"`
}
```


3. response definition



```go
type TicketDetailResponse struct {
	Id int64 `json:"id"`
	Title string `json:"title"`
	Content string `json:"content"`
	Category string `json:"category"`
	Priority string `json:"priority"`
	Status string `json:"status"`
	Assignee string `json:"assignee"`
	Tags []string `json:"tags"`
	PlayerId string `json:"playerId"`
	Contact string `json:"contact"`
	GameId string `json:"gameId"`
	Env string `json:"env"`
	Source string `json:"source"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	Comments []Comment `json:"comments,omitempty"`
}

type Ticket struct {
	Id int64 `json:"id"`
	Title string `json:"title"`
	Content string `json:"content"`
	Category string `json:"category"`
	Priority string `json:"priority"`
	Status string `json:"status"`
	Assignee string `json:"assignee"`
	Tags []string `json:"tags"`
	PlayerId string `json:"playerId"`
	Contact string `json:"contact"`
	GameId string `json:"gameId"`
	Env string `json:"env"`
	Source string `json:"source"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 4. "更新工单"

1. route definition

- Url: /api/v1/tickets/:id
- Method: PUT
- Request: `TicketUpdateRequest`
- Response: `TicketDetailResponse`

2. request definition



```go
type TicketUpdateRequest struct {
	ID string `path:"id"`
	Title string `json:"title,optional"`
	Content string `json:"content,optional"`
	Category string `json:"category,optional"`
	Priority string `json:"priority,optional"`
	Assignee string `json:"assignee,optional"`
	Tags []string `json:"tags,optional"`
}
```


3. response definition



```go
type TicketDetailResponse struct {
	Id int64 `json:"id"`
	Title string `json:"title"`
	Content string `json:"content"`
	Category string `json:"category"`
	Priority string `json:"priority"`
	Status string `json:"status"`
	Assignee string `json:"assignee"`
	Tags []string `json:"tags"`
	PlayerId string `json:"playerId"`
	Contact string `json:"contact"`
	GameId string `json:"gameId"`
	Env string `json:"env"`
	Source string `json:"source"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	Comments []Comment `json:"comments,omitempty"`
}

type Ticket struct {
	Id int64 `json:"id"`
	Title string `json:"title"`
	Content string `json:"content"`
	Category string `json:"category"`
	Priority string `json:"priority"`
	Status string `json:"status"`
	Assignee string `json:"assignee"`
	Tags []string `json:"tags"`
	PlayerId string `json:"playerId"`
	Contact string `json:"contact"`
	GameId string `json:"gameId"`
	Env string `json:"env"`
	Source string `json:"source"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 5. "删除工单"

1. route definition

- Url: /api/v1/tickets/:id
- Method: DELETE
- Request: `TicketDeleteRequest`
- Response: `-`

2. request definition



```go
type TicketDeleteRequest struct {
	ID string `path:"id"`
}
```


3. response definition


### 6. "工单状态转换"

1. route definition

- Url: /api/v1/tickets/:id/transition
- Method: POST
- Request: `TicketTransitionRequest`
- Response: `TicketDetailResponse`

2. request definition



```go
type TicketTransitionRequest struct {
	ID string `path:"id"`
	Status string `json:"status"` // open, in_progress, resolved, closed
	Note string `json:"note,optional"`
}
```


3. response definition



```go
type TicketDetailResponse struct {
	Id int64 `json:"id"`
	Title string `json:"title"`
	Content string `json:"content"`
	Category string `json:"category"`
	Priority string `json:"priority"`
	Status string `json:"status"`
	Assignee string `json:"assignee"`
	Tags []string `json:"tags"`
	PlayerId string `json:"playerId"`
	Contact string `json:"contact"`
	GameId string `json:"gameId"`
	Env string `json:"env"`
	Source string `json:"source"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	Comments []Comment `json:"comments,omitempty"`
}

type Ticket struct {
	Id int64 `json:"id"`
	Title string `json:"title"`
	Content string `json:"content"`
	Category string `json:"category"`
	Priority string `json:"priority"`
	Status string `json:"status"`
	Assignee string `json:"assignee"`
	Tags []string `json:"tags"`
	PlayerId string `json:"playerId"`
	Contact string `json:"contact"`
	GameId string `json:"gameId"`
	Env string `json:"env"`
	Source string `json:"source"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
```

### 7. "获取工单评论"

1. route definition

- Url: /api/v1/tickets/:ticketId/comments
- Method: GET
- Request: `TicketCommentsRequest`
- Response: `TicketCommentsResponse`

2. request definition



```go
type TicketCommentsRequest struct {
	TicketID string `path:"ticketId"`
}
```


3. response definition



```go
type TicketCommentsResponse struct {
	Items []Comment `json:"items"`
}
```

### 8. "创建工单评论"

1. route definition

- Url: /api/v1/tickets/:ticketId/comments
- Method: POST
- Request: `TicketCommentCreateRequest`
- Response: `TicketCommentsResponse`

2. request definition



```go
type TicketCommentCreateRequest struct {
	TicketID string `path:"ticketId"`
	Content string `json:"content"`
}
```


3. response definition



```go
type TicketCommentsResponse struct {
	Items []Comment `json:"items"`
}
```

