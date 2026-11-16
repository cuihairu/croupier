package session

import (
	"path/filepath"
	"strings"
	"testing"
)

// Close 与会话清理链的残余 goroutine 并发不再有数据竞争（2026-11-17 CI
// race 门禁实证：t.Cleanup 的 rec.Close 置 f=nil 与清理中会话的 EndStats
// 读 f 竞争，TestHubConsumerEvictionOnTimeout 随机红）。f 的全部访问已收进
// r.mu；本用例在 -race 下并发对撞，并锁定关档后的行为契约。
func TestRecorderConcurrentCloseNoRace(t *testing.T) {
	rec, err := OpenRecorder(filepath.Join(t.TempDir(), "sessions"))
	if err != nil {
		t.Fatalf("open recorder: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			_ = rec.EndStats("s-1", "eof", CastStats{
				File:   filepath.Join("casts", "s-1.cast"),
				Events: i,
				Bytes:  int64(i),
			})
			_ = rec.End("s-1", "eof")
		}
	}()
	for i := 0; i < 10; i++ {
		_ = rec.Close()
	}
	<-done

	// 关档后写入统一返回 recorder is closed；Close 幂等。
	if err := rec.EndStats("s-1", "eof", CastStats{}); err == nil ||
		!strings.Contains(err.Error(), "recorder is closed") {
		t.Fatalf("want recorder is closed, got %v", err)
	}
	if err := rec.End("s-1", "eof"); err == nil ||
		!strings.Contains(err.Error(), "recorder is closed") {
		t.Fatalf("want recorder is closed, got %v", err)
	}
	if err := rec.Close(); err != nil {
		t.Fatalf("idempotent close: %v", err)
	}
}
