package message

type MessageDetailRequest struct {
	ID string `uri:"id"`
}

type MessageItem struct {
	ID        interface{} `json:"id"`
	To        string      `json:"to"`
	Type      string      `json:"type"`
	Title     string      `json:"title"`
	Content   string      `json:"content"`
	Data      interface{} `json:"data,omitempty"`
	Status    string      `json:"status"`
	ReadAt    string      `json:"readAt,omitempty"`
	CreatedAt string      `json:"createdAt"`
	UpdatedAt string      `json:"updatedAt"`
}

type MessageDetailResponse = MessageItem

type MessageReadRequest struct {
	ID string `uri:"id"`
}

type MessageReadResponse = MessageItem

type MessageSendRequest struct {
	To      string      `json:"to"`
	Type    string      `json:"type"`
	Title   string      `json:"title"`
	Content string      `json:"content"`
	Data    interface{} `json:"data"`
}

type MessageSendResponse = MessageItem

type MessagesListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Type     string `form:"type"`
	Status   string `form:"status"`
}

type MessagesListResponse struct {
	Items    []MessageItem `json:"items"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
}

type MessagesUnreadCountRequest struct{}

type MessagesUnreadCountResponse struct {
	Count int64 `json:"count"`
}

// StreamMessagesRequest is for streaming messages
type StreamMessagesRequest struct {
}

// StreamMessagesResponse is for streaming messages response
type StreamMessagesResponse struct {
	Items []MessageItem `json:"items"`
}
