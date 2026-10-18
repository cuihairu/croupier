package journal

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cuihairu/croupier/pkg/cdc"
	"github.com/cuihairu/croupier/pkg/cdc/filter"
)

func TestAppendIsDurableIdempotentAndReadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	j, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	e := event("tx-1", 1, true)
	seq, id, err := j.Append(e)
	if err != nil {
		t.Fatal(err)
	}
	if seq != 1 || id == "" {
		t.Fatalf("Append() = %d, %q", seq, id)
	}
	if gotSeq, gotID, err := j.Append(e); err != nil || gotSeq != seq || gotID != id {
		t.Fatalf("duplicate Append() = %d, %q, %v", gotSeq, gotID, err)
	}
	conflict := event("tx-1", 1, false)
	conflict.ID = id
	if _, _, err := j.Append(conflict); !errors.Is(err, ErrIDConflict) {
		t.Fatalf("conflicting duplicate error = %v", err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	j, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	entries, err := j.Read(1, 0)
	if err != nil || len(entries) != 1 || entries[0].ID != id || entries[0].Event.ID != id {
		t.Fatalf("Read() = %#v, %v", entries, err)
	}
}

func TestOpenTruncatesOnlyPartialFinalRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	j, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := j.Append(event("tx-1", 1, true)); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(`{"sequence":2`)
	_ = f.Close()
	j, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if st, err := os.Stat(path); err != nil || st.Size() == 0 {
		t.Fatalf("journal after recovery: stat=%v err=%v", st, err)
	}
	if _, _, err := j.Append(event("tx-2", 1, true)); err != nil {
		t.Fatalf("append after tail recovery: %v", err)
	}
}

func TestOpenRejectsCorruptCompleteRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte("not-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("expected corruption error")
	}
}

func TestReplayGroupsTransactionsAndRejectsCutRanges(t *testing.T) {
	j, err := Open(filepath.Join(t.TempDir(), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	for _, e := range []cdc.ChangeEvent{event("tx-a", 1, false), event("tx-a", 2, true), event("tx-b", 1, true)} {
		if _, _, err := j.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	sink := &recordingSink{}
	last, err := j.Replay(context.Background(), 1, 3, sink)
	if err != nil || last != 3 || !reflect.DeepEqual(sink.groups, [][]string{{"tx-a", "tx-a"}, {"tx-b"}}) {
		t.Fatalf("Replay() = %d, %v, groups=%v", last, err, sink.groups)
	}
	if _, err := j.Replay(context.Background(), 2, 3, &recordingSink{}); !errors.Is(err, ErrRangeCutsTx) {
		t.Fatalf("cut transaction error = %v", err)
	}
}

func TestReplayRejectsIncompleteTransactionAndSinkFailureDoesNotAdvance(t *testing.T) {
	j, err := Open(filepath.Join(t.TempDir(), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if _, _, err := j.Append(event("tx-open", 1, false)); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Replay(context.Background(), 1, 1, &recordingSink{}); !errors.Is(err, ErrIncompleteTx) {
		t.Fatalf("incomplete transaction error = %v", err)
	}
	fail := &recordingSink{err: errors.New("apply failed")}
	if last, err := j.Replay(context.Background(), 1, 1, fail); err == nil || last != 0 {
		t.Fatalf("failed replay = %d, %v", last, err)
	}
}

func TestReplayRejectsEventsWithoutTransactionBoundaryCapability(t *testing.T) {
	j, err := Open(filepath.Join(t.TempDir(), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if _, _, err := j.Append(event("", 0, false)); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Replay(context.Background(), 1, 1, &recordingSink{}); !errors.Is(err, ErrMissingTxMetadata) {
		t.Fatalf("missing metadata error = %v", err)
	}
}

func TestCapturePersistsSelectedEventsBeforeAcknowledging(t *testing.T) {
	j, err := Open(filepath.Join(t.TempDir(), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	selected := event("", 0, false)
	excluded := event("", 0, false)
	excluded.Table = "ignored"
	source := &fakeSource{events: []cdc.ChangeEvent{selected, excluded}}
	selection := filter.Filter{Include: []filter.Selector{{Database: "db", Table: "items"}}}
	stats, err := Capture(context.Background(), source, j, selection, cdc.PollRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if stats != (CaptureStats{Seen: 2, Selected: 1, Acked: 2}) || len(source.acked) != 2 {
		t.Fatalf("stats=%+v acked=%d", stats, len(source.acked))
	}
	rows, err := j.Read(1, 0)
	if err != nil || len(rows) != 1 || rows[0].Event.Table != "items" {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
}

func TestCaptureDoesNotAcknowledgeWhenAppendFails(t *testing.T) {
	j, err := Open(filepath.Join(t.TempDir(), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	bad := event("", 0, false)
	bad.Operation = cdc.OpUpdate // both images absent: invalid journal record
	bad.After = nil
	source := &fakeSource{events: []cdc.ChangeEvent{bad}}
	_, err = Capture(context.Background(), source, j, filter.Filter{Include: []filter.Selector{{}}}, cdc.PollRequest{})
	if err == nil || len(source.acked) != 0 {
		t.Fatalf("Capture error=%v acked=%d", err, len(source.acked))
	}
}

func event(tx string, seq int64, end bool) cdc.ChangeEvent {
	return cdc.ChangeEvent{
		SourceID: "source", SourceType: "mysql", Database: "db", Table: "items",
		Operation: cdc.OpInsert, After: map[string]json.RawMessage{"id": json.RawMessage(`1`)},
		Tx: cdc.TxInfo{ID: tx, Seq: seq, End: end}, Position: cdc.Position{Kind: "gtid", Value: "uuid:1"}, TsUnixMs: 1000,
	}
}

type recordingSink struct {
	groups [][]string
	err    error
}

type fakeSource struct {
	cdc.SourceConnector
	events []cdc.ChangeEvent
	acked  []cdc.Position
}

func (s *fakeSource) Poll(context.Context, cdc.PollRequest) (<-chan cdc.ChangeEvent, error) {
	out := make(chan cdc.ChangeEvent, len(s.events))
	for _, event := range s.events {
		out <- event
	}
	close(out)
	return out, nil
}

func (s *fakeSource) AckCheckpoint(_ context.Context, position cdc.Position) error {
	s.acked = append(s.acked, position)
	return nil
}

func (s *recordingSink) Apply(_ context.Context, events []cdc.ChangeEvent) error {
	group := make([]string, len(events))
	for i := range events {
		group[i] = events[i].Tx.ID
	}
	s.groups = append(s.groups, group)
	return s.err
}
