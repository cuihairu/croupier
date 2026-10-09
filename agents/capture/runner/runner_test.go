package runner

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/config"
	"github.com/cuihairu/croupier/agents/capture/source"
)

// fakeProvider 可编程事件源：Open 推 events 后按 closeAfter 控制是否关流。
type fakeProvider struct {
	mu         sync.Mutex
	events     []source.ChangeEvent
	closeAfter int // 推完多少条后关流；0=保持开
	bookmarked source.Bookmark
	closed     bool
	ch         chan source.ChangeEvent
}

func (f *fakeProvider) Open(_ context.Context, bm source.Bookmark) (<-chan source.ChangeEvent, error) {
	f.ch = make(chan source.ChangeEvent, len(f.events)+1)
	for _, ev := range f.events {
		f.ch <- ev
	}
	go func() {
		if f.closeAfter > 0 {
			close(f.ch)
		}
	}()
	if bm.Valid() {
		f.bookmarked = bm
	}
	return f.ch, nil
}

func (f *fakeProvider) Bookmark() source.Bookmark {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.bookmarked.Position == "" {
		f.bookmarked = source.Bookmark{Position: "gtid:fake-1", UpdatedAtUnixMs: time.Now().UnixMilli()}
	}
	return f.bookmarked
}

func (f *fakeProvider) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func testConfig(t *testing.T, enabled bool) config.Config {
	t.Helper()
	dir := t.TempDir()
	return config.Config{
		Agent: config.Agent{ID: "capture-test", ServerAddr: "127.0.0.1:1", GameID: "demo", Env: "prod"},
		Capture: config.Capture{
			Enabled:     enabled,
			BookmarkDir: dir,
			MySQL:       &source.MySQLConfig{Host: "127.0.0.1", Port: 1, User: "u", ServerID: 7, Tables: map[string]bool{"game.items": true}},
		},
	}
}

func TestBuildPayloadCarriesScopeLabels(t *testing.T) {
	r := New(testConfig(t, false), nil)
	req, err := r.buildPayload(context.Background())
	if err != nil {
		t.Fatalf("buildPayload: %v", err)
	}
	if req.AgentId != "capture-test" || req.GameId != "demo" || req.Env != "prod" {
		t.Fatalf("payload = %+v", req)
	}
	if req.Labels["role"] != "capture" || req.Labels["hostname"] == "" {
		t.Fatalf("labels = %v", req.Labels)
	}
}

func TestRunDisabledOnlyRegisters(t *testing.T) {
	cfg := testConfig(t, false)
	called := 0
	r := New(cfg, nil)
	r.newSource = func() source.Provider {
		called++
		return &fakeProvider{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	if err := r.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if called != 0 {
		t.Fatal("source must not open when capture disabled")
	}
}

func TestRunConsumesEventsAndFlushesBookmark(t *testing.T) {
	cfg := testConfig(t, true)
	fp := &fakeProvider{events: []source.ChangeEvent{
		{SourceType: "mysql", Database: "game", Table: "items", Op: "insert"},
		{SourceType: "mysql", Database: "game", Table: "items", Op: "update"},
		{SourceType: "mysql", Database: "game", Table: "items", Op: "delete"},
		{SourceType: "mysql", Database: "game", Table: "items", Op: "ddl"},
	}}
	r := New(cfg, nil)
	r.newSource = func() source.Provider { return fp }
	r.backoffInitial = time.Millisecond
	r.backoffMax = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond) // 留出事件消费+周期刷位点
		cancel()
	}()
	if err := r.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	st := r.Stats()
	if st.Total != 4 || st.Insert != 1 || st.Update != 1 || st.Delete != 1 || st.Skipped != 1 {
		t.Fatalf("stats = %+v", st)
	}
	raw, err := os.ReadFile(filepath.Join(cfg.Capture.BookmarkDir, "mysql.json"))
	if err != nil {
		t.Fatalf("bookmark file: %v", err)
	}
	bm, err := source.NewStore(filepath.Join(cfg.Capture.BookmarkDir, "mysql.json")).Load()
	if err != nil {
		t.Fatalf("bookmark load: %v", err)
	}
	if bm.Position != "gtid:fake-1" {
		t.Fatalf("bookmark = %q (raw=%s)", bm.Position, raw)
	}
	if !fp.closed {
		t.Fatal("provider should be closed on shutdown")
	}
}

func TestRunReopensAfterStreamEnd(t *testing.T) {
	cfg := testConfig(t, true)
	attempts := 0
	var lastProvider *fakeProvider
	r := New(cfg, nil)
	r.newSource = func() source.Provider {
		attempts++
		p := &fakeProvider{closeAfter: 1}
		lastProvider = p
		return p
	}
	r.backoffInitial = time.Millisecond
	r.backoffMax = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	if err := r.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want >=2 after stream end", attempts)
	}
	if lastProvider == nil || !lastProvider.closed {
		t.Fatal("last provider should be closed before reopen")
	}
}

func TestRunAbortsOnCorruptBookmark(t *testing.T) {
	cfg := testConfig(t, true)
	if err := os.MkdirAll(cfg.Capture.BookmarkDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	bad := filepath.Join(cfg.Capture.BookmarkDir, "mysql.json")
	if err := os.WriteFile(bad, []byte("{corrupt"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	r := New(cfg, nil)
	r.newSource = func() source.Provider {
		t.Fatal("source must not open with corrupt bookmark")
		return &fakeProvider{}
	}
	if err := r.Run(context.Background()); err == nil {
		t.Fatal("expected bookmark load failure to abort run")
	}
}

func TestConsumeCountsAndWhitelistDoubleGuard(t *testing.T) {
	r := New(testConfig(t, true), nil)
	r.consume(source.ChangeEvent{Op: "insert"})
	r.consume(source.ChangeEvent{Op: "update"})
	r.consume(source.ChangeEvent{Op: "update"})
	r.consume(source.ChangeEvent{Op: "delete"})
	st := r.Stats()
	if st.Total != 4 || st.Insert != 1 || st.Update != 2 || st.Delete != 1 {
		t.Fatalf("stats = %+v", st)
	}
	if st.LastEventUnixMs == 0 {
		t.Fatal("LastEventUnixMs should be set")
	}
}

func TestStatsSnapshotIsCopy(t *testing.T) {
	r := New(testConfig(t, true), nil)
	r.consume(source.ChangeEvent{Op: "insert"})
	s1 := r.Stats()
	s1.Total = 999
	if r.Stats().Total != 1 {
		t.Fatal("Stats must return a copy")
	}
}
