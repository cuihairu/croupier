package migrate

// AUD-098: the session-lock unlock must run under its own bounded cleanup
// budget (independent of the possibly cancelled/expired migration context),
// its failure must be surfaced and joined with the original migration error,
// and when the release cannot be confirmed the physical connection must be
// discarded instead of being returned to the idle pool with a session-scoped
// lock possibly still held.

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newLockSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *sql.Conn) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	conn, err := sqlDB.Conn(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return sqlDB, mock, conn
}

// (d) Happy path unchanged: a confirmed unlock hands the physical connection
// back to the pool exactly like the pre-AUD-098 release did.
func TestReleaseSessionLock_MySQLSuccessReturnsConnToPool(t *testing.T) {
	sqlDB, mock, conn := newLockSQLMock(t)

	mock.ExpectExec("SELECT RELEASE_LOCK").
		WithArgs(mysqlMigrationLockName).
		WillReturnResult(sqlmock.NewResult(0, 0))

	require.NoError(t, releaseSessionLock(conn, "mysql", time.Second))
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, 1, sqlDB.Stats().OpenConnections)
	assert.Equal(t, 1, sqlDB.Stats().Idle)
}

// (b) Unlock delayed beyond the cleanup budget: cleanup stops at the budget,
// the failure is reported, and the physical connection is discarded rather
// than reused.
func TestReleaseSessionLock_MySQLBudgetExceededDiscardsConn(t *testing.T) {
	sqlDB, mock, conn := newLockSQLMock(t)

	// The server-side release stalls far beyond the cleanup budget; sqlmock
	// honours ctx.Done() during WillDelayFor and returns ErrCancelled when
	// the deadline fires.
	mock.ExpectExec("SELECT RELEASE_LOCK").
		WithArgs(mysqlMigrationLockName).
		WillDelayFor(3 * time.Second).
		WillReturnResult(sqlmock.NewResult(0, 0))

	start := time.Now()
	err := releaseSessionLock(conn, "mysql", 100*time.Millisecond)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.ErrorIs(t, err, sqlmock.ErrCancelled)
	assert.Contains(t, err.Error(), "release session lock")
	// Cleanup stopped at its own budget instead of waiting out the stall.
	assert.Less(t, elapsed, 2*time.Second, "cleanup ran past its budget: %s", elapsed)
	// Release unconfirmed -> the physical conn must not be back in the pool.
	// Zero open and zero idle connections is the direct "discarded, not
	// reused" evidence: a pooled conn would still count as open. (No re-dial
	// assertion is possible against go-sqlmock: its driver deletes the DSN
	// registry entry once every physical conn is closed, so a fresh dial after
	// a full discard errors out.)
	assert.Equal(t, 0, sqlDB.Stats().OpenConnections)
	assert.Equal(t, 0, sqlDB.Stats().Idle)
}

// (c) Unlock returns a non-connection-level (SQL-level) error: it is surfaced
// to the caller. database/sql cannot distinguish it from a connection-level
// failure, so the physical conn is conservatively discarded as well (documented
// boundary in releaseSessionLock).
func TestReleaseSessionLock_MySQLErrorSurfacedAndConnDiscarded(t *testing.T) {
	sqlDB, mock, conn := newLockSQLMock(t)

	unlockBoom := errors.New("lock is not owned by this session")
	mock.ExpectExec("SELECT RELEASE_LOCK").
		WithArgs(mysqlMigrationLockName).
		WillReturnError(unlockBoom)

	err := releaseSessionLock(conn, "mysql", time.Second)
	require.Error(t, err)
	assert.ErrorIs(t, err, unlockBoom)
	assert.Contains(t, err.Error(), "release session lock")
	assert.Equal(t, 0, sqlDB.Stats().OpenConnections)
}

// (c) Same handling on a second dialect: the unlock error is surfaced and the
// physical connection is discarded.
func TestReleaseSessionLock_PostgresErrorSurfacedAndConnDiscarded(t *testing.T) {
	sqlDB, mock, conn := newLockSQLMock(t)

	unlockBoom := errors.New("advisory unlock boom")
	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_unlock(1668444021, 1885955442)")).
		WillReturnError(unlockBoom)

	err := releaseSessionLock(conn, "postgres", time.Second)
	require.Error(t, err)
	assert.ErrorIs(t, err, unlockBoom)
	assert.Contains(t, err.Error(), "release session lock")
	assert.Equal(t, 0, sqlDB.Stats().OpenConnections)
}

// (a) Migration cancelled mid-flight: the unlock is still attempted under its
// own (fresh, uncancelled) budget and the original migration error is
// preserved verbatim.
func TestEnsureUpToDate_CancelledMigrationStillUnlocks(t *testing.T) {
	db, mock := newMySQLTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)

	mock.ExpectQuery("SELECT GET_LOCK").
		WithArgs(mysqlMigrationLockName).
		WillReturnRows(sqlmock.NewRows([]string{"l"}).AddRow(int64(1)))
	// No version table -> the baseline bridge runs and fails.
	mock.ExpectQuery("information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
	// The unlock runs after the migration ctx was cancelled; its 50ms delay
	// only completes because the cleanup context is fresh, not derived from
	// the cancelled migration context (a derived context would fail instantly
	// with ErrCancelled and be joined into the result).
	mock.ExpectExec("SELECT RELEASE_LOCK").
		WithArgs(mysqlMigrationLockName).
		WillDelayFor(50 * time.Millisecond).
		WillReturnResult(sqlmock.NewResult(0, 0))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	migrationBoom := errors.New("baseline boom")
	baseline := func(*gorm.DB) error {
		cancel() // cancellation lands mid-migration, before cleanup
		return migrationBoom
	}

	_, err = ensureUpToDate(ctx, db, mapFS(map[string]string{
		"0001_a.sql": "-- +goose Up\nSELECT 1;\n\n-- +goose Down\nSELECT 1;\n",
	}), ScopeMeta, baseline)

	// Original migration error preserved; no cleanup error replaces it.
	require.Error(t, err)
	assert.ErrorIs(t, err, migrationBoom)
	assert.NotContains(t, err.Error(), "release session lock")
	// Unlock was still attempted (and confirmed) despite the cancelled ctx.
	assert.NoError(t, mock.ExpectationsWereMet())
	// Confirmed release -> connection back in the pool. Two open conns: the
	// version probe dialed its own pooled conn, the lock conn was returned by
	// the release — none discarded.
	assert.Equal(t, 2, sqlDB.Stats().OpenConnections)
}

// (c) Integration join: when both the migration and the unlock fail, the
// returned error carries both — the original migration error is joined, not
// replaced — and the physical connection is discarded.
func TestEnsureUpToDate_UnlockFailureJoinedWithMigrationError(t *testing.T) {
	db, mock := newMySQLTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)

	probeBoom := errors.New("probe boom")
	unlockBoom := errors.New("unlock boom")

	mock.ExpectQuery("SELECT GET_LOCK").
		WithArgs(mysqlMigrationLockName).
		WillReturnRows(sqlmock.NewRows([]string{"l"}).AddRow(int64(1)))
	mock.ExpectQuery("information_schema.tables").
		WillReturnError(probeBoom)
	mock.ExpectExec("SELECT RELEASE_LOCK").
		WithArgs(mysqlMigrationLockName).
		WillReturnError(unlockBoom)

	_, err = ensureUpToDate(context.Background(), db, nil, ScopeMeta, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, probeBoom)
	assert.ErrorIs(t, err, unlockBoom)
	assert.Contains(t, err.Error(), "release session lock")
	// Release unconfirmed -> the lock conn is discarded, not pooled. One open
	// conn remains: the version probe's own pooled conn (the pass dialed two
	// physical conns; the lock conn went 2 -> 1 via discard).
	assert.Equal(t, 1, sqlDB.Stats().OpenConnections)
	assert.NoError(t, mock.ExpectationsWereMet())
}
