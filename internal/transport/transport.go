// Package transport defines shared transport abstractions for Croupier.
package transport

import "context"

// Kind identifies a transport implementation.
type Kind string

const (
	KindNNG Kind = "nng"
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
