// 覆盖目标（组 J）：db.go Read 的 rows.Scan 失败分支（166.44,168.4）。
//
// database/sql 在 Rows.Next 里按驱动 Columns() 的“当次”返回值分配 lastcols，
// 而调用方（Read）此前用第一次 Columns() 的结果构造了 Scan 目标。用自定义
// 驱动让两次 Columns() 返回不同列数，即可确定性地触发
// "sql: expected N destination arguments in Scan, not M"。
package configsource

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// staticRowsV10 是列数恒定的最小 driver.Rows 实现。
type staticRowsV10 struct {
	cols []string
	rows [][]driver.Value
	pos  int
}

func (r *staticRowsV10) Columns() []string { return r.cols }
func (r *staticRowsV10) Close() error      { return nil }
func (r *staticRowsV10) Next(dest []driver.Value) error {
	if r.pos >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.pos])
	r.pos++
	return nil
}

// scanMismatchRowsV10 第一次 Columns()（Read 构造 Scan 目标用）报 2 列，
// Next 内部的第二次询问报 1 列 → lastcols 与 Scan 目标长度不一致。
type scanMismatchRowsV10 struct {
	columnCalls int
	rowsServed  int
}

func (r *scanMismatchRowsV10) Columns() []string {
	r.columnCalls++
	if r.columnCalls <= 1 {
		return []string{"id", "name"}
	}
	return []string{"id"}
}

func (r *scanMismatchRowsV10) Close() error { return nil }
func (r *scanMismatchRowsV10) Next(dest []driver.Value) error {
	if r.rowsServed >= 1 {
		return io.EOF
	}
	r.rowsServed++
	if len(dest) == 0 {
		return errors.New("no destination slots")
	}
	dest[0] = int64(1)
	return nil
}

// fakeDriverConnV10 只支持 QueryContext：拦截原始 SQL 并返回预置行。
type fakeDriverConnV10 struct {
	query func(query string) (driver.Rows, error)
}

func (c *fakeDriverConnV10) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not supported")
}
func (c *fakeDriverConnV10) Close() error              { return nil }
func (c *fakeDriverConnV10) Begin() (driver.Tx, error) { return nil, errors.New("begin not supported") }
func (c *fakeDriverConnV10) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	return c.query(query)
}

type fakeRowsDriverV10 struct {
	query func(string) (driver.Rows, error)
}

func (d fakeRowsDriverV10) Open(string) (driver.Conn, error) {
	return nil, errors.New("open must go through connector")
}

type fakeRowsConnectorV10 struct {
	query func(string) (driver.Rows, error)
}

func (c fakeRowsConnectorV10) Connect(context.Context) (driver.Conn, error) {
	return &fakeDriverConnV10{query: c.query}, nil
}
func (c fakeRowsConnectorV10) Driver() driver.Driver { return fakeRowsDriverV10{query: c.query} }

// newFakeDriverDBSourceV10 用假驱动打开 gorm 连接（绕过真实 MySQL），
// 语义与生产一致：SHOW TABLES / SELECT 均由 query 回调应答。
func newFakeDriverDBSourceV10(t *testing.T, query func(string) (driver.Rows, error)) *dbSource {
	t.Helper()
	sqlDB := sql.OpenDB(fakeRowsConnectorV10{query: query})
	t.Cleanup(func() { _ = sqlDB.Close() })
	gdb, err := gorm.Open(gormmysql.New(gormmysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	require.NoError(t, err)
	return &dbSource{db: gdb}
}

func TestDBSourceReadScanErrorV10(t *testing.T) {
	selectRows := &scanMismatchRowsV10{}
	s := newFakeDriverDBSourceV10(t, func(query string) (driver.Rows, error) {
		if strings.Contains(query, "SHOW TABLES") {
			return &staticRowsV10{
				cols: []string{"Tables_in_db"},
				rows: [][]driver.Value{{"t1"}},
			}, nil
		}
		return selectRows, nil
	})

	_, err := s.Read(context.Background(), "t1.csv")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "destination arguments in Scan")
}

// 对照组：同一假驱动下列数恒定时 Read 正常产出 CSV（确保上面的失败
// 确实来自 Scan 目标错位而非驱动接线问题）。
func TestDBSourceReadWithFakeDriverOKV10(t *testing.T) {
	s := newFakeDriverDBSourceV10(t, func(query string) (driver.Rows, error) {
		if strings.Contains(query, "SHOW TABLES") {
			return &staticRowsV10{
				cols: []string{"Tables_in_db"},
				rows: [][]driver.Value{{"game_activity"}},
			}, nil
		}
		return &staticRowsV10{
			cols: []string{"id", "name"},
			rows: [][]driver.Value{{int64(1), "Alice"}},
		}, nil
	})

	out, err := s.Read(context.Background(), "game_activity.csv")
	require.NoError(t, err)
	assert.Equal(t, "id,name\n1,Alice\n", string(out))
}
