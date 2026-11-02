package logs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDrainReadsNewLinesOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, []byte("INFO hello\nERROR boom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tr := NewTailer([]string{path}, 0, 0)

	got := tr.Drain()
	if len(got) != 2 {
		t.Fatalf("want 2 entries, got %d", len(got))
	}
	if got[0].Message != "INFO hello" || got[0].Level != "info" || got[0].Seq != 1 {
		t.Errorf("entry0 = %+v", got[0])
	}
	if got[1].Message != "ERROR boom" || got[1].Level != "error" || got[1].Seq != 2 {
		t.Errorf("entry1 = %+v", got[1])
	}
	if got[0].File != path {
		t.Errorf("file = %q", got[0].File)
	}

	// No new lines -> nothing.
	if e := tr.Drain(); len(e) != 0 {
		t.Fatalf("second drain want 0, got %d", len(e))
	}

	// Append one line; seq continues.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("WARN careful\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	got = tr.Drain()
	if len(got) != 1 || got[0].Level != "warn" || got[0].Seq != 3 {
		t.Fatalf("append drain = %+v", got)
	}
}

func TestDrainSkipsPartialLineUntilComplete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, []byte("INFO hello\nINFO par"), 0o644); err != nil {
		t.Fatal(err)
	}
	tr := NewTailer([]string{path}, 0, 0)
	if got := tr.Drain(); len(got) != 1 {
		t.Fatalf("want 1 complete line, got %d", len(got))
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("tial\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	got := tr.Drain()
	if len(got) != 1 || got[0].Message != "INFO partial" || got[0].Seq != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestDrainHandlesRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, []byte("INFO one\nINFO two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tr := NewTailer([]string{path}, 0, 0)
	tr.Drain()
	// Rotate: file replaced with a shorter one.
	if err := os.WriteFile(path, []byte("INFO fresh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := tr.Drain()
	if len(got) != 1 || got[0].Message != "INFO fresh" {
		t.Fatalf("rotation drain = %+v", got)
	}
	if got[0].Seq != 3 {
		t.Errorf("seq must stay monotonic across rotation, got %d", got[0].Seq)
	}
}

func TestDrainCapsLinesPerCycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	var sb strings.Builder
	for i := 0; i < 5; i++ {
		sb.WriteString("INFO x\n")
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	tr := NewTailer([]string{path}, 0, 2)
	if got := tr.Drain(); len(got) != 2 {
		t.Fatalf("want capped 2, got %d", len(got))
	}
	if got := tr.Drain(); len(got) != 2 {
		t.Fatalf("want capped 2, got %d", len(got))
	}
	if got := tr.Drain(); len(got) != 1 {
		t.Fatalf("want last 1, got %d", len(got))
	}
}

func TestLevelAndTimestampParsing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	content := "2026-11-01 12:00:05.123 [ERROR] bad thing\n" +
		"2026-11-01T12:00:06Z level=warning gentle\n" +
		"no metadata here\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got := NewTailer([]string{path}, 0, 0).Drain()
	if got[0].Level != "error" || got[0].TsUnixMs == 0 {
		t.Errorf("entry0 = %+v", got[0])
	}
	if got[1].Level != "warn" {
		t.Errorf("warning must normalize to warn, got %q", got[1].Level)
	}
	if got[2].Level != "" || got[2].TsUnixMs != 0 {
		t.Errorf("entry2 = %+v", got[2])
	}
}

func TestClipLongLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	long := strings.Repeat("汉", 5000) + "\n" // 15000 bytes of 3-byte runes
	if err := os.WriteFile(path, []byte(long), 0o644); err != nil {
		t.Fatal(err)
	}
	tr := NewTailer([]string{path}, 100, 0)
	got := tr.Drain()
	if len(got) != 1 {
		t.Fatal("want 1 entry")
	}
	m := got[0].Message
	if len(m) > 100 {
		t.Errorf("clipped length = %d", len(m))
	}
	if !utf8.ValidString(m) {
		t.Error("clipped message must stay valid UTF-8")
	}
	if !strings.HasSuffix(m, "...") {
		t.Errorf("clipped message must end with ellipsis: %q", m[len(m)-8:])
	}
}

func TestMissingFileSkipped(t *testing.T) {
	dir := t.TempDir()
	tr := NewTailer([]string{filepath.Join(dir, "nope.log")}, 0, 0)
	if got := tr.Drain(); len(got) != 0 {
		t.Fatalf("want 0, got %d", len(got))
	}
}

func TestDefaults(t *testing.T) {
	tr := NewTailer(nil, 0, 0)
	if tr.MaxLineBytes() != DefaultMaxLineBytes || tr.MaxLinesPerCycle() != DefaultMaxLinesPerCycle {
		t.Fatalf("defaults not applied: %d %d", tr.MaxLineBytes(), tr.MaxLinesPerCycle())
	}
	if got := tr.Drain(); got != nil {
		t.Fatalf("nil paths drain = %+v", got)
	}
}
