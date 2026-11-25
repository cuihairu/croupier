package migrate

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The default fallback branch of tryAcquireLock: the outer switch in
// acquireSessionLock already filters dialects to mysql/postgres/mssql, so this
// branch is unreachable in production — drive it directly with an unknown
// dialect; the branch never touches conn (nil-safe).
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
