package session

import (
	"testing"
	"time"
)

// hbPruneSignalStore is a deterministic SessionStore mock: every PruneStale
// invocation signals the configured TTL on a buffered channel (non-blocking
// send) and reports one pruned session.
type hbPruneSignalStore struct {
	calls chan time.Duration
}

func (s *hbPruneSignalStore) PruneStale(ttl time.Duration) int {
	select {
	case s.calls <- ttl:
	default:
	}
	return 1
}

func (s *hbPruneSignalStore) Count() int { return 0 }

// TestHeartbeatManager_StartAfterStopped covers the already-stopped branch
// in Start: when stopCh is closed, Start must return without launching the
// run loop. Stop() recycles stopCh, so the only way to reach this state is
// to close the channel directly (white-box, same package).
func TestHeartbeatManager_StartAfterStopped(t *testing.T) {
	store := &hbPruneSignalStore{calls: make(chan time.Duration, 8)}
	m := NewHeartbeatManager(store, time.Second, time.Millisecond, nil)

	oldCh := m.stopCh
	close(m.stopCh) // simulate a manager whose stop channel is already closed

	m.Start() // must take the already-stopped branch and return immediately

	if m.stopCh != oldCh {
		t.Fatal("Start must not replace stopCh")
	}
	// Sanity check: no prune tick may fire after Start on a stopped manager.
	select {
	case <-store.calls:
		t.Fatal("Start on a stopped manager must not run prune ticks")
	case <-time.After(50 * time.Millisecond):
	}
}

// TestHeartbeatManager_StopAlreadyStopped covers the already-stopped branch
// in Stop: when stopCh is already closed (and not recycled by a previous
// Stop), Stop must be a no-op and must not recreate the channel.
func TestHeartbeatManager_StopAlreadyStopped(t *testing.T) {
	store := &hbPruneSignalStore{calls: make(chan time.Duration, 8)}
	m := NewHeartbeatManager(store, time.Second, time.Millisecond, nil)

	oldCh := m.stopCh
	close(m.stopCh)

	m.Stop() // must take the already-stopped branch

	if m.stopCh != oldCh {
		t.Fatal("Stop on an already-stopped manager must not recycle stopCh")
	}
}

// TestHeartbeatManager_PruneWithHandler covers the pruneStale branch where
// the store reports pruned > 0 while a non-nil handler is installed.
// Synchronization is done via the calls channel; no sleeps.
func TestHeartbeatManager_PruneWithHandler(t *testing.T) {
	store := &hbPruneSignalStore{calls: make(chan time.Duration, 8)}
	m := NewHeartbeatManager(store, 3*time.Second, time.Millisecond, func(session any) {})

	m.Start()
	defer m.Stop()

	select {
	case ttl := <-store.calls:
		if ttl != 3*time.Second {
			t.Fatalf("PruneStale ttl = %v, want %v", ttl, 3*time.Second)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("prune tick never fired")
	}
}
