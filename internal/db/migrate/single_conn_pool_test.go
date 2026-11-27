package migrate

// AUD-091: a session lock holds one pooled connection for the whole migration
// pass while the version probe, baseline and goose catch-up borrow more from
// the same pool. With MaxOpenConns(1) that deadlocks until the caller's
// deadline. These tests use a real *sql.DB pool (SetMaxOpenConns(1)) backed by
// a small MySQL-like driver surrogate that actually tracks session locks per
// connection, so the pool exhaustion and the cross-handle lock mutual
// exclusion are exercised by database/sql itself, not stubbed in the test.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var aud091DriverSeq atomic.Uint64

type aud091LockState struct {
	mu             sync.Mutex
	owner          int // connection id holding the session lock; 0 = free
	nextConn       int
	queries        []string
	probeWhileHeld bool // a version probe ran on a conn other than the lock owner
}

func (s *aud091LockState) snapshot() (owner int, queries []string, probeWhileHeld bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.owner, append([]string(nil), s.queries...), s.probeWhileHeld
}

func (s *aud091LockState) countQuery(fragment string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, q := range s.queries {
		if strings.Contains(q, fragment) {
			n++
		}
	}
	return n
}

type aud091Driver struct{ state *aud091LockState }

type aud091Conn struct {
	state *aud091LockState
	id    int
}

func (d *aud091Driver) Open(string) (driver.Conn, error) {
	d.state.mu.Lock()
	d.state.nextConn++
	id := d.state.nextConn
	d.state.mu.Unlock()
	return &aud091Conn{state: d.state, id: id}, nil
}

func (c *aud091Conn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("aud091: prepared statements unsupported")
}
func (c *aud091Conn) Close() error { return nil }
func (c *aud091Conn) Begin() (driver.Tx, error) {
	return nil, errors.New("aud091: transactions unsupported")
}

func (c *aud091Conn) record(query string) {
	c.state.mu.Lock()
	c.state.queries = append(c.state.queries, query)
	c.state.mu.Unlock()
}

// queryRows handles the read side of the migration pass. GET_LOCK is
// session-scoped: it is granted only when no other connection owns it.
func (c *aud091Conn) queryRows(query string) (*aud091Rows, error) {
	c.record(query)
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	switch {
	case strings.Contains(query, "GET_LOCK"):
		if c.state.owner != 0 {
			return &aud091Rows{cols: []string{"l"}, rows: [][]driver.Value{{int64(0)}}}, nil
		}
		c.state.owner = c.id
		return &aud091Rows{cols: []string{"l"}, rows: [][]driver.Value{{int64(1)}}}, nil
	case strings.Contains(query, "RELEASE_LOCK"):
		if c.state.owner == c.id {
			c.state.owner = 0
			return &aud091Rows{cols: []string{"r"}, rows: [][]driver.Value{{int64(1)}}}, nil
		}
		return &aud091Rows{cols: []string{"r"}, rows: [][]driver.Value{{int64(0)}}}, nil
	case strings.Contains(query, "information_schema.tables"):
		if c.state.owner != 0 && c.state.owner != c.id {
			c.state.probeWhileHeld = true
		}
		// Both the plain COUNT probe and goose's EXISTS probe accept 1.
		return &aud091Rows{cols: []string{"c"}, rows: [][]driver.Value{{int64(1)}}}, nil
	case strings.Contains(query, "goose_db_version ORDER BY id DESC"):
		rows := make([][]driver.Value, 0, 70)
		for v := int64(1); v <= 70; v++ {
			rows = append(rows, []driver.Value{v, true})
		}
		return &aud091Rows{cols: []string{"version_id", "is_applied"}, rows: rows}, nil
	case strings.Contains(query, "COALESCE(MAX(version_id)"):
		return &aud091Rows{cols: []string{"m"}, rows: [][]driver.Value{{int64(70)}}}, nil
	default:
		return nil, fmt.Errorf("aud091: unexpected query %q", query)
	}
}

func (c *aud091Conn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	return c.queryRows(query)
}

func (c *aud091Conn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	// AUD-098's release path uses ExecContext(SELECT RELEASE_LOCK(?)),
	// so route it through the same lock-state transition.
	if strings.Contains(query, "RELEASE_LOCK") {
		if len(args) == 0 {
			return nil, errors.New("aud091: RELEASE_LOCK missing argument")
		}
		rows, err := c.queryRows(query)
		if err != nil {
			return nil, err
		}
		defer func() { _ = rows.Close() }()
		return driver.RowsAffected(1), nil
	}
	return nil, fmt.Errorf("aud091: unexpected exec %q", query)
}

type aud091Rows struct {
	cols []string
	rows [][]driver.Value
	idx  int
}

func (r *aud091Rows) Columns() []string { return r.cols }
func (r *aud091Rows) Close() error      { return nil }
func (r *aud091Rows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.idx])
	r.idx++
	return nil
}

// newAud091DB opens a fresh surrogate MySQL with its own lock namespace and a
// real database/sql connection pool capped at maxOpen (unlimited if <= 0).
func newAud091DB(t *testing.T, maxOpen int) (*aud091LockState, *sql.DB) {
	t.Helper()
	state := &aud091LockState{}
	name := fmt.Sprintf("aud091_mysql_%d", aud091DriverSeq.Add(1))
	sql.Register(name, &aud091Driver{state: state})
	sqlDB, err := sql.Open(name, "")
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(maxOpen)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return state, sqlDB
}

func aud091GormDB(t *testing.T, sqlDB *sql.DB) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	return db
}

// (1) The misconfigured pool is rejected before any connection is checked
// out or GET_LOCK is attempted — no waiting for the caller's deadline.
func TestAUD091_SingleConnectionPoolRejectedBeforeLock(t *testing.T) {
	state, sqlDB := newAud091DB(t, 1)
	require.Equal(t, 1, sqlDB.Stats().MaxOpenConnections)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	start := time.Now()
	release, err := acquireSessionLock(ctx, sqlDB, "mysql")
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Nil(t, release)
	assert.Contains(t, err.Error(), "MaxOpenConns")
	assert.Contains(t, err.Error(), "at least 2")
	assert.Less(t, elapsed, 500*time.Millisecond, "rejection must precede any deadline wait: %s", elapsed)
	_, queries, _ := state.snapshot()
	assert.Empty(t, queries, "no GET_LOCK should be issued on a 1-connection pool")
	assert.Zero(t, sqlDB.Stats().WaitCount, "no connection-wait should be needed")
}

// (2) End-to-end: a full migration pass on a 1-connection pool fails fast
// with a configuration error; it must never reach the version probe (the old
// behavior blocked there until the caller's deadline with sqlDB.WaitCount>0).
func TestAUD091_EnsureUpToDateSingleConnectionPoolFailsFast(t *testing.T) {
	state, sqlDB := newAud091DB(t, 1)
	db := aud091GormDB(t, sqlDB)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	start := time.Now()
	_, err := ensureUpToDate(ctx, db, probeFSThrough(70), ScopeMeta, nil)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection pool")
	assert.Contains(t, err.Error(), "MaxOpenConns")
	assert.NotContains(t, err.Error(), "probe version table")
	assert.Less(t, elapsed, 500*time.Millisecond, "must fail before waiting out the caller deadline: %s", elapsed)
	_, queries, _ := state.snapshot()
	assert.Empty(t, queries, "neither GET_LOCK nor the version probe may be issued")
	assert.Zero(t, sqlDB.Stats().WaitCount)
}

// (3) Normal pool sizes keep the exact lock/probe/migrate/release sequence:
// the lock conn stays checked out while a DIFFERENT conn runs the version
// probe, and the session lock is released at the end.
func TestAUD091_NormalPoolStillProbesUnderHeldLock(t *testing.T) {
	state, sqlDB := newAud091DB(t, 2)
	db := aud091GormDB(t, sqlDB)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	version, err := ensureUpToDate(ctx, db, probeFSThrough(70), ScopeMeta, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(70), version)

	owner, queries, probeWhileHeld := state.snapshot()
	assert.Zero(t, owner, "session lock released after migration")
	assert.True(t, probeWhileHeld, "version probe must run on a second conn while the lock is held")
	assert.Equal(t, 1, state.countQuery("GET_LOCK"))
	assert.Equal(t, 1, state.countQuery("RELEASE_LOCK"))
	assert.GreaterOrEqual(t, len(queries), 3)
	assert.Equal(t, 2, sqlDB.Stats().MaxOpenConnections)
}

// (4) Cross-process mutual exclusion is unchanged: two independent *sql.DB
// handles (two "processes" sharing the same server-side lock namespace) still
// cannot both hold the lock, and a denied waiter succeeds after release.
func TestAUD091_SessionLockMutualExclusionAcrossDBHandles(t *testing.T) {
	state := &aud091LockState{}
	newDB := func() *sql.DB {
		name := fmt.Sprintf("aud091_mysql_%d", aud091DriverSeq.Add(1))
		sql.Register(name, &aud091Driver{state: state})
		sqlDB, err := sql.Open(name, "")
		require.NoError(t, err)
		sqlDB.SetMaxOpenConns(2)
		t.Cleanup(func() { _ = sqlDB.Close() })
		return sqlDB
	}
	first, second := newDB(), newDB()

	releaseFirst, err := acquireSessionLock(context.Background(), first, "mysql")
	require.NoError(t, err)
	ownerBefore, _, _ := state.snapshot()
	assert.NotZero(t, ownerBefore)

	waitCtx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	_, err = acquireSessionLock(waitCtx, second, "mysql")
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	ownerDuring, _, _ := state.snapshot()
	assert.Equal(t, ownerBefore, ownerDuring, "denied process must not take over the lock")
	assert.Equal(t, 0, state.countQuery("RELEASE_LOCK"), "denied process must not unlock someone else's session")

	require.NoError(t, releaseFirst())
	ownerAfter, _, _ := state.snapshot()
	assert.Zero(t, ownerAfter)

	releaseSecond, err := acquireSessionLock(context.Background(), second, "mysql")
	require.NoError(t, err)
	require.NoError(t, releaseSecond())
	assert.GreaterOrEqual(t, state.countQuery("GET_LOCK"), 3, "waiter retried while the lock was held")
}
