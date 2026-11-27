package router

// AUD-077 regression tests: Close and the cache publish must share a
// lifecycle check. The audit's SQLite probe reproduced the defect — a
// first-open whose last migration step was suspended until after Close
// returned still published its pool into the drained cache, leaving the pool
// count back at 1 with nobody owning the connection. These tests pin the
// fixed behavior: the late pool is closed at the rejected publish, the caller
// receives ErrClosed, and the pool count stays zero.

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// cacheLen reads the cache size under the router mutex (package-internal
// assertion helper).
func cacheLen(r *Router) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.cache)
}

// requirePoolClosed asserts the underlying pool of a gorm.DB is really closed
// (not leaked): pings must fail and no connection may remain open.
func requirePoolClosed(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NotNil(t, db, "db handle must not be nil")
	sqlDB, err := db.DB()
	require.NoError(t, err, "late pool must still expose its *sql.DB")
	pingErr := sqlDB.PingContext(context.Background())
	require.Error(t, pingErr, "late pool must be closed, not leaked")
	require.Contains(t, pingErr.Error(), "closed", "ping on a closed pool must report the closed state, got %v", pingErr)
	require.Equal(t, 0, sqlDB.Stats().OpenConnections, "late pool must hold zero open connections")
}

// TestGameDB_AfterCloseReturnsClosedError: after Close, GameDB must not serve
// a cached pool (the cache was drained) nor open a new one — it returns a
// clear closed error and the pool count stays zero.
func TestGameDB_AfterCloseReturnsClosedError(t *testing.T) {
	r, metaDB := newTestRouter(t, Config{})
	defer func() { _ = openSQLiteClose(metaDB) }()

	db, err := r.GameDB(context.Background(), "demo", "prod")
	require.NoError(t, err)
	require.NotNil(t, db)
	require.Equal(t, 1, cacheLen(r), "open scope must be cached before Close")

	require.NoError(t, r.Close())
	require.Equal(t, 0, cacheLen(r), "Close must drain the cache")
	require.Equal(t, 0, r.Stats().PoolCount, "pool count must be zero after Close")

	_, err = r.GameDB(context.Background(), "demo", "prod")
	require.Error(t, err, "GameDB after Close must fail")
	require.ErrorIs(t, err, ErrClosed, "GameDB after Close must report the closed error")

	// Resolve shares the same lifecycle gate via GameDB.
	_, _, err = r.Resolve(context.Background(), "demo", "prod")
	require.ErrorIs(t, err, ErrClosed, "Resolve after Close must report the closed error")

	// A scope never opened before Close must also be refused without any I/O.
	_, err = r.GameDB(context.Background(), "fresh", "prod")
	require.ErrorIs(t, err, ErrClosed)
	require.Equal(t, 0, cacheLen(r), "refused open must not repopulate the cache")
	require.Empty(t, r.OpenScopes(), "closed router must report no open scopes")
}

// TestClose_SuspendedMigrationLatePublishRejected is the audit's SQLite probe
// (AUD-077): the last migration step is suspended, Close completes while it is
// still parked, the migration then returns SUCCESS — and the late pool must be
// rejected at publish (closed, not leaked), the caller must receive the closed
// error, and the pool count must stay zero.
func TestClose_SuspendedMigrationLatePublishRejected(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var migrateErr error
	var lateDB *gorm.DB

	r, metaDB := newTestRouter(t, Config{
		MigrateGame: func(_ context.Context, db *gorm.DB) error {
			lateDB = db
			close(entered) // suspend the last migration step
			<-release      // parked until after Close has completed
			return nil     // migration itself succeeds after close
		},
	})
	defer func() { _ = openSQLiteClose(metaDB) }()

	done := make(chan error, 1)
	go func() {
		_, err := r.GameDB(context.Background(), "demo", "prod")
		done <- err
	}()
	<-entered

	// Close completes while the migration is still suspended.
	require.NoError(t, r.Close(), "Close must not wait for the suspended open")
	require.Equal(t, 0, cacheLen(r), "cache must be drained before the migration resumes")

	// Let the migration return success; the late pool must not be published.
	close(release)
	select {
	case err := <-done:
		require.Error(t, err, "a caller whose open raced Close must get an error, not a late pool")
		require.ErrorIs(t, err, ErrClosed, "the racing caller must receive the closed error")
	case <-time.After(2 * time.Second):
		t.Fatal("GameDB did not return after the suspended migration was released")
	}

	require.NoError(t, migrateErr, "migration must have returned success (nil) after close")
	require.Equal(t, 0, cacheLen(r), "late pool must not be published into the closed router")
	require.Equal(t, 0, r.Stats().PoolCount, "pool count must stay zero (audit SQLite probe)")
	require.Empty(t, r.OpenScopes())
	requirePoolClosed(t, lateDB)
}

// TestClose_ConcurrentOpensRaceLeavesNoLatePools hammers open-vs-Close: N
// goroutines open distinct scopes while Close drains concurrently. After both
// sides settle the cache must hold zero pools and every handle ever handed out
// must be closed — a late publish either lost the race (closed at the
// rejection point) or won it (closed by the drain), never leaked.
func TestClose_ConcurrentOpensRaceLeavesNoLatePools(t *testing.T) {
	r, metaDB := newTestRouter(t, Config{})
	defer func() { _ = openSQLiteClose(metaDB) }()

	const workers = 24
	type outcome struct {
		db  *gorm.DB
		err error
	}
	results := make([]outcome, workers)

	var wg sync.WaitGroup
	wg.Add(workers + 1)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			db, err := r.GameDB(context.Background(), fmt.Sprintf("hammer%d", idx), "prod")
			results[idx] = outcome{db: db, err: err}
		}(i)
	}
	go func() {
		defer wg.Done()
		_ = r.Close() // races with the opens; must still leave zero pools
	}()
	wg.Wait()

	require.Equal(t, 0, cacheLen(r), "no pool may survive Close")
	require.Equal(t, 0, r.Stats().PoolCount, "pool count must be zero after the open/close race")
	require.Empty(t, r.OpenScopes(), "closed router must report no open scopes")

	for i, res := range results {
		switch {
		case res.err != nil:
			// Accept the two legitimate failure shapes: the closed gate or the
			// lifecycle-cancel abort of a first-open in flight.
			if !errors.Is(res.err, ErrClosed) && !errors.Is(res.err, context.Canceled) {
				t.Fatalf("worker %d: unexpected error %v", i, res.err)
			}
			require.Nil(t, res.db, "worker %d: failed call must not hand out a pool", i)
		case res.db != nil:
			// The open won the race before the drain: Close must have closed
			// this pool with the rest of the cache.
			requirePoolClosed(t, res.db)
		default:
			t.Fatalf("worker %d: neither db nor error", i)
		}
	}
}
