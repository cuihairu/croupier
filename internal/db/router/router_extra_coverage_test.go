package router

// router.go supplementary tests: the Stats bubble-sort swap arm (inject
// out-of-order durations), the recordFirstOpen ring overwrite arm (window full
// at 1024), the miss-continue arm of Forget, the ensure/open phase
// cancellation arms of openGameDB (cancel the context up front and cancel the
// parent context inside EnsureDatabase), and the ping failure arm (close the
// underlying sql.DB so PingContext errors).
//
// Honestly skipped: the P95 index clamp in Stats (338 — the 313 empty guard
// ensures len>=1, and int(0.95*len) < len always holds, so the clamp is
// unreachable); the migrate-phase cancellation arm of openGameDB (500) — the
// only deterministic cancellation source is budget expiry, and if the budget
// expires during ping, PingContext notices the ctx cancellation first and
// reports a ping error (495); the cancellation window between ping success and
// check("migrate") is a zero-width race that cannot be injected
// deterministically.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRouterStatsBubbleSortSwapArm(t *testing.T) {
	r := &Router{}
	// Out-of-order durations: the bubble sort must swap to hit the 332 swap arm.
	r.openDurations = []time.Duration{3 * time.Second, 1 * time.Second, 2 * time.Second}
	s := r.Stats()
	require.Equal(t, 3, s.Window)
	require.Equal(t, 2*time.Second, s.AvgLatency)
	require.Equal(t, 3*time.Second, s.MaxLatency)
	require.Equal(t, 3*time.Second, s.P95Latency)
}

func TestRouterRecordFirstOpenRingOverwrite(t *testing.T) {
	r := &Router{}
	r.openDurations = make([]time.Duration, maxOpenStatsWindow)
	r.recordFirstOpen(7 * time.Second)
	require.Len(t, r.openDurations, maxOpenStatsWindow)
	// After the ring rotation the newest sample lands at the last slot (copy overwrite arm hit).
	require.Equal(t, 7*time.Second, r.openDurations[maxOpenStatsWindow-1])
	require.Equal(t, time.Duration(0), r.openDurations[0])
}

func TestRouterForgetNonMatchingContinue(t *testing.T) {
	r := &Router{
		cache:    map[string]*gorm.DB{"g1_prod": nil},
		gameOfDB: map[string]scopeKey{"g1_prod": {gameID: "g1", env: "prod"}},
	}
	// Target scope differs from the cache entry → the loop continues (410); no match returns nil.
	require.NoError(t, r.Forget("g2", "prod"))
}

func TestOpenGameDBCancelledAtEnsure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (&Router{}).openGameDB(ctx, "gamedb")
	require.ErrorContains(t, err, "cancelled")
	require.ErrorContains(t, err, "ensure")
}

func TestOpenGameDBCancelledAtOpen(t *testing.T) {
	parentCtx, cancel := context.WithCancel(context.Background())
	r := &Router{}
	r.cfg = Config{
		Driver: "sqlite",
		DSNForDatabase: func(metaDSN, dbName string) string {
			return dbName
		},
		// Cancel the parent context inside the ensure phase → the derived budget
		// child context is cancelled with it → check("open") notices the
		// cancellation (479).
		EnsureDatabase: func(ctx context.Context, driver, metaDSN, dbName string) (string, error) {
			cancel()
			return "dsn", nil
		},
	}
	_, err := r.openGameDB(parentCtx, "gamedb")
	require.ErrorContains(t, err, "cancelled")
	require.ErrorContains(t, err, "open")
}

func TestOpenGameDBPingError(t *testing.T) {
	closed := openSQLiteDB(t, "file:router_ping?mode=memory&cache=shared")
	sqlDB, err := closed.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	r := &Router{}
	r.cfg = Config{
		Driver: "sqlite",
		DSNForDatabase: func(metaDSN, dbName string) string {
			return dbName
		},
		Open: func(driver, dsn string) (*gorm.DB, error) {
			return closed, nil
		},
	}
	_, err = r.openGameDB(context.Background(), "gamedb")
	require.ErrorContains(t, err, "ping database")
}
