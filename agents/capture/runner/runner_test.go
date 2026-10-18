package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/config"
	"github.com/cuihairu/croupier/agents/capture/connector"
	"github.com/cuihairu/croupier/agents/capture/source"
)

// fakeConnector 可编程契约源：Open 推 events 后按 closeAfter 控制是否关流。
type fakeConnector struct {
	mu         sync.Mutex
	events     []connector.ChangeEvent
	closeAfter int // 推完多少条后关流；0=保持开
	closed     bool
	ch         chan connector.ChangeEvent
	checkpoint connector.Position
}

func (f *fakeConnector) Open(_ context.Context, _ connector.SourceConfig) error {
	f.ch = make(chan connector.ChangeEvent, len(f.events)+1)
	for _, ev := range f.events {
		f.ch <- ev
	}
	if f.closeAfter > 0 {
		close(f.ch)
	}
	return nil
}

func (f *fakeConnector) Discover(context.Context, connector.DiscoveryRequest) (*connector.DiscoveryResult, error) {
	return nil, errors.New("discover not implemented")
}

func (f *fakeConnector) Snapshot(context.Context, connector.SnapshotRequest) (<-chan connector.ChangeEvent, error) {
	return nil, connector.ErrUnsupportedSnapshot
}

func (f *fakeConnector) Poll(_ context.Context, _ connector.PollRequest) (<-chan connector.ChangeEvent, error) {
	return f.ch, nil
}

func (f *fakeConnector) Checkpoint(context.Context) (connector.Position, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.checkpoint.Kind != "" {
		return f.checkpoint, nil
	}
	return connector.Position{Kind: "gtid", Value: "fake-1"}, nil
}

func (f *fakeConnector) AckCheckpoint(_ context.Context, position connector.Position) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if position.Valid() {
		f.checkpoint = position
	}
	return nil
}

func (f *fakeConnector) Close(context.Context) error {
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

func TestBuildPayloadCarriesLineageDegradedLabel(t *testing.T) {
	r := New(testConfig(t, false), nil)
	if req, err := r.buildPayload(context.Background()); err != nil {
		t.Fatalf("buildPayload: %v", err)
	} else if _, ok := req.Labels["captureLineage"]; ok {
		t.Fatalf("labels = %v, want no lineage label without lineage gate", req.Labels)
	}

	// 面板下发血缘规则热生效后，下次注册载荷即带降级标注。
	lineageJSON := `{"lineage":{"rules":[{"name":"gw","table":"game.items","allowAccounts":["app"]}]}}`
	if err := r.applyGates(context.Background(), []byte(lineageJSON)); err != nil {
		t.Fatalf("applyGates: %v", err)
	}
	req, err := r.buildPayload(context.Background())
	if err != nil {
		t.Fatalf("buildPayload: %v", err)
	}
	if req.Labels["captureLineage"] != "degraded" {
		t.Fatalf("labels = %v, want captureLineage=degraded", req.Labels)
	}
}

func TestRunDisabledOnlyRegisters(t *testing.T) {
	cfg := testConfig(t, false)
	called := 0
	r := New(cfg, nil)
	r.newConnector = func() connector.SourceConnector {
		called++
		return &fakeConnector{}
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
	fp := &fakeConnector{events: []connector.ChangeEvent{
		{SourceType: "mysql", Database: "game", Table: "items", Operation: connector.OpInsert},
		{SourceType: "mysql", Database: "game", Table: "items", Operation: connector.OpUpdate},
		{SourceType: "mysql", Database: "game", Table: "items", Operation: connector.OpDelete},
		{SourceType: "mysql", Database: "game", Table: "items", Operation: "ddl"},
	}}
	r := New(cfg, nil)
	r.newConnector = func() connector.SourceConnector { return fp }
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
		t.Fatal("connector should be closed on shutdown")
	}
}

func TestRunReopensAfterStreamEnd(t *testing.T) {
	cfg := testConfig(t, true)
	attempts := 0
	var lastConnector *fakeConnector
	r := New(cfg, nil)
	r.newConnector = func() connector.SourceConnector {
		attempts++
		c := &fakeConnector{closeAfter: 1}
		lastConnector = c
		return c
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
	if lastConnector == nil || !lastConnector.closed {
		t.Fatal("last connector should be closed before reopen")
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
	r.newConnector = func() connector.SourceConnector {
		t.Fatal("source must not open with corrupt bookmark")
		return &fakeConnector{}
	}
	if err := r.Run(context.Background()); err == nil {
		t.Fatal("expected bookmark load failure to abort run")
	}
}

func TestConsumeCountsAndWhitelistDoubleGuard(t *testing.T) {
	r := New(testConfig(t, true), nil)
	r.consume(connector.ChangeEvent{Operation: connector.OpInsert})
	r.consume(connector.ChangeEvent{Operation: connector.OpUpdate})
	r.consume(connector.ChangeEvent{Operation: connector.OpUpdate})
	r.consume(connector.ChangeEvent{Operation: connector.OpDelete})
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
	r.consume(connector.ChangeEvent{Operation: connector.OpInsert})
	s1 := r.Stats()
	s1.Total = 999
	if r.Stats().Total != 1 {
		t.Fatal("Stats must return a copy")
	}
}

// loadPosition 位点形态兼容：kind:value 契约形态直用；legacy 裸值串（kind
// 不在闭集）按源类型回落——旧位点文件平滑续读，不强制迁移。
func TestLoadPositionLegacyBookmarkFallback(t *testing.T) {
	r := New(testConfig(t, true), nil)
	pos, err := r.loadPosition()
	if err != nil || pos.Valid() {
		t.Fatalf("empty bookmark = %+v, %v", pos, err)
	}
	if err := r.store.Save(source.Bookmark{Position: "gtid:0-1-42"}); err != nil {
		t.Fatalf("save contract form: %v", err)
	}
	if pos, err = r.loadPosition(); err != nil || pos.Kind != "gtid" || pos.Value != "0-1-42" {
		t.Fatalf("contract position = %+v, %v", pos, err)
	}
	// legacy 裸 GTID（UUID 含冒号但 kind 不在闭集）：回落 gtid 续读
	if err := r.store.Save(source.Bookmark{Position: "0-1-42:5-12"}); err != nil {
		t.Fatalf("save legacy form: %v", err)
	}
	if pos, err = r.loadPosition(); err != nil || pos.Kind != "gtid" || pos.Value != "0-1-42:5-12" {
		t.Fatalf("legacy fallback position = %+v, %v", pos, err)
	}
}
