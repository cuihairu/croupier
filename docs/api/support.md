# 支持 API

> 响应契约：成功返回业务 JSON（无 `{code,message,data}` 包装），错误使用 HTTP 状态码 + `{ "error": ..., "message": ... }`。本页早期版本中的 envelope 结构已废弃，以下 response 定义已按当前 DTO 修正。

### 1. "获取FAQ列表"

1. route definition

- Url: /api/v1/support/faq
- Method: GET
- Request: `SupportFAQListRequest`
- Response: `SupportFAQListResponse`

2. request definition

```go
type SupportFAQListRequest struct {
	Category string `form:"category,optional"`
}
```

3. response definition

```go
type SupportFAQListResponse struct {
	Items []FAQ `json:"items"`
	Total int64 `json:"total"`
	Page int `json:"page"`
	Size int `json:"pageSize"`
}
```

### 2. "创建FAQ"

1. route definition

- Url: /api/v1/support/faq
- Method: POST
- Request: `SupportFAQCreateRequest`
- Response: `SupportFAQCreateResponse`

2. request definition

```go
type SupportFAQCreateRequest struct {
	Category string `json:"category"`
	Question string `json:"question"`
	Answer string `json:"answer"`
}
```

3. response definition

```go
type SupportFAQCreateResponse struct {
	FAQ
}
```

### 3. "更新FAQ"

1. route definition

- Url: /api/v1/support/faq/:id
- Method: PUT
- Request: `SupportFAQUpdateRequest`
- Response: `SupportFAQUpdateResponse`

2. request definition

```go
type SupportFAQUpdateRequest struct {
	ID string `path:"id"`
	Category string `json:"category,optional"`
	Question string `json:"question,optional"`
	Answer string `json:"answer,optional"`
}
```

3. response definition

```go
type SupportFAQUpdateResponse struct {
	FAQ
}
```

### 4. "删除FAQ"

1. route definition

- Url: /api/v1/support/faq/:id
- Method: DELETE
- Request: `SupportFAQDeleteRequest`
- Response: `SupportFAQDeleteResponse`

2. request definition

```go
type SupportFAQDeleteRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
type SupportFAQDeleteResponse struct {
	// 204 No Content
}
```

### 5. "获取反馈列表"

1. route definition

- Url: /api/v1/support/feedback
- Method: GET
- Request: `SupportFeedbackListRequest`
- Response: `SupportFeedbackListResponse`

2. request definition

```go
type SupportFeedbackListRequest struct {
	Page int `form:"page,optional"`
	PageSize int `form:"pageSize,optional"`
}
```

3. response definition

```go
type SupportFeedbackListResponse struct {
	Items []Feedback `json:"items"`
	Total int64 `json:"total"`
	Page int `json:"page"`
	Size int `json:"pageSize"`
}
```

### 6. "创建反馈"

1. route definition

- Url: /api/v1/support/feedback
- Method: POST
- Request: `SupportFeedbackCreateRequest`
- Response: `SupportFeedbackCreateResponse`

2. request definition

```go
type SupportFeedbackCreateRequest struct {
	Type string `json:"type"`
	Content string `json:"content"`
}
```

3. response definition

```go
type SupportFeedbackCreateResponse struct {
	Feedback
}
```

### 7. "更新反馈"

1. route definition

- Url: /api/v1/support/feedback/:id
- Method: PUT
- Request: `SupportFeedbackUpdateRequest`
- Response: `SupportFeedbackUpdateResponse`

2. request definition

```go
type SupportFeedbackUpdateRequest struct {
	ID string `path:"id"`
	Status string `json:"status,optional"`
	Comment string `json:"comment,optional"`
}
```

3. response definition

```go
type SupportFeedbackUpdateResponse struct {
	Feedback
}
```

### 8. "删除反馈"

1. route definition

- Url: /api/v1/support/feedback/:id
- Method: DELETE
- Request: `SupportFeedbackDeleteRequest`
- Response: `SupportFeedbackDeleteResponse`

2. request definition

```go
type SupportFeedbackDeleteRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
type SupportFeedbackDeleteResponse struct {
	// 204 No Content
}
```

### 9. "获取工单列表"

1. route definition

- Url: /api/v1/support/tickets
- Method: GET
- Request: `SupportTicketsListRequest`
- Response: `SupportTicketsListResponse`

2. request definition

```go
type SupportTicketsListRequest struct {
	Page int `form:"page,optional"`
	PageSize int `form:"pageSize,optional"`
	Status string `form:"status,optional"`
}
```

3. response definition

```go
type SupportTicketsListResponse struct {
	Items []Ticket `json:"items"`
	Total int64 `json:"total"`
	Page int `json:"page"`
	Size int `json:"pageSize"`
}
```

### 10. "创建工单"

1. route definition

- Url: /api/v1/support/tickets
- Method: POST
- Request: `SupportTicketCreateRequest`
- Response: `SupportTicketCreateResponse`

2. request definition

```go
type SupportTicketCreateRequest struct {
	Subject string `json:"subject"`
	Content string `json:"content"`
}
```

3. response definition

```go
type SupportTicketCreateResponse struct {
	Ticket
} // TODO verify
```

### 11. "获取工单详情"

1. route definition

- Url: /api/v1/support/tickets/:id
- Method: GET
- Request: `SupportTicketDetailRequest`
- Response: `SupportTicketDetailResponse`

2. request definition

```go
type SupportTicketDetailRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
type SupportTicketDetailResponse struct {
	Ticket
	Comments []Comment `json:"comments,omitempty"`
} // TODO verify
```

### 12. "更新工单"

1. route definition

- Url: /api/v1/support/tickets/:id
- Method: PUT
- Request: `SupportTicketUpdateRequest`
- Response: `SupportTicketUpdateResponse`

2. request definition

```go
type SupportTicketUpdateRequest struct {
	ID string `path:"id"`
	Subject string `json:"subject,optional"`
	Content string `json:"content,optional"`
}
```

3. response definition

```go
type SupportTicketUpdateResponse struct {
	Ticket
} // TODO verify
```

### 13. "删除工单"

1. route definition

- Url: /api/v1/support/tickets/:id
- Method: DELETE
- Request: `SupportTicketDeleteRequest`
- Response: `SupportTicketDeleteResponse`

2. request definition

```go
type SupportTicketDeleteRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
type SupportTicketDeleteResponse struct {
	// 204 No Content
} // TODO verify
```

### 14. "工单状态转换"

1. route definition

- Url: /api/v1/support/tickets/:id/transition
- Method: POST
- Request: `SupportTicketTransitionRequest`
- Response: `SupportTicketTransitionResponse`

2. request definition

```go
type SupportTicketTransitionRequest struct {
	ID string `path:"id"`
	Status string `json:"status"`
}
```

3. response definition

```go
type SupportTicketTransitionResponse struct {
	Ticket
	Comments []Comment `json:"comments,omitempty"`
} // TODO verify
```

### 15. "获取工单评论"

1. route definition

- Url: /api/v1/support/tickets/:ticketId/comments
- Method: GET
- Request: `SupportCommentsListRequest`
- Response: `SupportCommentsListResponse`

2. request definition

```go
type SupportCommentsListRequest struct {
	TicketID string `path:"ticketId"`
	Page int `form:"page,optional"`
	PageSize int `form:"pageSize,optional"`
}
```

3. response definition

```go
type SupportCommentsListResponse struct {
	Items []Comment `json:"items"`
} // TODO verify
```

### 16. "创建工单评论"

1. route definition

- Url: /api/v1/support/tickets/:ticketId/comments
- Method: POST
- Request: `SupportCommentCreateRequest`
- Response: `SupportCommentCreateResponse`

2. request definition

```go
type SupportCommentCreateRequest struct {
	TicketID string `path:"ticketId"`
	Content string `json:"content"`
}
```

3. response definition

```go
type SupportCommentCreateResponse struct {
	Items []Comment `json:"items"`
} // TODO verify
```
