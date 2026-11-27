package protocol

import (
	"sync"
	"testing"
)

// AUD-101: the per-connection negotiation rule matrix. Today the shipped
// window is [1,1] — the N/N-1 rows rehearse the next version bump via
// explicit windows before Version1 moves.
func TestNegotiation_Matrix(t *testing.T) {
	cases := []struct {
		name      string
		min, max  uint8
		offers    []uint8
		wantLock  uint8
		wantErrAt int // index of offers that must error; -1 = none
	}{
		{"v1 only, v1 peer", 1, 1, []uint8{1}, 1, -1},
		{"N-1 peer on N window locks down", 1, 2, []uint8{1}, 1, -1},
		{"N peer on N window locks up", 1, 2, []uint8{2}, 2, -1},
		{"out of window never locks", 1, 1, []uint8{2}, 1, 0},
		{"conflict after lock", 1, 2, []uint8{2, 1}, 2, 1},
		{"conflict after downgrade lock", 1, 2, []uint8{1, 2}, 1, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := NewNegotiationWindow(tc.min, tc.max)
			for i, offer := range tc.offers {
				got, err := n.Offer(offer)
				if i == tc.wantErrAt {
					if err == nil {
						t.Fatalf("offer %d: expected error", offer)
					}
					if _, isConflict := err.(*ErrVersionConflict); offer <= tc.max && offer >= tc.min && !isConflict {
						t.Fatalf("offer %d: in-window deviation must be ErrVersionConflict, got %v", offer, err)
					}
					continue
				}
				if err != nil {
					t.Fatalf("offer %d: unexpected error %v", offer, err)
				}
				if got != tc.wantLock {
					t.Fatalf("offer %d: locked %d, want %d", offer, got, tc.wantLock)
				}
			}
			if n.Version() != tc.wantLock {
				t.Fatalf("final version %d, want %d", n.Version(), tc.wantLock)
			}
		})
	}
}

func TestNegotiation_UnlockedVersionIsLocalMax(t *testing.T) {
	if got := NewNegotiation().Version(); got != Version1 {
		t.Fatalf("unlocked default negotiation version %d, want Version1", got)
	}
	if got := NewNegotiationWindow(1, 2).Version(); got != 2 {
		t.Fatalf("unlocked widened negotiation version %d, want 2", got)
	}
}

func TestNegotiation_ReconnectStartsFresh(t *testing.T) {
	first := NewNegotiationWindow(1, 2)
	if _, err := first.Offer(2); err != nil {
		t.Fatalf("first connection offer: %v", err)
	}
	// A new TCP connection must negotiate from scratch, not inherit the lock.
	second := NewNegotiationWindow(1, 2)
	if got, err := second.Offer(1); err != nil || got != 1 {
		t.Fatalf("reconnect offer 1: got %d err %v", got, err)
	}
}

func TestNegotiation_InvalidWindowPanics(t *testing.T) {
	mustPanic(t, func() { NewNegotiationWindow(0, 1) })
	mustPanic(t, func() { NewNegotiationWindow(2, 1) })
}

func TestNegotiation_ConcurrentOffersAreSticky(t *testing.T) {
	n := NewNegotiationWindow(1, 2)
	var wg sync.WaitGroup
	versions := make([]uint8, 8)
	for i := range versions {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			v, _ := n.Offer(2)
			versions[slot] = v
		}(i)
	}
	wg.Wait()
	for slot, v := range versions {
		if v != 2 {
			t.Fatalf("slot %d got version %d, want locked 2", slot, v)
		}
	}
}

func mustPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic")
		}
	}()
	fn()
}
