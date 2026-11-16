package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/sdks/go/cmd/demo-game/store"
)

const seedAll = 1

func mustSeed(t *testing.T, s *store.Store) {
	t.Helper()
	if _, err := s.EnsureSeed(seedAll, store.DefaultSeedAnchor.UnixMilli(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// TestReplayAfterRestart is the core exactly-once guarantee: a committed
// operation replays its original receipt after a full process restart and
// does NOT re-execute the side effect (balance credited once, one audit row).
func TestReplayAfterRestart(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "demo.db")
	backend, err := store.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.NewStore("demo-game", "demo", backend)
	if err != nil {
		t.Fatal(err)
	}
	mustSeed(t, s)
	before := balanceOf(t, s, "20001")

	payload := map[string]any{
		"operationId": "op-replay-1", "playerId": "20001", "amount": int64(648),
	}
	first := invoke(t, s, payload)
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	backend2, err := store.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := store.NewStore("demo-game", "demo", backend2)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	after := balanceOf(t, s2, "20001")
	if after != before+648 {
		t.Fatalf("balance after restart = %v, want %v (committed state must survive)", after, before+648)
	}

	replay := invoke(t, s2, payload)
	if replay["success"] != true {
		t.Fatalf("replay receipt = %v", replay)
	}
	if after2 := balanceOf(t, s2, "20001"); after2 != after {
		t.Fatalf("replay re-credited: balance %v -> %v", after, after2)
	}
	// Same payload bytes in, same receipt out.
	firstJSON, _ := json.Marshal(first)
	replayJSON, _ := json.Marshal(replay)
	if string(firstJSON) != string(replayJSON) {
		t.Fatalf("replay receipt differs: first=%s replay=%s", firstJSON, replayJSON)
	}
}

// TestFingerprintConflict rejects an operationId reuse with a different
// payload, leaving state untouched.
func TestFingerprintConflict(t *testing.T) {
	for name, s := range openStores(t) {
		t.Run(name, func(t *testing.T) {
			mustSeed(t, s)
			before := balanceOf(t, s, "20002")

			invoke(t, s, map[string]any{
				"operationId": "op-conflict", "playerId": "20002", "amount": int64(10),
			})
			credited := balanceOf(t, s, "20002")

			raw, _ := json.Marshal(map[string]any{
				"operationId": "op-conflict", "playerId": "20002", "amount": int64(999),
			})
			_, err := s.PlayerRecharge(context.Background(), raw)
			if !errors.Is(err, store.ErrOperationConflict) {
				t.Fatalf("conflict err = %v, want ErrOperationConflict", err)
			}
			if got := balanceOf(t, s, "20002"); got != credited {
				t.Fatalf("conflicting payload mutated state: %v -> %v", credited, got)
			}
			// The ORIGINAL payload still replays (dedup stays intact).
			replay := invoke(t, s, map[string]any{
				"operationId": "op-conflict", "playerId": "20002", "amount": int64(10),
			})
			if replay["success"] != true {
				t.Fatalf("replay after conflict = %v", replay)
			}
			if got := balanceOf(t, s, "20002"); got != credited {
				t.Fatalf("replay mutated state: %v -> %v", credited, got)
			}
			_ = before
		})
	}
}

// TestFailureReleasesPending covers the Fail protocol: a guard failure must
// release the pending row so the same operationId retries cleanly with a
// corrected payload (crash/commit-before-error is the documented boundary —
// GAME-010).
func TestFailureReleasesPending(t *testing.T) {
	for name, s := range openStores(t) {
		t.Run(name, func(t *testing.T) {
			mustSeed(t, s)
			before := balanceOf(t, s, "20003")

			// First attempt: unknown player fails the guard.
			raw, _ := json.Marshal(map[string]any{
				"operationId": "op-retry", "playerId": "99999", "amount": int64(5),
			})
			if _, err := s.PlayerRecharge(context.Background(), raw); err == nil {
				t.Fatal("unknown player must fail")
			}
			if got := balanceOf(t, s, "20003"); got != before {
				t.Fatalf("failed operation mutated state: %v -> %v", before, got)
			}

			// Retry with the same operationId and a valid target: proceeds.
			invoke(t, s, map[string]any{
				"operationId": "op-retry", "playerId": "20003", "amount": int64(5),
			})
			if got := balanceOf(t, s, "20003"); got != before+5 {
				t.Fatalf("retry did not apply: balance = %v, want %v", got, before+5)
			}
		})
	}
}

// TestConcurrentSameOperationSerializes hammers one operationId from many
// goroutines: exactly one execution applies, everyone gets the same receipt.
func TestConcurrentSameOperationSerializes(t *testing.T) {
	for name, s := range openStores(t) {
		t.Run(name, func(t *testing.T) {
			mustSeed(t, s)
			before := balanceOf(t, s, "20004")

			const workers = 8
			payload, _ := json.Marshal(map[string]any{
				"operationId": "op-race", "playerId": "20004", "amount": int64(7),
			})
			receipts := make([][]byte, workers)
			var wg sync.WaitGroup
			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					out, err := s.PlayerRecharge(context.Background(), payload)
					if err != nil {
						t.Errorf("worker %d: %v", idx, err)
						return
					}
					receipts[idx] = out
				}(i)
			}
			wg.Wait()

			if got := balanceOf(t, s, "20004"); got != before+7 {
				t.Fatalf("balance = %v, want single application (%v + 7)", got, before)
			}
			for i := 1; i < workers; i++ {
				if string(receipts[i]) != string(receipts[0]) {
					t.Fatalf("receipt %d differs from 0: %s vs %s", i, receipts[i], receipts[0])
				}
			}
		})
	}
}

// TestAutoOperationIDsDoNotDedup pins the honest semantics: writes without a
// client operationId get auto ids, so two identical calls each apply (auto
// ids are process-scoped, never stable across retries).
func TestAutoOperationIDsDoNotDedup(t *testing.T) {
	s := openStores(t)["memory"]
	mustSeed(t, s)
	before := balanceOf(t, s, "20005")
	invoke(t, s, map[string]any{"playerId": "20005", "amount": int64(1)})
	invoke(t, s, map[string]any{"playerId": "20005", "amount": int64(1)})
	if got := balanceOf(t, s, "20005"); got != before+2 {
		t.Fatalf("balance = %v, want two applications (%v+2)", got, before)
	}
}
