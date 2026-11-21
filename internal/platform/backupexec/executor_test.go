package backupexec

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMySQLDSN(t *testing.T) {
	cases := []struct {
		dsn            string
		host, port, db string
		user, pass     string
	}{
		{"user:pass@tcp(localhost:3306)/mydb?charset=utf8", "localhost", "3306", "mydb", "user", "pass"},
		{"root@tcp(db.internal:3307)/game_demo_prod", "db.internal", "3307", "game_demo_prod", "root", ""},
		{"mysql://u:p@host:3309/dbname", "host", "3309", "dbname", "u", "p"},
	}
	for _, tc := range cases {
		host, port, user, pass, db, err := parseMySQLDSN(tc.dsn)
		require.NoError(t, err, tc.dsn)
		assert.Equal(t, tc.host, host, tc.dsn)
		assert.Equal(t, tc.port, port, tc.dsn)
		assert.Equal(t, tc.user, user, tc.dsn)
		assert.Equal(t, tc.pass, pass, tc.dsn)
		assert.Equal(t, tc.db, db, tc.dsn)
	}
	_, _, _, _, _, err := parseMySQLDSN("garbage")
	assert.Error(t, err)
}

func TestParsePostgresDSN(t *testing.T) {
	// URL 形态。
	host, port, user, pass, db, err := parsePostgresDSN("postgres://u:pw@pg.internal:5433/croupier_meta?sslmode=disable")
	require.NoError(t, err)
	assert.Equal(t, "pg.internal", host)
	assert.Equal(t, "5433", port)
	assert.Equal(t, "u", user)
	assert.Equal(t, "pw", pass)
	assert.Equal(t, "croupier_meta", db)

	// key=value 形态。
	host, port, user, _, db, err = parsePostgresDSN("host=localhost user=postgres password=x dbname=croupier port=5433")
	require.NoError(t, err)
	assert.Equal(t, "localhost", host)
	assert.Equal(t, "5433", port)
	assert.Equal(t, "postgres", user)
	assert.Equal(t, "croupier", db)

	// 缺关键字段。
	_, _, _, _, _, err = parsePostgresDSN("host=localhost")
	assert.Error(t, err)
}

// writeSQLiteDB 在 path 建一个真实 SQLite 库（含 demo 表），返回行数。
// VACUUM INTO 只能作用于合法数据库文件（旧测试用纯文本冒充 sqlite 文件，
// 修复后该前提必须是真的库）。
func writeSQLiteDB(t *testing.T, path string) int {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS demo (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	n := 3
	for i := 0; i < n; i++ {
		_, err = db.Exec("INSERT INTO demo (name) VALUES (?)", fmt.Sprintf("row-%d", i))
		require.NoError(t, err)
	}
	return n
}

// countSQLiteRows 打开库并数 demo 表行数（恢复核对：快照必须可当库读回）。
func countSQLiteRows(t *testing.T, path string) int {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	var n int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM demo").Scan(&n))
	return n
}

// newTestExecutorWithSQLite 建一个真实 SQLite 源库并返回 executor + 源路径。
func newTestExecutorWithSQLite(t *testing.T, dsn string) (*Executor, string) {
	t.Helper()
	src := filepath.Join(t.TempDir(), "src.db")
	writeSQLiteDB(t, src)
	return New("sqlite", src, nil, nil, "backups"), src
}

// TestDumpSQLite_ConcurrentWriterDuringDump：dump 期间并发写入不得破坏快照
// 一致性（VACUUM INTO 以只读事务取一致性快照，WAL 写者不阻塞）。断言快照
// 至少包含 dump 开始前已提交的行，且是可读回的合法库。
func TestDumpSQLite_ConcurrentWriterDuringDump(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "wal.db")

	writer, err := sql.Open("sqlite", src)
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Close() })
	_, err = writer.Exec("PRAGMA journal_mode=WAL")
	require.NoError(t, err)
	_, err = writer.Exec("CREATE TABLE demo (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		_, err = writer.Exec("INSERT INTO demo (name) VALUES (?)", fmt.Sprintf("pre-%d", i))
		require.NoError(t, err)
	}

	e := New("sqlite", src, nil, nil, "backups")
	type dumpRes struct {
		path string
		err  error
	}
	resCh := make(chan dumpRes, 1)
	go func() {
		p, _, _, derr := e.dump(context.Background(), "bk-conc")
		resCh <- dumpRes{path: p, err: derr}
	}()

	// dump 进行中并发写入（WAL 写者与 VACUUM INTO 读事务并行；锁竞争
	// SQLITE_BUSY 属正常，计入日志不强求全部成功）。
	for i := 0; i < 20; i++ {
		if _, werr := writer.Exec("INSERT INTO demo (name) VALUES (?)", fmt.Sprintf("conc-%d", i)); werr != nil {
			t.Logf("concurrent insert %d: %v", i, werr)
		}
	}
	res := <-resCh
	require.NoError(t, res.err)
	defer func() { _ = os.Remove(res.path) }()
	assert.GreaterOrEqual(t, countSQLiteRows(t, res.path), 5,
		"快照必须至少包含 dump 开始前已提交的行")
}

func TestDumpSQLite_Snapshot(t *testing.T) {
	e, src := newTestExecutorWithSQLite(t, "")
	defer func() { _ = os.Remove(src) }()

	// dump 直测：快照是自包含库，可当库打开并读回全部行。
	path, size, checksum, err := e.dump(context.Background(), "bk-test")
	require.NoError(t, err)
	defer func() { _ = os.Remove(path) }()
	assert.Greater(t, size, int64(0))
	assert.Len(t, checksum, 64) // sha256 hex
	assert.Equal(t, 3, countSQLiteRows(t, path))
}

// TestDumpSQLite_WalCommittedRowsIncluded 是 AUD-009 回归：WAL 模式下已提交
// 事务可能尚未 checkpoint 回主文件，直接复制主文件会静默丢行（审计实证：源库
// 1 行、备份 0 行）。快照必须包含这些行。写连接全程保持打开，确保 -wal 帧不
// 被 checkpoint 回主文件（真实服务器长驻连接的形态）。
func TestDumpSQLite_WalCommittedRowsIncluded(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "wal.db")

	writer, err := sql.Open("sqlite", src)
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Close() })
	_, err = writer.Exec("PRAGMA journal_mode=WAL")
	require.NoError(t, err)
	_, err = writer.Exec("CREATE TABLE demo (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		_, err = writer.Exec("INSERT INTO demo (name) VALUES (?)", fmt.Sprintf("row-%d", i))
		require.NoError(t, err)
	}
	// 前提取证：未 checkpoint 的帧确实留在 -wal（主文件不含这些行）。
	walInfo, err := os.Stat(src + "-wal")
	require.NoError(t, err, "WAL file should exist")
	require.Greater(t, walInfo.Size(), int64(0), "uncheckpointed frames should live in the WAL")

	e := New("sqlite", src, nil, nil, "backups")
	path, _, _, err := e.dump(context.Background(), "bk-wal")
	require.NoError(t, err)
	defer func() { _ = os.Remove(path) }()
	assert.Equal(t, 5, countSQLiteRows(t, path), "快照必须包含 WAL 中已提交的全部行")
}

func TestDumpUnsupportedDriver(t *testing.T) {
	e := New("sqlserver", "whatever", nil, nil, "backups")
	_, _, _, err := e.dump(context.Background(), "bk-x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported backup driver")
}

func TestMySQLCommandArgs_NoShellInjection(t *testing.T) {
	// 通过注入 execCommand 验证：密码只走环境变量，参数不经 shell。
	e := New("mysql", "u:secret@tcp(h:3306)/db", nil, nil, "")
	var gotName string
	var gotArgs []string
	e.execCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		gotName, gotArgs = name, args
		// 触发 --result-file 写出。
		for _, a := range args {
			if v, ok := stringsCutPrefix(a, "--result-file="); ok {
				_ = os.WriteFile(v, []byte("dump"), 0o600)
			}
		}
		return nil, nil
	}
	_, _, _, err := e.dump(context.Background(), "bk-m")
	require.NoError(t, err)
	assert.Equal(t, "mysqldump", gotName)
	joined := fmt.Sprint(gotArgs)
	assert.NotContains(t, joined, "secret") // 密码不得出现在命令行参数
}

func stringsCutPrefix(s, p string) (string, bool) {
	if len(s) >= len(p) && s[:len(p)] == p {
		return s[len(p):], true
	}
	return "", false
}

func TestObjectKey(t *testing.T) {
	e := New("mysql", "", nil, nil, "backups/")
	assert.Regexp(t, `^backups/\d{8}/full-bk1\.sql$`, e.objectKey("bk1", "full"))

	e2 := New("sqlite", "", nil, nil, "")
	assert.Regexp(t, `^backups/\d{8}/full-bk2\.sqlite$`, e2.objectKey("bk2", "full"))
}
