package tcp

// ConnectionSession nil 防御与 Set/Get 往返：这些方法此前无任何用例触达
// （interconnect 握手键 HandshakeDone 只在真实连接路径上出现）。

import (
	"context"
	"testing"
)

func TestConnectionSession_NilReceiverGuards(t *testing.T) {
	var s *ConnectionSession
	if s.HandshakeDone() {
		t.Fatal("nil session must report handshake not done")
	}
	s.Set("k", "v") // nil receiver no-op, must not panic
	if v, ok := s.Get("k"); ok || v != nil {
		t.Fatalf("nil session Get = (%v, %v), want (nil, false)", v, ok)
	}
	if got := ConnectionSessionFromContext(nil); got != nil {
		t.Fatalf("nil ctx session = %v, want nil", got)
	}
}

func TestConnectionSession_SetGetAndHandshake(t *testing.T) {
	s := NewConnectionSession()
	s.Set(ConnectionHandshakeKey, true)
	s.Set("trace", "abc")
	if !s.HandshakeDone() {
		t.Fatal("expected handshake done after setting handshake key")
	}
	if v, ok := s.Get("trace"); !ok || v != "abc" {
		t.Fatalf("Get(trace) = (%v, %v), want (abc, true)", v, ok)
	}
	if _, ok := s.Get("missing"); ok {
		t.Fatal("missing key must report not-found")
	}
	if v, ok := s.Get(ConnectionHandshakeKey); !ok || v != true {
		t.Fatalf("Get(handshake) = (%v, %v), want (true, true)", v, ok)
	}
}

func TestConnectionSession_ContextRoundTrip(t *testing.T) {
	s := NewConnectionSession()
	ctx := WithConnectionSession(context.Background(), s)
	if got := ConnectionSessionFromContext(ctx); got != s {
		t.Fatalf("session from ctx = %p, want %p", got, s)
	}
	if got := ConnectionSessionFromContext(context.Background()); got != nil {
		t.Fatalf("bare ctx session = %p, want nil", got)
	}
}
