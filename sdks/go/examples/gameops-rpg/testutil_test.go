package main

import (
	"context"
	"encoding/json"
	"math/rand"
	"os"
	"testing"
)

func contextTODO() context.Context { return context.Background() }

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func testStore(t *testing.T) *rpgStore {
	t.Helper()
	return newRPGStore(rand.New(rand.NewSource(42)))
}

func mapResult(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func goldOf(t *testing.T, s *rpgStore, id string) int64 {
	t.Helper()
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := s.playerByIDLocked(id)
	if p == nil {
		t.Fatalf("player %s missing", id)
	}
	return p.Gold
}

func diamondsOf(t *testing.T, s *rpgStore, id string) int64 {
	t.Helper()
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := s.playerByIDLocked(id)
	if p == nil {
		t.Fatalf("player %s missing", id)
	}
	return p.Diamonds
}

func inventoryCount(t *testing.T, s *rpgStore, playerID, itemID string) int64 {
	t.Helper()
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := s.playerByIDLocked(playerID)
	if p == nil {
		t.Fatalf("player %s missing", playerID)
	}
	return p.Inventory[itemID]
}

func writeJSONFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
