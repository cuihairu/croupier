// 覆盖目标（组 E）：
//   - probe.Probe 的 sql.Open 失败分支（probe.go:68-71）
//   - service.UpdateSource 的更新后重读失败分支（service.go:103-105）
package dbmon

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 无 "/" 的 mysql DSN 在 go-sql-driver 的 OpenConnector 阶段即被拒绝
// （ParseDSN: missing the slash），sql.Open 返回错误 → 探测结果承载
// open 错误而非接口报错。
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

// UpdateSource 流程中的 query 序列：FindOne(#1) → Update → FindOne(#2)。
// 注入第 2 次 query 起失败的回调，覆盖更新成功后重读失败的分支。
func TestService_UpdateSource_ReloadErrorGroupE(t *testing.T) {
	dbmonSvc, srcModel, _, db := newDBMonDBFixture(t)
	ctx := context.Background()
	src := &model.DBSource{
		Name: "s", Driver: "mysql", Kind: model.DBSourceKindSelf,
		DSN: goodDSN, Enabled: true,
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

// 覆盖目标：probe.Probe 的成功主路径（probe.go res.Error=="" → res.OK=true）。
// driver 名被 driverName 钉死为已注册真 driver（mysql/pgx），无法注册同名
// 假 driver，故经 openDB 注入缝替换为 sqlmock 构造的 *sql.DB（B 类）。
func TestProbe_OKViaSQLMockGroupS(t *testing.T) {
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
