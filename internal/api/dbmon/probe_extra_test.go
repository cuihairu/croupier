// Coverage targets: the success and failure branches of
// probeMySQL/probePostgres in probe.go (a sqlmock-simulated *sql.DB, no real
// MySQL/Postgres), plus the model-layer error paths in service.go/handler.go
// (List/Create/Update/ProbeAll failures, probe fallback for a DSN-less
// source).
package dbmon

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sync/atomic"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ginEngine is just a readability alias; it really is *gin.Engine.
type ginEngine = gin.Engine

// ---- probeMySQL / probePostgres (call the internal probes directly with a
// sqlmock-built *sql.DB) ----

func TestProbeMySQL_SuccessViaSQLMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	statusRows := sqlmock.NewRows([]string{"Variable_name", "Value"}).
		AddRow("Threads_connected", "3").
		AddRow("Threads_running", "1").
		AddRow("Innodb_deadlocks", "2").
		AddRow("Queries", "1234").
		AddRow("Com_commit", "10").
		AddRow("Com_rollback", "5")
	mock.ExpectQuery("SHOW GLOBAL STATUS").WillReturnRows(statusRows)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT @@max_connections")).
		WillReturnRows(sqlmock.NewRows([]string{"max_connections"}).AddRow(int64(55)))
	lockRows := sqlmock.NewRows([]string{"waiter", "blocker", "table_name", "wait_secs", "query"}).
		AddRow("trx-1", "trx-9", "game.players", 12.5, "UPDATE players SET banned = 1")
	mock.ExpectQuery("innodb_lock_waits").WillReturnRows(lockRows)

	res := &ProbeResult{}
	probeMySQL(context.Background(), db, res)

	require.Empty(t, res.Error)
	require.NotNil(t, res.Connections)
	assert.Equal(t, 3, res.Connections.Current)
	assert.Equal(t, 1, res.Connections.Active)
	assert.Equal(t, 55, res.Connections.Max)
	require.NotNil(t, res.DeadlockCount)
	assert.Equal(t, int64(2), *res.DeadlockCount)
	assert.NotEmpty(t, res.DeadlockNote)
	require.NotNil(t, res.QueryCount)
	assert.Equal(t, int64(1234), *res.QueryCount)
	require.NotNil(t, res.TxnCount)
	assert.Equal(t, int64(15), *res.TxnCount)
	require.Len(t, res.LockWaits, 1)
	assert.Equal(t, "trx-1", res.LockWaits[0].WaitPIDorID)
	assert.Equal(t, "trx-9", res.LockWaits[0].BlockedBy)
	assert.Equal(t, "game.players", res.LockWaits[0].Table)
	assert.InDelta(t, 12.5, res.LockWaits[0].WaitSecs, 0.001)
	assert.Contains(t, res.LockWaits[0].Query, "UPDATE players")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProbeMySQL_MinimalStatusViaSQLMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// No deadlock/query-count keys → the corresponding pointers stay nil;
	// a failing max_connections query → Max=-1.
	statusRows := sqlmock.NewRows([]string{"Variable_name", "Value"}).
		AddRow("Threads_connected", "7").
		AddRow("Threads_running", "2")
	mock.ExpectQuery("SHOW GLOBAL STATUS").WillReturnRows(statusRows)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT @@max_connections")).
		WillReturnError(errors.New("denied"))
	mock.ExpectQuery("innodb_lock_waits").
		WillReturnRows(sqlmock.NewRows([]string{"waiter", "blocker", "table_name", "wait_secs", "query"}))

	res := &ProbeResult{}
	probeMySQL(context.Background(), db, res)

	require.Empty(t, res.Error)
	assert.Equal(t, 7, res.Connections.Current)
	assert.Equal(t, 2, res.Connections.Active)
	assert.Equal(t, -1, res.Connections.Max)
	assert.Nil(t, res.DeadlockCount)
	assert.Nil(t, res.QueryCount)
	require.NotNil(t, res.TxnCount)
	assert.Equal(t, int64(0), *res.TxnCount)
	assert.Empty(t, res.LockWaits)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProbeMySQL_StatusQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery("SHOW GLOBAL STATUS").WillReturnError(errors.New("permission denied"))

	res := &ProbeResult{}
	probeMySQL(context.Background(), db, res)

	// The availability verdict belongs to the DBChecker reachability verdict;
	// a failing battery query only degrades the metrics.
	assert.Empty(t, res.Error)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProbeMySQL_LockWaitScanErrorSkipped(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery("SHOW GLOBAL STATUS").
		WillReturnRows(sqlmock.NewRows([]string{"Variable_name", "Value"}))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT @@max_connections")).
		WillReturnRows(sqlmock.NewRows([]string{"max_connections"}))
	// A string in the wait_secs column → Scan into float64 fails → the row is
	// skipped.
	mock.ExpectQuery("innodb_lock_waits").WillReturnRows(
		sqlmock.NewRows([]string{"waiter", "blocker", "table_name", "wait_secs", "query"}).
			AddRow("trx-1", "trx-9", "t", "not-a-float", "q"))

	res := &ProbeResult{}
	probeMySQL(context.Background(), db, res)

	assert.Empty(t, res.Error)
	assert.Empty(t, res.LockWaits)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProbePostgres_SuccessViaSQLMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery("pg_stat_activity").WillReturnRows(
		sqlmock.NewRows([]string{"cur", "active", "maxc"}).AddRow(int64(8), int64(2), int64(100)))
	mock.ExpectQuery("pg_stat_database").WillReturnRows(
		sqlmock.NewRows([]string{"deadlocks"}).AddRow(int64(7)))
	mock.ExpectQuery("pg_locks").WillReturnRows(
		sqlmock.NewRows([]string{"blocked_pid", "blocking_pid", "relname", "wait_secs", "query"}).
			AddRow("101", "202", "public.players", 3.25, "SELECT 1"))

	res := &ProbeResult{}
	probePostgres(context.Background(), db, res)

	require.Empty(t, res.Error)
	require.NotNil(t, res.Connections)
	assert.Equal(t, 8, res.Connections.Current)
	assert.Equal(t, 2, res.Connections.Active)
	assert.Equal(t, 100, res.Connections.Max)
	require.NotNil(t, res.DeadlockCount)
	assert.Equal(t, int64(7), *res.DeadlockCount)
	require.Len(t, res.LockWaits, 1)
	assert.Equal(t, "101", res.LockWaits[0].WaitPIDorID)
	assert.Equal(t, "202", res.LockWaits[0].BlockedBy)
	assert.Equal(t, "public.players", res.LockWaits[0].Table)
	assert.InDelta(t, 3.25, res.LockWaits[0].WaitSecs, 0.001)
	assert.Equal(t, "SELECT 1", res.LockWaits[0].Query)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProbePostgres_StatusQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery("pg_stat_activity").WillReturnError(errors.New("permission denied"))

	res := &ProbeResult{}
	probePostgres(context.Background(), db, res)

	// The availability verdict belongs to the DBChecker reachability verdict;
	// a failing battery query only degrades the metrics.
	assert.Empty(t, res.Error)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProbePostgres_DeadlockAndLockScanFailuresDegrade(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery("pg_stat_activity").WillReturnRows(
		sqlmock.NewRows([]string{"cur", "active", "maxc"}).AddRow(int64(1), int64(0), int64(10)))
	// Deadlock-count and lock-wait queries fail → degraded, no Error set.
	mock.ExpectQuery("pg_stat_database").WillReturnError(errors.New("denied"))
	mock.ExpectQuery("pg_locks").WillReturnError(errors.New("denied"))

	res := &ProbeResult{}
	probePostgres(context.Background(), db, res)

	assert.Empty(t, res.Error)
	assert.Nil(t, res.DeadlockCount)
	assert.Empty(t, res.LockWaits)
	require.NoError(t, mock.ExpectationsWereMet())
}

// ---- Probe reachability verdict (closed out into DBChecker; §6: keep a
// single copy of the probe implementation) ----

// swapOpenDB overrides the openDB test seam, injects a sqlmock connection and
// returns a restore function.
func swapOpenDB(t *testing.T, db *sql.DB) func() {
	t.Helper()
	orig := openDB
	openDB = func(_, _ string) (*sql.DB, error) { return db, nil }
	return func() { openDB = orig }
}

func TestProbe_CheckerVerdictFailureShortCircuits(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	defer swapOpenDB(t, db)()

	// Reachability verdict fails (Ping fails) → Error = the checker reason
	// string, the battery is not executed.
	mock.ExpectPing().WillReturnError(errors.New("dial tcp: refused"))

	src := &model.DBSource{Model: gorm.Model{ID: 1}, Name: "dead-db",
		Driver: "mysql", Kind: model.DBSourceKindSelf}
	res, perr := Probe(context.Background(), src, goodDSN)
	require.NoError(t, perr)
	require.NotNil(t, res)
	assert.False(t, res.OK)
	assert.Contains(t, res.Error, "db ping")
	assert.Nil(t, res.Connections, "连不上时电池不执行")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProbe_CheckerVerdictSuccessRunsBattery(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	defer swapOpenDB(t, db)()

	// Reachability verdict (Ping + the SELECT 1 read-only assertion) passes →
	// the battery runs, OK=true.
	mock.ExpectPing()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1")).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery("SHOW GLOBAL STATUS").WillReturnRows(
		sqlmock.NewRows([]string{"Variable_name", "Value"}).
			AddRow("Threads_connected", "3").AddRow("Threads_running", "1"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT @@max_connections")).
		WillReturnRows(sqlmock.NewRows([]string{"@@max_connections"}).AddRow(int64(100)))
	mock.ExpectQuery("innodb_lock_waits").WillReturnRows(
		sqlmock.NewRows([]string{"waiter", "blocker", "table_name", "wait_secs", "query"}))

	src := &model.DBSource{Model: gorm.Model{ID: 2}, Name: "ok-db",
		Driver: "MySQL", Kind: model.DBSourceKindSelf}
	res, perr := Probe(context.Background(), src, goodDSN)
	require.NoError(t, perr)
	assert.True(t, res.OK)
	assert.Empty(t, res.Error)
	require.NotNil(t, res.Connections)
	assert.Equal(t, 3, res.Connections.Current)
	assert.Equal(t, 100, res.Connections.Max)
	require.NoError(t, mock.ExpectationsWereMet())
}

// ---- service-layer error paths ----

func newDBMonDBFixture(t *testing.T) (*Service, *model.DBSourceModel, *model.AlertModel, *gorm.DB) {
	t.Helper()
	name := fmt.Sprintf("dbmon_extra_%d", atomic.AddInt64(&dbmonTestDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	srcModel := model.NewDBSourceModel(db)
	alertModel := model.NewAlertModel(db)
	return NewService(&svc.ServiceContext{
		DBSourceModel: srcModel,
		AlertModel:    alertModel,
	}), srcModel, alertModel, db
}

func TestDBMonService_ListSources_ModelError(t *testing.T) {
	dbmonSvc, _, _, db := newDBMonDBFixture(t)
	require.NoError(t, db.Migrator().DropTable("db_sources"))

	_, err := dbmonSvc.ListSources(context.Background())
	require.Error(t, err)
}

func TestDBMonService_CreateSource_ModelError(t *testing.T) {
	dbmonSvc, _, _, db := newDBMonDBFixture(t)
	require.NoError(t, db.Migrator().DropTable("db_sources"))

	_, err := dbmonSvc.CreateSource(context.Background(), &SourceUpsertRequest{
		Name: "s", Driver: "mysql", DSN: goodDSN, GameID: "demo", Env: "prod",
	})
	require.Error(t, err)
}

func TestDBMonService_UpdateSource_UpdateError(t *testing.T) {
	dbmonSvc, srcModel, _, db := newDBMonDBFixture(t)
	ctx := context.Background()
	src := &model.DBSource{Name: "s", Driver: "mysql", Kind: model.DBSourceKindSelf, DSN: goodDSN,
		GameID: "demo", Env: "prod", Enabled: true}
	require.NoError(t, srcModel.Create(ctx, src))

	// Drop the updated-target column: FindOne succeeds and Validate passes,
	// but Updates fails with a SQL error.
	require.NoError(t, db.Exec("ALTER TABLE db_sources DROP COLUMN sort").Error)
	sort := 3
	_, err := dbmonSvc.UpdateSource(ctx, &SourceUpdateRequest{
		ID: fmt.Sprintf("%d", src.ID), Name: "s2", Sort: &sort,
	})
	require.Error(t, err)
}

func TestDBMonService_ProbeAll_ListError(t *testing.T) {
	dbmonSvc, _, _, db := newDBMonDBFixture(t)
	require.NoError(t, db.Migrator().DropTable("db_sources"))

	_, err := dbmonSvc.ProbeAll(context.Background())
	require.Error(t, err)
}

func TestDBMonService_ProbeAll_SourceWithoutDSNReportsError(t *testing.T) {
	dbmonSvc, srcModel, _, _ := newDBMonDBFixture(t)
	ctx := context.Background()
	src := &model.DBSource{Name: "no-dsn", Driver: "mysql", Kind: model.DBSourceKindSelf, DSN: goodDSN,
		GameID: "demo", Env: "prod", Enabled: true}
	require.NoError(t, srcModel.Create(ctx, src))
	// Bypass the service validation and blank the DSN, simulating legacy dirty
	// data.
	require.NoError(t, srcModel.Update(ctx, src.ID, map[string]interface{}{"dsn": ""}))

	resp, err := dbmonSvc.ProbeAll(ctx)
	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.False(t, resp.Results[0].OK)
	assert.Contains(t, resp.Results[0].Error, "no DSN")
}

// ---- handler-layer error paths ----

func newDBMonEnvWithDB(t *testing.T) (*ginEngine, *gorm.DB) {
	t.Helper()
	dbmonSvc, _, _, db := newDBMonDBFixture(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(dbmonSvc)
	api := r.Group("/dbmon")
	api.GET("/sources", h.ListSources)
	api.POST("/sources", h.CreateSource)
	api.PUT("/sources/:id", h.UpdateSource)
	api.DELETE("/sources/:id", h.DeleteSource)
	api.POST("/probe", h.ProbeAll)
	return r, db
}

func TestDBMonHandler_ListSources_ServiceError(t *testing.T) {
	r, db := newDBMonEnvWithDB(t)
	require.NoError(t, db.Migrator().DropTable("db_sources"))

	rec := doDBMonReq(r, http.MethodGet, "/dbmon/sources", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestDBMonHandler_CreateSource_ModelError(t *testing.T) {
	r, db := newDBMonEnvWithDB(t)
	require.NoError(t, db.Migrator().DropTable("db_sources"))

	rec := doDBMonReq(r, http.MethodPost, "/dbmon/sources",
		fmt.Sprintf(`{"name":"s","driver":"mysql","dsn":%q,"gameId":"demo","env":"prod"}`, goodDSN))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestDBMonHandler_UpdateSource_BadJSONBody(t *testing.T) {
	r, db := newDBMonEnvWithDB(t)
	require.NoError(t, model.NewDBSourceModel(db).Create(context.Background(),
		&model.DBSource{Name: "s", Driver: "mysql", Kind: model.DBSourceKindSelf, DSN: goodDSN,
			GameID: "demo", Env: "prod", Enabled: true}))
	var id int64
	require.NoError(t, db.Raw("SELECT id FROM db_sources LIMIT 1").Scan(&id).Error)

	rec := doDBMonReq(r, http.MethodPut, fmt.Sprintf("/dbmon/sources/%d", id), `{not-json`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDBMonHandler_ProbeAll_ServiceError(t *testing.T) {
	r, db := newDBMonEnvWithDB(t)
	require.NoError(t, db.Migrator().DropTable("db_sources"))

	rec := doDBMonReq(r, http.MethodPost, "/dbmon/probe", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
