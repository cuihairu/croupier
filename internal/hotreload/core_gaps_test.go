package hotreload

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syncedBuffer is a concurrency-safe slog destination so tests can assert on
// log output produced by background goroutines.
type syncedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncedBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncedBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// pkgDir is captured at process start: earlier tests may leave (and remove)
// the process working directory, so os.Getwd is unreliable later on.
var pkgDir, _ = os.Getwd()

// chdirAbsolute switches CWD without relying on os.Getwd and always returns
// to the package directory afterwards.
func chdirAbsolute(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(pkgDir) })
}

// --- NewHotReloader / InitGlobal: fsnotify.NewWatcher failure ---

// countOpenFDs reports how many file descriptors the process currently holds.
func countOpenFDs(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	require.NoError(t, err)
	return len(entries)
}

func TestNewHotReloaderWatcherCreationFailure(t *testing.T) {
	if _, err := os.Stat("/proc/self/fd"); err != nil {
		t.Skipf("/proc/self/fd unavailable: %v", err)
	}
	cfg := &Config{WatchDirs: []string{}}
	logger := discardLogger()

	// Force netpoll initialization while the fd limit is still high so the
	// runtime never needs another fd while the limit is lowered.
	pr, pw, err := os.Pipe()
	require.NoError(t, err)
	defer pr.Close()
	defer pw.Close()

	var orig syscall.Rlimit
	require.NoError(t, syscall.Getrlimit(syscall.RLIMIT_NOFILE, &orig))
	low := orig
	low.Cur = uint64(countOpenFDs(t) - 1)
	restore := func() { _ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &orig) }
	defer restore()

	require.NoError(t, syscall.Setrlimit(syscall.RLIMIT_NOFILE, &low))
	_, err = NewHotReloader(cfg, logger)
	restore()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create watcher")

	// The same failure surfaces through the global convenience initializer.
	require.NoError(t, syscall.Setrlimit(syscall.RLIMIT_NOFILE, &low))
	err = InitGlobal(cfg, logger)
	restore()
	require.Error(t, err)

	// With the limit restored, watcher creation works again.
	hr, err := NewHotReloader(cfg, logger)
	require.NoError(t, err)
	require.NoError(t, hr.Stop())
}

// --- addWatchDir: walk error on a missing root and non-dir entries ---

func TestAddWatchDirWalkError(t *testing.T) {
	hr := newTestReloader(t, &Config{WatchDirs: []string{}})
	err := hr.addWatchDir(filepath.Join(t.TempDir(), "does-not-exist"))
	require.Error(t, err)

	// Walking a tree that also contains a regular file must succeed: the
	// callback returns nil for non-directory entries.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "note.txt"), []byte("x"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	require.NoError(t, hr.addWatchDir(dir))
}

// --- watchLoop: Events channel closed ---

func TestWatchLoopExitsOnClosedEventsChannel(t *testing.T) {
	// fsnotify closes Errors before Events; by starting the loop only after
	// both channels are closed, the select commits to either closed-channel
	// case with equal probability. Repeating makes the Events case certain.
	for i := 0; i < 40; i++ {
		hr := newTestReloader(t, &Config{WatchDirs: []string{}})
		require.NoError(t, hr.watcher.Close())
		time.Sleep(20 * time.Millisecond)

		done := make(chan struct{})
		go func() { hr.watchLoop(context.Background()); close(done) }()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("iteration %d: watchLoop did not exit on closed channels", i)
		}
		require.NoError(t, hr.Stop())
	}
}

// --- handleFileEvent: debounced reload failure is logged ---

func TestHandleFileEventDebouncedReloadFailureLogged(t *testing.T) {
	sb := &syncedBuffer{}
	cfg := DefaultConfig()
	cfg.DebounceTime = 20 * time.Millisecond
	cfg.BackupEnabled = false
	hr, err := NewHotReloader(cfg, slog.New(slog.NewTextHandler(sb, nil)))
	require.NoError(t, err)
	hrImpl := hr.(*croupierHotReloader)

	hrImpl.handleFileEvent(context.Background(), fsnotify.Event{
		Name: filepath.Join(t.TempDir(), "missing.json"),
		Op:   fsnotify.Write,
	})

	assert.Eventually(t, func() bool {
		return strings.Contains(sb.String(), "Failed to reload file")
	}, 3*time.Second, 10*time.Millisecond, "debounced reload of a missing file should be logged")
}

// --- reloadFile/backupFile: backup failures ---

func TestReloadFileBackupFailureLogged(t *testing.T) {
	dir := t.TempDir()
	chdirAbsolute(t, dir)

	// A regular file named "backups" makes MkdirAll("./backups/hotreload")
	// fail with ENOTDIR.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "backups"), []byte("x"), 0o644))

	sb := &syncedBuffer{}
	cfg := DefaultConfig()
	cfg.BackupEnabled = true
	hr, err := NewHotReloader(cfg, slog.New(slog.NewTextHandler(sb, nil)))
	require.NoError(t, err)
	hrImpl := hr.(*croupierHotReloader)

	target := filepath.Join(dir, "app.json")
	require.NoError(t, os.WriteFile(target, []byte(`{}`), 0o644))

	// Backup failure is logged, not propagated.
	require.NoError(t, hrImpl.Reload(target))
	assert.Contains(t, sb.String(), "Failed to backup file")

	// The MkdirAll failure is reported by backupFile itself.
	require.Error(t, hrImpl.backupFile(target))
}

func TestBackupFileReadFailure(t *testing.T) {
	dir := t.TempDir()
	chdirAbsolute(t, dir)

	hr := newTestReloader(t, &Config{WatchDirs: []string{}})
	err := hr.backupFile(filepath.Join(t.TempDir(), "missing.json"))
	require.Error(t, err)
}

// --- remoteSyncLoop: checkRemoteUpdates failures are logged ---

func TestRemoteSyncLoopLogsCheckErrors(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	sb := &syncedBuffer{}
	cfg := DefaultConfig()
	cfg.PollInterval = 10 * time.Millisecond
	cfg.ServerURL = srv.URL
	cfg.GameID = "demo"
	cfg.Environment = "prod"
	hr, err := NewHotReloader(cfg, slog.New(slog.NewTextHandler(sb, nil)))
	require.NoError(t, err)
	hrImpl := hr.(*croupierHotReloader)

	done := make(chan struct{})
	go func() { hrImpl.remoteSyncLoop(context.Background()); close(done) }()

	assert.Eventually(t, func() bool {
		return atomic.LoadInt32(&hits) >= 3 &&
			strings.Contains(sb.String(), "Remote sync failed")
	}, 3*time.Second, 10*time.Millisecond, "failed remote checks should be logged")

	close(hrImpl.stopChan)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("remoteSyncLoop did not exit on closed stopChan")
	}
}

// --- GoPluginHandler: successful plugin load and existing-slot branch ---

// buildTestPlugin compiles a minimal, dependency-free Go plugin with
// -buildmode=plugin using the same toolchain as the test binary.
func buildTestPlugin(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("plugin buildmode requires linux")
	}
	dir := t.TempDir()
	plugdir := filepath.Join(dir, "src")
	require.NoError(t, os.MkdirAll(plugdir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(plugdir, "go.mod"),
		[]byte("module example.com/hotreloadtestplugin\n\ngo 1.26\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(plugdir, "main.go"),
		[]byte("package main\n"), 0o644))

	out := filepath.Join(dir, "plug.so")
	cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", out, ".")
	cmd.Dir = plugdir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	buildOut, err := cmd.CombinedOutput()
	require.NoError(t, err, "plugin build failed: %s", buildOut)
	return out
}

func TestGoPluginHandlerHandleLoadsPlugin(t *testing.T) {
	so := buildTestPlugin(t)
	h := NewGoPluginHandler(discardLogger())

	// Pre-populate the plugin slot to exercise the "old plugin reference"
	// branch; Go plugins cannot be unloaded, so a nil reference is fine.
	h.plugins[so] = nil

	require.NoError(t, h.Handle(context.Background(), ReloadEvent{
		Type: ReloadTypePlugin,
		Path: so,
	}))

	loaded, ok := h.GetPlugin(so)
	require.True(t, ok, "plugin should be registered after a successful load")
	require.NotNil(t, loaded)
}

// --- HandlerManager: config/script error aggregation ---

func TestHandlerManagerAggregatesConfigAndScriptErrors(t *testing.T) {
	m := NewHandlerManager(discardLogger())
	ctx := context.Background()

	err := m.Handle(ctx, ReloadEvent{
		Type:    ReloadTypeConfig,
		Path:    "bad.json",
		Content: []byte("{bad"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reload handler errors")

	// No interpreter registered for ".lua": the script handler fails and the
	// error is aggregated the same way.
	err = m.Handle(ctx, ReloadEvent{
		Type:    ReloadTypeScript,
		Path:    "a.lua",
		Content: []byte("x"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reload handler errors")
}

// --- RegisterGameConfigHandler: YAML parse failure ---

func TestRegisterGameConfigHandlerInvalidYAML(t *testing.T) {
	fake := newTestHotReloader(t)
	called := false
	require.NoError(t, RegisterGameConfigHandler(fake, "/game.yaml", func(*GameConfig) error {
		called = true
		return nil
	}))
	handler := fake.handlers["/game.yaml"][0]

	err := handler(context.Background(), ReloadEvent{
		Type:    ReloadTypeConfig,
		Path:    "/game.yaml",
		Content: []byte("{bad"),
	})
	require.Error(t, err)
	assert.False(t, called, "onChange must not run for malformed YAML")
}
