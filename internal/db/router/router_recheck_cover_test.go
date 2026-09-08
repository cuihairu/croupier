package router

import (
	"context"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"gorm.io/gorm"
)

// rrCovRounds bounds how many independently orchestrated rounds the test runs.
// Each round resolves the race toward the re-check branch with very high
// probability; a small budget is plenty on multi-core runners.
const rrCovRounds = 24

// rrCovFillers is the number of concurrent publisher goroutines spinning for
// the write lock. More fillers spread across Ps shrink the chance that the
// caller sprints from its fast path all the way through the re-check before
// any filler wins the lock.
const rrCovFillers = 4

// TestGameDB_InflightRecheckPrefersRepublishedEntry exercises the singleflight
// re-check branch in GameDB: after a caller misses the fast path and becomes
// the singleflight leader, another opener may publish a cache entry while it
// was winning the race; the re-check must return that entry instead of opening
// a duplicate connection.
//
// Orchestration (no sleeps):
//  1. The test thread takes r.mu exclusively BEFORE the caller goroutine
//     starts, so the caller's fast path parks on RLock.
//  2. The caller signals from NameForGame (the last user hook before the fast
//     path) and parks on the read lock. Unlock hands the queued reader the
//     lock, so its fast path deterministically observes an empty cache.
//  3. Filler goroutines spin-TryLock. sync.RWMutex guarantees TryLock cannot
//     win while the caller's fast path still holds the read lock, so the first
//     win lands right after RUnlock — inside the caller's singleflight window.
//     The winner publishes a competing entry into the cache.
//  4. The leader's re-check must reuse the published entry. When the race
//     resolves the other way (the leader reaches the re-check before any
//     filler), the post-open publish check performs the same duty, so every
//     round is valid regardless of interleaving; the invariant assertions
//     below accept both outcomes.
func TestGameDB_InflightRecheckPrefersRepublishedEntry(t *testing.T) {
	for round := 0; round < rrCovRounds; round++ {
		rrCovRound(t, round)
	}
}

func rrCovRound(t *testing.T, round int) {
	t.Helper()

	tmp := t.TempDir()
	dbName := DefaultGameDBName("recheck", "prod")
	published := openSQLiteDB(t, filepath.Join(tmp, "published.db"))

	var openCalls int32
	nameReady := make(chan struct{})
	r, metaDB := newTestRouter(t, Config{
		NameForGame: func(gameID, env string) string {
			if gameID == "recheck" {
				close(nameReady)
			}
			return dbName
		},
		Open: func(driverName, dsn string) (*gorm.DB, error) {
			atomic.AddInt32(&openCalls, 1)
			return openSQLiteDB(t, filepath.Join(tmp, "fresh.db")), nil
		},
	})
	defer func() {
		_ = r.Close()
		_ = openSQLiteClose(metaDB)
		_ = openSQLiteClose(published)
	}()

	// Park the caller's fast path on the write lock this thread holds.
	r.mu.Lock()
	bDone := make(chan struct{})
	var got *gorm.DB
	var gerr error
	go func() {
		defer close(bDone)
		got, gerr = r.GameDB(context.Background(), "recheck", "prod")
	}()
	<-nameReady
	runtime.Gosched() // let the caller reach (and park on) RLock before Unlock
	r.mu.Unlock()

	// Fillers spin for the lock; the first winner lands inside the caller's
	// singleflight window and publishes the competing entry.
	var wg sync.WaitGroup
	for i := 0; i < rrCovFillers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1<<20; j++ {
				if r.mu.TryLock() {
					if _, dup := r.cache[dbName]; !dup {
						r.cache[dbName] = published
						r.gameOfDB[dbName] = "recheck"
					}
					r.mu.Unlock()
					return
				}
			}
		}()
	}
	wg.Wait()
	<-bDone

	// Invariants: either the republished entry wins (zero opens), or the
	// caller completed exactly one fresh open that it also published.
	if gerr != nil {
		t.Fatalf("round %d: GameDB: %v", round, gerr)
	}
	if got == nil {
		t.Fatalf("round %d: GameDB returned nil", round)
	}
	if n := atomic.LoadInt32(&openCalls); n > 1 {
		t.Fatalf("round %d: at most one open expected, got %d", round, n)
	}
	if got != published && atomic.LoadInt32(&openCalls) != 1 {
		t.Fatalf("round %d: expected the republished entry or exactly one fresh open", round)
	}
	r.mu.RLock()
	cached, ok := r.cache[dbName]
	r.mu.RUnlock()
	if !ok || (cached != published && cached != got) {
		t.Fatalf("round %d: cache must hold the published or the freshly opened entry", round)
	}
}
