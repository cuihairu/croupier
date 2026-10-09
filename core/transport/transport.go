// Package transport Croupier 传输层抽象（自研 TCP 产品契约，agent-core K 批
// 后补上收：agents/* 依赖方向不得 import internal/，capture C1 注册上线以
// 本包为通道面；internal 侧留 shim，import 路径零变化）。
package transport

import "context"

// Kind identifies a transport implementation.
type Kind string

const (
	KindTCP Kind = "tcp"
)

// Handler handles a request and returns the response body.
type Handler interface {
	Handle(ctx context.Context, msgID uint32, reqID uint32, body []byte) (respBody []byte, err error)
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(ctx context.Context, msgID uint32, reqID uint32, body []byte) (respBody []byte, err error)

// Handle calls f(ctx, msgID, reqID, body).
func (f HandlerFunc) Handle(ctx context.Context, msgID uint32, reqID uint32, body []byte) (respBody []byte, err error) {
	return f(ctx, msgID, reqID, body)
}

// Client is a request-response transport client.
type Client interface {
	Call(ctx context.Context, msgID uint32, reqBody []byte) (respMsgID uint32, respBody []byte, err error)
	Close() error
	IsClosed() bool
}

// Server is a request-response transport server.
type Server interface {
	Serve(ctx context.Context) error
	Close() error
	IsClosed() bool
	Addr() string
}

// SessionCaller sends a request over an established TCP session and returns the response.
type SessionCaller interface {
	Call(ctx context.Context, msgID uint32, reqBody []byte) (respMsgID uint32, respBody []byte, err error)
}
