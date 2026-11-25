// Coverage targets (group E):
//   - the sql.Open failure branch of probe.Probe (probe.go:68-71)
//   - the reload-after-update failure branch of service.UpdateSource
//     (service.go:103-105)
package dbmon

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// A mysql DSN without "/" is rejected already at the OpenConnector stage of
// go-sql-driver (ParseDSN: missing the slash), sql.Open returns an error, and
// the probe result carries the open error instead of the endpoint erroring.
func TestProbe_SQLOpenInvalidDSNGroupE(t *testing.T) {
	src := &model.DBSource{Model: gorm.Model{ID: 1}, Name: "bad-dsn", Driver: "mysql", Kind: model.DBSourceKindSelf}

	res, err := Probe(context.Background(), src, "groupE-no-slash-dsn")

	require.NoError(t, err, "probe failures are reported on the result")
	require.NotNil(t, res)
	assert.False(t, res.OK)
	assert.Contains(t, res.Error, "open:")
	assert.NotZero(t, res.SourceID)
	assert.Equal(t, "mysql", res.Driver)
}

// Query sequence in the UpdateSource flow: FindOne(#1) → Update → FindOne(#2).
// Inject a callback that fails from the 2nd query on, covering the reload
// failure after a successful update.
func TestService_UpdateSource_ReloadErrorGroupE(t *testing.T) {
	dbmonSvc, srcModel, _, db := newDBMonDBFixture(t)
	ctx := context.Background()
	src := &model.DBSource{
		Name: "s", Driver: "mysql", Kind: model.DBSourceKindSelf,
		DSN: goodDSN, GameID: "demo", Env: "prod", Enabled: true,
	}
	require.NoError(t, srcModel.Create(ctx, src))

	var n int
	require.NoError(t, db.Callback().Query().Before("gorm:query").
		Register("groupE_fail_query", func(tx *gorm.DB) {
			n++
			if n >= 2 {
				_ = tx.AddError(errors.New("forced reload failure"))
			}
		}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove("groupE_fail_query") })

	resp, err := dbmonSvc.UpdateSource(ctx, &SourceUpdateRequest{
		ID:   fmt.Sprintf("%d", src.ID),
		Name: "s2",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced reload failure")
	assert.Nil(t, resp)
}

// Coverage target: the success main path of probe.Probe (reachability verdict
// passes → the battery runs → res.OK=true). The driver name is pinned by
// driverName to the registered real drivers (mysql/pgx) and a fake driver
// under the same name cannot be registered, so the openDB injection seam
// substitutes a sqlmock-built *sql.DB (class B).
func TestProbe_OKViaSQLMockGroupS(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Reachability verdict (closed out into DBChecker): Ping + the read-only
	// assertion SELECT 1.
	mock.ExpectPing()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1")).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery("pg_stat_activity").WillReturnRows(
		sqlmock.NewRows([]string{"cur", "active", "maxc"}).AddRow(int64(8), int64(2), int64(100)))
	mock.ExpectQuery("pg_stat_database").WillReturnRows(
		sqlmock.NewRows([]string{"deadlocks"}).AddRow(int64(7)))
	mock.ExpectQuery("pg_locks").WillReturnRows(
		sqlmock.NewRows([]string{"blocked_pid", "blocking_pid", "relname", "wait_secs", "query"}).
			AddRow("101", "202", "public.players", 3.25, "SELECT 1"))

	orig := openDB
	openDB = func(_, _ string) (*sql.DB, error) { return db, nil }
	t.Cleanup(func() { openDB = orig })

	src := &model.DBSource{Model: gorm.Model{ID: 9}, Name: "s", Driver: "postgres", Kind: model.DBSourceKindSelf}
	res, err := Probe(context.Background(), src, "postgres://u:p@127.0.0.1:5432/game")

	require.NoError(t, err, "probe success must not surface as interface error")
	require.NotNil(t, res)
	assert.True(t, res.OK)
	assert.Empty(t, res.Error)
	require.NotNil(t, res.Connections)
	assert.Equal(t, 8, res.Connections.Current)
	require.NoError(t, mock.ExpectationsWereMet())
}
