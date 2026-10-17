# 消息 API

### 1. "获取消息列表"

1. route definition

- Url: /api/v1/messages
- Method: GET
- Request: `MessagesListRequest`
- Response: `MessagesListResponse`

2. request definition

```go
type MessagesListRequest struct {
	Page int `form:"page,optional"`
	PageSize int `form:"pageSize,optional"`
	Type string `form:"type,optional"`
	Status string `form:"status,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 2. "发送消息"

1. route definition

- Url: /api/v1/messages
- Method: POST
- Request: `MessageSendRequest`
- Response: `MessageSendResponse`

2. request definition

```go
type MessageSendRequest struct {
	To string `json:"to"`
	Type string `json:"type"`
	Title string `json:"title,optional"`
	Content string `json:"content"`
	Data interface{} `json:"data,optional"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 3. "获取消息详情"

1. route definition

- Url: /api/v1/messages/:id
- Method: GET
- Request: `MessageDetailRequest`
- Response: `MessageDetailResponse`

2. request definition

```go
type MessageDetailRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 4. "标记消息已读"

1. route definition

- Url: /api/v1/messages/:id/read
- Method: POST
- Request: `MessageReadRequest`
- Response: `MessageReadResponse`

2. request definition

```go
type MessageReadRequest struct {
	ID string `path:"id"`
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 5. "消息流（实时推送）"

1. route definition

- Url: /api/v1/messages/stream
- Method: GET
- Request: `StreamMessagesRequest`
- Response: `StreamMessagesResponse`

2. request definition

```go
type StreamMessagesRequest struct {
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```

### 6. "获取未读消息数量"

1. route definition

- Url: /api/v1/messages/unread-count
- Method: GET
- Request: `MessagesUnreadCountRequest`
- Response: `MessagesUnreadCountResponse`

2. request definition

```go
type MessagesUnreadCountRequest struct {
}
```

3. response definition

```go
// 实际响应为裸 payload（业务 DTO 直接 JSON 序列化），无 code/message envelope。
// 错误统一 { "error", "message", "details" }（见 rest.md）。
```
