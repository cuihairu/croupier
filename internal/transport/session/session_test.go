package session

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/transport/tcp"
)

var errNotFound = errors.New("not found")

// mockSession implements Session for store tests.
type mockSession struct {
	id       string
	lastSeen time.Time
	closed   bool
}

func (m *mockSession) SessionID() string              { return m.id }
func (m *mockSession) UpdateLastSeen()                { m.lastSeen = time.Now() }
func (m *mockSession) GetLastSeen() time.Time         { return m.lastSeen }
func (m *mockSession) IsStale(ttl time.Duration) bool { return time.Since(m.lastSeen) > ttl }
func (m *mockSession) Close() error                   { m.closed = true; return nil }

func newTestMuxConnPair(t *testing.T) (*tcp.MuxConn, net.Conn) {
	t.Helper()
	c1, c2 := net.Pipe()
	return tcp.NewMuxConn(c1, nil, nil), c2
}

// --- SessionKey tests ---

func TestSessionKey_String(t *testing.T) {
	k := SessionKey{ID: "agent-1", Type: "agent"}
	if got := k.String(); got != "agent:agent-1" {
		t.Errorf("got %q, want %q", got, "agent:agent-1")
	}
}

// --- BaseSession tests ---

func TestNewBaseSession(t *testing.T) {
	conn, remote := newTestMuxConnPair(t)
	defer conn.Close()
	defer remote.Close()

	s := NewBaseSession(conn, "sess-1", "1.0.0")
	if s.SessionID() != "sess-1" {
		t.Errorf("SessionID = %q, want %q", s.SessionID(), "sess-1")
	}
	if s.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", s.Version, "1.0.0")
	}
	if s.Conn() != conn {
		t.Error("Conn() should return the conn passed to NewBaseSession")
	}
	if s.ConnectedAt.IsZero() {
		t.Error("ConnectedAt should not be zero")
	}
}

func TestBaseSession_UpdateAndGetLastSeen(t *testing.T) {
	s := &BaseSession{sessionID: "s1"}
	s.lastSeen.Store(time.Now().Add(-time.Second).Unix())
	initial := s.GetLastSeen()

	s.UpdateLastSeen()

	if !s.GetLastSeen().After(initial) {
		t.Error("GetLastSeen should be after UpdateLastSeen")
	}
}

func TestBaseSession_IsStale(t *testing.T) {
	s := &BaseSession{sessionID: "s1"}
	s.lastSeen.Store(time.Now().Add(-2 * time.Second).Unix())

	if !s.IsStale(time.Second) {
		t.Error("expected stale after 2s with 1s TTL")
	}
	if s.IsStale(5 * time.Second) {
		t.Error("should not be stale with 5s TTL")
	}
}

func TestBaseSession_Close(t *testing.T) {
	t.Run("with conn", func(t *testing.T) {
		conn, remote := newTestMuxConnPair(t)
		defer remote.Close()
		s := &BaseSession{conn: conn}
		if err := s.Close(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("nil conn", func(t *testing.T) {
		s := &BaseSession{}
		if err := s.Close(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestBaseSession_RemoteAddr(t *testing.T) {
	t.Run("with conn", func(t *testing.T) {
		conn, remote := newTestMuxConnPair(t)
		defer conn.Close()
		defer remote.Close()
		s := &BaseSession{conn: conn}
		_ = s.RemoteAddr()
	})

	t.Run("nil conn", func(t *testing.T) {
		s := &BaseSession{}
		if got := s.RemoteAddr(); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

func TestBaseSession_LocalAddr(t *testing.T) {
	t.Run("with conn", func(t *testing.T) {
		conn, remote := newTestMuxConnPair(t)
		defer conn.Close()
		defer remote.Close()
		s := &BaseSession{conn: conn}
		_ = s.LocalAddr()
	})

	t.Run("nil conn", func(t *testing.T) {
		s := &BaseSession{}
		if got := s.LocalAddr(); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

// --- BaseStore tests ---

func TestBaseStore_Add(t *testing.T) {
	store := NewBaseStore()
	s := &mockSession{id: "s1", lastSeen: time.Now()}

	if err := store.Add("k1", s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.Count() != 1 {
		t.Errorf("count = %d, want 1", store.Count())
	}
}

func TestBaseStore_Add_Duplicate(t *testing.T) {
	store := NewBaseStore()
	s1 := &mockSession{id: "s1", lastSeen: time.Now()}
	s2 := &mockSession{id: "s2", lastSeen: time.Now()}

	_ = store.Add("k1", s1)
	err := store.Add("k1", s2)
	if err == nil {
		t.Fatal("expected error for duplicate key")
	}
	if !IsSessionExists(err) {
		t.Error("expected SessionExistsError")
	}
}

func TestBaseStore_Upsert(t *testing.T) {
	store := NewBaseStore()
	s1 := &mockSession{id: "s1", lastSeen: time.Now()}
	s2 := &mockSession{id: "s2", lastSeen: time.Now()}

	store.Upsert("k1", s1, false)
	store.Upsert("k1", s2, true) // should close s1

	if !s1.closed {
		t.Error("expected old session to be closed")
	}
	got, ok := store.Get("k1")
	if !ok || got.(*mockSession).id != "s2" {
		t.Error("expected s2 after upsert")
	}
}

func TestBaseStore_Upsert_NoClose(t *testing.T) {
	store := NewBaseStore()
	s1 := &mockSession{id: "s1", lastSeen: time.Now()}
	s2 := &mockSession{id: "s2", lastSeen: time.Now()}

	store.Upsert("k1", s1, false)
	store.Upsert("k1", s2, false) // should NOT close s1

	if s1.closed {
		t.Error("old session should not be closed when closeExisting=false")
	}
}

func TestBaseStore_Get_NotFound(t *testing.T) {
	store := NewBaseStore()
	got, ok := store.Get("missing")
	if ok || got != nil {
		t.Error("expected nil, false for missing key")
	}
}

func TestBaseStore_Remove(t *testing.T) {
	store := NewBaseStore()
	s := &mockSession{id: "s1", lastSeen: time.Now()}
	_ = store.Add("k1", s)

	store.Remove("k1")
	if store.Count() != 0 {
		t.Errorf("count = %d, want 0 after remove", store.Count())
	}
	if !s.closed {
		t.Error("expected session to be closed on remove")
	}
}

func TestBaseStore_Remove_NotFound(t *testing.T) {
	store := NewBaseStore()
	store.Remove("missing") // should not panic
}

func TestBaseStore_List(t *testing.T) {
	store := NewBaseStore()
	s1 := &mockSession{id: "s1", lastSeen: time.Now()}
	s2 := &mockSession{id: "s2", lastSeen: time.Now()}
	_ = store.Add("k1", s1)
	_ = store.Add("k2", s2)

	list := store.List()
	if len(list) != 2 {
		t.Errorf("list length = %d, want 2", len(list))
	}
}

func TestBaseStore_List_Empty(t *testing.T) {
	store := NewBaseStore()
	list := store.List()
	if len(list) != 0 {
		t.Errorf("list length = %d, want 0", len(list))
	}
}

func TestBaseStore_PruneStale(t *testing.T) {
	store := NewBaseStore()
	stale := &mockSession{id: "stale", lastSeen: time.Now()}
	fresh := &mockSession{id: "fresh", lastSeen: time.Now()}
	_ = store.Add("k1", stale)
	_ = store.Add("k2", fresh)

	// Set stale AFTER Add, since Add calls UpdateLastSeen
	stale.lastSeen = time.Now().Add(-5 * time.Second)

	pruned := store.PruneStale(2 * time.Second)
	if pruned != 1 {
		t.Errorf("pruned = %d, want 1", pruned)
	}
	if store.Count() != 1 {
		t.Errorf("count = %d, want 1", store.Count())
	}
	if !stale.closed {
		t.Error("expected stale session to be closed")
	}
	if fresh.closed {
		t.Error("fresh session should not be closed")
	}
}

func TestBaseStore_PruneStale_NilSession(t *testing.T) {
	store := NewBaseStore()
	store.mu.Lock()
	store.sessions["nil-sess"] = &sessionWrapper{session: nil}
	store.mu.Unlock()

	pruned := store.PruneStale(time.Second)
	if pruned != 1 {
		t.Errorf("pruned = %d, want 1", pruned)
	}
}

func TestSessionExistsError(t *testing.T) {
	e := &SessionExistsError{Key: "k1"}
	if e.Error() != "session already exists: k1" {
		t.Errorf("unexpected error: %s", e.Error())
	}
	if !IsSessionExists(e) {
		t.Error("IsSessionExists should return true")
	}
	if IsSessionExists(nil) {
		t.Error("IsSessionExists(nil) should return false")
	}
	if IsSessionExists(errNotFound) {
		t.Error("IsSessionExists should return false for other errors")
	}
}

// --- HeartbeatManager tests ---

func TestHeartbeatManager_StartStop(t *testing.T) {
	store := NewBaseStore()
	m := NewHeartbeatManager(store, time.Second, 50*time.Millisecond, nil)

	m.Start()
	time.Sleep(120 * time.Millisecond)
	m.Stop()
}

func TestHeartbeatManager_DoubleStart(t *testing.T) {
	store := NewBaseStore()
	m := NewHeartbeatManager(store, time.Second, 50*time.Millisecond, nil)

	m.Start()
	m.Start() // should be idempotent
	m.Stop()
}

func TestHeartbeatManager_DoubleStop(t *testing.T) {
	store := NewBaseStore()
	m := NewHeartbeatManager(store, time.Second, 50*time.Millisecond, nil)

	m.Start()
	m.Stop()
	m.Stop() // should be idempotent
}

func TestHeartbeatManager_PruneStale(t *testing.T) {
	store := NewBaseStore()
	stale := &mockSession{id: "stale", lastSeen: time.Now()}
	_ = store.Add("k1", stale)
	stale.lastSeen = time.Now().Add(-5 * time.Second)

	m := NewHeartbeatManager(store, time.Second, 50*time.Millisecond, nil)

	m.Start()
	time.Sleep(150 * time.Millisecond)
	m.Stop()

	if store.Count() != 0 {
		t.Errorf("count = %d, want 0 after prune", store.Count())
	}
}

// --- Adapter tests ---

func TestAgentSessionAdapter(t *testing.T) {
	conn, remote := newTestMuxConnPair(t)
	defer conn.Close()
	defer remote.Close()

	sess := NewAgentSessionAdapter(conn, "agent-1", "sess-1", "game-1", "prod", "1.0.0")

	if sess.AgentID != "agent-1" {
		t.Errorf("AgentID = %q", sess.AgentID)
	}
	if sess.GameID != "game-1" {
		t.Errorf("GameID = %q", sess.GameID)
	}
	if sess.Env != "prod" {
		t.Errorf("Env = %q", sess.Env)
	}
	if sess.SessionID() != "sess-1" {
		t.Errorf("SessionID = %q", sess.SessionID())
	}
	if sess.Version != "1.0.0" {
		t.Errorf("Version = %q", sess.Version)
	}
}

func TestAgentSessionStoreAdapter(t *testing.T) {
	store := NewAgentSessionStoreAdapter()

	conn, remote := newTestMuxConnPair(t)
	defer conn.Close()
	defer remote.Close()

	sess := NewAgentSessionAdapter(conn, "agent-1", "sess-1", "game-1", "prod", "1.0.0")

	if err := store.Add(sess); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, ok := store.Get("agent-1")
	if !ok || got.AgentID != "agent-1" {
		t.Error("Get should return the added session")
	}

	list := store.List()
	if len(list) != 1 {
		t.Errorf("List length = %d", len(list))
	}

	store.Remove("agent-1")
	_, ok = store.Get("agent-1")
	if ok {
		t.Error("session should be removed")
	}
}

func TestAgentSessionStoreAdapter_Upsert(t *testing.T) {
	store := NewAgentSessionStoreAdapter()

	c1, r1 := net.Pipe()
	c2, r2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	defer r1.Close()
	defer r2.Close()

	s1 := NewAgentSessionAdapter(tcp.NewMuxConn(c1, nil, nil), "agent-1", "s1", "g1", "p", "1.0")
	s2 := NewAgentSessionAdapter(tcp.NewMuxConn(c2, nil, nil), "agent-1", "s2", "g2", "p", "2.0")

	store.Add(s1)
	store.Upsert(s2)

	got, ok := store.Get("agent-1")
	if !ok || got.SessionID() != "s2" {
		t.Error("expected s2 after upsert")
	}
}

func TestProviderSessionAdapter(t *testing.T) {
	conn, remote := newTestMuxConnPair(t)
	defer conn.Close()
	defer remote.Close()

	sess := NewProviderSessionAdapter(conn, "sess-1", "svc-1", "1.0.0")
	if sess.ServiceID != "svc-1" {
		t.Errorf("ServiceID = %q", sess.ServiceID)
	}
	if sess.SessionID() != "sess-1" {
		t.Errorf("SessionID = %q", sess.SessionID())
	}
	if sess.Functions == nil {
		t.Error("Functions should be initialized")
	}
}

func TestProviderSessionStoreAdapter(t *testing.T) {
	store := NewProviderSessionStoreAdapter()

	conn, remote := newTestMuxConnPair(t)
	defer conn.Close()
	defer remote.Close()

	sess := NewProviderSessionAdapter(conn, "sess-1", "svc-1", "1.0.0")

	if err := store.Add(sess); err != nil {
		t.Fatalf("Add: %v", err)
	}

	bySession, ok := store.GetBySessionID("sess-1")
	if !ok || bySession.ServiceID != "svc-1" {
		t.Error("GetBySessionID failed")
	}

	byService, ok := store.GetByServiceID("svc-1")
	if !ok || byService.SessionID() != "sess-1" {
		t.Error("GetByServiceID failed")
	}

	store.Remove("sess-1")
	_, ok = store.GetBySessionID("sess-1")
	if ok {
		t.Error("should be removed by session ID")
	}
	_, ok = store.GetByServiceID("svc-1")
	if ok {
		t.Error("should be removed by service ID")
	}
}

func TestProviderSessionStoreAdapter_GetBySessionID_NotFound(t *testing.T) {
	store := NewProviderSessionStoreAdapter()
	_, ok := store.GetBySessionID("missing")
	if ok {
		t.Error("expected not found")
	}
}

func TestProviderSessionStoreAdapter_GetByServiceID_NotFound(t *testing.T) {
	store := NewProviderSessionStoreAdapter()
	_, ok := store.GetByServiceID("missing")
	if ok {
		t.Error("expected not found")
	}
}

func TestProviderSessionStoreAdapter_Remove_NotFound(t *testing.T) {
	store := NewProviderSessionStoreAdapter()
	store.Remove("missing") // should not panic
}

func TestProviderSessionStoreAdapter_Add_Duplicate(t *testing.T) {
	store := NewProviderSessionStoreAdapter()

	c1, r1 := net.Pipe()
	c2, r2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	defer r1.Close()
	defer r2.Close()

	s1 := NewProviderSessionAdapter(tcp.NewMuxConn(c1, nil, nil), "sess-1", "svc-1", "1.0")
	s2 := NewProviderSessionAdapter(tcp.NewMuxConn(c2, nil, nil), "sess-1", "svc-2", "2.0")

	if err := store.Add(s1); err != nil {
		t.Fatalf("Add s1: %v", err)
	}
	err := store.Add(s2)
	if err == nil {
		t.Error("expected error for duplicate session ID")
	}
}

// --- Interface compliance ---

func TestBaseSession_ImplementsSession(t *testing.T) {
	var _ Session = &BaseSession{}
	var _ Session = &mockSession{}
}
