package healthprobe

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPChecker(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	assert.NoError(t, NewHTTPChecker(srv.URL, time.Second).Check(context.Background()))

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	assert.Error(t, NewHTTPChecker(bad.URL, time.Second).Check(context.Background()))

	assert.Error(t, NewHTTPChecker("http://127.0.0.1:1/nope", 200*time.Millisecond).Check(context.Background()))
}

func TestTCPChecker(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	assert.NoError(t, NewTCPChecker(ln.Addr().String(), time.Second).Check(context.Background()))
	assert.Error(t, NewTCPChecker("127.0.0.1:1", 200*time.Millisecond).Check(context.Background()))
}

func TestProcessChecker(t *testing.T) {
	assert.NoError(t, NewProcessChecker(os.Getpid()).Check(context.Background()))

	// 已退出的子进程：Wait 收尸后 signal 0 返回 ESRCH。
	cmd := exec.Command("true")
	require.NoError(t, cmd.Start())
	pid := cmd.Process.Pid
	require.NoError(t, cmd.Wait())
	assert.Error(t, NewProcessChecker(pid).Check(context.Background()))
}

func TestFileTimelineRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "timeline.jsonl")
	tw, err := OpenFileTimeline(path)
	require.NoError(t, err)

	w1 := Window{Target: "db", Semantics: "readiness", StartTS: 100, EndTS: 150, DurationMS: 50, LastReason: "boom", GameID: "demo", Env: "prod"}
	w2 := Window{Target: "db", Semantics: "readiness", StartTS: 300}
	require.NoError(t, tw.AppendWindow(WindowEvent{Kind: EventRecovered, Window: w1}))
	require.NoError(t, tw.AppendWindow(WindowEvent{Kind: EventUnavailable, Window: w2}))
	require.NoError(t, tw.Close())
	require.Error(t, tw.AppendWindow(WindowEvent{})) // 关闭后写失败

	events, err := LoadTimeline(path)
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, EventRecovered, events[0].Kind)
	assert.Equal(t, "boom", events[0].Window.LastReason)
	assert.Equal(t, EventUnavailable, events[1].Kind)
	assert.Equal(t, int64(0), events[1].Window.EndTS)

	// JSON 行形态（机器可解析契约）。
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var line map[string]any
	require.NoError(t, json.Unmarshal(firstLine(t, string(data)), &line))
	assert.Contains(t, line, "kind")
	assert.Contains(t, line, "window")
}

func TestLoadTimelineMissingFile(t *testing.T) {
	_, err := LoadTimeline(filepath.Join(t.TempDir(), "missing.jsonl"))
	assert.Error(t, err)
}

func TestProbeWritesTimeline(t *testing.T) {
	n := 0
	path := filepath.Join(t.TempDir(), "tl.jsonl")
	tw, err := OpenFileTimeline(path)
	require.NoError(t, err)
	defer func() { _ = tw.Close() }()

	p := New("db", SemanticsReadiness, CheckerFunc(func(context.Context) error {
		n++
		if n <= 2 {
			return assert.AnError
		}
		return nil
	}), WithThresholds(2, 1), WithTimeline(tw))
	for range 3 {
		p.ProbeOnce(context.Background())
	}
	events, err := LoadTimeline(path)
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, EventUnavailable, events[0].Kind)
	assert.Equal(t, EventRecovered, events[1].Kind)
}

func firstLine(t *testing.T, s string) []byte {
	t.Helper()
	for i, r := range s {
		if r == '\n' {
			return []byte(s[:i])
		}
	}
	t.Fatal("no newline in timeline output")
	return nil
}
