package chain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewWriterMkdirFailureV9(t *testing.T) {
	tmpDir := t.TempDir()
	blocker := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if _, err := NewWriter(filepath.Join(blocker, "audit.log")); err == nil {
		t.Error("expected MkdirAll failure when parent path is a regular file")
	}
}

func TestNewWriterOpenFileFailureV9(t *testing.T) {
	tmpDir := t.TempDir()
	if _, err := NewWriter(tmpDir); err == nil {
		t.Error("expected OpenFile failure when target path is a directory")
	}
}

func TestWriterLogWriteErrorV9(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "audit.log")
	w, err := NewWriter(logPath)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if err := w.Log("kind", "actor", "target", nil); err == nil {
		t.Error("expected write error after writer closed")
	}
}

func TestNewWriterAppendsToExistingFileV9(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "audit.log")

	w1, err := NewWriter(logPath)
	if err != nil {
		t.Fatalf("first NewWriter failed: %v", err)
	}
	if err := w1.Log("first", "actor", "target", nil); err != nil {
		t.Fatalf("first Log failed: %v", err)
	}
	if err := w1.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}

	w2, err := NewWriter(logPath)
	if err != nil {
		t.Fatalf("second NewWriter failed: %v", err)
	}
	if err := w2.Log("second", "actor", "target", nil); err != nil {
		t.Fatalf("second Log failed: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	var ev Event
	if err := json.Unmarshal([]byte(lines[0]), &ev); err != nil {
		t.Fatalf("unmarshal first event failed: %v", err)
	}
	if ev.Kind != "first" {
		t.Errorf("first event kind = %q", ev.Kind)
	}
	if err := json.Unmarshal([]byte(lines[1]), &ev); err != nil {
		t.Fatalf("unmarshal second event failed: %v", err)
	}
	if ev.Kind != "second" {
		t.Errorf("second event kind = %q", ev.Kind)
	}
	if ev.Prev == "" {
		t.Error("second event should carry previous hash")
	}
}
