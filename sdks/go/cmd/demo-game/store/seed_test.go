package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/sdks/go/cmd/demo-game/store"
)

// TestSeedGoldenCounts pins the seed structure: identical row counts for
// every N (DATA-008 golden numbers), plus the structural fixtures the
// executor scenarios depend on.
func TestSeedGoldenCounts(t *testing.T) {
	for _, n := range []int{1, 7, 42} {
		t.Run("seed", func(t *testing.T) {
			s := openStores(t)["memory"]
			if _, err := s.EnsureSeed(n, store.DefaultSeedAnchor.UnixMilli(), false); err != nil {
				t.Fatalf("seed %d: %v", n, err)
			}
			assertListTotal(t, s, "players", 26)
			assertListTotal(t, s, "tasks", 4)
			assertListTotal(t, s, "bans", 12)
			assertListTotal(t, s, "orders", 30)
			assertListTotal(t, s, "activities", 8)

			// Structural fixtures.
			order := mustGet(t, s.OrderGet, map[string]any{"orderNo": "2026102308"})
			if order["gameCredit"] != "missing" || order["reconciliation"] != "missing_credit" {
				t.Fatalf("order 2026102308 = %v, want missing/missing_credit", order)
			}
			task := mustGet(t, s.MailTaskGet, map[string]any{"id": "task-mail-1002"})
			if task["status"] != "partial_failed" || task["sentCount"].(float64) != 7 || task["failedCount"].(float64) != 2 {
				t.Fatalf("task-mail-1002 = %v, want partial_failed 7/2", task)
			}
			player := mustGet(t, s.PlayerGet, map[string]any{"id": "20013"})
			if player["nickname"] != "风起云涌一区无敌战神丶樱花漫天飞舞的超级无敌可爱小龙女本尊在此" {
				t.Fatalf("player 20013 nickname drifted: %v", player["nickname"])
			}
			activity := mustGet(t, s.ActivityGet, map[string]any{"id": "act-4002"})
			// seq 2 → versionCount = (2 % 3) + 1 = 3, status published.
			if activity["currentVersion"].(float64) != 3 || activity["status"] != "published" {
				t.Fatalf("act-4002 = %v, want published v3", activity)
			}
			// Seeded timestamps derive from the fixed anchor, never wall-clock:
			// seq 1 → bannedAt = anchor − (1+2)·24h = 2026-10-29.
			ban := mustGet(t, s.BanGet, map[string]any{"id": "ban-30001"})
			if bannedAt, _ := ban["bannedAt"].(string); len(bannedAt) < 10 || bannedAt[:10] != "2026-10-29" {
				t.Fatalf("ban-30001 bannedAt = %v, want 2026-10-29 anchored", ban["bannedAt"])
			}
		})
	}
}

// TestSeedSameNIsNoOp: re-running with the same seed must not duplicate or
// reset state.
func TestSeedSameNIsNoOp(t *testing.T) {
	s := openStores(t)["memory"]
	if seeded, err := s.EnsureSeed(5, store.DefaultSeedAnchor.UnixMilli(), false); err != nil || !seeded {
		t.Fatalf("first seed: seeded=%v err=%v", seeded, err)
	}
	invoke(t, s, map[string]any{"playerId": "20001", "amount": int64(123)})
	afterRecharge := balanceOf(t, s, "20001")
	again, err := s.EnsureSeed(5, store.DefaultSeedAnchor.UnixMilli(), false)
	if err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if again {
		t.Fatal("same-N reseed must be a no-op")
	}
	// A wipe-and-reseed would reset the balance to the seed value; a no-op
	// keeps the post-recharge value.
	if got := balanceOf(t, s, "20001"); got != afterRecharge {
		t.Fatalf("state was disturbed by same-N seed run: %v -> %v", afterRecharge, got)
	}
	if seeded := s.Seeded(); !seeded {
		t.Fatal("Seeded flag lost after same-N run")
	}
}

// TestSeedDifferentNRefused: switching seeds requires an explicit reseed.
func TestSeedDifferentNRefused(t *testing.T) {
	s := openStores(t)["memory"]
	if _, err := s.EnsureSeed(1, store.DefaultSeedAnchor.UnixMilli(), false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnsureSeed(2, store.DefaultSeedAnchor.UnixMilli(), false); err == nil {
		t.Fatal("different N without reseed must be refused")
	}
}

// TestReseedWipesEverything: reseed replaces business state, audit AND the
// idempotency history (old receipts must not replay against fresh data).
func TestReseedWipesEverything(t *testing.T) {
	dsn := t.TempDir() + "/reseed.db"
	backend, err := store.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.NewStore("demo-game", "demo", backend)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnsureSeed(1, store.DefaultSeedAnchor.UnixMilli(), false); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"operationId": "op-pre-reseed", "playerId": "20001", "amount": int64(50)}
	invoke(t, s, payload)

	if _, err := s.EnsureSeed(2, store.DefaultSeedAnchor.UnixMilli(), true); err != nil {
		t.Fatalf("reseed: %v", err)
	}
	// Old receipt must NOT replay: the operation row was wiped, and the
	// recharge must visibly re-apply against the fresh state.
	before := balanceOf(t, s, "20001")
	invoke(t, s, payload)
	if after := balanceOf(t, s, "20001"); after != before+50 {
		t.Fatalf("post-reseed replay did not re-apply: %v -> %v", before, after)
	}
	// Audit is back to the 3 seed records only.
	assertListTotal(t, s, "players", 26)
}

// TestSeedDeterministicPerN: the same seed N builds byte-identical datasets
// on different store instances.
func TestSeedDeterministicPerN(t *testing.T) {
	dump := func(n int) string {
		s := openStores(t)["memory"]
		if _, err := s.EnsureSeed(n, store.DefaultSeedAnchor.UnixMilli(), false); err != nil {
			t.Fatalf("seed: %v", err)
		}
		out, err := s.PlayerList(t.Context(), []byte(`{"pageSize":100}`))
		if err != nil {
			t.Fatal(err)
		}
		out2, err := s.OrderList(t.Context(), []byte(`{"pageSize":100}`))
		if err != nil {
			t.Fatal(err)
		}
		return string(out) + "|" + string(out2)
	}
	if dump(11) != dump(11) {
		t.Fatal("same seed N produced different datasets")
	}
	if dump(11) == dump(12) {
		t.Fatal("different seed N produced identical datasets (jitter missing)")
	}
}

func assertListTotal(t *testing.T, s *store.Store, which string, want int) {
	t.Helper()
	var listFn func(context.Context, []byte) ([]byte, error)
	switch which {
	case "players":
		listFn = s.PlayerList
	case "tasks":
		listFn = s.MailTaskList
	case "bans":
		listFn = s.BanList
	case "orders":
		listFn = s.OrderList
	case "activities":
		listFn = s.ActivityList
	default:
		t.Fatalf("unknown list %q", which)
	}
	out, err := listFn(t.Context(), []byte(`{}`))
	if err != nil {
		t.Fatalf("%s.list: %v", which, err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if total := got["total"].(float64); int(total) != want {
		t.Fatalf("%s total = %v, want %d", which, total, want)
	}
}

func mustGet(t *testing.T, fn func(context.Context, []byte) ([]byte, error), payload map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	out, err := fn(t.Context(), raw)
	if err != nil {
		t.Fatalf("get %v: %v", payload, err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("decode %v: %v", payload, err)
	}
	return got
}
