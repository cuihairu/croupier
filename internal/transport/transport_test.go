// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package transport

import (
	"context"
	"testing"
)

func TestKindValues(t *testing.T) {
	tests := []struct {
		name string
		kind Kind
		want string
	}{
		{"KindTCP", KindTCP, "tcp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(tt.kind); got != tt.want {
				t.Errorf("Kind = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandlerFunc(t *testing.T) {
	ctx := context.Background()
	var called bool

	f := HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) (respBody []byte, err error) {
		called = true
		if msgID != 123 {
			t.Errorf("msgID = %d, want 123", msgID)
		}
		if reqID != 456 {
			t.Errorf("reqID = %d, want 456", reqID)
		}
		if string(body) != "test body" {
			t.Errorf(`body = %s, want "test body"`, string(body))
		}
		return []byte("response"), nil
	})

	resp, err := f.Handle(ctx, 123, 456, []byte("test body"))
	if err != nil {
		t.Errorf("Handle() error = %v", err)
	}
	if !called {
		t.Error("HandlerFunc was not called")
	}
	if string(resp) != "response" {
		t.Errorf(`Handle() response = %s, want "response"`, string(resp))
	}
}

// mockHandler implements Handler for testing
type mockHandler struct {
	handleFunc func(ctx context.Context, msgID uint32, reqID uint32, body []byte) (respBody []byte, err error)
}

func (m *mockHandler) Handle(ctx context.Context, msgID uint32, reqID uint32, body []byte) (respBody []byte, err error) {
	if m.handleFunc != nil {
		return m.handleFunc(ctx, msgID, reqID, body)
	}
	return nil, nil
}

func TestHandlerInterface(t *testing.T) {
	ctx := context.Background()
	expectedResponse := []byte("test response")

	handler := &mockHandler{
		handleFunc: func(ctx context.Context, msgID uint32, reqID uint32, body []byte) (respBody []byte, err error) {
			if msgID != 1 {
				t.Errorf("msgID = %d, want 1", msgID)
			}
			if reqID != 2 {
				t.Errorf("reqID = %d, want 2", reqID)
			}
			return expectedResponse, nil
		},
	}

	resp, err := handler.Handle(ctx, 1, 2, nil)
	if err != nil {
		t.Errorf("Handle() error = %v", err)
	}
	if string(resp) != string(expectedResponse) {
		t.Errorf("Handle() response = %s, want %s", string(resp), string(expectedResponse))
	}
}

func TestHandlerFuncImplementsHandler(t *testing.T) {
	// Verify HandlerFunc implements Handler interface
	var _ Handler = HandlerFunc(nil)
}

// mockClient implements Client for testing
type mockClient struct {
	callFunc     func(ctx context.Context, msgID uint32, reqBody []byte) (respMsgID uint32, respBody []byte, err error)
	closeFunc    func() error
	isClosedFunc func() bool
}

func (m *mockClient) Call(ctx context.Context, msgID uint32, reqBody []byte) (respMsgID uint32, respBody []byte, err error) {
	if m.callFunc != nil {
		return m.callFunc(ctx, msgID, reqBody)
	}
	return 0, nil, nil
}

func (m *mockClient) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *mockClient) IsClosed() bool {
	if m.isClosedFunc != nil {
		return m.isClosedFunc()
	}
	return false
}

func TestClientInterface(t *testing.T) {
	ctx := context.Background()
	client := &mockClient{
		callFunc: func(ctx context.Context, msgID uint32, reqBody []byte) (respMsgID uint32, respBody []byte, err error) {
			return 456, []byte("response"), nil
		},
		closeFunc: func() error {
			return nil
		},
		isClosedFunc: func() bool {
			return false
		},
	}

	respMsgID, respBody, err := client.Call(ctx, 123, []byte("request"))
	if err != nil {
		t.Errorf("Call() error = %v", err)
	}
	if respMsgID != 456 {
		t.Errorf("Call() respMsgID = %d, want 456", respMsgID)
	}
	if string(respBody) != "response" {
		t.Errorf(`Call() respBody = %s, want "response"`, string(respBody))
	}

	if err := client.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if client.IsClosed() {
		t.Error("IsClosed() = true, want false")
	}
}

// mockServer implements Server for testing
type mockServer struct {
	serveFunc    func(ctx context.Context) error
	closeFunc    func() error
	isClosedFunc func() bool
	addrFunc     func() string
}

func (m *mockServer) Serve(ctx context.Context) error {
	if m.serveFunc != nil {
		return m.serveFunc(ctx)
	}
	return nil
}

func (m *mockServer) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *mockServer) IsClosed() bool {
	if m.isClosedFunc != nil {
		return m.isClosedFunc()
	}
	return false
}

func (m *mockServer) Addr() string {
	if m.addrFunc != nil {
		return m.addrFunc()
	}
	return ""
}

func TestServerInterface(t *testing.T) {
	ctx := context.Background()
	server := &mockServer{
		serveFunc: func(ctx context.Context) error {
			return nil
		},
		closeFunc: func() error {
			return nil
		},
		isClosedFunc: func() bool {
			return false
		},
		addrFunc: func() string {
			return "127.0.0.1:8080"
		},
	}

	if err := server.Serve(ctx); err != nil {
		t.Errorf("Serve() error = %v", err)
	}

	if err := server.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if server.IsClosed() {
		t.Error("IsClosed() = true, want false")
	}

	if addr := server.Addr(); addr != "127.0.0.1:8080" {
		t.Errorf("Addr() = %s, want 127.0.0.1:8080", addr)
	}
}

// mockSessionCaller implements SessionCaller for testing
type mockSessionCaller struct {
	callFunc func(ctx context.Context, msgID uint32, reqBody []byte) (respMsgID uint32, respBody []byte, err error)
}

func (m *mockSessionCaller) Call(ctx context.Context, msgID uint32, reqBody []byte) (respMsgID uint32, respBody []byte, err error) {
	if m.callFunc != nil {
		return m.callFunc(ctx, msgID, reqBody)
	}
	return 0, nil, nil
}

func TestSessionCallerInterface(t *testing.T) {
	ctx := context.Background()
	caller := &mockSessionCaller{
		callFunc: func(ctx context.Context, msgID uint32, reqBody []byte) (respMsgID uint32, respBody []byte, err error) {
			return 789, []byte("caller response"), nil
		},
	}

	respMsgID, respBody, err := caller.Call(ctx, 100, []byte("caller request"))
	if err != nil {
		t.Errorf("Call() error = %v", err)
	}
	if respMsgID != 789 {
		t.Errorf("Call() respMsgID = %d, want 789", respMsgID)
	}
	if string(respBody) != "caller response" {
		t.Errorf(`Call() respBody = %s, want "caller response"`, string(respBody))
	}
}
