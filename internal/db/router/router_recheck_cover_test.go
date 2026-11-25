package router

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"

	"gorm.io/gorm"
)

// TestGameDB_InflightRecheckPrefersRepublishedEntry exercises the singleflight
// re-check branch in GameDB: after a caller misses the fast path and becomes
// the singleflight leader, another opener may publish a cache entry while it
// was winning the race; the re-check must return that entry instead of opening
// a duplicate connection.
//
// The product code itself has no suspension point inside this interleaving window
// (the stretch between the fast-path RUnlock and the re-check RLock is synchronous),
// so the real concurrency cannot construct it; historically a 24-round phase-aligned
// orchestration of "park a caller on the main thread holding the write lock + filler
// spinning on TryLock" triggered it only probabilistically, and under full-suite
// load entire rounds missed (the root cause of coverage flapping). Following the
// "timing branches get an injection point" approach, gameDBInflightHook now
// synchronously injects the racing entry at the callback entry, covering the branch
// deterministically in a single round.
func TestGameDB_InflightRecheckPrefersRepublishedEntry(t *testing.T) {
	tmp := t.TempDir()
	dbName := DefaultGameDBName("recheck", "prod")
	published := openSQLiteDB(t, filepath.Join(tmp, "published.db"))

	r, metaDB := newTestRouter(t, Config{})
	defer func() {
		_ = r.Close()
		_ = openSQLiteClose(metaDB)
	}()

	var openCalls int32
	r.cfg.Open = func(driverName, dsn string) (*gorm.DB, error) {
		atomic.AddInt32(&openCalls, 1)
		return openSQLiteDB(t, filepath.Join(tmp, "fresh.db")), nil
	}

	// Synchronously publish the racing entry at the callback entry: the re-check must reuse it,
	// and the leader must not open again.
	gameDBInflightHook = func() {
		r.mu.Lock()
		if _, dup := r.cache[dbName]; !dup {
			r.cache[dbName] = published
			r.gameOfDB[dbName] = scopeKey{gameID: "recheck", env: "prod"}
		}
		r.mu.Unlock()
	}
	t.Cleanup(func() { gameDBInflightHook = nil })

	got, err := r.GameDB(context.Background(), "recheck", "prod")
	if err != nil {
		t.Fatalf("GameDB: %v", err)
	}
	if got != published {
		t.Fatal("re-check 必须返回已发布的连接，而不是新开的")
	}
	if n := atomic.LoadInt32(&openCalls); n != 0 {
		t.Fatalf("re-check 命中后不应触发任何 open，got %d", n)
	}
	r.mu.RLock()
	cached := r.cache[dbName]
	r.mu.RUnlock()
	if cached != published {
		t.Fatal("cache 必须保留已发布的条目")
	}
}
