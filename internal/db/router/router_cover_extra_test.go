package router

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// ---- a driver with a failing Close: makes *sql.DB.Close() return an error ----

type badCloseDriver struct{}

type badCloseConn struct{}

var (
	registerOnce sync.Once
	errBadClose  = errors.New("close boom")
)

func (badCloseConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unimplemented") }
func (badCloseConn) Close() error                        { return errBadClose }
func (badCloseConn) Begin() (driver.Tx, error)           { return nil, errors.New("unimplemented") }
func (badCloseConn) Ping() error                         { return nil }

func (badCloseDriver) Open(string) (driver.Conn, error) { return badCloseConn{}, nil }

type badCloseDialector struct {
	sqlDB *sql.DB
}

func (badCloseDialector) Name() string { return "badclose" }

func (d badCloseDialector) Initialize(db *gorm.DB) error {
	db.ConnPool = d.sqlDB
	return nil
}

func (badCloseDialector) Migrator(*gorm.DB) gorm.Migrator                       { return nil }
func (badCloseDialector) DataTypeOf(*schema.Field) string                       { return "" }
func (badCloseDialector) DefaultValueOf(*schema.Field) clause.Expression        { return clause.Expr{} }
func (badCloseDialector) BindVarTo(clause.Writer, *gorm.Statement, interface{}) {}
func (badCloseDialector) QuoteTo(clause.Writer, string)                         {}
func (badCloseDialector) Explain(sql string, vars ...interface{}) string        { return "" }

// openBadCloseDB returns a gorm.DB whose underlying connection errors on Close.
func openBadCloseDB(t *testing.T) *gorm.DB {
	t.Helper()
	registerOnce.Do(func() { sql.Register("badclose-driver", badCloseDriver{}) })
	sqlDB, err := sql.Open("badclose-driver", "unused-dsn")
	if err != nil {
		t.Fatalf("open badclose: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	// Establish a real connection so it enters the idle pool, triggering the driver error on Close.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		t.Fatalf("ping badclose: %v", err)
	}
	db, err := gorm.Open(badCloseDialector{sqlDB: sqlDB}, &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm open badclose: %v", err)
	}
	return db
}

func TestClose_PropagatesConnCloseError(t *testing.T) {
	r, metaDB := newTestRouter(t, Config{})
	defer func() { _ = openSQLiteClose(metaDB) }()

	bad := openBadCloseDB(t)
	r.cache["game_bad_prod"] = bad
	r.gameOfDB["game_bad_prod"] = scopeKey{gameID: "bad", env: "prod"}

	err := r.Close()
	if err == nil {
		t.Fatal("Close must surface the first connection close error")
	}
	if !contains(err.Error(), "close boom") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.cache) != 0 || len(r.gameOfDB) != 0 {
		t.Fatal("Close must clear the cache even on error")
	}
}

func TestForgetGame_PropagatesConnCloseError(t *testing.T) {
	r, metaDB := newTestRouter(t, Config{})
	defer func() { _ = openSQLiteClose(metaDB) }()

	bad := openBadCloseDB(t)
	r.cache["game_bad_prod"] = bad
	r.gameOfDB["game_bad_prod"] = scopeKey{gameID: "bad", env: "prod"}

	err := r.ForgetGame("bad")
	if err == nil {
		t.Fatal("ForgetGame must surface the connection close error")
	}
	if !contains(err.Error(), "close boom") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := r.cache["game_bad_prod"]; ok {
		t.Fatal("ForgetGame must drop matching entries even on error")
	}
}

// On the GameDB slow path the cache has already been populated by a concurrent
// opener at publish time: the existing connection must be reused preferentially
// and the freshly opened one closed, to avoid a leak.
func TestGameDB_PublishRacePrefersExistingEntry(t *testing.T) {
	r, metaDB := newTestRouter(t, Config{})
	defer func() { _ = openSQLiteClose(metaDB) }()

	tmp := t.TempDir()
	dbName := DefaultGameDBName("race", "prod")
	existing := openSQLiteDB(t, filepath.Join(tmp, "existing.db"))

	r.cfg.Open = func(driverName, dsn string) (*gorm.DB, error) {
		// Simulate another opener publishing a cache entry during this open.
		r.mu.Lock()
		r.cache[dbName] = existing
		r.mu.Unlock()
		return openSQLiteDB(t, filepath.Join(tmp, "opened.db")), nil
	}

	db, err := r.GameDB(context.Background(), "race", "prod")
	if err != nil {
		t.Fatalf("GameDB: %v", err)
	}
	if db != existing {
		t.Fatal("GameDB must return the already-published connection")
	}
	if r.cache[dbName] != existing {
		t.Fatal("cache must keep the existing entry")
	}
}
