package migrate

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tryAcquireLock 的 default 兜底分支：acquireSessionLock 外层 switch 已把
// 方言过滤为 mysql/postgres/mssql，生产路径不可达——直接以未知方言驱动，
// 且该分支不触碰 conn（nil 安全）。
func TestTryAcquireLock_UnknownDialect(t *testing.T) {
	ok, err := tryAcquireLock(context.Background(), nil, "oracle")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestTryAcquireLock_Postgres(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	conn, err := sqlDB.Conn(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	mock.ExpectQuery("SELECT pg_try_advisory_lock").
		WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(true))
	ok, err := tryAcquireLock(context.Background(), conn, "postgres")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestTryAcquireLock_MySQL(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	conn, err := sqlDB.Conn(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	mock.ExpectQuery("SELECT GET_LOCK").
		WithArgs(mysqlMigrationLockName).
		WillReturnRows(sqlmock.NewRows([]string{"l"}).AddRow(int64(1)))
	ok, err := tryAcquireLock(context.Background(), conn, "mysql")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestTryAcquireLock_MySQLHeldElsewhere(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	conn, err := sqlDB.Conn(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	mock.ExpectQuery("SELECT GET_LOCK").
		WithArgs(mysqlMigrationLockName).
		WillReturnRows(sqlmock.NewRows([]string{"l"}).AddRow(int64(0)))
	ok, err := tryAcquireLock(context.Background(), conn, "mysql")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestTryAcquireLock_MSSQL(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	conn, err := sqlDB.Conn(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	mock.ExpectQuery("sp_getapplock").
		WillReturnRows(sqlmock.NewRows([]string{"r"}).AddRow(int32(0)))
	ok, err := tryAcquireLock(context.Background(), conn, "mssql")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestTryAcquireLock_MSSQLDenied(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	conn, err := sqlDB.Conn(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	mock.ExpectQuery("sp_getapplock").
		WillReturnRows(sqlmock.NewRows([]string{"r"}).AddRow(int32(-1)))
	ok, err := tryAcquireLock(context.Background(), conn, "mssql")
	require.NoError(t, err)
	assert.False(t, ok)
}
