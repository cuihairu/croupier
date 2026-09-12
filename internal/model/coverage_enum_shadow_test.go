// 覆盖目标：migrateOneEnumColumn 的 RENAME shadow column 失败分支
// （migration_enum.go:176）与 tableHasColumn 的 PRAGMA 行扫描失败 continue 分支
// （migration_legacy_cleanup.go:67）。
package model

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failRenameConnPool 委托全部查询给真实 sqlite 连接，仅拦截
// RENAME COLUMN 语句并注入失败——驱动 migrateOneEnumColumn 的
// "drop 旧列成功、rename shadow 失败" 分支。
type failRenameConnPool struct {
	*sql.DB
}

func (p *failRenameConnPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if strings.Contains(query, "RENAME COLUMN") {
		return nil, errors.New("simulated rename failure")
	}
	return p.DB.ExecContext(ctx, query, args...)
}

func TestMigrateEnumColumnsRenameShadowFailure(t *testing.T) {
	db := newMigrationTestDB(t)
	require.NoError(t, db.Exec(`CREATE TABLE messages (id INTEGER PRIMARY KEY, status TEXT)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO messages (status) VALUES ('unread')`).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	pool := &failRenameConnPool{DB: sqlDB}
	db.ConnPool = pool
	if db.Statement != nil {
		db.Statement.ConnPool = pool
	}

	err = MigrateEnumColumns(db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rename shadow column messages.status")

	// DROP COLUMN 已成功执行，说明失败确实发生在 rename 这一步。
	var cols []string
	require.NoError(t, db.Raw(`SELECT name FROM pragma_table_info('messages')`).Scan(&cols).Error)
	assert.NotContains(t, cols, "status")
	assert.Contains(t, cols, "status_enum_migrate")
}

// tableHasColumn：sqlite PRAGMA 行内 name 列为 NULL 时 Scan 失败 → continue
// → 返回 false。
func TestTableHasColumnPragmaScanErrorSkipsRow(t *testing.T) {
	db := newMigrationTestDB(t)
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	swapConnPool(db, sqlDB)

	mock.ExpectQuery(regexp.QuoteMeta("PRAGMA table_info(members)")).
		WillReturnRows(sqlmock.NewRows([]string{"cid", "name", "type", "notnull", "dflt_value", "pk"}).
			AddRow(0, nil, "TEXT", 0, nil, 0))

	assert.False(t, tableHasColumn(db, "members", "name"))
	assert.NoError(t, mock.ExpectationsWereMet())
}
