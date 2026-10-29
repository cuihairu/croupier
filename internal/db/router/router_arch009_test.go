package router_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/db/router"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// archRouterOpts carries overrides for the ARCH-009 test router.
type archRouterOpts struct {
	openBudget time.Duration
	ensure     router.DatabaseEnsurer
	open       router.DatabaseOpener
}

// newArchRouter builds a router backed by real SQLite (for the success paths)
// with an overridable EnsureDatabase / Open / OpenBudget.
func newArchRouter(t *testing.T, opts archRouterOpts) *router.Router {
	t.Helper()
	tmp := t.TempDir()
	metaDBPath := filepath.Join(tmp, "meta.db")
	metaDB := openSQLite(t, metaDBPath)

	ensure := opts.ensure
	if ensure == nil {
		ensure = func(_ context.Context, driver, metaDSN, dbName string) (string, error) {
			return filepath.Join(filepath.Dir(metaDSN), dbName+".db"), nil
		}
	}
	open := opts.open
	if open == nil {
		open = func(driver, dsn string) (*gorm.DB, error) {
			return gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
		}
	}

	r := router.New(router.Config{
		Driver:  "sqlite",
		MetaDSN: metaDBPath,
		DSNForDatabase: func(metaDSN, dbName string) string {
			return filepath.Join(filepath.Dir(metaDSN), dbName+".db")
		},
		EnsureDatabase: ensure,
		Open:           open,
		OpenBudget:     opts.openBudget,
	}, metaDB)
	t.Cleanup(func() { _ = r.Close() })
	t.Cleanup(func() { closeDB(t, metaDB) })
	return r
}

// TestGameDB_CancelledWaiterDoesNotAbortSharedOpen verifies ARCH-009's core
// guarantee: when one caller cancels its wait while a shared first-open is in
// flight, that caller returns promptly with an error, but the shared open (run
// on the lifecycle context) continues and the waiting caller still receives the
// connection. The shared I/O must run exactly once.
func TestGameDB_CancelledWaiterDoesNotAbortSharedOpen(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var ensureCalls atomic.Int32
	var enteredOnce atomic.Bool

	ensure := func(ctx context.Context, driver, metaDSN, dbName string) (string, error) {
		ensureCalls.Add(1)
		if enteredOnce.CompareAndSwap(false, true) {
			close(entered)
		}
		select {
		case <-release:
			return filepath.Join(filepath.Dir(metaDSN), dbName+".db"), nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	r := newArchRouter(t, archRouterOpts{ensure: ensure})

	// Caller A starts the shared open (blocks inside ensure).
	ctxA, cancelA := context.WithCancel(context.Background())
	doneA := make(chan error, 1)
	go func() {
		_, err := r.GameDB(ctxA, "demo", "prod")
		doneA <- err
	}()
	<-entered

	// Caller B joins while the shared open is still in flight.
	doneB := make(chan error, 1)
	go func() {
		_, err := r.GameDB(context.Background(), "demo", "prod")
		doneB <- err
	}()

	// Give B a moment to register as a waiter, then cancel A.
	time.Sleep(50 * time.Millisecond)
	cancelA()

	select {
	case err := <-doneA:
		require.Error(t, err, "cancelled waiter must fail fast")
		require.Contains(t, err.Error(), "cancelled")
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled waiter did not return promptly")
	}

	// The shared open must still be running (A's cancellation must not abort
	// it). Release it; B must then receive the connection.
	close(release)
	select {
	case err := <-doneB:
		require.NoError(t, err, "waiting caller must complete once the shared open finishes")
	case <-time.After(2 * time.Second):
		t.Fatal("waiting caller did not complete after the shared open released")
	}
	require.Equal(t, int32(1), ensureCalls.Load(), "shared first-open must run exactly once")
}

// TestGameDB_PreCancelledContextNoIO verifies that a caller that arrives with
// an already-cancelled context triggers zero first-open I/O.
func TestGameDB_PreCancelledContextNoIO(t *testing.T) {
	var ensureCalls, openCalls atomic.Int32
	ensure := func(context.Context, string, string, string) (string, error) {
		ensureCalls.Add(1)
		return "", nil
	}
	open := func(string, string) (*gorm.DB, error) {
		openCalls.Add(1)
		return nil, nil
	}
	r := newArchRouter(t, archRouterOpts{ensure: ensure, open: open})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.GameDB(ctx, "demo", "prod")
	require.Error(t, err)
	require.Contains(t, err.Error(), "cancelled before start")
	require.Equal(t, int32(0), ensureCalls.Load(), "pre-cancelled caller must not ensure")
	require.Equal(t, int32(0), openCalls.Load(), "pre-cancelled caller must not open")
}

// TestGameDB_OpenBudgetExceeded verifies that when the per-open budget expires,
// the ensure step observes cancellation and the open fails with a budget error,
// recorded as a failed first-open in Stats.
func TestGameDB_OpenBudgetExceeded(t *testing.T) {
	var ensureErr error
	ensure := func(ctx context.Context, driver, metaDSN, dbName string) (string, error) {
		<-ctx.Done() // block until the budget expires
		ensureErr = ctx.Err()
		return "", ctx.Err()
	}
	r := newArchRouter(t, archRouterOpts{ensure: ensure, openBudget: 30 * time.Millisecond})

	_, err := r.GameDB(context.Background(), "demo", "prod")
	require.Error(t, err, "budget expiry must fail the open")
	require.ErrorIs(t, ensureErr, context.DeadlineExceeded, "ensure must observe the budget deadline")

	st := r.Stats()
	require.Equal(t, int64(1), st.OpenFailureCount)
	require.Equal(t, int64(0), st.FirstOpenCount)
	require.Equal(t, 0, st.PoolCount)
}

// TestGameDB_LifecycleCancelAbortsInFlightOpen verifies that Close cancels the
// router lifecycle, which aborts a shared first-open still in flight, and the
// caller receives an error rather than hanging.
func TestGameDB_LifecycleCancelAbortsInFlightOpen(t *testing.T) {
	entered := make(chan struct{})
	var ensureErr error
	ensure := func(ctx context.Context, driver, metaDSN, dbName string) (string, error) {
		close(entered)
		<-ctx.Done() // block until lifecycle cancellation
		ensureErr = ctx.Err()
		return "", ctx.Err()
	}
	r := newArchRouter(t, archRouterOpts{ensure: ensure})

	done := make(chan error, 1)
	go func() {
		_, err := r.GameDB(context.Background(), "demo", "prod")
		done <- err
	}()
	<-entered

	require.NoError(t, r.Close(), "Close must not block on the in-flight open")

	select {
	case err := <-done:
		require.Error(t, err, "in-flight open must abort after Close")
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight open did not abort after lifecycle cancellation")
	}
	require.ErrorIs(t, ensureErr, context.Canceled, "ensure must observe lifecycle cancellation")
}

// TestGameDB_DifferentScopesOpenInParallel verifies that first-opens for two
// different databases proceed concurrently (neither blocks the other) while
// same-db callers are still deduplicated by singleflight.
func TestGameDB_DifferentScopesOpenInParallel(t *testing.T) {
	prodEntered := make(chan struct{})
	stagingEntered := make(chan struct{})
	releaseStaging := make(chan struct{})
	enteredOnce := map[string]*atomic.Bool{
		"game_demo_prod":    {},
		"game_demo_staging": {},
	}
	closeOnce := func(name string, ch chan struct{}) {
		if enteredOnce[name].CompareAndSwap(false, true) {
			close(ch)
		}
	}

	ensure := func(ctx context.Context, driver, metaDSN, dbName string) (string, error) {
		switch dbName {
		case "game_demo_prod":
			closeOnce("game_demo_prod", prodEntered)
			// Prod blocks until staging's open has started, proving the two
			// opens overlap (different dbs are not serialized).
			<-stagingEntered
		case "game_demo_staging":
			closeOnce("game_demo_staging", stagingEntered)
			<-releaseStaging
		}
		return filepath.Join(filepath.Dir(metaDSN), dbName+".db"), nil
	}
	r := newArchRouter(t, archRouterOpts{ensure: ensure})

	// Start prod; it blocks inside ensure waiting for staging to enter.
	doneProd := make(chan error, 1)
	go func() {
		_, err := r.GameDB(context.Background(), "demo", "prod")
		doneProd <- err
	}()
	<-prodEntered

	// Start staging; its ensure starting while prod's is still blocked proves
	// the opens run in parallel.
	doneStaging := make(chan error, 1)
	go func() {
		_, err := r.GameDB(context.Background(), "demo", "staging")
		doneStaging <- err
	}()

	select {
	case <-stagingEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("staging open never started while prod open was in flight")
	}
	close(releaseStaging)

	for _, done := range []chan error{doneProd, doneStaging} {
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("parallel open did not complete")
		}
	}
	require.Equal(t, 2, r.Stats().PoolCount)
}

// TestGameDB_StatsAfterOpensAndFailure verifies Stats() surfaces the pool size,
// completed opens, failures and latency figures over the retained window.
func TestGameDB_StatsAfterOpensAndFailure(t *testing.T) {
	ensure := func(_ context.Context, driver, metaDSN, dbName string) (string, error) {
		if dbName == "game_demo_broken" {
			return "", errors.New("boom")
		}
		return filepath.Join(filepath.Dir(metaDSN), dbName+".db"), nil
	}
	r := newArchRouter(t, archRouterOpts{ensure: ensure})

	_, err := r.GameDB(context.Background(), "demo", "prod")
	require.NoError(t, err)
	_, err = r.GameDB(context.Background(), "demo", "staging")
	require.NoError(t, err)
	_, err = r.GameDB(context.Background(), "demo", "broken")
	require.Error(t, err)

	st := r.Stats()
	require.Equal(t, 2, st.PoolCount)
	require.Equal(t, int64(2), st.FirstOpenCount)
	require.Equal(t, int64(1), st.OpenFailureCount)
	require.Equal(t, 2, st.Window, "only successful opens are sampled")
	require.True(t, st.AvgLatency > 0)
	require.True(t, st.MaxLatency >= st.AvgLatency)
	require.True(t, st.P95Latency > 0)
	require.True(t, st.P95Latency >= st.AvgLatency)
}
