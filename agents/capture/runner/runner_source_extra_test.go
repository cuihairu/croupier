package runner

// runner.go 补测：runSourceOnce 生命周期分支（位点损坏 / Open / Poll /
// journal 过滤编译与打开失败 / 流结束收尾 / ctx 取消终刷）、flushBookmark
// 分支、loadPosition legacy 回落、probeTarget 地址解析变体。

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/agents/capture/config"
	"github.com/cuihairu/croupier/agents/capture/connector"
	"github.com/cuihairu/croupier/agents/capture/source"
	"github.com/stretchr/testify/require"
)

// stubSource 可编程错误注入的最小连接器（区别于 runner_test.go 的 fakeConnector：
// 那个只走成功路径）。
type stubSource struct {
	openErr  error
	pollErr  error
	cpErr    error
	cp       connector.Position
	block    bool                    // Poll 返回永不关闭的通道（模拟常驻流）
	events   []connector.ChangeEvent // 非空时 Poll 交付这批事件后关流
	ackErr   error                   // AckCheckpoint 注入失败
	closeCnt int
}

func (s *stubSource) Open(context.Context, connector.SourceConfig) error { return s.openErr }

func (s *stubSource) Discover(context.Context, connector.DiscoveryRequest) (*connector.DiscoveryResult, error) {
	return nil, errors.New("unsupported")
}

func (s *stubSource) Snapshot(context.Context, connector.SnapshotRequest) (<-chan connector.ChangeEvent, error) {
	return nil, connector.ErrUnsupportedSnapshot
}

func (s *stubSource) Poll(context.Context, connector.PollRequest) (<-chan connector.ChangeEvent, error) {
	if s.pollErr != nil {
		return nil, s.pollErr
	}
	if s.block {
		return make(chan connector.ChangeEvent), nil
	}
	if len(s.events) > 0 {
		ch := make(chan connector.ChangeEvent, len(s.events))
		for _, ev := range s.events {
			ch <- ev
		}
		close(ch)
		return ch, nil
	}
	ch := make(chan connector.ChangeEvent, 1)
	close(ch)
	return ch, nil
}

func (s *stubSource) Checkpoint(context.Context) (connector.Position, error) { return s.cp, s.cpErr }

func (s *stubSource) AckCheckpoint(context.Context, connector.Position) error { return s.ackErr }

func (s *stubSource) Close(context.Context) error { s.closeCnt++; return nil }

func TestRunSourceOnce_BookmarkLoadFailure(t *testing.T) {
	r := New(testConfig(t, false), nil)
	bmPath := filepath.Join(t.TempDir(), "bm.json")
	r.store = source.NewStore(bmPath)
	require.NoError(t, os.WriteFile(bmPath, []byte("{bad"), 0o644))

	err := r.runSourceOnce(context.Background())
	require.ErrorIs(t, err, errBookmarkLoad)
}

func TestRunSourceOnce_OpenAndPollFailures(t *testing.T) {
	ctx := context.Background()

	stub := &stubSource{openErr: errors.New("no dial")}
	r := New(testConfig(t, false), nil)
	r.newConnector = func() connector.SourceConnector { return stub }
	require.ErrorContains(t, r.runSourceOnce(ctx), "open connector")

	stub = &stubSource{pollErr: errors.New("repl gone")}
	r.newConnector = func() connector.SourceConnector { return stub }
	require.ErrorContains(t, r.runSourceOnce(ctx), "poll connector")
	require.Equal(t, 1, stub.closeCnt, "poll failure must release the connector")
}

func TestRunSourceOnce_JournalBranches(t *testing.T) {
	ctx := context.Background()
	stub := &stubSource{}

	// enabled 但 maxBytes 非正 → 过滤编译失败，连接器须释放
	cfg := testConfig(t, false)
	cfg.Capture.Journal = config.Journal{Enabled: true}
	r := New(cfg, nil)
	r.newConnector = func() connector.SourceConnector { return stub }
	require.ErrorContains(t, r.runSourceOnce(ctx), "compile journal filter")
	require.Equal(t, 1, stub.closeCnt)

	// journal 文件打开失败：路径父目录是普通文件
	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))
	cfg2 := testConfig(t, false)
	cfg2.Capture.Journal = config.Journal{
		Enabled: true, MaxBytes: 1024,
		Path: filepath.Join(blocker, "sub", "journal.ndjson"),
	}
	r2 := New(cfg2, nil)
	r2.newConnector = func() connector.SourceConnector { return stub }
	require.ErrorContains(t, r2.runSourceOnce(ctx), "open event journal")
	require.Equal(t, 2, stub.closeCnt)
}

func TestRunSourceOnce_StreamEndedFlushesBookmark(t *testing.T) {
	r := New(testConfig(t, false), nil)
	stub := &stubSource{cp: connector.Position{Kind: "gtid", Value: "g-end"}}
	r.newConnector = func() connector.SourceConnector { return stub }

	err := r.runSourceOnce(context.Background())
	require.ErrorIs(t, err, errStreamEnded)
	bm, loadErr := r.store.Load()
	require.NoError(t, loadErr)
	require.Contains(t, bm.Position, "g-end", "stream end must flush final bookmark")
	require.Equal(t, 1, stub.closeCnt)
}

func TestRunSourceOnce_CtxCancelFlushesFinalBookmark(t *testing.T) {
	r := New(testConfig(t, false), nil)
	stub := &stubSource{block: true, cp: connector.Position{Kind: "gtid", Value: "g-live"}}
	r.newConnector = func() connector.SourceConnector { return stub }

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.NoError(t, r.runSourceOnce(ctx))
	bm, loadErr := r.store.Load()
	require.NoError(t, loadErr)
	require.Contains(t, bm.Position, "g-live", "ctx cancel must flush final bookmark")
	require.Equal(t, 1, stub.closeCnt)
}

func TestFlushBookmark_Branches(t *testing.T) {
	r := New(testConfig(t, false), nil)
	stub := &stubSource{}
	ctx := context.Background()

	// Checkpoint 读取失败：不落盘
	stub.cpErr = errors.New("cp unavailable")
	r.flushBookmark(ctx, stub, true)
	bm, err := r.store.Load()
	require.NoError(t, err)
	require.Empty(t, bm.Position)

	// 零位点：跳过落盘
	stub.cpErr = nil
	r.flushBookmark(ctx, stub, false)
	bm, err = r.store.Load()
	require.NoError(t, err)
	require.Empty(t, bm.Position)

	// 有效位点：kind:value 落盘
	stub.cp = connector.Position{Kind: "gtid", Value: "g-9"}
	r.flushBookmark(ctx, stub, true)
	bm, err = r.store.Load()
	require.NoError(t, err)
	require.Contains(t, bm.Position, "g-9")

	// 落盘失败：store 路径父目录是普通文件（走 warn 分支，不 panic）
	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))
	r.store = source.NewStore(filepath.Join(blocker, "bm.json"))
	r.flushBookmark(ctx, stub, false)
}

func TestLoadPosition_LegacyBareValueAndEmpty(t *testing.T) {
	r := New(testConfig(t, false), nil)

	// legacy 裸值串：kind 按源类型回落（缺省 mysql → gtid）
	require.NoError(t, r.store.Save(source.Bookmark{Position: "g-legacy", UpdatedAtUnixMs: 1}))
	pos, err := r.loadPosition()
	require.NoError(t, err)
	require.Equal(t, "gtid", pos.Kind)
	require.Equal(t, "g-legacy", pos.Value)

	// 空位点 → 零值
	require.NoError(t, r.store.Save(source.Bookmark{}))
	pos, err = r.loadPosition()
	require.NoError(t, err)
	require.False(t, pos.Valid())
}

func TestProbeTarget_Variants(t *testing.T) {
	// legacy capture.mysql 直取 host:port
	r := New(testConfig(t, false), nil)
	addr, ok := r.probeTarget()
	require.True(t, ok)
	require.Equal(t, "127.0.0.1:1", addr)

	cfg := testConfig(t, false)
	cfg.Capture.MySQL = nil
	r = New(cfg, nil)

	// canonical 无 DSNEnv → 降级跳过
	_, ok = r.probeTarget()
	require.False(t, ok)

	cfg.Capture.SourceType = "postgres"
	cfg.Capture.DSNEnv = "CROUPIER_TEST_PROBE_PG"
	t.Setenv("CROUPIER_TEST_PROBE_PG", "postgres://u:p@db.internal:5433/game")
	r = New(cfg, nil)
	addr, ok = r.probeTarget()
	require.True(t, ok)
	require.Equal(t, "db.internal:5433", addr)

	cfg.Capture.SourceType = "mongodb"
	cfg.Capture.DSNEnv = "CROUPIER_TEST_PROBE_MONGO"
	t.Setenv("CROUPIER_TEST_PROBE_MONGO", "mongodb://u:p@db.internal:27018/game")
	r = New(cfg, nil)
	addr, ok = r.probeTarget()
	require.True(t, ok)
	require.Equal(t, "db.internal:27018", addr)

	cfg.Capture.SourceType = "mysql"
	cfg.Capture.DSNEnv = "CROUPIER_TEST_PROBE_MYSQL"
	t.Setenv("CROUPIER_TEST_PROBE_MYSQL", "u:p@tcp(127.0.0.1:3307)/game")
	r = New(cfg, nil)
	addr, ok = r.probeTarget()
	require.True(t, ok)
	require.Equal(t, "127.0.0.1:3307", addr)

	// 环境变量缺失 → 降级
	cfg.Capture.DSNEnv = "CROUPIER_TEST_PROBE_MISSING"
	r = New(cfg, nil)
	_, ok = r.probeTarget()
	require.False(t, ok)
}
