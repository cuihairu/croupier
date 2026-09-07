package fsutil

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tmpFilePattern reports whether name looks like the temp file created by
// WriteFileAtomic for base (".<base>-RANDOM.tmp").
func tmpFilePattern(name, base string) bool {
	return strings.HasPrefix(name, "."+base+"-") && strings.HasSuffix(name, ".tmp")
}

// TestWriteFileAtomicChmodError covers the os.Chmod error branch: a concurrent
// goroutine unlinks the temp file between creation and chmod. On Linux writes
// through the still-open fd keep succeeding, so the deferred
// os.Chmod(tmpName, perm) fails with ENOENT while Write and Close succeeded.
func TestWriteFileAtomicChmodError(t *testing.T) {
	dir := t.TempDir()
	base := "state.json"

	var stop int32
	go func() {
		for atomic.LoadInt32(&stop) == 0 {
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if tmpFilePattern(e.Name(), base) {
					_ = os.Remove(filepath.Join(dir, e.Name()))
				}
			}
		}
	}()
	defer atomic.StoreInt32(&stop, 1)

	// A large payload widens the write window so the unlink reliably lands
	// before the chmod executes.
	payload := make([]byte, 8*1024*1024)

	hit := false
	deadline := time.Now().Add(15 * time.Second)
	for !hit && time.Now().Before(deadline) {
		err := WriteFileAtomic(filepath.Join(dir, base), payload, 0o644)
		var pathErr *os.PathError
		if errors.As(err, &pathErr) && pathErr.Op == "chmod" {
			hit = true
		}
	}
	require.True(t, hit, "expected the concurrently unlinked temp file to make os.Chmod fail")

	// The failed attempt must not leave the final file behind.
	_, statErr := os.Stat(filepath.Join(dir, base))
	assert.True(t, os.IsNotExist(statErr), "final file must not exist after a failed write")
}

// TestWriteFileAtomicCloseError covers the tmp.Close() error branch: a
// concurrent goroutine closes the fd backing the temp file behind os.File's
// back (found via /proc/self/fd). With empty data tmp.Write performs no
// syscall, so the invalidated fd only surfaces at tmp.Close() as EBADF.
func TestWriteFileAtomicCloseError(t *testing.T) {
	if _, err := os.Stat("/proc/self/fd"); err != nil {
		t.Skipf("/proc/self/fd unavailable: %v", err)
	}
	dir := t.TempDir()
	base := "state.json"

	var stop int32
	go func() {
		for atomic.LoadInt32(&stop) == 0 {
			entries, err := os.ReadDir("/proc/self/fd")
			if err != nil {
				continue
			}
			for _, e := range entries {
				link, err := os.Readlink(filepath.Join("/proc/self/fd", e.Name()))
				if err != nil {
					continue
				}
				// Only ever touch fds pointing at our temp file.
				if strings.HasPrefix(link, dir+string(os.PathSeparator)) &&
					tmpFilePattern(filepath.Base(link), base) {
					fd, err := strconv.Atoi(e.Name())
					if err != nil {
						continue
					}
					_ = syscall.Close(fd)
				}
			}
		}
	}()
	defer atomic.StoreInt32(&stop, 1)

	hit := false
	deadline := time.Now().Add(15 * time.Second)
	for !hit && time.Now().Before(deadline) {
		// Empty payload: Write never issues a syscall, so the closed fd is
		// reported by Close instead of Write.
		err := WriteFileAtomic(filepath.Join(dir, base), nil, 0o644)
		var pathErr *os.PathError
		if errors.As(err, &pathErr) && pathErr.Op == "close" {
			hit = true
		}
	}
	require.True(t, hit, "expected the invalidated temp fd to make tmp.Close fail")
}
