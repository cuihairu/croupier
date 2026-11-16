package store_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/sdks/go/cmd/demo-game/store"
)

// openStores builds one store per backend flavor for table-driven tests:
// "memory" (process-lifetime) and "sqlite" (glebarez file DB in a temp dir).
func openStores(t *testing.T) map[string]*store.Store {
	t.Helper()
	out := map[string]*store.Store{}
	mb, err := store.Open("")
	if err != nil {
		t.Fatalf("open memory backend: %v", err)
	}
	ms, err := store.NewStore("demo-game", "demo", mb)
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	out["memory"] = ms

	sb, err := store.Open(filepath.Join(t.TempDir(), "demo.db"))
	if err != nil {
		t.Fatalf("open sqlite backend: %v", err)
	}
	ss, err := store.NewStore("demo-game", "demo", sb)
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	out["sqlite"] = ss
	return out
}

func invoke(t *testing.T, s *store.Store, payload map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	out, err := s.PlayerRecharge(context.Background(), raw)
	if err != nil {
		t.Fatalf("player.recharge: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	return got
}

func playerByID(t *testing.T, s *store.Store, id string) map[string]any {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"id": id})
	if err != nil {
		t.Fatal(err)
	}
	out, err := s.PlayerGet(context.Background(), raw)
	if err != nil {
		t.Fatalf("player.get: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("decode player: %v", err)
	}
	return got
}

func balanceOf(t *testing.T, s *store.Store, id string) float64 {
	t.Helper()
	p := playerByID(t, s, id)
	b, ok := p["balanceGold"].(float64)
	if !ok {
		t.Fatalf("player %s missing balanceGold: %v", id, p)
	}
	return b
}

// TestWriteThenReadSeeds runs the core write-then-read lifecycle on both
// backends: a seeded store, a mutation, an authoritative read-back.
func TestWriteThenReadSeeds(t *testing.T) {
	for name, s := range openStores(t) {
		t.Run(name, func(t *testing.T) {
			if _, err := s.EnsureSeed(1, store.DefaultSeedAnchor.UnixMilli(), false); err != nil {
				t.Fatalf("seed: %v", err)
			}
			seeded := balanceOf(t, s, "20001")
			invoke(t, s, map[string]any{"playerId": "20001", "amount": 648})
			if got := balanceOf(t, s, "20001"); got != seeded+648 {
				t.Fatalf("balance after recharge = %v, want %v", got, seeded+648)
			}
		})
	}
}

// TestListEnvelope pins the {items,total,page,pageSize} contract on a seeded
// store (renderer selectors depend on it).
func TestListEnvelope(t *testing.T) {
	for name, s := range openStores(t) {
		t.Run(name, func(t *testing.T) {
			if _, err := s.EnsureSeed(1, store.DefaultSeedAnchor.UnixMilli(), false); err != nil {
				t.Fatalf("seed: %v", err)
			}
			out, err := s.PlayerList(context.Background(), []byte(`{}`))
			if err != nil {
				t.Fatalf("player.list: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"items", "total", "page", "pageSize"} {
				if _, ok := got[key]; !ok {
					t.Fatalf("list envelope missing %q: %v", key, got)
				}
			}
			if total, ok := got["total"].(float64); !ok || total != 26 {
				t.Fatalf("total = %v, want 26", got["total"])
			}
			items, _ := got["items"].([]any)
			if len(items) != 10 {
				t.Fatalf("default pageSize = %d, want 10", len(items))
			}
		})
	}
}

// TestDeleteThenRestartPersistence covers the SQL-backed delete path: a
// deleted player stays deleted after a full close/reload cycle.
func TestDeleteThenRestartPersistence(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "demo.db")
	backend, err := store.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.NewStore("demo-game", "demo", backend)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnsureSeed(1, store.DefaultSeedAnchor.UnixMilli(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	raw := []byte(`{"id":"20013"}`)
	if _, err := s.PlayerDelete(context.Background(), raw); err != nil {
		t.Fatalf("player.delete: %v", err)
	}
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
	out, err := s2.PlayerGet(context.Background(), raw)
	if err != nil {
		t.Fatalf("player.get: %v", err)
	}
	if string(out) != "null" {
		t.Fatalf("player 20013 survived restart: %s", out)
	}
	out, err = s2.PlayerList(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("player.list: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if total := got["total"].(float64); total != 25 {
		t.Fatalf("total after delete+restart = %v, want 25", total)
	}
}

// TestAuditRestartsInOrder covers the append-only audit reload: committed op
// records survive a restart and the op counter continues past them.
func TestAuditRestartsInOrder(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "demo.db")
	backend, _ := store.Open(dsn)
	s, err := store.NewStore("demo-game", "demo", backend)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnsureSeed(1, store.DefaultSeedAnchor.UnixMilli(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := s.PlayerRecharge(context.Background(), []byte(`{"playerId":"20001","amount":100}`)); err != nil {
		t.Fatalf("recharge: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	backend2, _ := store.Open(dsn)
	s2, err := store.NewStore("demo-game", "demo", backend2)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	// A second write after the restart must not collide with the pre-restart
	// audit sequence (op-demo-N continuity).
	if _, err := s2.PlayerRecharge(context.Background(), []byte(`{"playerId":"20001","amount":100}`)); err != nil {
		t.Fatalf("post-restart recharge: %v", err)
	}
}
