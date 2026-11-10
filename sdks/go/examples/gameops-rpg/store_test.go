package main

import (
	"math/rand"
	"path/filepath"
	"testing"
)

func TestSeedInvariants(t *testing.T) {
	s := testStore(t)
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.players) != 10 {
		t.Fatalf("players=%d want 10", len(s.players))
	}
	if len(s.items) != 8 || len(s.products) != 5 || len(s.quests) != 6 {
		t.Fatalf("catalog items=%d products=%d quests=%d", len(s.items), len(s.products), len(s.quests))
	}
	if s.mailByIDLocked("mail-9001") == nil {
		t.Fatal("compensation mail missing")
	}
	if !s.mailExpiredLocked(s.mailByIDLocked("mail-9002")) {
		t.Fatal("seeded expired mail must be past expiry")
	}
	if s.activeBanLocked("10007") == nil {
		t.Fatal("seeded active ban for 10007 missing")
	}
	if o := s.orderByNoLocked("20261110001"); o == nil || o.Status != "paid" {
		t.Fatalf("seeded paid order: %+v", o)
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rpg-state.json")
	s := newRPGStore(rand.New(rand.NewSource(42)), path)
	before := diamondsOf(t, s, "10001")
	if _, err := s.shipOrder("20261110001", "tester"); err != nil {
		t.Fatalf("ship: %v", err)
	}
	if _, err := s.claimMail("mail-9001", "player"); err != nil {
		t.Fatalf("claim: %v", err)
	}

	// A fresh process reconstructs the mutated world from the snapshot.
	reborn := newRPGStore(rand.New(rand.NewSource(1)), path)
	if got := diamondsOf(t, reborn, "10001"); got != before+6 {
		t.Fatalf("restored diamonds=%d want %d", got, before+6)
	}
	reborn.mu.RLock()
	claimed := reborn.mailByIDLocked("mail-9001").Claimed
	order := reborn.orderByNoLocked("20261110001").Status
	runSeq := reborn.reportRunSeq
	catalog := len(reborn.items)
	reborn.mu.RUnlock()
	if !claimed || order != "shipped" {
		t.Fatalf("restored claimed=%v order=%s", claimed, order)
	}
	if catalog == 0 {
		t.Fatal("catalog lost on restore")
	}

	// reportRunSeq increments persist too: run one report, restart, run again.
	if _, err := reborn.reportGenerate(contextTODO(), mustJSON(t, map[string]any{"delayMs": 0})); err != nil {
		t.Fatalf("report: %v", err)
	}
	again := newRPGStore(rand.New(rand.NewSource(2)), path)
	again.mu.RLock()
	seq := again.reportRunSeq
	again.mu.RUnlock()
	if seq != runSeq+1 {
		t.Fatalf("reportRunSeq across restart=%d want %d", seq, runSeq+1)
	}
}

func TestSnapshotRejectsUnknownVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rpg-state.json")
	s := newRPGStore(rand.New(rand.NewSource(42)), path)
	writeJSONFile(t, path, `{"version": 999, "players": []}`)
	reborn := newRPGStore(rand.New(rand.NewSource(42)), path)
	reborn.mu.RLock()
	players := len(reborn.players)
	reborn.mu.RUnlock()
	if players != 10 {
		t.Fatalf("future version must fall back to seed, players=%d", players)
	}
	_ = s
}
