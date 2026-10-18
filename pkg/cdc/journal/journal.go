// Package journal stores selected CDC events in a durable append-only log and
// replays complete transaction groups through a caller-provided sink.
package journal

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/cuihairu/croupier/pkg/cdc"
)

var (
	ErrIDConflict        = errors.New("journal: event ID already exists with different content")
	ErrIncompleteTx      = errors.New("journal: incomplete transaction in replay range")
	ErrMissingTxMetadata = errors.New("journal: event has no transaction boundary metadata")
	ErrCapacityExceeded  = errors.New("journal: configured capacity exceeded")
	ErrRangeCutsTx       = errors.New("journal: replay range cuts a transaction")
	ErrInvalidReplayRow  = errors.New("journal: event is not replayable")
)

type record struct {
	Sequence uint64          `json:"sequence"`
	ID       string          `json:"id"`
	Event    cdc.ChangeEvent `json:"event"`
}

// Journal is a single-writer append-only event log. A process must not open
// the same path through multiple Journal instances concurrently.
type Journal struct {
	mu       sync.Mutex
	path     string
	file     *os.File
	next     uint64
	byID     map[string]record
	closed   bool
	fault    error
	maxBytes int64
	size     int64
}

// Open opens or creates a journal. A partial final record is truncated because
// it can only result from an interrupted append; malformed complete records
// are reported as corruption and left untouched.
func Open(path string) (*Journal, error) {
	return OpenWithLimit(path, 0)
}

// OpenWithLimit opens a journal with an optional hard file-size limit. A zero
// limit is unlimited; callers that persist continuously should set a limit and
// arrange retention or operator intervention before it is reached.
func OpenWithLimit(path string, maxBytes int64) (*Journal, error) {
	if path == "" {
		return nil, errors.New("journal: path is required")
	}
	if maxBytes < 0 {
		return nil, errors.New("journal: maxBytes cannot be negative")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("journal: create directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("journal: open: %w", err)
	}
	j := &Journal{path: path, file: f, byID: make(map[string]record), maxBytes: maxBytes}
	if err := j.recover(); err != nil {
		_ = f.Close()
		return nil, err
	}
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("journal: seek end: %w", err)
	}
	if st, err := f.Stat(); err == nil {
		j.size = st.Size()
	}
	return j, nil
}

func (j *Journal) recover() error {
	if _, err := j.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	r := bufio.NewReader(j.file)
	var offset int64
	for {
		line, err := r.ReadBytes('\n')
		if err == io.EOF {
			if len(line) != 0 {
				if truncateErr := j.file.Truncate(offset); truncateErr != nil {
					return fmt.Errorf("journal: truncate partial tail: %w", truncateErr)
				}
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("journal: read: %w", err)
		}
		var row record
		if err := json.Unmarshal(line, &row); err != nil {
			return fmt.Errorf("journal: corrupt record at byte %d: %w", offset, err)
		}
		if row.Sequence != j.next+1 || row.ID == "" || row.Event.ID != row.ID {
			return fmt.Errorf("journal: invalid record at byte %d", offset)
		}
		if err := row.Event.Validate(); err != nil {
			return fmt.Errorf("journal: invalid event at byte %d: %w", offset, err)
		}
		if previous, exists := j.byID[row.ID]; exists {
			if !sameEvent(previous.Event, row.Event) {
				return fmt.Errorf("journal: conflicting duplicate ID %q at byte %d", row.ID, offset)
			}
			return fmt.Errorf("journal: duplicate ID %q at byte %d", row.ID, offset)
		}
		j.next = row.Sequence
		j.byID[row.ID] = row
		offset += int64(len(line))
	}
}

// Append durably appends an event and returns its sequence and stable ID. A
// repeated identical event is idempotent; reuse of an ID for different content
// is rejected. The event must already have passed the caller's filter.
func (j *Journal) Append(event cdc.ChangeEvent) (uint64, string, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return 0, "", os.ErrClosed
	}
	if j.fault != nil {
		return 0, "", fmt.Errorf("journal: unavailable after prior write failure: %w", j.fault)
	}
	id := event.ID
	event.ID = ""
	if err := event.Validate(); err != nil {
		return 0, "", fmt.Errorf("journal: %w", err)
	}
	if id == "" {
		id = eventID(event)
	}
	event.ID = id
	if existing, ok := j.byID[id]; ok {
		if !sameEvent(existing.Event, event) {
			return 0, "", ErrIDConflict
		}
		return existing.Sequence, id, nil
	}
	row := record{Sequence: j.next + 1, ID: id, Event: event}
	data, err := json.Marshal(row)
	if err != nil {
		return 0, "", fmt.Errorf("journal: encode: %w", err)
	}
	data = append(data, '\n')
	if j.maxBytes > 0 && j.size+int64(len(data)) > j.maxBytes {
		return 0, "", ErrCapacityExceeded
	}
	if _, err := j.file.Seek(0, io.SeekEnd); err != nil {
		j.fault = err
		return 0, "", fmt.Errorf("journal: seek end before append: %w", err)
	}
	n, err := j.file.Write(data)
	if err != nil || n != len(data) {
		if err == nil {
			err = io.ErrShortWrite
		}
		j.fault = err
		return 0, "", fmt.Errorf("journal: append: %w", err)
	}
	if err := j.file.Sync(); err != nil {
		j.fault = err
		return 0, "", fmt.Errorf("journal: sync: %w", err)
	}
	j.next = row.Sequence
	j.size += int64(len(data))
	j.byID[id] = row
	return row.Sequence, id, nil
}

func eventID(event cdc.ChangeEvent) string {
	data, _ := json.Marshal(event)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func sameEvent(a, b cdc.ChangeEvent) bool {
	a.ID, b.ID = "", ""
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}

// Close flushes and closes the journal. It is safe to call more than once.
func (j *Journal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return nil
	}
	j.closed = true
	if err := j.file.Sync(); err != nil {
		_ = j.file.Close()
		return err
	}
	return j.file.Close()
}

// Read returns journaled events in inclusive sequence range [from, to]. A zero
// bound means the beginning or current end respectively.
func (j *Journal) Read(from, to uint64) ([]Entry, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return nil, os.ErrClosed
	}
	if from == 0 {
		from = 1
	}
	if to == 0 || to > j.next {
		to = j.next
	}
	if from > to+1 {
		return nil, fmt.Errorf("journal: invalid sequence range %d..%d", from, to)
	}
	if from > j.next || to == 0 {
		return []Entry{}, nil
	}
	if _, err := j.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	rows := make([]Entry, 0)
	scanner := bufio.NewScanner(j.file)
	scanner.Buffer(make([]byte, 64*1024), 64*1024*1024)
	var sequence uint64
	for scanner.Scan() {
		sequence++
		if sequence < from {
			continue
		}
		if sequence > to {
			break
		}
		var row record
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return nil, fmt.Errorf("journal: decode sequence %d: %w", sequence, err)
		}
		if row.Sequence != sequence {
			return nil, fmt.Errorf("journal: unexpected sequence %d at row %d", row.Sequence, sequence)
		}
		rows = append(rows, Entry{Sequence: row.Sequence, ID: row.ID, Event: row.Event})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if uint64(len(rows)) != to-from+1 {
		return nil, fmt.Errorf("journal: requested sequence range %d..%d is incomplete", from, to)
	}
	return rows, nil
}

// LastSequence returns the last durable sequence in this journal.
func (j *Journal) LastSequence() uint64 {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.next
}

// Entry is one durable journal row.
type Entry struct {
	Sequence uint64
	ID       string
	Event    cdc.ChangeEvent
}

// Sink applies a complete source transaction atomically. Implementations must
// make repeated application idempotent because a crash can happen after Apply
// commits but before the caller persists its replay cursor.
type Sink interface {
	Apply(context.Context, []cdc.ChangeEvent) error
}

// Replay applies complete transactions from an inclusive range. It returns the
// last sequence acknowledged by the sink. A transaction that begins before or
// ends after the range is rejected rather than partially applied.
func (j *Journal) Replay(ctx context.Context, from, to uint64, sink Sink) (uint64, error) {
	if sink == nil {
		return 0, errors.New("journal: replay sink is required")
	}
	entries, err := j.Read(from, to)
	if err != nil {
		return 0, err
	}
	if len(entries) == 0 {
		return 0, nil
	}
	if from == 0 {
		from = entries[0].Sequence
	}
	if to == 0 {
		to = entries[len(entries)-1].Sequence
	}
	if from > 1 {
		previous, err := j.Read(from-1, from-1)
		if err != nil {
			return 0, err
		}
		if sameTransaction(previous[0].Event, entries[0].Event) {
			return 0, ErrRangeCutsTx
		}
	}
	last := uint64(0)
	var group []Entry
	flush := func() error {
		if len(group) == 0 {
			return nil
		}
		events := make([]cdc.ChangeEvent, len(group))
		for i := range group {
			events[i] = group[i].Event
		}
		if err := sink.Apply(ctx, events); err != nil {
			return err
		}
		last = group[len(group)-1].Sequence
		group = nil
		return nil
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return last, err
		}
		if err := entry.Event.Validate(); err != nil {
			return last, fmt.Errorf("%w: %v", ErrInvalidReplayRow, err)
		}
		if entry.Event.Tx.ID == "" && !entry.Event.Tx.End {
			return last, ErrMissingTxMetadata
		}
		if len(group) > 0 && !sameTransaction(group[0].Event, entry.Event) {
			if group[0].Event.Tx.ID != "" && !group[len(group)-1].Event.Tx.End {
				return last, ErrIncompleteTx
			}
			if err := flush(); err != nil {
				return last, err
			}
		}
		if len(group) > 0 && entry.Event.Tx.ID != "" && entry.Event.Tx.Seq != group[len(group)-1].Event.Tx.Seq+1 {
			return last, ErrIncompleteTx
		}
		if len(group) == 0 && entry.Event.Tx.ID != "" && entry.Event.Tx.Seq != 1 {
			return last, ErrIncompleteTx
		}
		group = append(group, entry)
		if entry.Event.Tx.End {
			if err := flush(); err != nil {
				return last, err
			}
		}
	}
	if len(group) > 0 {
		return last, ErrIncompleteTx
	}
	if last != 0 && last < to {
		return last, ErrRangeCutsTx
	}
	return last, nil
}

func sameTransaction(a, b cdc.ChangeEvent) bool {
	if a.Tx.ID == "" || b.Tx.ID == "" {
		return false
	}
	return a.SourceID == b.SourceID && a.Tx.ID == b.Tx.ID
}
