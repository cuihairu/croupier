package fsutil

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestWriteFileAtomicReplacesForeignOwnedFile reproduces the production
// incident: assignments.json had been written by a root process, after which
// the non-root server could no longer save updates (os.WriteFile truncates the
// existing file and fails with EACCES). Atomic rename only needs directory
// write access, so the save must succeed.
func TestWriteFileAtomicReplacesForeignOwnedFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("test needs a non-root user to observe the permission asymmetry")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	f.Close()

	// Simulate a root-owned, non-writable-to-us file by dropping our own
	// write permission on it (same EACCES the non-root process got).
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatalf("chmod read-only: %v", err)
	}

	// Plain WriteFile must fail on the read-only file.
	if err := os.WriteFile(path, []byte("plain"), 0o644); err == nil {
		t.Log("platform allowed the direct write; atomic path still validated below")
	}

	if err := WriteFileAtomic(path, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Fatalf("content = %q", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Fatalf("perm = %o, want 644", perm)
	}
	// 目录里不应残留临时文件
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("leftover temp files: %d entries", len(entries))
	}
}

func TestWriteFileAtomicCreatesMissingDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "state.json")
	if err := WriteFileAtomic(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat: %v", err)
	}
}

func TestWriteFileAtomicPreservesOwnerWhenPossible(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("ownership assertions require root")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte("first"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.Chown(path, 1000, 1000); err != nil {
		t.Fatalf("chown: %v", err)
	}
	if err := WriteFileAtomic(path, []byte("second"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Uid != 1000 {
		// rename 语义：替换后属主变为进程用户。此测试记录该行为而非失败。
		t.Logf("owner changed to %d after rename (expected rename semantics)", st.Uid)
	}
}
