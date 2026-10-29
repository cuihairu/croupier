package sidecar

import (
	"net"
	"testing"
	"time"

	tcptr "github.com/cuihairu/croupier/core/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Tests for ProviderSession ---

func TestProviderSession_Conn(t *testing.T) {
	sess := &ProviderSession{
		SessionID: "session-1",
	}

	// Conn is nil for test sessions
	assert.Nil(t, sess.Conn())
}

func TestProviderSession_UpdateLastSeen(t *testing.T) {
	sess := &ProviderSession{
		SessionID: "session-1",
	}

	before := time.Now().Unix()
	sess.UpdateLastSeen()
	after := time.Now().Unix()

	lastSeen := sess.GetLastSeen().Unix()
	assert.True(t, lastSeen >= before)
	assert.True(t, lastSeen <= after)
}

func TestProviderSession_GetLastSeen(t *testing.T) {
	sess := &ProviderSession{
		SessionID: "session-1",
	}

	// Initially should be zero
	lastSeen := sess.GetLastSeen()
	assert.Equal(t, int64(0), lastSeen.Unix())

	// After update
	sess.UpdateLastSeen()
	lastSeen = sess.GetLastSeen()
	assert.NotEqual(t, int64(0), lastSeen.Unix())
}

func TestProviderSession_FunctionIDs(t *testing.T) {
	t.Run("with functions", func(t *testing.T) {
		sess := &ProviderSession{
			SessionID: "session-1",
			Functions: []*sdkv1.ProviderFunctionDescriptor{
				{Id: "func-1"},
				{Id: "func-2"},
				{Id: "func-3"},
			},
		}

		ids := sess.FunctionIDs()
		assert.Len(t, ids, 3)
		assert.Contains(t, ids, "func-1")
		assert.Contains(t, ids, "func-2")
		assert.Contains(t, ids, "func-3")
	})

	t.Run("with nil functions", func(t *testing.T) {
		sess := &ProviderSession{
			SessionID: "session-1",
			Functions: []*sdkv1.ProviderFunctionDescriptor{
				nil,
				{Id: "func-1"},
				{Id: ""},
			},
		}

		ids := sess.FunctionIDs()
		assert.Len(t, ids, 1)
		assert.Contains(t, ids, "func-1")
	})

	t.Run("with empty functions", func(t *testing.T) {
		sess := &ProviderSession{
			SessionID: "session-1",
			Functions: []*sdkv1.ProviderFunctionDescriptor{},
		}

		ids := sess.FunctionIDs()
		assert.Empty(t, ids)
	})
}

func TestProviderSession_Close(t *testing.T) {
	sess := &ProviderSession{
		SessionID: "session-1",
	}

	// Should not panic even with nil conn
	err := sess.Close()
	assert.NoError(t, err)
}

// --- Tests for ProviderSessionStore ---

func TestProviderSessionStore_NewProviderSessionStore(t *testing.T) {
	store := NewProviderSessionStore()
	assert.NotNil(t, store)
	assert.NotNil(t, store.bySessionID)
	assert.NotNil(t, store.byServiceID)
}

func TestProviderSessionStore_Add(t *testing.T) {
	t.Run("add new session", func(t *testing.T) {
		store := NewProviderSessionStore()

		sess := &ProviderSession{
			SessionID: "session-1",
			ServiceID: "svc-1",
		}

		err := store.Add(sess)
		assert.NoError(t, err)

		// Verify session was added
		got, ok := store.GetBySessionID("session-1")
		assert.True(t, ok)
		assert.Equal(t, "session-1", got.SessionID)
	})

	t.Run("add duplicate session", func(t *testing.T) {
		store := NewProviderSessionStore()

		sess1 := &ProviderSession{
			SessionID: "session-1",
			ServiceID: "svc-1",
		}

		sess2 := &ProviderSession{
			SessionID: "session-1",
			ServiceID: "svc-2",
		}

		err := store.Add(sess1)
		assert.NoError(t, err)

		err = store.Add(sess2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})
}

func TestProviderSessionStore_Upsert(t *testing.T) {
	t.Run("upsert new session", func(t *testing.T) {
		store := NewProviderSessionStore()

		sess := &ProviderSession{
			SessionID: "session-1",
			ServiceID: "svc-1",
		}

		store.Upsert(sess)

		got, ok := store.GetBySessionID("session-1")
		assert.True(t, ok)
		assert.Equal(t, "svc-1", got.ServiceID)
	})

	t.Run("upsert replaces existing session", func(t *testing.T) {
		store := NewProviderSessionStore()

		sess1 := &ProviderSession{
			SessionID: "session-1",
			ServiceID: "svc-1",
		}

		sess2 := &ProviderSession{
			SessionID: "session-2",
			ServiceID: "svc-1", // Same service ID
		}

		store.Upsert(sess1)
		store.Upsert(sess2)

		// Old session should be replaced
		_, ok := store.GetBySessionID("session-1")
		assert.False(t, ok)

		// New session should exist
		got, ok := store.GetBySessionID("session-2")
		assert.True(t, ok)
		assert.Equal(t, "svc-1", got.ServiceID)
	})
}

func TestProviderSessionStore_GetBySessionID(t *testing.T) {
	t.Run("get existing session", func(t *testing.T) {
		store := NewProviderSessionStore()

		sess := &ProviderSession{
			SessionID: "session-1",
			ServiceID: "svc-1",
		}

		_ = store.Add(sess)

		got, ok := store.GetBySessionID("session-1")
		assert.True(t, ok)
		assert.Equal(t, "svc-1", got.ServiceID)
	})

	t.Run("get non-existing session", func(t *testing.T) {
		store := NewProviderSessionStore()

		got, ok := store.GetBySessionID("non-existing")
		assert.False(t, ok)
		assert.Nil(t, got)
	})
}

func TestProviderSessionStore_GetByServiceID(t *testing.T) {
	t.Run("get existing session", func(t *testing.T) {
		store := NewProviderSessionStore()

		sess := &ProviderSession{
			SessionID: "session-1",
			ServiceID: "svc-1",
		}

		_ = store.Add(sess)

		got, ok := store.GetByServiceID("svc-1")
		assert.True(t, ok)
		assert.Equal(t, "session-1", got.SessionID)
	})

	t.Run("get non-existing session", func(t *testing.T) {
		store := NewProviderSessionStore()

		got, ok := store.GetByServiceID("non-existing")
		assert.False(t, ok)
		assert.Nil(t, got)
	})
}

func TestProviderSessionStore_Remove(t *testing.T) {
	t.Run("remove existing session", func(t *testing.T) {
		store := NewProviderSessionStore()

		sess := &ProviderSession{
			SessionID: "session-1",
			ServiceID: "svc-1",
		}

		_ = store.Add(sess)
		store.Remove("session-1")

		got, ok := store.GetBySessionID("session-1")
		assert.False(t, ok)
		assert.Nil(t, got)

		got, ok = store.GetByServiceID("svc-1")
		assert.False(t, ok)
		assert.Nil(t, got)
	})

	t.Run("remove non-existing session", func(t *testing.T) {
		store := NewProviderSessionStore()

		// Should not panic
		store.Remove("non-existing")
	})
}

func TestProviderSessionStore_RemoveByServiceID(t *testing.T) {
	t.Run("remove existing session", func(t *testing.T) {
		store := NewProviderSessionStore()

		sess := &ProviderSession{
			SessionID: "session-1",
			ServiceID: "svc-1",
		}

		_ = store.Add(sess)
		store.RemoveByServiceID("svc-1")

		got, ok := store.GetBySessionID("session-1")
		assert.False(t, ok)
		assert.Nil(t, got)

		got, ok = store.GetByServiceID("svc-1")
		assert.False(t, ok)
		assert.Nil(t, got)
	})

	t.Run("remove non-existing session", func(t *testing.T) {
		store := NewProviderSessionStore()

		// Should not panic
		store.RemoveByServiceID("non-existing")
	})
}

func TestProviderSessionStore_List(t *testing.T) {
	t.Run("list multiple sessions", func(t *testing.T) {
		store := NewProviderSessionStore()

		sess1 := &ProviderSession{SessionID: "session-1", ServiceID: "svc-1"}
		sess2 := &ProviderSession{SessionID: "session-2", ServiceID: "svc-2"}

		_ = store.Add(sess1)
		_ = store.Add(sess2)

		sessions := store.List()
		assert.Len(t, sessions, 2)
	})

	t.Run("list empty store", func(t *testing.T) {
		store := NewProviderSessionStore()

		sessions := store.List()
		assert.Empty(t, sessions)
	})
}

func TestProviderSessionStore_Count(t *testing.T) {
	t.Run("count multiple sessions", func(t *testing.T) {
		store := NewProviderSessionStore()

		sess1 := &ProviderSession{SessionID: "session-1", ServiceID: "svc-1"}
		sess2 := &ProviderSession{SessionID: "session-2", ServiceID: "svc-2"}

		_ = store.Add(sess1)
		_ = store.Add(sess2)

		count := store.Count()
		assert.Equal(t, 2, count)
	})

	t.Run("count empty store", func(t *testing.T) {
		store := NewProviderSessionStore()

		count := store.Count()
		assert.Equal(t, 0, count)
	})
}

func TestProviderSessionStore_PruneStale(t *testing.T) {
	t.Run("prune stale sessions", func(t *testing.T) {
		store := NewProviderSessionStore()

		sess1 := &ProviderSession{SessionID: "session-1", ServiceID: "svc-1"}
		sess2 := &ProviderSession{SessionID: "session-2", ServiceID: "svc-2"}

		_ = store.Add(sess1)
		_ = store.Add(sess2)

		// Prune sessions older than 1 second
		pruned := store.PruneStale(1 * time.Second)
		assert.Equal(t, 0, pruned) // All sessions are fresh

		// Wait a bit and prune again
		time.Sleep(1100 * time.Millisecond)
		pruned = store.PruneStale(1 * time.Second)
		assert.Equal(t, 2, pruned) // All sessions should be pruned
	})

	t.Run("prune empty store", func(t *testing.T) {
		store := NewProviderSessionStore()

		pruned := store.PruneStale(1 * time.Second)
		assert.Equal(t, 0, pruned)
	})
}

// The role owns the same established core session; binding must neither dial
// a replacement connection nor initialize registration or heartbeat state.
func TestNewProviderSessionBindsEstablishedConnection(t *testing.T) {
	local, peer := net.Pipe()
	defer peer.Close()
	conn := tcptr.NewMuxConn(local, nil, nil)
	defer conn.Close()

	sess := NewProviderSession(conn)
	assert.Same(t, conn, sess.Conn())
	assert.Empty(t, sess.SessionID)
	assert.Zero(t, sess.LastSeen.Load())
	assert.NoError(t, sess.Close())
	assert.True(t, conn.IsClosed())
}

// --- ARCH-016: provider drain state machine ---

func TestProviderSessionBeginCallBeforeDrain(t *testing.T) {
	sess := &ProviderSession{}
	assert.True(t, sess.BeginCall())
	assert.Equal(t, 1, sess.InflightCount())
	sess.EndCall()
	assert.Equal(t, 0, sess.InflightCount())
	assert.False(t, sess.IsDraining())
}

// 已摘流后 BeginCall 拒绝新调用（停止新路由）。
func TestProviderSessionDrainRejectsNewCalls(t *testing.T) {
	sess := &ProviderSession{}
	sess.StartDraining()
	assert.True(t, sess.IsDraining())
	assert.False(t, sess.BeginCall(), "draining session must reject new calls")
	assert.Equal(t, 0, sess.InflightCount())
}

// 摘流等待在途：有在途调用时 StartDraining 不立即返回，EndCall 后解除。
func TestProviderSessionDrainWaitsForInflight(t *testing.T) {
	sess := &ProviderSession{}
	require.True(t, sess.BeginCall())

	ch := sess.StartDraining()
	assert.False(t, sess.BeginCall(), "new calls rejected while draining")

	select {
	case <-ch:
		t.Fatal("drain channel must not close while a call is in flight")
	default:
	}

	sess.EndCall()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("drain channel must close once inflight reaches zero")
	}
	require.True(t, sess.WaitDrained(time.Second), "already drained")
}

// 摘流超时退出：在途调用未在窗口内结束，WaitDrained 返回 false。
func TestProviderSessionDrainTimeout(t *testing.T) {
	sess := &ProviderSession{}
	require.True(t, sess.BeginCall())

	assert.False(t, sess.WaitDrained(20*time.Millisecond), "must time out with a call in flight")
	// 超时后调用仍结束、仍被计数释放。
	sess.EndCall()
	assert.Equal(t, 0, sess.InflightCount())
}

// 无在途调用时摘流立即完成。
func TestProviderSessionDrainImmediateWhenIdle(t *testing.T) {
	sess := &ProviderSession{}
	require.True(t, sess.WaitDrained(0), "idle session drains immediately")
	assert.True(t, sess.IsDraining())
	assert.False(t, sess.BeginCall())
}
