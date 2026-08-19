package hotreload

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
)

func newTestReloader(t *testing.T, cfg *Config) *croupierHotReloader {
	t.Helper()
	if cfg == nil {
		cfg = DefaultConfig()
	}
	hrIface, err := NewHotReloader(cfg, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	require.NoError(t, err)
	return hrIface.(*croupierHotReloader)
}

func TestShouldHandleFile_Branches(t *testing.T) {
	hr := newTestReloader(t, &Config{
		WatchExts:      []string{".json", ".yaml"},
		IgnorePatterns: []string{"*.tmp", "secret-*"},
	})

	assert.True(t, hr.shouldHandleFile("/configs/app.json"))
	assert.False(t, hr.shouldHandleFile("/configs/app.toml"), "extension not watched")
	assert.False(t, hr.shouldHandleFile("/configs/draft.json.tmp"), "ignore pattern")
	assert.False(t, hr.shouldHandleFile("/configs/secret-a.json"), "ignore prefix pattern")
	assert.True(t, hr.shouldHandleFile("/configs/a.yaml"))
}

func TestReloadFile_ReadError(t *testing.T) {
	hr := newTestReloader(t, nil)
	err := hr.Reload(filepath.Join(t.TempDir(), "missing.json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}

func TestReloadFile_BackupCreatedAndNoHandler(t *testing.T) {
	// backupFile writes to ./backups/hotreload relative to CWD; chdir into a
	// temp dir so the test never pollutes the repository.
	dir := t.TempDir()
	restore := chdir(t, dir)

	cfg := DefaultConfig()
	cfg.BackupEnabled = true
	cfg.DebounceTime = time.Millisecond
	hr := newTestReloader(t, cfg)

	target := filepath.Join(dir, "watch", "app.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
	require.NoError(t, os.WriteFile(target, []byte(`{"v":1}`), 0o644))

	// No handler registered: reload succeeds and only logs a warning.
	require.NoError(t, hr.Reload(target))

	backupDir := filepath.Join(dir, "backups", "hotreload")
	entries, err := os.ReadDir(backupDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	content, err := os.ReadFile(filepath.Join(backupDir, entries[0].Name()))
	require.NoError(t, err)
	assert.JSONEq(t, `{"v":1}`, string(content))
	_ = restore
}

func TestCallHandlers_Branches(t *testing.T) {
	hr := newTestReloader(t, nil)
	ctx := context.Background()

	// Invalid glob pattern is skipped without failing the call.
	require.NoError(t, hr.RegisterHandler("[bad", func(context.Context, ReloadEvent) error {
		return nil
	}))
	require.NoError(t, hr.callHandlers(ctx, ReloadEvent{Path: "/any/file.json"}))

	// Handler failure is returned as the last error.
	hr2 := newTestReloader(t, nil)
	require.NoError(t, hr2.RegisterHandler("*.json", func(context.Context, ReloadEvent) error {
		return assert.AnError
	}))
	err := hr2.callHandlers(ctx, ReloadEvent{Path: "cfg.json"})
	require.ErrorIs(t, err, assert.AnError)

	// Matching handler success.
	hr3 := newTestReloader(t, nil)
	called := false
	require.NoError(t, hr3.RegisterHandler("*.json", func(context.Context, ReloadEvent) error {
		called = true
		return nil
	}))
	require.NoError(t, hr3.callHandlers(ctx, ReloadEvent{Path: "cfg.json"}))
	assert.True(t, called)
}

func TestHandleFileEvent_SkipsUnwatchedAndDebounces(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DebounceTime = 30 * time.Millisecond
	cfg.BackupEnabled = false
	hr := newTestReloader(t, cfg)
	ctx := context.Background()

	// Unwatched extension: no debounce timer is scheduled.
	hr.handleFileEvent(ctx, fsnotify.Event{Name: "/x/file.txt", Op: fsnotify.Write})
	hr.mutex.RLock()
	pending := len(hr.debouncers)
	hr.mutex.RUnlock()
	assert.Equal(t, 0, pending)

	dir := t.TempDir()
	reloadCount := int32(0)
	require.NoError(t, hr.RegisterHandler(filepath.Join(dir, "*.json"), func(context.Context, ReloadEvent) error {
		atomic.AddInt32(&reloadCount, 1)
		return nil
	}))

	target := filepath.Join(dir, "cfg.json")
	require.NoError(t, os.WriteFile(target, []byte(`{}`), 0o644))

	// Two rapid events collapse into a single debounced reload.
	hr.handleFileEvent(ctx, fsnotify.Event{Name: target, Op: fsnotify.Write})
	hr.handleFileEvent(ctx, fsnotify.Event{Name: target, Op: fsresponsiveWrite()})

	assert.Eventually(t, func() bool {
		return atomic.LoadInt32(&reloadCount) == 1
	}, 2*time.Second, 10*time.Millisecond, "debounced reload should fire exactly once")

	hr.Stop()
}

func fsresponsiveWrite() fsnotify.Op { return fsnotify.Write }

func TestWatchLoop_ExitsOnWatcherClose(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DebounceTime = time.Millisecond
	hr := newTestReloader(t, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	require.NoError(t, hr.StartWatching(ctx))
	// Closing the watcher closes the Events channel and the loop returns.
	require.NoError(t, hr.watcher.Close())
	hr.Stop()
}

func TestCheckRemoteUpdates_NotConfigured(t *testing.T) {
	hr := newTestReloader(t, &Config{})
	assert.NoError(t, hr.checkRemoteUpdates(context.Background()))
}

func TestCheckRemoteUpdates_ServerErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("game_id") {
		case "g-500":
			w.WriteHeader(http.StatusInternalServerError)
		case "g-badjson":
			_, _ = w.Write([]byte("not-json"))
		default:
			_ = jsonEncode(w, VersionInfo{Version: "1.0.0"})
		}
	}))
	defer srv.Close()
	ctx := context.Background()

	hrErr := newTestReloader(t, &Config{ServerURL: srv.URL, GameID: "g-500", Environment: "prod"})
	err := hrErr.checkRemoteUpdates(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")

	hrBad := newTestReloader(t, &Config{ServerURL: srv.URL, GameID: "g-badjson", Environment: "prod"})
	err = hrBad.checkRemoteUpdates(ctx)
	require.Error(t, err)

	// Unreachable server.
	hrDown := newTestReloader(t, &Config{ServerURL: "http://127.0.0.1:1", GameID: "g"})
	err = hrDown.checkRemoteUpdates(ctx)
	require.Error(t, err)
}

func TestCheckRemoteUpdates_AppliesNewRemoteVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = jsonEncode(w, VersionInfo{Version: "9.9.9", Files: map[string]string{"a.json": "h1"}})
	}))
	defer srv.Close()

	hr := newTestReloader(t, &Config{ServerURL: srv.URL, GameID: "demo", Environment: "prod"})
	require.NoError(t, hr.checkRemoteUpdates(context.Background()))

	got := hr.GetVersion()
	assert.Equal(t, "9.9.9", got.Version)
	assert.Equal(t, "h1", got.Files["a.json"])

	// Same version: no change, no error.
	require.NoError(t, hr.checkRemoteUpdates(context.Background()))
	assert.Equal(t, "9.9.9", hr.GetVersion().Version)
}

func TestRemoteSyncLoop_TicksAndStops(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_ = jsonEncode(w, VersionInfo{Version: "1.0.0"})
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.PollInterval = 10 * time.Millisecond
	cfg.ServerURL = srv.URL
	cfg.GameID = "demo"
	hr := newTestReloader(t, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { hr.remoteSyncLoop(ctx); close(done) }()

	assert.Eventually(t, func() bool { return atomic.LoadInt32(&hits) >= 2 }, 2*time.Second, 5*time.Millisecond)

	// Both exit paths: context cancellation and stop channel.
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("remoteSyncLoop did not exit on ctx cancel")
	}

	done2 := make(chan struct{})
	go func() { hr.remoteSyncLoop(context.Background()); close(done2) }()
	close(hr.stopChan)
	select {
	case <-done2:
	case <-time.After(2 * time.Second):
		t.Fatal("remoteSyncLoop did not exit on stopChan")
	}
}

func TestGetVersion_DeepCopy(t *testing.T) {
	hr := newTestReloader(t, nil)
	hr.version.Files["app.json"] = "hash-1"

	v1 := hr.GetVersion()
	v2 := hr.GetVersion()
	require.Equal(t, "1.0.0", v1.Version)
	require.Equal(t, "hash-1", v1.Files["app.json"])

	// Mutating the copy must not affect internal state.
	v1.Files["app.json"] = "tampered"
	assert.Equal(t, "hash-1", v2.Files["app.json"])
	assert.Equal(t, "hash-1", hr.GetVersion().Files["app.json"])
}

func TestStartWatching_AlreadyRunning(t *testing.T) {
	hr := newTestReloader(t, nil)
	ctx := context.Background()
	require.NoError(t, hr.StartWatching(ctx))
	err := hr.StartWatching(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
	require.NoError(t, hr.Stop())
	require.NoError(t, hr.Stop(), "Stop must be idempotent")
}

func TestGlobalHelpers_NotInitialized(t *testing.T) {
	err := RegisterGlobalHandler("*.json", func(context.Context, ReloadEvent) error { return nil })
	require.Error(t, err)

	require.Error(t, StartGlobalWatching(context.Background()))

	// StopGlobal with nil global is a no-op.
	require.NoError(t, StopGlobal())
}

func TestGlobalLifecycle(t *testing.T) {
	require.NoError(t, InitGlobal(&Config{WatchDirs: []string{}}, slog.Default()))
	require.NoError(t, RegisterGlobalHandler("*.json", func(context.Context, ReloadEvent) error { return nil }))
	require.NoError(t, StartGlobalWatching(context.Background()))
	require.NoError(t, StopGlobal())
}

func TestGoPluginHandler_LoadFailure(t *testing.T) {
	h := NewGoPluginHandler(slog.Default())

	// A .so file that is not a valid Go plugin must produce a load error.
	dir := t.TempDir()
	fakeSO := filepath.Join(dir, "fake.so")
	require.NoError(t, os.WriteFile(fakeSO, []byte("not an elf object"), 0o644))

	err := h.Handle(context.Background(), ReloadEvent{
		Type: ReloadTypePlugin, Path: fakeSO, Content: []byte("x"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load plugin")
}

func TestHandlerManager_PluginErrorAggregates(t *testing.T) {
	m := NewHandlerManager(slog.Default())
	dir := t.TempDir()
	fakeSO := filepath.Join(dir, "bad.so")
	require.NoError(t, os.WriteFile(fakeSO, []byte("junk"), 0o644))

	err := m.Handle(context.Background(), ReloadEvent{
		Type: ReloadTypePlugin, Path: fakeSO,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reload handler errors")

	// Unknown reload type only warns.
	require.NoError(t, m.Handle(context.Background(), ReloadEvent{Type: ReloadType("weird")}))
}

func TestRegisterGameConfigHandler_MalformedJSON(t *testing.T) {
	hr := newTestReloader(t, nil)
	require.NoError(t, RegisterGameConfigHandler(hr, "/cfg/app.json", func(*GameConfig) error {
		return nil
	}))
	handler := hr.handlers["/cfg/app.json"]
	require.NotNil(t, handler)

	err := handler(context.Background(), ReloadEvent{
		Type: ReloadTypeConfig, Path: "/cfg/app.json", Content: []byte("{bad"),
	})
	require.Error(t, err)
}

func TestGenerateVersion_Format(t *testing.T) {
	hr := newTestReloader(t, nil)
	v := hr.generateVersion("/x/a.json", make([]byte, 7))
	assert.Regexp(t, `^\d{14}_7$`, v)
}

// chdir switches the test working directory and registers cleanup.
func chdir(t *testing.T, dir string) func() {
	t.Helper()
	old, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	return func() { _ = os.Chdir(old) }
}

func jsonEncode(w http.ResponseWriter, v interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(v)
}
