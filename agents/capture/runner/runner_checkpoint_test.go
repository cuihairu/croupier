package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/connector"
	"github.com/cuihairu/croupier/agents/capture/source"
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

func mustCheckpoint(t *testing.T, cn connector.SourceConnector) connector.Position {
	t.Helper()
	pos, err := cn.Checkpoint(context.Background())
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	return pos
}
