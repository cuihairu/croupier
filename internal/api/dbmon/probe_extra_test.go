// 覆盖目标：probe.go 的 probeMySQL/probePostgres 成功与失败分支（经 sqlmock
// 模拟 *sql.DB，不依赖真实 MySQL/Postgres），以及 service.go/handler.go 中
// 模型层错误路径（List/Create/Update/ProbeAll 失败、无 DSN 源的探测回退）。
package dbmon

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
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

// ginEngine 仅为可读性别名，实际就是 *gin.Engine。
type ginEngine = gin.Engine

// ---- probeMySQL / probePostgres（直接以 sqlmock 构造 *sql.DB 调用内部探针） ----

func TestProbeMySQL_SuccessViaSQLMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

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
	defer db.Close()

	// 无死锁/查询计数键 → 对应指针保持 nil；max_connections 查询失败 → Max=-1。
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
	defer db.Close()

	mock.ExpectQuery("SHOW GLOBAL STATUS").WillReturnError(errors.New("unreachable"))

	res := &ProbeResult{}
	probeMySQL(context.Background(), db, res)

	assert.Contains(t, res.Error, "status query")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProbeMySQL_LockWaitScanErrorSkipped(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SHOW GLOBAL STATUS").
		WillReturnRows(sqlmock.NewRows([]string{"Variable_name", "Value"}))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT @@max_connections")).
		WillReturnRows(sqlmock.NewRows([]string{"max_connections"}))
	// wait_secs 列给字符串 → Scan 到 float64 失败 → 该行被跳过。
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
	defer db.Close()

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
	defer db.Close()

	mock.ExpectQuery("pg_stat_activity").WillReturnError(errors.New("permission denied"))

	res := &ProbeResult{}
	probePostgres(context.Background(), db, res)

	assert.Contains(t, res.Error, "status query")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProbePostgres_DeadlockAndLockScanFailuresDegrade(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("pg_stat_activity").WillReturnRows(
		sqlmock.NewRows([]string{"cur", "active", "maxc"}).AddRow(int64(1), int64(0), int64(10)))
	// 死锁计数与锁等待查询失败 → 降级，不置 Error。
	mock.ExpectQuery("pg_stat_database").WillReturnError(errors.New("denied"))
	mock.ExpectQuery("pg_locks").WillReturnError(errors.New("denied"))

	res := &ProbeResult{}
	probePostgres(context.Background(), db, res)

	assert.Empty(t, res.Error)
	assert.Nil(t, res.DeadlockCount)
	assert.Empty(t, res.LockWaits)
	require.NoError(t, mock.ExpectationsWereMet())
}

// ---- service 层错误路径 ----

func newDBMonDBFixture(t *testing.T) (*Service, *model.DBSourceModel, *model.AlertModel, *gorm.DB) {
	t.Helper()
	name := fmt.Sprintf("dbmon_extra_%s", strings.ReplaceAll(t.Name(), "/", "_"))
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
		Name: "s", Driver: "mysql", DSN: goodDSN,
	})
	require.Error(t, err)
}

func TestDBMonService_UpdateSource_UpdateError(t *testing.T) {
	dbmonSvc, srcModel, _, db := newDBMonDBFixture(t)
	ctx := context.Background()
	src := &model.DBSource{Name: "s", Driver: "mysql", Kind: model.DBSourceKindSelf, DSN: goodDSN, Enabled: true}
	require.NoError(t, srcModel.Create(ctx, src))

	// 删除更新目标列：FindOne 成功、Validate 通过，但 Updates 报 SQL 错误。
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
	src := &model.DBSource{Name: "no-dsn", Driver: "mysql", Kind: model.DBSourceKindSelf, DSN: goodDSN, Enabled: true}
	require.NoError(t, srcModel.Create(ctx, src))
	// 绕过 service 校验把 DSN 清空，模拟历史脏数据。
	require.NoError(t, srcModel.Update(ctx, src.ID, map[string]interface{}{"dsn": ""}))

	resp, err := dbmonSvc.ProbeAll(ctx)
	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.False(t, resp.Results[0].OK)
	assert.Contains(t, resp.Results[0].Error, "no DSN")
}

// ---- handler 层错误路径 ----

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
		fmt.Sprintf(`{"name":"s","driver":"mysql","dsn":%q}`, goodDSN))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestDBMonHandler_UpdateSource_BadJSONBody(t *testing.T) {
	r, db := newDBMonEnvWithDB(t)
	require.NoError(t, model.NewDBSourceModel(db).Create(context.Background(),
		&model.DBSource{Name: "s", Driver: "mysql", Kind: model.DBSourceKindSelf, DSN: goodDSN, Enabled: true}))
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
