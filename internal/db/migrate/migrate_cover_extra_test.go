package migrate

import (
	"context"
	"regexp"
	"testing"
	"testing/fstest"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// expectMigratedMySQL simulates a MySQL that has "migrated to 21, nothing pending":
// session lock acquired, version table exists, goose has no pending migrations.
func expectMigratedMySQL(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("SELECT GET_LOCK").
		WithArgs(mysqlMigrationLockName).
		WillReturnRows(sqlmock.NewRows([]string{"l"}).AddRow(int64(1)))
	// ensureUpToDate version table probe (COUNT)
	mock.ExpectQuery("information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
	// goose HasPending: TableExists probe (EXISTS)
	mock.ExpectQuery("information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"e"}).AddRow(true))
	// goose HasPending: ListMigrations (both 1 and 21 applied → nothing pending, nothing missing)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version_id, is_applied from goose_db_version ORDER BY id DESC")).
		WillReturnRows(sqlmock.NewRows([]string{"version_id", "is_applied"}).
			AddRow(int64(21), true).
			AddRow(int64(1), true))
	// CurrentVersion version table probe (COUNT)
	mock.ExpectQuery("information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
}

func expectReleaseLock(mock sqlmock.Sqlmock) {
	mock.ExpectExec("SELECT RELEASE_LOCK").
		WillReturnResult(sqlmock.NewResult(0, 0))
}

func newMySQLTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	return db, mock
}

// Up succeeds but COALESCE(MAX(version_id),0) returns NULL → ok=false →
// "no recorded version after Up".
func TestEnsureUpToDate_NoRecordedVersionAfterUp(t *testing.T) {
	db, mock := newMySQLTestDB(t)
	expectMigratedMySQL(mock)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version")).
		WillReturnRows(sqlmock.NewRows([]string{"m"}).AddRow(nil))
	expectReleaseLock(mock)

	fsys := fstest.MapFS{"0001_baseline.sql": &fstest.MapFile{Data: []byte("-- +goose Up\nSELECT 1;\n\n-- +goose Down\nSELECT 1;\n")}}
	_, err := ensureUpToDate(context.Background(), db, fsys, ScopeMeta, nil)
	assert.ErrorContains(t, err, "no recorded version after Up")
}

// Up succeeds but the version read returns a value that cannot convert to int64 → CurrentVersion errors.
func TestEnsureUpToDate_CurrentVersionReadErrorAfterUp(t *testing.T) {
	db, mock := newMySQLTestDB(t)
	expectMigratedMySQL(mock)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version")).
		WillReturnRows(sqlmock.NewRows([]string{"m"}).AddRow("not-a-number"))
	expectReleaseLock(mock)

	fsys := fstest.MapFS{"0001_baseline.sql": &fstest.MapFile{Data: []byte("-- +goose Up\nSELECT 1;\n\n-- +goose Down\nSELECT 1;\n")}}
	_, err := ensureUpToDate(context.Background(), db, fsys, ScopeMeta, nil)
	assert.Error(t, err)
}
