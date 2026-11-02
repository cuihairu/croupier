// Package logs implements the OPS-005 application log tailer: it
// incrementally reads configured log files and drains new complete lines as
// opsv1.LogEntry batches that ride the periodic metrics uplink.
package logs

import (
	"bufio"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

const (
	// DefaultMaxLineBytes caps a single log line when the caller passes 0.
	DefaultMaxLineBytes = 4096
	// DefaultMaxLinesPerCycle caps emitted lines per file per Drain when the
	// caller passes 0.
	DefaultMaxLinesPerCycle = 500
)

// levelRe extracts a log level from a raw line. "warning" normalizes to
// "warn"; unknown levels yield "".
var levelRe = regexp.MustCompile(`(?i)\b(debug|info|warn(?:ing)?|error|fatal)\b`)

// timestampFormats are the line-leading timestamps recognized for
// LogEntry.TsUnixMs. Anything else leaves the timestamp at 0 and the server
// falls back to received_at.
var timestampFormats = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05.000",
	"2006-01-02 15:04:05",
}

type fileState struct {
	offset int64
	seq    int64
}

// Tailer incrementally tails a fixed set of log files. Drain returns the new
// complete lines observed since the previous call, capped per file, with each
// line cut to MaxLineBytes on a UTF-8 boundary.
type Tailer struct {
	mu               sync.Mutex
	paths            []string
	maxLineBytes     int
	maxLinesPerCycle int
	states           map[string]*fileState
}

// NewTailer creates a tailer for paths. A maxLineBytes of 0 selects
// DefaultMaxLineBytes; a maxLinesPerCycle of 0 selects
// DefaultMaxLinesPerCycle.
func NewTailer(paths []string, maxLineBytes, maxLinesPerCycle int) *Tailer {
	if maxLineBytes <= 0 {
		maxLineBytes = DefaultMaxLineBytes
	}
	if maxLinesPerCycle <= 0 {
		maxLinesPerCycle = DefaultMaxLinesPerCycle
	}
	return &Tailer{
		paths:            append([]string(nil), paths...),
		maxLineBytes:     maxLineBytes,
		maxLinesPerCycle: maxLinesPerCycle,
		states:           make(map[string]*fileState, len(paths)),
	}
}

// MaxLineBytes returns the effective per-line byte cap.
func (t *Tailer) MaxLineBytes() int { return t.maxLineBytes }

// MaxLinesPerCycle returns the effective per-file per-cycle line cap.
func (t *Tailer) MaxLinesPerCycle() int { return t.maxLinesPerCycle }

// Drain reads every configured file and returns the new complete lines as
// entries. Files that do not exist (yet) are skipped; a file that shrank is
// treated as rotated and re-read from the start. The per-file seq stays
// monotonic across rotations so the server-side dedup stays correct.
func (t *Tailer) Drain() []*opsv1.LogEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	var entries []*opsv1.LogEntry
	for _, path := range t.paths {
		entries = append(entries, t.drainFile(path)...)
	}
	return entries
}

func (t *Tailer) drainFile(path string) []*opsv1.LogEntry {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.IsDir() {
		return nil
	}

	st := t.states[path]
	if st == nil {
		st = &fileState{}
		t.states[path] = st
	}
	// Rotation or log rotation-style cut: restart from the beginning, the
	// per-file seq keeps increasing.
	if info.Size() < st.offset {
		st.offset = 0
	}
	if _, err := f.Seek(st.offset, 0); err != nil {
		return nil
	}

	var entries []*opsv1.LogEntry
	reader := bufio.NewReaderSize(f, 64*1024)
	for len(entries) < t.maxLinesPerCycle {
		line, err := readLine(reader)
		if err != nil {
			// EOF or read error: never emit a partial line; the offset
			// stays put so the next Drain re-reads it once the write
			// completes.
			break
		}
		st.offset += int64(len(line))
		st.seq++
		entries = append(entries, newEntry(path, line, st.seq, t.maxLineBytes))
	}
	return entries
}

// readLine returns the next line including its trailing newline so the
// caller can advance its offset byte-for-byte; the newline is stripped from
// the entry message. A final line without a newline surfaces as an error so
// the partial write is retried on the next cycle.
func readLine(reader *bufio.Reader) (string, error) {
	var b strings.Builder
	for {
		chunk, err := reader.ReadSlice('\n')
		b.Write(chunk)
		if err == nil {
			return b.String(), nil
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		return b.String(), err
	}
}

func newEntry(path, rawLine string, seq int64, maxLineBytes int) *opsv1.LogEntry {
	line := strings.TrimRight(rawLine, "\r\n")
	level := ""
	if m := levelRe.FindStringSubmatch(line); m != nil {
		level = strings.ToLower(m[1])
		if level == "warning" {
			level = "warn"
		}
	}
	return &opsv1.LogEntry{
		File:     path,
		Level:    level,
		Message:  clip(line, maxLineBytes),
		TsUnixMs: parseTs(line),
		Seq:      seq,
	}
}

// clip cuts s to at most max bytes without splitting a UTF-8 rune, appending
// an ellipsis marker when anything was removed.
func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max - 3
	for cut > 0 && !utf8.ValidString(s[:cut]) {
		cut--
	}
	if cut < 0 {
		cut = 0
	}
	return s[:cut] + "..."
}

// parseTs extracts a line-leading timestamp and returns it as Unix
// milliseconds; 0 when no parseable timestamp is present.
func parseTs(line string) int64 {
	for _, layout := range timestampFormats {
		if len(line) < len(layout) {
			continue
		}
		if ts, err := time.Parse(layout, line[:len(layout)]); err == nil {
			return ts.UnixMilli()
		}
	}
	return 0
}
