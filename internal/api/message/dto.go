package message

type MessageDetailRequest struct {
	ID string `uri:"id"`
}

type MessageDetailResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type MessageReadRequest struct {
	ID string `uri:"id"`
}

type MessageReadResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type MessageSendRequest struct {
	To      string      `json:"to"`
	Type    string      `json:"type"`
	Title   string      `json:"title"`
	Content string      `json:"content"`
	Data    interface{} `json:"data"`
}

type MessageSendResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type MessagesListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Type     string `form:"type"`
	Status   string `form:"status"`
}

type MessagesListResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type MessagesUnreadCountRequest struct{}

type MessagesUnreadCountResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// StreamMessagesRequest is for streaming messages
type StreamMessagesRequest struct {
}

// StreamMessagesResponse is for streaming messages response
type StreamMessagesResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
