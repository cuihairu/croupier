package runner

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/connector"
	"github.com/cuihairu/croupier/agents/capture/source"
	cdcfilter "github.com/cuihairu/croupier/pkg/cdc/filter"
	cdjournal "github.com/cuihairu/croupier/pkg/cdc/journal"
)

func TestRunShutdownKeepsConsumedCheckpoint(t *testing.T) {
	cfg := testConfig(t, true)
	var checkpoints []connector.Position
	fp := &fakeConnector{events: []connector.ChangeEvent{{
		SourceType: "mysql", Database: "game", Table: "items",
		Operation: connector.OpInsert,
		Position:  connector.Position{Kind: "gtid", Value: "shutdown-cp"},
		TsUnixMs:  time.Now().UnixMilli(),
	}}}
	r := New(cfg, nil)
	r.cfg.Capture.BookmarkFlushIntervalSec = 1
	r.newConnector = func() connector.SourceConnector {
		return fp
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		pos := mustCheckpoint(t, fp)
		if pos.Value == "shutdown-cp" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("checkpoint not acked, got %+v", pos)
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
	checkpoints = append(checkpoints, mustCheckpoint(t, fp))
	if len(checkpoints) != 1 || checkpoints[0].Value != "shutdown-cp" {
		t.Fatalf("checkpoint snapshots = %+v, want consumed checkpoint", checkpoints)
	}

	pos, err := source.NewStore(filepath.Join(cfg.Capture.BookmarkDir, "mysql.json")).Load()
	if err != nil {
		t.Fatalf("load bookmark: %v", err)
	}
	if pos.Position != "gtid:shutdown-cp" {
		t.Fatalf("bookmark = %q, want gtid:shutdown-cp", pos.Position)
	}
	if _, err := os.Stat(r.hitSpool.path); !os.IsNotExist(err) {
		t.Fatalf("alert spool should be empty, stat err=%v", err)
	}
}

func TestConsumeDurableJournalsBeforeCheckpointAck(t *testing.T) {
	cfg := testConfig(t, true)
	r := New(cfg, nil)
	log, err := cdjournal.Open(cfg.Capture.JournalPath())
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	r.eventJournal = log
	r.journalFilter = cdcfilter.Filter{Include: []cdcfilter.Selector{{
		SourceID: cfg.Agent.ID, SourceType: "mysql", Database: "game", Table: "items",
	}}}
	cn := &fakeConnector{}
	event := connector.ChangeEvent{
		SourceID: cfg.Agent.ID, SourceType: "mysql", Database: "game", Table: "items",
		Operation: connector.OpInsert, After: map[string]json.RawMessage{"id": json.RawMessage(`1`)},
		Position: connector.Position{Kind: "gtid", Value: "persisted-first"}, TsUnixMs: time.Now().UnixMilli(),
	}
	if err := r.consumeDurable(context.Background(), event, cn); err != nil {
		t.Fatal(err)
	}
	entries, err := log.Read(1, 0)
	if err != nil || len(entries) != 1 || entries[0].Event.Position.Value != "persisted-first" {
		t.Fatalf("journal entries=%+v err=%v", entries, err)
	}
	if got := mustCheckpoint(t, cn); got.Value != "persisted-first" {
		t.Fatalf("checkpoint = %+v", got)
	}
}

func TestConsumeDurableDoesNotAckWhenJournalIsFull(t *testing.T) {
	cfg := testConfig(t, true)
	r := New(cfg, nil)
	log, err := cdjournal.OpenWithLimit(cfg.Capture.JournalPath(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	r.eventJournal = log
	r.journalFilter = cdcfilter.Filter{Include: []cdcfilter.Selector{{}}}
	cn := &fakeConnector{}
	event := connector.ChangeEvent{
		SourceID: cfg.Agent.ID, SourceType: "mysql", Database: "game", Table: "items",
		Operation: connector.OpInsert, After: map[string]json.RawMessage{"id": json.RawMessage(`1`)},
		Position: connector.Position{Kind: "gtid", Value: "not-acked"}, TsUnixMs: time.Now().UnixMilli(),
	}
	if err := r.consumeDurable(context.Background(), event, cn); err == nil {
		t.Fatal("expected capacity error")
	}
	if got := mustCheckpoint(t, cn); got.Value == "not-acked" {
		t.Fatal("source checkpoint advanced despite journal capacity failure")
	}
}

func TestRunSourceOnceWiresConfiguredJournalSelection(t *testing.T) {
	cfg := testConfig(t, true)
	cfg.Capture.Journal.Enabled = true
	cfg.Capture.Journal.MaxBytes = 4096
	cfg.Capture.Journal.Operations = []string{"insert"}
	fake := &fakeConnector{
		closeAfter: 1,
		events: []connector.ChangeEvent{{
			SourceID: cfg.Agent.ID, SourceType: "mysql", Database: "game", Table: "items",
			Operation: connector.OpInsert, After: map[string]json.RawMessage{"id": json.RawMessage(`1`)},
			Position: connector.Position{Kind: "gtid", Value: "runner-journal"}, TsUnixMs: time.Now().UnixMilli(),
		}},
	}
	r := New(cfg, nil)
	r.newConnector = func() connector.SourceConnector { return fake }
	if err := r.runSourceOnce(context.Background()); !errors.Is(err, errStreamEnded) {
		t.Fatalf("runSourceOnce error = %v", err)
	}
	log, err := cdjournal.Open(cfg.Capture.JournalPath())
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	entries, err := log.Read(1, 0)
	if err != nil || len(entries) != 1 || entries[0].Event.Position.Value != "runner-journal" {
		t.Fatalf("journal entries=%+v err=%v", entries, err)
	}
}

func mustCheckpoint(t *testing.T, cn connector.SourceConnector) connector.Position {
	t.Helper()
	pos, err := cn.Checkpoint(context.Background())
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	return pos
}
