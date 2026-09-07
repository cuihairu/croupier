package migrate

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcquireSessionLockDeadlineExceeded(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the full 60s sessionLockDeadline window")
	}
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	for i := 0; i < 300; i++ {
		mock.ExpectQuery("SELECT GET_LOCK").
			WithArgs(mysqlMigrationLockName).
			WillReturnRows(sqlmock.NewRows([]string{"l"}).AddRow(int64(0)))
	}

	start := time.Now()
	_, err = acquireSessionLock(context.Background(), sqlDB, "mysql")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not acquire migration lock")
	assert.GreaterOrEqual(t, time.Since(start), 55*time.Second)
}
