package healthprobe

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 假 driver：仅实现 Ping + 只读断言可观测的 QueryerContext，验证 DBChecker 的
// 「连接→Ping→只读断言」链路与 QueryModePing 跳过路径。

type fakeDBDriver struct {
	openErr  error
	pingErr  error
	queryErr error
	queries  []string
	queryHit atomic.Int32
}

func (d *fakeDBDriver) Open(string) (driver.Conn, error) {
	if d.openErr != nil {
		return nil, d.openErr
	}
	return &fakeDBConn{d: d}, nil
}

type fakeDBConn struct {
	d *fakeDBDriver
}

func (c *fakeDBConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("unreachable")
}
func (c *fakeDBConn) Close() error              { return nil }
func (c *fakeDBConn) Begin() (driver.Tx, error) { return nil, errors.New("unreachable") }

// Ping implements driver.Pinger。
func (c *fakeDBConn) Ping(context.Context) error { return c.d.pingErr }

// QueryContext implements driver.QueryerContext：记录断言语句并返回可关闭空集。
func (c *fakeDBConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.d.queryHit.Add(1)
	c.d.queries = append(c.d.queries, query)
	if c.d.queryErr != nil {
		return nil, c.d.queryErr
	}
	return &fakeDBRows{}, nil
}

type fakeDBRows struct{}

func (r *fakeDBRows) Columns() []string         { return []string{"1"} }
func (r *fakeDBRows) Close() error              { return nil }
func (r *fakeDBRows) Next([]driver.Value) error { return errors.New("no rows") }

var fakeDriverSeq atomic.Int64

func nextFakeDriverID() int64 { return fakeDriverSeq.Add(1) }

// registerFakeDB 注册一个唯一名字的假 driver（sql.Register 无注销 API，同名
// 会 panic，故用递增序号保证进程内唯一）。
func registerFakeDB(t *testing.T, d *fakeDBDriver) string {
	t.Helper()
	name := fmt.Sprintf("healthprobe-fake-%d", nextFakeDriverID())
	sql.Register(name, d)
	return name
}

func TestDBCheckerPingAndAssertion(t *testing.T) {
	d := &fakeDBDriver{}
	name := registerFakeDB(t, d)
	c := NewDBChecker("dsn", name, time.Second)

	require.NoError(t, c.Check(context.Background()))
	assert.Equal(t, int32(1), d.queryHit.Load())
	// 空 Query 走默认 SELECT 1。
	require.Len(t, d.queries, 1)
	assert.Equal(t, "SELECT 1", d.queries[0])
	assert.True(t, c.Check(context.Background()) == nil)
	assert.Equal(t, int32(2), d.queryHit.Load())
}

func TestDBCheckerCustomQuery(t *testing.T) {
	d := &fakeDBDriver{}
	name := registerFakeDB(t, d)
	c := NewDBChecker("dsn", name, time.Second)
	c.Query = "SELECT count(*) FROM players"

	require.NoError(t, c.Check(context.Background()))
	require.Len(t, d.queries, 1)
	assert.Equal(t, "SELECT count(*) FROM players", d.queries[0])
}

func TestDBCheckerPingFailure(t *testing.T) {
	d := &fakeDBDriver{pingErr: errors.New("connection refused")}
	name := registerFakeDB(t, d)
	err := NewDBChecker("dsn", name, time.Second).Check(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db ping")
	assert.Contains(t, err.Error(), "connection refused")
	// Ping 失败即返回，不执行断言。
	assert.Equal(t, int32(0), d.queryHit.Load())
}

func TestDBCheckerOpenFailure(t *testing.T) {
	// database/sql 的 Open 是懒连接：driver.Open 的首个错误经 Ping 冒泡。
	d := &fakeDBDriver{openErr: errors.New("driver exploded")}
	name := registerFakeDB(t, d)
	err := NewDBChecker("dsn", name, time.Second).Check(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db ping")
	assert.Contains(t, err.Error(), "driver exploded")
}

func TestDBCheckerQueryFailure(t *testing.T) {
	d := &fakeDBDriver{queryErr: errors.New("syntax error")}
	name := registerFakeDB(t, d)
	err := NewDBChecker("dsn", name, time.Second).Check(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db query")
	assert.Contains(t, err.Error(), "syntax error")
}

func TestDBCheckerQueryModePingSkipsAssertion(t *testing.T) {
	d := &fakeDBDriver{}
	name := registerFakeDB(t, d)
	c := NewDBChecker("dsn", name, time.Second)
	c.Query = QueryModePing

	require.NoError(t, c.Check(context.Background()))
	assert.Equal(t, int32(0), d.queryHit.Load())
	assert.Empty(t, d.queries)
	// 大小写不敏感。
	c.Query = "none"
	require.NoError(t, c.Check(context.Background()))
	assert.Equal(t, int32(0), d.queryHit.Load())
}

func TestDBCheckerMissingParams(t *testing.T) {
	d := &fakeDBDriver{}
	name := registerFakeDB(t, d)
	ctx := context.Background()

	err := NewDBChecker("", name, time.Second).Check(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dsn required")

	err = NewDBChecker("dsn", "", time.Second).Check(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "driver required")
}

func TestDBCheckerDefaultTimeout(t *testing.T) {
	// timeout<=0 时 Check 内部用 DefaultDBTimeout（不回写结构体字段）。
	d := &fakeDBDriver{}
	name := registerFakeDB(t, d)
	c := NewDBChecker("dsn", name, 0)
	require.NoError(t, c.Check(context.Background()))
}

func TestDBCheckerDetails(t *testing.T) {
	d := &fakeDBDriver{}
	name := registerFakeDB(t, d)
	c := NewDBChecker("dsn", name, time.Second)

	det := c.Details()
	assert.Equal(t, "db", det["probeType"])
	assert.Equal(t, name, det["driver"])
	assert.Equal(t, "SELECT 1", det["assertion"])

	c.Query = "SELECT 1"
	assert.Equal(t, "SELECT 1", c.Details()["assertion"])
}

// CheckWith 装配侧注入缝：dbmon 诊断端点把连通判定与诊断电池合到同一次
// 执行（同一连接）——连接已就绪时 Driver/DSN 不参与，不新开第二套连接。
func TestDBCheckerCheckWithReusesProvidedConn(t *testing.T) {
	d := &fakeDBDriver{}
	name := registerFakeDB(t, d)
	db, err := sql.Open(name, "dsn")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Driver/DSN 留空：连接已提供时不参与（只有自开连接才需要它们）。
	c := &DBChecker{Timeout: time.Second}
	require.NoError(t, c.CheckWith(context.Background(), db))
	assert.Equal(t, int32(1), d.queryHit.Load())
	require.Len(t, d.queries, 1)
	assert.Equal(t, "SELECT 1", d.queries[0])

	// Ping 失败同样短路，不执行断言。
	d2 := &fakeDBDriver{pingErr: errors.New("gone")}
	name2 := registerFakeDB(t, d2)
	db2, err := sql.Open(name2, "dsn")
	require.NoError(t, err)
	defer func() { _ = db2.Close() }()
	err = (&DBChecker{Timeout: time.Second}).CheckWith(context.Background(), db2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db ping")
	assert.Equal(t, int32(0), d2.queryHit.Load())
}
