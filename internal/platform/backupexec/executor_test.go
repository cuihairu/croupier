package backupexec

import (
	"context"
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

// fakeStore 收集 Put 调用。
type fakeStore struct {
	keys []string
}

func (f *fakeStore) Put(ctx context.Context, key string, r os.File, size int64, contentType string) error {
	f.keys = append(f.keys, key)
	return nil
}

// newTestExecutor 注入假命令执行器（pg_dump 形态验证参数装配）。
func newTestExecutorWithSQLite(t *testing.T, dsn string) (*Executor, string) {
	t.Helper()
	dir := t.TempDir()
	// sqlite 数据源文件。
	src := filepath.Join(dir, "src.db")
	require.NoError(t, os.WriteFile(src, []byte("sqlite-database-content"), 0o600))
	return New("sqlite", src, nil, nil, "backups"), src
}

func TestDumpSQLite_CopiesFile(t *testing.T) {
	e, src := newTestExecutorWithSQLite(t, "")
	defer os.Remove(src)

	// dump 直测：临时文件内容 = 源文件内容。
	path, size, checksum, err := e.dump(context.Background(), "bk-test")
	require.NoError(t, err)
	defer os.Remove(path)
	assert.Equal(t, int64(len("sqlite-database-content")), size)
	assert.Len(t, checksum, 64) // sha256 hex
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "sqlite-database-content", string(content))
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
